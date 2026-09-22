#!/usr/bin/env python3
"""核心配置体检：把 `data/runtime/` 下的核心配置逐个交给**真实核心**做 dry-run 校验。

为什么需要它：本项目踩过多次「配置生成看着没问题、但核心拒绝启动」的坑，而且**单测拦不住** ——
单测只能锁住"我以为是那样"，字段名/层级写错了照样通过：
  - hy2 出站缺 `tls`              → `FATAL initialize outbound: TLS required`
  - sing-box 的 ws 写成 `transport.host`（正确是 `headers.Host`）
                                  → `FATAL decode config: … unknown field "host"`
  - 同机双核心时 hy2 出站进了 xray 配置 → `FATAL … unknown config id: hysteria2`
  - transport 名 `raw`（xray 专有）进了 sing-box → `unknown transport type: raw`
以上四条都只有真实核心能发现。

做法：对每个 `<core>.jsonc` 用对应核心的 dry-run 跑一遍，并列出该配置里的**链式出站清单**
（便于一眼看出"某条链的出站跑到了另一个核心的配置里"）。

用法：
  python3 scripts/check_configs.py                     # 校验 data/runtime/ 下全部
  python3 scripts/check_configs.py --only xray         # 只校验 xray
  python3 scripts/check_configs.py --runtime-dir /tmp/rt --binaries-dir /tmp/bin
  python3 scripts/check_configs.py --quiet             # 只打印结论

退出码：有任何 FAIL → 1；否则 0（SKIP 不算失败，但一定带原因）。
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# 与 scripts/e2e/e2e_share_matrix.py 保持一致：二进制路径可用环境变量覆盖。
XRAY_BIN = os.environ.get("E2E_XRAY", f"{ROOT}/data/binaries/active/xray/xray-linux-amd64")
SBOX_BIN = os.environ.get("E2E_SINGBOX", f"{ROOT}/data/binaries/active/singbox/singbox-linux-amd64")

# xray 的 geosite/geoip 资源目录（`run -test` 需要，否则会因找不到 geosite 报错）。
XRAY_ASSET_CANDIDATES = [
    f"{ROOT}/data/binaries/active/corefusion/data/rules",
    f"{ROOT}/data/rules",
]

# 链式出站的协议矩阵（与 agent.ChainSupportedProtocols 同步）：
# xray 没有 hysteria2 出站 —— 出现即说明"按核心过滤"失效了。
XRAY_CHAIN_PROTOCOLS = {"vless", "vmess", "trojan", "shadowsocks"}
SINGBOX_CHAIN_PROTOCOLS = XRAY_CHAIN_PROTOCOLS | {"hysteria2", "hysteria"}


def first_existing(paths):
    for p in paths:
        if os.path.exists(p):
            return p
    return paths[0]


def load_config(path):
    """读 jsonc（允许 // 行注释）。解析失败返回 (None, 原因)。"""
    try:
        with open(path, encoding="utf-8") as fh:
            raw = fh.read()
    except OSError as exc:
        return None, f"读取失败：{exc}"
    lines = [ln for ln in raw.splitlines() if not ln.lstrip().startswith("//")]
    try:
        return json.loads("\n".join(lines)), None
    except json.JSONDecodeError as exc:
        return None, f"JSON 解析失败：{exc}"


def chain_outbounds(cfg):
    """列出配置里的链式出站：(tag, 协议/类型)。"""
    out = []
    for ob in (cfg or {}).get("outbounds") or []:
        if not isinstance(ob, dict):
            continue
        tag = str(ob.get("tag") or "")
        if tag.startswith("chain-"):
            out.append((tag, ob.get("protocol") or ob.get("type") or "?"))
    return out


def run(cmd, cwd, env=None):
    merged = dict(os.environ)
    if env:
        merged.update(env)
    proc = subprocess.run(cmd, cwd=cwd, env=merged, capture_output=True, text=True)
    return proc.returncode, (proc.stdout or "") + (proc.stderr or "")


def check_xray(bin_path, cfg_path, data_dir):
    if not os.path.exists(bin_path):
        return "SKIP", f"二进制不存在：{bin_path}", []
    # ⚠ `-c` 必须传**绝对路径** ✗：写死 "runtime/xray.jsonc" 会让 `--runtime-dir` 形同虚设 ——
    # 校验的其实是 data/runtime 下的那个文件，对副本体检会给出**假通过** ✗（本脚本第一版就踩了 ✓）。
    rc, out = run(
        [bin_path, "run", "-test", "-c", cfg_path],
        cwd=data_dir,
        env={"XRAY_LOCATION_ASSET": first_existing(XRAY_ASSET_CANDIDATES)},
    )
    ok = rc == 0 and "Configuration OK." in out
    detail = [ln for ln in out.splitlines() if ln.strip() and "deprecated" not in ln.lower()]
    return ("PASS" if ok else "FAIL"), (detail[-1] if detail else ""), detail


