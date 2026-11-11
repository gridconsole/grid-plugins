package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

const hdRDeployLink = `{"id":"J-3","links":[{"to":"brain:project/deploy#Steps","rel":"uses","why":"Keep deploys the same"}]}`

// hdRJob writes <task>/jobs/<id>/job.yaml with body and returns its path.
func hdRJob(t *testing.T, f *eventFixture, id, body string) string {
	t.Helper()
	dir := filepath.Join(f.dir, "jobs", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "job.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// hdRRead runs a Read of the note's file.
func hdRRead(t *testing.T, f *eventFixture, e brain.Entry) {
	t.Helper()
	in := hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, e.Path)}}
	if out, log, code := f.run(t, "PostToolUse", in); out != "" || log != "" || code != 0 {
		t.Fatalf("read: %q %q %d", out, log, code)
	}
}

// hdRLine is the one `brain read` line of the fixture task for the note id.
func hdRLine(t *testing.T, f *eventFixture, id string) string {
	t.Helper()
	var found []string
	for _, l := range hdSReadLines(t, f) {
		if strings.Contains(l, "· brain read · "+id+" ·") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one line for %s, got %d: %s", id, len(found), historyOf(t, f))
	}
	return found[0]
}

// hdRForms runs the Read path and the show path, each in a fresh fixture that
// setup has prepared, and returns the deploy line each wrote.
func hdRForms(t *testing.T, job string, setup func(t *testing.T, f *eventFixture)) map[string]string {
	t.Helper()
	lines := map[string]string{}
	for _, form := range []string{"read", "show"} {
		f, deploy := showFixture(t)
		if job != "" {
			f.env["GRID_JOB"] = job
		}
		if setup != nil {
			setup(t, f)
		}
		if form == "read" {
			hdRRead(t, f, deploy)
		} else {
			hdSQuiet(t, f, "grid brain show deploy")
		}
		lines[form] = hdRLine(t, f, "brain:project/deploy")
	}
	return lines
}

func TestHdRSessionNamed(t *testing.T) {
	f, _ := showFixture(t)
	hdSQuiet(t, f, "grid brain show deploy")
	if h := historyOf(t, f); strings.Count(h, "· brain read · brain:project/deploy · by claude · via session session-demo") != 1 || strings.Count(h, "brain read") != 1 {
		t.Fatalf("show outside a Job: %s", h)
	}
	if l := hdRLine(t, f, "brain:project/deploy"); strings.Contains(l, " · why ") {
		t.Fatalf("a show has no why: %s", l)
	}
	// A Read names its session the same way.
	g, deploy := showFixture(t)
	hdRRead(t, g, deploy)
	if l := hdRLine(t, g, "brain:project/deploy"); !strings.HasSuffix(l, "· by claude · via session session-demo") {
		t.Fatalf("read outside a Job: %s", l)
	}
	// Two sessions of one task read the same note: two lines that tell them apart.
	h, _ := showFixture(t)
	for _, sid := range []string{"session-a", "session-b"} {
		in := bash("grid brain show deploy")
		in.SessionID = sid
		if out, log, code := h.run(t, "PostToolUse", in); out != "" || log != "" || code != 0 {
			t.Fatalf("%s: %q %q %d", sid, out, log, code)
		}
	}
	for _, want := range []string{"· via session session-a", "· via session session-b"} {
		if !strings.Contains(historyOf(t, h), want) {
			t.Fatalf("missing %q: %s", want, historyOf(t, h))
		}
	}
}

func TestHdRJobLinkWhy(t *testing.T) {
	for form, l := range hdRForms(t, "J-3", func(t *testing.T, f *eventFixture) { hdRJob(t, f, "J-3", hdRDeployLink) }) {
		if !strings.Contains(l, "· via J-3 · why Keep deploys the same") || !strings.HasSuffix(l, "why Keep deploys the same") {
			t.Fatalf("%s in a Job with a link: %s", form, l)
		}
	}
	// A link with no anchor names the note too.
	plain := `{"links":[{"to":"brain:project/deploy","rel":"uses","why":"Same steps"}]}`
	for form, l := range hdRForms(t, "J-3", func(t *testing.T, f *eventFixture) { hdRJob(t, f, "J-3", plain) }) {
		if !strings.HasSuffix(l, "· via J-3 · why Same steps") {
			t.Fatalf("%s with a link without anchor: %s", form, l)
		}
	}
	// The link's why wins over the search words; the other hit keeps them.
	f, _ := showFixture(t)
	f.env["GRID_JOB"] = "J-3"
	hdRJob(t, f, "J-3", hdRDeployLink)
	hdSQuiet(t, f, "grid brain search deploy")
	if l := hdRLine(t, f, "brain:project/deploy"); !strings.HasSuffix(l, "· via J-3 · why Keep deploys the same") || strings.Contains(l, "search: deploy") {
		t.Fatalf("search hit on a linked note: %s", l)
	}
	if l := hdRLine(t, f, "brain:home/x"); !strings.HasSuffix(l, "· via J-3 · why search: deploy") {
		t.Fatalf("search hit on an unlinked note: %s", l)
	}
}

