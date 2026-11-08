// Package migrate plans and applies offline migrations from the old named Grid
// homes into one versioned home. Planning never writes to disk.
package migrate

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// Action describes a source-relative to destination-relative file operation.
type Action struct{ Kind, From, To string }

// Plan is a read-only preview. Apply uses its private snapshot, and refuses if
// either input tree has changed since Scan. Conflicts and Unmapped block apply.
// Interrupted is set instead of a plan when an earlier run stopped part way; Apply
// then finishes or undoes that run.
type Plan struct {
	From, Home                 string
	Actions                    []Action
	Conflicts, Unmapped, Notes []string
	Current                    bool
	Interrupted                string

	source, destination snap
	result              tree
	records             []plannedRecord
	machine             string // this computer's id when the home or source has one, else made at apply
	ceiling             string
	recovery            *recovery
	sums                map[string]string
	absLinks            []string
	pluginData          string
	plugin              snap
	pointers            []pointerChange
}

// Option adjusts a scan.
type Option func(*Plan)

// WithCeiling stops the walk for a project's git repo at dir, as grid where does.
func WithCeiling(dir string) Option { return func(p *Plan) { p.ceiling = dir } }

// WithPluginData selects the old hook data folder (defaults to CLAUDE_PLUGIN_DATA).
func WithPluginData(dir string) Option { return func(p *Plan) { p.pluginData = dir } }

// pendingMachine stands in for the machine id in a plan made before one exists.
const pendingMachine = "MACHINE-ID-MADE-ON-APPLY"

