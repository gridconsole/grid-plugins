package account

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const ybStEvidence = "testdata/yb_stream_evidence.log"

func ybStEnv(t *testing.T) (Env, string) {
	t.Helper()
	home := t.TempDir()
	now := time.Unix(1794700000, 0) // before the evidence line's resetsAt
	return Env{Home: home, GridHome: filepath.Join(home, ".grid"), Now: func() time.Time { return now }}, filepath.Join(home, ".claude-yb")
}

func ybStLog(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "attempt-1.log")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func ybStEvent(typ string, util float64, resets int64) string {
	return `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":` + strconv.FormatInt(resets, 10) +
		`,"rateLimitType":"` + typ + `","utilization":` + strconv.FormatFloat(util, 'f', -1, 64) + `}}`
}

func TestYbStEvidenceLineGivesAStreamSnapshot(t *testing.T) {
	env, dir := ybStEnv(t)
	if err := NoteStream(env, ybStEvidence, dir); err != nil {
		t.Fatal(err)
	}
	s, ok := readSnapshot(env, dir)
	if !ok || s.SevenDay == nil || s.SevenDay.Used != 87 || s.SevenDay.ResetsAt != 1794708000 || s.FiveHour != nil || s.Source != "stream" {
		t.Fatalf("%+v ok=%v", s, ok)
	}
	st, _ := os.Stat(ybStEvidence)
	if want := st.ModTime().UTC().Format(time.RFC3339); s.At != want {
		t.Fatalf("At %q, want the log's modification time %q", s.At, want)
	}
	raw, _ := os.ReadFile(snapshotPath(env, dir))
	if !strings.Contains(string(raw), `"source":"stream"`) {
		t.Fatalf("file: %s", raw)
	}
	w := windowsFor(env, "claude", Claude, []string{dir})
	if w.Source != "stream" || len(w.Windows) != 1 || w.Windows[0].Label != "7d" || w.Windows[0].Used != 87 {
		t.Fatalf("%+v", w)
	}
}

func TestYbStFiveHourAndSevenDayKeepBoth(t *testing.T) {
	env, dir := ybStEnv(t)
	log := ybStLog(t, ybStEvent("five_hour", 0.31, 1794710000), ybStEvent("seven_day", 0.5, 1794720000), ybStEvent("overage", 0.9, 5))
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	s, ok := readSnapshot(env, dir)
	if !ok || s.FiveHour == nil || s.SevenDay == nil || s.FiveHour.Used != 31 || s.SevenDay.Used != 50 || s.FiveHour.ResetsAt != 1794710000 {
		t.Fatalf("%+v ok=%v", s, ok)
	}
}

func TestYbStLaterLineOfOneTypeReplacesAnEarlierOne(t *testing.T) {
	env, dir := ybStEnv(t)
	log := ybStLog(t, ybStEvent("seven_day", 0.2, 1794720000), ybStEvent("five_hour", 0.1, 1794710000), ybStEvent("seven_day", 0.44, 1794730000))
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	s, _ := readSnapshot(env, dir)
	if s.SevenDay == nil || s.SevenDay.Used != 44 || s.SevenDay.ResetsAt != 1794730000 || s.FiveHour == nil || s.FiveHour.Used != 10 {
		t.Fatalf("%+v", s)
	}
}

func TestYbStNoEventWritesNoFile(t *testing.T) {
	env, dir := ybStEnv(t)
	log := ybStLog(t, `{"type":"system","subtype":"init"}`, `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","rateLimitType":"seven_day"}}`)
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshotPath(env, dir)); !os.IsNotExist(err) {
		t.Fatalf("a snapshot was written: %v", err)
	}
	if err := NoteStream(env, filepath.Join(t.TempDir(), "missing.log"), dir); err == nil {
		t.Fatal("a missing log is an error")
	}
}

func TestYbStBadJSONLineAmongGoodOnesIsSkipped(t *testing.T) {
	env, dir := ybStEnv(t)
	log := ybStLog(t, ybStEvent("five_hour", 0.12, 1794710000), `not json rate_limit_event {`, ybStEvent("seven_day", 0.6, 1794720000))
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	s, _ := readSnapshot(env, dir)
	if s.FiveHour == nil || s.SevenDay == nil || s.FiveHour.Used != 12 || s.SevenDay.Used != 60 {
		t.Fatalf("%+v", s)
	}
}

func TestYbStSnapshotNewerThanTheLogIsKeptByteForByte(t *testing.T) {
	env, dir := ybStEnv(t)
	log := ybStLog(t, ybStEvent("seven_day", 0.9, 1794720000))
	mtime := time.Unix(1794600000, 0)
	if err := os.Chtimes(log, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	path := snapshotPath(env, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	existing := `{"configDir":"x","seven_day":{"used_percentage":5,"resets_at":1794720000},"at":"` + time.Unix(1794650000, 0).UTC().Format(time.RFC3339) + `"}` + "\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != existing {
		t.Fatalf("the newer snapshot changed:\n%s", got)
	}
	// An older snapshot is replaced.
	older := strings.Replace(existing, time.Unix(1794650000, 0).UTC().Format(time.RFC3339), time.Unix(1794500000, 0).UTC().Format(time.RFC3339), 1)
	if err := os.WriteFile(path, []byte(older), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NoteStream(env, log, dir); err != nil {
		t.Fatal(err)
	}
	s, _ := readSnapshot(env, dir)
	if s.Source != "stream" || s.SevenDay.Used != 90 {
		t.Fatalf("%+v", s)
	}
}

func TestYbStSnapshotWithNoSourceKeyStillReadsStatusline(t *testing.T) {
	env, dir := ybStEnv(t)
	path := snapshotPath(env, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"configDir":"x","five_hour":{"used_percentage":12,"resets_at":1794710000},"at":"2026-11-11T10:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	w := windowsFor(env, "claude", Claude, []string{dir})
	if w.Source != "statusline" || len(w.Windows) != 1 || w.Windows[0].Used != 12 {
		t.Fatalf("%+v", w)
	}
}
