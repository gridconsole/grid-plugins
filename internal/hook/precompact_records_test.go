package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// PreCompact only records: it prints no guidance for the summary (simplify.md
// row 6), exits 0, and still writes the session activity record.
func TestPreCompactOnlyRecords(t *testing.T) {
	f := eventSetup(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := session.Store{TaskDir: f.dir, Now: func() time.Time { return now }}
	key := session.Key{SessionID: "session-demo", MachineID: "machine-demo"}
	r, err := s.Start(session.Record{Key: key, PID: 123, PIDStart: "start", Lease: "J-1", Holder: "member@example.com", Fence: 2, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reconcile(key, session.Observation{Process: session.Alive, PID: 123, PIDStart: "start"}); err != nil {
		t.Fatal(err)
	}
	out, log, code := f.run(t, "PreCompact", hookio.Input{})
	if code != 0 || log != "" {
		t.Fatalf("precompact: %d %q", code, log)
	}
	if strings.Contains(out, "in the summary") || strings.TrimSpace(out) != "" {
		t.Fatalf("precompact printed guidance: %q", out)
	}
	got, err := s.Show(key)
	if err != nil || got.Last.Equal(r.Last) {
		t.Fatalf("session record not written: %+v %v", got, err)
	}
}

// A settings.json written before the precompact hook was removed still loads:
// the key is ignored and the hook list has no precompact row.
func TestRetiredPreCompactKeyIsIgnored(t *testing.T) {
	home := t.TempDir()
	for _, level := range []string{"warn", "block"} {
		body := `{"hooks":{"precompact":"` + level + `","host-guard":"warn"}}`
		if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		levels, err := settings.ReadHookLevels(home, "")
		if err != nil {
			t.Fatalf("settings naming precompact %s refused: %v", level, err)
		}
		if _, ok := levels["precompact"]; ok {
			t.Fatalf("retired hook still listed: %v", levels)
		}
		if levels["host-guard"] != "warn" {
			t.Fatalf("host-guard = %q, want warn", levels["host-guard"])
		}
	}
}
