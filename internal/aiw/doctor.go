package aiw

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type DoctorReport struct {
	OK     bool          `json:"ok"`
	Root   string        `json:"root,omitempty"`
	Checks []DoctorCheck `json:"checks"`
}

func (report *DoctorReport) add(name, status, detail string) {
	report.Checks = append(report.Checks, DoctorCheck{Name: name, Status: status, Detail: detail})
	if status == "fail" {
		report.OK = false
	}
}

func (report *DoctorReport) check(name, success string, err error) {
	if err != nil {
		report.add(name, "fail", err.Error())
	} else {
		report.add(name, "pass", success)
	}
}

func (r commandEnv) doctor() error {
	report := DoctorReport{OK: true}
	report.add("aiw", "pass", buildVersion())
	home, homeErr := r.dataHome()
	if homeErr == nil {
		homeErr = checkWritableHome(home)
	}
	report.check("aiw-home", home, homeErr)
	version, gitErr := r.git(r.Cwd, "--version")
	report.check("git", version, gitErr)
	for _, tool := range []string{"node", "npx"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			report.add(tool, "info", "unavailable; init uses bundled skills when Node or npx is missing")
		} else {
			report.add(tool, "pass", path)
		}
	}
	root, err := projectRoot(r.Cwd)
	switch {
	case errors.Is(err, errProjectNotFound):
		report.add("agent-repo", "info", "no Agent Repo at current path")
	case err != nil:
		report.check("agent-repo", "", err)
	default:
		report.Root = root
		if gitErr != nil {
			report.add("agent-repo", "info", "project checks need working Git")
		} else {
			r.checkAgentRepo(&report, root, home)
		}
	}
	if r.jsonOutput {
		err = r.writeJSON(report)
	} else {
		rows := make([][]string, 0, len(report.Checks))
		for _, check := range report.Checks {
			rows = append(rows, []string{check.Name, check.Status, check.Detail})
		}
		err = r.printTable([]string{"CHECK", "STATUS", "DETAIL"}, rows, 1)
	}
	if err != nil {
		return err
	}
	if !report.OK {
		return fmt.Errorf("doctor found failing checks")
	}
	return nil
}

func checkWritableHome(path string) error {
	for dir := path; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("AIW_HOME parent is not a directory: %s", dir)
		}
		probe, err := os.CreateTemp(dir, ".aiw-doctor-*")
		if err != nil {
			return err
		}
		return errors.Join(probe.Close(), os.Remove(probe.Name()))
	}
}

func (r commandEnv) checkAgentRepo(report *DoctorReport, root, home string) {
	actual, err := r.gitRoot(root)
	if err == nil && actual != root {
		err = fmt.Errorf("aiw.yaml must be at the Agent Repo worktree root: %s", root)
	}
	report.check("agent-repo", root, err)
	if err != nil {
		return
	}
	cfg, err := r.loadProject(root)
	report.check("aiw.yaml", fmt.Sprintf("%s (%d repos)", cfg.Name, len(cfg.Repos)), err)
	report.check("repos-ignore", "/repos/ is ignored and contains no tracked files", r.checkReposIgnore(root, cfg))
	report.check("agents-block", "AGENTS.md has exactly one complete AIW block", checkAgentBlock(root))
	if home == "" {
		return
	}
	if err == nil {
		r.checkStores(report, cfg, home)
	}
	project, err := r.gitCommonDir(root)
	if err == nil {
		err = r.checkOrphanChanges(report, home, project)
	}
	if err != nil {
		report.check("orphan-changes", "", err)
	}
}

func (r commandEnv) checkReposIgnore(root string, cfg ProjectConfig) error {
	tracked, err := r.git(root, "ls-files", "--", "repos")
	if err != nil {
		return err
	}
	if tracked != "" {
		return fmt.Errorf("repos/ contains tracked Agent Repo files")
	}
	for _, name := range append([]string{".aiw-doctor-check"}, repoNames(cfg)...) {
		if !r.gitOk(root, "check-ignore", "--quiet", "--no-index", "repos/"+name) {
			return fmt.Errorf("repos/%s is not ignored; add /repos/ to .gitignore", name)
		}
	}
	return nil
}

func checkAgentBlock(root string) error {
	path := filepath.Join(root, "AGENTS.md")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("AGENTS.md is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Contains(data, []byte(agentStart)) {
		return fmt.Errorf("missing AIW managed block; run aiw agents sync")
	}
	_, err = updateAgentBlock(data)
	return err
}

func (r commandEnv) checkStores(report *DoctorReport, cfg ProjectConfig, home string) {
	registry, err := loadRegistry(home)
	if err != nil {
		report.check("registry", "", err)
		return
	}
	report.check("registry", registryPath(home), nil)
	for _, name := range repoNames(cfg) {
		repo := cfg.Repos[name]
		check := "store/" + name
		if _, ok := registry.Repos[canonicalRemote(repo.Remote)]; !ok {
			report.add(check, "info", "not registered")
			continue
		}
		store, err := r.registeredStore(registry, name, repo)
		if err == nil {
			remote, gitErr := r.git(store, "remote", "get-url", "origin")
			err = gitErr
			if err == nil && canonicalRemote(remote) != canonicalRemote(repo.Remote) {
				err = fmt.Errorf("managed store origin does not match aiw.yaml")
			}
		}
		report.check(check, store, err)
	}
}

func (r commandEnv) checkOrphanChanges(report *DoctorReport, home, project string) error {
	states, err := listChangeStates(home, project)
	if err != nil {
		return err
	}
	orphaned := false
	for _, state := range states {
		exists, err := rootExists(state.Root)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		var worktrees []string
		for _, repo := range state.Repos {
			if repo.Path == "" {
				continue
			}
			exists, err := rootExists(repo.Path)
			if err != nil {
				return err
			}
			if exists {
				worktrees = append(worktrees, repo.Path)
			}
		}
		if len(worktrees) > 0 {
			orphaned = true
			sort.Strings(worktrees)
			report.add("orphan/"+state.Branch, "warn", fmt.Sprintf("root %s is missing; business worktrees remain: %v", state.Root, worktrees))
		}
	}
	if !orphaned {
		report.check("orphan-changes", "no orphaned business worktrees", nil)
	}
	return nil
}
