# project.md: the landing rule and the environments

`project.md` sits in the project's Grid data folder, `<projects folder>/<project>/project.md`, never in a repository. A plain terminal session and the Grid app read it the same way: `scripts/project-md.mjs` here is the plugin's reader, `crates/grid-core/src/project_md.rs` is the app's, and both read the fixtures in `tests/fixtures/project-md/`.

The workflow order is: deploy DEV, wait for TEST, land, approve PROD, deploy PROD, clean up. The names DEV, TEST and PROD are the project's own environment names; the format fixes none of them.

## The landing rule

Frontmatter keys, as they have always been. Nothing is renamed, and a project.md without them loads unchanged.

| Key | Holds |
|---|---|
| `lands` | one of `Open a pull request`, `Draft pull request`, `Push the branch`, `Merge, don't push`, `Merge, then push`, `Ask each time`. The old `Commit and push` reads as `Merge, then push`. |
| `lands_from` | `setup` while setup filled `lands` in; a person's edit removes it |
| `deploys` | `Don't deploy`, `Deploy from this machine` or `CI/CD deploys` |
| `deploy_command` | the command `Deploy from this machine` runs |

## The environments

A body section, after the frontmatter, running to the next `## ` heading or the end of the file. A heading inside a code fence does not count. The section is Grid's own: the app rewrites it whole, and everything outside it stays byte for byte as it was.

```
## Environments

### staging
name: staging
kind: remote
where: https://staging.example.test
how: make deploy ENV=staging
runs: v2.8.1-rc2
test: curl -fsS https://staging.example.test/health
from: setup
approval: no
who:
  - Ana
secrets:
  - HEALTH_CHECK_KEY (missing)
steps:
  - {"wf":"Release","step":"Preview the result","text":"Deploy and check"}
```

- One `### <id>` block per environment, in file order. The id is a slug (`a-z`, `0-9`, dashes): it is what a gate's `env` and `item-record.mjs --env` name. A block with another heading, or with no `name`, is left out.
- `key: value` takes the text after the first colon to the end of the line, trimmed, as it is: no quotes, no escapes. Empty keys are left out. Keys, in the order Grid writes them: `name`, `kind` (`local`, `remote` or `cloud`), `where` (a host or a URL), `how` (the deploy command or the pipeline that deploys it), `runs`, `test` (the command that tests the connection), `from` (`setup` when setup added it), `approval` (`yes` when a person must approve a deploy there, `no` when not; absent means the project's gates decide).
- `who:` is a list of names, one `  - name` line each: the people who may deploy there.
- `secrets:` is a list of `  - NAME (set)` or `  - NAME (missing)`: names only, never a value.
- `steps:` is a list of workflow steps that may use it, one JSON object per line, `  - {"wf":"..","step":"..","text":".."}` (`text` optional), so a comma or a colon in a name cannot break the line.
- A section with no block is an empty list. A project.md with no `## Environments` section is "not migrated": the reader returns `null`, and the environments are the project's `environments.json` (the Grid app copies that list into the section on the first environment edit and then reads only the section).

## Reading it

`node "${CLAUDE_PLUGIN_ROOT}/skills/deliver/scripts/project-md.mjs" <project.md> landing` prints `{lands, deploys, deployCommand}`; `... environments` prints the list, or `null` when the file has no section.
