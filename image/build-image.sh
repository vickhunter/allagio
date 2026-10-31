#!/bin/sh
# Build the hosted image. The landing pages are required. A missing
# file is a failed build, not a skipped copy.
set -eu
cd "$(dirname "$0")/.."
landing="${LANDING_V9_DIR:-affilio-wp/admin/build/landing-v9}"
if [ ! -d "$landing" ]; then
  echo "image/build-image.sh: $landing is missing. The image is not built without the landing pages." >&2
  exit 1
fi
required="llms.txt skill.md llms-full.txt home.md makers.md publishers.md publishers-setup.md protocol.md you.md privacy.md terms.md agents.md"
missing=""
for name in $required; do
  if [ ! -f "$landing/$name" ]; then
    missing="$missing $name"
  fi
done
if [ -n "$missing" ]; then
  echo "image/build-image.sh: $landing is missing:$missing" >&2
  exit 1
fi
exec docker build -f Dockerfile.mvp -t allagio:local .
