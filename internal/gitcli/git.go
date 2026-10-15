package gitcli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func gitBin() string {
	if value := os.Getenv("AIW_GIT"); value != "" {
		return value
	}
	return "git"
}

// Run invokes Git and returns trimmed stdout. Errors retain the operation,
// wrapped process error, and credential-redacted stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command(gitBin(), args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		operation := "command"
		if len(args) > 0 {
			operation = args[0]
		}
		return "", fmt.Errorf("git %s: %w: %s", operation, err, RedactCredentials(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(stdout.String()), nil
}

var urlCredential = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s]+@`)

// RedactCredentials hides credentials in URL-shaped stderr text.
func RedactCredentials(message string) string {
	return urlCredential.ReplaceAllString(message, "${1}[redacted]@")
}

// OK reports whether the Git command succeeds.
func OK(dir string, args ...string) bool {
	_, err := Run(dir, args...)
	return err == nil
}

// Root returns the canonical top-level path of the repository at dir.
func Root(dir string) (string, error) {
	root, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

// Branch returns the current named branch.
func Branch(dir string) (string, error) {
	b, err := Run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || b == "" {
		return "", fmt.Errorf("a named branch is required in %s", dir)
	}
	return b, nil
}

// Head returns the current commit SHA.
func Head(dir string) (string, error) { return Run(dir, "rev-parse", "HEAD") }

// RefSHA resolves ref to a commit SHA.
func RefSHA(dir, ref string) (string, error) {
	return Run(dir, "rev-parse", "--verify", ref+"^{commit}")
}

// CommonDir returns the canonical shared Git directory for the repository at dir.
func CommonDir(dir string) (string, error) {
	p, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// AssertBranchName reports whether name is a valid Git branch name.
func AssertBranchName(name string) error {
	if name == "" || !OK("", "check-ref-format", "--branch", name) {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	return nil
}

type GitStatus struct {
	Head     string `json:"head"`
	Branch   string `json:"branch"`
	Dirty    bool   `json:"dirty"`
	Changed  int    `json:"changedFiles"`
	Upstream string `json:"upstream,omitempty"`
	Ahead    string `json:"ahead,omitempty"`
	Behind   string `json:"behind,omitempty"`
}

// Status reads the current branch, commit, and worktree summary.
func Status(dir string) (GitStatus, error) {
	output, err := Run(dir, "status", "--porcelain=v2", "--branch", "--untracked-files=all")
	if err != nil {
		return GitStatus{}, err
	}
	var s GitStatus
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			s.Head = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.head "):
			s.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.upstream "):
			s.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%s -%s", &s.Ahead, &s.Behind)
		case line != "" && !strings.HasPrefix(line, "# "):
			s.Dirty = true
			s.Changed++
		}
	}
	return s, nil
}

// DirtyDetails returns short status lines including untracked and ignored files.
func DirtyDetails(dir string) (string, error) {
	return Run(dir, "status", "--short", "--untracked-files=all", "--ignored")
}

// RemoteHead returns the SHA of the named origin branch.
func RemoteHead(dir, name string) (string, error) {
	remote, err := Run(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	out, err := Run(dir, "ls-remote", "--heads", remote, "refs/heads/"+name)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == "refs/heads/"+name {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("remote branch %s does not exist", name)
}
