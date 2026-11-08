package contractdata

import _ "embed"

// Commands describes the CLI surface and its versioned JSON outputs.
//
//go:embed commands.json
var Commands []byte
