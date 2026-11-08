package feed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPostTimings measures what feed-contract.md section 11 asks for on the
// real pattern: state records (synced before the ack) from 16 concurrent
// writer processes. GRID_FEED_TIMING_EACH sets the posts per writer; without it
// the test is skipped. The p99 bar is 50 ms; the figures are recorded in docs/lanes/L5.md.
func TestPostTimings(t *testing.T) {
	each, _ := strconv.Atoi(os.Getenv("GRID_FEED_TIMING_EACH"))
	if dir := os.Getenv("GRID_FEED_TIMING_CHILD"); dir != "" {
		i, _ := strconv.Atoi(os.Getenv("GRID_FEED_TIMING_WRITER"))
		s := Store{TaskDir: dir, TaskID: testTask}
		var out strings.Builder
		for j := 0; j < each; j++ {
			in := Input{Type: "question", Job: "J-1", Body: contractBody(t, "question", map[string]any{"question": fmt.Sprintf("%d/%d", i, j)})}
			start := time.Now()
			if _, err := s.Post(in, stressWriter(i)); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintln(&out, time.Since(start).Nanoseconds())
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("GRID_FEED_TIMING_OUT"), strconv.Itoa(i)), []byte(out.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if each <= 0 {
		t.Skip("set GRID_FEED_TIMING_EACH to run the 16-writer state timing")
	}
	const writers = 16
	s := testStore(t)
	outDir := t.TempDir()
	cmds := make([]*exec.Cmd, writers)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestPostTimings$")
		cmds[i].Env = append(os.Environ(), "GRID_FEED_TIMING_CHILD="+s.TaskDir, "GRID_FEED_TIMING_OUT="+outDir, "GRID_FEED_TIMING_EACH="+strconv.Itoa(each), fmt.Sprintf("GRID_FEED_TIMING_WRITER=%d", i))
		cmds[i].Stderr = os.Stderr
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}
	var all []int64
	for i := 0; i < writers; i++ {
		raw, err := os.ReadFile(filepath.Join(outDir, strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range strings.Fields(string(raw)) {
			n, _ := strconv.ParseInt(f, 10, 64)
			all = append(all, n)
		}
	}
	sort.Slice(all, func(a, b int) bool { return all[a] < all[b] })
	if len(all) != writers*each {
		t.Fatalf("posts=%d want=%d", len(all), writers*each)
	}
	at := func(q float64) time.Duration { return time.Duration(all[int(q*float64(len(all)-1))]) }
	t.Logf("%d writers x %d state posts: p50 %s p99 %s max %s", writers, each, at(0.5).Round(time.Microsecond), at(0.99).Round(time.Microsecond), at(1).Round(time.Microsecond))
}