// Scan accepts either an old Grid root containing named homes or a single old
// home containing projects/. Home is the new Grid home (normally ~/.grid).
func Scan(from, home string, opts ...Option) (*Plan, error) {
	if from == "" || home == "" {
		return nil, fmt.Errorf("source and Grid home are required")
	}
	from = gridhome.Real(from)
	home = gridhome.Real(home)
	if from != home && within(home, from) {
		return nil, fmt.Errorf("destination must not be inside the source")
	}
	p := &Plan{From: from, Home: home, sums: map[string]string{}, pluginData: os.Getenv("CLAUDE_PLUGIN_DATA")}
	for _, o := range opts {
		o(p)
	}
	if p.ceiling != "" {
		p.ceiling = gridhome.Real(p.ceiling)
	}
	rec, err := findRecovery(home)
	if err != nil {
		return nil, err
	}
	if rec != nil {
		p.recovery, p.Interrupted = rec, rec.text
		return p, nil
	}
	current, err := versionAt(home)
	if err != nil {
		return nil, err
	}
	if current {
		p.Current = true
		return p, nil
	}
	if p.destination, err = snapshot(home); err != nil {
		return nil, err
	}
	p.source = p.destination
	if from != home {
		if p.source, err = snapshot(from); err != nil {
			return nil, err
		}
	}
	if e, ok := p.destination.get("."); ok && !e.Mode.IsDir() {
		return nil, fmt.Errorf("Grid home is not a directory")
	}
	if e, ok := p.source.get("."); ok && !e.Mode.IsDir() {
		return nil, fmt.Errorf("source is not a directory")
	}
	if len(p.source.m) == 0 {
		return nil, fmt.Errorf("old Grid data folder does not exist: %s", from)
	}
	if current, err = p.sourceVersion(); err != nil {
		return nil, err
	} else if current && from != home {
		return nil, fmt.Errorf("source is already a current Grid home")
	}
	p.result = tree{}
	for k, v := range p.destination.m {
		v.Src = filepath.Join(home, filepath.FromSlash(k))
		p.result[k] = v
	}
	p.result["."] = entry{Mode: os.ModeDir | 0700}
	p.specialFiles()
	homes := []string{}
	if e, ok := p.source.get("projects"); ok && e.Mode.IsDir() {
		homes = append(homes, "")
	}
	for _, name := range children(p.source, "") {
		if e, ok := p.source.get(path.Join(name, "projects")); ok && e.Mode.IsDir() {
			homes = append(homes, name)
		}
	}
	if len(homes) == 0 {
		return nil, fmt.Errorf("no old projects folder found")
	}
	homeSet := map[string]bool{}
	for _, h := range homes {
		homeSet[h] = true
	}
	if from != home {
		for _, name := range children(p.destination, "") {
			if e, ok := p.destination.get(path.Join(name, "projects")); ok && e.Mode.IsDir() && !within(filepath.Join(home, name), from) {
				p.Conflicts = append(p.Conflicts, name+": old destination home is outside --from; migrate the whole home together")
			}
		}
	}
	p.machine = gridhome.Machine(home)
	if p.machine == "" && from != home {
		p.machine = gridhome.Machine(from)
	}
	handled := map[string]bool{".": true, "VERSION": true}
	occupied := map[string]bool{}
	for _, name := range children(p.destination, "projects") {
		occupied[name] = true
	}
	if from == home { // Direct projects are migrated in place, not suffixed against themselves.
		for _, name := range children(p.source, "projects") {
			delete(occupied, name)
		}
	}
	paths := map[string]string{}
	projectTargets := map[string]string{}
	for _, h := range homes {
		for _, name := range children(p.source, path.Join(h, "projects")) {
			src := path.Join(h, "projects", name)
			e, _ := p.source.get(src)
			if !e.Mode.IsDir() {
				p.Unmapped = append(p.Unmapped, src+": expected project folder")
				continue
			}
			target := name
			for n := 2; occupied[target]; n++ {
				target = name + "-" + strconv.Itoa(n)
			}
			occupied[target] = true
			dst := path.Join("projects", target)
			if target != name {
				p.Notes = append(p.Notes, src+": name collision resolved as "+target)
			}
			projectTargets[src] = dst
			paths[filepath.Join(from, filepath.FromSlash(src))] = filepath.Join(home, filepath.FromSlash(dst))
		}
	}
	for _, src := range sortedKeys(projectTargets) {
		dst := projectTargets[src]
		p.copyPrefix(src, dst, handled)
		p.projectRecord(src, dst)
		for _, task := range children(p.source, path.Join(src, "tasks")) {
			p.taskRecord(path.Join(src, "tasks", task), path.Join(dst, "tasks", task), path.Base(dst))
		}
	}
	for _, h := range homes {
		handled[path.Join(h, "projects")] = true
		if h != "" {
			handled[h] = true
		}
		for _, kind := range []string{"knowledge", "sessions"} {
			p.copyPrefix(path.Join(h, kind), kind, handled)
		}
		for _, name := range children(p.source, path.Join(h, "worktrees")) {
			src := path.Join(h, "worktrees", name)
			dst, ok := projectTargets[path.Join(h, "projects", name)]
			if !ok {
				p.Unmapped = append(p.Unmapped, src+": no matching project")
				continue
			}
			target := path.Join(dst, "worktrees")
			p.copyPrefix(src, target, handled)
			paths[filepath.Join(from, filepath.FromSlash(src))] = filepath.Join(home, filepath.FromSlash(target))
			p.Notes = append(p.Notes, src+": preserves legacy task/worker subfolders; repo/job mapping is unavailable")
		}
		handled[path.Join(h, "worktrees")] = true
	}
	if homes[0] != "" {
		for _, kind := range []string{"knowledge", "sessions"} {
			p.copyPrefix(kind, kind, handled)
		}
	}
	if e, ok := p.source.get("machine"); ok {
		handled["machine"] = true
		e.Src = filepath.Join(from, "machine")
		p.add("copy", "machine", "machine", e)
	}
	if err := p.pluginSessions(); err != nil {
		return nil, err
	}
	homeSet["CLAUDE_PLUGIN_DATA"] = true
	p.rewriteSessions(homeSet, paths)
	p.rewriteWorktrees(paths)
	for _, f := range p.source.below("") {
		if handled[f] {
			continue
		}
		// Keep unknown subtrees at their original relative paths and list them.
		parent := path.Dir(f)
		if parent != "." && !handled[parent] {
			continue
		}
		p.Notes = append(p.Notes, f+": unknown legacy entry carried over untouched")
		p.copyPrefix(f, f, handled)
	}
	if from == home || within(from, home) {
		prefix := ""
		if from != home {
			prefix, _ = filepath.Rel(home, from)
			prefix = filepath.ToSlash(prefix)
		}
		// An entry that is also a destination, or holds one, stays.
		used := map[string]bool{}
		for _, a := range p.Actions {
			for d := a.To; d != "." && !used[d]; d = path.Dir(d) {
				used[d] = true
			}
		}
		for f := range handled {
			old := path.Join(prefix, f)
			if old != "." && !used[old] {
				delete(p.result, old)
			}
		}
	}
	if n := len(p.absLinks); n > 0 {
		p.Notes = append(p.Notes, fmt.Sprintf("%d absolute symlinks (first: %s) are kept as links, not followed and not rewritten", n, p.absLinks[0]))
	}
	p.makeMachine()
	p.add("create", "", "VERSION", dataEntry([]byte(strconv.Itoa(gridhome.FormatVersion)+"\n"), 0600))
	if _, ok := p.destination.get("migration.log"); ok {
		p.Conflicts = append(p.Conflicts, "migration.log: existing migration log")
	}
	p.addParents()
	p.checkIdentities()
	p.Notes = append(p.Notes, "history, Feed, usage and recall files are preserved without schema conversion")
	return p, nil
}

