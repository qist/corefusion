#!/usr/bin/env bash
# 路径拼接静态检查（安全审计 v6 §6 的统一建议）。
#
# 目的：路径穿越修复不能只靠"新增接口时记得调 safepath"。这里把
# 「filepath.Join 的参数里**直接**出现请求输入」的可疑点找出来，
# 除非同一处上下文（前 5 行）已经有 safepath. 校验或显式豁免注释。
#
# 覆盖不到的写法（已知局限）：
#   name := c.Param("name")      // 变量中转
#   filepath.Join(dir, name)     // 这里匹配不到 → 仍需 code review / gosec G102
#
# 豁免方式（确属安全的固定输入）：
#   // safepath-check: ok: <原因>
#
# 退出码：发现未校验的可疑点 → 1（CI 直接失败）。
set -euo pipefail
cd "$(dirname "$0")/.."

pattern='filepath\.Join\([^)]*(c\.(Param|Query|PostForm|GetHeader)|req\.[A-Za-z_][A-Za-z0-9_]*)'
fail=0

while IFS= read -r hit; do
  [ -z "$hit" ] && continue
  file="${hit%%:*}"
  rest="${hit#*:}"
  line="${rest%%:*}"
  text="${rest#*:}"

  # 纯注释（例如 safepath 包里的用法示例）不参与检查
  case "$text" in
    //*|"	//"*|" "*"//"*) continue ;;
  esac

  start=$(( line > 5 ? line - 5 : 1 ))
  ctx="$(sed -n "${start},${line}p" "$file")"
  if printf '%s\n' "$ctx" | grep -qE 'safepath\.|safepath-check: ok'; then
    continue
  fi

  echo "✗ ${file}:${line}: ${text}"
  fail=1
done < <(grep -rnE --include='*.go' --exclude='*_test.go' "$pattern" internal cmd pkg || true)

if [ "$fail" -ne 0 ]; then
  cat <<'EOF'

存在未校验的 filepath.Join(用户输入)。
修法：改用 safepath.ResolveUnder(...) 或 safepath.CheckName(...)（见 internal/util/safepath）。
确属安全的固定输入，可在调用点前加注释：// safepath-check: ok: <原因>
EOF
  exit 1
fi

echo "✓ safepath check passed"
