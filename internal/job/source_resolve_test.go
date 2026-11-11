package job_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// These tests pin how Files.Resolve turns a Job's source pointers into text, and
// what it refuses. render_test.go covers the happy path through Render; the cases
// here are the refusals and the selections it does not reach.

func srWrite(t *testing.T, dir, name, text string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// srTask makes an empty task folder named T-07, so the task number is 07.
func srTask(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "T-07")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func srRefused(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want an error containing %q, got none", parts)
	}
	for _, p := range parts {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("error %q does not contain %q", err, p)
		}
	}
}

// srNotes is a brain store with canned answers that records what it was asked.
type srNotes struct {
	entries map[string]brain.Entry
	err     error
	asked   []string
}

func (n *srNotes) Show(id string) (brain.Entry, error) {
	n.asked = append(n.asked, id)
	if n.err != nil {
		return brain.Entry{}, n.err
	}
	if e, ok := n.entries[id]; ok {
		return e, nil
	}
	return brain.Entry{}, errors.New("no such note")
}

func srNote(level, name, status, body string) brain.Entry {
	return brain.Entry{ID: "brain:" + level + "/" + name, Note: brain.Note{
		Name: name, Topic: "ops", Type: "convention", Description: "d", Summary: "How " + name + " works",
		Updated: "2026-01-01T00:00:00Z", Source: "fixture", Level: level, Status: status,
		Origin: brain.Origin{Kind: "session", ID: "fixture"}, Body: body,
	}}
}

func TestResolveBrainPointerReadsAnAcceptedNote(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	h.Note("project", "deploy", "conventions", "How we deploy", "# Deploy\nRun the release.\n\n## Rollback\nRevert the tag.\nCheck the health page.\n")
	f := &job.Files{Task: h.Task, Notes: h.Store}

	s, err := f.Resolve("brain:project/deploy")
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "# Deploy\nRun the release.\n\n## Rollback\nRevert the tag.\nCheck the health page." {
		t.Errorf("text %q", s.Text)
	}
	if s.Tier != contract.Agent || s.State != "accepted" || s.Topic != "conventions" || s.Summary != "How we deploy" || s.PointerOnly {
		t.Errorf("%+v", s)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s.Digest) {
		t.Errorf("digest %q is not a sha256", s.Digest)
	}

	// An anchor or a range selects part of the body; the pointer stays as written.
	for pointer, want := range map[string]string{
		"brain:project/deploy#rollback": "## Rollback\nRevert the tag.\nCheck the health page.",
		"brain:project/deploy#L2-L2":    "Run the release.",
		"brain:project/deploy:2-2":      "Run the release.",
	} {
		s, err := f.Resolve(pointer)
		if err != nil || s.Text != want || s.Pointer != pointer || s.Tier != contract.Agent {
			t.Errorf("%s: %+v %v, want text %q", pointer, s, err, want)
		}
	}
	if _, err := f.Resolve("brain:project/deploy#nope"); err == nil || !strings.Contains(err.Error(), `anchor "nope" not found`) {
		t.Errorf("a missing anchor in a note: %v", err)
	}
}

// A note that was only proposed is not in the store's accepted set, so it never reaches a Job.
func TestResolveBrainPointerRefusesAProposalInTheRealStore(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	draft := brain.Note{Name: "draft", Topic: "conventions", Type: "convention", Description: "d", Summary: "A draft", Source: "fixture", Level: "project", Body: "DRAFT-BODY-MARKER\n"}
	if _, err := h.Store.Propose(draft, brain.Origin{Kind: "session", ID: "fixture"}); err != nil {
		t.Fatal(err)
	}
	s, err := (&job.Files{Task: h.Task, Notes: h.Store}).Resolve("brain:project/draft")
	srRefused(t, err, "brain:project/draft")
	if s.Text != "" || strings.Contains(err.Error(), "DRAFT-BODY-MARKER") {
		t.Fatalf("a proposal leaked: %+v %v", s, err)
	}
}

