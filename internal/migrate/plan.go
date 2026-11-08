// Package migrate plans and applies offline migrations from the old named Grid
// homes into one versioned home. Planning never writes to disk.
package migrate

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// Action describes a source-relative to destination-relative file operation.
type Action struct{ Kind, From, To string }

// Plan is a read-only preview. Apply uses its private snapshot, and refuses if
// either input tree has changed since Scan. Conflicts and Unmapped block apply.
type Plan struct {
	From, Home                  string
	Actions                     []Action
	Conflicts, Unmapped, Notes  []string
	Current                     bool
	source, destination, result tree
}

// Scan accepts either an old Grid root containing named homes or a single old
// home containing projects/. Home is the new Grid home (normally ~/.grid).
func Scan(from, home string) (*Plan, error) {
	if from == "" || home == "" {
		return nil, fmt.Errorf("source and Grid home are required")
	}
	from = gridhome.Real(from)
	home = gridhome.Real(home)
	if from != home && within(home, from) {
		return nil, fmt.Errorf("destination must not be inside the source")
	}
	p := &Plan{From: from, Home: home}
	var err error
	p.destination, err = snapshot(home)
	if err != nil {
		return nil, err
	}
	current, err := checkVersion(p.destination)
	if err != nil {
		return nil, err
	}
	if current {
		p.Current = true
		return p, nil
	}
	p.source, err = snapshot(from)
	if err != nil {
		return nil, err
	}
	if len(p.source) == 0 {
		return nil, fmt.Errorf("old Grid data folder does not exist: %s", from)
	}
	if current, err = checkVersion(p.source); err != nil {
		return nil, err
	} else if current && from != home {
		return nil, fmt.Errorf("source is already a current Grid home")
	}
	p.result = tree{}
	for k, v := range p.destination {
		p.result[k] = v
	}
	p.result["."] = entry{Mode: os.ModeDir | 0700}
	homes := []string{}
	if e, ok := p.source["projects"]; ok && e.Mode.IsDir() {
		homes = append(homes, "")
	}
	for _, name := range children(p.source, "") {
		if e, ok := p.source[path.Join(name, "projects")]; ok && e.Mode.IsDir() {
			homes = append(homes, name)
		}
	}
	if len(homes) == 0 {
		return nil, fmt.Errorf("no old projects folder found")
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
			e := p.source[src]
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
			// Moving live git worktrees needs writes to the original repo's git metadata.
			linked := false
			for f := range p.source {
				if under(f, src) && path.Base(f) == ".git" {
					linked = true
				}
			}
			if linked {
				p.Unmapped = append(p.Unmapped, src+": git worktree needs repository metadata repair; leave stopped and relocate separately")
				continue
			}
			target := path.Join(dst, "worktrees")
			p.copyPrefix(src, target, handled)
			paths[filepath.Join(from, filepath.FromSlash(src))] = filepath.Join(home, filepath.FromSlash(target))
			p.Notes = append(p.Notes, src+": preserves legacy task/worker subfolders; repo/job mapping is unavailable")
		}
		handled[path.Join(h, "worktrees")] = true
	}
	// Session JSON is structured; pointer lists are rewritten one full path per line.
	for _, a := range p.Actions {
		if a.Kind != "copy" {
			continue
		}
		if !strings.Contains("/"+a.From, "/sessions/") {
			continue
		}
		e := p.result[a.To]
		if e.Mode.IsDir() {
			continue
		}
		switch path.Ext(a.From) {
		case ".usage", ".recalled":
			continue
		case ".json":
			b, err := rewriteSession(e.Data, paths)
			if err != nil {
				p.Unmapped = append(p.Unmapped, a.From+": invalid session JSON")
				continue
			}
			e.Data = b
		default:
			lines := strings.Split(string(e.Data), "\n")
			for i, line := range lines {
				if line == "" {
					continue
				}
				mapped := mapPath(line, paths)
				if mapped == line {
					p.Unmapped = append(p.Unmapped, a.From+": session pointer cannot map "+line)
				}
				lines[i] = mapped
			}
			e.Data = []byte(strings.Join(lines, "\n"))
		}
		p.result[a.To] = e
	}
	for _, f := range sortedKeys(p.source) {
		if handled[f] {
			continue
		}
		// Report unknown subtrees once, but keep them out of the new layout.
		parent := path.Dir(f)
		if parent != "." && !handled[parent] {
			continue
		}
		p.Unmapped = append(p.Unmapped, f+": no mapping for legacy data")
	}
	if from == home || within(from, home) {
		prefix := ""
		if from != home {
			prefix, _ = filepath.Rel(home, from)
			prefix = filepath.ToSlash(prefix)
		}
		for f := range handled {
			old := path.Join(prefix, f)
			if old == "." {
				continue
			}
			// Do not remove an entry also used by a destination action.
			used := false
			for _, a := range p.Actions {
				if a.To == old || under(a.To, old) {
					used = true
					break
				}
			}
			if !used {
				delete(p.result, old)
			}
		}
	}
	if _, ok := p.destination["machine"]; !ok {
		p.add("create", "", "machine", entry{Data: []byte(legacyID("machine:"+home) + "\n"), Mode: 0600})
	}
	p.add("create", "", "VERSION", entry{Data: []byte(strconv.Itoa(gridhome.FormatVersion) + "\n"), Mode: 0600})
	if _, ok := p.destination["migration.log"]; ok {
		p.Conflicts = append(p.Conflicts, "migration.log: existing migration log")
	}
	p.Notes = append(p.Notes, "history, Feed, usage and recall files are preserved without schema conversion")
	return p, nil
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func under(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
func children(t tree, dir string) []string {
	out := map[string]bool{}
	prefix := ""
	if dir != "" && dir != "." {
		prefix = dir + "/"
	}
	for k := range t {
		if !strings.HasPrefix(k, prefix) || k == "." {
			continue
		}
		tail := strings.TrimPrefix(k, prefix)
		if tail != "" && !strings.Contains(tail, "/") {
			out[tail] = true
		}
	}
	return sortedKeys(out)
}
func checkVersion(t tree) (bool, error) {
	e, ok := t["VERSION"]
	if !ok {
		return false, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(e.Data)))
	if err != nil || n < 0 {
		return false, fmt.Errorf("invalid Grid VERSION")
	}
	if n > gridhome.FormatVersion {
		return false, fmt.Errorf("Grid format %d is newer than supported format %d; update grid", n, gridhome.FormatVersion)
	}
	return n == gridhome.FormatVersion, nil
}
func (p *Plan) add(kind, from, to string, e entry) {
	if old, ok := p.result[to]; ok && !same(old, e) {
		samePath := from != "" && filepath.Join(p.From, filepath.FromSlash(from)) == filepath.Join(p.Home, filepath.FromSlash(to))
		if !samePath && to != "VERSION" {
			p.Conflicts = append(p.Conflicts, to+": destination already has different data")
			return
		}
	}
	p.result[to] = e
	p.Actions = append(p.Actions, Action{kind, from, to})
}
func (p *Plan) copyPrefix(src, dst string, handled map[string]bool) {
	for _, f := range sortedKeys(p.source) {
		if under(f, src) {
			handled[f] = true
			p.add("copy", f, dst+strings.TrimPrefix(f, src), p.source[f])
		}
	}
}
func (p *Plan) projectRecord(src, dst string) {
	file := path.Join(src, "project.md")
	e, ok := p.source[file]
	if !ok {
		p.Unmapped = append(p.Unmapped, src+": missing project.md")
		return
	}
	r, err := parseRecord(e.Data)
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
	repos, keys := identity(folder)
	id := r.value("id")
	if !ulidRE.MatchString(id) {
		id = legacyID("project:" + p.From + ":" + src + ":" + string(e.Data))
	}
	fields := map[string][]string{"id": {id}, "folder": {folder}, "key": keys, "repo": repos}
	e.Data = r.rewrite(fields)
	p.result[path.Join(dst, "project.md")] = e
	p.Actions = append(p.Actions, Action{"identify", file, path.Join(dst, "project.md")})
}
func (p *Plan) taskRecord(src, dst, projectName string) {
	file := path.Join(src, "task.md")
	e, ok := p.source[file]
	if !ok {
		p.Unmapped = append(p.Unmapped, src+": missing task.md")
		return
	}
	r, err := parseRecord(e.Data)
	if err != nil {
		p.Unmapped = append(p.Unmapped, file+": "+err.Error())
		return
	}
	id := r.value("id")
	display := r.value("display")
	if taskRE.MatchString(id) {
		display = id
		id = legacyID("task:" + p.From + ":" + src + ":" + string(e.Data))
	}
	if !ulidRE.MatchString(id) || !taskRE.MatchString(display) {
		p.Unmapped = append(p.Unmapped, file+": expected T-NN id or ULID with display number")
		return
	}
	e.Data = r.rewrite(map[string][]string{"id": {id}, "display": {display}, "project": {projectName}, "contract": {version.Contract}})
	p.result[path.Join(dst, "task.md")] = e
	p.Actions = append(p.Actions, Action{"identify", file, path.Join(dst, "task.md")})
}

// WriteText prints stable, relative action paths suitable for review and golden tests.
func (p *Plan) WriteText(w io.Writer) error {
	var b strings.Builder
	if p.Current {
		fmt.Fprintf(&b, "migrate: already at format %d; no changes\n", gridhome.FormatVersion)
	} else {
		fmt.Fprintf(&b, "migrate: plan format %d\n", gridhome.FormatVersion)
		fmt.Fprintln(&b, "backup: sibling <home>-backup/<UTC-date>-<unique>/ before changes")
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
