package aiw

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func dataHome() (string, error) {
	if custom := os.Getenv("AIW_HOME"); custom != "" {
		return filepath.Abs(custom)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "aiw"), nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func canonicalRemote(value string) string {
	value = strings.TrimSpace(value)
	if u, err := url.Parse(value); err == nil {
		if u.Scheme == "file" {
			return "file:" + filepath.Clean(u.Path)
		}
		if u.Host != "" {
			host := strings.ToLower(u.Hostname())
			if u.Port() != "" {
				host += ":" + u.Port()
			}
			return host + "/" + strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
		}
	}
	if at := strings.IndexByte(value, '@'); at >= 0 {
		if colon := strings.IndexByte(value[at:], ':'); colon >= 0 {
			return strings.ToLower(value[at+1:at+colon]) + "/" + strings.TrimSuffix(value[at+colon+1:], ".git")
		}
	}
	if abs, err := filepath.Abs(value); err == nil {
		return "file:" + filepath.Clean(abs)
	}
	return value
}
