#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把系统 nginx（配合 xray-ui）的站点按 CoreFusion 现有设计对等迁移。

迁移目标（不是 raw 透传）：
  * 每个域名组 → CoreFusion 站点（同域名多 location 自动合并成一个 server 块）
  * location → template=http/grpc/websocket/static/return_code/redirect
  * 证书 → 上传到 CoreFusion 证书中心，站点引用 dataDir 相对路径
  * stream ssl_preread → template=sni 的记录（一条域名一条）
  * 非 TLS 探测流量 → settings.stream_nontls_target
  * 公网 443 被 stream 占用、HTTP 只监听 127.0.0.1:443 → listen_port=127.0.0.1:443 + quic_listen=443

脚本只读取 nginx 配置与证书，所有写入都走 CoreFusion 管理端 API。

用法：
  # 1) 生成计划并人工核对（不写面板）
  python3 scripts/migrate-nginx-sites.py --root /root/migrate/nginx \
      --certs-dir /root/migrate/sslkey --panel http://127.0.0.1:9080 --dry-run

  # 2) 执行（上传证书 + 建站点 + SNI + settings）
  python3 scripts/migrate-nginx-sites.py --root /root/migrate/nginx \
      --certs-dir /root/migrate/sslkey \
      --panel http://127.0.0.1:9080 --user admin --password 'xxx'
