package account

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The fixtures below keep the shape of the Codex session records the brief
// quotes (codex-cli 0.167.0): one JSON object per line, rate-limit lines are
// {"timestamp", "type":"event_msg", "payload":{"type":"token_count", "info",
// "rate_limits"}}. cxRealLimits is the real reading verbatim (grid, 09 Nov
// 02:54Z: a pro plan with one 10080 minute window as "primary"); cxPremium is
// the shape of the newest lines, a "premium" limit with both windows null.
// A five-hour window was never seen on the real accounts, so the two-window
// fixtures are derived from the same shape with a 300 minute window.
const (
	cxInfo       = `{"total_token_usage":{"input_tokens":1200,"cached_input_tokens":0,"output_tokens":80,"reasoning_output_tokens":0,"total_tokens":1280},"last_token_usage":{"input_tokens":1200,"cached_input_tokens":0,"output_tokens":80,"reasoning_output_tokens":0,"total_tokens":1280},"model_context_window":258400}`
	cxRealLimits = `{"limit_id":"codex","limit_name":null,"primary":{"used_percent":100.0,"window_minutes":10080,"resets_at":1794778709},"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":"0"},"individual_limit":null,"spend_control_reached":null,"plan_type":"pro","rate_limit_reached_type":null}`
	cxPremium    = `{"limit_id":"premium","limit_name":null,"primary":null,"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":"0"},"individual_limit":null,"spend_control_reached":null,"plan_type":"pro","rate_limit_reached_type":null}`
	cxReset7d    = 1794778709
	cxReset5h    = 1794710000
)

var cxNow = time.Unix(1794700000, 0) // before both resets

func cxEnv(now time.Time) Env {
	return Env{Now: func() time.Time { return now }}
}

// cxLine is one token_count record with the given timestamp, info and rate_limits.
func cxLine(ts, info, limits string) string {
	return `{"timestamp":"` + ts + `","type":"event_msg","payload":{"type":"token_count","info":` + info + `,"rate_limits":` + limits + `}}`
}

// cxLimits is a "codex" rate_limits object with the given window objects.
func cxLimits(primary, secondary string) string {
	return `{"limit_id":"codex","limit_name":null,"primary":` + primary + `,"secondary":` + secondary + `,"credits":{"has_credits":false,"unlimited":false,"balance":"0"},"individual_limit":null,"spend_control_reached":null,"plan_type":"pro","rate_limit_reached_type":null}`
}

func cxWindow(used float64, minutes int, resets int64) string {
	return fmt.Sprintf(`{"used_percent":%v,"window_minutes":%d,"resets_at":%d}`, used, minutes, resets)
}

