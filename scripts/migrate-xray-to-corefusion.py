#!/usr/bin/env python3
"""xray-ui（3x-ui 分支）→ CoreFusion 迁移：只迁「入口（入站）」和「路由」，不迁出口。

背景：生产 xray-ui 里只有一个 wireguard（WARP）出口，其余是 direct/blocked 内置。
CoreFusion 的出口由管理员手工创建（WARP），所以本脚本：

  - 入站：读 x-ui 的 inbounds（协议/端口/监听/settings/streamSettings/sniffing），
    通过面板 API 写成 CoreFusion 的 inbound_configs；同 tag 已存在则更新（幂等）。
  - 路由：读 x-ui 模板里的 routing 段，通过面板 API 合并进 core_configs 的
    xray/routing 段；出口 tag 按映射改写（direct/blocked 直接对应，wireguard →
    --warp-tag 指定的 CoreFusion WARP 出口）。
  - 出口：**完全不碰** outbound_configs，WARP 需要管理员先在面板里建好。

用法（在面板所在机器上执行，避免把明文令牌发到远端）：

  python3 scripts/migrate-xray-to-corefusion.py \
      --db /etc/xray-ui/xray-ui.db \
      --panel https://127.0.0.1:10111 --insecure \
      --token "$TOKEN" \
      --server-id 1 \
      --warp-tag warp-masque-nnoi \
      --dry-run

去掉 --dry-run 才会真正写入。先跑 dry-run 看报告，确认无误再执行。
"""
from __future__ import annotations

import argparse
import json
import ssl
import sqlite3
import sys
import urllib.error
import urllib.request


def api_call(panel: str, token: str, method: str, path: str, body=None, insecure=False, timeout=30):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        panel.rstrip("/") + path,
        data=data,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
        method=method,
    )
    ctx = ssl._create_unverified_context() if insecure else None
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=timeout) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as exc:
        try:
            payload = json.load(exc)
        except Exception:
            payload = {"message": exc.read().decode("utf-8", "ignore")}
        return exc.code, payload


def unwrap(payload):
    """面板统一响应是 {code, data, message}，这里取 data；兼容裸数据。"""
    if isinstance(payload, dict) and "data" in payload:
        return payload["data"]
    return payload


def normalize_listen(raw: str) -> str:
    """x-ui 的 socket 监听带权限尾巴：/dev/shm/x.sock,0666 → /dev/shm/x.sock。"""
    listen = (raw or "").strip()
    if not listen:
        return ""
    if listen.startswith("/") and "," in listen:
        listen = listen.split(",", 1)[0]
    return listen


def load_inbounds(db_path: str):
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    rows = conn.execute(
        "select id, remark, enable, listen, port, protocol, settings, stream_settings, tag, sniffing "
        "from inbounds order by id"
    ).fetchall()
    conn.close()
    return [dict(r) for r in rows]


def load_template(db_path: str):
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    row = conn.execute("select value from settings where key='xrayTemplateConfig'").fetchone()
    conn.close()
    if not row:
        return {}
    return json.loads(row[0])


# 需要在 xray 上跳过的入站协议。
# ⚠ 不要凭印象判断支持性：xray 26.x（本仓库当前二进制 26.10.31）**已支持 hysteria**
#   （protocol=hysteria + streamSettings.network=hysteria）。不确定时先用
#   `xray -test -c <config>` 实测再决定是否加入此集合。
XRAY_UNSUPPORTED = {"tuic", "anytls"}


def build_inbound_config(item, core_type: str = "xray") -> dict:
    tag = (item.get("tag") or "").strip() or f"inbound-{item['port']}"
    cfg = {
        "tag": tag,
        "protocol": item["protocol"],
        "port": int(item["port"]),
        "listen": normalize_listen(item.get("listen")),
        "settings": json.loads(item["settings"]) if item.get("settings") else {},
        "streamSettings": json.loads(item["stream_settings"]) if item.get("stream_settings") else {},
        "sniffing": json.loads(item["sniffing"]) if item.get("sniffing") else {},
    }
    return {
        "remark": item.get("remark") or tag,
        "core_type": core_type,
        "protocol": item["protocol"],
        "tag": tag,
        "listen": cfg["listen"],
        "port": cfg["port"],
        "config_json": cfg,
        "enabled": True,
    }


