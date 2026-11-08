package migrate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
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

type record struct {
	lines []string
	end   int
}

func parseRecord(data []byte) (record, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return record{}, fmt.Errorf("missing frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return record{lines, i}, nil
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
	return []byte(strings.Join(out, "\n"))
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// identity uses local git facts only; it never contacts a remote or writes a repo.
func identity(folder string) (repos, keys []string) {
	seen := map[string]bool{}
	add := func(repo string) {
		repos = append(repos, repo)
		for _, remote := range gitx.Remotes(repo) {
			if k := project.NormalizeRemote(remote); k != "" {
				seen["remote:"+k] = true
			}
		}
		if root := gitx.RootCommit(repo); root != "" {
			seen["root:"+root] = true
		}
	}
	root, found := gitx.FindRoot(folder, "")
	if found {
		_, err := gitx.CommonDir(root)
		found = err == nil
	}
	if found {
		add(root)
	} else {
		// Match the resolver's two-level multi-repo discovery without following links.
		_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(folder, path)
			if rel == "." {
				return nil
			}
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if gitx.IsRepoRoot(path) {
				add(path)
				return filepath.SkipDir
			}
			if depth >= 2 {
				return filepath.SkipDir
			}
			return nil
		})
	}
	if len(seen) == 0 {
		seen["folder:"+gridhome.Real(folder)] = true
	}
	return repos, sortedKeys(seen)
}

func rewriteSession(data []byte, paths map[string]string) ([]byte, error) {
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("session must be an object")
	}
	for _, key := range []string{"cwd", "transcript"} {
		if s, ok := v[key].(string); ok {
			v[key] = mapPath(s, paths)
		}
	}
	b, err := json.Marshal(v)
	return append(b, '\n'), err
}
func mapPath(path string, paths map[string]string) string {
	best := ""
	for old := range paths {
		if (path == old || strings.HasPrefix(path, old+string(filepath.Separator))) && len(old) > len(best) {
			best = old
		}
	}
	if best != "" {
		return paths[best] + strings.TrimPrefix(path, best)
	}
	return path
}

func knownPath(value string, paths map[string]string) bool {
	for old := range paths {
		if value == old || strings.HasPrefix(value, old+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
