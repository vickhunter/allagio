#!/bin/sh
# Build the hosted image. The landing pages are required. A missing
# landing-v9 directory is a failed build, not a skipped copy.
set -eu
cd "$(dirname "$0")/.."
landing="${LANDING_V9_DIR:-affilio-wp/admin/build/landing-v9}"
if [ ! -d "$landing" ]; then
  echo "image/build-image.sh: $landing is missing. The image is not built without the landing pages." >&2
  exit 1
fi
exec docker build -f Dockerfile.mvp -t allagio:local .