def plan_inbounds(inbounds, existing_by_tag, core_type: str = "xray"):
    created, updated, skipped, errors = [], [], [], []
    for item in inbounds:
        if not int(item.get("enable") or 0):
            skipped.append(f"id={item['id']} {item.get('remark')} 未启用")
            continue
        proto = (item.get("protocol") or "").lower()
        if core_type == "xray" and proto in XRAY_UNSUPPORTED:
            skipped.append(f"id={item['id']} {item.get('remark')} 协议 {proto} 需要 sing-box，xray 不支持，先跳过（迁移后按 sing-box 入站单独建）")
            continue
        if not item.get("port"):
            errors.append(f"id={item['id']} {item.get('remark')} 端口为 0，无法迁移")
            continue
        payload = build_inbound_config(item, core_type)
        if payload["tag"] in existing_by_tag:
            payload["_id"] = existing_by_tag[payload["tag"]]
            updated.append(payload)
        else:
            created.append(payload)
    return created, updated, skipped, errors


def canonical(rule) -> str:
    return json.dumps(rule, sort_keys=True, ensure_ascii=False, separators=(",", ":"))


def plan_routing(template, existing_section, tag_map):
    routing = template.get("routing") or {}
    rules = routing.get("rules") or []
    kept, skipped = [], []
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        if rule.get("outboundTag") == "api" or "api" in (rule.get("inboundTag") or []):
            skipped.append("API 路由由 CoreFusion 自己生成，跳过")
            continue
        out = rule.get("outboundTag")
        if out:
            mapped = tag_map.get(out)
            if not mapped:
                skipped.append(f"出口 {out} 未映射（用 --map {out}=<tag> 或 --warp-tag），跳过：{canonical(rule)[:120]}")
                continue
            rule = dict(rule)
            rule["outboundTag"] = mapped
        kept.append(rule)

    merged = list(existing_section.get("rules") or [])
    seen = {canonical(r) for r in merged}
    added = []
    for rule in kept:
        key = canonical(rule)
        if key in seen:
            continue
        seen.add(key)
        merged.append(rule)
        added.append(rule)

    section = dict(existing_section or {})
    section["rules"] = merged
    if routing.get("domainStrategy"):
        section["domainStrategy"] = routing["domainStrategy"]
    return section, added, skipped