// cxRollout writes sessions/YYYY/MM/DD/rollout-<time>-<uuid>.jsonl under the
// Codex folder home, with the lines joined by newlines and mtime as given.
func cxRollout(t *testing.T, home, day, stamp string, mtime time.Time, lines ...string) string {
	t.Helper()
	path := filepath.Join(home, "sessions", filepath.FromSlash(day), "rollout-"+stamp+"-0198f2c4-7a10-7c2e-9b1d-3e5a6c7d8e9f.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

func cxNone(t *testing.T, got AccountWindows) {
	t.Helper()
	if got.Source != "none" || got.Windows == nil || len(got.Windows) != 0 || got.AsOf != "" || got.Note != "Codex use is not read yet" {
		t.Fatalf("want source none, no windows and the note, got %+v", got)
	}
}

func TestCxRealReadingGivesSevenDayAndPremiumLineIsSkipped(t *testing.T) {
	home := t.TempDir()
	// The real reading, then a newer premium line with null windows in the same
	// file, and a newer file holding only premium lines.
	cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 2, 56, 0, 0, time.UTC),
		cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxRealLimits),
		cxLine("2026-11-09T02:55:10.100Z", cxInfo, cxPremium))
	cxRollout(t, home, "2026/11/10", "2026-11-10T09-00-00", time.Date(2026, 11, 10, 9, 5, 0, 0, time.UTC),
		cxLine("2026-11-10T09:01:00.000Z", cxInfo, cxPremium),
		cxLine("2026-11-10T09:02:00.000Z", `null`, cxPremium))
	got := windowsFor(cxEnv(cxNow), "codex-cara", Codex, []string{home})
	want := AccountWindows{ID: "codex-cara", Kind: Codex, Windows: []Window{{"7d", 100, cxReset7d}}, AsOf: "2026-11-09T02:54:26Z", Source: "codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestCxFiveHourAndSevenDayBothRead(t *testing.T) {
	home := t.TempDir()
	cxRollout(t, home, "2026/11/12", "2026-11-12T10-00-00", time.Date(2026, 11, 12, 10, 30, 0, 0, time.UTC),
		cxLine("2026-11-12T10:20:00.500Z", cxInfo, cxLimits(cxWindow(31.5, 300, cxReset5h), cxWindow(62, 10080, cxReset7d))))
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
	want := []Window{{"5h", 31.5, cxReset5h}, {"7d", 62, cxReset7d}}
	if got.Source != "codex" || got.AsOf != "2026-11-12T10:20:00Z" || !reflect.DeepEqual(got.Windows, want) || got.Note != "" {
		t.Fatalf("got %+v\nwant windows %+v", got, want)
	}
}

func TestCxPositionsAreNotMeanings(t *testing.T) {
	home := t.TempDir()
	// The long window comes first and the short one second: the answer still
	// lists 5h then 7d, each by its length.
	cxRollout(t, home, "2026/11/12", "2026-11-12T10-00-00", time.Date(2026, 11, 12, 10, 30, 0, 0, time.UTC),
		cxLine("2026-11-12T10:20:00.500Z", cxInfo, cxLimits(cxWindow(62, 10080, cxReset7d), cxWindow(31.5, 300, cxReset5h))))
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
	want := []Window{{"5h", 31.5, cxReset5h}, {"7d", 62, cxReset7d}}
	if got.Source != "codex" || !reflect.DeepEqual(got.Windows, want) {
		t.Fatalf("got %+v\nwant windows %+v", got, want)
	}
}

