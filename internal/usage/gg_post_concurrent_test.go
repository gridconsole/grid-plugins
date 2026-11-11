package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests post to one task book from several processes at the same moment.
// Each child is this test binary run again on TestGgUsageConcurrentChild. A child
// blocks on its stdin until the parent has started every child and every child
// has said it is waiting, so the writes overlap. Nothing sleeps and nothing is
// timed: the parent holds on exit codes and on pipes closing.

const (
	ggChildEnv   = "GG_USAGE_CHILD"
	ggTaskEnv    = "GG_USAGE_TASK"
	ggRootEnv    = "GG_USAGE_ROOT"
	ggIDEnv      = "GG_USAGE_ID"
	ggPostsEnv   = "GG_USAGE_POSTS"
	ggKeyEnv     = "GG_USAGE_KEY"
	ggDollarsEnv = "GG_USAGE_DOLLARS"

	// ggWaiting is the one line a child writes on stderr when it is blocked on stdin.
	ggWaiting = "GGWAITING"
	// ggResultTag starts the one line a child writes on stdout: its own result.
	ggResultTag = "GGRESULT "
)

// ggHomeVars are the variables no test of this file may leave pointing at a real home.
var ggHomeVars = []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"}

// ggReport is what one child tells the parent about its own posts.
type ggReport struct {
	ID int `json:"id"`
	// Posts is how many Post calls the child made, Booked how many answered AlreadyBooked.
	Posts  int `json:"posts"`
	Booked int `json:"booked"`
	// Dollars and TotalPosts are the totals the child's last Post answered with.
	Dollars    float64 `json:"dollars"`
	TotalPosts int     `json:"totalPosts"`
}

// ggRequireUnder fails the test unless path is inside root.
func ggRequireUnder(t *testing.T, name, path, root string) {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if root == "" || path == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		t.Fatalf("%s is not under the test root", name)
	}
}

// ggIsolate points the four home variables at folders under a fresh temp root.
// It returns the root and the entries every child needs to get the same four.
func ggIsolate(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var env []string
	for _, name := range ggHomeVars {
		dir := filepath.Join(root, "home-"+strings.ToLower(name))
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, dir)
		env = append(env, name+"="+dir)
	}
	ggRequireUnder(t, "HOME", os.Getenv("HOME"), root)
	return root, env
}

// ggStore is a Store on a new task folder under root. Parent posts use a fixed clock; children use the real one.
func ggStore(t *testing.T, root string) Store {
	t.Helper()
	task := filepath.Join(root, "task")
	if err := os.Mkdir(task, 0o700); err != nil {
		t.Fatal(err)
	}
	return Store{TaskPath: task, GridHome: os.Getenv("GRID_HOME"), Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 678000000, time.UTC) }}
}

// ggChild is one running child process.
type ggChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout bytes.Buffer
	// stderr is written by the drain goroutine and read only after drained is closed.
	stderr bytes.Buffer
	// waiting gets true when the child wrote its waiting line, false when stderr ended without it.
	waiting chan bool
	drained chan struct{}
}

// ggOutcome is what the parent saw of one child once it had exited.
type ggOutcome struct {
	ID     int
	Err    error
	Stdout string
	Stderr string
}

// ggGroup is a set of children that are all blocked on their stdin.
type ggGroup struct {
	children []*ggChild
}

// ggStart starts n children, all before any is waited on, and returns once
// every one of them is blocked on its stdin. Extra entries come from env(i).
func ggStart(t *testing.T, s Store, root string, home []string, n int, env func(i int) []string) *ggGroup {
	t.Helper()
	g := &ggGroup{}
	t.Cleanup(func() {
		for _, c := range g.children {
			c.stdin.Close()
			c.cmd.Process.Kill()
			<-c.drained
			c.cmd.Wait()
		}
	})
	for i := 0; i < n; i++ {
		c := &ggChild{waiting: make(chan bool, 1), drained: make(chan struct{})}
		c.cmd = exec.Command(os.Args[0], "-test.run=^TestGgUsageConcurrentChild$")
		c.cmd.Env = append(os.Environ(), home...)
		c.cmd.Env = append(c.cmd.Env, ggChildEnv+"=1", ggTaskEnv+"="+s.TaskPath, ggRootEnv+"="+root, ggIDEnv+"="+strconv.Itoa(i))
		c.cmd.Env = append(c.cmd.Env, env(i)...)
		c.cmd.Stdout = &c.stdout
		stdin, err := c.cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		c.stdin = stdin
		stderr, err := c.cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		g.children = append(g.children, c)
		go func() {
			defer close(c.drained)
			br := bufio.NewReader(stderr)
			line, err := br.ReadString('\n')
			c.waiting <- err == nil && line == ggWaiting+"\n"
			c.stderr.WriteString(line)
			io.Copy(&c.stderr, br)
		}()
	}
	for i, c := range g.children {
		if !<-c.waiting {
			t.Fatalf("child %d did not reach the start barrier", i)
		}
	}
	return g
}

