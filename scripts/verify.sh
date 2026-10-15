#!/usr/bin/env bash
set -euo pipefail

if (( $# != 0 )); then
  echo "usage: scripts/verify.sh" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

scripts/check-architecture.sh
go test ./...
go vet ./...
scripts/build-release.sh
