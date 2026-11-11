package usage

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func number(n float64) *float64 { return &n }

func testStore(t *testing.T) Store {
	t.Helper()
	return Store{TaskPath: t.TempDir(), Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 678000000, time.UTC) }}
}

func TestPostValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    Input
		valid bool
	}{
		{"missing", Input{Job: "J-1"}, false},
		{"negative", Input{Job: "J-1", Dollars: number(-1)}, false},
		{"nan", Input{Job: "J-1", Dollars: number(math.NaN())}, false},
		{"inf", Input{Job: "J-1", Minutes: number(math.Inf(1))}, false},
		{"job", Input{Job: "J-0", Dollars: number(1)}, false},
		{"zero-component", Input{Job: "J-1.0", Dollars: number(1)}, false},
		{"leading-zero", Input{Job: "J-01", Dollars: number(1)}, false},
		{"lead", Input{Job: "lead", Dollars: number(0)}, true},
		{"nested", Input{Job: "J-1.2.3", Minutes: number(2)}, true},
		{"session-path", Input{Job: "lead", Dollars: number(1), Session: "../transcript"}, false},
		{"model-text", Input{Job: "lead", Dollars: number(1), Model: "prompt text"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			_, err := s.Post(tc.in)
			if (err == nil) != tc.valid {
				t.Fatalf("Post: %v", err)
			}
			if !tc.valid {
				if _, err := os.Stat(filepath.Join(s.TaskPath, "usage")); !os.IsNotExist(err) {
					t.Fatal("invalid input wrote data")
				}
			}
		})
	}
}

func TestFoldAndRepair(t *testing.T) {
	s := testStore(t)
	for _, in := range []Input{
		{Job: "lead", Dollars: number(1), Minutes: number(2)},
		{Job: "J-2", Dollars: number(3)},
		{Job: "lead", Minutes: number(4)},
		{Job: "J-1.1", Dollars: number(2)},
	} {
		if _, err := s.Post(in); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(s.TaskPath, "usage", "usage.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err := json.Unmarshal([]byte(strings.Split(string(raw), "\n")[0]), &r); err != nil {
		t.Fatal(err)
	}
	if r.At != "2026-01-02T03:04:05.678Z" || r.V != 1 {
		t.Fatal(r)
	}
	if err := os.WriteFile(path, append(raw, []byte(`{"v":1`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Show()
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.Dollars != 6 || got.Totals.Minutes != 6 || got.Totals.Posts != 4 || len(got.Jobs) != 3 || got.Jobs[2].Job != "lead" || got.Jobs[2].Dollars != 1 || got.Jobs[2].Minutes != 6 || got.Jobs[2].Posts != 2 {
		t.Fatal(got)
	}
	if _, err := s.Post(Input{Job: "lead", Dollars: number(1)}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Show()
	if err != nil || got.Totals.Posts != 5 || got.Totals.Dollars != 7 {
		t.Fatalf("%+v: %v", got, err)
	}
	for _, bad := range []string{"oops\n", "{}\n", "null\n", strings.Replace(string(raw[:strings.IndexByte(string(raw), '\n')+1]), `"dollars":1`, `"dollars":null`, 1)} {
		t.Run(bad, func(t *testing.T) {
			broken := append(append(append([]byte{}, raw...), []byte(bad)...), raw...)
			if err := os.WriteFile(path, broken, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Show(); err == nil || !strings.Contains(err.Error(), "line 5") {
				t.Fatalf("Show: %v", err)
			}
			if _, err := s.Post(Input{Job: "lead", Dollars: number(0)}); err == nil || !strings.Contains(err.Error(), "line 5") {
				t.Fatalf("Post: %v", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(broken) {
				t.Fatal("corrupt log changed")
			}
		})
	}
}

func TestEmptyAndOverflow(t *testing.T) {
	s := testStore(t)
	if out, err := s.Show(); err != nil || out.Jobs == nil || out.Totals.Posts != 0 {
		t.Fatalf("%+v %v", out, err)
	}
	for i := 0; i < 2; i++ {
		if _, err := s.Post(Input{Job: "lead", Dollars: number(2e12)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Post(Input{Job: "lead", Dollars: number(2e12)}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := s.Post(Input{Job: "lead", Dollars: number(math.MaxFloat64)}); err == nil {
		t.Fatal("out of range amount accepted")
	}
	if out, err := s.Show(); err != nil || out.Totals.Posts != 2 {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestReadsCreateNothing(t *testing.T) {
	s := testStore(t)
	if _, err := s.Show(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskPath, "usage")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read created usage folder: %v", err)
	}
	if _, err := s.Post(Input{Job: "lead", Dollars: number(1)}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.TaskPath, "usage", ".lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Show(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskPath, "usage", ".lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read created lock: %v", err)
	}
}

func TestTokenRecordCompatibility(t *testing.T) {
	// Version 1 fixture captured from plugin main 5dcc524's Record encoding.
	const old = `{"v":1,"at":"2026-01-02T03:04:05.678Z","job":"lead","session":"","model":"","dollars":1,"minutes":2}` + "\n"
	out, _, err := fold(strings.NewReader(old))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(out)
	if !strings.Contains(string(data), `"tokens":0`) {
		t.Fatalf("old record tokens: %s", data)
	}
	withTokens := strings.Replace(old, `"minutes":2`, `"minutes":2,"tokens":7`, 1)
	out, _, err = fold(strings.NewReader(old + withTokens))
	if err != nil {
		t.Fatalf("token record: %v", err)
	}
	data, _ = json.Marshal(out)
	if !strings.Contains(string(data), `"tokens":7`) {
		t.Fatalf("token record totals: %s", data)
	}
	for _, value := range []string{"null", "-1", "1.5", `"7"`, "18446744073709551616"} {
		if _, _, err := fold(strings.NewReader(strings.Replace(withTokens, `"tokens":7`, `"tokens":`+value, 1))); err == nil {
			t.Fatalf("accepted token record %s", value)
		}
	}
}

func TestTokenRecordRangeAndOmission(t *testing.T) {
	s := testStore(t)
	max, zero, one := uint64(math.MaxUint64), uint64(0), uint64(1)
	for _, in := range []Input{
		{Job: "lead", Dollars: number(0)},
		{Job: "lead", Tokens: &zero},
		{Job: "J-1", Tokens: &max},
	} {
		if _, err := s.Post(in); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(s.TaskPath, "usage", "usage.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(before), "\n")
	if strings.Contains(lines[0], `"tokens"`) || !strings.Contains(lines[1], `"tokens":0`) || !strings.Contains(lines[2], `"tokens":18446744073709551615`) {
		t.Fatalf("token omission/zero/range: %s", before)
	}
	if _, err := s.Post(Input{Job: "lead", Tokens: &one}); err == nil {
		t.Fatal("token overflow accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("overflow changed log")
	}
	out, err := s.Show()
	if err != nil || out.Totals.Tokens != max || out.Totals.Posts != 3 {
		t.Fatalf("%+v %v", out, err)
	}
}
