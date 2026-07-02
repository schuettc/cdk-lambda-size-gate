package gate

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Run is the CLI entry point. Exit codes: 0 = OK/WARN, 1 = a function
// exceeds the hard limit, 2 = usage error or cdk.out missing.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cdk-lambda-size-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cdkOut := fs.String("cdk-out", "cdk.out",
		"path to the synthesized cdk.out directory")
	hard := fs.Int64("hard-limit", DefaultHardLimitBytes,
		"FAIL above this many unzipped bytes (AWS limit: 262144000)")
	warn := fs.Int64("warn-limit", DefaultWarnLimitBytes,
		"WARN above this many unzipped bytes")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Fail CI if any zip-packaged Lambda staged by `cdk synth` exceeds")
		fmt.Fprintln(stderr, "the AWS 250 MiB unzipped limit (function code + layers combined).")
		fmt.Fprintln(stderr, "\nUsage: cdk-lambda-size-gate [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fi, err := os.Stat(*cdkOut); err != nil || !fi.IsDir() {
		fmt.Fprintf(stderr,
			"ERROR: cdk.out directory not found at '%s'. Run `cdk synth` first (or pass -cdk-out).\n",
			*cdkOut)
		return 2
	}

	limits := Limits{Hard: *hard, Warn: *warn}
	assets, anyFail := Evaluate(*cdkOut, limits)
	fmt.Fprintln(stdout, FormatReport(assets, limits))
	for _, line := range Annotations(assets, limits) {
		fmt.Fprintln(stdout, line)
	}

	if anyFail {
		fmt.Fprintln(stderr, "\nBundle-size gate FAILED: trim the oversized Lambda(s) above before deploy.")
		return 1
	}
	fmt.Fprintln(stdout, "\nBundle-size gate passed.")
	return 0
}
