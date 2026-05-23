package main

import (
	"fmt"
	"os"

	"github.com/zdim/mdv/tui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mdv <file.md>")
		os.Exit(2)
	}
	path := os.Args[1]
	if err := tui.Run(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
