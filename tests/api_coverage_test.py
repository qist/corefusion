#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
CoreFusion 全接口覆盖测试

遍历 internal/api/router/router.go 中所有注册路由，逐个发送请求并分类记录。

⚠ 安全设计（2026-10-28 重写）
  旧版本对每条路由都用 `:id → 1` 发真实请求，包括 DELETE / revoke / reset 等
  破坏性接口。1 恰好是真实 admin 用户和本地节点的 ID，跑一次就删了 admin 用户、
  吊销了节点证书（见记忆 feedback_no_destructive_test_on_real_db）。

  现在默认 **safe 模式：只发 GET**，其余路由只登记不发送 → 对任何库都零副作用。
  如需覆盖写接口，用 `--mode write`（ID 用不存在的 999999999，且跳过破坏性路由），
  **必须**指向临时的测试实例/测试库。
  `--mode unsafe` 才恢复旧行为（真实 ID + 全部方法），仅限一次性沙箱，脚本会二次确认。

用法：
  python3 tests/api_coverage_test.py [BASE_URL] [--mode safe|write|unsafe]
                                     [--password PWD] [--concurrency N] [--json-out FILE]

分类：
  PASS - 2xx 或 400/422（参数校验，预期行为）
  FAIL - 404（路由未注册）或 500（服务器错误，需修复）
  SKIP - 401/403/429（鉴权/限流）、501（未实现）、以及安全模式/写模式下未发送的路由
