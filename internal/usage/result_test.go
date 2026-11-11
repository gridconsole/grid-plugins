package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openResultFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestUsageParseResultFixture(t *testing.T) {
	in, key, err := ParseResult(openResultFixture(t, "attempt-1.log"))
	if err != nil {
		t.Fatal(err)
	}
	if key != "result:5f1c2a9e-7b3d-4c1e-9a52-3d0e8f6b7a11" || in.Key != key {
		t.Fatalf("key %q / %q", key, in.Key)
	}
	if in.Session != "5f1c2a9e-7b3d-4c1e-9a52-3d0e8f6b7a11" || in.Model != "claude-opus-4-1" {
		t.Fatalf("session/model: %+v", in)
	}
	if in.Dollars == nil || *in.Dollars != 0.476067 {
		t.Fatalf("dollars: %v", in.Dollars)
	}
	if in.Minutes == nil || *in.Minutes != 125400.0/60000 {
		t.Fatalf("minutes: %v", in.Minutes)
	}
	if in.Tokens == nil || *in.Tokens != 12+3400+150000+21000 {
		t.Fatalf("tokens: %v", in.Tokens)
	}
}

func TestUsageParseResultLastLineWins(t *testing.T) {
	log := `{"type":"result","session_id":"s-1","total_cost_usd":1}` + "\n" +
		`{"type":"result","session_id":"s-1","total_cost_usd":2,"modelUsage":{"b-model":{"costUSD":1},"a-model":{"costUSD":1},"c-model[1m]":{"costUSD":0.5}}}` + "\n" +
		"not json at all\n" + `{"type":"assistant"}` + "\n" + `{broken "result"` + "\n"
	in, _, err := ParseResult(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if *in.Dollars != 2 || in.Model != "a-model" || in.Minutes != nil || in.Tokens != nil {
		t.Fatalf("%+v", in)
	}
	// a result line cut off without a final newline still counts
	in, _, err = ParseResult(strings.NewReader(`{"type":"result","session_id":"s-2","total_cost_usd":0}`))
	if err != nil || *in.Dollars != 0 {
		t.Fatalf("%+v %v", in, err)
	}
	// a model name with a context tag loses the tag
	in, _, err = ParseResult(strings.NewReader(`{"type":"result","session_id":"s-3","total_cost_usd":1,"modelUsage":{"claude-opus-4-5[1m]":{"costUSD":1}}}`))
	if err != nil || in.Model != "claude-opus-4-5" {
		t.Fatalf("%+v %v", in, err)
	}
}

func TestUsageParseResultRefusals(t *testing.T) {
	for name, log := range map[string]string{
		"empty":         "",
		"no-result":     `{"type":"system","session_id":"s-1"}` + "\n",
		"no-cost":       `{"type":"result","session_id":"s-1"}` + "\n",
		"null-cost":     `{"type":"result","session_id":"s-1","total_cost_usd":null}` + "\n",
		"negative-cost": `{"type":"result","session_id":"s-1","total_cost_usd":-1}` + "\n",
		"no-session":    `{"type":"result","total_cost_usd":1}` + "\n",
		"bad-session":   `{"type":"result","session_id":"../x","total_cost_usd":1}` + "\n",
		"bad-duration":  `{"type":"result","session_id":"s-1","total_cost_usd":1,"duration_ms":-5}` + "\n",
		"bad-tokens":    `{"type":"result","session_id":"s-1","total_cost_usd":1,"usage":{"input_tokens":-1}}` + "\n",
	} {
		if _, _, err := ParseResult(strings.NewReader(log)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func resultStore(t *testing.T) Store {
	t.Helper()
	task := t.TempDir()
	return Store{TaskPath: task, Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}
}

func TestUsagePostKeyBooksOnce(t *testing.T) {
	s := resultStore(t)
	in, _, err := ParseResult(openResultFixture(t, "attempt-1.log"))
	if err != nil {
		t.Fatal(err)
	}
	in.Job = "J-2"
	first, err := s.Post(in)
	if err != nil || first.AlreadyBooked || first.Totals.Posts != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	path := filepath.Join(s.TaskPath, "usage", "usage.jsonl")
	before, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(before), `"key":"result:5f1c2a9e-7b3d-4c1e-9a52-3d0e8f6b7a11"`) {
		t.Fatalf("%s %v", before, err)
	}
	second, err := s.Post(in)
	if err != nil || !second.AlreadyBooked || second.Totals.Posts != 1 || second.Totals.Dollars != 0.476067 {
		t.Fatalf("%+v %v", second, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("a repeat appended: %s", after)
	}
	// the key is global to the task's log, not to the Job: the same session under another Job is the same run
	in.Job = "J-3"
	if again, err := s.Post(in); err != nil || !again.AlreadyBooked {
		t.Fatalf("%+v %v", again, err)
	}
	// typed posts carry no key and are never skipped
	d := 1.0
	for i := 0; i < 2; i++ {
		if _, err := s.Post(Input{Job: "J-2", Dollars: &d}); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := s.Show()
	if err != nil || sum.Totals.Posts != 3 {
		t.Fatalf("%+v %v", sum, err)
	}
}

func TestUsageLogWithoutKeyFolds(t *testing.T) {
	s := resultStore(t)
	dir := filepath.Join(s.TaskPath, "usage")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"v":1,"at":"2026-01-02T03:04:05.000Z","job":"J-1","session":"","model":"","dollars":1.5,"minutes":2,"tokens":7}` + "\n" +
		`{"v":1,"at":"2026-01-02T03:04:06.000Z","job":"lead","session":"s","model":"m","dollars":0.25,"minutes":0}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "usage.jsonl"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := s.Show()
	if err != nil {
		t.Fatal(err)
	}
	if sum.Totals.Dollars != 1.75 || sum.Totals.Minutes != 2 || sum.Totals.Tokens != 7 || sum.Totals.Posts != 2 || len(sum.Jobs) != 2 {
		t.Fatalf("%+v", sum)
	}
	// a keyed post beside rows without keys is booked, then skipped
	in, _, err := ParseResult(strings.NewReader(`{"type":"result","session_id":"s-9","total_cost_usd":0.25}`))
	if err != nil {
		t.Fatal(err)
	}
	in.Job = "J-1"
	if out, err := s.Post(in); err != nil || out.AlreadyBooked || out.Totals.Posts != 3 {
		t.Fatalf("%+v %v", out, err)
	}
}
