#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把 xray-ui（qist/xray-ui）的 Xray 配置迁移到 CoreFusion。

迁移范围（只处理 Xray 侧，不改 nginx）：
  * 入站（inbounds）：tag/remark/protocol/listen/port/settings/streamSettings/sniffing 原样透传
  * 出站（outbounds）：direct/blocked/wireguard 等原样透传
  * 路由（routing）：domain/ip/geoip -> outboundTag 原样透传（自动剔除 api 规则）

xray-ui 的 client 通常没有 email，而 CoreFusion 订阅按
`{客户邮箱}_{入站tag片段}` 精确匹配，没有 email 会被静默跳过。
默认行为（--email-mode auto）：按**凭据**分组，为每组随机生成一个 base email
（`u<8hex>@cf.local`），再把各入站里的 client 补成 `{base}_{tag片段}`。
同一凭据在多个入站复用时，会得到同一个客户邮箱（不同 tag 后缀），凭据本身不变，
旧分享链接继续可用。

需要固定邮箱时用 --assign-emails user@example.com；要原样保留用 --email-mode keep。

用法示例：
  # 1. 先看迁移计划（不写任何东西）
  python3 scripts/migrate-xray-ui.py --db /tmp/x-ui.db --config /tmp/config.json --dry-run

  # 2. 确认后执行
  python3 scripts/migrate-xray-ui.py --db /tmp/x-ui.db --config /tmp/config.json \
      --panel http://127.0.0.1:8080 --user admin --password 'xxx'

  # 3. 端口需要手工指定时
  python3 scripts/migrate-xray-ui.py ... --port-map inbound-20002=20002 --port-map inbound-51878=443

