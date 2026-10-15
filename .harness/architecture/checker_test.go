package main

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed rules.json
var realRules embed.FS

func rulesBytes(t *testing.T) []byte {
	t.Helper()
	data, err := realRules.ReadFile("rules.json")
	if err != nil {
		t.Fatal("read embedded architecture rules")
	}
	return data
}

func writeSources(t *testing.T, root string, sources map[string]string) {
	t.Helper()
	for relative, source := range sources {
		filename := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal("create fixture directory")
		}
		if err := os.WriteFile(filename, []byte(source), 0644); err != nil {
			t.Fatal("write fixture source")
		}
	}
}

func validSourceTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSources(t, root, map[string]string{
		"cmd/aiw/main.go": `package main
import "github.com/KiritoKing/aiw-cli/internal/commands"
var _ = commands.Run
`,
		"internal/commands/root.go": `package commands
import (
  "github.com/KiritoKing/aiw-cli/internal/aiw"
  "github.com/spf13/cobra"
)
var _ aiw.Service
var _ cobra.Command
`,
		"internal/commands/root_test.go": `package commands
import "gopkg.in/yaml.v3"
var _ = yaml.Unmarshal
`,
		"internal/aiw/service.go": `package aiw
import (
  "encoding/json"
  "github.com/KiritoKing/aiw-cli/internal/gitcli"
  "github.com/KiritoKing/aiw-cli/internal/localfs"
  "github.com/KiritoKing/aiw-cli/skills"
  "gopkg.in/yaml.v3"
)
var _, _, _, _ = json.Marshal, gitcli.Run, localfs.AtomicWrite, skills.A
var _ = yaml.Unmarshal
`,
		"internal/gitcli/git.go": `package gitcli
import "os/exec"
var _ = exec.Command
`,
		"internal/localfs/localfs.go": `package localfs
import "syscall"
var _ = syscall.Flock
`,
		"skills/embed.go": `package skills
import _ "embed"
`,
		".harness/architecture/tool.go": `package main
import (
  "github.com/OpenPeeDeeP/depguard/v2"
  "github.com/gobwas/glob"
  "golang.org/x/tools/go/analysis"
)
var _, _, _ = depguard.NewAnalyzer, glob.Compile, analysis.Analyzer{}
`,
	})
	return root
}

func TestRealRulesAllowTheDeclaredDependencyLayers(t *testing.T) {
	if _, err := checkRepository(validSourceTree(t), rulesBytes(t)); err != nil {
		t.Fatalf("real architecture rules rejected the allowed fixture: %v", err)
	}
}

