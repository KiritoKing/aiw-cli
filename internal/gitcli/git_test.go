package gitcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0700); err != nil {
		t.Fatal("write fake Git executable")
	}
}

func TestRunUsesAIWGitAndPreservesArguments(t *testing.T) {
	gitPath := filepath.Join(t.TempDir(), "fake-git")
	writeExecutable(t, gitPath, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	t.Setenv("AIW_GIT", gitPath)

	got, err := Run("", "remote", "get-url", "origin with space")
	if err != nil {
		t.Fatal("Run failed with the configured fake Git executable")
	}
	want := "remote\nget-url\norigin with space"
	if got != want {
		t.Fatalf("Run changed the command arguments: got %q, want %q", got, want)
	}
}

func TestRunRedactsCredentialsAndPreservesErrorShape(t *testing.T) {
	gitPath := filepath.Join(t.TempDir(), "fake-git")
	writeExecutable(t, gitPath, "#!/bin/sh\nprintf '%s\\n' 'fatal: https://test-user:test-password@example.invalid/repo.git denied' >&2\nexit 17\n")
	t.Setenv("AIW_GIT", gitPath)

	_, err := Run("", "fetch", "--all")
	if err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	message := err.Error()
	if strings.Contains(message, "test-user") || strings.Contains(message, "test-password") {
		t.Fatal("Run exposed credentials from Git stderr")
	}
	want := "git fetch: exit status 17: fatal: https://[redacted]@example.invalid/repo.git denied"
	if message != want {
		t.Fatal("Run changed the Git error or stderr formatting")
	}
}
