# P-person-check: `grid person check`

The app runs `grid person check --json` once at startup and logged "grid person check not available (older plugin)" (the plugin answered `person: unknown command "check"`, exit 2) or "grid person check failed; approvals unavailable". The plugin now answers.

## The app caller (app ref main 53f6230, read only)

- `internal/kernel/plugincli/person_check.go:67` runs `person check --json` (the only spelling the app uses).
- `internal/kernel/plugincli/client.go:190-203` and `:489` turn exit 1 or 2 with "unknown command/verb/subcommand" naming person or check into "older plugin".
- `internal/kernel/plugincli/person.go:20` `PersonRefusal` accepts exactly exit 3 with stdout one object `{"error":"person_required","reason":"<non-empty>"}`.
- Exit 0 means approvals are available; the app decodes stdout as exactly one JSON value (trailing output is an error). Any other failure is `person_check_failed`.

## The contract built

- `grid person check [--json]`, no operands. Exit 0: `{"ok":true}` (without `--json`: `person check: ok`).
- Exit 3 for an agent: stdout (with `--json`) `{"error":"person_required","reason":R,"hint":H}`, stderr `grid person check: H (reason: R).` R is `agent_worker` (GRID_AGENT, GRID_WORKER_ID or GRID_WORKER is set) or `agent_role` (a GRID_ROLE whose tier is Agent in `contract.TierFor`).
- A person is: no role or `cli` (the local machine), or owner, member, lead, guest, external (with or without the `grid-` prefix). Guests are people: the app runs the plugin as GRID_ROLE=guest for a signed-in guest.
- Exit 2 for a stray operand or flag. An unknown `person` verb is unchanged.
- No new policy: the worker markers are the ones `agentShell` already reads (now shared as `workerMarked`), and the roles are the existing `contract.TierFor` tiers. The command reads the environment, writes nothing, and no other command consults it. It adds no guard, gate or hold on any agent; it only reports, as the app asks.

## Files

`internal/cli/person_check.go` (new), `person.go` (dispatch and help), `task_write.go` (`workerMarked` extracted from `agentShell`, same behaviour), `person_check_test.go` (new), `contract_inventory_test.go` (one annotation line for the new route), `contract/commands.json` (the new route, edited by hand because UPDATE_CONTRACT_COMMANDS=1 could not be set; the inventory test compares it byte for byte and passes).

## Checks

- New tests on base (person.go stashed): `TestPersonCheckAnswersPerson/*` 7 FAIL `exit 2: ... person: unknown command "check"`, `TestPersonCheckRefusesAgent/*` 5 FAIL the same, `TestPersonCheckUsage` FAIL `plain: exit 2 ""`. On the lane: all PASS.
- Tests per answer the caller handles: person (7 roles) = exit 0 one JSON value; agent (5 forms) = exit 3 person_required with reason; usage exit 2; older-plugin text kept for unknown verbs.
- `go test ./internal/cli/ ./cmd/grid/` (cmd/grid is the only importer of internal/cli): exit 0.
- `make test`: exit 0. `make lint` (go vet): exit 0. `GOOS=windows CGO_ENABLED=0 go build ./...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. `gofmt -l internal/cli`: empty.

## NOT VERIFIED

- Against the real app binary: the app's tests use fake runners; I did not run the app against this plugin.
- The refusal reason words `agent_worker` and `agent_role` are mine (the app shows any non-empty word); the owner may want other words.
- Whether the owner wants an agent-answering check at all: it blocks nothing in the plugin, but the app uses the answer to disable approvals.
