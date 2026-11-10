# A Grid home at GRID_HOME and a small repo that must stay untouched.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
mkdir -p work/notes-app/src
printf '# Notes app\n' > work/notes-app/README.md
cat > work/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
];

export function listNotes() {
  return [...notes];
}
S
