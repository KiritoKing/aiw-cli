package main

import (
	"fmt"
	"os"

	"github.com/KiritoKing/aiw-cli/internal/commands"
)

func main() {
	if err := commands.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "aiw:", err)
		os.Exit(1)
	}
}