// addParents lists the folders that only hold other entries, such as projects/, so
// the staged copy can be checked against the plan exactly.
func (p *Plan) addParents() {
	for _, k := range sortedKeys(p.result) {
		for d := path.Dir(k); d != "."; d = path.Dir(d) {
			if _, ok := p.result[d]; !ok {
				p.result[d] = entry{Mode: os.ModeDir | 0700}
			}
		}
	}
}

// makeMachine settles the machine id file. A home that has none gets one made at
// apply; the plan renders its records with a placeholder until then.
func (p *Plan) makeMachine() {
	e, ok := p.result["machine"]
	switch {
	case !ok:
		p.add("create", "", "machine", dataEntry([]byte("<made on apply>\n"), 0600))
	case !e.Mode.IsRegular():
		p.Conflicts = append(p.Conflicts, "machine: must be an id file; move the existing directory or link aside (for example, mv machine machine-legacy), then rerun grid migrate")
	case p.machine == "":
		p.Conflicts = append(p.Conflicts, "machine: the machine id file is empty")
	}
}

func (p *Plan) machineID() string {
	if p.machine == "" {
		return pendingMachine
	}
	return p.machine
}

// sourceVersion reads VERSION of the source folder, if it has one.
func (p *Plan) sourceVersion() (bool, error) {
	e, ok := p.source.get("VERSION")
	if !ok {
		return false, nil
	}
	if !e.Mode.IsRegular() {
		return false, fmt.Errorf("VERSION must be a regular file")
	}
	data, err := readSmall(filepath.Join(p.From, "VERSION"), e.Size)
	if err != nil {
		return false, err
	}
	return checkVersion(data)
}

// specialFiles blocks the plan on sockets, pipes and devices, which cannot be copied.
func (p *Plan) specialFiles() {
	seen := map[string]bool{}
	for _, s := range []snap{p.source, p.destination} {
		for _, k := range s.keys {
			if isSpecial(s.m[k].Mode) && !seen[k] {
				seen[k] = true
				p.Unmapped = append(p.Unmapped, k+": socket, pipe or device file cannot be copied; remove it or move it away")
			}
		}
	}
}

// rewriteSessions maps the paths in session pointer lists (<home>/sessions/<id>)
// and in the session records of each task. Other files named sessions, such as
// those inside a worktree's repo, are user content and are left alone.
func (p *Plan) rewriteSessions(homes map[string]bool, paths map[string]string) {
	for _, a := range p.Actions {
		if a.Kind != "copy" || !isSessionFile(a.From, homes) {
			continue
		}
		e := p.result[a.To]
		if !e.Mode.IsRegular() {
			continue
		}
		var data []byte
		switch path.Ext(a.From) {
		case ".json":
			raw, err := readSmall(e.Src, e.Size)
			if err == nil {
				data, err = rewriteSession(raw, paths)
			}
			if err != nil {
				p.Unmapped = append(p.Unmapped, a.From+": invalid session JSON")
				continue
			}
		default:
			ext := path.Ext(a.From)
			if ext != "" && ext != ".attached" && ext != ".planner" && ext != ".tasks" && !strings.HasPrefix(ext, ".tasks-") {
				continue
			}
			raw, err := readSmall(e.Src, e.Size)
			if err != nil {
				p.Unmapped = append(p.Unmapped, a.From+": "+err.Error())
				continue
			}
			lines := strings.Split(string(raw), "\n")
			for i, line := range lines {
				if line == "" {
					continue
				}
				ending := ""
				if strings.HasSuffix(line, "\r") {
					line = strings.TrimSuffix(line, "\r")
					ending = "\r"
				}
				mapped, ok := mapPath(line, paths)
				if !ok {
					p.Unmapped = append(p.Unmapped, a.From+": session pointer cannot map "+line)
				}
				lines[i] = mapped + ending
			}
			data = []byte(strings.Join(lines, "\n"))
		}
		e.Data, e.Size = data, int64(len(data))
		p.result[a.To] = e
	}
}