func TestCxWindowIsMappedByItsLength(t *testing.T) {
	cases := []struct {
		name   string
		window string
		label  string // "" means the window is not read
	}{
		{"one hour is 5h", cxWindow(10, 60, cxReset5h), "5h"},
		{"300 is 5h", cxWindow(10, 300, cxReset5h), "5h"},
		{"360 is the longest 5h", cxWindow(10, 360, cxReset5h), "5h"},
		{"361 is between and not read", cxWindow(10, 361, cxReset5h), ""},
		{"1439 is between and not read", cxWindow(10, 1439, cxReset5h), ""},
		{"1440 is the shortest 7d", cxWindow(10, 1440, cxReset7d), "7d"},
		{"10080 is 7d", cxWindow(10, 10080, cxReset7d), "7d"},
		{"no length is not read", `{"used_percent":10.0,"resets_at":1794710000}`, ""},
		{"no used_percent is not read", `{"window_minutes":300,"resets_at":1794710000}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			cxRollout(t, home, "2026/11/12", "2026-11-12T10-00-00", time.Date(2026, 11, 12, 10, 30, 0, 0, time.UTC),
				cxLine("2026-11-12T10:20:00.500Z", cxInfo, cxLimits(c.window, `null`)))
			got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
			if c.label == "" {
				cxNone(t, got)
				return
			}
			if got.Source != "codex" || len(got.Windows) != 1 || got.Windows[0].Label != c.label || got.Windows[0].Used != 10 {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestCxUnmappedLineFallsBackToAnOlderMappedLine(t *testing.T) {
	home := t.TempDir()
	cxRollout(t, home, "2026/11/12", "2026-11-12T10-00-00", time.Date(2026, 11, 12, 10, 30, 0, 0, time.UTC),
		cxLine("2026-11-12T10:10:00.000Z", cxInfo, cxLimits(cxWindow(40, 10080, cxReset7d), `null`)),
		cxLine("2026-11-12T10:20:00.000Z", cxInfo, cxLimits(cxWindow(90, 600, cxReset5h), `null`)))
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
	if got.Source != "codex" || len(got.Windows) != 1 || got.Windows[0] != (Window{"7d", 40, cxReset7d}) || got.AsOf != "2026-11-12T10:10:00Z" {
		t.Fatalf("got %+v", got)
	}
}

func TestCxPassedResetGivesZeroUsed(t *testing.T) {
	home := t.TempDir()
	cxRollout(t, home, "2026/11/12", "2026-11-12T10-00-00", time.Date(2026, 11, 12, 10, 30, 0, 0, time.UTC),
		cxLine("2026-11-12T10:20:00.500Z", cxInfo, cxLimits(cxWindow(31.5, 300, cxReset5h), cxWindow(62, 10080, cxReset7d))))
	// The 5h window reset at cxReset5h; the 7d window has not.
	now := time.Unix(cxReset5h, 0)
	got := windowsFor(cxEnv(now), "codex", Codex, []string{home})
	want := []Window{{"5h", 0, cxReset5h}, {"7d", 62, cxReset7d}}
	if got.Source != "codex" || !reflect.DeepEqual(got.Windows, want) {
		t.Fatalf("at the reset second: got %+v\nwant %+v", got.Windows, want)
	}
	// One second before the reset the reading stands.
	got = windowsFor(cxEnv(now.Add(-time.Second)), "codex", Codex, []string{home})
	if got.Windows[0] != (Window{"5h", 31.5, cxReset5h}) {
		t.Fatalf("before the reset: got %+v", got.Windows)
	}
	// Both passed.
	got = windowsFor(cxEnv(time.Unix(cxReset7d+1, 0)), "codex", Codex, []string{home})
	if got.Windows[0].Used != 0 || got.Windows[1].Used != 0 || got.Windows[1].ResetsAt != cxReset7d {
		t.Fatalf("after both resets: got %+v", got.Windows)
	}
}

func TestCxNoReadingKeepsTheNoneAnswer(t *testing.T) {
	t.Run("no folder", func(t *testing.T) {
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{filepath.Join(t.TempDir(), "absent")}))
	})
	t.Run("no dirs", func(t *testing.T) {
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, nil))
	})
	t.Run("folder without sessions", func(t *testing.T) {
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{t.TempDir()}))
	})
	t.Run("only premium lines and an empty file", func(t *testing.T) {
		home := t.TempDir()
		cxRollout(t, home, "2026/11/10", "2026-11-10T09-00-00", time.Date(2026, 11, 10, 9, 5, 0, 0, time.UTC),
			cxLine("2026-11-10T09:01:00.000Z", cxInfo, cxPremium))
		cxRollout(t, home, "2026/11/10", "2026-11-10T10-00-00", time.Date(2026, 11, 10, 10, 5, 0, 0, time.UTC), "")
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{home}))
	})
	t.Run("a reading in a file that is not a rollout", func(t *testing.T) {
		home := t.TempDir()
		p := filepath.Join(home, "sessions", "2026", "11", "09", "notes.jsonl")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxRealLimits)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{home}))
	})
}

func TestCxTornLineIsSkipped(t *testing.T) {
	good := cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxRealLimits)
	t.Run("a torn last line without its newline", func(t *testing.T) {
		home := t.TempDir()
		p := cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), good)
		torn := cxLine("2026-11-09T02:58:00.000Z", cxInfo, cxLimits(cxWindow(1, 300, cxReset5h), `null`))
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(torn[:len(torn)-60]); err != nil { // cut inside rate_limits, no closing braces
			t.Fatal(err)
		}
		f.Close()
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if got.Source != "codex" || len(got.Windows) != 1 || got.Windows[0] != (Window{"7d", 100, cxReset7d}) || got.AsOf != "2026-11-09T02:54:26Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("a torn line between two good ones", func(t *testing.T) {
		home := t.TempDir()
		later := cxLine("2026-11-09T02:59:00.000Z", cxInfo, cxLimits(cxWindow(77, 10080, cxReset7d), `null`))
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC),
			good, `{"timestamp":"2026-11-09T02:57:00.000Z","type":"event_msg","payload":{"type":"token_count","rate_li`, later)
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 77 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("a file that is only a torn line", func(t *testing.T) {
		home := t.TempDir()
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), good[:len(good)-40])
		cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{home}))
	})
}

