package aiw

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildVersion is replaced by release builds with the Git tag that produced them.
var BuildVersion = "dev"

const (
	toolchainDirName  = ".aiw"
	toolchainFileName = "aiw-toolchain.env"
	bootstrapFileName = "bootstrap-aiw.sh"
	bootstrapScript   = `#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=aiw-toolchain.env
source "$script_dir/aiw-toolchain.env"

if [[ -z "${AIW_VERSION:-}" || "$AIW_VERSION" == "dev" ]]; then
  echo "AIW_VERSION must name a published release; update $script_dir/aiw-toolchain.env" >&2
  exit 1
fi
if [[ -z "${AIW_ARTIFACT_BASE_URL:-}" ]]; then
  echo "AIW_ARTIFACT_BASE_URL is required in $script_dir/aiw-toolchain.env" >&2
  exit 1
fi

case "$(uname -s)" in
  Darwin) aiw_os="darwin" ;;
  Linux) aiw_os="linux" ;;
  *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  arm64|aarch64) aiw_arch="arm64" ;;
  x86_64|amd64) aiw_arch="amd64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

checksum_var="AIW_SHA256_$(printf '%s_%s' "$aiw_os" "$aiw_arch" | tr '[:lower:]' '[:upper:]')"
expected_sha="${!checksum_var:-}"
if [[ ! "$expected_sha" =~ ^[[:xdigit:]]{64}$ ]]; then
  echo "$checksum_var must contain a 64-character SHA-256 before downloading AIW" >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to download AIW" >&2
  exit 1
fi

archive="aiw_${AIW_VERSION}_${aiw_os}_${aiw_arch}.tar.gz"
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/aiw.XXXXXX")"
cleanup() { rm -rf -- "$tmp_dir"; }
trap cleanup EXIT
curl --fail --location --proto '=https' --tlsv1.2 --retry 3 \
  --output "$tmp_dir/$archive" "${AIW_ARTIFACT_BASE_URL%/}/$archive"

if command -v shasum >/dev/null 2>&1; then
  actual_sha="$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual_sha="$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')"
else
  echo "shasum or sha256sum is required to verify AIW" >&2
  exit 1
fi
if [[ "$actual_sha" != "$expected_sha" ]]; then
  echo "AIW archive checksum mismatch" >&2
  exit 1
fi
if [[ "$(tar -tzf "$tmp_dir/$archive")" != "aiw" ]]; then
  echo "AIW archive must contain exactly one aiw executable" >&2
  exit 1
fi
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"
if [[ ! -f "$tmp_dir/aiw" ]]; then
  echo "AIW archive did not contain an executable" >&2
  exit 1
fi

aiw_bin_dir="${AIW_BIN_DIR:-${XDG_BIN_HOME:-${HOME:?HOME is required}/.local/bin}}"
mkdir -p "$aiw_bin_dir"
install -m 0755 "$tmp_dir/aiw" "$aiw_bin_dir/aiw"
if [[ "$("$aiw_bin_dir/aiw" version)" != "$AIW_VERSION" ]]; then
  echo "installed AIW version did not match AIW_VERSION" >&2
  exit 1
fi
printf '%s\n' "$aiw_bin_dir"
`
)

func toolchainPath(root string) string {
	return filepath.Join(root, toolchainDirName, toolchainFileName)
}

func bootstrapPath(root string) string {
	return filepath.Join(root, toolchainDirName, bootstrapFileName)
}

func buildVersion() string {
	if safeKey.MatchString(BuildVersion) {
		return BuildVersion
	}
	return "dev"
}

func toolchainContents() []byte {
	return []byte(fmt.Sprintf(`# AIW bootstrap lock. Commit a published version and its release checksums.
# A local development binary intentionally writes AIW_VERSION="dev"; bootstrap refuses it.
AIW_VERSION=%q
# Base directory that contains aiw_<version>_<os>_<arch>.tar.gz. HTTPS only.
AIW_ARTIFACT_BASE_URL=""
AIW_SHA256_DARWIN_ARM64=""
AIW_SHA256_DARWIN_AMD64=""
AIW_SHA256_LINUX_ARM64=""
AIW_SHA256_LINUX_AMD64=""
`, buildVersion()))
}

func checkToolchainPaths(root string) error {
	dir := filepath.Join(root, toolchainDirName)
	if info, err := os.Lstat(dir); err == nil && !info.IsDir() {
		return fmt.Errorf("AIW toolchain directory is not a regular directory: %s", dir)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, path := range []string{toolchainPath(root), bootstrapPath(root)} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing AIW bootstrap file: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func writeToolchainFiles(root string) error {
	if err := atomicWrite(toolchainPath(root), toolchainContents()); err != nil {
		return err
	}
	if err := os.Chmod(toolchainPath(root), 0644); err != nil {
		return err
	}
	if err := atomicWrite(bootstrapPath(root), []byte(bootstrapScript)); err != nil {
		return err
	}
	return os.Chmod(bootstrapPath(root), 0755)
}
