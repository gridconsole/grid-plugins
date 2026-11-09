package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

func TestRoutineLimitRefusals(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"routine_timeout", "1s"}, {"routine_timeout", "3h"}, {"routine_timeout", "1day"},
		{"routine_timeout", "12"}, {"routine_timeout", "null"}, {"routine_timeout", "[1m]"},
		{"routine_max_turns", "0"}, {"routine_max_turns", "101"}, {"routine_max_turns", "1.5"},
		{"routine_max_turns", `"3"`}, {"routine_max_turns", "true"}, {"routine_max_turns", "null"},
		{"routine_budget", "0"}, {"routine_budget", "-1"}, {"routine_budget", "50.01"},
		{"routine_budget", `"1"`}, {"routine_budget", ".nan"}, {"routine_budget", ".inf"},
		{"routine_budget", "null"}, {"routine_budget", "{}"},
	} {
		t.Run(tc.key+"_"+tc.value, func(t *testing.T) {
			h := newHarness(t)
			store := routine.Store{GridHome: h.gridHome}
			e, err := store.Add("daily", "1h", "grid version")
			if err != nil {
				t.Fatal(err)
			}
			config := brain.Store{GridHome: h.gridHome}
			var before []byte
			err = config.EditConfig("home", "routines", e.Name, func(old []byte) ([]byte, error) {
				e.LastRun = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
				e.Outcome = "success"
				old, err = brain.EncodeConfig(e, "Keep these notes.")
				if err != nil {
					return nil, err
				}
				before = bytes.Replace(old, []byte("---\n"), []byte("---\n"+tc.key+": "+tc.value+"\n"), 1)
				return before, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			h.env.Executable = func() (string, error) { return filepath.Join(h.root, "must-not-run"), nil }
			for _, due := range []bool{false, true} {
				args := []string{"routine", "run", "daily", "--json"}
				if due {
					args = append(args, "--due")
				}
				if code := h.run(h.root, args...); code != 2 {
					t.Errorf("invalid %s: exit %d, want 2: %s", tc.key, code, h.errb)
				}
				if !strings.Contains(h.errb.String(), tc.key) || strings.Contains(h.errb.String(), h.root) {
					t.Errorf("unsafe or unnamed refusal: %s", h.errb)
				}
				after, err := config.ReadConfig("home", "routines", e.Name)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("refusal changed last_run or config: %v", err)
				}
				if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "personal", "notes")); !os.IsNotExist(err) {
					t.Fatalf("refusal produced failure side effects: %v", err)
				}
			}
		})
	}
}

func TestRoutineJSONLimits(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		h := newHarness(t)
		args := []string{"routine", "add", "daily", "--every", "1h", "--run", "grid version"}
		want := map[string]any{"routine_timeout": "10m", "routine_max_turns": float64(10), "routine_budget": float64(1)}
		if explicit {
			args = append(args, "--routine-timeout", "1m", "--routine-max-turns", "3", "--routine-budget", "0.25")
			want = map[string]any{"routine_timeout": "1m", "routine_max_turns": float64(3), "routine_budget": 0.25}
		}
		if code := h.run(h.root, args...); code != 0 {
			t.Fatalf("add: exit %d: %s", code, h.errb)
		}
		h.env.Executable = func() (string, error) { return filepath.Join(h.root, "missing-grid"), nil }
		for _, verb := range []string{"list", "run"} {
			args := []string{"routine", verb}
			if verb == "run" {
				args = append(args, "daily")
			}
			code := h.run(h.root, append(args, "--json")...)
			if (verb == "list" && code != 0) || (verb == "run" && code != 1) {
				t.Fatalf("%s: exit %d: %s", verb, code, h.errb)
			}
			var result map[string]any
			data := h.out.Bytes()
			if verb == "list" {
				var entries []json.RawMessage
				if err := json.Unmarshal(data, &entries); err != nil || len(entries) != 1 {
					t.Fatalf("entries: %s %v", data, err)
				}
				data = entries[0]
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			limits, ok := result["limits"].(map[string]any)
			if !ok {
				t.Fatalf("%s JSON missing limits: %s", verb, data)
			}
			for key, value := range want {
				if limits[key] != value {
					t.Errorf("%s: %v, want %v", key, limits[key], value)
				}
			}
		}
	}
}

func TestRoutineAddRejectsBadLimits(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"routine-timeout", "1s"}, {"routine-timeout", "unknown"},
		{"routine-max-turns", "101"}, {"routine-max-turns", "1.5"},
		{"routine-budget", "0"}, {"routine-budget", "true"},
	} {
		h := newHarness(t)
		code := h.run(h.root, "routine", "add", "bad", "--every", "1h", "--run", "signals", "--"+tc.key, tc.value)
		key := strings.ReplaceAll(tc.key, "-", "_")
		if code != 2 || !strings.Contains(h.errb.String(), key) {
			t.Errorf("%s: exit %d, want 2 naming key: %s", key, code, h.errb)
		}
		entries, err := (routine.Store{GridHome: h.gridHome}).List()
		if err != nil || len(entries) != 0 {
			t.Fatalf("bad add wrote routine: %v %v", entries, err)
		}
	}
}

func TestRoutineHelpNamesLimitDefaults(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"routine", "--help"}, {"routine", "add", "--help"}, {"routine", "run", "--help"}} {
		if code := h.run(h.root, args...); code != 0 {
			t.Fatalf("help: exit %d", code)
		}
		text := h.out.String() + h.errb.String()
		for _, want := range []string{"routine_timeout (default 10m", "routine_max_turns (default 10", "routine_budget (default 1"} {
			if !strings.Contains(text, want) {
				t.Errorf("help missing %q: %s", want, text)
			}
		}
	}
}

func TestRoutineLimitErrorsDoNotEchoValues(t *testing.T) {
	for _, key := range []string{"routine_timeout", "routine_max_turns", "routine_budget"} {
		h := newHarness(t)
		privateValue := filepath.Join(h.root, "private-value")
		flag := "--" + strings.ReplaceAll(key, "_", "-")
		if code := h.run(h.root, "routine", "add", "daily", "--every", "1h", "--run", "signals", flag, privateValue); code != 2 || strings.TrimSpace(h.errb.String()) != "grid: invalid "+key {
			t.Fatalf("unsafe add refusal: exit %d: %s", code, h.errb)
		}
		e, err := (routine.Store{GridHome: h.gridHome}).Add("daily", "1h", "signals")
		if err != nil {
			t.Fatal(err)
		}
		if err := (brain.Store{GridHome: h.gridHome}).EditConfig("home", "routines", e.Name, func(old []byte) ([]byte, error) {
			return bytes.Replace(old, []byte("---\n"), []byte("---\n"+key+": "+strconv.Quote(privateValue)+"\n"), 1), nil
		}); err != nil {
			t.Fatal(err)
		}
		if code := h.run(h.root, "routine", "run", e.Name); code != 2 || strings.TrimSpace(h.errb.String()) != "grid: invalid "+key {
			t.Fatalf("unsafe run refusal: exit %d: %s", code, h.errb)
		}
	}
}
