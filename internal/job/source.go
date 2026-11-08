package job

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// Source is a resolved, pinned pointer. Tier comes from where the text was read
// from, never from the text itself; unknown provenance is external. A PointerOnly
// source is a personal note: the Job names it and nothing of it is copied.
type Source struct {
	State       string             `json:"state,omitempty"`
	Pointer     string             `json:"pointer"`
	Summary     string             `json:"summary"`
	Text        string             `json:"text"`
	Tier        contract.TrustTier `json:"tier"`
	Digest      string             `json:"sha256"`
	Topic       string             `json:"topic,omitempty"`
	Updated     string             `json:"updated,omitempty"`
	Links       []Link             `json:"links,omitempty"`
	PointerOnly bool               `json:"pointer_only,omitempty"`
}
type Resolver interface {
	Resolve(pointer string) (Source, error)
}

// NoteReader is the brain store. Notes are read only through it, so a proposed,
// rejected or unknown note never reaches a Job.
type NoteReader interface {
	Show(id string) (brain.Entry, error)
}

// Files resolves task, brain, repo and prototype pointers. Repo and prototype
// files must lie inside one of Folders, the project's own folders on this machine.
type Files struct {
	Task      string
	Owner     contract.TrustTier // the task owner's tier; it sets the tier of intent.md
	Notes     NoteReader
	Folders   []string
	Repo      string            // primary repo root
	Repos     map[string]string // named repo roots
	Prototype string
	cache     map[string]Source
	// afterIdentify is a per-resolver test seam for path replacement races.
	afterIdentify func()
}

var lineAnchor = regexp.MustCompile(`^L([0-9]+)(?:-L?([0-9]+))?$`)
var rangeSuffix = regexp.MustCompile(`:([0-9]+)-([0-9]+)$`)
var slugDrop = regexp.MustCompile(`[^\pL\pN -]`)

// ParseBrain splits brain:<level>/<name>, the one id shape the brain store uses.
func ParseBrain(id string) (level, name string, ok bool) {
	level, name, ok = strings.Cut(strings.TrimPrefix(id, "brain:"), "/")
	return level, name, strings.HasPrefix(id, "brain:") && ok && (level == "home" || level == "project") && brain.ValidName(name)
}

// ErrPlanNeedsApproval refuses worker text from an unapproved or changed plan.
var ErrPlanNeedsApproval = errors.New("plan needs approval: review and approve the current plan before rendering a Job")

func (f *Files) Resolve(pointer string) (Source, error) {
	rel, anchor := pointer, ""
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
	var s Source
	var err error
	switch {
	case strings.HasPrefix(rel, "brain:"):
		if s, ok := f.cache[pointer]; ok {
			return s, nil
		}
		s, err = f.brain(pointer, rel, anchor)
	case strings.HasPrefix(rel, "prototype:"):
		s, err = f.file(pointer, f.Prototype, strings.TrimPrefix(rel, "prototype:"), anchor, true)
	case strings.HasPrefix(rel, "repo:"):
		root, path := f.Repo, strings.TrimPrefix(rel, "repo:")
		if name, p, ok := strings.Cut(path, ":"); ok {
			root, path = f.Repos[name], p
		}
		s, err = f.file(pointer, root, path, anchor, true)
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
		s, err = f.file(pointer, f.Task, filepath.Join("jobs", id, "brief.md"), anchor, false)
		s.Tier = contract.Agent // rendered by this CLI from the accepted plan
	default:
		rel = strings.TrimPrefix(rel, "task:")
		s, err = f.file(pointer, f.Task, rel, anchor, false)
		s.Tier = f.taskTier(rel)
	}
	if err != nil {
		return s, err
	}
	if f.cache == nil {
		f.cache = map[string]Source{}
	}
	f.cache[pointer] = s
	return s, nil
}

// taskTier names who wrote a file in the task folder. The owner's words are the
// owner's tier, the plan is the planner's, and anything else (research, pasted
// input) is a file the Job did not author.
func (f *Files) taskTier(rel string) contract.TrustTier {
	switch filepath.ToSlash(filepath.Clean(rel)) {
	case "intent.md":
		if f.Owner != "" {
			return f.Owner
		}
	case "plan.md":
		return contract.Agent
	}
	return contract.External
}

