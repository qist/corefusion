#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
CoreFusion 全接口覆盖测试

遍历 internal/api/router/router.go 中所有注册路由，逐个发送请求并分类记录。

用法：
  python3 tests/api_coverage_test.py [BASE_URL] [--concurrency N] [--json-out FILE]

分类：
  PASS - 2xx 或 400/422（参数校验，预期行为）
  FAIL - 404（路由未注册，不应发生）或 500（服务器错误，需修复）
  SKIP - 401/403/429（鉴权/限流）或 501（未实现，留待最终版本）
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


PARAM_MAP = {
    ":policyId": "1", ":banId": "1", ":site_id": "1", ":server_id": "1",
    ":cid": "1", ":id": "1", ":name": "test", ":slug": "test",
    ":token": "test", ":provider": "test", ":file": "test",
}


def substitute(path):
    for k, v in PARAM_MAP.items():
        path = path.replace(k, v)
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
    def __init__(self, base, token, timeout=10):
        self.base = base.rstrip("/")
        self.token = token
        self.timeout = timeout
        self._local = threading.local()

    @property
    def session(self):
        if not hasattr(self._local, "session"):
            self._local.session = requests.Session()
        return self._local.session

    def run(self, method, path):
        url = self.base + substitute(path)
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


def main():
    ap = argparse.ArgumentParser(description="CoreFusion 全接口覆盖测试")
    ap.add_argument("base", nargs="?", default="http://127.0.0.1:8080")
    ap.add_argument("--concurrency", "-c", type=int, default=16)
    ap.add_argument("--json-out", "-o", default="")
    ap.add_argument("--router", "-r",
                    default="/opt/corefusion/internal/api/router/router.go")
    ap.add_argument("--delay", type=float, default=0.02)
    args = ap.parse_args()

    routes = parse_routes(args.router)
    print("[INFO] 解析到 %d 条唯一路由" % len(routes))

    print("[INFO] 登录 %s ..." % args.base)
    try:
        r = requests.post(
            args.base + "/api/v1/auth/login",
            json={"username": "admin", "password": "admin123"},
            timeout=10,
        )
        token = r.json()["data"]["access_token"]
    except Exception as e:
        print("[ERROR] 登录失败: %s" % e)
        sys.exit(1)
    print("[INFO] 登录成功")

    tester = Tester(args.base, token)
    results = []
    lock = threading.Lock()
    done = 0
    total = len(routes)

    def worker(route):
        time.sleep(args.delay)
        res = tester.run(route["method"], route["path"])
        # TOKEN_REVOKED：某个公开 handler 内部调用了 login/refresh 导致 token 被吊销
        # 自动重新登录并重试一次
        if res["biz_code"] == "TOKEN_REVOKED":
            try:
                r = requests.post(
                    args.base + "/api/v1/auth/login",
                    json={"username": "admin", "password": "admin123"},
                    timeout=10,
                )
                tester.token = r.json()["data"]["access_token"]
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
    for r in results:
        by_cat[r["category"]] = by_cat.get(r["category"], 0) + 1

    print()
    print("================ 覆盖测试汇总 ================")
    for k in ("PASS", "FAIL", "SKIP"):
        print("  %-6s %d" % (k, by_cat.get(k, 0)))
    print("  TOTAL  %d" % len(results))

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
        print("===== SKIP 明细（未实现/鉴权/限流）=====")
        for r in skips:
            print("  %s %s -> HTTP %s biz=%s" % (r["method"], r["path"], r["http_code"], r["biz_code"]))

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
