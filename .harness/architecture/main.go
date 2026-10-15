package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <repo-root>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}
	checked, err := Check(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("architecture check passed: %d Go files across %d layers\n", checked.files, checked.layers)
}
