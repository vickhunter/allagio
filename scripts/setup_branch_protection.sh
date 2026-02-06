#!/usr/bin/env sh
set -eu

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI is required"
  exit 1
fi

if [ -z "${GITHUB_OWNER:-}" ] || [ -z "${GITHUB_REPO:-}" ]; then
  echo "error: set GITHUB_OWNER and GITHUB_REPO"
  exit 1
fi

BRANCH="${BRANCH_NAME:-main}"

echo "Configuring branch protection for ${GITHUB_OWNER}/${GITHUB_REPO} (${BRANCH})"

gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  "/repos/${GITHUB_OWNER}/${GITHUB_REPO}/branches/${BRANCH}/protection" \
  -f required_status_checks.strict=true \
  -f required_status_checks.contexts[]="affilio-api-tests" \
  -f enforce_admins=true \
  -f required_pull_request_reviews.dismiss_stale_reviews=true \
  -f required_pull_request_reviews.required_approving_review_count=1 \
  -f required_linear_history=false \
  -f allow_force_pushes=false \
  -f allow_deletions=false \
  -f block_creations=false \
  -f required_conversation_resolution=true \
  -f lock_branch=false \
  -f allow_fork_syncing=true \
  -f restrictions=

echo "Branch protection applied successfully."