// brain reads one accepted note through the brain store. A note at the home level
// is personal: it is checked to exist and nothing of it is kept.
func (f *Files) brain(pointer, rel, anchor string) (Source, error) {
	level, name, ok := ParseBrain(rel)
	if !ok {
		return Source{}, fmt.Errorf("%s: expected brain:<home|project>/<name>", pointer)
	}
	if f.Notes == nil {
		return Source{}, fmt.Errorf("%s: no brain store is configured", pointer)
	}
	e, err := f.Notes.Show("brain:" + level + "/" + name)
	if err != nil {
		return Source{}, fmt.Errorf("%s: %w", pointer, err)
	}
	n := e.Note
	if n.Status != "accepted" {
		return Source{}, fmt.Errorf("%s: note is %s, not accepted", pointer, n.Status)
	}
	if n.SupersededBy != "" {
		return Source{}, fmt.Errorf("%s: note is superseded by %s", pointer, n.SupersededBy)
	}
	s := Source{Pointer: pointer, Tier: contract.TrustTier(n.Origin.Tier), State: n.Status}
	if level == "home" {
		s.PointerOnly = true
		return s, nil
	}
	s.Topic, s.Summary, s.Updated = n.Topic, n.Summary, n.Updated
	for _, l := range n.Links {
		s.Links = append(s.Links, Link{To: l.To, Rel: l.Rel, Why: l.Why})
	}
	s.Text = strings.TrimRight(n.Body, "\n")
	if anchor != "" {
		if s.Text, err = selectText(n.Body, anchor); err != nil {
			return s, fmt.Errorf("%s: %w", pointer, err)
		}
	}
	canonical, err := brain.Encode(n)
	if err != nil {
		return s, fmt.Errorf("%s: %w", pointer, err)
	}
	sum := sha256.Sum256(canonical)
	s.Digest = hex.EncodeToString(sum[:])
	return s, nil
}

