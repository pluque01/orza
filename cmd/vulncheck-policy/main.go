package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pluque01/orza/internal/tools/vulnpolicy"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, time.Now))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, now func() time.Time) int {
	flags := flag.NewFlagSet("vulncheck-policy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	exceptionPath := flags.String("exceptions", ".vulnerability-exceptions.json", "path to vulnerability exception policy")
	date := flags.String("date", "", "policy date in YYYY-MM-DD (for deterministic automation)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "vulnerability policy: unexpected positional arguments")
		return 2
	}

	policyTime := now().UTC()
	if *date != "" {
		parsed, err := time.Parse(time.DateOnly, *date)
		if err != nil || parsed.Format(time.DateOnly) != *date {
			fmt.Fprintln(stderr, "vulnerability policy: -date must use YYYY-MM-DD")
			return 2
		}
		policyTime = parsed
	}

	exceptions, err := os.Open(*exceptionPath)
	if err != nil {
		fmt.Fprintln(stderr, "vulnerability policy: exception file could not be opened")
		return 1
	}
	defer exceptions.Close()

	if err := vulnpolicy.Evaluate(stdin, exceptions, policyTime, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
