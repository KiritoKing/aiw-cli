package aiw

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func registerRepo(root, home, name, source string) error {
	cfg, err := loadProject(root)
	if err != nil {
		return err
	}
	repo, ok := cfg.Repos[name]
	if !ok {
		return fmt.Errorf("repo %q is not declared in aiw.yaml", name)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	source, err = gitRoot(source)
	if err != nil {
		return fmt.Errorf("source %s: %w", source, err)
	}
	remote, err := git(source, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("source %s has no origin remote: %w", source, err)
	}
	key := canonicalRemote(repo.Remote)
	if canonicalRemote(remote) != key {
		return fmt.Errorf("repo %s source origin does not match aiw.yaml remote", name)
	}
	registry, err := loadRegistry(home)
	if err != nil {
		return err
	}
	if old, exists := registry.Repos[key]; exists {
		if !gitOk(old.Store, "rev-parse", "--is-bare-repository") {
			return fmt.Errorf("registered store is unavailable: %s", old.Store)
		}
		fmt.Printf("reused %s: %s\n", name, old.Store)
		return nil
	}
	store := filepath.Join(home, "repos", hash(key)+".git")
	if _, err := os.Lstat(store); err == nil {
		bare, bareErr := git(store, "rev-parse", "--is-bare-repository")
		storedRemote, remoteErr := git(store, "remote", "get-url", "origin")
		if bareErr != nil || remoteErr != nil || bare != "true" || canonicalRemote(storedRemote) != key {
			return fmt.Errorf("unregistered store path already exists and cannot be adopted: %s", store)
		}
		registry.Repos[key] = RegistryEntry{Remote: repo.Remote, Source: source, Store: store}
		if err := saveRegistry(home, registry); err != nil {
			return err
		}
		fmt.Printf("adopted %s: %s\n", name, store)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(store), 0755); err != nil {
		return err
	}
	if _, err := git("", "clone", "--bare", "--local", source, store); err != nil {
		return err
	}
	if _, err := git(store, "remote", "set-url", "origin", repo.Remote); err != nil {
		return err
	}
	baseRef := "refs/remotes/origin/" + repo.Base
	if !gitOk(source, "rev-parse", "--verify", baseRef+"^{commit}") {
		baseRef = "refs/heads/" + repo.Base
	}
	if !gitOk(source, "rev-parse", "--verify", baseRef+"^{commit}") {
		return fmt.Errorf("source %s has no local baseline for %s; fetch it in the source or use aiw repo sync after registration", source, repo.Base)
	}
	if _, err := git(store, "fetch", "--no-tags", source, "+"+baseRef+":refs/remotes/origin/"+repo.Base); err != nil {
		return err
	}
	registry.Repos[key] = RegistryEntry{Remote: repo.Remote, Source: source, Store: store}
	if err := saveRegistry(home, registry); err != nil {
		return err
	}
	fmt.Printf("registered %s: %s\n", name, store)
	return nil
}

func storeFor(cfg ProjectConfig, registry Registry, name string) (string, error) {
	repo, ok := cfg.Repos[name]
	if !ok {
		return "", fmt.Errorf("unknown repo %q", name)
	}
	entry, ok := registry.Repos[canonicalRemote(repo.Remote)]
	if !ok {
		return "", fmt.Errorf("repo %s is not registered; run aiw repo register or aiw repo scan", name)
	}
	bare, err := git(entry.Store, "rev-parse", "--is-bare-repository")
	if err != nil || bare != "true" {
		return "", fmt.Errorf("invalid managed store for %s: %s", name, entry.Store)
	}
	return entry.Store, nil
}

func syncRepo(root, home, name string) error {
	cfg, err := loadProject(root)
	if err != nil {
		return err
	}
	r, err := loadRegistry(home)
	if err != nil {
		return err
	}
	names := []string{}
	if name != "" {
		names = append(names, name)
	} else {
		for item := range cfg.Repos {
			names = append(names, item)
		}
		sort.Strings(names)
	}
	for _, item := range names {
		store, err := storeFor(cfg, r, item)
		if err != nil {
			return err
		}
		base := cfg.Repos[item].Base
		_, err = git(store, "fetch", "origin", "+refs/heads/"+base+":refs/remotes/origin/"+base)
		if err != nil {
			return err
		}
		sha, err := refSHA(store, "refs/remotes/origin/"+base)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n", item, base, sha)
	}
	return nil
}

func scanRepos(root, home, scanRoot string) error {
	cfg, err := loadProject(root)
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		return fmt.Errorf("aiw.yaml declares no repos; add each repo's remote and base branch, then rerun aiw repo scan")
	}
	scanRoot, err = filepath.Abs(scanRoot)
	if err != nil {
		return err
	}
	matches := map[string][]string{}
	err = filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if path == home {
			return filepath.SkipDir
		}
		if entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
			remote, err := git(path, "remote", "get-url", "origin")
			if err == nil {
				matches[canonicalRemote(remote)] = append(matches[canonicalRemote(remote)], path)
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return err
	}
	names := make([]string, 0, len(cfg.Repos))
	for name := range cfg.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		paths := matches[canonicalRemote(cfg.Repos[name].Remote)]
		if len(paths) > 1 {
			return fmt.Errorf("multiple local checkouts match %s: %s", name, strings.Join(paths, ", "))
		}
	}
	matched := 0
	for _, name := range names {
		paths := matches[canonicalRemote(cfg.Repos[name].Remote)]
		if len(paths) == 1 {
			if err := registerRepo(root, home, name, paths[0]); err != nil {
				return err
			}
			matched++
		}
	}
	if matched == 0 {
		fmt.Printf("no aiw.yaml repos matched under %s\n", scanRoot)
	}
	return nil
}
