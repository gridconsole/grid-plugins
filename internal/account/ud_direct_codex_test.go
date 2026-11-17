package account

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// udFakeCodex is `codex` for the direct read. `login status` exits 0 when the
// folder has a fake-login file. `app-server` logs the three lines it is sent,
// answers initialize, waits for account/rateLimits/read and answers it by the
// mode in <folder>/fake-rl: ok, error-once (an error object on the first start of
// the folder, the result after), error-always or hang. The result is the first
// line of <folder>/fake-rl-json. It then waits for stdin to close.
const udFakeCodex = `#!/bin/sh
dir="${CODEX_HOME:-$FAKE_HOME/.codex}"
case "$1" in
login)
  [ -f "$dir/fake-login" ] && exit 0
  exit 1;;
app-server)
  if [ "$*" != "app-server" ]; then echo "bad codex args $*" >> "$FAKE_LOG"; exit 3; fi
  n=0
  [ -f "$dir/fake-starts" ] && read -r n < "$dir/fake-starts"
  n=$((n+1))
  echo "$n" > "$dir/fake-starts"
  echo "codexstart dir=$dir homevar=${CODEX_HOME+set} start=$n" >> "$FAKE_LOG"
  read -r l1; echo "rx $l1" >> "$FAKE_LOG"
  id=${l1#*\"id\":}; id=${id%%[,\}]*}
  printf '{"id":%s,"result":{"userAgent":"fake"}}\n' "$id"
  read -r l2; echo "rx $l2" >> "$FAKE_LOG"
  read -r l3; echo "rx $l3" >> "$FAKE_LOG"
  id=${l3#*\"id\":}; id=${id%%[,\}]*}
  mode=ok
  [ -f "$dir/fake-rl" ] && read -r mode < "$dir/fake-rl"
  payload=""
  [ -f "$dir/fake-rl-json" ] && read -r payload < "$dir/fake-rl-json"
  case "$mode" in
  hang) sleep 60 & echo $! >> "$FAKE_LOG.pids"; wait; exit 0;;
  error-always) printf '{"id":%s,"error":{"code":-32603,"message":"SECRET-CODEX-ERROR"}}\n' "$id";;
  error-once)
    if [ "$n" = 1 ]; then
      printf '{"id":%s,"error":{"code":-32603,"message":"SECRET-CODEX-ERROR"}}\n' "$id"
    else
      printf '{"id":%s,"result":%s}\n' "$id" "$payload"
    fi;;
  *) printf '{"id":%s,"result":%s}\n' "$id" "$payload";;
  esac
  read -r l4
  exit 0;;
esac
exit 2
`

func udCodexPayload(used1 float64, mins1 int, reset1 int64, used2 float64, mins2 int, reset2 int64) string {
	rl := fmt.Sprintf(`{"limitId":"codex","primary":{"usedPercent":%v,"windowDurationMins":%d,"resetsAt":%d},"secondary":{"usedPercent":%v,"windowDurationMins":%d,"resetsAt":%d},"credits":null,"planType":"plus","rateLimitReachedType":null}`,
		used1, mins1, reset1, used2, mins2, reset2)
	return `{"rateLimits":` + rl + `,"rateLimitsByLimitId":{"codex":` + rl + `},"ordinaryUsageAllowed":true}`
}

func (m *udMach) codex(t *testing.T, mode, payload string) string {
	t.Helper()
	dir := m.folder(".codex")
	write(t, filepath.Join(dir, "fake-login"), "")
	write(t, filepath.Join(dir, "fake-rl"), mode+"\n")
	write(t, filepath.Join(dir, "fake-rl-json"), payload+"\n")
	return dir
}

// Row 2: Codex windows come from `codex app-server`: initialize, initialized,
// account/rateLimits/read, by windowDurationMins.
func TestUdCodexWindowsFromAppServer(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	dir := m.codex(t, "ok", udCodexPayload(12.5, 300, five, 40, 10080, week))

	got := udRow(t, m.windows(t, ""), "codex")
	udWant(t, got, "direct", Window{"5h", 12.5, five}, Window{"7d", 40, week})
	if got.AsOf != fixedNow.Format(time.RFC3339) {
		t.Fatalf("asOf: %+v", got)
	}
	m.noBad(t)
	if starts := m.logLines("codexstart"); len(starts) != 1 || !strings.Contains(starts[0], "dir="+dir+" ") || !strings.Contains(starts[0], "homevar=set") {
		t.Fatalf("one app-server on the folder, CODEX_HOME set: %v", starts)
	}
	rx := strings.Join(m.logLines("rx"), "\n")
	i, j, k := strings.Index(rx, `"method":"initialize"`), strings.Index(rx, `"method":"initialized"`), strings.Index(rx, `"method":"account/rateLimits/read"`)
	if i < 0 || j < i || k < j {
		t.Fatalf("the app-server must be sent initialize, initialized, then account/rateLimits/read, in that order:\n%s", rx)
	}
	if len(m.logLines("rx")) != 3 {
		t.Fatalf("three lines are sent: %s", rx)
	}
}