func TestArchitectureViolations(t *testing.T) {
	gitcliImport := `"github.com/KiritoKing/aiw-cli/internal/gitcli"`
	cobraImport := `"github.com/spf13/cobra"`
	coreImport := `"github.com/KiritoKing/aiw-cli/internal/aiw"`
	commandsImport := `"github.com/KiritoKing/aiw-cli/internal/commands"`
	analysisImport := `"golang.org/x/tools/go/analysis"`
	tests := []struct {
		name   string
		path   string
		source string
		want   string
	}{
		{name: "commands reaches infrastructure", path: "internal/commands/violation.go", source: "package commands\nimport " + gitcliImport, want: "architecture violations"},
		{name: "business imports Cobra", path: "internal/aiw/violation.go", source: "package aiw\nimport " + cobraImport, want: "architecture violations"},
		{name: "infrastructure imports core", path: "internal/gitcli/violation.go", source: "package gitcli\nimport " + coreImport, want: "architecture violations"},
		{name: "skills imports commands", path: "skills/violation.go", source: "package skills\nimport " + commandsImport, want: "architecture violations"},
		{name: "command tests are scoped", path: "internal/commands/violation_test.go", source: "package commands\nimport " + gitcliImport, want: "architecture violations"},
		{name: "unknown package without imports", path: "internal/future/future.go", source: "package future\n", want: "matches 0 architecture layers"},
		{name: "aliased import", path: "internal/commands/alias.go", source: "package commands\nimport git " + gitcliImport, want: "architecture violations"},
		{name: "whitespace import block", path: "internal/commands/spaced.go", source: "package commands\nimport (\n\t  " + gitcliImport + "\n)\n", want: "architecture violations"},
		{name: "blank import", path: "internal/commands/blank.go", source: "package commands\nimport _ " + gitcliImport, want: "architecture violations"},
		{name: "raw string import", path: "internal/commands/raw.go", source: "package commands\nimport _ `github.com/KiritoKing/aiw-cli/internal/gitcli`", want: "architecture violations"},
		{name: "escaped import", path: "internal/commands/escaped.go", source: "package commands\nimport _ \"github.com/KiritoKing/aiw-cli/internal\\x2fgitcli\"", want: "architecture violations"},
		{name: "custom build tag", path: "internal/commands/tagged.go", source: "//go:build architecture_custom\n\npackage commands\nimport " + gitcliImport, want: "architecture violations"},
		{name: "linux suffix", path: "internal/commands/violation_linux.go", source: "package commands\nimport " + gitcliImport, want: "architecture violations"},
		{name: "darwin suffix", path: "internal/commands/violation_darwin.go", source: "package commands\nimport " + gitcliImport, want: "architecture violations"},
		{name: "nolint does not suppress", path: "internal/commands/nolint.go", source: "package commands\n//nolint:depguard\nimport " + gitcliImport, want: "architecture violations"},
		{name: "golang.org is not standard library", path: "internal/localfs/nonstandard.go", source: "package localfs\nimport " + analysisImport, want: "architecture violations"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := validSourceTree(t)
			writeSources(t, root, map[string]string{test.path: test.source})
			_, err := checkRepository(root, rulesBytes(t))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected architecture rejection containing %q", test.want)
			}
		})
	}
}

func TestOverlappingRulesFailBeforeImportAnalysis(t *testing.T) {
	var config rulesFile
	if err := json.Unmarshal(rulesBytes(t), &config); err != nil {
		t.Fatal("decode embedded rules")
	}
	config.Layers["duplicate-commands"] = config.Layers["commands-production"]
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal("encode overlapping rules")
	}
	_, err = checkRepository(validSourceTree(t), data)
	if err == nil || !strings.Contains(err.Error(), "matches 2 architecture layers") {
		t.Fatal("overlapping file rules were not rejected")
	}
}

func TestInvalidRulesFailClosed(t *testing.T) {
	valid := `{"version":1,"layers":{"test":{"files":["cmd/aiw/*.go"],"allow":["$gostd"]}}}`
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "empty bytes", data: "", want: "EOF"},
		{name: "empty object", data: "{}", want: "unsupported architecture rules version"},
		{name: "missing layers", data: `{"version":1}`, want: "at least one layer"},
		{name: "unknown field", data: `{"version":1,"layers":{"test":{"files":["cmd/aiw/*.go"],"allow":["$gostd"],"typo":true}}}`, want: "unknown field"},
		{name: "malformed", data: `{"version":`, want: "invalid architecture rules JSON"},
		{name: "trailing JSON", data: valid + ` {}`, want: "trailing JSON"},
		{name: "duplicate key", data: `{"version":1,"version":1,"layers":{"test":{"files":["cmd/aiw/*.go"],"allow":["$gostd"]}}}`, want: "duplicate JSON key"},
		{name: "unsupported version", data: strings.Replace(valid, `"version":1`, `"version":2`, 1), want: "unsupported architecture rules version"},
		{name: "bare import prefix", data: `{"version":1,"layers":{"test":{"files":["cmd/aiw/*.go"],"allow":["$gostd","github.com/example/module"]}}}`, want: "must end in $ or /"},
		{name: "invalid glob", data: strings.Replace(valid, `cmd/aiw/*.go`, `cmd/aiw/[*.go`, 1), want: "invalid file glob"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newChecker([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected invalid rules error containing %q", test.want)
			}
		})
	}

	if _, err := Check(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read .harness/architecture/rules.json") {
		t.Fatal("missing rules file was accepted")
	}
}

