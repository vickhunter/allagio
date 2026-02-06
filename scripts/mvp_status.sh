#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
repo_root=$(dirname "$script_dir")
today=$(date '+%B %d, %Y')

echo "Affilio MVP status summary"
echo "Date: $today"
echo

checklist_path="$repo_root/MVP_CHECKLIST.md"
status_path="$repo_root/STATUS.md"

if [ -f "$checklist_path" ]; then
  echo "Checklist: MVP_CHECKLIST.md"
  tmp_file="$(mktemp)"
  awk -v today="$today" '
    /^Last Updated:/ { print "Last Updated: " today; next }
    { print }
  ' "$checklist_path" > "$tmp_file"
  mv "$tmp_file" "$checklist_path"
else
  echo "Checklist: missing"
fi

if [ -f "$status_path" ]; then
  echo "Status doc: STATUS.md"
else
  echo "Status doc: missing"
fi

if [ -f "$repo_root/affilio-api/Makefile" ]; then
  echo "Test target: affilio-api/Makefile (make test)"
else
  echo "Test target: missing"
fi

if [ -f "$repo_root/affilio-api/docker-compose.test.yml" ]; then
  echo "Test DB compose: affilio-api/docker-compose.test.yml"
else
  echo "Test DB compose: missing"
fi

echo
echo "Tips:"
echo "  - Update MVP_CHECKLIST.md when a task is completed."
echo "  - Use STATUS.md for narrative updates and milestones."
