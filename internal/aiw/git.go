package aiw

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

func git(dir string, args ...string) (string, error) {
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
		return "", fmt.Errorf("git %s: %w: %s", operation, err, redactCredentials(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(stdout.String()), nil
}

var urlCredential = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s]+@`)

func redactCredentials(message string) string {
	return urlCredential.ReplaceAllString(message, "${1}[redacted]@")
}

func gitOk(dir string, args ...string) bool {
	_, err := git(dir, args...)
	return err == nil
}

func gitRoot(dir string) (string, error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

func branch(dir string) (string, error) {
	b, err := git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || b == "" {
		return "", fmt.Errorf("a named branch is required in %s", dir)
	}
	return b, nil
}

func head(dir string) (string, error) { return git(dir, "rev-parse", "HEAD") }

func refSHA(dir, ref string) (string, error) {
	return git(dir, "rev-parse", "--verify", ref+"^{commit}")
}

func gitCommonDir(dir string) (string, error) {
	p, err := git(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func assertBranchName(name string) error {
	if name == "" || !gitOk("", "check-ref-format", "--branch", name) {
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

func status(dir string) (GitStatus, error) {
	output, err := git(dir, "status", "--porcelain=v2", "--branch", "--untracked-files=all")
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

func dirtyDetails(dir string) (string, error) {
	return git(dir, "status", "--short", "--untracked-files=all", "--ignored")
}

func remoteHead(dir, name string) (string, error) {
	remote, err := git(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	out, err := git(dir, "ls-remote", "--heads", remote, "refs/heads/"+name)
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
