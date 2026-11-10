package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// --grid-home sets the Grid home with no environment variable, and a flag wins over
// the variable; the variable alone still works.
func TestGridHomeFlag(t *testing.T) {
	h, task := feedHarness(t)
	noHome := h.env
	noHome.GridHome, noHome.Machine, noHome.Cwd = "", "", task
	var out, errb strings.Builder
	noHome.Stdout, noHome.Stderr = &out, &errb
	if code := Run([]string{"where", "--json"}, noHome); code != 1 {
		t.Fatalf("no home and no flag: want exit 1, got %d", code)
	}
	if code := Run([]string{"--grid-home", h.gridHome, "where", "--json"}, noHome); code != 0 || !strings.Contains(out.String(), "example") {
		t.Fatalf("--grid-home: %d %s %s", code, out.String(), errb.String())
	}
	out.Reset()
	if code := Run([]string{"--grid-home=" + h.gridHome, "where", "--json"}, noHome); code != 0 {
		t.Fatalf("--grid-home=DIR: %d %s", code, errb.String())
	}
	t.Setenv("GRID_HOME", t.TempDir())
	if env := OSEnv(); env.GridHome == "" {
		t.Fatal("GRID_HOME alone no longer sets the Grid home")
	}
	envHome := OSEnv()
	envHome.Cwd, envHome.Stdout, envHome.Stderr = task, &out, &errb
	out.Reset()
	if code := Run([]string{"--grid-home", h.gridHome, "where", "--json"}, envHome); code != 0 || !strings.Contains(out.String(), "example") {
		t.Fatalf("flag did not win over GRID_HOME: %d %s %s", code, out.String(), errb.String())
	}
	if code := Run([]string{"--grid-home"}, envHome); code != 2 {
		t.Fatalf("--grid-home with no value: want exit 2, got %d", code)
	}
}

// A person takes the task lease with `grid lease take task --hold` and no process
// id, names themself with --member, and `grid job next` works with the epoch it
// printed; without a lease, or after release, it does not.
func TestLeaseHoldThenJobNext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the task lease needs a Unix host")
	}
	h, task := feedHarness(t)
	t.Setenv("GRID_MEMBER", "other") // the flags below must win over this
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	who := []string{"--member", "member", "--role", "member"}
	run := func(args ...string) int { return h.run(task, append(append([]string{}, who...), args...)...) }

	// No lease yet: no epoch exists to give, and a made-up one is refused.
	if code := run("job", "next", "--json"); code != 1 || !strings.Contains(h.errb.String(), "grid lease take task") {
		t.Fatalf("job next with no lease: want exit 1 naming the lease, got %d %s", code, h.errb)
	}
	if code := run("--lease-epoch", "0.1", "job", "next", "--json"); code == 0 {
		t.Fatalf("job next passed with a made-up epoch and no lease: %s", h.out)
	}

	// Take it from the CLI; the holder is the --member name, not GRID_MEMBER.
	if code := run("lease", "take", "task", "--hold", "--json"); code != 0 {
		t.Fatalf("lease take --hold: %d %s", code, h.errb)
	}
	var tenure lease.Tenure
	if err := json.Unmarshal(h.out.Bytes(), &tenure); err != nil {
		t.Fatal(err, h.out.String())
	}
	holder := "member@" + feedMachine
	if tenure.Holder == nil || *tenure.Holder != holder || tenure.PID == nil || *tenure.PID < 1 {
		t.Fatalf("tenure: %+v", tenure)
	}
	if code := run("lease", "status", "task", "--json"); code != 0 || !strings.Contains(h.out.String(), `"holder":"`+holder+`"`) {
		t.Fatalf("lease status: %d %s %s", code, h.out, h.errb)
	}
	records, err := store.Read(feed.Filter{Type: "handover"})
	if err != nil || len(records.Records) != 1 || records.Records[0].Worker != holder {
		t.Fatalf("handover record does not name the holder: %+v %v", records, err)
	}
	epoch := fmt.Sprintf("%d.%d", tenure.Gen, tenure.Fence)

	// A second hold is refused while the first stands.
	if code := run("lease", "take", "task", "--hold"); code == 0 {
		t.Fatalf("a second --hold took a held lease: %s", h.out)
	}

	// Post a Job under the lease, then job next assigns it.
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: holder, Lease: "task", Gen: tenure.Gen, Fence: tenure.Fence}
	job := readJobFixture(t)
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: job}, w); err != nil {
		t.Fatal(err)
	}
	if code := run("--lease-epoch", epoch, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"state":"assigned"`) {
		t.Fatalf("job next after lease take: %d %s %s", code, h.out, h.errb)
	}

	// Release it; the same epoch no longer works.
	if code := run("lease", "release", "task", "--epoch", epoch); code != 0 {
		t.Fatalf("lease release: %d %s", code, h.errb)
	}
	if code := run("--lease-epoch", epoch, "job", "next", "--json"); code == 0 {
		t.Fatalf("job next passed after the lease was released: %s", h.out)
	}
	if code := run("lease", "status", "task"); code != 0 || !strings.Contains(h.out.String(), "free") {
		t.Fatalf("lease status after release: %d %s", code, h.out)
	}
}

func readJobFixture(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestLeaseHoldRefusesProcessFlags(t *testing.T) {
	h, task := feedHarness(t)
	if code := h.run(task, "lease", "take", "task", "--hold", "--pid", "1", "--pid-start", "2026-01-01T00:00:00Z"); code != 2 {
		t.Fatalf("--hold with --pid: want exit 2, got %d %s", code, h.errb)
	}
}