// file reads one regular file under root. Repo and prototype files must also lie
// inside the project's folders; the tier of such a file is external.
func (f *Files) file(pointer, root, rel, anchor string, project bool) (Source, error) {
	s := Source{Pointer: pointer, Tier: contract.External}
	path, err := Within(root, rel)
	if err != nil {
		return s, fmt.Errorf("%s: %w", pointer, err)
	}
	if project && !inside(f.Folders, path) {
		return s, fmt.Errorf("%s: source is outside the project folders", pointer)
	}
	file, err := openSource(path)
	if err != nil {
		return s, fmt.Errorf("%s: cannot read source: %w", pointer, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return s, fmt.Errorf("%s: cannot read source: not a regular file", pointer)
	}
	isPlan := false
	if f.Task != "" {
		if planInfo, err := os.Stat(filepath.Join(f.Task, "plan.md")); err == nil {
			isPlan = os.SameFile(info, planInfo)
		}
	}
	// Identify the file before using the cache: every plan anchor, including
	// aliases, must recheck the current snapshot and receipt on every read.
	if !isPlan || anchor == "" {
		if cached, ok := f.cache[pointer]; ok {
			return cached, nil
		}
	}
	if f.afterIdentify != nil {
		f.afterIdentify()
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return s, fmt.Errorf("%s: cannot read source: %w", pointer, err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if isPlan && anchor != "" {
		// Validate the bytes we will select from, not a second read of plan.md.
		if !task.ApprovedDigest(f.Task, plan.Hash(string(b)), plan.Digest(string(b))) {
			return s, fmt.Errorf("%s: %w", pointer, ErrPlanNeedsApproval)
		}
	}
	// Only the task folder's own Markdown carries front matter worth reading; a repo
	// file that starts with --- (a YAML file) is plain text.
	if !project && strings.HasPrefix(text, "---\n") {
		d, err := frontmatter.Parse(text)
		if err != nil {
			return s, fmt.Errorf("%s: %w", pointer, err)
		}
		s.Summary, _ = d.Data["summary"].(string)
		if s.Summary == "" {
			s.Summary, _ = d.Data["title"].(string)
		}
		s.State, _ = d.Data["status"].(string)
		s.Updated, _ = d.Data["updated"].(string)
		if anchor == "" {
			s.Text = strings.TrimRight(d.Body, "\n")
		}
	}
	if anchor != "" {
		if isPlan && (strings.HasPrefix(anchor, "step-") || anchor == "verify") {
			s.Text, err = plan.Anchor(text, anchor)
		} else {
			var first, last int
			first, last, err = selectRange(text, anchor)
			if err == nil {
				if isPlan && !plan.DigestCoversLines(text, first+1, last) {
					return s, fmt.Errorf("%s: anchor selects lines outside the approval digest", pointer)
				}
				s.Text = selectedText(text, first, last, anchor)
			}
		}
		if err != nil {
			return s, fmt.Errorf("%s: %w", pointer, err)
		}
	} else if s.Text == "" {
		s.Text = strings.TrimRight(text, "\n")
	}
	if s.Summary == "" {
		s.Summary = firstLine(s.Text)
	}
	sum := sha256.Sum256(b)
	s.Digest = hex.EncodeToString(sum[:])
	return s, nil
}

func taskNumber(path string) string {
	parts := strings.Split(filepath.Base(path), "-")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Within resolves rel below root and refuses a path that leaves root, by name or
// through a symbolic link.
func Within(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("source root is not configured")
	}
	if filepath.IsAbs(rel) || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", fmt.Errorf("source paths must stay inside their root")
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(real, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	if !inside([]string{real}, path) {
		return "", fmt.Errorf("source escapes its root")
	}
	return path, nil
}

// inside reports whether path (symbolic links already resolved) is at or below
// one of dirs. It compares the files themselves, not their spelling: on a
// case-insensitive file system two spellings of one folder match, and on a
// case-sensitive one two folders that differ only in case do not.
func inside(dirs []string, path string) bool {
	var roots []os.FileInfo
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil {
			roots = append(roots, info)
		}
	}
	for p := path; ; {
		if info, err := os.Stat(p); err == nil {
			for _, r := range roots {
				if os.SameFile(r, info) {
					return true
				}
			}
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}
func selectText(text, anchor string) (string, error) {
	first, last, err := selectRange(text, anchor)
	if err != nil {
		return "", err
	}
	return selectedText(text, first, last, anchor), nil
}

func selectedText(text string, first, last int, anchor string) string {
	ls := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	selected := strings.Join(ls[first:last], "\n")
	if !lineAnchor.MatchString(anchor) {
		selected = strings.TrimRight(selected, "\n")
	}
	return selected
}

func selectRange(text, anchor string) (int, int, error) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if m := lineAnchor.FindStringSubmatch(anchor); m != nil {
		a, _ := strconv.Atoi(m[1])
		b := a
		if m[2] != "" {
			b, _ = strconv.Atoi(m[2])
		}
		if a < 1 || b < a || b > len(lines) {
			return 0, 0, fmt.Errorf("range outside file (1-%d)", len(lines))
		}
		return a - 1, b, nil
	}
	start, end := -1, len(lines)
	level := 0
	for i, l := range lines {
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
		slug = slugDrop.ReplaceAllString(slug, "")
		slug = strings.ReplaceAll(slug, " ", "-")
		if slug == anchor || strings.HasSuffix(heading, "{#"+anchor+"}") {
			if start >= 0 {
				return 0, 0, fmt.Errorf("ambiguous anchor %s", anchor)
			}
			start = i
			level = n
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("anchor %q not found", anchor)
	}
	return start, end, nil
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

// LoadPlan selects exactly one planned Job by item number or Job id.
func LoadPlan(dir, selector string) (Job, error) {
	path, err := Within(dir, "plan.md")
	if err != nil {
		return Job{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Job{}, err
	}
	text := string(b)
	if !task.ApprovedDigest(dir, plan.Hash(text), plan.Digest(text)) {
		return Job{}, ErrPlanNeedsApproval
	}
	items, err := plan.Items(text)
	if err != nil {
		return Job{}, err
	}
	matches := []Job{}
	for _, item := range items {
		for _, raw := range item.Jobs {
			j, problems := DecodePlanned(raw, item.Kind)
			if strconv.Itoa(j.Item) != selector && j.ID != selector {
				continue
			}
			if len(problems) > 0 {
				return Job{}, fmt.Errorf("plan.md Job %s: %s", selector, strings.Join(problems, "; "))
			}
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
	for _, key := range []string{"kind", "visible", "irreversible", "review_reason"} {
		if value, ok := m[key]; ok {
			fmt.Fprintf(&out, "%s: %s\n", key, value)
		}
	}
	for _, key := range []string{"id", "item", "title", "role", "skills", "review", "risk", "max_rounds", "intent", "links", "keep", "decisions", "read", "write", "out_of_scope", "acceptance", "verify", "budget", "on_blocked", "after", "attempt", "continues", "discovered_from", "close_reason", "checkpoint", "review_rounds", "level_raised", "worktree", "posts_back", "status", "follow_up"} {
		fmt.Fprintf(&out, "%s: %s\n", key, m[key])
	}
	out.WriteString("---\n")
	return out.String(), nil
}
