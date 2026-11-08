The Feed contract: schemas, fixtures and the conformance suite; contract/commands.json (each command, arguments, output schema) comes later.

## Command output

Plain output of `where` is one `name: value` line per field, with the same names as `--json`; a list gives one line per entry under the field's name, an empty field is left out, and paths under the home folder start with `~`; `--json` paths are absolute. `doctor` prints one `ok|fail <name>: <detail>` line per check. `version` prints `grid <version>`, `contract <n>` and `app_min <version>`. The golden files in `testdata/golden/` hold an example of each.

Every command except `help`, `version`, `doctor` and `migrate` first checks the Grid home's format version (`<GRID_HOME>/VERSION`). A newer format exits 1 with `This data is format <n>; this grid reads up to <m>. Update grid.`, an older one with `This data is format <n>; this grid reads <m>. Run: grid migrate`. A `VERSION` that cannot be read exits 1 with `Cannot read the format version in <GRID_HOME>/VERSION. Run: grid doctor`. A home without `VERSION` passes. `grid hook` prints nothing and exits 0 instead.

### `grid version --json`

    {"version": string, "contract": string, "app_min": string}

### `grid where --json [--task <path or id>] [--project <name or id>]`

    {
      "project":   string,    // project folder name under <GRID_HOME>/projects, or "Personal"
      "id":        string,    // the record's id; "" without a record
      "rule":      "explicit" | "task" | "record" | "multi-repo" | "git" | "folder" | "personal",
      "folder":    string,    // the project folder on this machine, the current folder for Personal; "" when unknown
      "record":    bool,      // whether <GRID_HOME>/projects/<project>/project.md exists
      "repos":     [string],  // multi-repo: the repo folders found; else []
      "keys":      [string],  // match keys: common:<git dir>, remote:<normalised url>, root:<commit>
      "task":      string,    // the task folder name; "" outside a task
      "task_path": string     // the task folder path; "" outside a task
    }

Exit 1 with one line on stderr when the folder cannot be resolved (a task path outside the Grid home, an unknown project or task, a project.md that does not parse).

### `grid doctor --json`

    {"ok": bool, "checks": [{"name": string, "ok": bool, "detail": string}]}

Checks, in order: `grid home`, `grid home writable`, `format version`, `git`. Exit 1 when any check fails.

### `grid hook <event>`

Reads the event JSON on stdin and always exits 0. `sessionstart` prints

    {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": string}}

where `additionalContext` names the project or task `grid where` resolved for the event's `cwd`. Other events print nothing yet.
