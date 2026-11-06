#!/usr/bin/env python3
"""生产资产防泄漏检查：被 git 跟踪的源码/脚本/文档里不允许出现真实公网 IP。

允许：私有/回环/链路本地/CGNAT/组播/保留地址、RFC 5737/3849 文档段、
常见公共 DNS（默认配置会用到）。其余公网 IP 一律拦下。

背景：曾把生产机 IP 写进注释，随公开仓库泄露过一次。
"""
from __future__ import annotations

import ipaddress
import re
import subprocess
import sys

DOC_NETS = [
    ipaddress.ip_network("192.0.2.0/24"),
    ipaddress.ip_network("198.51.100.0/24"),
    ipaddress.ip_network("203.0.113.0/24"),
    ipaddress.ip_network("2001:db8::/32"),
    # Cloudflare 公共服务地址（WARP/DoH 默认值），不是自有资产
    ipaddress.ip_network("104.16.0.0/13"),
    ipaddress.ip_network("2606:4700::/32"),
]
CGNAT = ipaddress.ip_network("100.64.0.0/10")
ALLOW = {
    ipaddress.ip_address(x)
    for x in (
        "1.2.3.4",
        "1.2.3.5",
        "1.2.3.6",
        "2.2.2.2",
        "3.3.3.3",
        "5.5.5.5",
        "5.6.7.8",
        "198.51.101.1",
        "162.159.198.2",
        "203.0.114.1",
        "2001:4860:4860::8888",
        "104.16.1.1",
        "8.8.8.8",
        "8.8.4.4",
        "1.1.1.1",
        "1.0.0.1",
        "9.9.9.9",
        "223.5.5.5",
        "119.29.29.29",
        "114.114.114.114",
        "208.67.222.222",
        "208.67.220.220",
        "2606:4700:4700::1111",
        "2400:3200::1",
        "2402:4e00::",
    )
}
SKIP_PREFIXES = (
    "node_modules/",
    "dist/",
    "internal/app/embed/dist/",
    "scripts/e2e/artifacts/",
)
# 只扫源码/脚本/文档；规则数据（.rule/.list/.json 等）里全是公网网段，不参与判断。
SCAN_SUFFIXES = (
    ".go", ".js", ".vue", ".ts", ".py", ".sh", ".md", ".yml", ".yaml",
)
SCAN_NAMES = (".env.example", "Makefile", "Dockerfile")
IP_RE = re.compile(
    r"(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])"
    r"|(?<![\w:])(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}(?![\w:])"
)


def tracked_files() -> list[str]:
    out = subprocess.check_output(["git", "ls-files", "-z"], text=True)
    return [p for p in out.split("\0") if p]


def allowed(ip: ipaddress._BaseAddress) -> bool:
    if ip in ALLOW:
        return True
    if any(ip in net for net in DOC_NETS):
        return True
    if ip.version == 4 and ip in CGNAT:
        return True
    return (
        ip.is_private
        or ip.is_loopback
        or ip.is_link_local
        or ip.is_multicast
        or ip.is_reserved
        or ip.is_unspecified
    )


def main() -> int:
    bad = 0
    for path in tracked_files():
        if path.startswith(SKIP_PREFIXES):
            continue
        if not (path.endswith(SCAN_SUFFIXES) or path.endswith(SCAN_NAMES)):
            continue
        if path == "scripts/check-prod-ips.py":
            continue
        try:
            text = open(path, encoding="utf-8", errors="ignore").read()
        except OSError:
            continue
        for lineno, line in enumerate(text.splitlines(), 1):
            for m in IP_RE.finditer(line):
                raw = m.group(0)
                # CIDR 网段（a.b.c.0/16）属于规则数据，不算资产泄漏
                if m.end() < len(line) and line[m.end()] == "/":
                    continue
                if m.start() > 0 and line[m.start() - 1] == "/":
                    continue
                try:
                    ip = ipaddress.ip_address(raw)
                except ValueError:
                    continue
                if allowed(ip):
                    continue
                print(f"::error file={path},line={lineno}::疑似生产 IP 泄漏: {raw}")
                bad += 1
    if bad:
        print("发现疑似生产 IP：请改用 RFC 5737/3849 文档地址（203.0.113.x / 2001:db8::x）。",
              file=sys.stderr)
        return 1
    print("no production IPs found")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
