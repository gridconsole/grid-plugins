// Command packaging regenerates plugin metadata or builds a distribution.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/packaging"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// deps are the outside calls run makes, so a test can stub them.
type deps struct {
	getwd    func() (string, error)
	describe func() ([]byte, error)
	build    func(root, version string, log io.Writer) error
	write    func(root, version string) error
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, deps{
		getwd:    os.Getwd,
		describe: gitDescribe,
		build:    packaging.Build,
		write:    packaging.Write,
	}))
}

// run is main without the process exit: it returns the exit code.
func run(args []string, stdout, stderr io.Writer, d deps) int {
	fs := flag.NewFlagSet("packaging", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dist := fs.Bool("dist", false, "build all release platforms and the installable plugin")
	release := fs.String("version", version.Version, "release version")
	describe := fs.Bool("describe", false, "print the semver version for this checkout from git describe and exit")
	releaseVersion := fs.Bool("release", false, "print the publish version 0.2.<commit count> for this checkout and exit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *describe {
		fmt.Fprintln(stdout, checkoutVersion(d.describe))
		return 0
	}
	root, err := d.getwd()
	if err == nil && *releaseVersion {
		var v string
		if v, err = packaging.ReleaseVersion(root); err == nil {
			fmt.Fprintln(stdout, v)
			return 0
		}
	}
	if err == nil {
		if *dist {
			err = d.build(root, *release, stdout)
		} else {
			err = d.write(root, *release)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func gitDescribe() ([]byte, error) {
	return exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
}

// checkoutVersion falls back to the source default outside git or when describe is unreadable.
func checkoutVersion(describe func() ([]byte, error)) string {
	out, err := describe()
	if err != nil {
		return version.Version
	}
	v, err := version.FromDescribe(strings.TrimSpace(string(out)))
	if err != nil {
		return version.Version
	}
	return v
}
