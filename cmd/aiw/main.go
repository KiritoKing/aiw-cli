package main

import (
	"fmt"
	"os"

	"github.com/KiritoKing/aiw-cli/internal/aiw"
)

func main() {
	if err := aiw.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "aiw:", err)
		os.Exit(1)
	}
}
