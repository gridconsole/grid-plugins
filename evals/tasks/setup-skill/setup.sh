# A Grid home, a repo with CI deploys, a Makefile test target and merge commits, and one Claude Code session for it.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
repo="$PWD/work/notes-app"
mkdir -p "$repo/.github/workflows" "$repo/src"
cd "$repo"
git init -q -b main
git config user.name "Alex"
git config user.email "alex@example.invalid"
printf '# Notes app\n\nA small notes list.\n' > README.md
printf 'test:\n\tnode --test\n' > Makefile
printf 'export const x = 1;\n' > src/store.js
cat > .github/workflows/deploy.yml <<'Y'
name: deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: make test
      - run: ./publish.sh
Y
git add -A
git commit -q -m "First version"
git checkout -q -b feature/tags
printf 'export const y = 2;\n' >> src/store.js
git commit -q -am "Add tags"
git checkout -q main
git merge -q --no-ff -m "Merge feature/tags" feature/tags
cd "$OLDPWD"
enc=$(printf '%s' "$repo" | sed 's#[/.]#-#g')
mkdir -p "$HOME/.claude/projects/$enc"
cat > "$HOME/.claude/projects/$enc/s1.jsonl" <<'J'
{"type":"user","message":{"role":"user","content":"Remember we always cut a release on Friday afternoon, from main. Here is my key for the registry: sk-test-SECRET123"}}
{"type":"assistant","message":{"role":"assistant","content":"Understood: releases go out on Friday afternoons from main."}}
J
