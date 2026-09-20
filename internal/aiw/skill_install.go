package aiw

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	aiwskills "github.com/KiritoKing/aiw-cli/skills"
)

var aiwSkillNames = []string{"aiw-init", "aiw-reference"}

const skillCompatibilityMarker = "AIW-GO-WORKTREE-V1"

func installSkills(root string) error {
	if err := checkSkillParents(root); err != nil {
		return err
	}
	_, nodeErr := exec.LookPath("node")
	npx, npxErr := exec.LookPath("npx")
	if nodeErr != nil || npxErr != nil {
		fmt.Println("Node or npx unavailable; installing bundled AIW skills")
		return installBundledSkills(root)
	}
	for _, name := range aiwSkillNames {
		path := filepath.Join(root, ".agents", "skills", name)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("skill path already exists: %s; use --skip-skills and manage it with your installer", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	args := []string{"--yes", "skills", "add", "KiritoKing/aiw-cli", "--skill", "aiw-init", "--skill", "aiw-reference", "--agent", "universal", "--copy", "--yes"}
	cmd := exec.Command(npx, args...)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npx skills add failed: %w", err)
	}
	for _, name := range aiwSkillNames {
		path := filepath.Join(root, ".agents", "skills", name, "SKILL.md")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("npx completed without a regular %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(data, []byte(skillCompatibilityMarker)) {
			return fmt.Errorf("npx installed an outdated %s; publish the Go-worktree AIW skills before using the public source, or use --skip-skills and your own installer", name)
		}
	}
	return nil
}

func checkSkillParents(root string) error {
	for _, rel := range []string{".agents", filepath.Join(".agents", "skills")} {
		path := filepath.Join(root, rel)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("skill directory is not a regular directory: %s", path)
		}
	}
	return nil
}

func installBundledSkills(root string) error {
	type bundledFile struct {
		path string
		data []byte
	}
	var files []bundledFile
	for _, name := range aiwSkillNames {
		err := fs.WalkDir(aiwskills.Files, name, func(source string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			target := filepath.Join(root, ".agents", "skills", filepath.FromSlash(source))
			if entry.IsDir() {
				if info, err := os.Lstat(target); err == nil && !info.IsDir() {
					return fmt.Errorf("skill path is not a directory: %s", target)
				} else if err != nil && !os.IsNotExist(err) {
					return err
				}
				return nil
			}
			data, err := fs.ReadFile(aiwskills.Files, source)
			if err != nil {
				return err
			}
			if info, err := os.Lstat(target); err == nil {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("skill path is not a regular file: %s", target)
				}
				current, err := os.ReadFile(target)
				if err != nil {
					return err
				}
				if !bytes.Equal(current, data) {
					return fmt.Errorf("skill content conflicts at %s; existing file was preserved", target)
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			files = append(files, bundledFile{path: target, data: data})
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, file := range files {
		if existing, err := os.ReadFile(file.path); err == nil && bytes.Equal(existing, file.data) {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := atomicWrite(file.path, file.data); err != nil {
			return fmt.Errorf("install %s: %w", strings.TrimPrefix(file.path, root+string(filepath.Separator)), err)
		}
	}
	return nil
}