func TestHdRJobWithoutLinkHasNoWhy(t *testing.T) {
	other := `{"links":[{"to":"brain:home/x","rel":"uses","why":"Other note"},{"to":"brain:project/deploy-more","rel":"uses","why":"Near name"},{"to":"plan.md#deploy","rel":"reads","why":"Not a note"}]}`
	for form, l := range hdRForms(t, "J-3", func(t *testing.T, f *eventFixture) { hdRJob(t, f, "J-3", other) }) {
		if !strings.HasSuffix(l, "· by claude · via J-3") || strings.Contains(l, " · why ") {
			t.Fatalf("%s in a Job with no link to the note: %s", form, l)
		}
	}
	f, _ := showFixture(t)
	f.env["GRID_JOB"] = "J-3"
	hdRJob(t, f, "J-3", other)
	hdSQuiet(t, f, "grid brain search deploy")
	if l := hdRLine(t, f, "brain:project/deploy"); !strings.HasSuffix(l, "· via J-3 · why search: deploy") {
		t.Fatalf("search outside any link keeps the words: %s", l)
	}
	// The link to home/x gives that note its why, and only that note.
	if l := hdRLine(t, f, "brain:home/x"); !strings.HasSuffix(l, "· via J-3 · why Other note") {
		t.Fatalf("link to the other note: %s", l)
	}
}

func TestHdRUnreadableJobFileGivesNoWhy(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "job.yaml")
	if err := os.WriteFile(secret, []byte(`{"links":[{"to":"brain:project/deploy","rel":"uses","why":"outside"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// stray writes a job.yaml with a link why "outside" at rel below the task
	// folder, a file a Job id with dot dot parts would reach by name if it were opened.
	stray := func(t *testing.T, f *eventFixture, rel string) {
		t.Helper()
		p := filepath.Join(f.dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"links":[{"to":"brain:project/deploy","rel":"uses","why":"outside"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		job   string
		setup func(t *testing.T, f *eventFixture)
		via   string
		skip  string
	}{
		{name: "missing", job: "J-3", via: "J-3", setup: func(t *testing.T, f *eventFixture) {}},
		{name: "not JSON", job: "J-3", via: "J-3", setup: func(t *testing.T, f *eventFixture) { hdRJob(t, f, "J-3", "links: [not json") }},
		{name: "links of the wrong type", job: "J-3", via: "J-3", setup: func(t *testing.T, f *eventFixture) { hdRJob(t, f, "J-3", `{"links":"brain:project/deploy"}`) }},
		{name: "a folder", job: "J-3", via: "J-3", setup: func(t *testing.T, f *eventFixture) {
			if err := os.MkdirAll(filepath.Join(f.dir, "jobs", "J-3", "job.yaml"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink out of the task", job: "J-3", via: "J-3", skip: "creating a symlink needs privileges on Windows", setup: func(t *testing.T, f *eventFixture) {
			if err := os.MkdirAll(filepath.Join(f.dir, "jobs", "J-3"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(secret, filepath.Join(f.dir, "jobs", "J-3", "job.yaml")); err != nil {
				t.Fatal(err)
			}
		}},
		// jobs/../x/job.yaml is <task>/x/job.yaml.
		{name: "Job id ../x", job: "../x", via: "../x", setup: func(t *testing.T, f *eventFixture) { stray(t, f, "x/job.yaml") }},
		// jobs/../../x/job.yaml is a file beside the task folder, outside it.
		{name: "Job id ../../x", job: "../../x", via: "../../x", setup: func(t *testing.T, f *eventFixture) { stray(t, f, "../x/job.yaml") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.skip != "" && runtime.GOOS == "windows" {
				t.Skip(c.skip)
			}
			for _, form := range []string{"read", "show"} {
				f, deploy := showFixture(t)
				f.env["GRID_JOB"] = c.job
				c.setup(t, f)
				if form == "read" {
					hdRRead(t, f, deploy)
				} else {
					hdSQuiet(t, f, "grid brain show deploy")
				}
				l := hdRLine(t, f, "brain:project/deploy")
				if !strings.HasSuffix(l, "· by claude · via "+c.via) || strings.Contains(l, " · why ") || strings.Contains(historyOf(t, f), "outside") {
					t.Fatalf("%s %s: %s", c.name, form, l)
				}
			}
			// A search keeps its words.
			f, _ := showFixture(t)
			f.env["GRID_JOB"] = c.job
			c.setup(t, f)
			hdSQuiet(t, f, "grid brain search deploy")
			if l := hdRLine(t, f, "brain:project/deploy"); !strings.HasSuffix(l, "· via "+c.via+" · why search: deploy") {
				t.Fatalf("%s search: %s", c.name, l)
			}
		})
	}
}