func TestResolveBrainPointerRefusals(t *testing.T) {
	notes := &srNotes{entries: map[string]brain.Entry{
		"brain:project/ok":       srNote("project", "ok", "accepted", "fine\n"),
		"brain:project/proposed": srNote("project", "proposed", "proposed", "PROPOSED-MARKER\n"),
		"brain:project/rejected": srNote("project", "rejected", "rejected", "REJECTED-MARKER\n"),
		"brain:project/old": func() brain.Entry {
			e := srNote("project", "old", "accepted", "OLD-MARKER\n")
			e.Note.SupersededBy = "brain:project/new"
			return e
		}(),
	}}
	f := &job.Files{Task: srTask(t), Notes: notes}

	// A malformed id is refused before the store is asked for anything.
	for _, p := range []string{"brain:team/x", "brain:project/a/b", "brain:project/../x", "brain:project/", "brain:x", "brain:"} {
		s, err := f.Resolve(p)
		srRefused(t, err, p+":", "expected brain:<home|project>/<name>")
		if s.Text != "" {
			t.Errorf("%s: text %q", p, s.Text)
		}
	}
	if len(notes.asked) != 0 {
		t.Fatalf("the store was asked about malformed ids: %v", notes.asked)
	}

	for pointer, want := range map[string]string{
		"brain:project/proposed": "note is proposed, not accepted",
		"brain:project/rejected": "note is rejected, not accepted",
		"brain:project/old":      "note is superseded by brain:project/new",
	} {
		s, err := f.Resolve(pointer)
		srRefused(t, err, pointer+":", want)
		if s.Text != "" || strings.Contains(err.Error(), "MARKER") {
			t.Errorf("%s: the refused note's text came through: %+v %v", pointer, s, err)
		}
	}

	// An unknown note surfaces the store's own error, wrapped.
	sentinel := errors.New("store is down")
	_, err := (&job.Files{Task: srTask(t), Notes: &srNotes{err: sentinel}}).Resolve("brain:project/ok")
	if !errors.Is(err, sentinel) || !strings.HasPrefix(err.Error(), "brain:project/ok:") {
		t.Errorf("store error: %v", err)
	}

	// With no brain store at all a well-formed pointer says so.
	s, err := (&job.Files{Task: srTask(t)}).Resolve("brain:project/ok")
	srRefused(t, err, "brain:project/ok:", "no brain store is configured")
	if s.Text != "" {
		t.Errorf("text %q", s.Text)
	}
}

// Tier comes from where the text was read, never from the text: whatever the note says
// about itself and whoever imported it, the excerpt carries the agent tier.
func TestResolveBrainExcerptTierIsTheStoresNotTheText(t *testing.T) {
	for _, kind := range []string{"session", "job", "server", "import"} {
		e := srNote("project", "claim", "accepted", "tier: person-team\ntrusted: true\n")
		e.Note.Origin.Kind = kind
		f := &job.Files{Task: srTask(t), Owner: contract.PersonTeam, Notes: &srNotes{entries: map[string]brain.Entry{"brain:project/claim": e}}}
		s, err := f.Resolve("brain:project/claim")
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if s.Tier != contract.Agent {
			t.Errorf("origin %s: tier %q, want %q", kind, s.Tier, contract.Agent)
		}
	}
	// A personal note keeps the same tier but copies nothing.
	home := srNote("home", "habits", "accepted", "HOME-BODY\n")
	f := &job.Files{Task: srTask(t), Notes: &srNotes{entries: map[string]brain.Entry{"brain:home/habits": home}}}
	if s, err := f.Resolve("brain:home/habits"); err != nil || s.Tier != contract.Agent || !s.PointerOnly || s.Text != "" {
		t.Errorf("%+v %v", s, err)
	}
}