func TestCxNewestReadingByTimestampAcrossFiles(t *testing.T) {
	older := cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxLimits(cxWindow(20, 10080, cxReset7d), `null`))
	newer := cxLine("2026-11-10T08:00:00.000Z", cxInfo, cxLimits(cxWindow(55, 10080, cxReset7d), `null`))
	t.Run("a newer file next to an older one", func(t *testing.T) {
		home := t.TempDir()
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), older)
		cxRollout(t, home, "2026/11/10", "2026-11-10T07-50-00", time.Date(2026, 11, 10, 8, 5, 0, 0, time.UTC), newer)
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 55 || got.AsOf != "2026-11-10T08:00:00Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("a newer file with only premium lines falls back to the older file", func(t *testing.T) {
		home := t.TempDir()
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), older)
		cxRollout(t, home, "2026/11/10", "2026-11-10T07-50-00", time.Date(2026, 11, 10, 8, 5, 0, 0, time.UTC),
			cxLine("2026-11-10T08:00:00.000Z", cxInfo, cxPremium))
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 20 || got.AsOf != "2026-11-09T02:54:26Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("a resumed session: the older named file holds the newer line", func(t *testing.T) {
		home := t.TempDir()
		// Named for 09 Nov but appended to on 12 Nov; the 10 Nov file is newer by name.
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC),
			older, cxLine("2026-11-12T08:59:00.000Z", cxInfo, cxLimits(cxWindow(71, 10080, cxReset7d), `null`)))
		cxRollout(t, home, "2026/11/10", "2026-11-10T07-50-00", time.Date(2026, 11, 10, 8, 5, 0, 0, time.UTC), newer)
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 71 || got.AsOf != "2026-11-12T08:59:00Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("the latest timestamp wins over the latest file time", func(t *testing.T) {
		home := t.TempDir()
		// The file touched last holds an old reading and later lines of another
		// kind; the file touched before it holds the newer reading.
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC),
			older, `{"timestamp":"2026-11-12T08:59:00.000Z","type":"event_msg","payload":{"type":"agent_message","message":"done"}}`)
		cxRollout(t, home, "2026/11/10", "2026-11-10T07-50-00", time.Date(2026, 11, 11, 8, 5, 0, 0, time.UTC), newer)
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 55 || got.AsOf != "2026-11-10T08:00:00Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("the last reading of one file wins", func(t *testing.T) {
		home := t.TempDir()
		cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC),
			cxLine("2026-11-09T02:50:00.000Z", cxInfo, cxLimits(cxWindow(5, 10080, cxReset7d), `null`)),
			cxLine("2026-11-09T02:52:00.000Z", cxInfo, cxLimits(cxWindow(9, 10080, cxReset7d), `null`)),
			cxLine("2026-11-09T02:53:00.000Z", `null`, `null`))
		got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
		if len(got.Windows) != 1 || got.Windows[0].Used != 9 {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestCxLongLinesAreRead(t *testing.T) {
	home := t.TempDir()
	// Lines of 70355 bytes were seen; here one before the reading and the
	// reading line itself are 200000 bytes, far past a default bufio.Scanner buffer.
	pad := `{"cwd":"` + strings.Repeat("x", 200000) + `"}`
	long := cxLine("2026-11-09T02:54:26.916Z", pad, cxRealLimits)
	if len(long) <= 70355 {
		t.Fatalf("fixture line is %d bytes", len(long))
	}
	cxRollout(t, home, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC),
		`{"timestamp":"2026-11-09T02:50:00.000Z","type":"response_item","payload":{"text":"`+strings.Repeat("y", 200000)+`"}}`, long,
		cxLine("2026-11-09T02:55:10.100Z", cxInfo, cxPremium))
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{home})
	if got.Source != "codex" || len(got.Windows) != 1 || got.Windows[0] != (Window{"7d", 100, cxReset7d}) {
		t.Fatalf("got %+v", got)
	}
}

