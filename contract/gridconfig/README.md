# Grid config fixtures

Each folder is one case of `~/.grid/config.json` and the data folder it resolves to.
`config.json` is the file as it sits in `~/.grid` (no file: the case has no config);
`expect.json` holds what `gridhome.Resolve` gives for it with no `GRID_HOME` and no flag:
`data` (the data folder), `source` (`config`, `legacy` or `default`) and `member` (the
current workspace's member, "" for none). `env` names the variables the case sets besides
`HOME` (the login for the v1 name rule).

`~` in both files stands for the user's home folder; a test replaces it before reading.

The owner's own `config.json` was not readable where these were written, so
`v2-owner` is a hand-written, redacted sample in the shape the earlier desktop wrote
(`v` 2, `workspaces[]`, `current`), not a copy of that file.

The app's resolver reads the same cases and must give the same folder and source.