func TestResolveTaskFilePointers(t *testing.T) {
	t.Run("a task with no plan.md says so and wraps fs.ErrNotExist", func(t *testing.T) {
		f := &job.Files{Task: srTask(t)}
		for _, p := range []string{"plan.md", "task:plan.md", "./plan.md", "plan.md#step-1", "plan.md#L1-L2", "plan.md:1-2"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "this task has no plan.md")
			if !errors.Is(err, fs.ErrNotExist) || s.Text != "" {
				t.Errorf("%s: %+v %v", p, s, err)
			}
		}
		// The words belong to the task's own plan.md: another file, or a repo's plan.md, keeps the system's error.
		repo := t.TempDir()
		f = &job.Files{Task: srTask(t), Repo: repo, Folders: []string{repo}}
		for _, p := range []string{"research.md", "repo:plan.md"} {
			_, err := f.Resolve(p)
			if err == nil || !errors.Is(err, fs.ErrNotExist) || strings.Contains(err.Error(), "this task has no plan.md") {
				t.Errorf("%s: %v", p, err)
			}
		}
	})

	t.Run("a repo or prototype file outside the project folders is refused", func(t *testing.T) {
		root, elsewhere := t.TempDir(), t.TempDir()
		srWrite(t, root, "src/a.go", "package a\n")
		srWrite(t, root, "proto/index.html", "<p>hi</p>\n")
		srWrite(t, root, "other/b.go", "package b\n")
		for _, folders := range [][]string{nil, {elsewhere}} {
			f := &job.Files{Task: srTask(t), Repo: root, Prototype: filepath.Join(root, "proto"), Repos: map[string]string{"side": root}, Folders: folders}
			for _, p := range []string{"repo:src/a.go", "repo:side:other/b.go", "prototype:index.html"} {
				s, err := f.Resolve(p)
				srRefused(t, err, p+":", "source is outside the project folders")
				if s.Text != "" {
					t.Errorf("%s: text %q came through", p, s.Text)
				}
			}
		}
		// The same pointers read once the folder is the project's.
		f := &job.Files{Task: srTask(t), Repo: root, Prototype: filepath.Join(root, "proto"), Repos: map[string]string{"side": root}, Folders: []string{root}}
		for p, want := range map[string]string{"repo:src/a.go": "package a", "repo:side:other/b.go": "package b", "prototype:index.html": "<p>hi</p>"} {
			if s, err := f.Resolve(p); err != nil || s.Text != want || s.Tier != contract.External {
				t.Errorf("%s: %+v %v", p, s, err)
			}
		}
		// A repo root that was never configured is the same refusal from Within.
		_, err := (&job.Files{Task: srTask(t), Folders: []string{root}}).Resolve("repo:src/a.go")
		srRefused(t, err, "source root is not configured")
	})

	t.Run("a directory is not a regular file", func(t *testing.T) {
		root := t.TempDir()
		task := srTask(t)
		srWrite(t, task, "docs/a.md", "a\n")
		srWrite(t, root, "pkg/a.go", "package a\n")
		f := &job.Files{Task: task, Repo: root, Folders: []string{root}}
		for _, p := range []string{"docs", "task:docs", "docs#L1", "repo:pkg", "repo:pkg:1-1"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "not a regular file")
			if s.Text != "" {
				t.Errorf("%s: text %q", p, s.Text)
			}
		}
	})

	t.Run("a symlink out of the task folder is refused by Within", func(t *testing.T) {
		task, away := srTask(t), t.TempDir()
		srWrite(t, away, "secret.md", "SECRET-MARKER\n")
		srWrite(t, away, "dir/secret.md", "SECRET-MARKER\n")
		srWrite(t, task, "inside.md", "inside\n")
		for name, target := range map[string]string{"file.md": filepath.Join(away, "secret.md"), "dir": filepath.Join(away, "dir")} {
			if err := os.Symlink(target, filepath.Join(task, name)); err != nil {
				t.Skipf("symbolic links are not available here: %v", err)
			}
		}
		if err := os.Symlink("inside.md", filepath.Join(task, "alias.md")); err != nil {
			t.Skipf("symbolic links are not available here: %v", err)
		}
		for _, rel := range []string{"file.md", "dir/secret.md"} {
			if p, err := job.Within(task, rel); err == nil || p != "" || !strings.Contains(err.Error(), "source escapes its root") {
				t.Errorf("Within(%s) = %q, %v", rel, p, err)
			}
		}
		f := &job.Files{Task: task}
		for _, p := range []string{"file.md", "task:dir/secret.md", "file.md#L1"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "source escapes its root")
			if strings.Contains(s.Text, "SECRET") || strings.Contains(err.Error(), "SECRET") {
				t.Errorf("%s: the outside file's text came through", p)
			}
		}
		// A link that stays inside the folder is not an escape.
		if s, err := f.Resolve("alias.md"); err != nil || s.Text != "inside" {
			t.Errorf("alias: %+v %v", s, err)
		}
	})
}