func TestCxFirstFolderWithAReadingWins(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	lineA := cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxLimits(cxWindow(11, 10080, cxReset7d), `null`))
	lineB := cxLine("2026-11-10T02:54:26.916Z", cxInfo, cxLimits(cxWindow(22, 10080, cxReset7d), `null`))
	cxRollout(t, b, "2026/11/10", "2026-11-10T02-50-00", time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC), lineB)
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{a, b})
	if len(got.Windows) != 1 || got.Windows[0].Used != 22 {
		t.Fatalf("the folder without a reading is passed over: %+v", got)
	}
	cxRollout(t, a, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), lineA)
	got = windowsFor(cxEnv(cxNow), "codex", Codex, []string{a, b})
	if len(got.Windows) != 1 || got.Windows[0].Used != 11 {
		t.Fatalf("the first folder with a reading wins even when the second is newer: %+v", got)
	}
}

func TestCxEmptyFolderIsNotTheWorkingDirectory(t *testing.T) {
	// A default account passes ~/.codex first, and it is "" when that folder does
	// not exist: that must not read ./sessions of the working directory.
	cwd := t.TempDir()
	cxRollout(t, cwd, "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC),
		cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxLimits(cxWindow(99, 10080, cxReset7d), `null`)))
	t.Chdir(cwd)
	real := t.TempDir()
	cxRollout(t, real, "2026/11/10", "2026-11-10T02-50-00", time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC),
		cxLine("2026-11-10T02:54:26.916Z", cxInfo, cxLimits(cxWindow(33, 10080, cxReset7d), `null`)))
	got := windowsFor(cxEnv(cxNow), "codex", Codex, []string{"", real})
	if len(got.Windows) != 1 || got.Windows[0].Used != 33 {
		t.Fatalf("got %+v", got)
	}
	cxNone(t, windowsFor(cxEnv(cxNow), "codex", Codex, []string{""}))
}

func TestCxWindowsAnswersACodexAccountFromItsFolder(t *testing.T) {
	env, _ := machine(t)
	env.Now = func() time.Time { return cxNow }
	// The account is signed in by auth.json, so no provider is run.
	write(t, filepath.Join(env.Home, ".codex", "auth.json"), `{"tokens":{"id_token":"`+idToken("cody@example.com")+`"}}`)
	cxRollout(t, filepath.Join(env.Home, ".codex"), "2026/11/09", "2026-11-09T02-50-00", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC),
		cxLine("2026-11-09T02:54:26.916Z", cxInfo, cxRealLimits),
		cxLine("2026-11-09T02:55:10.100Z", cxInfo, cxPremium))
	got, err := Windows(env, "")
	if err != nil {
		t.Fatal(err)
	}
	var codex []AccountWindows
	for _, w := range got {
		if w.Kind == Codex {
			codex = append(codex, w)
		}
	}
	if len(codex) != 1 || codex[0].Source != "codex" || len(codex[0].Windows) != 1 || codex[0].Windows[0] != (Window{"7d", 100, cxReset7d}) {
		t.Fatalf("got %+v", got)
	}
}