// isSessionFile matches sessions/<file>, <home>/sessions/<file> and
// <home>/projects/<project>/tasks/<task>/sessions/<file>.
func isSessionFile(from string, homes map[string]bool) bool {
	s := strings.Split(from, "/")
	n := len(s)
	if n == 2 && s[0] == "sessions" { // a pointer kept beside the named homes
		return true
	}
	if n >= 2 && s[n-2] == "sessions" && homes[strings.Join(s[:n-2], "/")] {
		return true
	}
	return n >= 6 && s[n-2] == "sessions" && s[n-4] == "tasks" && s[n-6] == "projects" && homes[strings.Join(s[:n-6], "/")]
}

func children(s snap, dir string) []string {
	out := map[string]bool{}
	prefix := ""
	if dir != "" && dir != "." {
		prefix = dir + "/"
	}
	for _, k := range s.below(dir) {
		if tail := strings.TrimPrefix(k, prefix); tail != "" && !strings.Contains(tail, "/") {
			out[tail] = true
		}
	}
	return sortedKeys(out)
}

// checkVersion reads the text of a VERSION file: whether it is the current format.
func checkVersion(data []byte) (bool, error) {
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return false, fmt.Errorf("invalid Grid VERSION")
	}
	if n > gridhome.FormatVersion {
		return false, fmt.Errorf("%s", gridhome.FormatProblem(n))
	}
	return n == gridhome.FormatVersion, nil
}

func (p *Plan) add(kind, from, to string, e entry) {
	for d := path.Dir(to); d != "."; d = path.Dir(d) {
		if parent, ok := p.result[d]; ok && !parent.Mode.IsDir() {
			p.Conflicts = append(p.Conflicts, to+": would overwrite non-directory "+d)
			return
		}
	}
	if old, ok := p.result[to]; ok && to != "VERSION" && !(p.samePath(e, to) && old.Src == e.Src) && !p.sameEntry(old, e) {
		p.Conflicts = append(p.Conflicts, to+": destination already has different data")
		return
	}
	p.result[to] = e
	p.Actions = append(p.Actions, Action{kind, from, to})
}

// samePath reports a copy of a file onto itself, as in an in-place migration.
func (p *Plan) samePath(e entry, to string) bool {
	return e.Src != "" && e.Src == filepath.Join(p.Home, filepath.FromSlash(to))
}

// sameEntry compares two entries of the same path. Contents are read, as a stream,
// only here, for the few paths two sources both claim.
func (p *Plan) sameEntry(a, b entry) bool {
	switch {
	case a.Mode.IsDir() || b.Mode.IsDir():
		return a.Mode.IsDir() && b.Mode.IsDir()
	case a.Mode&fs.ModeSymlink != 0 || b.Mode&fs.ModeSymlink != 0:
		return a.Mode&fs.ModeSymlink != 0 && b.Mode&fs.ModeSymlink != 0 && a.Link == b.Link
	case a.Src != "" && a.Src == b.Src && a.Data == nil && b.Data == nil:
		return true
	case wantSize(a) != wantSize(b):
		return false
	}
	sa, sb := p.sum(a), p.sum(b)
	return sa != "" && sa == sb
}

func (p *Plan) sum(e entry) string {
	if e.Data != nil {
		return hashBytes(e.Data)
	}
	if s, ok := p.sums[e.Src]; ok {
		return s
	}
	s, _ := hashFile(e.Src)
	p.sums[e.Src] = s
	return s
}

func (p *Plan) copyPrefix(src, dst string, handled map[string]bool) {
	if e, ok := p.source.get(src); ok {
		p.copyOne(src, dst, e, handled)
	}
	for _, f := range p.source.below(src) {
		p.copyOne(f, dst+strings.TrimPrefix(f, src), p.source.m[f], handled)
	}
}

func (p *Plan) copyOne(from, to string, e entry, handled map[string]bool) {
	handled[from] = true
	e.Src = filepath.Join(p.From, filepath.FromSlash(from))
	if e.Mode&fs.ModeSymlink != 0 && filepath.IsAbs(e.Link) {
		p.absLinks = append(p.absLinks, from)
	}
	p.add("copy", from, to, e)
}

