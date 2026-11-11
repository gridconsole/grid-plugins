package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// These tests hold the claim in Store.Add: two writers adding proof to one plan
// item at once keep every record. They wait on exit codes and channels, never
// on a sleep, and assert no timing.

const (
	ggItem    = "1"   // the plan item the tests add to
	ggItemDir = "i-1" // its folder below proof/
	ggMarker  = "GG_PROOF_CHILD"
	ggChild   = "^TestGgProofConcurrentChildHelper$"
)

var ggHomeKeys = []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"}

// ggUnder reports whether p is a folder below root.
func ggUnder(root, p string) bool {
	if root == "" || p == "" {
		return false
	}
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ggRequireHomes fails when any home key points outside root.
func ggRequireHomes(t *testing.T, root string) {
	t.Helper()
	for _, k := range ggHomeKeys {
		if !ggUnder(root, os.Getenv(k)) {
			t.Fatalf("%s is not under the test temp root", k)
		}
	}
}

// ggIsolate points the four home variables at folders under a fresh temp root
// and returns the root and the same four for a child's cmd.Env.
func ggIsolate(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var env []string
	for _, k := range ggHomeKeys {
		dir := filepath.Join(root, "gg-"+strings.ToLower(k))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(k, dir)
		env = append(env, k+"="+dir)
	}
	ggRequireHomes(t, root)
	t.Logf("isolated the keys %s under the test temp root", strings.Join(ggHomeKeys, ", "))
	return root, env
}

func ggBy(i int) string          { return fmt.Sprintf("goroutine-%02d", i) }
func ggText(i int) string        { return fmt.Sprintf("evidence text of goroutine %02d", i) }
func ggProcBy(w int) string      { return fmt.Sprintf("child-%d", w) }
func ggProcText(w, i int) string { return fmt.Sprintf("evidence text of child %d, record %d", w, i) }

// ggRecords reads proof/<item>/proof.json with the standard library only, so the
// test does not lean on the production reader.
func ggRecords(t *testing.T, task, item string) []Record {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(task, "proof", item, "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recs []Record
	if err := json.Unmarshal(b, &recs); err != nil {
		t.Fatalf("proof.json does not parse: %v", err)
	}
	return recs
}

// ggCheckItem requires want records with distinct File values, each file present
// with the bytes the record's SHA256 names, and no other file in the folder.
// It returns the text of each record, keyed by File.
func ggCheckItem(t *testing.T, task string, recs []Record, want int) map[string]string {
	t.Helper()
	if len(recs) != want {
		t.Fatalf("proof.json holds %d records, want %d", len(recs), want)
	}
	texts := map[string]string{}
	for _, r := range recs {
		if _, dup := texts[r.File]; dup {
			t.Fatalf("File %q appears in two records", r.File)
		}
		if path.Dir(r.File) != "proof/"+ggItemDir || path.Ext(r.File) != ".txt" {
			t.Fatalf("File %q is not a .txt in proof/%s", r.File, ggItemDir)
		}
		b, err := os.ReadFile(filepath.Join(task, filepath.FromSlash(r.File)))
		if err != nil {
			t.Fatalf("record %q lost its evidence file: %v", r.By, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != r.SHA256 {
			t.Fatalf("record %q: file hashes to %s, record carries %s", r.By, got, r.SHA256)
		}
		texts[r.File] = string(b)
	}
	entries, err := os.ReadDir(filepath.Join(task, "proof", ggItemDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	wantNames := []string{"proof.json"}
	for f := range texts {
		wantNames = append(wantNames, path.Base(f))
	}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("item folder holds %d entries, want proof.json and %d record files:\n got %v\nwant %v", len(names), want, names, wantNames)
	}
	return texts
}

// ggNoLeftovers walks the task folder: no temp file of the atomic writer, no
// *.tmp, and no evidence file that proof.json does not name.
func ggNoLeftovers(t *testing.T, task string) {
	t.Helper()
	referenced := map[string]bool{}
	var files []string
	err := filepath.WalkDir(task, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(task, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		if d.Name() == "proof.json" {
			var recs []Record
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &recs); err != nil {
				return fmt.Errorf("%s does not parse: %w", rel, err)
			}
			for _, r := range recs {
				referenced[r.File] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range files {
		name := path.Base(rel)
		switch {
		case strings.Contains(name, ".tmp"):
			t.Errorf("leftover temp file %s", rel)
		case strings.HasPrefix(name, ".") && name != ".proof.lock":
			t.Errorf("leftover atomic-write temp file %s", rel)
		case strings.HasPrefix(rel, "proof/") && name != "proof.json" && name != "index.json" && !referenced[rel]:
			t.Errorf("evidence file %s is in no record", rel)
		}
	}
}

func ggCount(idx Index) int {
	n := 0
	for _, it := range idx.Items {
		n += len(it.Evidence)
	}
	return n
}

// ggFinish is the row 3 check: Build then Check pass, and Rebuild gives the same
// index as Build, as TestRebuildAfterDeletingIndexMatchesBuild requires. Item 2
// of the fixture needs a screenshot, so one is added first.
func ggFinish(t *testing.T, s Store, want int) {
	t.Helper()
	shot := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(shot, []byte("image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Input{Item: "2", File: shot, Kind: "screenshot", By: "gg-shot"}); err != nil {
		t.Fatal(err)
	}
	built, err := s.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := built.Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got := ggCount(built); got != want+1 {
		t.Fatalf("Build counts %d records, want %d", got, want+1)
	}
	first, err := s.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	indexPath := filepath.Join(s.TaskDir, "proof", "index.json")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	again, err := s.Build()
	if err != nil {
		t.Fatalf("Build after deleting the index: %v", err)
	}
	rebuilt, err := s.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild after deleting the index: %v", err)
	}
	if !reflect.DeepEqual(built, first) || !reflect.DeepEqual(first, rebuilt) || !reflect.DeepEqual(again, rebuilt) {
		t.Fatalf("index differs:\nbuild   %+v\nrebuild %+v\nagain   %+v", built, first, rebuilt)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rebuilt index file differs: %v", err)
	}
}

// Row 1: 16 goroutines call Add on one item.
func TestGgProofConcurrentGoroutines(t *testing.T) {
	ggIsolate(t)
	s := fixture(t)
	const n = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	got := make([]Record, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // all goroutines are released together
			got[i], errs[i] = s.Add(Input{Item: ggItem, Kind: "log", By: ggBy(i), Text: ggText(i), Note: fmt.Sprintf("note %02d", i)})
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("Add %d: %v", i, err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	recs := ggRecords(t, s.TaskDir, ggItemDir)
	texts := ggCheckItem(t, s.TaskDir, recs, n)
	byFile := map[string]Record{}
	for _, r := range recs {
		byFile[r.File] = r
	}
	for i := 0; i < n; i++ {
		r, ok := byFile[got[i].File]
		if !ok {
			t.Fatalf("Add %d returned %q, but proof.json has no record with that File", i, got[i].File)
		}
		if !reflect.DeepEqual(r, got[i]) {
			t.Fatalf("Add %d: stored record differs from the returned one:\n stored %+v\n got    %+v", i, r, got[i])
		}
		if r.By != ggBy(i) || texts[r.File] != ggText(i) {
			t.Fatalf("Add %d: record by %q holds %q", i, r.By, texts[r.File])
		}
	}
	ggFinish(t, s, n)
	ggNoLeftovers(t, s.TaskDir)
}

// ggKid is one child process: the test binary re-run on the helper test.
type ggKid struct {
	worker int
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    bytes.Buffer
	errb   bytes.Buffer
	exit   int
}

type ggGroup struct {
	kids    []*ggKid
	release sync.Once
	wait    sync.Once
}

// ggStart starts every child and releases none: each blocks reading its stdin.
func ggStart(t *testing.T, root string, env []string, task string, workers, per int) *ggGroup {
	t.Helper()
	g := &ggGroup{}
	t.Cleanup(func() { g.Release(); g.Wait() }) // never leave a child behind a failed test
	for w := 0; w < workers; w++ {
		k := &ggKid{worker: w}
		k.cmd = exec.Command(os.Args[0], "-test.run="+ggChild)
		k.cmd.Env = append(append([]string{}, os.Environ()...), env...)
		k.cmd.Env = append(k.cmd.Env, ggMarker+"=1", "GG_PROOF_ROOT="+root, "GG_PROOF_TASK="+task,
			"GG_PROOF_WORKER="+strconv.Itoa(w), "GG_PROOF_COUNT="+strconv.Itoa(per))
		k.cmd.Stdout = &k.out
		k.cmd.Stderr = &k.errb
		in, err := k.cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		k.stdin = in
		if err := k.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		g.kids = append(g.kids, k)
	}
	return g
}

// Release closes every stdin pipe, which lets the children begin. Call it only
// after ggStart has returned, so all children have started.
func (g *ggGroup) Release() {
	g.release.Do(func() {
		for _, k := range g.kids {
			_ = k.stdin.Close()
		}
	})
}

// Wait collects every child's exit code. It never calls the testing package.
func (g *ggGroup) Wait() {
	g.wait.Do(func() {
		for _, k := range g.kids {
			err := k.cmd.Wait()
			var ee *exec.ExitError
			switch {
			case err == nil:
				k.exit = 0
			case errors.As(err, &ee):
				k.exit = ee.ExitCode()
			default:
				k.exit = -1
			}
		}
	})
}

// Verify requires exit 0 and exactly one result line, the child's own.
func (g *ggGroup) Verify(t *testing.T, per int) {
	t.Helper()
	for _, k := range g.kids {
		var lines []string
		for _, l := range strings.Split(k.out.String(), "\n") {
			if strings.HasPrefix(l, "GGRESULT ") {
				lines = append(lines, l)
			}
		}
		want := fmt.Sprintf("GGRESULT worker=%d added=%d", k.worker, per)
		if k.exit != 0 || len(lines) != 1 || lines[0] != want {
			t.Errorf("child %d: exit %d, result lines %q, want exit 0 and %q\nstdout: %s\nstderr: %s", k.worker, k.exit, lines, want, k.out.String(), k.errb.String())
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

// TestGgProofConcurrentChildHelper is the child of the process tests. It does
// nothing unless the parent set the marker.
func TestGgProofConcurrentChildHelper(t *testing.T) {
	if os.Getenv(ggMarker) != "1" {
		t.Skip("helper of the process tests; runs only as their child")
	}
	ggRequireHomes(t, os.Getenv("GG_PROOF_ROOT"))
	worker, err := strconv.Atoi(os.Getenv("GG_PROOF_WORKER"))
	if err != nil {
		t.Fatal(err)
	}
	per, err := strconv.Atoi(os.Getenv("GG_PROOF_COUNT"))
	if err != nil {
		t.Fatal(err)
	}
	// Block until the parent closes the pipe, after every child has started.
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
	s := Store{TaskDir: os.Getenv("GG_PROOF_TASK")}
	for i := 0; i < per; i++ {
		if _, err := s.Add(Input{Item: ggItem, Kind: "log", By: ggProcBy(worker), Text: ggProcText(worker, i)}); err != nil {
			t.Fatalf("worker %d, add %d: %v", worker, i, err)
		}
	}
	fmt.Printf("GGRESULT worker=%d added=%d\n", worker, per)
}

// ggCheckChildRecords requires every child's records, in the order that child
// added them. The records of different children interleave in any order.
func ggCheckChildRecords(t *testing.T, recs []Record, texts map[string]string, workers, per int) {
	t.Helper()
	next := make([]int, workers)
	for _, r := range recs {
		var w int
		if _, err := fmt.Sscanf(r.By, "child-%d", &w); err != nil || w < 0 || w >= workers {
			t.Fatalf("record by %q is from no child", r.By)
		}
		if next[w] >= per || texts[r.File] != ggProcText(w, next[w]) {
			t.Fatalf("child %d: record %d holds %q, want %q", w, next[w], texts[r.File], ggProcText(w, next[w]))
		}
		next[w]++
	}
	for w, n := range next {
		if n != per {
			t.Fatalf("child %d has %d records, want %d", w, n, per)
		}
	}
}

// Row 2: 4 child processes each add 5 records to one item.
func TestGgProofConcurrentProcesses(t *testing.T) {
	root, env := ggIsolate(t)
	s := fixture(t)
	const workers, per = 4, 5
	g := ggStart(t, root, env, s.TaskDir, workers, per)
	g.Release()
	g.Wait()
	g.Verify(t, per)
	recs := ggRecords(t, s.TaskDir, ggItemDir)
	texts := ggCheckItem(t, s.TaskDir, recs, workers*per)
	ggCheckChildRecords(t, recs, texts, workers, per)
	ggFinish(t, s, workers*per)
	ggNoLeftovers(t, s.TaskDir)
}

// Row 4: Build, without the lock, runs in a loop while 3 children add. Every
// Build succeeds and the record count never falls.
func TestGgProofConcurrentBuildWhileAdding(t *testing.T) {
	root, env := ggIsolate(t)
	s := fixture(t)
	const workers, per = 3, 4
	g := ggStart(t, root, env, s.TaskDir, workers, per)
	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()
	g.Release()
	last, builds := 0, 0
loop:
	for {
		idx, err := s.Build()
		if err != nil {
			t.Fatalf("Build %d while the children add: %v", builds, err)
		}
		if n := ggCount(idx); n < last {
			t.Fatalf("Build %d counts %d records after a Build that counted %d", builds, n, last)
		} else {
			last = n
		}
		builds++
		select {
		case <-done:
			break loop
		default:
		}
	}
	t.Logf("%d Build calls ran while the children added", builds)
	g.Verify(t, per)
	idx, err := s.Build()
	if err != nil {
		t.Fatalf("Build after the children exited: %v", err)
	}
	if n := ggCount(idx); n < last || n != workers*per {
		t.Fatalf("final Build counts %d records, want %d (last in the loop: %d)", n, workers*per, last)
	}
	recs := ggRecords(t, s.TaskDir, ggItemDir)
	texts := ggCheckItem(t, s.TaskDir, recs, workers*per)
	ggCheckChildRecords(t, recs, texts, workers, per)
	ggFinish(t, s, workers*per)
	ggNoLeftovers(t, s.TaskDir)
}
