package main

import (
	"os"

	"github.com/InkyQuill/x-skills/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(2)
	}
}
