// Command packaging regenerates plugin metadata or builds a distribution.
package main

import (
	"fmt"
	"os"

	"github.com/gridconsole/grid-plugins/internal/packaging"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func main() {
	if err := packaging.Write(".", version.Version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
