#!/usr/bin/env python3
"""分享 / 订阅「端到端连通性」测试驱动器（阶段 0/1 骨架）。

做法：对每个用例 ① 在目标节点上建入站 → ② 等端口监听 → ③ 取四种产物（uri/mihomo/singbox/xray）
→ ④ 真机二进制加载校验 → ⑤ 用真实客户端核心起本地 socks，curl 目标 URL 验证"真的连出去"
→ ⑥ 断言出口是国家/地区（默认 HK）且与开发环境出口不同 → ⑦ 出报告。

设计约束：
- 期望值三态：PASS / SKIP(设计不支持，必须带 reason) / FAIL。SKIP 必须是「整体不生成 + 有原因」，
  不能是"生成了半个配置"。
- 入站一律 `e2e-` 前缀 + 独立端口段，**默认保留**（`--cleanup` 才删），避免污染生产配置。
- 幂等：同 tag 的入站已存在则复用，方便反复跑。

用法：
  python3 scripts/e2e/e2e_share_matrix.py --node 18 --node-host 192.168.1.117
  python3 scripts/e2e/e2e_share_matrix.py --only vless-raw-none,vless-kcp-none
  python3 scripts/e2e/e2e_share_matrix.py --dry-run          # 只打印将执行什么
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime

# ─────────────────────────── 默认路径 / 常量 ───────────────────────────
ROOT = os.environ.get("CF_ROOT", "/opt/corefusion")
XRAY_BIN = os.environ.get("E2E_XRAY", f"{ROOT}/data/binaries/active/xray/xray-linux-amd64")
SBOX_BIN = os.environ.get("E2E_SINGBOX", f"{ROOT}/data/binaries/active/singbox/singbox-linux-amd64")
MIHOMO_BIN = os.environ.get("E2E_MIHOMO", "/tmp/mihomo-bin")

TRACE_URL = "https://www.cloudflare.com/cdn-cgi/trace"   # 响应里带 ip= / loc=
EXPECT_LOC = "HK"

PORT_BASE = 31000
SOCKS_BASE = 41000
FORMATS = ["uri", "mihomo", "singbox", "xray"]
CLIENTS = ["xray", "singbox", "mihomo"]

# ─────────────────────────── 用例表（阶段 1 子集）───────────────────────────
# 期望说明：expect 里每个 format → "pass" / "skip"。
#   skip 必须表现为 supported=false 且 reason 非空（驱动器会核对）。
CASES: list[dict] = [
    dict(id="vless-raw-none", core="xray", protocol="vless", transport="raw", security="none"),
    dict(id="vless-ws-none", core="xray", protocol="vless", transport="ws", security="none",
         extra={"path": "/e2ews", "host": "e2e.local"}),
    dict(id="vless-grpc-none", core="xray", protocol="vless", transport="grpc", security="none",
         extra={"serviceName": "e2egrpc"}),
    dict(id="vless-xhttp-none", core="xray", protocol="vless", transport="xhttp", security="none",
         extra={"path": "/e2ex", "mode": "auto"}, expect={"singbox": "skip"}),
    dict(id="vless-httpupgrade-none", core="xray", protocol="vless", transport="httpupgrade", security="none",
         extra={"path": "/e2ehu", "host": "e2e.local"}),
    dict(id="vless-kcp-none", core="xray", protocol="vless", transport="mkcp", security="none",
         expect={"mihomo": "skip", "singbox": "skip"}),
    dict(id="vless-raw-vlessenc", core="xray", protocol="vless", transport="raw", security="none",
         vless_encryption=True, expect={"singbox": "skip"}),
    dict(id="vless-ws-httpcamouflage", core="xray", protocol="vless", transport="raw", security="none",
         http_camouflage={"path": ["/"], "host": "www.baidu.com"}),
    dict(id="vmess-ws-none", core="xray", protocol="vmess", transport="ws", security="none",
         extra={"path": "/e2evm", "host": "e2e.local"}),
    dict(id="ss-raw-none", core="xray", protocol="shadowsocks", transport="raw", security="none",
         ss_method="aes-256-gcm"),
    # ⚠ 面板里 ss2022 的 DB/config protocol 仍写 `shadowsocks`，靠 method 区分（照抄面板自建入站 id=70）；
    # 写成 `shadowsocks2022` 核心会 `unknown config id: shadowsocks2022` 起不来。
    # 用**单用户**形态（不带 clients[]）：实测单用户 ss2022 能让三种客户端都连通；
    # 多用户形态（带 clients[]）在**本环境的核**上握手静默失败（服务端配置与 DB 一致，
    # 标准客户端 + 另一份 26.1.18 二进制也连不上）→ 属核/部署问题，另行排查，不是分享侧。
    dict(id="ss2022-single-raw-none", core="xray", protocol="shadowsocks", transport="raw", security="none",
         ss_method="2022-blake3-aes-128-gcm"),
    dict(id="http-raw-none", core="xray", protocol="http", transport="raw", security="none"),
    dict(id="socks-raw-none", core="xray", protocol="socks", transport="raw", security="none"),
]


# ─────────────────────────── HTTP 小工具 ───────────────────────────
def api(panel: str, token: str, method: str, path: str, body: dict | None = None):
    req = urllib.request.Request(panel + path, method=method,
                                 headers={"Authorization": "Bearer " + token,
                                          "Content-Type": "application/json"})
    data = json.dumps(body).encode() if body is not None else None
    try:
        with urllib.request.urlopen(req, data, timeout=30) as r:
            raw = r.read().decode()
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        return {"code": f"HTTP{e.code}", "message": raw[:300]}
    try:
        return json.loads(raw)
    except ValueError:
        return {"raw": raw[:300]}


def tcp_open(host: str, port: int, timeout: float = 2.0) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def wait_port(host: str, port: int, seconds: int = 30) -> bool:
    deadline = time.time() + seconds
    while time.time() < deadline:
        if tcp_open(host, port):
            return True
        time.sleep(1)
    return False


def run(cmd: list[str], timeout: int = 20) -> tuple[int, str]:
    try:
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return p.returncode, (p.stdout or "") + (p.stderr or "")
    except subprocess.TimeoutExpired:
        return 124, "timeout"


def core_service_id(panel: str, token: str, node: int, core_type: str) -> int | None:
    """从 process-status 里取该核心的 service_id（用于重启核心）。"""
    r = api(panel, token, "GET", f"/api/v1/node-servers/{node}/services/process-status")
    for s in r.get("data") or []:
        if s.get("core_type") == core_type:
            return s.get("service_id")
    return None


def restart_core(panel: str, token: str, node: int, core_type: str) -> bool:
    sid = core_service_id(panel, token, node, core_type)
    if not sid:
        return False
    r = api(panel, token, "POST", f"/api/v1/node-servers/{node}/services/{sid}/restart", {})
    return bool(r.get("data", {}).get("status")) or r.get("data") is not None


# ─────────────────────────── 服务端配置生成 ───────────────────────────
def stream_settings(case: dict) -> dict:
    net = case.get("transport", "raw")
    ss: dict = {"network": net, "security": case.get("security", "none")}
    extra = case.get("extra") or {}
    if net == "raw":
        ss["rawSettings"] = {"header": {"type": "none"}}
    elif net == "ws":
        ss["wsSettings"] = {"path": extra.get("path", "/e2ews"),
                            "headers": {"Host": extra.get("host", "e2e.local")}}
    elif net == "grpc":
        ss["grpcSettings"] = {"serviceName": extra.get("serviceName", "e2egrpc")}
    elif net == "xhttp":
        ss["xhttpSettings"] = {"path": extra.get("path", "/e2ex"), "mode": extra.get("mode", "auto")}
    elif net == "httpupgrade":
        ss["httpupgradeSettings"] = {"path": extra.get("path", "/e2ehu"), "host": extra.get("host", "e2e.local")}
    elif net in ("mkcp", "kcp"):
        # ⚠ 两点务必注意（都会让**整个核心启动失败**，不是只废掉这一个入站）：
        #   ① xray 26 已移除 `kcpSettings.header/seed`；
        #   ② `finalmask.udp[].type` 的取值在两个小版本间**不通用**（节点上是 26.3.27，只认
        #      header-*/mkcp-original/mkcp-aes128gcm；本机 26.9.9 只认 mkcp-legacy）——
        #      所以用例里**不写 finalmask**，只用最小 kcp 形状（两边都实测 Configuration OK）。
        ss["kcpSettings"] = {"mtu": 1350, "tti": 50, "uplinkCapacity": 5, "downlinkCapacity": 20,
                             "congestion": True, "readBufferSize": 2, "writeBufferSize": 2}
    camo = case.get("http_camouflage")
    if camo and net == "raw":
        ss["rawSettings"] = {"header": {"type": "http", "request": {
            "path": camo.get("path", ["/"]),
            "headers": {"Host": [camo.get("host", "www.baidu.com")]},
        }}}
    return ss


def vless_enc_pair(panel: str, token: str, node: int) -> tuple[str, str]:
    """向面板要一对合法的 VLESS 加密密钥（server=decryption，client=encryption）。

    自己编一个 `mlkem768x25519plus...` 字符串是不行的：xray 会
    `VLESS settings: unsupported "decryption"` 直接拒绝加载**整个核心**。
    """
    r = api(panel, token, "POST", f"/api/v1/node-servers/{node}/binaries/vlessenc", {})
    auths = ((r.get("data") or {}).get("auths") or [])
    if not auths:
        return "", ""
    return auths[0].get("encryption", ""), auths[0].get("decryption", "")


def server_config(case: dict, port: int, client_uuid: str, client_pass: str,
                  vless_enc: tuple[str, str] = ("", "")) -> dict:
    proto = case["protocol"]
    cfg: dict = {
        "tag": "e2e-" + case["id"],
        "protocol": proto,
        "port": port,
        "listen": "0.0.0.0",
        "_accessDomain": {"enabled": False, "type": "direct", "directDomain": "", "directPortMode": "config",
                          "cfDomain": "", "cfIP": "", "customDomain": "", "customPortMode": "custom",
                          "customPort": 443},
        "_portHop": {"enabled": False, "mode": "preset", "range": "20000-50000", "start": 20000,
                     "end": 50000, "interface": ""},
        "streamSettings": stream_settings(case),
        "sniffing": {"enabled": True, "destOverride": ["http", "tls", "quic"],
                     "metadataOnly": False, "routeOnly": False},
    }
    enc = ""
    if case.get("vless_encryption"):
        enc, dec = (vless_enc if vless_enc else ("", ""))
    else:
        dec = ""
    if proto == "vless":
        cfg["settings"] = {
            "clients": [{"id": client_uuid, "email": "e2e@test.local", "flow": "", "level": 0}],
            "encryption": enc or "none",
            "decryption": dec or "none",
        }
    elif proto == "vmess":
        cfg["settings"] = {"clients": [{"id": client_uuid, "email": "e2e@test.local", "alterId": 0, "level": 0}]}
    elif proto in ("shadowsocks", "shadowsocks2022"):
        method = case.get("ss_method", "aes-256-gcm")
        # ⚠ ss2022 的密钥长度必须与 method 匹配（aes-128-gcm=16B、aes-256-gcm/chacha20=32B）：
        # 长度不对 xray 会 `bad key length` 直接拒绝加载**整个核心**
        import base64 as _b64
        if method.startswith("2022-blake3"):
            keylen = 16 if "aes-128-gcm" in method else 32
            password = _b64.b64encode(os.urandom(keylen)).decode()
            # 单用户形态：密钥即服务端密钥（多用户形态在本环境连不通，见用例注释）
            cfg["settings"] = {"method": method, "password": password, "network": "tcp"}
        else:
            cfg["settings"] = {"method": method, "password": client_pass, "network": "tcp,udp"}
    elif proto == "http":
        cfg["settings"] = {"auth": "noauth"}
    elif proto == "socks":
        cfg["settings"] = {"auth": "noauth", "udp": True}
    elif proto == "trojan":
        cfg["settings"] = {"clients": [{"password": client_pass, "email": "e2e@test.local"}]}
    else:
        cfg["settings"] = {"clients": [{"id": client_uuid, "email": "e2e@test.local"}]}
    return cfg


# ─────────────────────────── 客户端包装 + 真连 ───────────────────────────
def wrap_client(fmt: str, artifact: str, socks_port: int, workdir: str, tag: str) -> tuple[str, list[str]]:
    """把"单出站产物"包成可运行的客户端配置，返回 (配置文件, 启动命令)。"""
    if fmt == "xray":
        ob = json.loads(artifact)
        path = os.path.join(workdir, "xray_client.json")
        json.dump({"log": {"loglevel": "warning"},
                   "inbounds": [{"listen": "127.0.0.1", "port": socks_port, "protocol": "socks",
                                 "settings": {"auth": "noauth", "udp": True}}],
                   "outbounds": [ob]}, open(path, "w"), ensure_ascii=False, indent=1)
        return path, [XRAY_BIN, "run", "-c", path]
    if fmt == "singbox":
        ob = json.loads(artifact)
        path = os.path.join(workdir, "singbox_client.json")
        json.dump({"log": {"level": "warn"},
                   "inbounds": [{"type": "mixed", "listen": "127.0.0.1", "listen_port": socks_port}],
                   "outbounds": [ob],
                   "route": {"final": ob.get("tag")}}, open(path, "w"), ensure_ascii=False, indent=1)
        return path, [SBOX_BIN, "run", "-c", path]
    # mihomo：产物本身就是一段 `proxies:` YAML
    body = artifact.split("proxies:", 1)[-1]
    import re
    m = re.search(r"-\s*name:\s*(\S+)", body)
    name = m.group(1) if m else tag
    path = os.path.join(workdir, "mihomo_client.yaml")
    with open(path, "w") as f:
        f.write(f"mixed-port: {socks_port}\nproxies:" + body + f"\nrules:\n  - MATCH,{name}\n")
    return path, [MIHOMO_BIN, "-d", workdir, "-f", path]


def load_check(fmt: str, path: str, workdir: str) -> tuple[bool, str]:
    if fmt == "xray":
        code, out = run([XRAY_BIN, "run", "-test", "-c", path], timeout=25)
        ok = "Configuration OK" in out
    elif fmt == "singbox":
        code, out = run([SBOX_BIN, "check", "-c", path], timeout=25)
        ok = code == 0
    else:
        code, out = run([MIHOMO_BIN, "-t", "-d", workdir, "-f", path], timeout=25)
        ok = "test is successful" in out
    return ok, out.strip().splitlines()[-1][:200] if out.strip() else ""


def connect_check(fmt: str, cmd: list[str], socks_port: int, workdir: str) -> tuple[bool, str]:
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
                            start_new_session=True)
    try:
        if not wait_port("127.0.0.1", socks_port, 20):
            return False, "客户端端口未就绪"
        code, out = run(["curl", "-s", "--max-time", "12", "--socks5-hostname",
                         f"127.0.0.1:{socks_port}", TRACE_URL], timeout=20)
        fields = dict(l.split("=", 1) for l in out.splitlines() if "=" in l)
        ip, loc = fields.get("ip", ""), fields.get("loc", "")
        if not ip:
            return False, f"未取到出口信息: {out.strip()[:120]}"
        if loc != EXPECT_LOC:
            return False, f"出口地区 loc={loc}（期望 {EXPECT_LOC}）"
        return True, f"exit ip={ip} loc={loc}"
    finally:
        try:
            proc.terminate()
            time.sleep(0.5)
            proc.kill()
            # 留档客户端输出，失败用例才有据可查
            out_txt = proc.stdout.read() if proc.stdout else ""
            with open(os.path.join(workdir, "client.log"), "w") as f:
                f.write(out_txt or "")
        except Exception:
            pass


CLIENTS_BIN = {"xray": [XRAY_BIN], "singbox": [SBOX_BIN], "mihomo": [MIHOMO_BIN]}


# ─────────────────────────── 主流程 ───────────────────────────
def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--panel", default="http://127.0.0.1:8080")
    ap.add_argument("--token", default=os.environ.get("E2E_TOKEN", ""))
    ap.add_argument("--node", type=int, default=18, help="目标节点（node-servers id）")
    ap.add_argument("--node-host", default="192.168.1.117", help="目标节点 IP（用于拨测端口）")
    ap.add_argument("--only", default="", help="只跑指定用例 id（逗号分隔）")
    ap.add_argument("--cleanup", action="store_true", help="跑完删除 e2e- 入站")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--outdir", default=f"{ROOT}/scripts/e2e/artifacts")
    args = ap.parse_args()

    if not args.token:
        tok_file = "/tmp/cf_tok"
        if os.path.exists(tok_file):
            args.token = open(tok_file).read().strip()
    if not args.token:
        print("缺少面板 token（--token 或 E2E_TOKEN）", file=sys.stderr)
        return 2

    cases = CASES
    if args.only:
        want = {x.strip() for x in args.only.split(",") if x.strip()}
        cases = [c for c in cases if c["id"] in want]
    if args.dry_run:
        for c in cases:
            print(f"{c['id']:<28} {c['protocol']:<16} transport={c['transport']:<12} security={c['security']}")
        return 0

    vless_enc = vless_enc_pair(args.panel, args.token, args.node) if any(c.get("vless_encryption") for c in cases) else ("", "")
    run_id = datetime.now().strftime("%Y-%m-%dT%H-%M-%S")
    outdir = os.path.join(args.outdir, run_id)
    os.makedirs(outdir, exist_ok=True)
    print(f"面板={args.panel} 节点={args.node}({args.node_host}) 用例={len(cases)} 产物目录={outdir}")

    # 版本快照：结论必须可复现（面板/节点核心/三种客户端二进制）
    vers = {}
    for name, path in (("xray_client", XRAY_BIN), ("singbox_client", SBOX_BIN), ("mihomo_client", MIHOMO_BIN)):
        _, out = run([path, "version"], timeout=10) if name != "mihomo_client" else run([path, "-v"], timeout=10)
        vers[name] = (out.strip().splitlines() or [""])[0][:80]
    r = api(args.panel, args.token, "GET", f"/api/v1/node-servers/{args.node}/services/process-status")
    for s in r.get("data") or []:
        if s.get("core_type") in ("xray", "singbox"):
            vers[f"node_{s.get('core_type')}"] = f"{s.get('version')} state={s.get('state')} pid={s.get('pid')}"
    with open(os.path.join(outdir, "versions.txt"), "w") as f:
        for k, v in vers.items():
            f.write(f"{k}: {v}\n")
    print("版本: " + "; ".join(f"{k}={v}" for k, v in vers.items()))

    # 已有入站（幂等复用）；列表接口可能返回 list，也可能包成 {items: […]}
    existing = {}
    resp = api(args.panel, args.token, "GET", f"/api/v1/node-servers/{args.node}/inbound-configs?limit=500")
    data = resp.get("data")
    items = data if isinstance(data, list) else ((data or {}).get("items") or [])
    for it in items:
        existing[it.get("tag")] = it

    # 基线：开发环境出口 IP（对比用）
    _, base_out = run(["curl", "-s", "--max-time", "10", "https://api.ipify.org"], timeout=15)
    base_ip = base_out.strip()
    print(f"开发环境出口 IP（基线）= {base_ip}")

    results = []
    # 端口按**全量用例表**里的位置分配（用 --only 过滤时不能按过滤后的下标，
    # 否则会与已存在入站的端口撞车 → 测出的是别的用例的入站）
    all_ids = [c["id"] for c in CASES]
    for idx, case in enumerate(cases):
        port = PORT_BASE + all_ids.index(case["id"]) + 1
        tag = "e2e-" + case["id"]
        row = dict(case_id=case["id"], protocol=case["protocol"], transport=case["transport"],
                   security=case["security"], port=port)
        print(f"\n=== {case['id']} (port {port}) ===")
        client_uuid = str(uuid.uuid4())
        client_pass = "e2e-pass-" + uuid.uuid4().hex[:8]
        if tag not in existing:
            cfg = server_config(case, port, client_uuid, client_pass, vless_enc)
            # ⚠ config_json 必须是 **JSON 对象**（后端字段是 json.RawMessage）：
            # 传 json.dumps(cfg) 会被当成字符串再编码一次 → 面板 PrepareFullConfig 报
            # `parse inbound xxx: json: cannot unmarshal string into Go value of type map[string]interface{}`
            # → 该节点的配置整体生成失败（新入站全不生效）。
            body = {"core_type": case["core"], "remark": tag, "protocol": case["protocol"], "tag": tag,
                    "listen": "0.0.0.0", "port": port,
                    "config_json": cfg, "enabled": True}
            r = api(args.panel, args.token, "POST", f"/api/v1/node-servers/{args.node}/inbound-configs", body)
            cfg_id = (r.get("data") or {}).get("id")
            if not cfg_id:
                row.update(status="FAIL", reason=f"建入站失败: {r}")
                print("  ✗", row["reason"]); results.append(row); continue
            # 建完入站后**必须让面板立刻重新生成配置并重启核心**：
            #   ① `POST /node-servers/:id/sync {core_type}` —— 从 DB 立刻重新生成（否则要等 30s 周期，
            #      期间重启核心拿到的还是旧配置，端口永远等不到）；
            #   ② 再重启该核心。
            api(args.panel, args.token, "POST", f"/api/v1/node-servers/{args.node}/sync",
                {"core_type": case["core"]})
            restart_core(args.panel, args.token, args.node, case["core"])
        else:
            cfg_id = existing[tag]["id"]
            # 复用现有入站的客户端凭据（保证产物里的用户与服务器一致）
            try:
                cj = json.loads(existing[tag]["config_json"])
                first = (cj.get("settings", {}).get("clients") or [{}])[0]
                client_uuid = first.get("id") or client_uuid
                client_pass = (cj.get("settings", {}).get("password") or first.get("password") or client_pass)
            except Exception:
                pass
        row["cfg_id"] = cfg_id

        # 重启后核心起来要几秒；复用已有入站时也要给它一次重载机会
        if case["transport"] in ("mkcp", "kcp", "quic"):
            pass
        elif not wait_port(args.node_host, port, 25):
            restart_core(args.panel, args.token, args.node, case["core"])
            wait_port(args.node_host, port, 25)
        # kcp/quic 是 **UDP** 传输：根本没有 TCP 监听，端口预检必须跳过
        # （否则会把能用的节点误判成"端口未监听"——kcp 用例就是这么被误判的）。
        if case["transport"] in ("mkcp", "kcp", "quic"):
            pass
        elif not tcp_open(args.node_host, port):
            row.update(status="FAIL", reason=f"节点端口 {port} 未监听（配置没生效 / 核心起不来）")
            print("  ✗", row["reason"]); results.append(row); continue

        # 逐格式：取产物 → 期望核对 → 加载 → 真连
        for f_i, fmt in enumerate(FORMATS):
            r = api(args.panel, args.token, "GET",
                    f"/api/v1/node-servers/{args.node}/inbound-configs/{cfg_id}/share-links?format={fmt}")
            d = r.get("data") or {}
            items = d.get("items") or []
            expect = (case.get("expect") or {}).get(fmt, "pass")
            if not items:
                ok = (expect == "skip") and not d.get("supported", True) and bool(d.get("reason"))
                row[f"{fmt}"] = "SKIP" if ok else "FAIL"
                row[f"{fmt}_detail"] = f"supported={d.get('supported')} reason={d.get('reason')}"
                if not ok:
                    print(f"  ✗ {fmt}: 期望 {expect} 但无产物且 reason={d.get('reason')}")
                continue
            artifact = items[0].get("text") or items[0].get("link") or ""
            workdir = os.path.join(outdir, case["id"], fmt)
            os.makedirs(workdir, exist_ok=True)
            if fmt == "uri":
                row["uri"] = "PASS" if artifact.startswith(("vless://", "vmess://", "trojan://", "ss://",
                                                           "hysteria2://", "tuic://", "anytls://", "socks://",
                                                           "http://", "https://")) else "FAIL"
                row["uri_detail"] = artifact[:120]
                continue
            path, cmd = wrap_client(fmt, artifact, SOCKS_BASE + idx * 3 + f_i, workdir, tag)
            loaded, lmsg = load_check(fmt, path, workdir)
            if not loaded:
                row[fmt] = "FAIL"
                row[f"{fmt}_detail"] = f"加载失败: {lmsg}"
                print(f"  ✗ {fmt}: 加载失败 {lmsg}")
                continue
            conn, cmsg = connect_check(fmt, cmd, SOCKS_BASE + idx * 3 + f_i, workdir)
            if not conn:
                row[fmt] = "FAIL"
                row[f"{fmt}_detail"] = f"加载OK 但连不通: {cmsg}"
                print(f"  ✗ {fmt}: {cmsg}")
                continue
            if base_ip and base_ip in cmsg:
                row[fmt] = "FAIL"
                row[f"{fmt}_detail"] = f"出口与开发环境相同（没走节点）: {cmsg}"
                print(f"  ✗ {fmt}: 没走节点 {cmsg}")
                continue
            row[fmt] = "PASS"
            row[f"{fmt}_detail"] = cmsg
            print(f"  ✓ {fmt}: {cmsg}")

        # 汇总这一行
        statuses = [row.get(f, "FAIL") for f in FORMATS if f in row]
        row["status"] = "FAIL" if "FAIL" in statuses else ("PARTIAL-SKIP" if "SKIP" in statuses else "PASS")
        results.append(row)

    # 报告
    cols = ["case_id", "protocol", "transport", "security", "port", "status"] + FORMATS
    with open(os.path.join(outdir, "report.csv"), "w") as f:
        f.write(",".join(cols) + "\n")
        for row in results:
            f.write(",".join(str(row.get(c, "")) for c in cols) + "\n")
    with open(os.path.join(outdir, "report.md"), "w") as f:
        f.write(f"# E2E 分享连通性报告 {run_id}\n\n节点 {args.node} ({args.node_host})，基线出口 {base_ip}\n\n")
        f.write("| " + " | ".join(cols) + " |\n|" + "---|" * len(cols) + "\n")
        for row in results:
            f.write("| " + " | ".join(str(row.get(c, "")) for c in cols) + " |\n")
        f.write("\n## 明细\n")
        for row in results:
            f.write(f"\n### {row['case_id']} — {row.get('status')}\n")
            for k, v in row.items():
                if k.endswith("_detail"):
                    f.write(f"- {k[:-7]}: {v}\n")

    total = len(results)
    npass = sum(1 for r in results if r.get("status") == "PASS")
    nskip = sum(1 for r in results if r.get("status") == "PARTIAL-SKIP")
    nfail = total - npass - nskip
    print(f"\nTOTAL {total} | PASS {npass} | SKIP(设计) {nskip} | FAIL {nfail} → {outdir}/report.md")

    if args.cleanup:
        for row in results:
            if row.get("cfg_id"):
                api(args.panel, args.token, "DELETE",
                    f"/api/v1/node-servers/{args.node}/inbound-configs/{row['cfg_id']}")
        api(args.panel, args.token, "POST", f"/api/v1/node-servers/{args.node}/sync-data",
            {"apply_and_start": True})
        print("已删除 e2e- 入站并重新同步")
    return 1 if nfail else 0


if __name__ == "__main__":
    sys.exit(main())
