package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

var ulidRE = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
var taskRE = regexp.MustCompile(`^T-[0-9]+$`)

// Migrated ids use a zero timestamp and deterministic 80-bit entropy. This makes
// repeated plans stable without writing allocation state during a dry run.
func legacyID(namespace string) string {
	h := sha256.Sum256([]byte(namespace))
	n := new(big.Int).SetBytes(h[:10])
	base := big.NewInt(32)
	rem := new(big.Int)
	out := []byte(strings.Repeat("0", 26))
	alphabet := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for i := 25; i >= 0; i-- {
		n.QuoRem(n, base, rem)
		out[i] = alphabet[rem.Int64()]
	}
	return string(out)
}

// record is the flat key: value front matter of an old project.md or task.md.
type record struct {
	lines   []string
	end     int
	newline string
}

func parseRecord(data []byte) (record, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return record{}, fmt.Errorf("missing frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			newline := "\n"
			if bytes.Contains(data, []byte("\r\n")) {
				newline = "\r\n"
			}
			return record{lines: lines, end: i, newline: newline}, nil
		}
	}
	return record{}, fmt.Errorf("unterminated frontmatter")
}
func (r record) values(key string) []string {
	var out []string
	for _, line := range r.lines[1:r.end] {
		k, v, ok := strings.Cut(line, ":")
		if !ok || k != key {
			continue
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "\"") {
			if u, err := strconv.Unquote(v); err == nil {
				v = u
			}
		} else if len(v) > 1 && v[0] == '\'' && v[len(v)-1] == '\'' {
			v = strings.ReplaceAll(v[1:len(v)-1], "''", "'")
		}
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
func (r record) value(key string) string {
	v := r.values(key)
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
func (r record) rewrite(fields map[string][]string) []byte {
	out := []string{"---"}
	for _, line := range r.lines[1:r.end] {
		k, _, _ := strings.Cut(line, ":")
		if _, ok := fields[k]; !ok {
			out = append(out, line)
		}
	}
	for _, key := range sortedKeys(fields) {
		for _, v := range fields[key] {
			out = append(out, key+": "+strconv.Quote(v))
		}
	}
	out = append(out, r.lines[r.end:]...)
	return []byte(strings.Join(out, r.newline))
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// projectBody returns the bytes after the closing "---" line of an old project.md,
// exactly as they were (line endings included).
func projectBody(data []byte) []byte {
	pos, first := 0, true
	for pos < len(data) {
		nl := bytes.IndexByte(data[pos:], '\n')
		end := len(data)
		if nl >= 0 {
			end = pos + nl + 1
		}
		line := strings.TrimRight(string(data[pos:end]), "\r\n")
		if !first && line == "---" {
			return data[end:]
		}
		first = false
		pos = end
	}
	return nil
}

// oldProjectKeys are the front matter keys a rewritten project.md replaces.
var oldProjectKeys = map[string]bool{"id": true, "folder": true, "repo": true, "key": true, "folders": true, "repos": true, "keys": true}

// keptLines returns the top-level front matter lines of an old project.md that the
// new record does not replace, each with the indented lines that continue it.
func (r record) keptLines() []string {
	var out []string
	keep := false
	for _, line := range r.lines[1:r.end] {
		if line == "" || line[0] != ' ' && line[0] != '\t' && line[0] != '-' {
			k, _, _ := strings.Cut(line, ":")
			keep = line != "" && !strings.HasPrefix(line, "#") && !oldProjectKeys[k]
		}
		if keep {
			out = append(out, line)
		}
	}
	return out
}

// plannedRecord is a project.md the migration writes: the record in the main
// format plus the old body. It is rendered again at apply, once the machine id is
// known, so the id in the plan can still be a placeholder.
type plannedRecord struct {
	dst   string
	rec   project.Record
	extra []string
	body  []byte
}

func (pr plannedRecord) render(machine string) ([]byte, error) {
	rec := pr.rec
	rec.Folders = append([]project.Folder(nil), rec.Folders...)
	for i := range rec.Folders {
		rec.Folders[i].Machine = machine
	}
	rec.Keys = append([]project.Key(nil), rec.Keys...)
	for i := range rec.Keys {
		if rec.Keys[i].Machine != "" {
			rec.Keys[i].Machine = machine
		}
	}
	front, err := project.Format(rec)
	if err != nil {
		return nil, err
	}
	front = bytes.TrimSuffix(front, []byte("---\n"))
	for _, l := range pr.extra {
		front = append(front, l+"\n"...)
	}
	out := append(front, "---\n"...)
	out = append(out, pr.body...)
	if _, err := project.Parse(rec.Name, string(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// discover reads local git facts only; it never contacts a remote or writes a repo.
// A folder that is gone gives no repos and no keys: its record keeps the folder.
func discover(folder, machine, ceiling string) ([]project.Repo, []project.Key) {
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return nil, nil
	}
	var roots []string
	if root, ok := gitx.FindRoot(folder, ceiling); ok {
		if _, err := gitx.CommonDir(root); err == nil {
			roots = []string{root}
		}
	}
	if roots == nil {
		roots = childRepos(folder)
	}
	var repos []project.Repo
	var keys []project.Key
	seen := map[string]bool{}
	addKey := func(k project.Key) {
		if id := k.Key + "@" + k.Machine; !seen[id] {
			seen[id] = true
			keys = append(keys, k)
		}
	}
	for _, root := range roots {
		rel, err := filepath.Rel(folder, root)
		if err != nil {
			continue
		}
		repo := project.Repo{Path: filepath.ToSlash(rel)}
		if common, err := gitx.CommonDir(root); err == nil {
			addKey(project.NewKey(machine, "common:"+common))
		}
		var remotes []string
		have := map[string]bool{}
		for _, raw := range gitx.Remotes(root) {
			if n := project.NormalizeRemote(raw); n != "" && !have[n] {
				have[n] = true
				remotes = append(remotes, n)
			}
		}
		sort.Strings(remotes)
		for _, n := range remotes {
			addKey(project.Key{Key: "remote:" + n})
		}
		if len(remotes) > 0 {
			repo.Remote = remotes[0]
		} else if sha := gitx.RootCommit(root); sha != "" {
			addKey(project.Key{Key: "root:" + sha}) // a fork shares it, so only a repo with no remote uses it
		}
		repos = append(repos, repo)
	}
	return repos, keys
}

// childRepos lists git repos one or two levels below dir, never entering a repo
// or following a link; it follows the same rules as grid where.
func childRepos(dir string) []string {
	var repos []string
	var walk func(d string, depth int)
	walk = func(d string, depth int) {
		entries, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(d, e.Name())
			if gitx.IsRepoRoot(p) {
				repos = append(repos, p)
			} else if depth < 2 {
				walk(p, depth+1)
			}
		}
	}
	walk(dir, 1)
	sort.Strings(repos)
	return repos
}

// rewriteSession maps the paths of a session record. Numbers and text are kept as
// they were read. When no path changes, the original bytes are returned.
func rewriteSession(data []byte, paths map[string]string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("data after the session object")
	}
	if v == nil {
		return nil, fmt.Errorf("session must be an object")
	}
	changed := false
	for _, key := range []string{"cwd", "transcript"} {
		if s, ok := v[key].(string); ok {
			mapped, _ := mapPath(s, paths)
			v[key] = mapped
			changed = changed || mapped != s
		}
	}
	if !changed {
		return data, nil
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// mapPath moves a path under an old project or worktree folder to its new place and
// reports whether it was under one. A path written through a symlink (macOS /var,
// /tmp) is resolved before it is looked up again.
func mapPath(p string, paths map[string]string) (string, bool) {
	if m, ok := lookup(p, paths); ok {
		return m, true
	}
	if filepath.IsAbs(p) {
		if r := gridhome.Real(p); r != p {
			if mapped, ok := lookup(r, paths); ok {
				return mapped, true
			}
		}
	}
	return p, false
}

func lookup(p string, paths map[string]string) (string, bool) {
	best := ""
	for old := range paths {
		if (p == old || strings.HasPrefix(p, old+string(filepath.Separator))) && len(old) > len(best) {
			best = old
		}
	}
	if best == "" {
		return p, false
	}
	return paths[best] + strings.TrimPrefix(p, best), true
}
