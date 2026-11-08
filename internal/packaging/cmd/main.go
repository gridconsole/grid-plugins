// Command packaging regenerates plugin metadata or builds a distribution.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gridconsole/grid-plugins/internal/packaging"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func main() {
	dist := flag.Bool("dist", false, "build all release platforms and the installable plugin")
	release := flag.String("version", version.Version, "release version from git describe")
	flag.Parse()
	root, err := os.Getwd()
	if err == nil {
		if *dist {
			err = packaging.Build(root, *release, os.Stdout)
		} else {
			err = packaging.Write(root, *release)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
