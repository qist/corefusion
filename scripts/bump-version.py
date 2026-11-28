#!/usr/bin/env python3
"""一处改版本，处处同步（发版时再也不用手动改示例里的版本号）。

用法：
    python3 scripts/bump-version.py v0.14.7
    make bump-version VERSION=v0.14.7

会同步以下位置的 CoreFusion 版本号：
  1. internal/constants/version          —— 编译进二进制的权威版本
  2. scripts/install-i18n/*.json         —— 安装脚本多语言文案的源文件
  3. scripts/install.sh                  —— 顶部注释示例 + 由 2 重新生成的 i18n 块
  4. docs/USER-GUIDE*.md                 —— 安装示例、参数表、文末「适用版本」

只替换「当前版本」所在的行，历史版本说明（如 v0.14.4 起xxx、
v0.13.0-rc.1 是预发布版）不会被误改。
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
VERSION_FILE = ROOT / "internal" / "constants" / "version"
I18N_DIR = ROOT / "scripts" / "install-i18n"
INSTALL_SH = ROOT / "scripts" / "install.sh"
DOCS_DIR = ROOT / "docs"

VER_RE = re.compile(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?")
# USER-GUIDE 里只有这三类行携带「当前版本」，其余 vX.Y.Z 是历史说明，不能动。
GUIDE_MARKERS = (
    "install.sh --version",
    "install.sh --source",
    "--version <TAG>",
    "CoreFusion v",
)


def die(msg: str) -> None:
    print(f"bump-version: {msg}", file=sys.stderr)
    raise SystemExit(1)


def rewrite(path: Path, text: str, old: str) -> None:
    if text != path.read_text(encoding="utf-8"):
        path.write_text(text, encoding="utf-8")
        print(f"  updated {path.relative_to(ROOT)}")


def main() -> int:
    if len(sys.argv) != 2:
        die("usage: python3 scripts/bump-version.py vX.Y.Z")
    new = sys.argv[1].strip()
    if not VER_RE.fullmatch(new):
        die(f"invalid version: {new!r} (expected vX.Y.Z or vX.Y.Z-suffix)")

    old = VERSION_FILE.read_text(encoding="utf-8").strip()
    print(f"bump-version: {old} -> {new}")

    # 1) 权威版本文件
    rewrite(VERSION_FILE, new + "\n", old)

    # 2) 安装脚本文案源文件：这些文件里出现的 CoreFusion 版本号就是要改的示例
    for path in sorted(I18N_DIR.glob("*.json")):
        text = path.read_text(encoding="utf-8")
        rewrite(path, VER_RE.sub(new, text), old)

    # 3) 安装脚本顶部注释示例（i18n 块稍后由生成器覆盖）
    text = INSTALL_SH.read_text(encoding="utf-8")
    begin = text.index("# >>> INSTALL_I18N_BEGIN")
    end = text.index("# <<< INSTALL_I18N_END")
    head = VER_RE.sub(new, text[:begin])
    tail = VER_RE.sub(new, text[end:])
    rewrite(INSTALL_SH, head + text[begin:end] + tail, old)

    subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "gen-install-i18n.py")],
        check=True,
    )

    # 4) 操作手册（中文源 + 各语言译文），只改带「当前版本」标记的行
    guides = [DOCS_DIR / "USER-GUIDE.md", *sorted(DOCS_DIR.glob("USER-GUIDE.*.md"))]
    for path in guides:
        if not path.exists():
            continue
        changed = False
        out = []
        for line in path.read_text(encoding="utf-8").splitlines(keepends=True):
            if any(m in line for m in GUIDE_MARKERS):
                updated = VER_RE.sub(new, line)
                changed |= updated != line
                line = updated
            out.append(line)
        if changed:
            path.write_text("".join(out), encoding="utf-8")
            print(f"  updated {path.relative_to(ROOT)}")

    print(f"bump-version: done ({new})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
