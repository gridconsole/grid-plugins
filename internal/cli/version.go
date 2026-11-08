package cli

import (
	"fmt"
	"time"

	"github.com/gridconsole/grid-plugins/internal/version"
)

func runVersion(args []string, env Env) int {
	fs := newFlags("version", env)
	asJSON := fs.Bool("json", false, "print JSON")
	check := fs.Bool("check", false, "check the release manifest (advisory)")
	address := fs.String("manifest-url", env.getenv("GRID_RELEASE_MANIFEST_URL"), "release manifest URL (or GRID_RELEASE_MANIFEST_URL)")
	timeout := fs.Duration("timeout", 2*time.Second, "release check timeout")
	app := fs.String("app-version", env.getenv("GRID_APP_VERSION"), "installed Grid app version (or GRID_APP_VERSION)")
	session := fs.String("session-id", env.getenv("GRID_SESSION_ID"), "session for the app warning (or GRID_SESSION_ID)")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *timeout <= 0 {
		fmt.Fprintln(env.Stderr, "version: timeout must be positive")
		return 2
	}
	if !*check {
		return printVersion(*asJSON, env)
	}
	report := struct {
		Version  string              `json:"version"`
		Contract string              `json:"contract"`
		AppMin   string              `json:"app_min"`
		Check    version.CheckResult `json:"check"`
		Warning  string              `json:"warning,omitempty"`
	}{Version: version.Version, Contract: version.Contract, AppMin: version.AppMin,
		Check:   version.Check(version.CheckOptions{URL: *address, GridHome: env.GridHome, Current: version.Version, Timeout: *timeout}),
		Warning: version.AppWarning(env.GridHome, *session, *app)}
	if *asJSON {
		return writeJSON(env.Stdout, report)
	}
	fmt.Fprintf(env.Stdout, "grid %s\ncontract %s\napp_min %s\ncheck: %s\n", report.Version, report.Contract, report.AppMin, report.Check.Status)
	if report.Check.Release != nil {
		fmt.Fprintf(env.Stdout, "latest: %s\n", report.Check.Release.Version)
	}
	if report.Check.Stale {
		fmt.Fprintln(env.Stdout, "release check: offline; using stale cached result")
	}
	if report.Warning != "" {
		fmt.Fprintln(env.Stdout, report.Warning)
	}
	return 0
}

func printVersion(asJSON bool, env Env) int {
	if asJSON {
		return writeJSON(env.Stdout, struct {
			Version  string `json:"version"`
			Contract string `json:"contract"`
			AppMin   string `json:"app_min"`
		}{version.Version, version.Contract, version.AppMin})
	}
	fmt.Fprintf(env.Stdout, "grid %s\ncontract %s\napp_min %s\n", version.Version, version.Contract, version.AppMin)
	return 0
}
