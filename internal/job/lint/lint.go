// Package lint checks legacy brief completeness and the Job contract.
package lint

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
)

type Finding struct {
	Rule     string `json:"rule"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}
type Options struct {
	Resolver job.Resolver
	Repos    []string
}

var sections = []string{"Intent", "Job", "Context", "Out of scope", "Open", "Acceptance", "Outcome", "Verdict", "Follow-up"}

var relations = wordSet("depends-on explains decided-by supersedes duplicates related blocks continues validates same-files uses owns decides approves knows")
var anchorName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var acceptanceID = regexp.MustCompile(`^A[1-9][0-9]*$`)
var words = regexp.MustCompile(`[\pL][\pL\pN_-]*`)
var stopWords = wordSet("a an the and or of to for from in on at with without by as is are be it this that these those its their our your we you they one all only not no new add adds make makes create creates implement implements support supports update updates change changes use uses must should will can do does has have step verify item plan test tests")

type checker struct {
	text     string
	doc      frontmatter.Document
	findings []Finding
}

func (c *checker) add(rule, key, message, fix string) {
	c.addAt(rule, c.line(key), "error", message, fix)
}
func (c *checker) sourceError(rule, key string, err error, fix string) {
	c.add(rule, key, err.Error(), fix)
}
func (c *checker) line(key string) int {
	if n := c.doc.Lines[key]; n > 0 {
		return n
	}
	return 1
}
func (c *checker) addAt(rule string, line int, severity, message, fix string) {
	c.findings = append(c.findings, Finding{rule, line, severity, message, fix})
}

// Check always returns a non-nil, deterministically ordered list. A missing
// source resolver still checks static rules; callers supply one for range and
// scope checks against the original plan text.
func Check(text string, o Options) []Finding {
	c := checker{text: text, findings: []Finding{}}
	d, e := frontmatter.Parse(text)
	c.doc = d
	if e != nil {
		c.add("brief.frontmatter", "", e.Error(), "Add a closed --- frontmatter block using supported YAML or JSON values.")
		return c.findings
	}
	for _, key := range []string{"id", "role", "status", "verify", "follow_up"} {
		if empty(d.Data[key]) {
			c.add("brief.field", key, "Missing or empty frontmatter field "+key, "Fill in "+key+".")
		}
	}
	headings := map[string]bool{}
	fence := ""
	for _, line := range strings.Split(d.Body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			marker := trim[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "##\t") {
			headings[strings.TrimSpace(line[3:])] = true
		}
	}
	for _, s := range sections {
		if !headings[s] {
			c.addAt("brief.section", d.BodyLine, "error", "Missing section ## "+s, "Add ## "+s+"; Outcome, Verdict and Follow-up may be empty.")
		}
	}
	for _, key := range job.PlannedFields {
		if _, ok := d.Data[key]; !ok {
			c.add("job.required", key, "Missing Job field "+key, "Add the planned "+key+" field; use [] for an empty list.")
		}
	}
	j, _, e := job.DecodeBrief(text)
	if e != nil {
		c.add("job.type", "", e.Error(), "Use the field types in the Job contract.")
		return c.finish()
	}
	c.validate(j, o)
	return c.finish()
}
func (c *checker) finish() []Finding {
	sort.SliceStable(c.findings, func(a, b int) bool {
		x, y := c.findings[a], c.findings[b]
		if x.Line != y.Line {
			return x.Line < y.Line
		}
		if x.Rule != y.Rule {
			return x.Rule < y.Rule
		}
		return x.Message < y.Message
	})
	return c.findings
}
func (c *checker) validate(j job.Job, o Options) {
	if !job.ValidID(j.ID) {
		c.add("job.id", "id", "Invalid Job id", "Use J-<number> or a child id such as J-1.2.")
	}
	if j.Item < 1 {
		c.add("job.item", "item", "Item must be a positive plan item number", "Set item to the plan step number.")
	}
	for key, value := range rangeSorted(map[string]string{"title": j.Title, "role": j.Role}) {
		if strings.TrimSpace(value) == "" {
			c.add("job.required", key, "Empty "+key, "Fill in "+key+" from the plan item.")
		}
	}
	for _, risk := range j.Risk {
		if !wordSet("concurrency security data-format cross-platform")[risk] {
			c.add("job.risk", "risk", "Unknown risk "+risk, "Use a risk named in the Job contract.")
		}
	}
	for _, p := range []struct{ key, value string }{{"owner", j.Intent.Owner}, {"item", j.Intent.Item}, {"goal", j.Intent.Goal}} {
		if p.value == "" {
			c.add("job.intent", "intent", "Missing intent."+p.key, "Add the source pointer for intent."+p.key+".")
		} else if !strings.Contains(p.value, "#") {
			c.add("job.intent", "intent", "intent."+p.key+" needs a precise pointer", "Name a line or anchor with #.")
		}
	}
	if j.OnBlocked != "ask-and-continue" && j.OnBlocked != "exit" {
		c.add("job.on-blocked", "on_blocked", "Invalid on_blocked", "Choose ask-and-continue or exit explicitly.")
	}
	if j.Budget.TurnsEstimate < 1 || j.Budget.ReadsBeforeFirstEdit < 1 {
		c.add("job.budget", "budget", "Budget estimates must be positive", "Set turns_estimate and reads_before_first_edit.")
	}
	if j.Verify.Command != "" {
		if j.Verify.Kind != "" {
			c.add("job.verify", "verify", "Verify cannot have both command and kind", "Choose a command or a non-code verification kind.")
		}
		if j.Verify.MinTests < 1 {
			c.add("job.verify-tests", "verify", "A test command must require at least one test", "Set min_tests to a positive test count.")
		}
	} else {
		switch j.Verify.Kind {
		case "file-exists":
			if j.Verify.Path == "" && len(j.Verify.Paths) == 0 {
				c.add("job.verify", "verify", "file-exists needs a path", "Name a task or worktree proof file.")
			}
		case "reviewer-check":
			if !job.ValidID(j.Verify.Job) {
				c.add("job.verify", "verify", "reviewer-check needs a Job id", "Name the Job whose Verdict verifies this work.")
			}
		default:
			c.add("job.verify", "verify", "Missing verification command or kind", "Use a command with min_tests, file-exists, or reviewer-check.")
		}
	}
	if (j.Status == "cancelled" || j.Status == "rejected") && (j.CloseReason == nil || strings.TrimSpace(*j.CloseReason) == "") {
		c.add("job.close-reason", "close_reason", "A closed Job needs a reason", "Record close_reason for cancellation or rejection.")
	}
	if j.Attempt < 0 || j.ReviewRounds < 0 {
		c.add("job.run-count", "attempt", "Run counts cannot be negative", "Use positive attempts and nonnegative review_rounds.")
	}
	for _, v := range []struct {
		key string
		p   *string
	}{{"continues", j.Continues}, {"discovered_from", j.DiscoveredFrom}} {
		if v.p != nil && !job.ValidID(*v.p) {
			c.add("job.relation-id", v.key, "Invalid "+v.key+" Job id", "Use an existing J-<number> id or null.")
		}
	}
	for _, l := range j.Links {
		c.link(l, "links", o)
	}
	for _, group := range []struct {
		key string
		ids []string
	}{{"keep", j.Keep}, {"decisions", j.Decisions}} {
		for _, id := range group.ids {
			if !validBrain(id) {
				c.add("job.brain-id", group.key, "Expected a brain id: "+id, "Use brain:<home|project>/<name>, never copied rule text.")
			}
		}
	}
	decisions := map[string]bool{}
	for _, d := range j.Decisions {
		decisions[d] = true
		name := strings.TrimPrefix(strings.TrimPrefix(d, "brain:project/"), "brain:home/")
		decisions[name] = true
		decisions["decisions/"+name] = true
	}
	ids := map[string]bool{}
	if len(j.Acceptance) == 0 {
		c.add("job.acceptance", "acceptance", "Acceptance must have at least one row", "Add observable A1, A2, ... rows with traces.")
	}
	dropped := false
	for _, a := range j.Acceptance {
		if !acceptanceID.MatchString(a.ID) || ids[a.ID] {
			c.add("job.acceptance-id", "acceptance", "Invalid or duplicate acceptance id "+a.ID, "Give each row a unique A<number> id.")
		}
		ids[a.ID] = true
		if strings.TrimSpace(a.Text) == "" {
			c.add("job.acceptance-text", "acceptance", "Acceptance row "+a.ID+" is empty", "Describe an observable result.")
		}
		if a.Traces != "item" && a.Traces != "owner" && !decisions[a.Traces] {
			c.add("job.acceptance-trace", "acceptance", "Uncited trace on "+a.ID, "Trace to item, owner, or an id in decisions.")
		}
		if a.State != "" && a.State != "not met" && !wordSet("met not-met dropped pending")[a.State] {
			c.add("job.acceptance-state", "acceptance", "Unknown state on "+a.ID, "Use pending, met, not-met or dropped.")
		}
		if a.State == "dropped" {
			if !decisions[a.Traces] {
				c.add("job.narrowing", "acceptance", "Dropped row "+a.ID+" has no decision", "Cite a decisions id for every dropped requirement.")
			} else {
				dropped = true
			}
		}
	}
	for _, d := range j.After {
		if !job.ValidID(d.Job) && !regexp.MustCompile(`^gate:Q-[0-9]+$`).MatchString(d.Job) {
			c.add("job.after", "after", "Invalid dependency "+d.Job, "Use a Job id or gate:Q-<number>.")
		}
		if d.Kind != "" && !wordSet("done failed children-done")[d.Kind] {
			c.add("job.after-kind", "after", "Invalid dependency kind", "Use done, failed or children-done.")
		}
		if d.Job == j.ID {
			c.add("job.after-cycle", "after", "A Job cannot depend on itself", "Remove the self-dependency.")
		}
		if d.Kind != "" && d.Kind != "done" && strings.TrimSpace(d.Why) == "" {
			c.add("job.after-why", "after", "Dependency kind needs a reason", "Explain why this dependency starts the Job.")
		}
	}
	for _, group := range []struct {
		key   string
		paths []string
	}{{"read", j.Read}, {"write", j.Write}, {"out_of_scope", j.OutOfScope}} {
		for _, p := range group.paths {
			c.repo(p, group.key, o.Repos)
		}
	}
	itemText := j.Title
	if o.Resolver != nil {
		for _, p := range []string{j.Intent.Owner, j.Intent.Item, j.Intent.Goal} {
			if p == "" {
				continue
			}
			s, e := o.Resolver.Resolve(p)
			if e != nil {
				c.sourceError("job.pointer", "intent", e, "Repair the pointer to the intended source lines.")
			} else if p == j.Intent.Item {
				itemText = s.Text
			}
		}
		for _, p := range j.Read {
			if !strings.Contains(p, "#") && !regexp.MustCompile(`:[0-9]+-[0-9]+$`).MatchString(p) {
				c.add("job.read-range", "read", "Read pointer has no range: "+p, "Name a line range or an explicit anchor.")
			}
			if _, e := o.Resolver.Resolve(p); e != nil {
				c.sourceError("job.read-range", "read", e, "Update the read range or anchor to match the source.")
			}
		}
		for _, group := range []struct {
			key string
			ids []string
		}{{"keep", j.Keep}, {"decisions", j.Decisions}} {
			for _, p := range group.ids {
				if s, e := o.Resolver.Resolve(p); e != nil {
					c.sourceError("job.pointer", group.key, e, "Use an existing brain entry id.")
				} else if s.PointerOnly {
					c.personal(group.key, p)
				}
			}
		}
	}
	nouns := contentWords(itemText)
	covered := ""
	for _, a := range j.Acceptance {
		covered += " " + a.Text
	}
	acceptWords := contentWords(covered)
	missing := []string{}
	for word := range nouns {
		if !acceptWords[word] {
			missing = append(missing, word)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 && !dropped {
		c.addAt("job.acceptance-coverage", c.line("acceptance"), "warning", "Plan words absent from acceptance: "+strings.Join(missing, ", "), "Cover these requirements or cite a decision that drops them.")
	}
	for _, scope := range j.OutOfScope {
		matches := []string{}
		for word := range contentWords(scope) {
			if nouns[word] {
				matches = append(matches, word)
			}
		}
		sort.Strings(matches)
		if len(matches) > 0 {
			c.add("job.scope-narrowing", "out_of_scope", "Out of scope names plan words: "+strings.Join(matches, ", "), "Revise the plan through a recorded decision; do not exclude a plan requirement in the Job.")
		}
	}
}

func (c *checker) link(l job.Link, key string, o Options) {
	if !relations[l.Rel] {
		c.add("job.link-rel", key, "Unknown relation "+l.Rel, "Use one of the 15 contract relations.")
	}
	if l.Rel == "blocks" || l.Rel == "same-files" {
		c.add("job.derived-link", key, "Relation "+l.Rel+" must be derived by the CLI", "Express dependencies in after and overlapping files in write.")
	}
	if !validBrain(l.To) && !regexp.MustCompile(`^job:(?:T-[0-9]+/)?J-[0-9]+(?:\.[0-9]+)*(?:#[^ ]+)?$`).MatchString(l.To) && !regexp.MustCompile(`^person:[a-zA-Z0-9_-]+$`).MatchString(l.To) {
		c.add("job.link-target", key, "Invalid link target "+l.To, "Use brain:<home|project>/<name>, a job or a person node id.")
	}
	why := strings.TrimSpace(l.Why)
	name := path.Base(strings.Split(l.To, "#")[0])
	n := utf8.RuneCountInString(why)
	if n < 8 || n > 160 || strings.EqualFold(why, "see also") || strings.EqualFold(why, "related") || strings.EqualFold(why, name) || strings.EqualFold(why, strings.ReplaceAll(name, "-", " ")) {
		c.add("job.link-why", key, "Link to "+l.To+" needs a useful reason", "Write 8 to 160 characters explaining what following the link provides.")
	}
	if o.Resolver != nil {
		s, e := o.Resolver.Resolve(l.To)
		if e != nil {
			c.sourceError("job.pointer", key, e, "Repair the link to an existing target.")
		} else if s.PointerOnly {
			c.personal(key, l.To)
		} else if strings.TrimSpace(s.Summary) == "" || utf8.RuneCountInString(s.Summary) > 300 {
			c.add("job.pointer-summary", key, "Target "+l.To+" needs a summary", "Give the target a summary of 1 to 300 characters.")
		}
	}
}

