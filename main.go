package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage())
		os.Exit(1)
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Println("tmux-agent " + version)
		return
	case "--help", "-h", "help":
		fmt.Println(usage())
		return
	}

	args, handled := parseGlobalFlags(args)
	if handled {
		return
	}

	if err := runSubcommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