依赖：Python 3.8+，仅标准库。
"""

import argparse
import hashlib
import json
import sqlite3
import sys
import urllib.error
import urllib.request


# ---------------------------------------------------------------- HTTP 客户端


class Panel:
    def __init__(self, base, token=None):
        self.base = base.rstrip("/")
        self.token = token

    def call(self, method, path, body=None):
        url = self.base + path
        data = json.dumps(body).encode("utf-8") if body is not None else None
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                raw = resp.read().decode("utf-8")
        except urllib.error.HTTPError as e:
            detail = e.read().decode("utf-8", "replace")
            raise RuntimeError("%s %s -> HTTP %s: %s" % (method, path, e.code, detail[:500]))
        return json.loads(raw).get("data")

    def login(self, username, password):
        data = self.call("POST", "/api/v1/auth/login",
                         {"username": username, "password": password})
        self.token = data["access_token"]
        return data


# ---------------------------------------------------------------- 读取 3x-ui


def load_inbounds(db_path):
    conn = sqlite3.connect("file:%s?mode=ro" % db_path, uri=True)
    conn.row_factory = sqlite3.Row
    rows = []
    for r in conn.execute("SELECT * FROM inbounds ORDER BY id"):
        d = dict(r)
        settings = json.loads(d.get("settings") or "{}")
        stream = json.loads(d.get("stream_settings") or d.get("streamSettings") or "{}")
        sniffing = json.loads(d.get("sniffing") or "{}")
        rows.append({
            "id": d.get("id"),
            "remark": d.get("remark") or ("inbound-%s" % d.get("port")),
            "protocol": d.get("protocol") or "",
            "tag": d.get("tag") or ("inbound-%s" % d.get("port")),
            "listen": d.get("listen") or "",
            "port": int(d.get("port") or 0),
            "enabled": bool(d.get("enable")),
            "settings": settings,
            "streamSettings": stream,
            "sniffing": sniffing,
        })
    conn.close()
    return rows


def load_runtime_config(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


# ---------------------------------------------------------------- 迁移逻辑

DEFAULT_OUTBOUND_TAGS = {"direct", "blocked"}


def is_internal_listen(listen):
    """listen 为 unix socket / abstract socket / 回环地址时，对外依赖 nginx 443。"""
    if not listen:
        return False
    return listen.startswith("/") or listen.startswith("@") or listen.startswith("unix:") \
        or listen.startswith("127.") or listen == "localhost"


# 内部协议（hysteria 回落用 socks、面板 API 用 dokodemo-door）不对外分享，
# 端口保持原值，避免把内部端口写进订阅链接。
INTERNAL_PROTOCOLS = {"socks", "dokodemo-door"}


def resolve_port(ib, public_port, port_map):
    if ib["tag"] in port_map:
        return port_map[ib["tag"]]
    if ib["protocol"].lower() in INTERNAL_PROTOCOLS:
        return ib["port"]
    if is_internal_listen(ib["listen"]):
        return public_port
    return ib["port"]


def strip_internal(obj):
    """递归删除 3x-ui 注入的 _ 前缀字段（CoreFusion 也会删，这里提前清理便于对比）。"""
    if isinstance(obj, dict):
        return {k: strip_internal(v) for k, v in obj.items() if not str(k).startswith("_")}
    if isinstance(obj, list):
        return [strip_internal(v) for v in obj]
    return obj


def tag_suffix(tag: str) -> str:
    """与 CoreFusion 的 generateInboundEmail 保持一致：
    inbound-vless-20296 -> vless20296"""
    parts = tag.split("-")
    if len(parts) >= 3:
        return parts[1] + parts[-1]
    return tag


def credential_key(item):
    """用凭据本身做"同一个用户"的判定（xray-ui 没有 email 可用）。"""
    # vless/vmess/trojan 用 id/password，hysteria 用 auth
    for k in ("id", "password", "auth"):
        v = item.get(k)
        if isinstance(v, str) and v:
            return k + ":" + v
    return None


def build_email_map(inbounds, mode, fixed):
    """凭据 -> base email。

    auto：xray-ui 没有 email，就按**凭据**分组生成一个 base
    （`u<8hex>@cf.local`），同一凭据在多个入站复用同一个 base；
    若某个 client 本来就填了 email，则拿它当 base（保留人可读的部分）。

    base 由凭据的 SHA256 派生（而不是每次真随机），这样 dry-run 看到的邮箱、
    重跑迁移、换一台新面板再迁一次，得到的都是同一套邮箱，便于对账。
    """
    mapping = {}
    if mode == "keep":
        return mapping
    for ib in inbounds:
        for key in ("clients", "users"):
            items = ib["settings"].get(key)
            if not isinstance(items, list):
                continue
            for item in items:
                if not isinstance(item, dict):
                    continue
                ck = credential_key(item)
                if not ck or ck in mapping:
                    continue
                existing = (item.get("email") or "").strip()
                if fixed:
                    mapping[ck] = fixed
                elif existing:
                    mapping[ck] = existing
                else:
                    digest = hashlib.sha256(ck.encode("utf-8")).hexdigest()[:8]
                    mapping[ck] = "u%s@cf.local" % digest
    return mapping


def assign_emails(settings, tag, email_map, mode):
    """把 client 的 email 规整成 {base}_{tag片段}，凭据不动。

    已经是 `{base}_{suffix}` 的保持原样（重复执行幂等）。CoreFusion 订阅按
    `generateInboundEmail(client.Email, tag)` 精确匹配，缺这段后缀就会跳过整个入站，
    所以即使 client 原本有 email，也要补上后缀。
    """
    if mode == "keep":
        return []
    changed = []
    suffix = tag_suffix(tag)
    for key in ("clients", "users"):
        items = settings.get(key)
        if not isinstance(items, list):
            continue
        for item in items:
            if not isinstance(item, dict):
                continue
            ck = credential_key(item)
            base = email_map.get(ck)
            if not base:
                continue
            current = (item.get("email") or "").strip()
            if current.endswith("_" + suffix):
                base = current[: -(len(suffix) + 1)]
            want = "%s_%s" % (base, suffix)
            if current == want:
                continue
            item["email"] = want
            changed.append(want)
    return changed


def build_inbound_payload(ib, public_port, port_map, email_map, email_mode):
    settings = json.loads(json.dumps(strip_internal(ib["settings"])))
    changed = assign_emails(settings, ib["tag"], email_map, email_mode)
    if changed:
        print("    [email] %s: 补齐 %d 个 -> %s" % (ib["tag"], len(changed), ", ".join(changed)))
    config = {"settings": settings}
    if ib["streamSettings"]:
        config["streamSettings"] = strip_internal(ib["streamSettings"])
    if ib["sniffing"]:
        config["sniffing"] = strip_internal(ib["sniffing"])
    port = resolve_port(ib, public_port, port_map)
    return {
        "core_type": "xray",
        "remark": ib["remark"],
        "protocol": ib["protocol"],
        "tag": ib["tag"],
        "listen": ib["listen"],
        "port": port,
        "config_json": config,
        "enabled": ib["enabled"],
    }


def build_outbound_payload(ob):
    return {
        "core_type": "xray",
        "remark": ob.get("tag") or "",
        "protocol": ob.get("protocol") or "",
        "tag": ob.get("tag") or "",
        "config_json": {k: v for k, v in ob.items() if k not in ("tag", "protocol")},
        "enabled": True,
        # xray 的兜底出站必须是第一个；CoreFusion 用 sort_order 表达，创建后再统一排序。
        "is_default": ob.get("tag") == "direct",
    }


def clean_routing(routing):
    """去掉 api 规则（CoreFusion 会自己注入 api inbound/outbound 与规则）。"""
    out = json.loads(json.dumps(routing or {}))
    rules = []
    for rule in out.get("rules") or []:
        tags = rule.get("inboundTag") or []
        if "api" in tags or rule.get("outboundTag") == "api":
            continue
        rules.append(rule)
    out["rules"] = rules
    return out


# ---------------------------------------------------------------- 主流程


def main():
    ap = argparse.ArgumentParser(description="迁移 3x-ui/xray-ui 的 Xray 配置到 CoreFusion")
    ap.add_argument("--db", required=True, help="x-ui.db 路径（只读）")
    ap.add_argument("--config", help="xray 运行配置 config.json（用于出站/路由；不给则跳过）")
    ap.add_argument("--panel", default="http://127.0.0.1:8080", help="CoreFusion 面板地址")
    ap.add_argument("--user", help="管理员用户名（执行迁移时必填）")
    ap.add_argument("--password", help="管理员密码（执行迁移时必填）")
    ap.add_argument("--token", help="直接使用 access token（与 --user/--password 二选一）")
    ap.add_argument("--node", type=int, default=0, help="目标节点 ID，默认自动选本地节点")
    ap.add_argument("--public-port", type=int, default=443,
                    help="unix socket/回环入站对外暴露的端口（默认 443）")
    ap.add_argument("--port-map", action="append", default=[],
                    metavar="TAG=PORT", help="按 tag 指定对外端口，可重复")
    ap.add_argument("--email-mode", choices=["auto", "keep", "fixed"], default="auto",
                    help="auto=按凭据随机生成（默认）；keep=保留原样；fixed=配合 --assign-emails 统一")
    ap.add_argument("--assign-emails", metavar="EMAIL",
                    help="固定邮箱（隐含 --email-mode fixed），无 email 的 client 统一用它做 base")
    ap.add_argument("--skip-inbounds", action="store_true")
    ap.add_argument("--skip-outbounds", action="store_true")
    ap.add_argument("--skip-routing", action="store_true")
    ap.add_argument("--dry-run", action="store_true", help="只打印计划，不写面板")
    args = ap.parse_args()

    port_map = {}
    for item in args.port_map:
        tag, _, port = item.partition("=")
        if not port.isdigit():
            sys.exit("--port-map 格式应为 TAG=PORT：%r" % item)
        port_map[tag] = int(port)

    inbounds = load_inbounds(args.db)
    runtime = load_runtime_config(args.config) if args.config else {}

    email_mode = "fixed" if args.assign_emails else args.email_mode
    email_map = build_email_map(inbounds, email_mode, args.assign_emails)

    print("== 迁移计划 ==")
    print("入站 %d 个，出站 %d 个，路由规则 %d 条"
          % (len(inbounds), len(runtime.get("outbounds") or []),
             len((runtime.get("routing") or {}).get("rules") or [])))
    print("\n端口推断（对外端口用于分享链接；unix/回环 -> %d）：" % args.public_port)
    for ib in inbounds:
        port = resolve_port(ib, args.public_port, port_map)
        flag = "" if ib["enabled"] else "  [disabled]"
        print("  %-28s %-10s listen=%-30r port=%s%s"
              % (ib["tag"], ib["protocol"], ib["listen"], port, flag))

    if email_map:
        print("\n随机邮箱映射（凭据前 8 位 -> base email；同凭据在多个入站会复用）：")
        for ck, email in sorted(email_map.items()):
            print("  %-20s -> %s" % (ck.split(":", 1)[1][:8], email))

    if args.dry_run:
        print("\n[dry-run] 不写面板。")
        return

    panel = Panel(args.panel)
    if args.token:
        panel.token = args.token
    else:
        if not args.user or not args.password:
            sys.exit("执行迁移需要 --user/--password 或 --token（或用 --dry-run 先看计划）")
        panel.login(args.user, args.password)
    print("\n登录成功：%s" % args.panel)

    server_id = args.node
    if server_id == 0:
        servers = panel.call("GET", "/api/v1/node-servers")
        items = servers.get("items") if isinstance(servers, dict) else servers
        if not items:
            sys.exit("面板没有节点，请先完成初始化")
        local = [s for s in items if s.get("is_local") or s.get("id") == 1]
        server_id = (local or items)[0]["id"]
    print("目标节点 ID：%s" % server_id)

    report = {"created": [], "updated": [], "skipped": [], "failed": []}

    # ---- 入站
    if not args.skip_inbounds:
        existing = panel.call("GET", "/api/v1/node-servers/%d/inbound-configs?core_type=xray" % server_id) or []
        by_tag = {c.get("tag"): c for c in existing}
        print("\n== 入站 ==")
        for ib in inbounds:
            payload = build_inbound_payload(ib, args.public_port, port_map, email_map, email_mode)
            try:
                cur = by_tag.get(payload["tag"])
                if cur:
                    panel.call("PUT", "/api/v1/node-servers/%d/inbound-configs/%d"
                               % (server_id, cur["id"]), payload)
                    report["updated"].append(("inbound", payload["tag"]))
                    print("  updated  %-28s port=%s" % (payload["tag"], payload["port"]))
                else:
                    panel.call("POST", "/api/v1/node-servers/%d/inbound-configs" % server_id, payload)
                    report["created"].append(("inbound", payload["tag"]))
                    print("  created  %-28s port=%s" % (payload["tag"], payload["port"]))
            except Exception as e:  # noqa: BLE001 - 单条失败不中断整体迁移
                report["failed"].append(("inbound", payload["tag"], str(e)))
                print("  FAILED   %-28s %s" % (payload["tag"], e))

    # ---- 出站
    if not args.skip_outbounds and runtime:
        existing = panel.call("GET", "/api/v1/node-servers/%d/outbound-configs?core_type=xray" % server_id) or []
        by_tag = {c.get("tag"): c for c in existing}
        print("\n== 出站 ==")
        for ob in runtime.get("outbounds") or []:
            tag = ob.get("tag")
            if not tag:
                continue
            payload = build_outbound_payload(ob)
            try:
                cur = by_tag.get(tag)
                if cur:
                    panel.call("PUT", "/api/v1/node-servers/%d/outbound-configs/%d"
                               % (server_id, cur["id"]), payload)
                    report["updated"].append(("outbound", tag))
                    print("  updated  %-16s %s" % (tag, payload["protocol"]))
                else:
                    panel.call("POST", "/api/v1/node-servers/%d/outbound-configs" % server_id, payload)
                    report["created"].append(("outbound", tag))
                    print("  created  %-16s %s" % (tag, payload["protocol"]))
            except Exception as e:  # noqa: BLE001
                report["failed"].append(("outbound", tag, str(e)))
                print("  FAILED   %-16s %s" % (tag, e))

    # ---- 路由
    if not args.skip_routing and runtime.get("routing"):
        routing = clean_routing(runtime["routing"])
        print("\n== 路由 == 写入 %d 条规则" % len(routing.get("rules") or []))
        try:
            panel.call("PUT", "/api/v1/node-servers/%d/protocols/xray/section/routing" % server_id,
                       routing)
            report["updated"].append(("routing", "section"))
        except Exception as e:  # noqa: BLE001
            report["failed"].append(("routing", "section", str(e)))
            print("  FAILED routing: %s" % e)

    print("\n== 汇总 ==")
    print("created=%d updated=%d failed=%d"
          % (len(report["created"]), len(report["updated"]), len(report["failed"])))
    if report["failed"]:
        print("失败项：")
        for item in report["failed"]:
            print("  - %s" % (item,))
        print("\n处理建议：REALITY serverNames 冲突时，先改冲突入站的 serverNames；"
              "端口冲突时用 --port-map 覆盖。")
        return 1
    print("迁移完成。下一步：核对 CoreFusion 生成的运行配置并重启 xray（面板会自动下发）。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
