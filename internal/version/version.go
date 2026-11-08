// Package version holds the versions grid reports.
package version

// Version is the CLI release. The Makefile overrides it with -ldflags.
var Version = "0.1.0"

const (
	// Contract is the version of the --json command contract and the Feed contract.
	Contract = "1"
	// AppMin is the oldest Grid app this plugin works with.
	AppMin = "0.1.0"
)
