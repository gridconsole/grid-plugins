# evals/suite

These are eval cases in the format `claude plugin eval` reads. They sit beside the Go harness: the older cases are in `evals/tasks` and run with `go run ./cmd/grid eval list|run|show` (see `evals/README.md`). Each folder here is one case. Which of the 19 original cases run in full, in part or not at all, and why, is in [evals/builtin-fit.md](../builtin-fit.md).

## Run one case

Always give `--eval-dir evals/suite`. Without it the command reads `evals/` and takes every folder with a `prompt.md` as a case, including `evals/tasks`, which have no graders.

```
claude plugin eval <plugin folder> --eval-dir evals/suite --case research-unverified --runs 1 --scaffold --allow-tools Bash Write Edit --trust-plugin --no-publish --ablation none --json research-unverified.json
```

`research-unverified` is only the example. Use any case folder name.

- `--eval-dir evals/suite`: read cases from this folder only, and write `results/` here.
- `--case <name>`: run only the case with this name. The name is the folder name. It is a glob.
- `--runs 1`: one run per case. The case default is 3.
- `--scaffold`: run the case's fixture script before the agent starts. Without it the fixture never runs.
- `--allow-tools Bash Write Edit`: the tools the agent may use. Anything not granted is removed from the session.
- `--trust-plugin`: needed when stdin or stdout is not a terminal, or with `--json`. Without it the run is refused with exit 1.
- `--no-publish`: keep `report.html` local, do not publish it as an artifact.
- `--ablation none`: run one arm (with the plugin) instead of two.
- `--json <file>`: write the result document to a file. The run prints no table.

On Linux a Bash grant needs `bubblewrap` and `socat` installed.

## What a run costs

A run costs money. Every run starts a real agent on your own login.

`--ablation with-without` runs a second arm without the plugin, so it doubles the runs. A path target does that by default, so `--ablation none` is what keeps one case at one run. A run of the whole suite (no `--case`) multiplies the cost by the number of cases. Run one case first.

## Where results go

Each run with at least one case writes `evals/suite/results/<timestamp>/` with `aggregate-result.json` and `report.html`. `--output-dir <dir>` moves them. `--json <file>` writes the result document to that file. `results/` is output, not a case: do not commit it.

Exit code: 0 when every case reaches the threshold, 1 for a low score, a load error or a refused trust, 2 for a partial run.

## Check a case without a run

Each case has a Go test that grades recorded transcripts with `GradeCase` (`internal/evalcmd/suitegrade.go`). It starts no agent and costs nothing:

```
go test ./internal/evalcmd/ -run 'TestHj'
```
