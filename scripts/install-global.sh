#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${AIW_INSTALL_DIR:-$HOME/.local/bin}"

mkdir -p "$target"
chmod +x "$root/bin/aiw"
ln -sf "$root/bin/aiw" "$target/aiw"

echo "Installed aiw -> $target/aiw"
if command -v aiw >/dev/null 2>&1; then
  echo "Resolved aiw: $(command -v aiw)"
else
  echo "aiw is installed, but $target is not on PATH for this shell."
fi
