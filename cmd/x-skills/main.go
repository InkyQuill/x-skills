package main

import (
	"fmt"
	"io"
	"os"

	"github.com/InkyQuill/x-skills/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if err := cli.Execute(args, stdin, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}
