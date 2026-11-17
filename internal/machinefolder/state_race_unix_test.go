//go:build unix

package machinefolder_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type statePost struct {
	Writer int    `json:"writer"`
	Seq    int    `json:"seq"`
	Pad    string `json:"pad"`
	End    string `json:"end"`
}

// Row 4: concurrent state.json writers go by atomic rename of a temp file in the
// same folder, so a reader never parses a partial file. The documents are large
// enough that a write in place would be seen half done.
func TestStateJSONConcurrentWritersReaderNeverSeesPartial(t *testing.T) {
	f := sharedFolder(t)
	const (
		writers = 4
		rounds  = 25
		readers = 4
	)
	make1 := func(w, seq int) []byte {
		pad := strings.Repeat(fmt.Sprintf("w%d-s%d;", w, seq), 6000)
		b, _ := json.Marshal(statePost{Writer: w, Seq: seq, Pad: pad, End: "end"})
		return b
	}
	if err := f.WriteState("T-01", make1(0, 0)); err != nil {
		t.Fatal(err)
	}

	var done atomic.Bool
	var wg, rg sync.WaitGroup
	var reads, bad atomic.Int64
	errs := make(chan string, readers+writers)

	for r := 0; r < readers; r++ {
		rg.Add(1)
		go func() {
			defer rg.Done()
			for !done.Load() {
				b, err := f.ReadState("T-01")
				if err != nil {
					errs <- "read: " + err.Error()
					return
				}
				var p statePost
				if err := json.Unmarshal(b, &p); err != nil || p.End != "end" {
					bad.Add(1)
					errs <- fmt.Sprintf("a reader parsed a partial file (%d bytes): %v", len(b), err)
					return
				}
				// The pad is one writer's: a file mixing two writes is a torn file.
				if want := fmt.Sprintf("w%d-s%d;", p.Writer, p.Seq); !strings.HasPrefix(p.Pad, want) || !strings.HasSuffix(p.Pad, want) {
					bad.Add(1)
					errs <- "a reader saw two writes mixed in one file"
					return
				}
				reads.Add(1)
			}
		}()
	}
	for w := 1; w <= writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for seq := 1; seq <= rounds; seq++ {
				if err := f.WriteState("T-01", make1(w, seq)); err != nil {
					errs <- "write: " + err.Error()
					return
				}
			}
		}(w)
	}
	wg.Wait()
	done.Store(true)
	rg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if reads.Load() == 0 {
		t.Error("no reader read anything")
	}

	// No temp file is left in the task's folder, and the file is whole and 0660.
	sp, _ := f.StatePath("T-01")
	entries, err := os.ReadDir(sp[:strings.LastIndex(sp, "/")])
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("task folder holds %v, want only state.json", names)
	}
	if got := modeOf(t, sp); got != 0o660 {
		t.Errorf("state.json mode %v, want 0660", got)
	}
}