func TestGoSourceSymlinksAreRejected(t *testing.T) {
	t.Run("source filename", func(t *testing.T) {
		root := validSourceTree(t)
		target := filepath.Join(t.TempDir(), "target.go")
		if err := os.WriteFile(target, []byte("package commands\n"), 0644); err != nil {
			t.Fatal("write symlink target")
		}
		link := filepath.Join(root, "internal", "commands", "linked.go")
		if err := os.Symlink(target, link); err != nil {
			t.Skip("symlinks are unavailable")
		}
		if _, err := checkRepository(root, rulesBytes(t)); err == nil || !strings.Contains(err.Error(), "Go source symlink is forbidden") {
			t.Fatal("Go source symlink was not rejected")
		}
	})

	t.Run("Go target outside code roots", func(t *testing.T) {
		root := validSourceTree(t)
		target := filepath.Join(t.TempDir(), "target.go")
		if err := os.WriteFile(target, []byte("package hidden\n"), 0644); err != nil {
			t.Fatal("write symlink target")
		}
		resourceDir := filepath.Join(root, "docs")
		if err := os.MkdirAll(resourceDir, 0755); err != nil {
			t.Fatal("create resource directory")
		}
		if err := os.Symlink(target, filepath.Join(resourceDir, "linked-resource")); err != nil {
			t.Skip("symlinks are unavailable")
		}
		if _, err := checkRepository(root, rulesBytes(t)); err == nil || !strings.Contains(err.Error(), "Go source symlink is forbidden") {
			t.Fatal("symlink to a Go file was not rejected")
		}
	})

	t.Run("source directory", func(t *testing.T) {
		root := validSourceTree(t)
		target := t.TempDir()
		link := filepath.Join(root, "internal", "commands", "linked-dir")
		if err := os.Symlink(target, link); err != nil {
			t.Skip("symlinks are unavailable")
		}
		if _, err := checkRepository(root, rulesBytes(t)); err == nil || !strings.Contains(err.Error(), "source directory symlink is forbidden") {
			t.Fatal("source directory symlink was not rejected")
		}
	})

	t.Run("code root directory", func(t *testing.T) {
		root := validSourceTree(t)
		if err := os.RemoveAll(filepath.Join(root, "cmd")); err != nil {
			t.Fatal("remove fixture command directory")
		}
		if err := os.Symlink(t.TempDir(), filepath.Join(root, "cmd")); err != nil {
			t.Skip("symlinks are unavailable")
		}
		if _, err := checkRepository(root, rulesBytes(t)); err == nil || !strings.Contains(err.Error(), "source directory symlink is forbidden") {
			t.Fatal("code root symlink was not rejected")
		}
	})
}

func TestNonSourceResourceSymlinkIsIgnored(t *testing.T) {
	root := validSourceTree(t)
	target := filepath.Join(t.TempDir(), "resource.txt")
	if err := os.WriteFile(target, []byte("data"), 0644); err != nil {
		t.Fatal("write resource target")
	}
	link := filepath.Join(root, "internal", "aiw", "resource.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks are unavailable")
	}
	if _, err := checkRepository(root, rulesBytes(t)); err != nil {
		t.Fatalf("non-source resource symlink should be ignored: %v", err)
	}
}

func TestExcludedDirectoriesAreNotScanned(t *testing.T) {
	root := validSourceTree(t)
	writeSources(t, root, map[string]string{
		".git/hidden.go":             "this is not Go syntax\n",
		"vendor/example/hidden.go":   "this is not Go syntax\n",
		"node_modules/mod/hidden.go": "this is not Go syntax\n",
	})
	if _, err := checkRepository(root, rulesBytes(t)); err != nil {
		t.Fatalf("excluded directories were scanned: %v", err)
	}
}
