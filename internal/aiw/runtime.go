package aiw

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// IO contains the streams and starting directory for a single CLI invocation.
type IO struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
	Cwd    string
}

type commandEnv struct {
	IO
	gitRunner
	noColor    bool
	noInput    bool
	jsonOutput bool
}

func defaultIO() IO {
	return IO{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
}

func newCommandEnv(ctx context.Context, streams IO) commandEnv {
	if streams.In == nil {
		streams.In = os.Stdin
	}
	if streams.Out == nil {
		streams.Out = os.Stdout
	}
	if streams.ErrOut == nil {
		streams.ErrOut = os.Stderr
	}
	return commandEnv{IO: streams, gitRunner: gitRunner{ctx: ctx}}
}

func (r commandEnv) inDirectory(override string) (commandEnv, error) {
	base, err := filepath.Abs(r.Cwd)
	if err != nil {
		return r, err
	}
	r.Cwd = base
	if override != "" {
		r.Cwd = r.resolvePath(override)
		info, err := os.Stat(r.Cwd)
		if err != nil {
			return r, fmt.Errorf("--cwd: %w", err)
		}
		if !info.IsDir() {
			return r, fmt.Errorf("--cwd: %s is not a directory", r.Cwd)
		}
	}
	return r, nil
}

func (r commandEnv) resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(r.Cwd, path)
	}
	return filepath.Clean(path)
}
