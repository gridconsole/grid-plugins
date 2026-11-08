package jobcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func traceWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func traceTask(t *testing.T, home, project, id string) string {
	t.Helper()
	dir := filepath.Join(gridhome.TasksDir(home, project), id+"-example")
	// This task predates owner, owner_role, gate and workflow fields.
	traceWrite(t, filepath.Join(dir, "task.md"), []byte(fmt.Sprintf("---\nid: %s\ntitle: Example\nproject: %s\nstate: open\ncreated: 2026-01-01T00:00:00Z\n---\n", id, project)))
	traceWrite(t, filepath.Join(dir, "intent.md"), []byte("Example intent\n"))
	return dir
}

func traceTestBundle(t *testing.T, dir, id string, sources ...job.Source) string {
	t.Helper()
	path := filepath.Join(dir, "jobs", id)
	b, err := json.Marshal(job.Inputs{Generation: "old-generation", Sources: sources})
	if err != nil {
		t.Fatal(err)
	}
	traceWrite(t, filepath.Join(path, "inputs.json"), b)
	traceWrite(t, filepath.Join(path, "job.yaml"), []byte(`{"id":"`+id+`"}`))
	return path
}

func traceCall(t *testing.T, home string, want int, args ...string) (map[string]json.RawMessage, string, string) {
	t.Helper()
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	code := Run(append([]string{"trace"}, args...), Env{Resolver: ownerResolver(home), GridHome: home, Stdout: out, Stderr: errs})
	if code != want {
		t.Fatalf("trace %v: exit %d, want %d; stdout=%s stderr=%s", args, code, want, out, errs)
	}
	var result map[string]json.RawMessage
	if want == 0 {
		for _, arg := range args {
			if arg == "--json" {
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
	}
	return result, out.String(), errs.String()
}

func traceTestRows(t *testing.T, home string, args ...string) []map[string]string {
	t.Helper()
	result, _, _ := traceCall(t, home, 0, append(args, "--json")...)
	var rows []map[string]string
	if err := json.Unmarshal(result["rows"], &rows); err != nil {
		t.Fatal(err)
	}
	if string(result["problems"]) != "[]" {
		t.Fatalf("unexpected problems: %s", result["problems"])
	}
	return rows
}

func TestJobTraceByBrainID(t *testing.T) {
	home := t.TempDir()
	traceTestBundle(t, traceTask(t, home, "alpha", "T-01"), "J-1", job.Source{Pointer: "brain:project/x", Digest: strings.Repeat("a", 64), Tier: contract.Agent, Updated: "2026-01-02"})
	traceTestBundle(t, traceTask(t, home, "alpha", "T-02"), "J-2", job.Source{Pointer: "brain:home/x"})
	rows := traceTestRows(t, home, "brain:project/x")
	if len(rows) != 1 || rows[0]["task"] != "T-01" || rows[0]["job"] != "J-1" || rows[0]["project"] != "alpha" || rows[0]["updated"] != "2026-01-02" || rows[0]["tier"] != "agent" {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestTraceCallerContext(t *testing.T) {
	home := t.TempDir()
	for _, project := range []string{"alpha", gridhome.Personal} {
		traceTestBundle(t, traceTask(t, home, project, "T-01"), "J-1", job.Source{Pointer: "brain:project/x"})
	}
	for _, tc := range []struct {
		role string
		want int
	}{
		{"", 0},
		{" \t\n", 0},
		{"owner", 2},
		{"member", 1},
		{"guest", 1},
	} {
		t.Run("role="+tc.role, func(t *testing.T) {
			resolver := where.Resolver{GridHome: home, Caller: where.Caller{Role: tc.role, Member: "sam"}}
			result, err := Trace(resolver, "brain:project/x", "", "")
			if strings.TrimSpace(tc.role) == "" {
				if err == nil || !strings.Contains(err.Error(), "missing member context") {
					t.Fatalf("expected missing member context, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(result.Rows) != tc.want {
				t.Fatalf("got %d rows, want %d: %+v", len(result.Rows), tc.want, result)
			}
		})
	}
}

func TestJobTraceBySHA(t *testing.T) {
	home := t.TempDir()
	dir := traceTask(t, home, "alpha", "T-01")
	for i, d := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		traceTestBundle(t, dir, fmt.Sprintf("J-%d", i+1), job.Source{Pointer: "brain:project/x", Digest: d})
	}
	rows := traceTestRows(t, home, "brain:project/x", "--sha256", strings.Repeat("b", 64))
	if len(rows) != 1 || rows[0]["job"] != "J-2" {
		t.Fatalf("rows: %+v", rows)
	}
	if rows := traceTestRows(t, home, "brain:project/x", "--sha256", strings.Repeat("c", 64)); len(rows) != 0 {
		t.Fatal(rows)
	}
}

func TestJobTraceIgnoresAnchor(t *testing.T) {
	for _, pointer := range []string{"brain:project/x#section", "brain:project/x:2-4"} {
		t.Run(pointer, func(t *testing.T) {
			home := t.TempDir()
			traceTestBundle(t, traceTask(t, home, "alpha", "T-01"), "J-1", job.Source{Pointer: pointer})
			rows := traceTestRows(t, home, "brain:project/x#another")
			if len(rows) != 1 || rows[0]["pointer"] != pointer {
				t.Fatal(rows)
			}
		})
	}
}

func TestJobTraceProjectFilter(t *testing.T) {
	home := t.TempDir()
	for i, proj := range []string{"alpha", "beta", gridhome.Personal} {
		traceTestBundle(t, traceTask(t, home, proj, fmt.Sprintf("T-%02d", i+1)), "J-1", job.Source{Pointer: "brain:home/x"})
	}
	traceWrite(t, filepath.Join(home, "projects", "alpha", "project.md"), []byte("---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n---\n"))
	for _, filter := range []string{"alpha", "01ARZ3NDEKTSV4RRFFQ69G5FAV", gridhome.Personal} {
		rows := traceTestRows(t, home, "brain:home/x", "--project", filter)
		want := "alpha"
		if filter == gridhome.Personal {
			want = filter
		}
		if len(rows) != 1 || rows[0]["project"] != want {
			t.Fatal(rows)
		}
	}
	traceCall(t, home, 1, "brain:home/x", "--project", "unknown", "--json")
}

func TestJobTraceGuestProjects(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"alpha", "beta", gridhome.Personal} {
		traceTestBundle(t, traceTask(t, home, name, "T-01"), "J-1", job.Source{Pointer: "shared.md"})
		traceWrite(t, filepath.Join(home, "projects", name, "project.md"), []byte("---\nid: id-"+name+"\n---\n"))
	}
	traceWrite(t, filepath.Join(home, "projects", "alpha", "project-roles.json"), []byte(`{"people":[{"id":"gus","perm":"Client"}]}`))
	for _, tc := range []struct {
		role, member string
		want         int
	}{
		{"owner", "gus", 4}, {"member", "gus", 2},
		{"guest", "gus", 1}, {"guest", "nobody", 0}, {"guest", "", 0},
	} {
		t.Run(tc.role+"/"+tc.member, func(t *testing.T) {
			r := where.Resolver{GridHome: home, Caller: where.Caller{Role: tc.role, Member: tc.member}}
			got, err := Trace(r, "shared.md", "", "")
			if err != nil || len(got.Rows) != tc.want || len(got.Problems) != 0 {
				t.Fatalf("trace rows = %d, want %d; problems=%d error=%v", len(got.Rows), tc.want, len(got.Problems), err)
			}
			if tc.role != "guest" {
				return
			}
			for _, row := range got.Rows {
				if row.Project != "alpha" {
					t.Errorf("guest saw project %q", row.Project)
				}
			}
			for _, selector := range []string{"beta", "id-beta", "unknown"} {
				got, err := Trace(r, "shared.md", "", selector)
				if err != errTraceProjectNotFound || len(got.Rows) != 0 || len(got.Problems) != 0 {
					t.Errorf("selector %q: expected project not found, got rows=%d error=%v", selector, len(got.Rows), err)
				}
			}
			if tc.member == "gus" {
				for _, selector := range []string{"alpha", "id-alpha"} {
					got, err := Trace(r, "shared.md", "", selector)
					if err != nil || len(got.Rows) != 1 || got.Rows[0].Project != "alpha" {
						t.Errorf("invited selector %q: rows=%d error=%v", selector, len(got.Rows), err)
					}
				}
			}
		})
	}
}

func TestJobTraceBadBundle(t *testing.T) {
	home := t.TempDir()
	dir := traceTask(t, home, "alpha", "T-01")
	good := traceTestBundle(t, dir, "J-1", job.Source{Pointer: "brain:project/x"})
	traceWrite(t, filepath.Join(dir, "jobs", "J-2", "inputs.json"), []byte("{broken"))
	// A link to a bundle within the task must also be refused.
	if err := os.Symlink(good, filepath.Join(dir, "jobs", "J-3")); err != nil {
		t.Fatal(err)
	}
	result, out, _ := traceCall(t, home, 0, "brain:project/x", "--json")
	var rows, problems []map[string]string
	if err := json.Unmarshal(result["rows"], &rows); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(result["problems"], &problems); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(problems) != 2 || problems[0]["job"] != "J-2" || problems[1]["job"] != "J-3" {
		t.Fatalf("%s", out)
	}
	if strings.Contains(out, home) {
		t.Fatal("home path leaked")
	}
	_, plain, _ := traceCall(t, home, 0, "brain:project/x")
	if lines := strings.Split(strings.TrimSpace(plain), "\n"); len(lines) != 3 || !strings.Contains(lines[0], "J-1") || !strings.Contains(lines[1], "J-2") || !strings.Contains(lines[2], "J-3") {
		t.Fatal(plain)
	}
}

func TestJobTraceOldBundle(t *testing.T) {
	home := t.TempDir()
	dir := traceTask(t, home, "alpha", "T-01")
	b, err := os.ReadFile(filepath.Join("testdata", "trace-old-inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, "jobs", "J-1")
	traceWrite(t, filepath.Join(bundle, "inputs.json"), b)
	filed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(bundle, filed, filed); err != nil {
		t.Fatal(err)
	}
	rows := traceTestRows(t, home, "brain:project/x")
	if len(rows) != 1 || rows[0]["filed"] != contract.FormatTime(filed) {
		t.Fatal(rows)
	}
}

func TestJobTraceCLI(t *testing.T) {
	home := t.TempDir()
	result, _, _ := traceCall(t, home, 0, "brain:project/x", "--json")
	if len(result) != 2 || string(result["rows"]) != "[]" || string(result["problems"]) != "[]" {
		t.Fatal(result)
	}
	for _, args := range [][]string{{}, {"--json"}, {"a", "b"}, {"a", "--sha256"}, {"a", "--project"}, {"a", "--unknown"}, {"brain:wrong/x"}, {"a", "--sha256", "bad"}, {"a", "--task", "T-01"}} {
		traceCall(t, home, 2, args...)
	}
	traceCall(t, home, 0, "--help")
}

func TestJobTraceExactPathAndSort(t *testing.T) {
	home := t.TempDir()
	for _, proj := range []string{"beta", "alpha"} {
		for _, id := range []string{"T-02", "T-01"} {
			dir := traceTask(t, home, proj, id)
			for _, j := range []string{"J-2", "J-1"} {
				traceTestBundle(t, dir, j, job.Source{Pointer: "repo:docs/x.md#title"}, job.Source{Pointer: "repo:docs/x.md:1-2"}, job.Source{Pointer: "repo:docs/x.md.bak"}, job.Source{Pointer: "docs/x.md"})
			}
		}
	}
	rows := traceTestRows(t, home, "repo:docs/x.md#other")
	if len(rows) != 16 {
		t.Fatal(rows)
	}
	previous := ""
	for _, row := range rows {
		key := row["project"] + row["task"] + row["job"]
		if key < previous || strings.Contains(row["pointer"], "bak") {
			t.Fatal(rows)
		}
		previous = key
	}
}

func TestJobTraceUnsafePaths(t *testing.T) {
	for _, kind := range []string{"escaping-bundle", "linked-inputs", "escaping-jobs", "dangling-jobs", "linked-task"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			good := traceTask(t, home, "alpha", "T-01")
			traceTestBundle(t, good, "J-1", job.Source{Pointer: "brain:project/x"})
			bad := traceTask(t, home, "alpha", "T-02")
			outside := filepath.Join(home, "outside")
			traceWrite(t, filepath.Join(outside, "inputs.json"), []byte(`{"sources":[{"pointer":"brain:project/x"}]}`))
			var from, to string
			switch kind {
			case "escaping-bundle":
				from, to = outside, filepath.Join(bad, "jobs", "J-2")
			case "linked-inputs":
				from, to = filepath.Join(outside, "inputs.json"), filepath.Join(bad, "jobs", "J-2", "inputs.json")
			case "escaping-jobs":
				from, to = outside, filepath.Join(bad, "jobs")
			case "dangling-jobs":
				from, to = filepath.Join(home, "absent"), filepath.Join(bad, "jobs")
			case "linked-task":
				from, to = good, filepath.Join(filepath.Dir(bad), "T-03-link")
			}
			if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(from, to); err != nil {
				t.Fatal(err)
			}
			result, err := Trace(ownerResolver(home), "brain:project/x", "", "")
			if err != nil || len(result.Rows) != 1 || len(result.Problems) != 1 {
				t.Fatalf("%+v, %v", result, err)
			}
			b, err := json.Marshal(result.Problems)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), home) {
				t.Fatal("home path leaked")
			}
		})
	}
}

func TestJobTraceSavedBundleReadOnly(t *testing.T) {
	home := t.TempDir()
	dir := traceTask(t, home, "alpha", "T-01")
	source := job.Source{Pointer: "brain:project/x", Digest: strings.Repeat("a", 64), Tier: contract.PersonTeam}
	rendered := job.Rendered{Brief: "A saved brief", Inputs: job.Inputs{Sources: []job.Source{source}, NotRendered: []string{source.Pointer}, BrainTokens: 100}}
	brief, err := save(dir, job.Job{ID: "J-1"}, rendered)
	if err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, name := range []string{"brief.md", "inputs.json", "job.yaml"} {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(brief), name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = b
	}
	// The live brain entry need not exist, and omitted render candidates still
	// appear in saved sources. Neither is a reason to discard historical evidence.
	result, err := Trace(ownerResolver(home), source.Pointer, source.Digest, "")
	if err != nil || len(result.Rows) != 1 || len(result.Problems) != 0 {
		t.Fatalf("%+v, %v", result, err)
	}
	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(filepath.Dir(brief), name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("changed %s", name)
		}
	}
}

func TestJobTraceHomeAlias(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "real")
	traceTestBundle(t, traceTask(t, home, "alpha", "T-01"), "J-1", job.Source{Pointer: "brain:project/x"})
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(home, alias); err != nil {
		t.Fatal(err)
	}
	rows := traceTestRows(t, alias, "brain:project/x")
	if len(rows) != 1 {
		t.Fatal(rows)
	}
}

