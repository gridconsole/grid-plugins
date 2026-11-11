package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/migrate"
)

func runMigrate(args []string, env Env) int {
	fs := newFlags("migrate", env)
	dry := fs.Bool("dry-run", false, "show actions without writing (the default)")
	apply := fs.Bool("apply", false, "back up and apply the migration; stop old Grid writers first")
	rollback := fs.Bool("rollback", false, "restore the old home if the migrated home is unchanged")
	from := fs.String("from", "", "old Grid data folder (defaults to the Grid home)")
	pluginData := fs.String("plugin-data", "", "old plugin data folder whose session pointers are migrated (defaults to CLAUDE_PLUGIN_DATA)")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *rollback && (*dry || *apply || *from != "" || *pluginData != "") {
		fmt.Fprintln(env.Stderr, "grid migrate: --rollback cannot be combined with --apply, --dry-run, --from or --plugin-data")
		return 2
	}
	if *dry && *apply {
		fmt.Fprintln(env.Stderr, "grid migrate: --dry-run and --apply are mutually exclusive")
		return 2
	}
	if env.GridHome == "" {
		fmt.Fprintln(env.Stderr, "grid migrate: cannot find the Grid home; set GRID_HOME")
		return 1
	}

	if *rollback {
		res, err := migrate.Rollback(env.GridHome)
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid migrate: %v\n", err)
			return 1
		}
		if res.Rerun {
			fmt.Fprintln(env.Stdout, "migrate: interrupted rollback preparation discarded; run grid migrate --rollback again")
			return 0
		}
		format, err := migrate.RestoredFormat(env.GridHome)
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid migrate: %v\n", err)
			return 1
		}
		if _, err := fmt.Fprintf(env.Stdout, "restored %s\nformat: %s\n", res.Backup, format); err != nil {
			return 1
		}
		return 0
	}
	source := absolute(*from, env)
	if source == "" {
		// No --from: a Grid home that is missing or empty is a fresh one.
		if entries, err := os.ReadDir(env.GridHome); os.IsNotExist(err) || (err == nil && len(entries) == 0) {
			fmt.Fprintln(env.Stdout, "migrate: nothing to migrate")
			return 0
		}
		source = env.GridHome
	}
	opts := []migrate.Option{migrate.WithCeiling(env.Ceiling)}
	if *pluginData != "" {
		opts = append(opts, migrate.WithPluginData(absolute(*pluginData, env)))
	}
	p, err := migrate.Scan(source, env.GridHome, opts...)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid migrate: %v\n", err)
		return 1
	}
	if err := p.WriteText(env.Stdout); err != nil {
		fmt.Fprintf(env.Stderr, "grid migrate: %v\n", err)
		return 1
	}
	if *apply {
		res, err := p.Apply()
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid migrate: %v\n", err)
			return 1
		}
		switch {
		case res.Rerun:
			fmt.Fprintln(env.Stdout, "migrate: the interrupted run was undone; run grid migrate --apply again")
		case res.Changed:
			if _, err := fmt.Fprintf(env.Stdout, "backup: %s\nmigrate: applied\n", res.Backup); err != nil {
				return 1
			}
		}
	} else if len(p.Conflicts) > 0 || len(p.Unmapped) > 0 {
		return 1
	}
	return 0
}

// absolute resolves a path given on the command line against the working folder.
func absolute(dir string, env Env) string {
	if dir != "" && !filepath.IsAbs(dir) && env.Cwd != "" {
		return filepath.Join(env.Cwd, dir)
	}
	return dir
}
