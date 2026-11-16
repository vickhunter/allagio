#!/usr/bin/env bash
# Install an opt-in pre-push hook that runs scripts/check.sh.
# Nothing in Railway or GitHub Actions calls this.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
hook="$root/.git/hooks/pre-push"
cat >"$hook" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
"$root/scripts/check.sh"
EOF
chmod +x "$hook"
echo "installed $hook"
