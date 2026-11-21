package aiw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func gitBin() string {
	if value := os.Getenv("AIW_GIT"); value != "" {
		return value
	}
	return "git"
}

type gitRunner struct {
	ctx     context.Context
	verbose io.Writer
}

func (g gitRunner) git(dir string, args ...string) (string, error) {
	start := time.Now()
	if g.verbose != nil {
		defer func() {
			safe := make([]string, len(args))
			for i, arg := range args {
				safe[i] = redactCredentials(arg)
			}
			fmt.Fprintf(g.verbose, "git %q in %q (%s)\n", safe, dir, time.Since(start).Round(time.Millisecond))
		}()
	}
	cmd := exec.CommandContext(g.ctx, gitBin(), args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if g.ctx.Err() != nil {
			return "", fmt.Errorf("git cancelled: %w", g.ctx.Err())
		}
		operation := "command"
		if len(args) > 0 {
			operation = args[0]
		}
		return "", fmt.Errorf("git %s: %w: %s", operation, err, redactCredentials(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(stdout.String()), nil
}

var (
	urlCredential = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/\s]+@`)
	urlParameters = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>?#]+)[?#][^\s'"<>]*`)
)

func redactCredentials(message string) string {
	message = urlCredential.ReplaceAllString(message, "${1}[redacted]@")
	return urlParameters.ReplaceAllString(message, "${1}[redacted]")
}

func (g gitRunner) gitOk(dir string, args ...string) bool {
	_, err := g.git(dir, args...)
	return err == nil
}

func (g gitRunner) gitRoot(dir string) (string, error) {
	root, err := g.git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

func (g gitRunner) branch(dir string) (string, error) {
	b, err := g.git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || b == "" {
		return "", fmt.Errorf("a named branch is required in %s", dir)
	}
	return b, nil
}

func (g gitRunner) head(dir string) (string, error) { return g.git(dir, "rev-parse", "HEAD") }

func (g gitRunner) refSHA(dir, ref string) (string, error) {
	return g.git(dir, "rev-parse", "--verify", ref+"^{commit}")
}

func (g gitRunner) gitCommonDir(dir string) (string, error) {
	p, err := g.git(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func (g gitRunner) assertBranchName(name string) error {
	if name == "" || !g.gitOk("", "check-ref-format", "--branch", name) {
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

func (g gitRunner) gitStatus(dir string) (GitStatus, error) {
	output, err := g.git(dir, "status", "--porcelain=v2", "--branch", "--untracked-files=all")
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

func (g gitRunner) dirtyDetails(dir string) (string, error) {
	return g.git(dir, "status", "--short", "--untracked-files=all", "--ignored")
}

func (g gitRunner) remoteHead(dir, name string) (string, error) {
	remote, err := g.git(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	out, err := g.git(dir, "ls-remote", "--heads", "--", remote, "refs/heads/"+name)
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
