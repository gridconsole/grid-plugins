package job

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
)

type Tier string

const (
	Team     Tier = "person-team"
	Guest    Tier = "person-guest"
	Agent    Tier = "agent"
	External Tier = "external"
)

// Source is a resolved, pinned pointer. Tier must come from trusted provenance,
// not from the body of the pointed-to file. Unknown provenance is external.
type Source struct {
	Pointer string `json:"pointer"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
	Tier    Tier   `json:"tier"`
	Digest  string `json:"sha256"`
	Topic   string `json:"topic,omitempty"`
	Updated string `json:"updated,omitempty"`
	Items   []int  `json:"items,omitempty"`
	Links   []Link `json:"links,omitempty"`
}
type Resolver interface {
	Resolve(pointer string) (Source, error)
}

// Files resolves only under explicitly configured roots. Trust is supplied by
// the caller's provenance reader; frontmatter cannot promote its own trust.
type Files struct {
	Task      string
	Repo      string
	Prototype string
	Repos     map[string]string
	Brain     map[string]string // level -> knowledge directory
	Trust     map[string]Tier
	cache     map[string]Source
}

var lineAnchor = regexp.MustCompile(`^L([0-9]+)(?:-L?([0-9]+))?$`)
var rangeSuffix = regexp.MustCompile(`:([0-9]+)-([0-9]+)$`)
var stepLine = regexp.MustCompile(`^- \[[ xX]\] ([0-9]+)\s+`)

func (f *Files) Resolve(pointer string) (Source, error) {
	if s, ok := f.cache[pointer]; ok {
		return s, nil
	}
	root, rel, anchor := f.Task, pointer, ""
	if at := strings.IndexByte(rel, '#'); at >= 0 {
		anchor = rel[at+1:]
		rel = rel[:at]
	}
	if m := rangeSuffix.FindStringSubmatch(rel); m != nil {
		if anchor != "" {
			return Source{}, fmt.Errorf("%s: cannot combine range and anchor", pointer)
		}
		anchor = "L" + m[1] + "-L" + m[2]
		rel = strings.TrimSuffix(rel, m[0])
	}
	topic := ""
	switch {
	case strings.HasPrefix(rel, "brain:"):
		level, id, ok := strings.Cut(strings.TrimPrefix(rel, "brain:"), "/")
		if !ok {
			return Source{}, fmt.Errorf("%s: expected brain:level/id", pointer)
		}
		root = f.Brain[level]
		rel = id
		if filepath.Ext(rel) == "" {
			rel += ".md"
		}
		topic = strings.Split(id, "/")[0]
	case strings.HasPrefix(rel, "task:"):
		rel = strings.TrimPrefix(rel, "task:")
	case strings.HasPrefix(rel, "prototype:"):
		root = f.Prototype
		rel = strings.TrimPrefix(rel, "prototype:")
	case strings.HasPrefix(rel, "repo:"):
		root = f.Repo
		rel = strings.TrimPrefix(rel, "repo:")
		if name, path, ok := strings.Cut(rel, ":"); ok {
			root = f.Repos[name]
			rel = path
		}
	case strings.HasPrefix(rel, "job:"):
		id := strings.TrimPrefix(rel, "job:")
		if task, j, ok := strings.Cut(id, "/"); ok {
			if task != strings.Split(filepath.Base(f.Task), "-")[0]+"-"+taskNumber(f.Task) {
				return Source{}, fmt.Errorf("%s: cross-task Job requires an index resolver", pointer)
			}
			id = j
		}
		if !jobID.MatchString(id) {
			return Source{}, fmt.Errorf("%s: invalid Job id", pointer)
		}
		rel = filepath.Join("jobs", id, "brief.md")
	}
	path, e := Within(root, rel)
	if e != nil {
		return Source{}, fmt.Errorf("%s: %w", pointer, e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return Source{}, fmt.Errorf("%s: cannot read source: %w", pointer, e)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	s := Source{Pointer: pointer, Tier: External, Topic: topic}
	if tier, ok := f.Trust[pointer]; ok {
		s.Tier = tier
	}
	if strings.HasPrefix(text, "---\n") {
		d, e := frontmatter.Parse(text)
		if e != nil {
			return s, fmt.Errorf("%s: %w", pointer, e)
		}
		s.Summary, _ = d.Data["summary"].(string)
		s.Updated, _ = d.Data["updated"].(string)
		if v, ok := d.Data["items"]; ok && strings.HasPrefix(pointer, "brain:") {
			if e := frontmatter.Into(v, &s.Items); e != nil {
				return s, e
			}
		}
		if v, ok := d.Data["links"]; ok {
			if e := frontmatter.Into(v, &s.Links); e != nil {
				return s, e
			}
		}
		if anchor == "" {
			s.Text = strings.TrimRight(d.Body, "\n")
		}
	}
	if anchor != "" {
		s.Text, e = selectText(text, anchor)
		if e != nil {
			return s, fmt.Errorf("%s: %w", pointer, e)
		}
	} else if s.Text == "" {
		s.Text = strings.TrimRight(text, "\n")
	}
	if s.Summary == "" && !strings.HasPrefix(pointer, "brain:") {
		s.Summary = firstLine(s.Text)
	}
	sum := sha256.Sum256(b)
	s.Digest = hex.EncodeToString(sum[:])
	if f.cache == nil {
		f.cache = map[string]Source{}
	}
	f.cache[pointer] = s
	return s, nil
}
func taskNumber(path string) string {
	parts := strings.Split(filepath.Base(path), "-")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Within refuses lexical and symlink escapes, including nonexistent roots.
func Within(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("source root is not configured")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute source paths are not allowed")
	}
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return "", e
	}
	path, e := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if e != nil {
		return "", e
	}
	r, e := filepath.Rel(root, path)
	if e != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source escapes its root")
	}
	return path, nil
}
func selectText(text, anchor string) (string, error) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if m := lineAnchor.FindStringSubmatch(anchor); m != nil {
		a, _ := strconv.Atoi(m[1])
		b := a
		if m[2] != "" {
			b, _ = strconv.Atoi(m[2])
		}
		if a < 1 || b < a || b > len(lines) {
			return "", fmt.Errorf("range outside file (1-%d)", len(lines))
		}
		return strings.Join(lines[a-1:b], "\n"), nil
	}
	start, end := -1, len(lines)
	level := 0
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(anchor, "step-") {
			if m := stepLine.FindStringSubmatch(trimmed); m != nil {
				if start >= 0 {
					end = i
					break
				}
				if "step-"+m[1] == anchor {
					start = i
					level = -1
					continue
				}
			}
		}
		if anchor == "verify" && strings.HasPrefix(trimmed, "Verify:") {
			return l, nil
		}
		if !strings.HasPrefix(l, "#") {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "#"))
		if n >= len(l) || l[n] != ' ' {
			continue
		}
		if start >= 0 && (level == -1 || n <= level) {
			end = i
			break
		}
		heading := strings.TrimSpace(l[n:])
		slug := strings.ToLower(heading)
		slug = regexp.MustCompile(`[^\pL\pN -]`).ReplaceAllString(slug, "")
		slug = strings.ReplaceAll(slug, " ", "-")
		if slug == anchor || strings.HasSuffix(heading, "{#"+anchor+"}") {
			if start >= 0 {
				return "", fmt.Errorf("ambiguous anchor %s", anchor)
			}
			start = i
			level = n
		}
	}
	if start < 0 {
		return "", fmt.Errorf("anchor %q not found", anchor)
	}
	return strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n"), nil
}
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(strings.TrimLeft(l, "# "))
		}
	}
	return ""
}

var jobID = regexp.MustCompile(`^J-[0-9]+(?:\.[0-9]+)*$`)

func ValidID(id string) bool { return jobID.MatchString(id) }

// LoadPlan selects exactly one structured item, by item number or Job id.
// The local interchange is plan.md frontmatter items: [Job, ...].
func LoadPlan(task, selector string) (Job, error) {
	path, e := Within(task, "plan.md")
	if e != nil {
		return Job{}, e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return Job{}, e
	}
	d, e := frontmatter.Parse(string(b))
	if e != nil {
		return Job{}, fmt.Errorf("plan.md: %w", e)
	}
	var items []Job
	if e = frontmatter.Into(d.Data["items"], &items); e != nil {
		return Job{}, fmt.Errorf("plan.md items: %w", e)
	}
	var matches []Job
	for _, j := range items {
		if strconv.Itoa(j.Item) == selector || j.ID == selector {
			matches = append(matches, j)
		}
	}
	if len(matches) != 1 {
		return Job{}, fmt.Errorf("plan.md: expected one item %q, found %d", selector, len(matches))
	}
	j := matches[0]
	j.Defaults()
	return j, nil
}

// DecodeBrief returns the typed Job and frontmatter locations for diagnostics.
func DecodeBrief(text string) (Job, frontmatter.Document, error) {
	d, e := frontmatter.Parse(text)
	if e != nil {
		return Job{}, d, e
	}
	var j Job
	e = frontmatter.Into(d.Data, &j)
	return j, d, e
}

// Metadata is JSON on one line per top-level field, a YAML-compatible subset.
func Metadata(j Job) (string, error) {
	b, e := json.Marshal(j)
	if e != nil {
		return "", e
	}
	var m map[string]json.RawMessage
	if e = json.Unmarshal(b, &m); e != nil {
		return "", e
	}
	// Struct field order, rather than map iteration, fixes the display order.
	var out strings.Builder
	out.WriteString("---\n")
	for _, key := range []string{"id", "item", "title", "role", "skills", "review", "risk", "max_rounds", "intent", "links", "keep", "decisions", "read", "write", "out_of_scope", "acceptance", "verify", "budget", "on_blocked", "after", "attempt", "continues", "discovered_from", "close_reason", "checkpoint", "review_rounds", "level_raised", "worktree", "posts_back", "status", "follow_up"} {
		fmt.Fprintf(&out, "%s: %s\n", key, m[key])
	}
	out.WriteString("---\n")
	return out.String(), nil
}
