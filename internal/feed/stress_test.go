package feed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testenv"
)

const stressWriters = 125

func stressWriter(i int) Writer {
	member := fmt.Sprintf("w%03d", i)
	return Writer{Author: Author{ID: member, Role: "member"}, Worker: member + "@" + testMachine}
}

// TestStressWriters mirrors exp1-appenders at a chosen size: 125 processes with
// distinct workers append 1 KB chatter. GRID_FEED_STRESS_EACH sets the records
// per writer; without it the test is skipped, so the fast tier stays fast.
func TestStressWriters(t *testing.T) {
	each, _ := strconv.Atoi(os.Getenv("GRID_FEED_STRESS_EACH"))
	if dir := os.Getenv("GRID_FEED_STRESS_CHILD"); dir != "" {
		i, _ := strconv.Atoi(os.Getenv("GRID_FEED_STRESS_WRITER"))
		s := Store{TaskDir: dir, TaskID: testTask}
		pad := strings.Repeat("x", 1000)
		for j := 0; j < each; j++ {
			if _, err := s.Post(message(fmt.Sprintf("%03d/%04d %s", i, j, pad)), stressWriter(i)); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	if each <= 0 {
		t.Skip("set GRID_FEED_STRESS_EACH to run the 125-writer stress test")
	}
	s := testStore(t)
	if err := os.MkdirAll(s.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	cmds := make([]*exec.Cmd, stressWriters)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestStressWriters$")
		cmds[i].Env = append(testenv.Temp(t, os.Environ()), "GRID_FEED_STRESS_CHILD="+s.TaskDir, fmt.Sprintf("GRID_FEED_STRESS_WRITER=%d", i))
		cmds[i].Stderr = os.Stderr
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		cmd := cmds[i]
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(start)
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	want := stressWriters * each
	if len(read.Records) != want || len(read.Bad) != 0 {
		t.Fatalf("records=%d bad=%v want=%d", len(read.Records), read.Bad, want)
	}
	ids := map[string]bool{}
	seqs := map[string]int64{}
	for _, e := range read.Records {
		if ids[e.ID] {
			t.Fatalf("duplicate record %s", e.ID)
		}
		ids[e.ID] = true
		if e.Seq != seqs[e.File]+1 {
			t.Fatalf("%s: seq %d after %d", e.File, e.Seq, seqs[e.File])
		}
		seqs[e.File] = e.Seq
	}
	files, _ := filepath.Glob(filepath.Join(s.Dir(), "*.jsonl"))
	if len(files) != stressWriters {
		t.Fatalf("tenure files=%d", len(files))
	}
	t.Logf("%d writers x %d = %d records in %s (%.1f ms per post)", stressWriters, each, want, elapsed.Round(time.Millisecond), float64(elapsed.Milliseconds())/float64(want))
}