// The tier of a task file is who wrote it: the owner's intent.md carries the owner's
// tier, plan.md the agent's, and anything else is a file the Job did not author.
func TestResolveTaskFileTiers(t *testing.T) {
	task := srTask(t)
	for name, text := range map[string]string{"intent.md": "Make it so.\n", "plan.md": "# Plan, rev 1 · 000000\n", "research.md": "Found it.\n", "sub/intent.md": "Nested.\n"} {
		srWrite(t, task, name, text)
	}
	for _, c := range []struct {
		owner   contract.TrustTier
		pointer string
		want    contract.TrustTier
	}{
		{contract.PersonTeam, "intent.md", contract.PersonTeam},
		{contract.PersonGuest, "task:intent.md", contract.PersonGuest},
		{contract.PersonGuest, "./intent.md", contract.PersonGuest},
		{contract.External, "intent.md", contract.External},
		{"", "intent.md", contract.External}, // an unknown owner is external
		{contract.PersonTeam, "plan.md", contract.Agent},
		{contract.PersonTeam, "task:./plan.md", contract.Agent},
		{contract.PersonTeam, "research.md", contract.External},
		{contract.PersonTeam, "sub/intent.md", contract.External}, // only the task's own intent.md is the owner's
	} {
		s, err := (&job.Files{Task: task, Owner: c.owner}).Resolve(c.pointer)
		if err != nil || s.Tier != c.want || s.Text == "" {
			t.Errorf("owner %q %s: tier %q want %q (%v)", c.owner, c.pointer, s.Tier, c.want, err)
		}
	}
}

