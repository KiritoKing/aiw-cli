#!/usr/bin/env bash
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.22+ is required to build AIW" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${AIW_DIST_DIR:-$repo_root/dist}"
mkdir -p "$output_dir"

for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  goos="${target%%/*}"
  goarch="${target##*/}"
  artifact="$output_dir/aiw-$goos-$goarch"
  (cd "$repo_root" && env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -buildvcs=false -trimpath -o "$artifact" ./cmd/aiw)
  echo "built $artifact"
done
