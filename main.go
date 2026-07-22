package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `gho — launch your working environment anywhere

usage:
  gho launch   fetch, unlock, and materialize a profile into an ephemeral shell
  gho pack     edit the payload, re-seal the vault, and push it
`

// dispatch routes a subcommand to its entry point and returns the process exit
// code. It is the whole CLI surface for ticket 01.
func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "launch":
		return runLaunch(args[1:], stdout, stderr)
	case "pack":
		return runPack(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "gho: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}
