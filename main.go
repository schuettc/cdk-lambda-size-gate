// Command cdk-lambda-size-gate fails CI before an oversized zip Lambda
// reaches deploy. See README.md.
package main

import (
	"os"

	"github.com/schuettc/cdk-lambda-size-gate/internal/gate"
)

func main() {
	os.Exit(gate.Run(os.Args[1:], os.Stdout, os.Stderr))
}
