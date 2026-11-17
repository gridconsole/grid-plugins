# Grid config fixtures

Each folder is one case of the shared Grid config and the folders it resolves to.

Where the files sit:
- `config.json` is the file at the config path: `~/.config/grid/config.json` on macOS and Linux,
  `%APPDATA%\Grid\config.json` on Windows. `XDG_CONFIG_HOME` is not read.
- `old-config.json` is the older file at `~/.grid/config.json`. It is read only when the config
  path has no file; the resolver then copies it once, byte for byte, to the config path and leaves
  the old file in place.
- No file: the case has no config.

`expect.json` holds what `gridhome.Resolve` gives for the case with no `GRID_HOME` and no flag:
- `data`: the data folder (tasks, projects, accounts, usage).
- `source`: `config`, `legacy` (the v1 top-level keys, migrated in memory) or `default`.
- `member`: the member the config names for the workspace, "" for none. When the config names
  none, setup offers the Claude account's `oauthAccount.displayName`, else the OS login name;
  that default is not part of these cases.
- `brain`: the workspace's `brain` folder, "" when the config names none (the brain stays in the
  data folder).
- `machine`: the machine folder, "" for the OS standard place: `~/Library/Application Support/Grid`
  on macOS, `~/.local/state/grid` on Linux, `%LOCALAPPDATA%\Grid` on Windows (`XDG_STATE_HOME` is
  not read). The top-level `machineFolder` key names another absolute folder.
- `copied`: true when the resolve copied `old-config.json` to the config path.
- `env`: the variables the case sets besides `HOME` (the login for the v1 name rule).

A missing key in `expect.json` means "" or false.

`~` in every file stands for the user's home folder; a test replaces it before reading.

The owner's own `config.json` was not readable where these were written, so `owner-v3` (the
older path, data in `~/.grid/<name>`) and `v2-owner` are hand-written, redacted samples, not
copies of that file.

The app's resolver reads the same cases and must give the same folders and source.
