# The lib Call

`lib.Run(lib.Call)` runs one grid command inside the caller's process. The output on Stdout and Stderr and the exit code are the ones the grid binary gives for the same arguments, environment and Grid home (lib/lib.go:1-3, lib/lib.go:120-125). A call takes its environment, working folder, home folder and streams from the Call and not from the host process (lib/lib.go:7-9).

Page base: plugin main `dc301fce`. Every statement cites `lib/lib.go:<line>` at that base. `contract/t12_lib_call_test.go` checks the field table, the exit codes, the example and the citations.

## Call fields

| Field | Type | Meaning | Empty or nil means |
| --- | --- | --- | --- |
| `Args` | `[]string` | The command line after the program name, such as `{"feed", "state", "--json"}`; the global flags `--member`, `--role`, `--lease-epoch` and `--grid-home` go at the front (lib/lib.go:73-74, lib/lib.go:12-15). | Not stated in the comment; it is an empty command line (lib/lib.go:74, lib/lib.go:138). |
| `GridHome` | `string` | The Grid data folder. It wins over `GRID_HOME` in Environ (lib/lib.go:76-77, lib/lib.go:144-146). | Reads `GRID_HOME` from Environ, then falls back to `.grid` under Home (lib/lib.go:77-78, lib/lib.go:18-19). |
| `Home` | `string` | The person's home folder, used to shorten paths and to find the default Grid home (lib/lib.go:79-80). | Reads `HOME` from Environ, or `USERPROFILE` on Windows (lib/lib.go:80, lib/lib.go:149-156). |
| `Dir` | `string` | The folder the command resolves its project and task from (lib/lib.go:82-83, lib/lib.go:10-11). | The Grid home (lib/lib.go:83-84, lib/lib.go:166-169). |
| `Environ` | `[]string` | The complete environment the command sees, as `KEY=VALUE` entries. The last entry of a name wins and names fold case on Windows (lib/lib.go:86-88, lib/lib.go:8-9). The member, role and lease travel here as `GRID_MEMBER`, `GRID_ROLE`, `GRID_LEASE`, `GRID_LEASE_EPOCH`, `GRID_JOB` and the other `GRID_*` names (lib/lib.go:12-14). | An empty environment; the host's own is not read (lib/lib.go:88-89, lib/lib.go:9-10). |
| `Stdin` | `io.Reader` | What the command reads (lib/lib.go:91). | Empty input (lib/lib.go:91, lib/lib.go:171-173). |
| `Stdout` | `io.Writer` | Receives the command's output (lib/lib.go:91-92). | The output is discarded (lib/lib.go:92, lib/lib.go:174-176). |
| `Stderr` | `io.Writer` | Receives the command's error output and the line `grid: internal error` after a panic (lib/lib.go:91-92, lib/lib.go:136). | The output is discarded (lib/lib.go:92, lib/lib.go:126-129). |
| `Executable` | `string` | The path of the grid binary, which `doctor` reports and routine schedules; a host that is not grid sets it (lib/lib.go:41-43, lib/lib.go:97-98). | Asks the host process for its own path, right only when the host is grid (lib/lib.go:98-99, lib/lib.go:42-43). |
| `OpenURL` | `func(string) error` | What `grid open` calls with the task page address; the lib never starts the system browser or the app (lib/lib.go:45-47, lib/lib.go:100). | Does nothing and returns nil (lib/lib.go:100-101, lib/lib.go:177-180). |
| `Context` | `context.Context` | Stops the commands the binary stops on SIGINT: the streams and runs of `agent`, `env`, `land`, `job next` and `feed tick`, `worker next`, `eval` and `job verify`. A `feed post` or `feed handover` waiting on a Feed file lock stops with the context's error and writes nothing. When it is set no signal handler is installed in the host (lib/lib.go:103-109). | Installs the binary's signal handler. Every other verb runs to its end whatever the context (lib/lib.go:107-108, lib/lib.go:106-107). |
| `OwnerPID` | `int` | The process that `task new`, `task attach`, `lease take --hold` and `env up` record as the owner of the task lease or port claim, in place of the host's own process id (lib/lib.go:111-114, lib/lib.go:32-37). | Zero keeps the host's own process id (lib/lib.go:114-115, lib/lib.go:35-37). |
| `OwnerStart` | `time.Time` | The start time of the OwnerPID process. It is read only when OwnerPID is set (lib/lib.go:114-115, lib/lib.go:191-193). | Zero looks the start time up for OwnerPID; `env up` always looks it up (lib/lib.go:115, lib/lib.go:114-115). |

## What a call reads and what stays with the host

A call does not claim that nothing is ever read from the host (lib/lib.go:23). These four items stay with the host process, as in the binary (lib/lib.go:23-24):

- The user name (`USER` and the system account) names a new task's owner. When Environ has no `GRID_MEMBER` and Args no `--member`, it is also the member of a Feed write, and that case runs `git config user.email` in Dir with the host's environment (lib/lib.go:25-28).
- `internal/account` falls back to the host's environment when it is given no getenv, and the migrate plan reads `CLAUDE_PLUGIN_DATA` from the host (lib/lib.go:29-31).
- The process that `ports claim`, `env up` and the task lease record as their owner is the Call's OwnerPID when set, else the host's own process id. Off Linux and macOS `lease take` is refused before `--hold` is read (lib/lib.go:32-37).
- The children that land, agent and routine runs start take the host's environment, not Environ (lib/lib.go:38-39).

What a call does read from the Call alone: the member, role and lease, the Grid home and the home folder, and the temporary folders `TMPDIR`, `TMP` and `TEMP` from Environ, then `/tmp` and `/var/tmp`; the host's temporary folder is not asked for (lib/lib.go:12-21, lib/lib.go:221-231). Two calls with different Grid homes can run at the same time and do not see each other (lib/lib.go:15-16).

## Exit codes

`Run` returns the code the binary would (lib/lib.go:120-121).

| Code | Meaning |
| --- | --- |
| 0 | Done (lib/lib.go:51, lib/lib.go:120). |
| 1 | Failed (lib/lib.go:51, lib/lib.go:120-121). |
| 2 | Usage. A panic inside the command is also 2, with the one line `grid: internal error` on Stderr (lib/lib.go:51, lib/lib.go:55-56, lib/lib.go:130-137). |
| 3 | Rejected: stale lease, invalid record or intent claim token. Do not retry (lib/lib.go:51-52, lib/lib.go:121). |
| 4 | Disk full (lib/lib.go:52, lib/lib.go:121). |
| 5 | Taken: the intent is already claimed or done. Skip (lib/lib.go:52-53, lib/lib.go:121-122). |
| 130 | Stopped by the Call's Context, for `worker next`, `job next` and `feed tick`, as the binary on Ctrl-C. The other runs Context stops exit with their own failure code (lib/lib.go:53-55, lib/lib.go:122). |

## Example

Run `version --json` with a fixed environment and Grid home (lib/lib.go:72-74, lib/lib.go:76-78, lib/lib.go:86-89). The output on Stdout is a `cmd-version` document and the code is 0 (lib/lib.go:120-125).

```go
var out bytes.Buffer
code := lib.Run(lib.Call{
	Args:     []string{"version", "--json"},
	GridHome: "/home/example/.grid",
	Environ:  []string{"HOME=/home/example", "GRID_MEMBER=example"},
	Stdout:   &out,
})
if code != 0 {
	return fmt.Errorf("grid version: exit %d", code)
}
```