func TestResolveRangesAndAnchors(t *testing.T) {
	task := srTask(t)
	srWrite(t, task, "notes.md", "one\ntwo\nthree\nfour\nfive\n")
	srWrite(t, task, "crlf.md", "a\r\nb\r\nc\r\n")
	srWrite(t, task, "doc.md", "# Title\nintro\n## Alpha\nalpha body\n### Alpha detail\ndetail\n## Beta {#custom-id}\nbeta body\n## Gamma\ngamma body\n")
	srWrite(t, task, "nested.md", "## Plan\nouter\n### Plan\ninner\n")
	f := &job.Files{Task: task}

	t.Run("selected lines", func(t *testing.T) {
		for pointer, want := range map[string]string{
			"notes.md#L2-L3": "two\nthree",
			"notes.md:2-3":   "two\nthree",
			"notes.md#L2-3":  "two\nthree",
			"notes.md#L4":    "four",
			"notes.md#L1-L5": "one\ntwo\nthree\nfour\nfive",
			"notes.md#L5":    "five",
			"crlf.md#L2":     "b",
		} {
			if s, err := f.Resolve(pointer); err != nil || s.Text != want || s.Pointer != pointer {
				t.Errorf("%s: %+v %v, want %q", pointer, s, err, want)
			}
		}
	})

	t.Run("a range outside the file names the file's size", func(t *testing.T) {
		for _, p := range []string{"notes.md#L9-L12", "notes.md:9-12", "notes.md#L6", "notes.md#L5-L6", "notes.md#L0-L1", "notes.md#L3-L2"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "range outside file (1-5)")
			if s.Text != "" {
				t.Errorf("%s: text %q", p, s.Text)
			}
		}
	})

	t.Run("a heading anchor selects its section to the next heading of the same or a higher level", func(t *testing.T) {
		for pointer, want := range map[string]string{
			"doc.md#title":        "# Title\nintro\n## Alpha\nalpha body\n### Alpha detail\ndetail\n## Beta {#custom-id}\nbeta body\n## Gamma\ngamma body",
			"doc.md#alpha":        "## Alpha\nalpha body\n### Alpha detail\ndetail",
			"doc.md#alpha-detail": "### Alpha detail\ndetail",
			"doc.md#custom-id":    "## Beta {#custom-id}\nbeta body",
			"doc.md#gamma":        "## Gamma\ngamma body",
		} {
			if s, err := f.Resolve(pointer); err != nil || s.Text != want {
				t.Errorf("%s: %q %v, want %q", pointer, s.Text, err, want)
			}
		}
	})

	t.Run("a missing anchor is not found", func(t *testing.T) {
		for _, p := range []string{"doc.md#nope", "doc.md#Alpha", "notes.md#intro"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "not found")
			if s.Text != "" {
				t.Errorf("%s: text %q", p, s.Text)
			}
		}
		// An empty anchor is no anchor: the whole file.
		if s, err := f.Resolve("doc.md#"); err != nil || !strings.HasPrefix(s.Text, "# Title") {
			t.Errorf("doc.md#: %q %v", s.Text, err)
		}
	})

	// Two headings of the same slug at the same level are deliberately not asserted
	// here: selectRange ends the section at the second one (source.go:396) before it
	// can report it as ambiguous (source.go:404-406), so the first is returned with no
	// error. The lane report lists that as a bug found.
	t.Run("an anchor heading that appears twice inside its own section is ambiguous", func(t *testing.T) {
		s, err := f.Resolve("nested.md#plan")
		srRefused(t, err, "nested.md#plan:", "ambiguous anchor plan")
		if s.Text != "" {
			t.Errorf("text %q", s.Text)
		}
	})

	t.Run("a pointer with both a range and an anchor is refused", func(t *testing.T) {
		for _, p := range []string{"notes.md:2-3#intro", "notes.md:2-3#L1-L2", "doc.md:1-2#alpha", "brain:project/x:1-2#a"} {
			s, err := f.Resolve(p)
			srRefused(t, err, p+":", "cannot combine range and anchor")
			if s.Text != "" {
				t.Errorf("%s: text %q", p, s.Text)
			}
		}
	})
}

// plan.md is read under an approval digest: lines the digest leaves out (the status
// prose under "Where it stands") are not a source for a Job.
func TestResolvePlanAnchorMustLieInsideTheApprovalDigest(t *testing.T) {
	text := "# Plan, rev 1 · 000000\n\n- [ ] 1 Do it\n  Detail.\n## Where it stands\nStatus prose the digest leaves out.\n## Verify\nCheck it.\n"
	task := srTask(t)
	srWrite(t, task, "plan.md", text)
	srWrite(t, task, "research.md", text) // the same lines in a file that is not the plan
	f := &job.Files{Task: task}

	// Lines the digest covers are selected as written.
	if s, err := f.Resolve("plan.md#L3-L4"); err != nil || s.Text != "- [ ] 1 Do it\n  Detail." || s.Tier != contract.Agent {
		t.Fatalf("covered lines: %+v %v", s, err)
	}
	if s, err := f.Resolve("plan.md#verify"); err != nil || !strings.Contains(s.Text, "Check it.") {
		t.Fatalf("the verify anchor: %+v %v", s, err)
	}
	// Lines it leaves out are refused, whole or part of the range, by anchor, range or alias.
	if err := os.Symlink("plan.md", filepath.Join(task, "alias.md")); err != nil {
		t.Log("no symbolic links here: alias case skipped:", err)
	}
	pointers := []string{"plan.md#where-it-stands", "plan.md#L5-L6", "plan.md#L6", "plan.md:5-6", "plan.md#L4-L6", "task:./plan.md#L5-L6"}
	if _, err := os.Lstat(filepath.Join(task, "alias.md")); err == nil {
		pointers = append(pointers, "alias.md#L6")
	}
	for _, p := range pointers {
		s, err := f.Resolve(p)
		srRefused(t, err, p+":", "anchor selects lines outside the approval digest")
		if s.Text != "" {
			t.Errorf("%s: text %q came through", p, s.Text)
		}
	}
	// The refusal belongs to plan.md: the same anchor on another file reads.
	if s, err := f.Resolve("research.md#where-it-stands"); err != nil || !strings.Contains(s.Text, "Status prose") {
		t.Errorf("research.md: %+v %v", s, err)
	}
}