func (p *Plan) projectRecord(src, dst string) {
	file := path.Join(src, "project.md")
	e, ok := p.source.get(file)
	if !ok {
		p.Unmapped = append(p.Unmapped, src+": missing project.md")
		return
	}
	data, err := readSmall(filepath.Join(p.From, filepath.FromSlash(file)), e.Size)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": "+err.Error())
		return
	}
	r, err := parseRecord(data)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": "+err.Error())
		return
	}
	folder := r.value("folder")
	if folder == "" {
		folder = r.value("repo")
	}
	if !filepath.IsAbs(folder) {
		p.Unmapped = append(p.Unmapped, file+": absolute folder or repo is required")
		return
	}
	folder = gridhome.Real(folder)
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		p.Notes = append(p.Notes, file+": repository folder unavailable; remote/root keys cannot be discovered, retaining folder identity")
	}
	machine := p.machineID()
	repos, keys := discover(folder, machine, p.ceiling)
	id := r.value("id")
	if !ulidRE.MatchString(id) {
		id = legacyID("project:" + p.From + ":" + src + ":" + string(data))
	}
	pr := plannedRecord{
		dst:   path.Join(dst, "project.md"),
		rec:   project.Record{Name: path.Base(dst), ID: id, Folders: []project.Folder{{Machine: machine, Path: folder}}, Repos: repos, Keys: keys},
		extra: r.keptLines(),
		body:  projectBody(data),
	}
	text, err := pr.render(machine)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": cannot write a valid project record: "+err.Error())
		return
	}
	e.Data, e.Size = text, int64(len(text))
	e.Src = filepath.Join(p.From, filepath.FromSlash(file))
	p.result[pr.dst] = e
	p.records = append(p.records, pr)
	p.Actions = append(p.Actions, Action{"identify", file, pr.dst})
}

func (p *Plan) taskRecord(src, dst, projectName string) {
	file := path.Join(src, "task.md")
	e, ok := p.source.get(file)
	if !ok {
		p.Unmapped = append(p.Unmapped, src+": missing task.md")
		return
	}
	data, err := readSmall(filepath.Join(p.From, filepath.FromSlash(file)), e.Size)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": "+err.Error())
		return
	}
	r, err := parseRecord(data)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": "+err.Error())
		return
	}
	id := r.value("id")
	display := r.value("display")
	if taskRE.MatchString(id) {
		display = id
		id = legacyID("task:" + p.From + ":" + src + ":" + string(data))
	}
	if !ulidRE.MatchString(id) || !taskRE.MatchString(display) {
		p.Unmapped = append(p.Unmapped, file+": expected T-NN id or ULID with display number")
		return
	}
	text := r.rewrite(map[string][]string{"id": {id}, "display": {display}, "project": {projectName}, "contract": {version.Contract}})
	e.Data, e.Size = text, int64(len(text))
	e.Src = filepath.Join(p.From, filepath.FromSlash(file))
	p.result[path.Join(dst, "task.md")] = e
	p.Actions = append(p.Actions, Action{"identify", file, path.Join(dst, "task.md")})
}

