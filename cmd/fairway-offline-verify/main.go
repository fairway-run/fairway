package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/subashram/fairway/internal/offlinebundle"
)

func main() {
	if err := offlinebundle.RunVerifierCLI(os.Args[1:], os.Stdout, os.LookupEnv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
