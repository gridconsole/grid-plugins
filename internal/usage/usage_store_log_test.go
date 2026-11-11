package usage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

// usErr is the read error ParseResult must pass through.
var usErr = errors.New("us read failure")

func usValid() Input { return Input{Job: "lead", Dollars: number(1)} }

func usLogPath(s Store) string { return filepath.Join(s.TaskPath, "usage", "usage.jsonl") }

func usWantErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func usWantPrefix(t *testing.T, err error, prefix string) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error = %v, want prefix %q", err, prefix)
	}
}

func TestUsageStoreTaskChecks(t *testing.T) {
	_, err := Store{}.Show()
	usWantErr(t, err, "a task is required")

	_, err = Store{}.Post(usValid())
	usWantErr(t, err, "a task is required")

	missing := Store{TaskPath: filepath.Join(t.TempDir(), "missing")}
	if _, err = missing.Show(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing task error = %v, want not-exist", err)
	}

	file := filepath.Join(t.TempDir(), "task")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Store{TaskPath: file}.Show()
	usWantErr(t, err, "task is not a directory")
}

func TestUsageStoreKeyCheck(t *testing.T) {
	s := testStore(t)
	in := usValid()
	in.Key = "a/b"
	_, err := s.Post(in)
	usWantErr(t, err, "key must be an identifier, not a path or text")
	sum, err := s.Show()
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Jobs) != 0 || sum.Totals.Posts != 0 {
		t.Fatalf("summary after refused key = %+v, want empty", sum)
	}
}

func TestUsageStorePostUsageIsFile(t *testing.T) {
	s := testStore(t)
	if err := os.WriteFile(filepath.Join(s.TaskPath, "usage"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.Post(usValid())
	usWantPrefix(t, err, "open ")
}

// usBadLine posts a valid input, changes one field of the line it wrote, and
// returns the store holding that single changed line.
func usBadLine(t *testing.T, change func(map[string]any)) Store {
	t.Helper()
	s := testStore(t)
	if _, err := s.Post(usValid()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(usLogPath(s))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	change(rec)
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usLogPath(s), append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUsageStoreBadLogLines(t *testing.T) {
	const prefix = "usage.jsonl line 1: "
	t.Run("two-objects", func(t *testing.T) {
		s := testStore(t)
		if _, err := s.Post(usValid()); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(usLogPath(s))
		if err != nil {
			t.Fatal(err)
		}
		one := strings.TrimSuffix(string(raw), "\n")
		if err := os.WriteFile(usLogPath(s), []byte(one+one+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = s.Show()
		usWantErr(t, err, prefix+"expected one JSON object")
	})
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"version", func(m map[string]any) { m["v"] = 2 }, "unsupported usage version"},
		{"at", func(m map[string]any) { m["at"] = "2026-01-02T03:04:05Z" }, "at must be UTC with millisecond precision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := usBadLine(t, tc.change)
			_, err := s.Show()
			usWantErr(t, err, prefix+tc.want)
		})
	}
}

func TestUsageParseResultEdges(t *testing.T) {
	t.Run("read-error", func(t *testing.T) {
		_, _, err := ParseResult(iotest.ErrReader(usErr))
		if !errors.Is(err, usErr) {
			t.Fatalf("error = %v, want usErr", err)
		}
	})
	t.Run("token-overflow", func(t *testing.T) {
		line := `{"type":"result","session_id":"s1","total_cost_usd":1,"usage":{"input_tokens":18446744073709551615,"output_tokens":1}}`
		_, _, err := ParseResult(strings.NewReader(line + "\n"))
		usWantErr(t, err, "result line token counts overflow")
	})
	t.Run("refused-model-skipped", func(t *testing.T) {
		line := `{"type":"result","session_id":"s1","total_cost_usd":1,"modelUsage":{"bad model!":{"costUSD":5},"m1":{"costUSD":1}}}`
		in, _, err := ParseResult(strings.NewReader(line + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if in.Model != "m1" {
			t.Fatalf("Model = %q, want m1", in.Model)
		}
	})
}
