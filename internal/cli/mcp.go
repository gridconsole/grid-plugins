package cli

import (
	"fmt"
	"os"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/mcp"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func runMCP(args []string, env Env) int {
	fs := newFlags("mcp", env)
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	server := mcp.Server{
		Version: version.Version,
		Resolve: func(task, project string) (feed.Store, error) { return sessionStore(env, task, project) },
		Session: func(in feed.Input) (feed.Input, feed.Writer, error) {
			if err := refuseAnswerPost(in.Type); err != nil {
				return in, feed.Writer{}, err
			}
			in, err := sessionDefaults(in)
			if err != nil {
				return in, feed.Writer{}, err
			}
			lease := os.Getenv("GRID_LEASE")
			if in.Type != "" { // feed_handover posts no type
				if err := refuseJobSide(in.Type, lease); err != nil {
					return in, feed.Writer{}, err
				}
			}
			writer, err := feedWriter(env, lease, os.Getenv("GRID_LEASE_EPOCH"))
			return in, writer, err
		},
	}
	if err := server.Serve(env.Stdin, env.Stdout); err != nil {
		fmt.Fprintf(env.Stderr, "grid mcp: %v\n", err)
		return 1
	}
	return 0
}