def main() -> int:
    ap = argparse.ArgumentParser(description="xray-ui → CoreFusion：迁移入站和路由（不迁出口）")
    ap.add_argument("--db", required=True, help="x-ui 数据库路径（只读打开）")
    ap.add_argument("--panel", required=True, help="面板地址，如 https://127.0.0.1:10111")
    ap.add_argument("--token", required=True, help="管理员 access token")
    ap.add_argument("--server-id", type=int, default=1, help="目标节点 ID（默认 1）")
    ap.add_argument("--core-type", choices=["xray", "singbox"], default="xray", help="入站写入哪个核心（默认 xray）")
    ap.add_argument("--warp-tag", default="", help="x-ui wireguard 出口对应的 CoreFusion 出口 tag")
    ap.add_argument("--map", action="append", default=[], help="额外出口映射 old=new，可重复")
    ap.add_argument("--only", choices=["all", "inbounds", "routing"], default="all")
    ap.add_argument("--insecure", action="store_true", help="跳过 TLS 校验（自签证书）")
    ap.add_argument("--dry-run", action="store_true", help="只打印计划，不写面板")
    args = ap.parse_args()

    tag_map = {"direct": "direct", "blocked": "blocked"}
    for pair in args.map:
        if "=" not in pair:
            print(f"--map 格式应为 old=new，收到 {pair!r}", file=sys.stderr)
            return 2
        old, new = pair.split("=", 1)
        tag_map[old.strip()] = new.strip()
    if args.warp_tag:
        tag_map["wireguard"] = args.warp_tag

    inbounds = load_inbounds(args.db)
    template = load_template(args.db)

    print(f"x-ui 入站 {len(inbounds)} 条，模板路由规则 {len((template.get('routing') or {}).get('rules') or [])} 条")
    print(f"出口映射：{json.dumps(tag_map, ensure_ascii=False)}")

    rc = 0
    if args.only in ("all", "inbounds"):
        status, payload = api_call(args.panel, args.token, "GET", f"/api/v1/node-servers/{args.server_id}/inbound-configs", insecure=args.insecure)
        if status != 200:
            print(f"读取现有入站失败：{status} {payload}", file=sys.stderr)
            return 1
        existing = unwrap(payload) or []
        if isinstance(existing, dict):
            existing = existing.get("items") or []
        existing_by_tag = {cfg.get("tag"): cfg.get("id") for cfg in existing if cfg.get("tag")}

        created, updated, skipped, errors = plan_inbounds(inbounds, existing_by_tag, args.core_type)
        print(f"入站：新建 {len(created)}，更新 {len(updated)}，跳过 {len(skipped)}，错误 {len(errors)}")
        for item in created:
            print(f"  + {item['tag']:32s} {item['protocol']:12s} {item['listen'] or '0.0.0.0'}:{item['port']}")
        for item in updated:
            print(f"  ~ {item['tag']:32s} id={item['_id']} {item['protocol']:12s} {item['listen'] or '0.0.0.0'}:{item['port']}")
        for msg in skipped:
            print(f"  - 跳过 {msg}")
        for msg in errors:
            print(f"  ! 错误 {msg}")
            rc = 1

        if not args.dry_run:
            for item in created:
                body = {k: v for k, v in item.items() if not k.startswith("_")}
                st, resp = api_call(args.panel, args.token, "POST", f"/api/v1/node-servers/{args.server_id}/inbound-configs", body, insecure=args.insecure)
                if st != 200:
                    print(f"  ! 新建 {item['tag']} 失败：{st} {unwrap(resp)}", file=sys.stderr)
                    rc = 1
            for item in updated:
                body = {k: v for k, v in item.items() if not k.startswith("_")}
                st, resp = api_call(args.panel, args.token, "PUT", f"/api/v1/node-servers/{args.server_id}/inbound-configs/{item['_id']}", body, insecure=args.insecure)
                if st != 200:
                    print(f"  ! 更新 {item['tag']} 失败：{st} {unwrap(resp)}", file=sys.stderr)
                    rc = 1

    if args.only in ("all", "routing"):
        status, payload = api_call(args.panel, args.token, "GET", f"/api/v1/node-servers/{args.server_id}/protocols/xray/section/routing", insecure=args.insecure)
        if status != 200:
            print(f"读取 routing 段失败：{status} {payload}", file=sys.stderr)
            return 1
        section = unwrap(payload) or {}
        if isinstance(section, dict) and "config" in section and len(section) == 1:
            section = section["config"]
        new_section, added, skipped = plan_routing(template, section, tag_map)
        print(f"路由：新增 {len(added)} 条，跳过 {len(skipped)} 条，合并后共 {len(new_section.get('rules') or [])} 条")
        for rule in added:
            print("  + " + canonical(rule))
        for msg in skipped:
            print("  - 跳过 " + msg)

        if added and not args.dry_run:
            st, resp = api_call(args.panel, args.token, "PUT", f"/api/v1/node-servers/{args.server_id}/protocols/xray/section/routing", new_section, insecure=args.insecure)
            if st != 200:
                print(f"  ! 保存 routing 失败：{st} {unwrap(resp)}", file=sys.stderr)
                rc = 1

    if args.dry_run:
        print("dry-run：未写入任何配置")
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