// WriteText prints stable, relative action paths suitable for review and golden tests.
func (p *Plan) WriteText(w io.Writer) error {
	var b strings.Builder
	switch {
	case p.Interrupted != "":
		fmt.Fprintf(&b, "migrate: interrupted run found\nrecover: %s\nrun: grid migrate --apply\n", p.Interrupted)
	case p.Current:
		fmt.Fprintf(&b, "migrate: already at format %d; no changes\n", gridhome.FormatVersion)
	default:
		fmt.Fprintf(&b, "migrate: plan format %d\n", gridhome.FormatVersion)
		fmt.Fprintf(&b, "from: %s\nhome: %s\n", p.From, p.Home)
		fmt.Fprintf(&b, "backup: %s-backup/<UTC-date>-<unique>/ before changes\n", p.Home)
		for _, a := range p.Actions {
			if a.From == "" {
				fmt.Fprintf(&b, "%s: %s\n", a.Kind, a.To)
			} else {
				fmt.Fprintf(&b, "%s: %s -> %s\n", a.Kind, a.From, a.To)
			}
		}
		fmt.Fprintln(&b, "write: migration.log (backup path, checksums and actions)")
		for _, n := range p.Notes {
			fmt.Fprintln(&b, "note: "+n)
		}
		for _, c := range p.Conflicts {
			fmt.Fprintln(&b, "conflict: "+c)
		}
		for _, u := range p.Unmapped {
			fmt.Fprintln(&b, "unmapped: "+u)
		}
		fmt.Fprintf(&b, "result: %d actions, %d conflicts, %d unmapped\n", len(p.Actions), len(p.Conflicts), len(p.Unmapped))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// versionAt allows a current home to be a true no-op, even with unrelated
// symlinks or inaccessible files elsewhere in that already migrated home.
func versionAt(home string) (bool, error) {
	file := filepath.Join(home, "VERSION")
	info, err := os.Lstat(file)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("VERSION must be a regular file")
	}
	data, err := readSmall(file, info.Size())
	if err != nil {
		return false, err
	}
	return checkVersion(data)
}

// checkIdentities flags two projects, or two tasks, that a later grid call could
// not tell apart: the same id, the same shared key (remote or root commit), the
// same machine-bound key on one machine, or the same folder on one machine.
func (p *Plan) checkIdentities() {
	byID := map[string]string{}
	machines := map[string]bool{}
	type found struct {
		file string
		rec  project.Record
	}
	var projects []found
	for _, file := range sortedKeys(p.result) {
		parts := strings.Split(file, "/")
		isProject := len(parts) == 3 && parts[0] == "projects" && parts[2] == "project.md"
		isTask := len(parts) == 5 && parts[0] == "projects" && parts[2] == "tasks" && parts[4] == "task.md"
		if !isProject && !isTask {
			continue
		}
		e := p.result[file]
		data := e.Data
		if data == nil {
			var err error
			if data, err = readSmall(e.Src, e.Size); err != nil {
				p.Conflicts = append(p.Conflicts, file+": destination record cannot be read")
				continue
			}
		}
		var id, display string
		if isProject {
			rec, err := project.Parse(parts[1], string(data))
			if err != nil {
				p.Conflicts = append(p.Conflicts, file+": destination record is invalid")
				continue
			}
			id = rec.ID
			projects = append(projects, found{file, rec})
			for _, f := range rec.Folders {
				machines[f.Machine] = true
			}
			for _, k := range rec.Keys {
				machines[k.Machine] = true
			}
		} else {
			r, err := parseRecord(data)
			if err != nil {
				p.Conflicts = append(p.Conflicts, file+": destination record is invalid")
				continue
			}
			id, display = r.value("id"), r.value("display")
		}
		if !ulidRE.MatchString(id) || (isTask && !taskRE.MatchString(display)) {
			p.Conflicts = append(p.Conflicts, file+": destination record still has a legacy identity")
		}
		if ulidRE.MatchString(id) {
			if prev, ok := byID[id]; ok {
				p.Conflicts = append(p.Conflicts, file+": id also used by "+prev)
			}
			byID[id] = file
		}
	}
	reported := map[string]bool{}
	for _, m := range sortedKeys(machines) {
		byKey := map[string]string{}
		for _, f := range projects {
			tokens := f.rec.KeysOn(m)
			for _, folder := range f.rec.FoldersOn(m) {
				tokens = append(tokens, "folder:"+folder)
			}
			for _, key := range tokens {
				token := key
				if !strings.HasPrefix(key, "remote:") && !strings.HasPrefix(key, "root:") {
					token = m + "|" + key
				}
				if prev, ok := byKey[token]; ok && prev != f.file {
					if msg := f.file + ": identity key also used by " + prev; !reported[msg] {
						reported[msg] = true
						p.Conflicts = append(p.Conflicts, msg)
					}
				}
				byKey[token] = f.file
			}
		}
	}
}

// pluginSessions imports hook pointers and usage without changing the old plugin data.
func (p *Plan) pluginSessions() error {
	if p.pluginData == "" {
		return nil
	}
	p.pluginData = gridhome.Real(filepath.Join(p.pluginData, "sessions"))
	var err error
	p.plugin, err = snapshot(p.pluginData)
	if err != nil {
		return fmt.Errorf("CLAUDE_PLUGIN_DATA sessions: %w", err)
	}
	for _, rel := range p.plugin.keys {
		e := p.plugin.m[rel]
		if isSpecial(e.Mode) {
			p.Unmapped = append(p.Unmapped, "CLAUDE_PLUGIN_DATA/sessions/"+rel+": cannot copy special file")
			continue
		}
		e.Src = filepath.Join(p.pluginData, filepath.FromSlash(rel))
		p.add("copy", path.Join("CLAUDE_PLUGIN_DATA/sessions", rel), path.Join("sessions", rel), e)
	}
	return nil
}
