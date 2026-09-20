package aiw

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed completions/_aiw
var zshCompletion string

//go:embed completions/aiw.bash
var bashCompletion string

func runCompletion(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: aiw completion zsh|bash")
	}
	switch args[0] {
	case "zsh":
		_, err := fmt.Fprint(os.Stdout, zshCompletion)
		return err
	case "bash":
		_, err := fmt.Fprint(os.Stdout, bashCompletion)
		return err
	default:
		return fmt.Errorf("unsupported shell %q; use zsh or bash", args[0])
	}
}
