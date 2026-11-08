// Command grid is the Grid command line: every write to task folders, the Feed and the brain.
package main

import (
	"os"

	"github.com/gridconsole/grid-plugins/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.OSEnv()))
}
