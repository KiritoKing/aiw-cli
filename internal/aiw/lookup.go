package aiw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
)

func changeLookup(action, name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := projectRoot(cwd)
	if err != nil {
		return err
	}
	project, err := gitcli.CommonDir(root)
	if err != nil {
		return err
	}
	home, err := dataHome()
	if err != nil {
		return err
	}
	if action == "list" {
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
	s, err := loadState(home, project, name)
	if err != nil {
		return err
	}
	if s.Root == "" {
		return fmt.Errorf("Change %s not registered on this machine", name)
	}
	if _, err := os.Stat(s.Root); err != nil {
		return fmt.Errorf("control-plane worktree unavailable: %s", s.Root)
	}
	fmt.Println(s.Root)
	return nil
}
