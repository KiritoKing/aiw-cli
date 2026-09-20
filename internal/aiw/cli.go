package aiw

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "agents":
		return runAgents(args[1:])
	case "doctor":
		return runDoctor()
	case "repo":
		return runRepo(args[1:])
	case "change":
		return runChange(args[1:])
	case "completion":
		return runCompletion(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run aiw --help", args[0])
	}
}

func printHelp() {
	fmt.Print(`AIW assembles business Git worktrees inside an Agent Repo worktree created by your control plane.

Usage:
  aiw init <agent-repo-path> [--skip-skills]
  aiw agents sync
  aiw doctor
  aiw repo list [--json] | register <name> --source <path> | scan <dir> | sync [name]
  aiw change select --repo <name> [--repo <name>...]
  aiw change add-repo <name> | remove-repo <name>
  aiw change materialize | repair | status [--json]
  aiw change setup [name] | cleanup <name>
  aiw change list | path <branch> | open <branch>
  aiw change close [--allow-unpushed] [--discard-changes]
  aiw completion zsh | bash

The control plane creates the Agent Repo worktree. During Propose, write .aiw/change.yaml.
Before dispatching business tasks, run aiw change materialize; it runs selected repo setup scripts.
The control plane calls aiw change close during cleanup; AIW runs repo cleanup before release.
`)
}

func runInit(args []string) error {
	var target string
	skipSkills := false
	for _, arg := range args {
		switch {
		case arg == "--skip-skills" && !skipSkills:
			skipSkills = true
		case strings.HasPrefix(arg, "-") || target != "":
			return fmt.Errorf("usage: aiw init <agent-repo-path> [--skip-skills]")
		default:
			target = arg
		}
	}
	if target == "" {
		return fmt.Errorf("usage: aiw init <agent-repo-path> [--skip-skills]")
	}
	path, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("refusing to initialize Git in a nonempty directory: %s", path)
		}
		if _, err := git(path, "init", "-b", "main"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	root, err := gitRoot(path)
	if err != nil {
		return fmt.Errorf("inspect Git root at %s: %w", path, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve Agent Repo path %s: %w", path, err)
	}
	if root != resolvedPath {
		return fmt.Errorf("path must be an Agent Repo root: %s (Git root: %s)", path, root)
	}
	if _, err := os.Stat(filepath.Join(path, "aiw.yaml")); err == nil {
		return fmt.Errorf("aiw.yaml already exists: %s", path)
	}
	if err := syncAgentInstructions(path); err != nil {
		return err
	}
	b, err := branch(path)
	if err != nil {
		return err
	}
	cfg := ProjectConfig{Version: 1, Name: filepath.Base(path), Base: b, Repos: map[string]RepoConfig{}}
	if err := saveProject(path, cfg); err != nil {
		return err
	}
	ignore := filepath.Join(path, ".gitignore")
	old, err := os.ReadFile(ignore)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !strings.Contains("\n"+string(old)+"\n", "\n/repos/\n") {
		content := string(old)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "/repos/\n"
		if err := atomicWrite(ignore, []byte(content)); err != nil {
			return err
		}
	}
	if !skipSkills {
		if err := installSkills(path); err != nil {
			return fmt.Errorf("Agent Repo initialized at %s, but skill installation failed: %w; install skills separately or use your own installer", path, err)
		}
	}
	fmt.Printf("initialized %s; edit aiw.yaml, then commit it in the Agent Repo\n", path)
	return nil
}

func runDoctor() error {
	version, err := git("", "--version")
	if err != nil {
		return err
	}
	fmt.Println(version)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		fmt.Println("no Agent Repo at current path")
		return nil
	}
	cfg, err := loadProject(root)
	if err != nil {
		return err
	}
	fmt.Printf("Agent Repo: %s (%s, %d repos)\n", cfg.Name, root, len(cfg.Repos))
	return nil
}

