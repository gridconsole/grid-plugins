package cli

import (
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/migrate"
)

func runMigrate(args []string, env Env) int {
	fs := newFlags("migrate", env)
	dry := fs.Bool("dry-run", false, "show actions without writing (the default)")
	apply := fs.Bool("apply", false, "back up and apply the migration; stop old Grid writers first")
	from := fs.String("from", "", "old Grid data folder (defaults to the Grid home)")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *dry && *apply {
		fmt.Fprintln(env.Stderr, "grid migrate: --dry-run and --apply are mutually exclusive")
		return 2
	}
	if env.GridHome == "" {
		fmt.Fprintln(env.Stderr, "grid migrate: cannot find the Grid home; set GRID_HOME")
		return 1
	}
	source := *from
	if source == "" {
		source = env.GridHome
	} else if !filepath.IsAbs(source) && env.Cwd != "" {
		source = filepath.Join(env.Cwd, source)
	}
	p, err := migrate.Scan(source, env.GridHome, migrate.WithCeiling(env.Ceiling))
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
