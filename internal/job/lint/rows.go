package lint

import (
	"path"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

var jobMention = regexp.MustCompile(`\bJ-[0-9]+(?:\.[0-9]+)*\b`)
var liveUIRow = regexp.MustCompile(`(?i)\b(?:live (?:dev )?ui|dev ui|installed app|installed build|live dev (?:app|server))\b`)

// rowPath is a repo file path in free text: it has a directory and an extension,
// and may carry a :line or :line-line suffix, which is dropped.
var rowPath = regexp.MustCompile(`(?:^|[\s(\x60"'])([A-Za-z0-9_.*-]+(?:/[A-Za-z0-9_.*-]+)+\.[A-Za-z0-9]{1,8})(?::[0-9]+(?:-[0-9]+)?)?`)

// afterMissing errors on a Job whose acceptance rows or intent name another Job
// that is not in its after list. The Job itself, discovered_from and continues
// are not dependencies and are not flagged.
//
// An orchestrator Job is exempt: its rows name the Jobs it plans or re-plans, which
// it reads and does not wait for.
func (c *checker) afterMissing(j job.Job) {
	if contract.NormalizeRole(j.Role) == "orchestrator" {
		return
	}
	known := map[string]bool{j.ID: true}
	for _, d := range j.After {
		known[d.Job] = true
	}
	for _, p := range []*string{j.DiscoveredFrom, j.Continues} {
		if p != nil {
			known[*p] = true
		}
	}
	text := j.Intent.Owner + " " + j.Intent.Item + " " + j.Intent.Goal
	for _, a := range j.Acceptance {
		text += " " + a.Text
	}
	reported := map[string]bool{}
	for _, id := range jobMention.FindAllString(text, -1) {
		if known[id] || reported[id] {
			continue
		}
		reported[id] = true
		c.add("job.after-missing", "after", "The acceptance or intent relies on "+id+", which is not in after", "Add {\"job\":\""+id+"\"} to after so this Job waits for it, or take "+id+" out of the rows.")
	}
}

// liveUI warns on a builder Job with a row that asks for a check on the live dev UI
// or the installed app: that check needs the change merged and installed first.
func (c *checker) liveUI(j job.Job) {
	if contract.NormalizeRole(j.Role) != "builder" {
		return
	}
	for _, a := range j.Acceptance {
		if liveUIRow.MatchString(a.Text) {
			c.addAt("job.live-ui-row", c.line("acceptance"), "warning", "Row "+a.ID+" asks for a check on the live dev UI or the installed app, which a builder cannot see before the merge", "Post a grid-checker Job after the merge and install for this check instead, and keep the builder's rows to what it can run.")
		}
	}
}

// rowOutsideWrite errors on a row that names a repo file path no write entry covers.
// A Job with no write entry names no files it may change, so it is not checked.
func (c *checker) rowOutsideWrite(j job.Job) {
	var writes []string
	for _, w := range j.Write {
		if strings.HasPrefix(w, "brain:") || strings.HasPrefix(w, "task:") || strings.HasPrefix(w, "job:") || strings.HasPrefix(w, "prototype:") {
			continue
		}
		writes = append(writes, strings.TrimPrefix(w, "repo:"))
	}
	if len(writes) == 0 {
		return
	}
	reported := map[string]bool{}
	for _, a := range j.Acceptance {
		for _, m := range rowPath.FindAllStringSubmatch(a.Text, -1) {
			p := m[1]
			if reported[p] || strings.Contains(p, "*") || coveredByWrite(writes, p) {
				continue
			}
			reported[p] = true
			c.add("job.row-outside-write", "acceptance", "Row "+a.ID+" names "+p+", which no write entry covers", "Add the file to write, or move the row to a Job that writes it.")
		}
	}
}

func coveredByWrite(writes []string, p string) bool {
	for _, w := range writes {
		if w == p {
			return true
		}
		if ok, _ := path.Match(w, p); ok {
			return true
		}
		if pre, _, found := strings.Cut(w, "**"); found {
			if strings.HasPrefix(p, pre) {
				return true
			}
			continue
		}
		if strings.HasPrefix(p, strings.TrimSuffix(w, "/")+"/") {
			return true
		}
	}
	return false
}