func TestResolveJobPointers(t *testing.T) {
	task := srTask(t)
	srWrite(t, task, "jobs/J-1/brief.md", "The brief for J-1.\n")
	srWrite(t, task, "jobs/J-2.1/brief.md", "The brief for J-2.1.\n")
	f := &job.Files{Task: task, Owner: contract.PersonTeam}

	// A Job of this task reads its brief, with the agent tier, with or without the task prefix.
	for pointer, want := range map[string]string{"job:J-1": "The brief for J-1.", "job:T-07/J-1": "The brief for J-1.", "job:T-07/J-2.1": "The brief for J-2.1."} {
		if s, err := f.Resolve(pointer); err != nil || s.Text != want || s.Tier != contract.Agent {
			t.Errorf("%s: %+v %v", pointer, s, err)
		}
	}
	// A Job of another task needs the index, which Files does not have.
	for _, p := range []string{"job:T-02/J-1", "job:T-7/J-1", "job:X-07/J-1", "job:T-08/J-1#L1"} {
		s, err := f.Resolve(p)
		srRefused(t, err, p+":", "cross-task Job requires an index resolver")
		if s.Text != "" {
			t.Errorf("%s: text %q", p, s.Text)
		}
	}
	// This task's prefix with an id that is not a Job id, or an id that tries to leave jobs/.
	for _, p := range []string{"job:T-07/bad id", "job:bad", "job:T-07/J-", "job:T-07/J-1x", "job:T-07/../plan.md", "job:T-07/J-1/../J-2", "job:J-1.", "job:"} {
		s, err := f.Resolve(p)
		srRefused(t, err, p+":", "invalid Job id")
		if s.Text != "" {
			t.Errorf("%s: text %q", p, s.Text)
		}
	}
	// A valid id with no brief is a missing file, not a pointer that resolves.
	if _, err := f.Resolve("job:J-9"); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("job:J-9: %v", err)
	}
}

func TestWithin(t *testing.T) {
	root := t.TempDir()
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	srWrite(t, root, "a/b.md", "b\n")

	if p, err := job.Within(root, "a/b.md"); err != nil || p != filepath.Join(real, "a", "b.md") {
		t.Errorf("a path below the root: %q %v", p, err)
	}
	if p, err := job.Within("", "a/b.md"); err == nil || p != "" || !strings.Contains(err.Error(), "source root is not configured") {
		t.Errorf("an unset root: %q %v", p, err)
	}
	for _, rel := range []string{"../x", "a/../../x", "..", "", "/etc/passwd", filepath.Join(root, "a", "b.md")} {
		if p, err := job.Within(root, rel); err == nil || p != "" || !strings.Contains(err.Error(), "source paths must stay inside their root") {
			t.Errorf("Within(%q) = %q, %v", rel, p, err)
		}
	}
	// A path that stays inside the root but names nothing is the file system's error.
	if _, err := job.Within(root, "a/missing.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file: %v", err)
	}
	if _, err := job.Within(filepath.Join(root, "no-such-root"), "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing root: %v", err)
	}
	// A symbolic link that leaves the root is refused; one that stays inside resolves to its target.
	away := t.TempDir()
	srWrite(t, away, "out.md", "out\n")
	if err := os.Symlink(away, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "inner")); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
	if p, err := job.Within(root, "escape/out.md"); err == nil || p != "" || !strings.Contains(err.Error(), "source escapes its root") {
		t.Errorf("a link out of the root: %q %v", p, err)
	}
	if p, err := job.Within(root, "inner/b.md"); err != nil || p != filepath.Join(real, "a", "b.md") {
		t.Errorf("a link inside the root: %q %v", p, err)
	}
}
