package braincmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pbRdRow says what a reader of the 2.2 table must look like now. marker is text
// the file must hold (the new layout's path or id call); old lists the only code
// lines that may still name knowledge/, the key-absent layout's own paths. exempt
// is a reason the row is not this lane's; a file with one is still searched for
// knowledge/ paths in code when old is given.
type pbRdRow struct {
	marker string
	old    []string
	exempt string
}

// pbRdTable has one entry per file the 2.2 plugin table names outside
// internal/brain. A file of the table with no entry here fails the walk, so a
// row added to the doc cannot go unread.
var pbRdTable = map[string]pbRdRow{
	"internal/braincmd/command.go":          {marker: "brain.ValidID"},
	"internal/braincmd/ke_rename.go":        {marker: "brain.ValidID"},
	"internal/braincmd/ke_write.go":         {marker: "brain.ValidID"},
	"internal/braincmd/hy_drop.go":          {marker: "brain.ValidID"},
	"internal/braincmd/cb_author_origin.go": {exempt: "ValidName at :19-24 reads session, job and task ids Grid generates; 2.2 keeps ValidName for names Grid makes"},
	"internal/braincmd/changes.go":          {marker: "brain.ValidID"},
	"internal/job/source.go":                {marker: "brain.ValidID"},
	"internal/workflow/brain.go":            {marker: `"grid/workflows"`, old: []string{`"knowledge"`, `"knowledge/"`}},
	"internal/routine/schedule.go":          {},
	"internal/routine/workflow.go":          {},
	"internal/routine/execute.go":           {},
	"internal/routine/store.go":             {marker: `"grid", "routines"`},
	"internal/people/store.go":              {marker: `"grid", "people"`},
	"internal/projectenv/projectenv.go":     {marker: `"grid", topic`},
	"internal/cli/ln_project_env.go":        {marker: "WithConfig"},
	"internal/land/land.go":                 {marker: `"grid"`, old: []string{`"knowledge"`, "knowledge/deploy/<name>.md"}},
	"internal/land/approval.go":             {marker: "projectTopic"},
	"internal/settings/claude.go":           {marker: "claudeBrainRules", old: []string{`claudeFold("knowledge")`}},
	"internal/settings/budget.go":           {marker: `"grid", "budget"`, old: []string{`"knowledge", "budget"`}},
	"internal/guard/context.go":             {marker: "brainRoot", old: []string{`"knowledge"`}},
	"internal/orchestrator/progress.go":     {marker: `"Proposals"`},
	"contract/formats.md":                   {marker: "Proposals/"},
	"skills/plan-jobs/SKILL.md":             {marker: "with a brain folder set"},
	"agents/library.md":                     {marker: "grid/"},
	"topics/README.md":                      {marker: "grid/"},
	"skills/brain-keeper/SKILL.md":          {exempt: "the brain-keeper skill follows the writers (P-brain-format), not in this brief's Touches"},
	"agents/curator.md":                     {exempt: "the curator agent follows the writers (P-brain-format), not in this brief's Touches"},
	"internal/hook/brain.go":                {exempt: "the hook's read path is the hook lane's, not in this brief's Touches"},
	"internal/jobcmd/command.go":            {exempt: "2.2 says Index() is unchanged"},
	"internal/hook/hook.go":                 {exempt: "2.2 says SessionStart Index() is unchanged in shape"},
	"internal/mdfile/markdown.go":           {exempt: "SplitOptional is the front-matter split P-brain-vault-read landed; it names no brain path"},
	"internal/migrate/plan.go":              {exempt: "read by the migration lane (2.2, section 8)"},
}

var (
	pbRdPath   = regexp.MustCompile(`[A-Za-z0-9_./-]+\.(?:go|md)`)
	pbRdKnowl  = regexp.MustCompile(`knowledge/|"knowledge`)
	pbRdAppRow = regexp.MustCompile(`^App \(`)
)

// pbRdDocPaths reads the files the 2.2 plugin table names, outside internal/brain.
// A bare file name takes the folder of the path before it in the same cell.
func pbRdDocPaths(t *testing.T, root string) []string {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join(root, "docs", "research", "brain-obsidian.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(doc), "### 2.2 Every reader that must change")
	if !ok {
		t.Fatal("docs/research/brain-obsidian.md has no 2.2 section")
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(rest, "\n") {
		if pbRdAppRow.MatchString(line) {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.HasPrefix(strings.TrimSpace(cells[1]), "Where") || strings.HasPrefix(strings.TrimSpace(cells[1]), "---") {
			continue
		}
		dir := ""
		for _, p := range pbRdPath.FindAllString(cells[1], -1) {
			p = strings.TrimRight(p, ".")
			if !strings.Contains(p, "/") {
				p = dir + p
			}
			dir = filepath.ToSlash(filepath.Dir(p)) + "/"
			if strings.HasPrefix(p, "internal/brain/") || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// pbRdCode is the code lines of a Go file: whole-line and trailing // comments dropped.
func pbRdCode(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if i := strings.Index(line, " // "); i >= 0 {
			line = line[:i]
		}
		out = append(out, line)
	}
	return out
}

// TestPbRdReaderTableWalk is row 1: it walks every file of the 2.2 reader table
// outside internal/brain (parsed from the doc, not copied from it) and fails when
// a file has no expectation, when a Go file still builds a knowledge/ path in code
// beyond the key-absent layout's own lines, or when a file lacks the call or text
// of the new layout. The per-package PbRd tests prove each reader finds its file
// on both layouts; this walk keeps a reader from being left out or put back.
func TestPbRdReaderTableWalk(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	paths := pbRdDocPaths(t, root)
	if len(paths) < 25 {
		t.Fatalf("the 2.2 table gave %d files outside internal/brain: %v", len(paths), paths)
	}
	inDoc := map[string]bool{}
	for _, p := range paths {
		inDoc[p] = true
		row, ok := pbRdTable[p]
		if !ok {
			t.Errorf("2.2 names %s, which has no entry in pbRdTable: decide its reader now", p)
			continue
		}
		if row.exempt != "" && row.marker == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		src := string(data)
		if row.marker != "" && !strings.Contains(src, row.marker) {
			t.Errorf("%s: no %q, the reader of the new layout", p, row.marker)
		}
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		hit := map[string]bool{}
		for _, line := range pbRdCode(src) {
			if !pbRdKnowl.MatchString(line) {
				continue
			}
			allowed := false
			for _, ok := range row.old {
				if strings.Contains(line, ok) {
					allowed, hit[ok] = true, true
				}
			}
			if !allowed {
				t.Errorf("%s still builds a knowledge/ brain path: %s", p, strings.TrimSpace(line))
			}
		}
		for _, ok := range row.old {
			if !hit[ok] {
				t.Errorf("%s: pbRdTable allows %q but no code line has it: drop the entry", p, ok)
			}
		}
	}
	for p := range pbRdTable {
		if !inDoc[p] {
			t.Errorf("pbRdTable has %s, which 2.2 does not name: stale entry", p)
		}
	}
}
