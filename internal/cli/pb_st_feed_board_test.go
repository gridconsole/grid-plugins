package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// pbStCliUpdate rewrites the golden: go test ./internal/cli -run PbStFeedBoard -pbst.cli.update
var pbStCliUpdate = flag.Bool("pbst.cli.update", false, "rewrite the board read golden")

// pbStBoardHarness is the Feed harness with a board/ folder in the task and a machine
// folder of its own: the task is on a per-post board.
func pbStBoardHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h, task := feedHarness(t)
	h.env.MachineFolder = machinefolder.Folder{Path: t.TempDir(), Source: machinefolder.SourceEnv}
	h.env.Login = "tester"
	if err := os.MkdirAll(filepath.Join(task, "board"), 0o700); err != nil {
		t.Fatal(err)
	}
	return h, task
}

// pbStScrub replaces the ids and times of a read, which differ on every run, with the
// order they come in, so the rest of the shape can be held by a golden.
func pbStScrub(t *testing.T, raw []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("read output: %v\n%s", err, raw)
	}
	recs, _ := doc["records"].([]any)
	names := map[string]string{}
	for i, r := range recs {
		m := r.(map[string]any)
		id, _ := m["id"].(string)
		names[id] = "ID-" + string(rune('A'+i))
	}
	for _, r := range recs {
		m := r.(map[string]any)
		m["id"] = names[m["id"].(string)]
		if _, ok := m["at"]; ok {
			m["at"] = "AT"
		}
		for _, key := range []string{"re", "after"} {
			switch v := m[key].(type) {
			case string:
				m[key] = names[v]
			case []any:
				for j := range v {
					s, _ := v[j].(string)
					v[j] = names[s]
				}
			}
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
}

// Row 14 (brain-board.md Q3): `grid feed read --json` of a board task. A record keeps
// id, type, seq, author and body; it gains by and file; it has no lease, fence, end or
// gen; worker stays as an alias of by. The golden holds the shape.
func TestPbStFeedBoardReadGolden(t *testing.T) {
	h, task := pbStBoardHarness(t)
	t.Setenv("GRID_WORKER", "member@"+feedMachine)
	for _, args := range [][]string{
		{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"first"}`, "--json"},
		{"feed", "post", "--type", "question", "--body", `{"question":"Continue?","who":"owner"}`, "--json"},
		{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"third"}`, "--json"},
	} {
		if code := h.run(task, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	if code := h.run(task, "feed", "read", "--json"); code != 0 {
		t.Fatalf("read: %d %s", code, h.errb)
	}
	got := pbStScrub(t, h.out.Bytes())
	path := filepath.Join("testdata", "pb_st_feed_board_read.golden.json")
	if *pbStCliUpdate {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("board read differs from %s:\n got %s\nwant %s", path, got, want)
	}
	// The old-shape fields are gone, not empty.
	var doc struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, r := range doc.Records {
		for _, gone := range []string{"lease", "fence", "end", "gen"} {
			if _, ok := r[gone]; ok {
				t.Fatalf("record has %q: %v", gone, r)
			}
		}
		if r["by"] == "" || r["by"] != r["worker"] {
			t.Fatalf("by %v, worker %v", r["by"], r["worker"])
		}
	}
}

// `grid feed post` on a board writes board/<seq>-<job|task>-<type>.md and answers with
// by, the file and no old-shape fields; the task has no feed/ folder.
func TestPbStFeedBoardPost(t *testing.T) {
	h, task := pbStBoardHarness(t)
	t.Setenv("GRID_WORKER", "member@"+feedMachine)
	if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"example"}`, "--json"); code != 0 {
		t.Fatalf("post: %d %s", code, h.errb)
	}
	var out struct {
		Post map[string]any `json:"post"`
		File string         `json:"file"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.File != "000001-task-post.md" || out.Post["file"] != nil && out.Post["file"] != out.File {
		t.Fatalf("file %q in %s", out.File, h.out)
	}
	if want := feedMachine + ":tester"; out.Post["by"] != want || out.Post["worker"] != want {
		t.Fatalf("by %v worker %v, want %s", out.Post["by"], out.Post["worker"], want)
	}
	for _, gone := range []string{"lease", "fence", "end", "gen"} {
		if _, ok := out.Post[gone]; ok {
			t.Fatalf("post has %q: %v", gone, out.Post)
		}
	}
	if _, err := os.Stat(filepath.Join(task, "board", out.File)); err != nil {
		t.Fatalf("post file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(task, "feed")); !os.IsNotExist(err) {
		t.Fatalf("feed/ in a board task: %v", err)
	}
}

// A handover is a Feed lease record; a board has no lease, so the CLI refuses it
// rather than writing a JSONL Feed beside the board.
func TestPbStFeedBoardHandoverRefused(t *testing.T) {
	h, task := pbStBoardHarness(t)
	t.Setenv("GRID_WORKER", "member@"+feedMachine)
	code := h.run(task, "feed", "handover", "--json")
	if code == 0 {
		t.Fatalf("handover on a board: %s", h.out)
	}
	if _, err := os.Stat(filepath.Join(task, "feed")); !os.IsNotExist(err) {
		t.Fatalf("feed/ in a board task: %v", err)
	}
}
