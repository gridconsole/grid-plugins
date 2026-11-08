// Package contractdata holds the canonical contract assets for the Go runtime.
package contractdata

import "embed"

// Schemas contains the published JSON Schema files, without generated copies.
//
//go:embed schemas/*.json
var Schemas embed.FS
