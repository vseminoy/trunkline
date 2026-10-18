// Command trunkline installs and maintains the trunkline development pipeline
// inside a project.
package main

import (
	"os"

	"github.com/vseminoy/trunkline/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
