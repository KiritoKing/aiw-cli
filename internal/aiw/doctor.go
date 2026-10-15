package aiw

import (
	"fmt"
	"os"

	"github.com/KiritoKing/aiw-cli/internal/gitcli"
)

func runDoctor() error {
	version, err := gitcli.Run("", "--version")
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
