package account

import (
	"testing"
	"time"
)

// Row 3 for Codex: the direct read is the first source. The newest rollout line
// of a session sits behind it, and wins only when it was written after the direct
// reading, as a status line note does for Claude. With no direct reading it is
// the first source left. The fake `codex` answers the app-server; the rollout is
// a real file under <folder>/sessions in the shape the P-codex-windows-read tests
// use.

// udRollout writes one rollout line (5h and 7d windows) to the folder dir.
func udRollout(t *testing.T, dir, ts string, mtime time.Time, used5 float64, reset5 int64, used7 float64, reset7 int64) {
	t.Helper()
	cxRollout(t, dir, "2026/11/11", "2026-11-11T09-00-00", mtime,
		cxLine(ts, cxInfo, cxLimits(cxWindow(used5, 300, reset5), cxWindow(used7, 10080, reset7))))
}

func TestUdCodexDirectBeatsAnOlderRollout(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	dir := m.codex(t, "ok", udCodexPayload(12.5, 300, five, 40, 10080, week))
	udRollout(t, dir, "2026-11-11T10:30:00.000Z", fixedNow.Add(-time.Hour), 80, five, 90, week)

	got := udRow(t, m.windows(t, ""), "codex")
	udWant(t, got, "direct", Window{"5h", 12.5, five}, Window{"7d", 40, week})
	if got.AsOf != fixedNow.Format(time.RFC3339) {
		t.Fatalf("the direct read's own time: %+v", got)
	}
}

func TestUdCodexNewerRolloutBeatsDirect(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	dir := m.codex(t, "ok", udCodexPayload(12.5, 300, five, 40, 10080, week))
	if got := udRow(t, m.windows(t, ""), "codex"); got.Source != "direct" {
		t.Fatalf("first read: %+v", got)
	}

	// A Codex turn writes a rollout line a minute later; the next answer is inside
	// the 2 minute gap, so no new app-server starts.
	udRollout(t, dir, "2026-11-11T12:01:00.000Z", fixedNow.Add(time.Minute), 33, five, 44, week)
	m.at(90 * time.Second)
	got := udRow(t, m.windows(t, ""), "codex")
	udWant(t, got, "codex", Window{"5h", 33, five}, Window{"7d", 44, week})
	if got.AsOf != "2026-11-11T12:01:00Z" || len(m.logLines("codexstart")) != 1 {
		t.Fatalf("the rollout line's time, and no second app-server: %+v, %d starts", got, len(m.logLines("codexstart")))
	}
}

func TestUdCodexRolloutAnswersWhenTheDirectReadFailed(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	dir := m.codex(t, "error-always", "{}")
	udRollout(t, dir, "2026-11-11T10:30:00.000Z", fixedNow.Add(-time.Hour), 80, five, 90, week)

	got := udRow(t, m.windows(t, ""), "codex")
	udWant(t, got, "codex", Window{"5h", 80, five}, Window{"7d", 90, week})
	if len(m.logLines("codexstart")) != 2 {
		t.Fatalf("the direct read was tried, with its one retry: %d starts", len(m.logLines("codexstart")))
	}
}
