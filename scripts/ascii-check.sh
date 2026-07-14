#!/usr/bin/env bash
# Fail if any tracked file contains a non-ASCII character.
#
# This project writes plain ASCII: no em dashes, ellipses, arrows, or typographic
# punctuation, in code, comments, docs, or printed strings. They are easy to
# introduce by copy-paste and awkward to grep for, so this check keeps them out.
#
# Use the ASCII equivalent instead: "-" for an em dash, "..." for an ellipsis,
# "->" for an arrow, "x" for a multiplication sign.
set -euo pipefail

status=0
while IFS= read -r file; do
    [ -f "$file" ] || continue
    if LC_ALL=C grep -nP '[^\x00-\x7F]' "$file" >/dev/null 2>&1; then
        echo "non-ASCII characters in ${file}:"
        LC_ALL=C grep -nP '[^\x00-\x7F]' "$file" | sed 's/^/  /'
        status=1
    fi
done < <(git ls-files)

if [ "$status" -eq 0 ]; then
    echo "ascii-check: all tracked files are ASCII"
fi
exit "$status"
