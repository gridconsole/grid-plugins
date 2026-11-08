# Lane L11: eval loop harness

Built `internal/eval` with a strict versioned task-suite format, fresh temporary
workspace and home per attempt, configurable agent argv, a default embedded fake
agent, file/command/literal-text checks, an injectable LLM judge interface,
separate gates and weighted check scores, bounded process output, timeouts,
cleanup, atomic JSON results and summary tables. The three sample tasks fix a
calculation, write release notes and clean a CSV.

`internal/evalcmd` implements `grid eval list`, `run` and `show`, JSON output,
tier/size selection and failure exit codes. `internal/cli/eval.go` adapts the CLI
environment; the only edit to `internal/cli/cli.go` is its eval table registration.
No changes were made to `internal/where`, `internal/project`, `go.mod` or the
Makefile. Usage and the complete format are in `evals/README.md`.

Run `make test` and `make lint` for the required checks (`make test` runs
`go test ./...`). Run `go test ./internal/eval ./internal/evalcmd` for focused tests.
Try `go run ./cmd/grid eval run`, then
`go run ./cmd/grid eval show <run-id>` with an id from the summary table.
Generated results default to the ignored `evals/runs` directory.

## Intended differences and reference lines

References below are relative to the supplied read-only reference collection.

| Difference | Reference and reason |
| --- | --- |
| Native Go commands replace a separate runtime-specific eval invocation; no app or worker is required. | `old-plugin-map.md:118` lists `grid eval` as new; `docs/plugin-design.md:346` requires an independent plugin track. |
| Versioned JSON metadata and checks, `prompt.md`, and `setup.sh` replace the old `case.yaml`, `fixture.sh`, and grader Markdown format. There is no automatic migration. | `old-plugin/evals/agent-reviewer-read-only/case.yaml:1` defines schema 1.1 and the scaffold at line 5; its `fixture.sh:1` is Bash. JSON permits strict standard-library-only decoding; new setup scripts use POSIX shell. |
| Three portable demonstration tasks replace neither the old 37 cases nor the proposed full corpus. A deterministic, visibly labelled fake agent is the default; real commands are explicit configuration. | `docs/plugin.md:116` counts the old cases; `docs/plugin-design.md:366` describes tier b and `docs/plugin-design.md:382`, `:393`, `:395` describe calculation-fix, release-writing and CSV task categories. Samples test the harness without model spend. |
| Agents receive prompts through stdin or an explicit argv placeholder. The harness runs configured commands directly instead of depending on `claude plugin eval`; Claude and Codex share checks. | `docs/plugin-design.md:366` specifies the Codex arm and `:369` describes the limitations of `claude plugin eval`. Runtime authentication stays external. |
| Checks support regular-file existence, literal text, exit status and a Go judge hook. Old regex and prose graders are not imported. | `old-plugin/evals/agent-reviewer-read-only/graders/code-untouched.md:2` uses regex; `old-plugin/evals/plan-skill/graders/step-reasons.md:2` uses an LLM rubric. The requested initial checks remain dependency-free; a missing judge is an explicit error. |
| Fresh local fixtures and a temporary home are used rather than cloning the five proposed project repositories or seeding their full brains. Setup may seed its temporary Grid home. | `docs/plugin-design.md:372` specifies the larger fixture set; `:378` requires cleanup after result persistence. Local samples are self-contained and do not require external repositories. |
| Results expose per-check evidence, code/judge attribution, gate counts and a weighted 0..5 check score; they do not fabricate the S1-S6 rubric or G1-G5 coverage. Errors and unmeasured scores remain explicit. | `docs/plugin-design.md:409` starts the gates/rubric table; `prototype/eval-run.html:176`, `:185`, `:203` distinguish scores, gates and evidence; `prototype/eval.html:297` shows unscored states. |
| Run JSON, unique ids, task hashes, timings and a CLI summary replace the prototype's interactive presentation for this lane. Raw output is bounded; workspace contents are discarded after saving. | `prototype/eval-run.html:240` shows run facts, `:243` gates, `:244` scores and `:246` digest; `docs/plugin-design.md:375` requires a persistent run record and `:378` requires throwaway-home cleanup. |
| Suite tier and task size are metadata and filters. Timeout bounds wall time; runtime versions, token budgets and monetary caps are not inferred or enforced. | `docs/plugin-design.md:365` defines levels, `:374` runtime/token caps and `:435` per-size budgets. Safe enforcement needs runtime-specific stream adapters, outside this generic command harness. |

## Not implemented and why

No paid Claude/Codex run was performed: this delivery verifies the harness with
the fake agent and configured local commands, without credentials or model spend.
The judge remains an interface as requested; cross-family selection and owner
calibration (`docs/plugin-design.md:375`) need a real judge implementation and
labelled calibration runs.

The full digest of Jobs, Feed posts, tool calls, topic injections, questions,
cost and changed files (`docs/plugin-design.md:375`), person stand-in (`:373`),
finding triage (`:376`), first-hour measurement (`:377`), R0-R5 comparisons
(`:425`), M1-M3b metrics and two-round exit bars (`:439`) need the other plugin
subsystems and real corpus evidence. This lane provides the run/check boundary
and persistent results, without claiming those measurements. There are no
`eval digest` or `eval codex` aliases; configured `eval run` is the initial entry
point. Scheduling and a web dashboard are outside the requested CLI harness.

Temporary workspaces are not a security sandbox. Scripts execute with the
caller's permissions and inherit environment variables except the documented
workspace/home overrides. Process-group cleanup is implemented for Linux and
macOS; other platforms require a POSIX shell and only cancel the direct process.
Text checks have a 1 MiB limit, and each stdout/stderr stream has a 64 KiB limit.

Validation passed with `make test` and `make lint`. In this managed environment,
verification used a writable temporary Go cache and ran outside the filesystem
sandbox: its synthetic `/tmp/.git` entry otherwise changes the existing
project-discovery goldens. No unrelated resolver changes were made to compensate.