// release lets every child go: it closes all stdin pipes, only after ggStart saw every child waiting.
func (g *ggGroup) release() {
	for _, c := range g.children {
		c.stdin.Close()
	}
}

// wait holds on every child's exit. It never calls t, so a goroutine may run it.
func (g *ggGroup) wait() []ggOutcome {
	out := make([]ggOutcome, len(g.children))
	for i, c := range g.children {
		<-c.drained
		out[i] = ggOutcome{ID: i, Err: c.cmd.Wait()}
		out[i].Stdout, out[i].Stderr = c.stdout.String(), c.stderr.String()
	}
	return out
}

// ggReports fails the test unless every child exited 0 and wrote exactly its own result line.
func ggReports(t *testing.T, outs []ggOutcome) []ggReport {
	t.Helper()
	reports := make([]ggReport, len(outs))
	for i, o := range outs {
		if o.Err != nil {
			t.Fatalf("child %d: %v\nstdout: %s\nstderr: %s", i, o.Err, o.Stdout, o.Stderr)
		}
		var lines []string
		for _, line := range strings.Split(o.Stdout, "\n") {
			if strings.HasPrefix(line, ggResultTag) {
				lines = append(lines, strings.TrimPrefix(line, ggResultTag))
			}
		}
		if len(lines) != 1 {
			t.Fatalf("child %d wrote %d result lines, want 1\nstdout: %s", i, len(lines), o.Stdout)
		}
		if err := json.Unmarshal([]byte(lines[0]), &reports[i]); err != nil {
			t.Fatalf("child %d result line: %v", i, err)
		}
		if reports[i].ID != i {
			t.Fatalf("child %d reported the result of child %d", i, reports[i].ID)
		}
	}
	return reports
}

// ggRun starts n children, releases them together and returns their reports.
func ggRun(t *testing.T, s Store, root string, home []string, n int, env func(i int) []string) []ggReport {
	t.Helper()
	g := ggStart(t, s, root, home, n, env)
	g.release()
	return ggReports(t, g.wait())
}

// ggLog reads usage.jsonl and fails unless it is a run of complete lines, each one valid JSON.
func ggLog(t *testing.T, s Store) (raw []byte, records []Record) {
	t.Helper()
	raw, err := os.ReadFile(usLogPath(s))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("log does not end in a complete line: %d bytes", len(raw))
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i+1, err)
		}
		if r.V != 1 {
			t.Fatalf("line %d: v = %d", i+1, r.V)
		}
		records = append(records, r)
	}
	return raw, records
}

// ggDistinctEnv is the env of a child that posts 10 distinct deltas of 1 dollar each.
func ggDistinctEnv(int) []string { return []string{ggPostsEnv + "=10"} }

