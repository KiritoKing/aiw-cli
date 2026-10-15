#!/usr/bin/env bash
set -euo pipefail

if (( $# != 0 )); then
  echo "usage: scripts/check-architecture.sh" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/.harness/architecture"

# Keep development-only dependencies separate from the CLI module. Any tool,
# self-test, configuration, or boundary failure stops the gate.
env GOWORK=off go test -mod=readonly ./...
env GOWORK=off go run -mod=readonly . "$repo_root"