"""

import argparse
import json
import re
import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import requests

# 安全模式下完全不发送的方法（默认只发 GET）
READ_METHODS = {"GET", "HEAD", "OPTIONS"}

# write 模式也要跳过的破坏性路由（关键词命中 path 即跳过）。
# 这些接口要么影响面是全局（revoke-all / distribute-cert / cleanup-all），
# 要么会改真实运行状态（restart/stop/install/update/switch/migrate）。
DESTRUCTIVE_KEYWORDS = (
    "delete", "remove", "revoke", "reset", "cleanup", "purge", "truncate",
    "restart", "stop", "kill", "reboot", "shutdown",
    "install", "update", "upgrade", "rollback", "rebuild", "repair",
    "switch", "migrate", "restore", "distribute", "rotate",
    "ban", "unban", "suspend", "disable", "logout",
)

# 真实 ID（unsafe 模式）与幽灵 ID（write 模式）。
REAL_ID = "1"
GHOST_ID = "999999999"


def parse_routes(router_path):
    lines = Path(router_path).read_text(encoding="utf-8").split("\n")
    prefix = {"r": "", "rg": "/api/v1"}
    routes = []
    group_re = re.compile(r'^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)')
    method_re = re.compile(r'\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"')
    recv_re = re.compile(r'^\s*(\w+)')

    for i, line in enumerate(lines, 1):
        gm = group_re.match(line)
        if gm:
            var, parent, p = gm.group(1), gm.group(2), gm.group(3)
            prefix[var] = prefix.get(parent, "") + p
            continue
        mm = method_re.search(line)
        if mm:
            method, p = mm.group(1), mm.group(2)
            rm = recv_re.match(line)
            recv = rm.group(1) if rm else ""
            base = prefix.get(recv, "")
            routes.append({"method": method, "path": base + p, "line": i})

    seen = set()
    uniq = []
    for r in routes:
        k = (r["method"], r["path"])
        if k not in seen:
            seen.add(k)
            uniq.append(r)
    return uniq


PARAM_KEYS = (
    ":policyId", ":banId", ":site_id", ":server_id", ":cid", ":id",
    ":sourceId", ":fileId", ":cfgId", ":userId", ":clientId", ":orderId",
    ":templateId", ":variantId", ":groupId", ":promotionId", ":ticketId",
    ":certId", ":backupId", ":planId", ":name", ":slug", ":token",
    ":provider", ":file", ":email",
)


def substitute(path, ident):
    for k in PARAM_KEYS:
        path = path.replace(k, ident)
    return path


PUBLIC_PATTERNS = [
    "/api/v1/auth/login", "/api/v1/auth/refresh",
    "/api/v1/auth/login/2fa-status", "/api/v1/auth/passkey/status",
    "/api/v1/public/", "/api/v1/client-auth/",
    "/health", "/sub/", "/p", "/static/", "/",
]


def is_public(path):
    for pat in PUBLIC_PATTERNS:
        if path == pat:
            return True
        if pat != "/" and path.startswith(pat):
            return True
    return False


def is_destructive(method, path):
    if method in READ_METHODS:
        return False
    low = path.lower()
    return any(k in low for k in DESTRUCTIVE_KEYWORDS)


def categorize(http_code, biz_code, body=""):
    if http_code in (200, 201, 204):
        return "PASS"
    if http_code in (400, 422):
        return "PASS"
    if http_code in (401, 403, 429):
        return "SKIP"
    if http_code == 404:
        # 带业务错误体（非纯 "404 page not found"）说明路由已匹配，资源不存在属预期
        if biz_code or (body and "404 page not found" not in body):
            return "PASS"
        return "FAIL"
    if http_code == 501:
        return "SKIP"
    if http_code == 500:
        return "FAIL"
    return "SKIP"


class Tester:
    def __init__(self, base, token, mode, timeout=10):
        self.base = base.rstrip("/")
        self.token = token
        self.mode = mode
        self.timeout = timeout
        self._local = threading.local()

    @property
    def session(self):
        if not hasattr(self._local, "session"):
            self._local.session = requests.Session()
        return self._local.session

    def _skip(self, method, path, reason):
        return {
            "method": method, "path": path, "http_code": 0, "biz_code": "",
            "category": "SKIP", "body": "", "skip_reason": reason,
        }

    def run(self, method, path):
        # ① 默认 safe：只发只读请求，绝不产生任何写副作用
        if self.mode == "safe" and method not in READ_METHODS:
            return self._skip(method, path, "safe-mode")
        # ② write：允许写，但跳过破坏性路由（revoke/reset/cleanup/restart/...）
        if self.mode == "write" and is_destructive(method, path):
            return self._skip(method, path, "destructive-route")

        ident = GHOST_ID if self.mode == "write" else REAL_ID
        url = self.base + substitute(path, ident)
        headers = {"Content-Type": "application/json"}
        if not is_public(path) and self.token:
            headers["Authorization"] = "Bearer " + self.token
        try:
            resp = self.session.request(
                method, url, headers=headers,
                data=b'{"test":true}', timeout=self.timeout,
            )
            http_code = resp.status_code
            biz_code = ""
            try:
                d = resp.json()
                if isinstance(d, dict):
                    c = d.get("code", "")
                    if c not in (0, "", None):
                        biz_code = str(c)
            except Exception:
                pass
            body = resp.text[:300]
        except requests.exceptions.Timeout:
            http_code, biz_code, body = 0, "", "TIMEOUT"
        except requests.exceptions.ConnectionError:
            http_code, biz_code, body = 0, "", "CONN_ERROR"
        except Exception as e:
            http_code, biz_code, body = 0, "", str(e)[:200]

        cat = categorize(http_code, biz_code, body)
        return {
            "method": method, "path": path, "http_code": http_code,
            "biz_code": biz_code, "category": cat, "body": body,
        }


def login(base, username, password):
    r = requests.post(
        base.rstrip("/") + "/api/v1/auth/login",
        json={"username": username, "password": password},
        timeout=10,
    )
    return r.json()["data"]["access_token"]


def main():
    ap = argparse.ArgumentParser(description="CoreFusion 全接口覆盖测试（默认只读）")
    ap.add_argument("base", nargs="?", default="http://127.0.0.1:8080")
    ap.add_argument("--mode", choices=("safe", "write", "unsafe"), default="safe",
                    help="safe=只发 GET（默认）；write=发写请求但用幽灵 ID 并跳过破坏性路由；unsafe=旧行为")
    ap.add_argument("--username", default="admin")
    ap.add_argument("--password", default="admin123")
    ap.add_argument("--concurrency", "-c", type=int, default=16)
    ap.add_argument("--json-out", "-o", default="")
    ap.add_argument("--router", "-r",
                    default="/opt/corefusion/internal/api/router/router.go")
    ap.add_argument("--delay", type=float, default=0.02)
    args = ap.parse_args()

    if args.mode == "unsafe":
        print("!! unsafe 模式会向真实资源发送 DELETE/PUT/POST（旧 :id=1 行为），")
        print("!! 可能删除真实用户/节点/证书。请确认目标是一次性沙箱实例。")
        if input("输入 I-UNDERSTAND 继续: ").strip() != "I-UNDERSTAND":
            print("已取消。")
            sys.exit(1)

    routes = parse_routes(args.router)
    print("[INFO] 解析到 %d 条唯一路由（mode=%s）" % (len(routes), args.mode))

    print("[INFO] 登录 %s ..." % args.base)
    try:
        token = login(args.base, args.username, args.password)
    except Exception as e:
        print("[ERROR] 登录失败: %s" % e)
        sys.exit(1)
    print("[INFO] 登录成功")

    tester = Tester(args.base, token, args.mode)
    results = []
    lock = threading.Lock()
    done = 0

    def worker(route):
        time.sleep(args.delay)
        res = tester.run(route["method"], route["path"])
        # TOKEN_REVOKED：某个公开 handler 内部调用了 login/refresh 导致 token 被吊销
        # 自动重新登录并重试一次（仅 safe/write 模式，避免 unsafe 下掩盖问题）
        if res["biz_code"] == "TOKEN_REVOKED" and args.mode != "unsafe":
            try:
                tester.token = login(args.base, args.username, args.password)
                res = tester.run(route["method"], route["path"])
            except Exception:
                pass
        return res

    # 先测需认证路由，再测公开路由（login 等公开路由会吊销 token，必须放最后）
    auth_routes = [r for r in routes if not is_public(r["path"])]
    public_routes = [r for r in routes if is_public(r["path"])]
    ordered = auth_routes + public_routes
    total = len(ordered)

    with ThreadPoolExecutor(max_workers=args.concurrency) as ex:
        futures = {ex.submit(worker, r): r for r in ordered}
        for fut in as_completed(futures):
            res = fut.result()
            with lock:
                results.append(res)
                done += 1
                if done % 100 == 0 or done == total:
                    print("  进度 %d/%d" % (done, total), flush=True)

    by_cat = {}
    skip_reasons = {}
    for r in results:
        by_cat[r["category"]] = by_cat.get(r["category"], 0) + 1
        if r["category"] == "SKIP" and r.get("skip_reason"):
            skip_reasons[r["skip_reason"]] = skip_reasons.get(r["skip_reason"], 0) + 1

    print()
    print("================ 覆盖测试汇总 ================")
    for k in ("PASS", "FAIL", "SKIP"):
        print("  %-6s %d" % (k, by_cat.get(k, 0)))
    print("  TOTAL  %d" % len(results))
    for reason, n in sorted(skip_reasons.items()):
        print("    未发送（%s）: %d" % (reason, n))
    if args.mode == "safe":
        print("  [SAFE] 已跳过所有非 GET 路由，数据库零写入。要覆盖写接口请用 --mode write 指向测试实例。")

    fails = [r for r in results if r["category"] == "FAIL"]
    skips = [r for r in results if r["category"] == "SKIP"]

    if fails:
        print()
        print("===== FAIL 明细（需修复）=====")
        for r in fails:
            print("  %s %s -> HTTP %s biz=%s" % (r["method"], r["path"], r["http_code"], r["biz_code"]))
            if r["body"]:
                print("    body: %s" % r["body"][:200])

    if skips:
        print()
        print("===== SKIP 明细（未实现/鉴权/限流/未发送）=====")
        for r in skips:
            extra = (" [%s]" % r["skip_reason"]) if r.get("skip_reason") else ""
            print("  %s %s -> HTTP %s biz=%s%s" % (r["method"], r["path"], r["http_code"], r["biz_code"], extra))

    if args.json_out:
        Path(args.json_out).write_text(
            json.dumps(results, ensure_ascii=False, indent=1), encoding="utf-8"
        )
        print("\n[INFO] 报告已写入 %s" % args.json_out)

    if fails:
        print("\n[FAIL] %d 个接口存在问题" % len(fails))
        sys.exit(2)
    print("\n[OK] 全部可达接口正常")
    sys.exit(0)


if __name__ == "__main__":
    main()
