// Command packaging regenerates plugin metadata or builds a distribution.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/packaging"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func main() {
	dist := flag.Bool("dist", false, "build all release platforms and the installable plugin")
	release := flag.String("version", version.Version, "release version")
	describe := flag.Bool("describe", false, "print the semver version for this checkout from git describe and exit")
	releaseVersion := flag.Bool("release", false, "print the publish version 0.1.<commit count> for this checkout and exit")
	flag.Parse()
	if *describe {
		fmt.Println(checkoutVersion())
		return
	}
	root, err := os.Getwd()
	if err == nil && *releaseVersion {
		var v string
		if v, err = packaging.ReleaseVersion(root); err == nil {
			fmt.Println(v)
			return
		}
	}
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

// checkoutVersion falls back to the source default outside git or when describe is unreadable.
func checkoutVersion() string {
	out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		return version.Version
	}
	v, err := version.FromDescribe(strings.TrimSpace(string(out)))
	if err != nil {
		return version.Version
	}
	return v
}
