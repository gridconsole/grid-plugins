# C-kb-readme-found-roles proof table

| Row | Status | Proof |
|---|---|---|
| Base: found-worker-role on main | HOLDS (indirect) | The `git merge-base --is-ancestor` command on the ref repo was refused in this sandbox. In the clone, contract/schemas/found.json:19-25 already lists reviewer, lead, worker, agent, cli. |
| 1. README diff is the found sentence and one added line | HOLDS | `git diff --stat`: contract/README.md, 2 insertions, 1 deletion. Sentence now at :262 (not :261, main has moved); new line after :353 (not :351). `go test -count=1 ./internal/contract/` exit 0. |
| 2. `make golden` | HOLDS | Exit 0, no failure shown. |
| 3. `make lint` and `make cross` | HOLDS | `make lint` (go vet ./...) exit 0; `make cross` exit 0 (windows/amd64 vet and test compile, package tests ok). The darwin target was not looked at separately; cross is the make target. |
| Tests of changed packages and importers | HOLDS | Only a markdown file changed. `go test ./internal/contract/` exit 0. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok. `go test -count=1 ./internal/job/lint/` ok. |
| Only Touches file changed | HOLDS | `git diff --stat` shows contract/README.md only. This report file is the required docs/lanes entry. |
| No guard, gate, cap | HOLDS | Docs only. |
| RealPlugin tests | n/a | No row names them. |

## What changed
contract/README.md:262 now reads "reviewer, lead, cli, worker and agent may post found". A new line after :353 says a Job agent posts found under its own Job with `grid job post --job <its Job> --type found`.

## NOT VERIFIED
I did not check the claim against internal/cli/orchestrator.go:245-249. I took the post path from the brief.
