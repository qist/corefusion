#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把现有系统 nginx 配置迁移到 CoreFusion 托管（raw 保真模式）。

做法：把 /etc/nginx/nginx.conf 里的 `include conf.d/*.conf`、`*.stream` 展开成
一份自包含的 nginx.conf，然后写入 CoreFusion 的 `proxy.nginx_settings.raw_main_config`。
CoreFusion 只负责启动/停止/reload nginx 进程，配置内容保持原样（证书绝对路径、
WAF lua、map、quic、SNI 分流都不变）。

为什么用 raw 模式而不是站点模板：
  - 生产 nginx 有 `map $ssl_preread_protocol`、多 server 共用 `127.0.0.1:443`、
    `unix:/dev/shm/anysni.sock`、if/set/grpc_pass 等，模板无法等价表达；
  - raw 模式 100% 保真，风险最低；之后可以逐站点迁到模板（raw_server）。

用法：
  # 1. 先在面板机上合并配置并检查（不写面板）
  python3 scripts/migrate-nginx-to-corefusion.py --root /root/migrate/nginx --dry-run

  # 2. 写入面板设置
  python3 scripts/migrate-nginx-to-corefusion.py --root /root/migrate/nginx \
      --panel http://127.0.0.1:9080 --user admin --password 'xxx'

  # 3. 停系统 nginx 后，由 CoreFusion 启动（也可以面板上点）
  python3 scripts/migrate-nginx-to-corefusion.py ... --start

依赖：Python 3.8+，仅标准库。
"""

import argparse
import glob
import json
import os
import re
import sys
import urllib.error
import urllib.request

INCLUDE_RE = re.compile(r'^(\s*)include\s+([^;]+);\s*$')
# 这些 include 不展开：mime.types 是标准表，保留绝对路径即可。
KEEP_INCLUDE = ("mime.types",)


def read(path):
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()


def expand(path, root, depth=0, seen=None):
    """展开 include，返回自包含配置文本。"""
    if seen is None:
        seen = set()
    real = os.path.realpath(path)
    if real in seen:
        return "# skipped recursive include: %s\n" % path
    seen.add(real)

    out = []
    for line in read(path).splitlines():
        m = INCLUDE_RE.match(line)
        if not m:
            out.append(line)
            continue
        indent, spec = m.group(1), m.group(2).strip().strip('"').strip("'")
        if any(spec.endswith(k) for k in KEEP_INCLUDE):
            out.append(line)
            continue
        pattern = spec
        if pattern.startswith("/etc/nginx/"):
            # 把生产绝对路径映射到 --root 指定的目录（本地拷贝/离线合并时用）。
            pattern = os.path.join(root, pattern[len("/etc/nginx/"):])
        elif not os.path.isabs(pattern):
            pattern = os.path.join(root, pattern)
        files = sorted(glob.glob(pattern))
        if not files:
            # 找不到就保留原样，让 nginx -t 报错暴露问题，而不是静默丢配置。
            out.append(line)
            continue
        out.append("%s# ==== expanded: %s (%d file(s)) ====" % (indent, spec, len(files)))
        for f in files:
            if os.path.isdir(f):
                continue
            if depth > 8:
                out.append("# include depth exceeded: %s" % f)
                continue
            out.append("%s# ---- %s ----" % (indent, os.path.basename(f)))
            body = expand(f, root, depth + 1, seen)
            for ln in body.splitlines():
                out.append((indent + ln) if ln.strip() else ln)
        out.append("%s# ==== end: %s ====" % (indent, spec))
    return "\n".join(out) + "\n"


class Panel:
    def __init__(self, base):
        self.base = base.rstrip("/")
        self.token = None

    def call(self, method, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(self.base + path, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode()).get("data")
        except urllib.error.HTTPError as e:
            raise RuntimeError("%s %s -> HTTP %s: %s"
                               % (method, path, e.code, e.read().decode("utf-8", "replace")[:400]))

    def login(self, user, password):
        self.token = self.call("POST", "/api/v1/auth/login",
                               {"username": user, "password": password})["access_token"]


def main():
    ap = argparse.ArgumentParser(description="迁移系统 nginx 到 CoreFusion（raw 保真模式）")
    ap.add_argument("--root", required=True, help="nginx 配置目录（含 nginx.conf 与 conf.d/）")
    ap.add_argument("--out", default="nginx.raw.conf", help="合并后的配置输出路径")
    ap.add_argument("--panel", default="http://127.0.0.1:8080")
    ap.add_argument("--user")
    ap.add_argument("--password")
    ap.add_argument("--node", type=int, default=0, help="目标节点 ID，默认自动选本地节点")
    ap.add_argument("--start", action="store_true", help="写入后立即启动 CoreFusion nginx")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    main_conf = os.path.join(args.root, "nginx.conf")
    if not os.path.isfile(main_conf):
        sys.exit("找不到 %s" % main_conf)

    merged = expand(main_conf, args.root)
    with open(args.out, "w", encoding="utf-8") as f:
        f.write(merged)

    servers = len(re.findall(r'^\s*server\s*\{', merged, re.M))
    streams = len(re.findall(r'^\s*stream\s*\{', merged, re.M))
    includes_left = len(re.findall(r'^\s*include\s+', merged, re.M))
    print("合并完成：%s" % args.out)
    print("  server 块: %d，stream 块: %d，保留 include: %d，大小: %d bytes"
          % (servers, streams, includes_left, len(merged)))
    for line in merged.splitlines():
        if re.match(r'^\s*include\s+', line):
            print("  保留 include: %s" % line.strip())
    for line in merged.splitlines():
        if "# skipped recursive" in line or "# include depth" in line:
            print("  WARN: %s" % line.strip())

    if args.dry_run:
        print("\n[dry-run] 未写入面板。请先人工检查输出文件，再执行写入。")
        return 0

    if not args.user or not args.password:
        sys.exit("执行写入需要 --user/--password（或先 --dry-run）")

    panel = Panel(args.panel)
    panel.login(args.user, args.password)

    server_id = args.node
    if server_id == 0:
        data = panel.call("GET", "/api/v1/node-servers")
        items = data.get("items") if isinstance(data, dict) else data
        local = [s for s in (items or []) if s.get("is_local") or s.get("id") == 1]
        server_id = (local or items)[0]["id"]
    print("目标节点 ID: %s" % server_id)

    settings = panel.call("GET", "/api/v1/node-servers/%d/proxy/nginx-settings" % server_id) or {}
    settings["raw_main_config"] = merged
    panel.call("POST", "/api/v1/node-servers/%d/proxy/nginx-settings" % server_id, settings)
    print("已写入 raw_main_config（保留现有 nginx_settings 其余字段）")

    if args.start:
        panel.call("POST", "/api/v1/node-servers/%d/proxy/start?type=nginx" % server_id)
        print("已请求启动 nginx（CoreFusion 托管）")
    else:
        print("下一步：停掉系统 nginx，然后在面板启动 nginx，或加 --start 重跑。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
