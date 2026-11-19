#!/bin/sh
# Keeps the public repo (remote github) in sync with main: one "Publish <version> from <sha>." commit per change.
# PUBLISH_PUSH=0 commits in .cache/publish without pushing.
set -eu

root=$(git rev-parse --show-toplevel)
cd "$root"

url=https://github.com/gridconsole/grid-plugins.git
clone=$root/.cache/publish

die() {
	echo "publish: $*" >&2
	exit 1
}

# 1. Source state: clean tree on main, or on fix-run-task at the same sha.
[ -z "$(git status --porcelain)" ] || die "the working tree is not clean"
branch=$(git rev-parse --abbrev-ref HEAD)
sha=$(git rev-parse HEAD)
main_sha=$(git rev-parse --verify -q refs/heads/main || true)
case "$branch" in
main) ;;
fix-run-task) [ "$sha" = "$main_sha" ] || die "fix-run-task is not at main's sha" ;;
*) die "run it on main (or on fix-run-task at main's sha), not on $branch" ;;
esac
short=$(git rev-parse --short=8 HEAD)

make dist
[ -d dist/plugin ] || die "make dist did not write dist/plugin"
version=$(jq -r .version dist/plugin/.claude-plugin/plugin.json)
[ -n "$version" ] && [ "$version" != null ] || die "no version in dist/plugin/.claude-plugin/plugin.json"

# 2. Clone of the public main, reset to github/main.
if [ ! -d "$clone/.git" ]; then
	mkdir -p "$root/.cache"
	git clone --quiet --no-checkout "$url" "$clone"
fi
git -C "$clone" remote set-url origin "$url"
git -C "$clone" fetch --quiet origin main
git -C "$clone" checkout --quiet -B main origin/main
git -C "$clone" reset --quiet --hard origin/main
git -C "$clone" clean --quiet -fd

# 3. Replace plugins/grid and stamp the root marketplace.json.
rm -rf "$clone/plugins/grid"
mkdir -p "$clone/plugins/grid"
cp -R dist/plugin/. "$clone/plugins/grid/"
market=$clone/.claude-plugin/marketplace.json
[ -f "$market" ] || die "the public repo has no .claude-plugin/marketplace.json"
jq --arg v "$version" '.plugins |= map(.version = $v)' "$market" >"$market.new"
mv "$market.new" "$market"

# 4. Refuse a wrong root layout or private text.
layout=
for entry in $(cd "$clone" && find . -mindepth 1 -maxdepth 1 ! -name .git | sed 's|^\./||' | LC_ALL=C sort); do
	layout="$layout$entry "
done
want=".claude-plugin .gitignore LICENSE README.md plugins "
[ "$layout" = "$want" ] || die "root layout is '$layout', want '$want'"

pattern='/Users/|/home/|/srv/gridwork|gmail|[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|sk-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_|xox[abprs]-|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY|(api[_-]?key|secret|token|password)["'"'"' ]*[:=]["'"'"' ]*[A-Za-z0-9+/_-]{24,}'
if hits=$(grep -rIEn --exclude-dir=.git "$pattern" "$clone"); then
	echo "$hits" | head -20 >&2
	die "private-looking text in the files above, nothing published"
fi

# 5. Commit and push.
git -C "$clone" add -A
if git -C "$clone" diff --cached --quiet; then
	echo "publish: nothing to publish, github main already holds $version"
	exit 0
fi
msg="Publish $version from $short."
git -C "$clone" -c user.name="$(git config user.name)" -c user.email="$(git config user.email)" commit --quiet -m "$msg"
echo "publish: committed '$msg'"
if [ "${PUBLISH_PUSH:-1}" = 0 ]; then
	echo "publish: PUBLISH_PUSH=0, not pushing"
	exit 0
fi
git -C "$clone" -c http.postBuffer=524288000 push origin main
echo "publish: pushed $version"
