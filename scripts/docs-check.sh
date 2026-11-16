#!/usr/bin/env bash
# docs-check.sh — ворота документации: ссылки, личные пути, сироты, паритет
# языков, якорные факты. Копируется в scripts/ репозитория как есть.
#
#   bash scripts/docs-check.sh            # проверить
#   DOCS=docs EN=docs/en RU=docs/ru bash scripts/docs-check.sh
#
# Данные, а не код, живут в репозитории:
#   $DOCS/anchors.txt    must|regex|file   — строка обязана найтись в файле
#                        mustnot|regex|file — строка не должна находиться
#   $DOCS/forbidden.txt  по одному ERE на строку — запрещённые подстроки
#                        (по умолчанию: путь агентской песочницы и домашний
#                        каталог текущего пользователя; внутренние хосты проекта
#                        перечисляются в самом файле)
set -uo pipefail
cd "$(dirname "$0")/.."

DOCS=${DOCS:-docs}
EN=${EN:-$DOCS/en}
RU=${RU:-$DOCS/ru}
fail=0
say() { printf '%s\n' "$*"; }
bad() { say "FAIL $*"; fail=1; }

# 1. Относительные ссылки на .md должны существовать.
broken=$(mktemp)
while IFS= read -r f; do
  grep -oE '\]\([^)#]+\.md[^)]*\)' "$f" 2>/dev/null | sed -E 's/^\]\(([^)#]+).*/\1/' |
    while IFS= read -r l; do
      case "$l" in http*|mailto*) continue ;; esac
      [ -f "$(dirname "$f")/$l" ] || echo "$f -> $l"
    done
done < <(find . -name '*.md' -not -path './.git/*' -not -path '*/node_modules/*' \
           -not -path './.tmp/*' -not -path './vendor/*') >>"$broken"
if [ -s "$broken" ]; then bad "broken links:"; sed 's/^/  /' "$broken" | head -20
else say "ok   links resolve"; fi
rm -f "$broken"

# 2. Ничего, чего нет у читателя.
forbidden=$(mktemp)
if [ -f "$DOCS/forbidden.txt" ]; then cp "$DOCS/forbidden.txt" "$forbidden"
else printf '%s\n' '/tmp/opencode' "/home/$USER/" >"$forbidden"; fi
hits=$(find . -name '*.md' -not -path './.git/*' -not -path '*/node_modules/*' \
        -not -path './.tmp/*' -not -path './vendor/*' -not -path "./$DOCS/history/*" \
        -print0 | xargs -0 grep -nE -f "$forbidden" 2>/dev/null)
if [ -n "$hits" ]; then bad "paths the reader cannot reach:"; printf '%s\n' "$hits" | head -10
else say "ok   no personal paths"; fi
rm -f "$forbidden"

# 3. Ни одной страницы-сироты (кроме индекса и архива).
orphans=""
while IFS= read -r f; do
  case "$f" in "$DOCS/index.md"|"$DOCS"/*/index.md|"$DOCS/history/"*) continue ;; esac
  name=$(basename "$f")
  refs=$(grep -rlF --include='*.md' -- "$name" . 2>/dev/null | grep -vxF "./$f" | grep -v '^./.tmp/' | wc -l)
  [ "$refs" -eq 0 ] && orphans="$orphans $f"
done < <(find "$DOCS" -name '*.md' -not -path "$DOCS/history/*" | sort)
if [ -n "$orphans" ]; then bad "orphan pages (no incoming link):$orphans"
else say "ok   no orphans"; fi

# 4. Языковой паритет: либо каталоги en/ и ru/, либо файлы-соседи *.ru.md.
#    AGENTS.md, CHANGELOG.md и docs/history/** зеркал не имеют — см. AGENTS.md, «Documentation rules».
if [ -d "$EN" ] && [ -d "$RU" ]; then
  diff=$(diff <(cd "$EN" && find . -name '*.md' | sort) <(cd "$RU" && find . -name '*.md' | sort))
  if [ -n "$diff" ]; then bad "language directories differ:"; printf '%s\n' "$diff" | head -10
  else say "ok   language parity"; fi
else
  miss=""
  while IFS= read -r f; do
    case "$f" in ./AGENTS.md|./CHANGELOG.md|./"$DOCS"/CURRENT-GOAL.md) continue ;; esac
    [ -f "${f%.md}.ru.md" ] || miss="$miss
  no mirror  $f"
  done < <(find . -name '*.md' -not -path './.git/*' -not -path './.tmp/*' -not -path './.github/*' \
             -not -path "./$DOCS/history/*" -not -name '*.ru.md' | sort)
  while IFS= read -r f; do
    [ -f "${f%.ru.md}.md" ] || miss="$miss
  no source  $f"
  done < <(find . -name '*.ru.md' -not -path './.git/*' -not -path './.tmp/*' \
             -not -path "./$DOCS/history/*" | sort)
  if [ -n "$miss" ]; then bad "language parity:$miss"; else say "ok   language parity"; fi
fi

# 5. Якорные факты: значения, которые расходятся с кодом молча.
if [ -f "$DOCS/anchors.txt" ]; then
  n=0; miss=""
  while IFS='|' read -r mode pattern path; do
    case "$mode" in ''|\#*) continue ;; esac
    n=$((n + 1))
    case "$mode" in
      must)    grep -qE -- "$pattern" "$path" 2>/dev/null || miss="$miss
  must    $pattern  ($path)" ;;
      mustnot) grep -qE -- "$pattern" "$path" 2>/dev/null && miss="$miss
  mustnot $pattern  ($path)" ;;
      *) say "WARN unknown mode '$mode' in $DOCS/anchors.txt" ;;
    esac
  done <"$DOCS/anchors.txt"
  if [ -n "$miss" ]; then bad "anchor facts:$miss"
  else say "ok   anchors ($n)"; fi
fi

exit $fail