"""

import argparse
import glob
import json
import os
import re
import sys
import urllib.error
import urllib.request

# ---------------------------------------------------------------- nginx 解析


def strip_comments(text):
    return "\n".join(l for l in text.splitlines() if not l.strip().startswith("#"))


def brace_block(text, start):
    """从 start（指向 '{'）开始配平花括号，返回块内文本与结束位置。"""
    d = 0
    for i in range(start, len(text)):
        if text[i] == "{":
            d += 1
        elif text[i] == "}":
            d -= 1
            if d == 0:
                return text[start + 1:i], i
    return text[start + 1:], len(text)


def find_blocks(text, keyword):
    out = []
    for m in re.finditer(r"(?m)^\s*" + keyword + r"\s*\{", text):
        s = text.index("{", m.start())
        body, _ = brace_block(text, s)
        out.append(body)
    return out


def parse_directives(body):
    """返回 (单行指令列表, 嵌套块列表[(head, inner)])。"""
    singles, blocks = [], []
    i, d, buf = 0, 0, ""
    while i < len(body):
        ch = body[i]
        if ch == "{":
            head = buf.strip()
            inner, end = brace_block(body, i)
            blocks.append((head, inner))
            buf = ""
            i = end + 1
            continue
        if ch == "}":
            i += 1
            continue
        if ch == ";":
            line = buf.strip()
            if line:
                singles.append(line)
            buf = ""
            i += 1
            continue
        buf += ch
        i += 1
    if buf.strip():
        singles.append(buf.strip())
    return singles, blocks


def parse_upstreams(path):
    result = {}
    if not os.path.isfile(path):
        return result
    text = strip_comments(open(path, encoding="utf-8", errors="replace").read())
    for body in find_blocks(text, "upstream"):
        # upstream 名在块前，需从原文取
        pass
    for m in re.finditer(r"(?m)^\s*upstream\s+([^\s{]+)\s*\{", text):
        s = text.index("{", m.start())
        body, _ = brace_block(text, s)
        servers = re.findall(r"(?m)^\s*server\s+([^\s;]+)", body)
        if servers:
            result[m.group(1)] = servers
    return result


def listen_addr(listen):
    """'127.0.0.1:443 ssl proxy_protocol' -> '127.0.0.1:443'"""
    return listen.split()[0]


def parse_servers(root):
    servers = []
    for fn in sorted(glob.glob(os.path.join(root, "conf.d", "*.conf"))):
        if ".bak" in fn:
            continue
        text = strip_comments(open(fn, encoding="utf-8", errors="replace").read())
        for body in find_blocks(text, "server"):
            singles, _ = parse_directives(body)
            listens = [d[len("listen "):] for d in singles if d.startswith("listen ")]
            names = []
            for d in singles:
                if d.startswith("server_name "):
                    names += d[len("server_name "):].split()
            certs = [d[len("ssl_certificate "):] for d in singles if d.startswith("ssl_certificate ")]
            servers.append({
                "file": os.path.basename(fn),
                "listens": listens,
                "names": names,
                "certs": certs,
                "body": body,
                "directives": singles,
            })
    return servers


def parse_locations(body):
    out = []
    for m in re.finditer(r"(?m)^\s*location\s+([^\s{]+)\s*\{", body):
        s = body.index("{", m.start())
        inner, _ = brace_block(body, s)
        out.append((m.group(1), inner))
    return out


def parse_sni(stream_path, upstreams):
    """default.stream -> [(domain, target)] + fallback target。"""
    if not os.path.isfile(stream_path):
        return [], ""
    text = strip_comments(open(stream_path, encoding="utf-8", errors="replace").read())
    # stream 内的 upstream 块
    local_up = dict(upstreams)
    for m in re.finditer(r"(?m)^\s*upstream\s+([^\s{]+)\s*\{", text):
        s = text.index("{", m.start())
        body, _ = brace_block(text, s)
        addrs = re.findall(r"(?m)^\s*server\s+([^\s;]+)", body)
        if addrs:
            local_up[m.group(1)] = addrs

    rules, fallback = [], ""
    for m in re.finditer(r"(?m)^\s*map\s+\$ssl_preread_server_name\s+\S+\s*\{", text):
        s = text.index("{", m.start())
        body, _ = brace_block(text, s)
        for line in body.splitlines():
            parts = line.strip().split()
            if len(parts) != 2:
                continue
            domain, target = parts[0], parts[1].rstrip(";")
            if domain == "default":
                fallback = ",".join(local_up.get(target, [target]))
                continue
            if domain.startswith("~"):
                continue  # 正则 SNI 不支持
            rules.append((domain, ",".join(local_up.get(target, [target]))))
    return rules, fallback


# ---------------------------------------------------------------- API


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
            with urllib.request.urlopen(req, timeout=60) as resp:
                return json.loads(resp.read().decode()).get("data")
        except urllib.error.HTTPError as e:
            raise RuntimeError("%s %s -> HTTP %s: %s"
                               % (method, path, e.code, e.read().decode("utf-8", "replace")[:400]))

    def login(self, user, password):
        self.token = self.call("POST", "/api/v1/auth/login",
                               {"username": user, "password": password})["access_token"]


# ---------------------------------------------------------------- 转换


def upstream_value(proxy_target, upstreams):
    """把 proxy_pass 目标解析成 CoreFusion 的 upstream 值。"""
    m = re.match(r"^(https?)://([^/]+)(/.*)?$", proxy_target)
    if m:
        scheme, host, uri = m.group(1), m.group(2), m.group(3) or ""
        if host in upstreams:
            addr = upstreams[host][0]
            # 直接上游：保留 scheme 与 URI（含 "://" 时生成器会原样 proxy_pass）
            return "%s://%s%s" % (scheme, addr, uri)
        return proxy_target
    return proxy_target


def detect_template(pass_target, loc_body):
    if pass_target.startswith("grpc://") or pass_target.startswith("grpcs://"):
        return "grpc"
    if re.search(r"(?m)^\s*root\s+", loc_body):
        return "static"
    if re.search(r"(?m)^\s*proxy_set_header\s+Upgrade\s+", loc_body):
        return "websocket"
    return "http"


def cert_domain_from_path(p):
    # /apps/nginx/sslkey/example.com/ecc/fullchain.crt -> example.com
    parts = [x for x in p.split("/") if x]
    if "sslkey" in parts:
        i = parts.index("sslkey")
        if i + 1 < len(parts):
            return parts[i + 1]
    return ""


def build_plan(root, certs_dir, upstreams):
    servers = parse_servers(root)
    sites = []
    skipped = []
    cert_dirs = set()

    for sv in servers:
        listens = sv["listens"]
        # 只迁 443 的业务 server 与 8080（HTTP）
        is_tls = any("127.0.0.1:443" in l or re.search(r"(^|:)\s*443(\s|$)", l) for l in listens)
        is_8080 = any(l.startswith("8080") for l in listens)
        if not (is_tls or is_8080):
            # 非 TLS 兜底 server（stream_nontls_target 指向它）必须迁移，
            # 否则非 TLS 探测流量会被转到不存在的后端。
            if any("127.0.0.1:8444" in l for l in listens):
                rets = re.findall(r"(?m)^\s*return\s+([^;]+);", sv["body"])
                sites.append({
                    "domain": "non-tls.invalid",
                    "template": "return_code",
                    "path": "/",
                    "upstream": "",
                    "upstream_tls": False,
                    "listen_port": "127.0.0.1:8444",
                    "proxy_protocol": True,
                    "enable_http2": False,
                    "enable_quic": False,
                    "cert_domain": "",
                    "backend_path": (rets[0].strip() if rets else "403"),
                    "server_config": "",
                    "location_config": "",
                    "enabled": True,
                })
            else:
                skipped.append((sv["file"], sv["names"], listens, "非 443/8080 的探测或内部 server"))
            continue
        if is_8080:
            # CoreFusion 一个域名只有一个 server 块（按域名组合并），无法同时表达
            # 同域名的 8080 与 443 两个 server；8080 只是 CDN 回源的静态页，先跳过。
            skipped.append((sv["file"], sv["names"], listens, "同域名 8080 回源页（单域名单 server 块无法并存）"))
            continue

        names = sv["names"] or ["_"]
        domain = ", ".join(names)
        listen_port = "443"
        if is_8080:
            listen_port = "8080"
        else:
            for l in listens:
                if "127.0.0.1:443" in l:
                    listen_port = "127.0.0.1:443"
                    break

        quic_listen = ""
        has_quic = False
        for l in listens:
            if re.search(r"\bquic\b", l) and not l.startswith("[::1]"):
                quic_listen = listen_addr(l)
                has_quic = True

        proxy_protocol = any("proxy_protocol" in l for l in listens)
        has_ipv6_loopback = any(l.startswith("[::1]:443") for l in listens)
        has_unix = [listen_addr(l) for l in listens if l.startswith("unix:")]

        # 证书目录
        cert_domain = ""
        for c in sv["certs"]:
            d = cert_domain_from_path(c)
            if d:
                cert_domain = d
                cert_dirs.add(d)

        # server 级配置：只保留 CoreFusion 不自动生成的指令
        drop_prefix = ("listen ", "server_name ", "ssl_certificate ", "ssl_certificate_key ",
                       "http2 ", "ssl_protocols ", "ssl_ciphers ", "ssl_ecdh_curve ",
                       "ssl_early_data", "location ")
        srv_cfg = []
        for d in sv["directives"]:
            if d.startswith(drop_prefix):
                continue
            srv_cfg.append(d)
        if has_ipv6_loopback:
            srv_cfg.append("listen [::1]:443 ssl proxy_protocol")
            if has_quic:
                srv_cfg.append("listen [::1]:443 quic")
        for u in has_unix:
            srv_cfg.append("listen %s ssl proxy_protocol" % u)

        # TLS 参数（用于站点字段）
        ssl_protocols = next((d.split(None, 1)[1] for d in sv["directives"]
                              if d.startswith("ssl_protocols ")), "")
        ssl_ciphers = next((d.split(None, 1)[1] for d in sv["directives"]
                            if d.startswith("ssl_ciphers ")), "")
        ssl_curve = next((d.split(None, 1)[1] for d in sv["directives"]
                          if d.startswith("ssl_ecdh_curve ")), "")
        early_data = any(d == "ssl_early_data on" for d in sv["directives"])

        locs = parse_locations(sv["body"])
        if not locs:
            # 无 location 的 server（如 unix:nginx.sock）跳过
            skipped.append((sv["file"], sv["names"], listens, "server 无 location"))
            continue

        for path, loc_body in locs:
            # location 修饰符与路径分开（CoreFusion 直接拼 location %s）
            loc_path = path
            targets = re.findall(r"(?m)^\s*(proxy_pass|grpc_pass)\s+([^;]+);", loc_body)
            rets = re.findall(r"(?m)^\s*return\s+([^;]+);", loc_body)
            roots = re.findall(r"(?m)^\s*root\s+([^;]+);", loc_body)

            if targets:
                kind, target = targets[0][0], targets[0][1].strip()
                template = detect_template(target if kind == "grpc_pass" else "", loc_body)
                if kind == "grpc_pass":
                    template = "grpc"
                    target = re.sub(r"^grpcs?://", "", target)
                    if target in upstreams:
                        upstream = upstreams[target][0]
                    else:
                        upstream = target
                    upstream_tls = False
                else:
                    upstream = upstream_value(target, upstreams)
                    upstream_tls = target.startswith("https://")
            elif roots:
                template = "static"
                upstream = roots[0]
                upstream_tls = False
            elif rets:
                code = rets[0].strip()
                if re.match(r"^30[12378]\b", code) or code.startswith("https://") or code.startswith("http://"):
                    template = "redirect"
                    upstream = ""
                    upstream_tls = False
                    backend_path = code
                else:
                    template = "return_code"
                    upstream = ""
                    upstream_tls = False
                    backend_path = code
            else:
                # 没有 proxy_pass/root/return：按静态站点处理（典型的 HTTP 回源页），
                # 根目录取 server 级 root，缺省用发行版默认 html 目录。
                srv_root = next((d.split(None, 1)[1] for d in sv["directives"]
                                 if d.startswith("root ")), "/usr/share/nginx/html")
                template = "static"
                upstream = srv_root
                upstream_tls = False

            # location 内其余指令原样保留（去掉模板已生成的）
            keep = []
            for line in loc_body.splitlines():
                s = line.strip()
                if not s:
                    continue
                if re.match(r"^(proxy_pass|grpc_pass|return|root|alias)\b", s):
                    continue
                keep.append(s)

            site = {
                "domain": domain,
                "template": template,
                "path": loc_path,
                "upstream": upstream,
                "upstream_tls": upstream_tls,
                "listen_port": listen_port,
                "quic_listen": quic_listen,
                "cert_domain": cert_domain,
                "proxy_protocol": proxy_protocol,
                "enable_http2": True,
                "enable_quic": has_quic,
                "enable_ipv6": False,
                "enable_early_data": early_data,
                "ssl_protocols": ssl_protocols,
                "ssl_ciphers": ssl_ciphers,
                "ssl_ecdh_curve": ssl_curve,
                "server_config": "\n".join(srv_cfg),
                "location_config": "\n".join(keep),
                "enabled": True,
            }
            if template in ("redirect", "return_code"):
                site["backend_path"] = backend_path
            sites.append(site)

    # SNI
    stream_path = os.path.join(root, "conf.d", "default.stream")
    sni_rules, fallback = parse_sni(stream_path, upstreams)
    sni_sites = []
    for domain, target in sni_rules:
        sni_sites.append({
            "domain": domain, "template": "sni", "path": "443",
            "sni_target": target, "enabled": True,
        })
    if fallback:
        sni_sites.append({
            "domain": "fallback", "template": "sni", "path": "443",
            "sni_target": fallback, "enabled": True,
        })

    return sites, sni_sites, sorted(cert_dirs), skipped


# ---------------------------------------------------------------- 主流程


def main():
    ap = argparse.ArgumentParser(description="按 CoreFusion 站点模型对等迁移 nginx")
    ap.add_argument("--root", required=True, help="nginx 配置目录（含 nginx.conf/conf.d）")
    ap.add_argument("--certs-dir", default="/apps/nginx/sslkey",
                    help="证书根目录（含 <domain>/fullchain.crt、<domain>/private.key）")
    ap.add_argument("--panel", default="http://127.0.0.1:8080")
    ap.add_argument("--user")
    ap.add_argument("--password")
    ap.add_argument("--node", type=int, default=0)
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--skip-certs", action="store_true")
    args = ap.parse_args()

    upstreams = parse_upstreams(os.path.join(args.root, "conf.d", "upstream.conf"))
    sites, sni_sites, cert_dirs, skipped = build_plan(args.root, args.certs_dir, upstreams)

    print("== 迁移计划 ==")
    print("站点 %d 条，SNI %d 条，需要证书 %s" % (len(sites), len(sni_sites), cert_dirs or "无"))
    for s in sites:
        print("  %-45s %-9s %-8s -> %-40s listen=%s"
              % (s["domain"][:45], s["template"], s["path"], s["upstream"][:40], s["listen_port"]))
    for s in sni_sites:
        print("  SNI %-40s -> %s" % (s["domain"], s["sni_target"]))
    if skipped:
        print("\n未迁移（探测页/内部 server，不影响代理转发）：")
        for f, n, l, why in skipped:
            print("  %s %s %s: %s" % (f, n, l, why))

    if args.dry_run:
        print("\n[dry-run] 未写面板。")
        return 0
    if not args.user or not args.password:
        sys.exit("执行需要 --user/--password")

    panel = Panel(args.panel)
    panel.login(args.user, args.password)
    server_id = args.node
    if server_id == 0:
        data = panel.call("GET", "/api/v1/node-servers")
        items = data.get("items") if isinstance(data, dict) else data
        local = [s for s in (items or []) if s.get("is_local") or s.get("id") == 1]
        server_id = (local or items)[0]["id"]
    print("\n目标节点 ID: %s" % server_id)

    # 1) 上传证书
    cert_map = {}
    if not args.skip_certs:
        for d in cert_dirs:
            # 优先 ECC，再 RSA（两者按 key_type 落不同子目录，可共存）
            candidates = [
                (os.path.join(args.certs_dir, d, "ecc"), "ecc"),
                (os.path.join(args.certs_dir, d), "default"),
            ]
            for base, label in candidates:
                crt = os.path.join(base, "fullchain.crt")
                key = os.path.join(base, "private.key")
                if not (os.path.isfile(crt) and os.path.isfile(key)):
                    continue
                if not os.path.isfile(crt):
                    continue
                with open(crt, "r", encoding="utf-8", errors="replace") as f:
                    crt_content = f.read()
                with open(key, "r", encoding="utf-8", errors="replace") as f:
                    key_content = f.read()
                cert = panel.call("POST", "/api/v1/certificates/upload", {
                    "name": "%s-%s" % (d, label),
                    "domain": d,
                    "cert_content": crt_content,
                    "key_content": key_content,
                })
                # ECC 作为主证书（后上传会覆盖同 key_type；不同 key_type 分目录共存）
                if label == "ecc" or d not in cert_map:
                    cert_map[d] = cert
                print("  证书已上传: %s (%s, key=%s)" % (d, label, cert.get("key_type")))

    # 2) 建/更新站点
    existing = panel.call("GET", "/api/v1/node-servers/%d/nginx/sites" % server_id) or []
    idx = {}
    for e in existing:
        idx[(e.get("domain"), e.get("path"), e.get("template"))] = e

    created = updated = failed = 0
    for s in sites + sni_sites:
        payload = {k: v for k, v in s.items() if k != "cert_domain"}
        if s.get("cert_domain") and s["cert_domain"] in cert_map:
            payload["cert_path"] = cert_map[s["cert_domain"]].get("cert_path", "")
            payload["key_path"] = cert_map[s["cert_domain"]].get("key_path", "")
        key = (payload.get("domain"), payload.get("path"), payload.get("template"))
        try:
            cur = idx.get(key)
            if cur:
                panel.call("PUT", "/api/v1/node-servers/%d/nginx/sites/%d" % (server_id, cur["id"]), payload)
                updated += 1
            else:
                panel.call("POST", "/api/v1/node-servers/%d/nginx/sites" % server_id, payload)
                created += 1
        except Exception as e:  # noqa: BLE001
            failed += 1
            print("  FAILED %s %s: %s" % (s.get("domain"), s.get("path"), e))

    # 3) stream 设置
    settings = panel.call("GET", "/api/v1/node-servers/%d/proxy/nginx-settings" % server_id) or {}
    settings.update({
        "stream_sni_enabled": True,
        "stream_nontls_target": "127.0.0.1:8444",
        "stream_proxy_protocol": True,
        "stream_ipv6": True,
        "ipv6": False,
        "waf_enabled": False,
        "custom_config": (settings.get("custom_config") or "") +
                         "\nresolver 127.0.0.1 valid=10s;\nresolver_timeout 60s;"
                         "\nproxy_ignore_client_abort on;\nclient_header_timeout 900;"
                         "\nclient_body_timeout 900;\nsend_timeout 900;",
    })
    panel.call("POST", "/api/v1/node-servers/%d/proxy/nginx-settings" % server_id, settings)

    print("\n== 汇总 ==")
    print("created=%d updated=%d failed=%d" % (created, updated, failed))
    if failed:
        return 1
    print("下一步：面板 → 代理 → 生成配置，先人工比对再 reload。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