def check_singbox(bin_path, cfg_path, data_dir):
    if not os.path.exists(bin_path):
        return "SKIP", f"二进制不存在：{bin_path}", []
    rc, out = run([bin_path, "check", "-c", cfg_path], cwd=data_dir)
    ok = rc == 0
    detail = [ln for ln in out.splitlines() if ln.strip()]
    return ("PASS" if ok else "FAIL"), (detail[-1] if detail else ""), detail


# 文件 → (核心名, 校验函数, 该核心允许的链式出口协议)
TARGETS = {
    "xray.jsonc": ("xray", check_xray, XRAY_CHAIN_PROTOCOLS),
    "singbox.jsonc": ("singbox", check_singbox, SINGBOX_CHAIN_PROTOCOLS),
}

# 有配置但核心不支持 dry-run 的（按 e2e 约定：SKIP 必须带原因）
NO_DRYRUN = {
    "hysteria2.jsonc": "hysteria2 无配置校验子命令（只有 server 模式）",
    "tuic.jsonc": "tuic 无配置校验子命令（-i 只生成示例）",
}


def main():
    ap = argparse.ArgumentParser(description="把 data/runtime/ 下的核心配置交给真实核心做 dry-run 校验")
    ap.add_argument("--runtime-dir", default=f"{ROOT}/data/runtime", help="配置目录（默认 data/runtime）")
    ap.add_argument("--binaries-dir", default=None, help="覆盖核心二进制所在目录（默认取内置路径）")
    ap.add_argument("--data-dir", default=None,
                    help="核心的工作目录（配置里的证书/日志都是相对它写的）；"
                         "默认取 runtime-dir 的上级目录")
    ap.add_argument("--only", default=None, help="只校验指定核心（xray / singbox）")
    ap.add_argument("--quiet", action="store_true", help="只打印结论行")
    args = ap.parse_args()

    runtime_dir = os.path.abspath(args.runtime_dir)
    # 核心以 data/ 为工作目录（配置里的 certs/、logs/ 都是相对它写的）。
    # runtime-dir 默认就是 data/runtime，所以取上级即可；显式传 --data-dir 可覆盖。
    data_dir = os.path.abspath(args.data_dir) if args.data_dir else os.path.dirname(runtime_dir)
    if not os.path.isdir(runtime_dir):
        print(f"runtime 目录不存在：{runtime_dir}")
        return 1

    xray_bin = os.path.join(args.binaries_dir, "xray", "xray-linux-amd64") if args.binaries_dir else XRAY_BIN
    sbox_bin = os.path.join(args.binaries_dir, "singbox", "singbox-linux-amd64") if args.binaries_dir else SBOX_BIN

    results = []
    for fname, (core, fn, allowed) in TARGETS.items():
        if args.only and args.only.lower() != core:
            continue
        path = os.path.join(runtime_dir, fname)
        if not os.path.exists(path):
            results.append((fname, core, "SKIP", "配置文件不存在", []))
            continue

        cfg, perr = load_config(path)
        chain = chain_outbounds(cfg)
        core_chain = [(t, p) for t, p in chain if p in ("hysteria2", "hysteria")]
        bad = [(t, p) for t, p in chain if p not in allowed]

        bin_path = xray_bin if core == "xray" else sbox_bin
        status, detail, raw = fn(bin_path, path, data_dir)

        # 附加静态体检：链式出站是否"跑错了核心"（真机踩过：hy2 出站进了 xray 配置 → 核心起不来）
        if bad and status == "PASS":
            status, detail = "FAIL", f"链式出站协议不属于该核心：{bad}（允许：{sorted(allowed)}）"

        results.append((fname, core, status, detail, chain))
        if not args.quiet:
            print(f"[{status}] {fname}（{core}）")
            if chain:
                print(f"        链式出站：{', '.join(f'{t}={p}' for t, p in chain)}")
            elif cfg is not None:
                print("        链式出站：无")
            if status == "FAIL":
                for ln in raw[-6:]:
                    print(f"        {ln}")
            elif detail:
                print(f"        {detail}")

    for fname, reason in NO_DRYRUN.items():
        if args.only:
            break
        if os.path.exists(os.path.join(runtime_dir, fname)):
            results.append((fname, fname.split(".")[0], "SKIP", reason, []))

    print("\n—— 结论 ——")
    for fname, core, status, detail, _ in results:
        extra = f"（{detail}）" if status in ("FAIL", "SKIP") and detail else ""
        print(f"  {status:4} {fname}{extra}")

    failed = [r for r in results if r[2] == "FAIL"]
    passed = [r for r in results if r[2] == "PASS"]
    skipped = [r for r in results if r[2] == "SKIP"]
    print(f"  通过 {len(passed)} / 失败 {len(failed)} / 跳过 {len(skipped)}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