// ggDistinctWant fails the test unless the log holds each of the 40 deltas of 4 children once, and Show agrees.
func ggDistinctWant(t *testing.T, s Store) {
	t.Helper()
	_, records := ggLog(t, s)
	if len(records) != 40 {
		t.Fatalf("log has %d lines, want 40", len(records))
	}
	var got, want []uint64
	for _, r := range records {
		if r.Tokens == nil || r.Dollars != 1 || r.Key != "" {
			t.Fatalf("unexpected record: %+v", r)
		}
		got = append(got, *r.Tokens)
	}
	for id := 0; id < 4; id++ {
		for k := 0; k < 10; k++ {
			want = append(want, uint64(id*1000+k))
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("deltas in the log = %v, want %v", got, want)
	}
	sum, err := s.Show()
	if err != nil || sum.Totals.Dollars != 40 || sum.Totals.Posts != 40 {
		t.Fatalf("Show = %+v: %v", sum.Totals, err)
	}
}

// TestGgUsageConcurrentChild is the body of every child. It skips unless a parent started it.
func TestGgUsageConcurrentChild(t *testing.T) {
	if os.Getenv(ggChildEnv) != "1" {
		t.Skip("body of a child process of the concurrent usage tests; a parent test starts it")
	}
	ggRequireUnder(t, "HOME", os.Getenv("HOME"), os.Getenv(ggRootEnv))
	for _, name := range ggHomeVars {
		ggRequireUnder(t, name, os.Getenv(name), os.Getenv(ggRootEnv))
	}
	id, err := strconv.Atoi(os.Getenv(ggIDEnv))
	if err != nil {
		t.Fatal(err)
	}
	posts, err := strconv.Atoi(os.Getenv(ggPostsEnv))
	if err != nil {
		t.Fatal(err)
	}
	dollars := 1.0
	if v := os.Getenv(ggDollarsEnv); v != "" {
		if dollars, err = strconv.ParseFloat(v, 64); err != nil {
			t.Fatal(err)
		}
	}
	key := os.Getenv(ggKeyEnv)
	s := Store{TaskPath: os.Getenv(ggTaskEnv)}
	fmt.Fprintln(os.Stderr, ggWaiting)
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
	rep := ggReport{ID: id}
	for k := 0; k < posts; k++ {
		tokens := uint64(id*1000 + k)
		res, err := s.Post(Input{Job: "J-1", Dollars: &dollars, Tokens: &tokens, Key: key})
		if err != nil {
			t.Fatal(err)
		}
		rep.Posts++
		if res.AlreadyBooked {
			rep.Booked++
		}
		rep.Dollars, rep.TotalPosts = res.Totals.Dollars, res.Totals.Posts
	}
	line, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(ggResultTag + string(line))
}

// Row 1: four processes post 10 deltas each, with no key; every delta is booked once.
func TestGgUsageConcurrentDistinct(t *testing.T) {
	root, home := ggIsolate(t)
	s := ggStore(t, root)
	for i, rep := range ggRun(t, s, root, home, 4, ggDistinctEnv) {
		if rep.Posts != 10 || rep.Booked != 0 {
			t.Fatalf("child %d: %+v", i, rep)
		}
	}
	ggDistinctWant(t, s)
}

// Row 2: eight processes post the same key at once; one books it, seven are told it was booked.
func TestGgUsageConcurrentSameKey(t *testing.T) {
	root, home := ggIsolate(t)
	s := ggStore(t, root)
	const key = "result:gg-concurrent-key"
	reports := ggRun(t, s, root, home, 8, func(int) []string {
		return []string{ggPostsEnv + "=1", ggDollarsEnv + "=3", ggKeyEnv + "=" + key}
	})
	var booked, first int
	for i, rep := range reports {
		booked += rep.Booked
		if rep.Booked == 0 {
			first++
		}
		if rep.Posts != 1 || rep.Dollars != 3 || rep.TotalPosts != 1 {
			t.Fatalf("child %d: %+v", i, rep)
		}
	}
	if first != 1 || booked != 7 {
		t.Fatalf("%d children booked the key and %d were told it was booked, want 1 and 7", first, booked)
	}
	_, records := ggLog(t, s)
	if len(records) != 1 || records[0].Key != key || records[0].Dollars != 3 {
		t.Fatalf("log = %+v", records)
	}
	sum, err := s.Show()
	if err != nil || sum.Totals.Dollars != 3 || sum.Totals.Posts != 1 {
		t.Fatalf("Show = %+v: %v", sum.Totals, err)
	}
}

// Row 3: the log ends in a cut line; four processes post at once; the cut line is repaired once and no delta is lost.
func TestGgUsageConcurrentTornTail(t *testing.T) {
	root, home := ggIsolate(t)
	s := ggStore(t, root)
	for _, d := range []float64{1, 2, 3} {
		if _, err := s.Post(Input{Job: "lead", Dollars: number(d)}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(usLogPath(s))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usLogPath(s), append(raw, []byte(`{"v":1`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if sum, err := s.Show(); err != nil || sum.Totals.Dollars != 6 || sum.Totals.Posts != 3 {
		t.Fatalf("Show before the posts = %+v: %v", sum.Totals, err)
	}
	reports := ggRun(t, s, root, home, 4, func(int) []string { return []string{ggPostsEnv + "=1"} })
	for i, rep := range reports {
		if rep.Posts != 1 || rep.Booked != 0 {
			t.Fatalf("child %d: %+v", i, rep)
		}
	}
	after, records := ggLog(t, s)
	if len(records) != 7 {
		t.Fatalf("log has %d lines, want 3 earlier and 4 new", len(records))
	}
	if !bytes.HasPrefix(after, raw) {
		t.Fatal("the earlier complete lines changed")
	}
	seen := map[uint64]bool{}
	for _, r := range records[3:] {
		if r.Tokens == nil || r.Dollars != 1 || seen[*r.Tokens] {
			t.Fatalf("unexpected new record: %+v", r)
		}
		seen[*r.Tokens] = true
	}
	sum, err := s.Show()
	if err != nil || sum.Totals.Dollars != 10 || sum.Totals.Posts != 7 {
		t.Fatalf("Show = %+v: %v", sum.Totals, err)
	}
}

// Row 4: the parent reads with Show for as long as the four children of row 1 post;
// no Show fails and the total never drops.
func TestGgUsageConcurrentShowWhilePosting(t *testing.T) {
	root, home := ggIsolate(t)
	s := ggStore(t, root)
	g := ggStart(t, s, root, home, 4, ggDistinctEnv)
	finished := make(chan []ggOutcome, 1)
	g.release()
	go func() { finished <- g.wait() }()
	var last float64
	var outs []ggOutcome
	for shows := 0; outs == nil; shows++ {
		sum, err := s.Show()
		if err != nil {
			t.Fatalf("Show %d: %v", shows, err)
		}
		if sum.Totals.Dollars < last {
			t.Fatalf("Show %d: total fell from %v to %v", shows, last, sum.Totals.Dollars)
		}
		last = sum.Totals.Dollars
		select {
		case outs = <-finished:
		default:
			runtime.Gosched()
		}
	}
	for i, rep := range ggReports(t, outs) {
		if rep.Posts != 10 || rep.Booked != 0 {
			t.Fatalf("child %d: %+v", i, rep)
		}
	}
	ggDistinctWant(t, s)
}