// The windows are named by their length, whichever of primary and secondary
// carries them; a length that is neither is not shown.
func TestUdCodexMapsWindowsByDuration(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(time.Hour).Unix(), fixedNow.Add(24*time.Hour).Unix()
	m.codex(t, "ok", udCodexPayload(70, 10080, week, 9, 300, five))
	udWant(t, udRow(t, m.windows(t, ""), "codex"), "direct", Window{"5h", 9, five}, Window{"7d", 70, week})

	m2 := udMachine(t)
	m2.codex(t, "ok", udCodexPayload(5, 60, five, 30, 10080, week))
	udWant(t, udRow(t, m2.windows(t, ""), "codex"), "direct", Window{"7d", 30, week})
}

// Row 2: an error reply is tried once more after a short back-off.
func TestUdCodexRetriesOneErrorReply(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	m.codex(t, "error-once", udCodexPayload(12.5, 300, five, 40, 10080, week))
	got := udRow(t, m.windows(t, ""), "codex")
	udWant(t, got, "direct", Window{"5h", 12.5, five}, Window{"7d", 40, week})
	if n := len(m.logLines("codexstart")); n != 2 {
		t.Fatalf("the first error reply gets one retry: %d starts", n)
	}
}

func TestUdCodexTwoErrorRepliesGiveAReason(t *testing.T) {
	m := udMachine(t)
	m.codex(t, "error-always", "{}")
	got := udRow(t, m.windows(t, ""), "codex")
	if got.Source != "none" || len(got.Windows) != 0 || !strings.Contains(got.Note, "error") || strings.Contains(got.Note, "SECRET") {
		t.Fatalf("%+v", got)
	}
	if n := len(m.logLines("codexstart")); n != 2 {
		t.Fatalf("one retry and no more: %d starts", n)
	}
	// The failure backs off for 5 minutes: no new app-server inside it.
	m.at(4 * time.Minute)
	m.windows(t, "")
	if n := len(m.logLines("codexstart")); n != 2 {
		t.Fatalf("back-off after a failure: %d starts", n)
	}
}

// A hung app-server is killed at the timeout, and a failure keeps the last good.
func TestUdCodexHangIsKilledAndKeepsTheLastGood(t *testing.T) {
	m := udMachine(t)
	five, week := fixedNow.Add(2*time.Hour).Unix(), fixedNow.Add(72*time.Hour).Unix()
	dir := m.codex(t, "ok", udCodexPayload(12.5, 300, five, 40, 10080, week))
	udWant(t, udRow(t, m.windows(t, ""), "codex"), "direct", Window{"5h", 12.5, five}, Window{"7d", 40, week})

	m.env.ProbeTimeout = 700 * time.Millisecond
	write(t, filepath.Join(dir, "fake-rl"), "hang\n")
	m.at(3 * time.Minute)
	start := time.Now()
	got := udRow(t, m.windows(t, ""), "codex")
	if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
		t.Fatalf("the hung app-server must be killed at the timeout: %v", elapsed)
	}
	udWant(t, got, "direct", Window{"5h", 12.5, five}, Window{"7d", 40, week})
	if got.AsOf != fixedNow.Format(time.RFC3339) {
		t.Fatalf("the last good keeps its asOf: %+v", got)
	}
}

// With no earlier reading, a hang gives the reason line.
func TestUdCodexHangWithNoLastGoodSaysTimedOut(t *testing.T) {
	m := udMachine(t)
	m.env.ProbeTimeout = 700 * time.Millisecond
	m.codex(t, "hang", "{}")
	got := udRow(t, m.windows(t, ""), "codex")
	if got.Source != "none" || !strings.Contains(got.Note, "timed out") {
		t.Fatalf("%+v", got)
	}
}
