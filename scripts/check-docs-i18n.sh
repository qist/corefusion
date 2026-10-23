#!/usr/bin/env bash
# 文档多语言校验：命名、元数据、语言导航、索引一致性。
# 用法：bash scripts/check-docs-i18n.sh
set -uo pipefail

cd "$(dirname "$0")/.." || exit 2

LOCALES=(en zh-TW ru ja ko de fr es pt tr id th vi ar fa)
errors=0
checked=0

fail() {
	printf '✗ %s\n' "$*" >&2
	errors=$((errors + 1))
}

is_locale() {
	local l=$1 x
	for x in "${LOCALES[@]}"; do
		[ "$x" = "$l" ] && return 0
	done
	return 1
}

has_lang_bar() {
	local file=$1
	head -n 12 "$file" | grep -qE '^>.*\]\([^)]*\.md\)'
}

mapfile -t files < <(find . -type f -name '*.md' \
	-not -path './.git/*' -not -path '*/node_modules/*' -not -path './data/*' \
	-not -path './dist/*' -not -path './.codebuddy/*' -not -path './.catpaw/*' | sort)

for f in "${files[@]}"; do
	dir=$(dirname "$f")
	base=$(basename "$f")
	checked=$((checked + 1))

	if [[ $base =~ ^(.+)\.([A-Za-z-]+)\.md$ ]]; then
		src_name="${BASH_REMATCH[1]}"
		loc="${BASH_REMATCH[2]}"

		if [ "$loc" = "zh-CN" ]; then
			fail "$f: 简体中文是源语言，请命名为 ${src_name}.md"
			continue
		fi
		if ! is_locale "$loc"; then
			fail "$f: 不支持的语言后缀 .$loc"
			continue
		fi

		src="$dir/$src_name.md"
		if [ ! -f "$src" ]; then
			fail "$f: 找不到源文件 $src"
			continue
		fi

		first=$(head -n 1 "$f")
		if [[ $first =~ ^\<\!--\ source:\ ([^|]+)\ \|\ locale:\ ([A-Za-z-]+)\ \|\ updated:\ [0-9]{4}-[0-9]{2}-[0-9]{2}\ --\>$ ]]; then
			meta_src=$(echo "${BASH_REMATCH[1]}" | xargs)
			meta_loc="${BASH_REMATCH[2]}"
			[ "$meta_src" = "$src_name.md" ] || fail "$f: 元数据 source=$meta_src，应为 $src_name.md"
			[ "$meta_loc" = "$loc" ] || fail "$f: 元数据 locale=$meta_loc，与文件名 .$loc 不一致"
		else
			fail "$f: 首行缺少元数据（<!-- source: NAME.md | locale: LOCALE | updated: YYYY-MM-DD -->）"
		fi

		has_lang_bar "$f" || fail "$f: 缺少语言导航（H1 后 12 行内的引用块链接）"
		has_lang_bar "$src" || fail "$src: 已有译文，但源文件缺少语言导航"
	else
		# 源文件：若存在译文，必须有导航
		for loc in "${LOCALES[@]}"; do
			if [ -f "$dir/${base%.md}.$loc.md" ]; then
				has_lang_bar "$f" || fail "$f: 已有 .$loc 译文，但缺少语言导航"
				break
			fi
		done
	fi
done

# 索引中登记的文档必须存在
if [ -f docs/README.md ]; then
	while IFS= read -r name; do
		name=$(echo "$name" | xargs)
		case "$name" in
		'*'|'') continue ;;
		esac
		if [[ $name == *'*'* ]]; then
			continue
		fi
		if [ ! -f "docs/$name" ] && [ ! -f "$name" ]; then
			fail "docs/README.md: 登记的文档不存在：$name"
		fi
	done < <(grep -oE '^\| [A-Za-z0-9._-]+\.md ' docs/README.md | sed 's/^| //; s/ $//')
fi

echo "已检查 $checked 个 Markdown 文件，错误 $errors 个。"
if [ "$errors" -gt 0 ]; then
	exit 1
fi
echo "✅ 文档多语言检查通过"