func TestJobTraceInvalidInputs(t *testing.T) {
	home := t.TempDir()
	dir := traceTask(t, home, "alpha", "T-01")
	for i, data := range []string{"null", "[]", `{"sources":"wrong"}`, `{} {}`} {
		traceWrite(t, filepath.Join(dir, "jobs", fmt.Sprintf("J-%d", i+1), "inputs.json"), []byte(data))
	}
	traceWrite(t, filepath.Join(dir, "jobs", ".render-in-progress", "inputs.json"), []byte("unfinished"))
	result, err := Trace(ownerResolver(home), "brain:project/x", "", "")
	if err != nil || len(result.Rows) != 0 || len(result.Problems) != 4 {
		t.Fatalf("%+v, %v", result, err)
	}
}

func TestTracePersonalCallerPolicy(t *testing.T) {
	home := t.TempDir()
	source := job.Source{Pointer: "brain:home/private"}
	traceTestBundle(t, traceTask(t, home, "personal", "T-01"), "J-1", source)
	for _, role := range []string{"owner", "member", "guest", "unknown"} {
		t.Run(role, func(t *testing.T) {
			resolver := where.Resolver{GridHome: home, Caller: where.Caller{Role: role, Member: "gus"}}
			result, err := Trace(resolver, source.Pointer, "", "")
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if role == "owner" {
				want = 1
			}
			if len(result.Rows) != want || len(result.Problems) != 0 {
				t.Fatalf("rows=%d, problems=%d, want rows=%d and no problems", len(result.Rows), len(result.Problems), want)
			}
		})
	}
}
