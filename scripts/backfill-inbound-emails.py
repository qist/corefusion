#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""给已有入站补齐缺失的 client email（xray 流量统计按 email 归户）。

背景：xray 的 stats user 名就是 `client.email`。从 xray-ui 迁移过来的客户端
通常没有 email，于是这些用户的流量既不出现在面板、也不会落到
inbound_user_traffic。服务端保存入站时已经会自动补（见 ensureClientEmails），
但**迁移时已经落库的历史数据**需要用本脚本回填一次。

默认只处理 xray 入站；email 格式与迁移脚本、服务端保持一致：
    u<8hex>@cf.local_{tag后缀}      例：u1a2b3c4d@cf.local_vless20296

用法：
  # 1. 预览（不写库）
  python3 scripts/backfill-inbound-emails.py --panel http://127.0.0.1:8080 \
      --user admin --password 'xxx' --dry-run

  # 2. 实际回填
  python3 scripts/backfill-inbound-emails.py --panel http://127.0.0.1:8080 \
      --user admin --password 'xxx'

  # 指定节点（默认 1 = 面板本机）
  python3 scripts/backfill-inbound-emails.py ... --node 2

依赖：Python 3.8+，仅标准库。
"""

import argparse
import json
import ssl
import sys
import urllib.error
import urllib.request


class Panel:
    def __init__(self, base, token=None, insecure=False):
        self.base = base.rstrip("/")
        self.token = token
        self.ctx = ssl._create_unverified_context() if insecure else None

    def call(self, method, path, body=None):
        data = json.dumps(body).encode("utf-8") if body is not None else None
        req = urllib.request.Request(self.base + path, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(req, timeout=60, context=self.ctx) as resp:
                raw = resp.read().decode("utf-8")
        except urllib.error.HTTPError as e:
            detail = e.read().decode("utf-8", "replace")
            raise RuntimeError("%s %s -> HTTP %s: %s" % (method, path, e.code, detail[:400]))
        if not raw.strip():
            return None
        return json.loads(raw).get("data")

    def login(self, username, password):
        data = self.call("POST", "/api/v1/auth/login",
                         {"username": username, "password": password})
        self.token = data["access_token"]
        return data


def parse_config(cfg):
    """config_json 可能是 dict 或 JSON 字符串，统一成 dict。"""
    if isinstance(cfg, dict):
        return cfg
    if isinstance(cfg, str):
        try:
            v = json.loads(cfg)
            return v if isinstance(v, dict) else None
        except Exception:
            return None
    return None


def missing_email_count(cfg):
    """返回该入站里缺 email 的客户端数量（只匹配服务端会处理的结构）。"""
    settings = cfg.get("settings")
    if not isinstance(settings, dict):
        return 0
    n = 0
    for key in ("clients", "users"):
        items = settings.get(key)
        if not isinstance(items, list):
            continue
        for it in items:
            if not isinstance(it, dict):
                continue
            if not (it.get("email") or "").strip():
                n += 1
    return n


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--panel", required=True, help="面板地址，如 http://127.0.0.1:8080")
    ap.add_argument("--token", help="已有 access token（优先于账号密码）")
    ap.add_argument("--user", help="管理员用户名")
    ap.add_argument("--password", help="管理员密码")
    ap.add_argument("--node", type=int, default=1, help="节点 server_id（默认 1=本机）")
    ap.add_argument("--core", default="xray", choices=["xray"], help="核心类型")
    ap.add_argument("--dry-run", action="store_true", help="只预览，不写库")
    ap.add_argument("--insecure", action="store_true", help="跳过 TLS 证书校验（自签证书面板）")
    args = ap.parse_args()

    p = Panel(args.panel, args.token, insecure=args.insecure)
    if not p.token:
        if not args.user or not args.password:
            ap.error("需要 --token，或同时提供 --user/--password")
        p.login(args.user, args.password)

    configs = p.call("GET", "/api/v1/node-servers/%d/inbound-configs?core_type=%s"
                     % (args.node, args.core)) or []
    if isinstance(configs, dict):
        configs = configs.get("items") or []
    print("读取到 %d 个 %s 入站（node=%d）\n" % (len(configs), args.core, args.node))

    changed = 0
    clients = 0
    for it in configs:
        cfg = parse_config(it.get("config_json"))
        if cfg is None:
            print("  [skip] %s: config_json 不是对象" % it.get("tag"))
            continue
        n = missing_email_count(cfg)
        if n == 0:
            continue
        tag = it.get("tag") or it.get("remark") or it.get("id")
        print("  [need] %s (id=%s): %d 个客户端缺 email" % (tag, it.get("id"), n))
        if args.dry_run:
            changed += 1
            clients += n
            continue
        # 只回写 config_json；其它字段缺失时服务端保留原值（Update 有防呆）。
        # 服务端 ensureClientEmails 会按凭据派生 email 并拼 tag 后缀。
        p.call("PUT", "/api/v1/node-servers/%d/inbound-configs/%s" % (args.node, it.get("id")),
               {"config_json": cfg, "enabled": bool(it.get("enabled", True))})
        changed += 1
        clients += n
        print("    -> 已回填")

    print("\n%s：%d 个入站、%d 个客户端%s" % (
        "预览" if args.dry_run else "完成",
        changed, clients,
        "（未写库）" if args.dry_run else "已写库（服务端会自动同步配置到节点）",
    ))
    return 0


if __name__ == "__main__":
    sys.exit(main())