func runRepo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aiw repo list|register|scan|sync")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return err
	}
	home, err := dataHome()
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 && !(len(args) == 2 && args[1] == "--json") {
			return fmt.Errorf("usage: aiw repo list [--json]")
		}
		cfg, err := loadProject(root)
		if err != nil {
			return err
		}
		registry, err := loadRegistry(home)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(cfg.Repos))
		for name := range cfg.Repos {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(args) == 2 {
			rows := make([]RepoListRow, 0, len(names))
			for _, name := range names {
				repo := cfg.Repos[name]
				row := RepoListRow{Name: name, Remote: redactCredentials(repo.Remote), Base: repo.Base}
				if entry, ok := registry.Repos[canonicalRemote(repo.Remote)]; ok {
					row.Registered, row.Source, row.Store = true, entry.Source, entry.Store
				}
				rows = append(rows, row)
			}
			out, err := json.MarshalIndent(rows, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		}
		for _, name := range names {
			state := "unregistered"
			if entry, ok := registry.Repos[canonicalRemote(cfg.Repos[name].Remote)]; ok {
				state = entry.Store
			}
			fmt.Printf("%-16s %-16s %s\n", name, cfg.Repos[name].Base, state)
		}
		return nil
	case "register":
		if len(args) < 2 {
			return fmt.Errorf("usage: aiw repo register <name> --source <path>")
		}
		fs := flag.NewFlagSet("repo register", flag.ContinueOnError)
		source := fs.String("source", "", "existing local checkout")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *source == "" {
			return fmt.Errorf("--source is required")
		}
		return withLock(filepath.Join(home, "registry.lock"), func() error {
			return registerRepo(root, home, args[1], *source)
		})
	case "scan":
		if len(args) != 2 {
			return fmt.Errorf("usage: aiw repo scan <dir>")
		}
		return withLock(filepath.Join(home, "registry.lock"), func() error {
			return scanRepos(root, home, args[1])
		})
	case "sync":
		if len(args) > 2 {
			return fmt.Errorf("usage: aiw repo sync [name]")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		return withLock(filepath.Join(home, "registry.lock"), func() error {
			return syncRepo(root, home, name)
		})
	default:
		return fmt.Errorf("unknown repo command %q", args[0])
	}
}

type RepoListRow struct {
	Name       string `json:"name"`
	Remote     string `json:"remote"`
	Base       string `json:"base"`
	Registered bool   `json:"registered"`
	Source     string `json:"source,omitempty"`
	Store      string `json:"store,omitempty"`
}

type stringFlags []string

func (f *stringFlags) String() string { return strings.Join(*f, ",") }
func (f *stringFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func runChange(args []string) error {
	if os.Getenv("AIW_LIFECYCLE_ACTIVE") == "1" {
		return fmt.Errorf("AIW Change commands cannot run inside a repository setup or cleanup script")
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: aiw change select|materialize|setup|cleanup|status|close|list|path|open|repair")
	}
	if args[0] == "list" || args[0] == "path" || args[0] == "open" {
		return changeLookup(args)
	}
	c, err := currentChange()
	if err != nil {
		return err
	}
	return changeLock(c, func() error { return runChangeLocked(c, args) })
}

func runChangeLocked(c ChangeContext, args []string) error {
	switch args[0] {
	case "select":
		fs := flag.NewFlagSet("change select", flag.ContinueOnError)
		var names stringFlags
		fs.Var(&names, "repo", "repository to select")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if len(names) == 0 || fs.NArg() != 0 {
			return fmt.Errorf("usage: aiw change select --repo <name> [--repo <name>...]")
		}
		return c.selectRepos(names)
	case "add-repo", "remove-repo":
		if len(args) != 2 {
			return fmt.Errorf("usage: aiw change %s <name>", args[0])
		}
		if args[0] == "add-repo" {
			return c.updateSelection("add", args[1])
		}
		return c.updateSelection("remove", args[1])
	case "materialize":
		if len(args) != 1 {
			return fmt.Errorf("usage: aiw change materialize")
		}
		return c.materialize(false)
	case "setup":
		if len(args) > 2 {
			return fmt.Errorf("usage: aiw change setup [name]")
		}
		return c.setup(args[1:])
	case "cleanup":
		if len(args) != 2 {
			return fmt.Errorf("usage: aiw change cleanup <name>")
		}
		return c.cleanup(args[1])
	case "repair":
		return c.repair()
	case "status":
		fs := flag.NewFlagSet("change status", flag.ContinueOnError)
		jsonOutput := fs.Bool("json", false, "JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return c.status(*jsonOutput)
	case "close":
		fs := flag.NewFlagSet("change close", flag.ContinueOnError)
		allow := fs.Bool("allow-unpushed", false, "close when remote branch is not confirmed equal")
		discard := fs.Bool("discard-changes", false, "discard local changes in AIW business worktrees")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return c.close(CloseOptions{AllowUnpushed: *allow, Discard: *discard})
	default:
		return fmt.Errorf("unknown change command %q", args[0])
	}
}

func changeLookup(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return err
	}
	project, err := gitCommonDir(root)
	if err != nil {
		return err
	}
	home, err := dataHome()
	if err != nil {
		return err
	}
	if args[0] == "list" {
		entries, err := os.ReadDir(filepath.Join(home, "changes", hash(project)))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(home, "changes", hash(project), entry.Name()))
			if err != nil {
				return err
			}
			var s ChangeState
			if err := json.Unmarshal(data, &s); err != nil {
				return err
			}
			fmt.Printf("%-24s %-16s %s\n", s.Branch, s.Phase, s.Root)
		}
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: aiw change %s <branch>", args[0])
	}
	s, err := loadState(home, project, args[1])
	if err != nil {
		return err
	}
	if s.Root == "" {
		return fmt.Errorf("Change %s not registered on this machine", args[1])
	}
	if _, err := os.Stat(s.Root); err != nil {
		return fmt.Errorf("control-plane worktree unavailable: %s", s.Root)
	}
	fmt.Println(s.Root)
	return nil
}
