// Package gridplugins holds the plugin files that code must carry with it.
package gridplugins

import _ "embed"

// OrchestratorAgent is agents/orchestrator.md: what a Job of role orchestrator
// tells the agent that runs it. The Job render copies its body into the prompt,
// so a worker that starts claude without --agent still runs the orchestrator.
//
//go:embed agents/orchestrator.md
var OrchestratorAgent string