// personal warns that a home note is named in the brief and never copied into it.
func (c *checker) personal(key, id string) {
	c.addAt("job.personal-pointer", c.line(key), "warning", id+" is a personal note; the brief names it and copies none of it", "Put what the Job needs in a project note, or read the note on your own computer.")
}
func (c *checker) repo(p, key string, repos []string) {
	if strings.HasPrefix(p, "brain:") || strings.HasPrefix(p, "task:") || strings.HasPrefix(p, "job:") || strings.HasPrefix(p, "prototype:") {
		return
	}
	p = strings.TrimPrefix(p, "repo:")
	prefix, _, ok := strings.Cut(p, ":")
	if !ok {
		return
	}
	if regexp.MustCompile(`^[0-9]+-[0-9]+$`).MatchString(strings.SplitN(p, ":", 2)[1]) {
		return
	}
	for _, r := range repos {
		if prefix == r {
			return
		}
	}
	c.add("job.repo", key, fmt.Sprintf("Repository %q is not listed", prefix), "Use a repository declared by the task/project.")
}
func validBrain(s string) bool {
	id, anchor, hasAnchor := strings.Cut(s, "#")
	_, _, ok := job.ParseBrain(id)
	return ok && (!hasAnchor || anchorName.MatchString(anchor))
}
func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case bool:
		return !x
	case float64:
		return x == 0
	}
	return false
}
func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}
func contentWords(s string) map[string]bool {
	s = regexp.MustCompile(`\{#[^}]+\}`).ReplaceAllString(s, "")
	m := map[string]bool{}
	for _, w := range words.FindAllString(strings.ToLower(s), -1) {
		if utf8.RuneCountInString(w) > 2 && !stopWords[w] && !strings.HasPrefix(w, "a-") {
			m[w] = true
		}
	}
	return m
}

// rangeSorted provides stable iteration over the few required string fields.
func rangeSorted(m map[string]string) func(func(string, string) bool) {
	return func(yield func(string, string) bool) {
		keys := []string{}
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
func HasErrors(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}
