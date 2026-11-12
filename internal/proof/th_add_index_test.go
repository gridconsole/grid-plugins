package proof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// These tests hold the claim in Store.Add: when it returns, proof/index.json
// already lists what it stored, written under the lock Add holds. They wait on
// a WaitGroup, never on a sleep.

func thIdxPath(s Store) string { return filepath.Join(s.TaskDir, "proof", "index.json") }

// thIdxRead returns the bytes of proof/index.json and the Index they decode to.
func thIdxRead(t *testing.T, s Store) ([]byte, Index) {
	t.Helper()
	raw, err := os.ReadFile(thIdxPath(s))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatalf("index.json does not parse: %v", err)
	}
	return raw, idx
}

// thIdxWant is what Rebuild writes: Build, indented, plus a newline.
func thIdxWant(t *testing.T, s Store) (Index, []byte) {
	t.Helper()
	built, err := s.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return built, append(b, '\n')
}

func thIdxItem(idx Index, id string) (Item, bool) {
	for _, it := range idx.Items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// Row 1: one Add leaves an index that lists the item and its one record.
func TestThIdxAddWritesIndex(t *testing.T) {
	s := fixture(t)
	r, err := s.Add(Input{Item: "1", Text: "checks pass", Kind: "log", By: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	raw, got := thIdxRead(t, s)
	built, want := thIdxWant(t, s)
	if !reflect.DeepEqual(got, built) {
		t.Fatalf("index differs from Build:\n index %+v\n build %+v", got, built)
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("index.json differs from Build, indented, plus a newline:\n got  %q\n want %q", raw, want)
	}
	item, ok := thIdxItem(got, "i-1")
	if !ok {
		t.Fatalf("index lists no item i-1: %+v", got.Items)
	}
	if !reflect.DeepEqual(item.Evidence, []Record{r}) {
		t.Fatalf("i-1 evidence = %+v, want the one record Add returned %+v", item.Evidence, r)
	}
	if other, ok := thIdxItem(got, "i-2"); !ok || len(other.Evidence) != 0 {
		t.Fatalf("i-2 = %+v, want listed with no evidence", other)
	}

	// A second Add refreshes the same file.
	r2, err := s.Add(Input{Item: "2", Text: "second", Kind: "log", By: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	_, got = thIdxRead(t, s)
	if item, _ := thIdxItem(got, "i-2"); !reflect.DeepEqual(item.Evidence, []Record{r2}) {
		t.Fatalf("i-2 evidence after the second Add = %+v, want %+v", item.Evidence, r2)
	}
	if item, _ := thIdxItem(got, "i-1"); !reflect.DeepEqual(item.Evidence, []Record{r}) {
		t.Fatalf("i-1 evidence after the second Add = %+v, want %+v", item.Evidence, r)
	}
}

// Row 2: 16 goroutines add to two items; the index file is exactly what Build
// gives once they have all returned, and lists all 16 records.
func TestThIdxConcurrentAddsListAll(t *testing.T) {
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
			got[i], errs[i] = s.Add(Input{Item: fmt.Sprint(i%2 + 1), Kind: "log", By: fmt.Sprintf("th-%02d", i), Text: fmt.Sprintf("th evidence %02d", i)})
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	raw, idx := thIdxRead(t, s)
	_, want := thIdxWant(t, s)
	if !bytes.Equal(raw, want) {
		t.Fatalf("index.json differs from Build, indented, plus a newline:\n got  %q\n want %q", raw, want)
	}
	listed := map[string]Record{}
	for _, it := range idx.Items {
		for _, r := range it.Evidence {
			listed[r.File] = r
		}
	}
	if len(listed) != n {
		t.Fatalf("index lists %d records, want %d", len(listed), n)
	}
	for i, r := range got {
		if !reflect.DeepEqual(listed[r.File], r) {
			t.Fatalf("Add %d returned %+v, index lists %+v", i, r, listed[r.File])
		}
	}
}

// Row 3: a gif is proof for the screenshot and video kinds and is kept as .gif;
// a refused add leaves no index behind.
func TestThIdxGif(t *testing.T) {
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	for _, kind := range []string{"screenshot", "video"} {
		t.Run(kind, func(t *testing.T) {
			s := fixture(t)
			r, err := s.Add(Input{Item: "2", File: proofAddBuildSource(t, "rec.GIF", gif), Kind: kind, By: "grid"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(r.File, ".gif") {
				t.Fatalf("File = %q, want a .gif name", r.File)
			}
			stored, err := os.ReadFile(filepath.Join(s.TaskDir, filepath.FromSlash(r.File)))
			if err != nil || !bytes.Equal(stored, gif) {
				t.Fatalf("stored %q, %v; want the gif bytes", stored, err)
			}
			_, idx := thIdxRead(t, s)
			item, _ := thIdxItem(idx, "i-2")
			if len(item.Evidence) != 1 || !strings.HasSuffix(item.Evidence[0].File, ".gif") || item.Evidence[0].Kind != kind {
				t.Fatalf("i-2 evidence = %+v, want one %s record whose file ends in .gif", item.Evidence, kind)
			}
			// i-2 asks for a screenshot, so only that kind proves it.
			if item.Proven != (kind == "screenshot") {
				t.Fatalf("i-2 Proven = %v after a %s gif", item.Proven, kind)
			}
		})
	}
	t.Run("refusals name gif", func(t *testing.T) {
		for kind, file := range map[string]string{"screenshot": "shot.txt", "video": "clip.png"} {
			s := fixture(t)
			_, err := s.Add(Input{Item: "1", File: proofAddBuildSource(t, file, []byte("bytes")), Kind: kind, By: "grid"})
			if err == nil || !strings.Contains(err.Error(), "gif") {
				t.Fatalf("%s with %s: got %v, want a message that names gif", kind, file, err)
			}
		}
	})
	t.Run("refused add leaves no index", func(t *testing.T) {
		s := fixture(t)
		_, err := s.Add(Input{Item: "1", File: proofAddBuildSource(t, "x.log", nil), Kind: "log", By: "grid"})
		if err == nil || err.Error() != "proof must contain 1 byte to 20 MiB" {
			t.Fatalf("got %v", err)
		}
		if _, err := os.Stat(thIdxPath(s)); !os.IsNotExist(err) {
			t.Fatalf("proof/index.json after a refused add: %v", err)
		}
	})
}

// A broken record on another item makes Build refuse. The evidence Add stored
// is already on disk, so Add still returns it with no error; reporting a
// failure would make the caller add it again.
func TestThIdxBrokenSiblingKeepsAdd(t *testing.T) {
	s := fixture(t)
	proofAddBuildMetadata(t, s, "i-2", []byte("{"))
	r, err := s.Add(Input{Item: "1", Text: "checks pass", Kind: "log", By: "grid"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, filepath.FromSlash(r.File))); err != nil {
		t.Fatalf("evidence file: %v", err)
	}
	if recs := ggRecords(t, s.TaskDir, "i-1"); len(recs) != 1 || recs[0] != r {
		t.Fatalf("proof.json = %+v, want the returned record", recs)
	}
	if _, err := os.Stat(thIdxPath(s)); !os.IsNotExist(err) {
		t.Fatalf("proof/index.json written from a Build that refused: %v", err)
	}
}
