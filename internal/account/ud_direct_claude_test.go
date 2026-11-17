package account

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// These tests cover the direct read of a Claude folder's windows: Grid asks
// Claude Code's own get_usage request, with no status line and no session. The
// provider is a fake `claude` on PATH; it never reads a real account.

// udFakeClaude is `claude` for the direct read. `auth status` answers from the
// fake-email file of the folder. `-p ...` is the usage probe: it refuses (exit 3
// and a "bad" line in FAKE_LOG) when the arguments, the one stdin line, the
// environment or the empty cwd are not what the brief says, logs one "probe"
// line, then answers from the first line of <folder>/fake-usage: a control_response
// payload, or hang, fail, garbage or error.
const udFakeClaude = `#!/bin/sh
dir="${CLAUDE_CONFIG_DIR:-$FAKE_HOME/.claude}"
case "$1" in
auth)
  if [ "$2" = status ]; then
    if [ -f "$dir/fake-email" ]; then
      read -r email < "$dir/fake-email"
      printf '{"loggedIn":true,"email":"%s"}\n' "$email"
      exit 0
    fi
    echo '{"loggedIn":false}'
    exit 1
  fi;;
-p)
  if [ "$*" != "-p --verbose --setting-sources local --input-format stream-json --output-format stream-json --no-session-persistence" ]; then
    echo "bad args $*" >> "$FAKE_LOG"; exit 3
  fi
  if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "bad api key in the child environment" >> "$FAKE_LOG"; exit 3
  fi
  read -r line
  if [ "$line" != '{"type":"control_request","request_id":"usage","request":{"subtype":"get_usage","skip_behaviors":true}}' ]; then
    echo "bad stdin $line" >> "$FAKE_LOG"; exit 3
  fi
  n=0
  for f in ./* ./.[!.]*; do [ -e "$f" ] && n=$((n+1)); done
  echo "probe dir=$dir cfgvar=${CLAUDE_CONFIG_DIR+set} cwd=$(pwd) entries=$n" >> "$FAKE_LOG"
  if [ -f "$dir/fake-delay" ]; then read -r d < "$dir/fake-delay"; sleep "$d"; fi
  mode=""
  [ -f "$dir/fake-usage" ] && read -r mode < "$dir/fake-usage"
  case "$mode" in
  hang) sleep 60 & echo $! >> "$FAKE_LOG.pids"; wait; exit 0;;
  fail) exit 1;;
  garbage)
    echo 'not json at all'
    echo '{"type":"control_response","response":{"subtype":"success","request_id":"other","response":{}}}'
    exit 0;;
  error)
    echo '{"type":"control_response","response":{"subtype":"error","request_id":"usage","error":"SECRET-ERROR-TEXT"}}'
    exit 0;;
  esac
  echo '{"type":"system","subtype":"init","session_id":"s","tools":[]}'
  printf '{"type":"control_response","response":{"subtype":"success","request_id":"usage","response":%s}}\n' "$mode"
  exit 0;;
esac
exit 2
`

// udSignedIn is the get_usage reply of a signed-in team account as the research
// doc records it: the two windows, null siblings, and many keys Grid ignores.
const udSignedIn = `{"subscription_type":"team","rate_limits_available":true,"rate_limits":{"five_hour":{"utilization":53,"resets_at":"2026-11-11T14:00:00Z","limit_dollars":null,"used_dollars":null,"remaining_dollars":null,"locked_reason":null},"seven_day":{"utilization":21.5,"resets_at":"2026-11-15T09:30:00Z","locked_reason":null},"seven_day_opus":null,"seven_day_sonnet":null,"model_scoped":[{"display_name":"Opus","utilization":3,"resets_at":"2026-11-15T09:30:00Z"}],"limits":[{"kind":"session","percent":53}],"spend":{"total":null},"extra_usage":null},"session":{"total_cost_usd":0}}`

const udSignedOut = `{"subscription_type":null,"rate_limits_available":false,"rate_limits":null}`

// udMach is a temporary machine with the fake providers on PATH and a clock the
// test moves.
type udMach struct {
	env Env
	bin string
	log string
	now time.Time
}

func udMachine(t *testing.T) *udMach {
	t.Helper()
	sleep, err := exec.LookPath("sleep")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("the fake providers are shell scripts that need sleep")
	}
	env, bin := machine(t)
	for name, body := range map[string]string{"claude": udFakeClaude, "codex": udFakeCodex} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(sleep, filepath.Join(bin, "sleep")); err != nil {
		t.Fatal(err)
	}
	m := &udMach{env: env, bin: bin, log: filepath.Join(t.TempDir(), "fake.log"), now: fixedNow}
	t.Setenv("FAKE_LOG", m.log)
	t.Setenv("ANTHROPIC_API_KEY", "")
	m.env.Now = func() time.Time { return m.now }
	m.env.ProbeRetryWait = time.Millisecond
	return m
}

// at sets the clock d after fixedNow.
func (m *udMach) at(d time.Duration) { m.now = fixedNow.Add(d) }

func (m *udMach) folder(name string) string { return filepath.Join(m.env.Home, name) }

// usage sets what the fake provider answers on a folder: the first line of fake-usage.
func (m *udMach) usage(t *testing.T, dir, body string) {
	t.Helper()
	write(t, filepath.Join(dir, "fake-usage"), body+"\n")
}

func (m *udMach) logLines(prefix string) []string {
	data, _ := os.ReadFile(m.log)
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return out
}

func (m *udMach) probes() int { return len(m.logLines("probe")) }

func (m *udMach) noBad(t *testing.T) {
	t.Helper()
	if bad := m.logLines("bad"); len(bad) != 0 {
		t.Fatalf("the fake provider refused a call: %v", bad)
	}
}

func (m *udMach) windows(t *testing.T, id string) []AccountWindows {
	t.Helper()
	got, err := Windows(m.env, id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func udRow(t *testing.T, list []AccountWindows, id string) AccountWindows {
	t.Helper()
	for _, a := range list {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no row %q in %+v", id, list)
	return AccountWindows{}
}

func udUnix(t *testing.T, iso string) int64 {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatal(err)
	}
	return ts.Unix()
}

func udWant(t *testing.T, got AccountWindows, source string, want ...Window) {
	t.Helper()
	if got.Source != source || len(got.Windows) != len(want) {
		t.Fatalf("want source %s with %d windows: %+v", source, len(want), got)
	}
	for i, w := range want {
		if got.Windows[i] != w {
			t.Fatalf("window %d: got %+v, want %+v (%+v)", i, got.Windows[i], w, got)
		}
	}
}

// Row 1: the windows come from get_usage with no status line and no session.
func TestUdClaudeWindowsFromGetUsageWithoutStatusLine(t *testing.T) {
	m := udMachine(t)
	home, own := m.folder(".claude"), m.folder(".claude-ann")
	m.usage(t, home, udSignedIn)
	m.usage(t, own, strings.Replace(udSignedIn, `"utilization":53`, `"utilization":10`, 1))
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-must-not-reach-the-child")

	list := m.windows(t, "")
	m.noBad(t)
	five, week := udUnix(t, "2026-11-11T14:00:00Z"), udUnix(t, "2026-11-15T09:30:00Z")
	got := udRow(t, list, "claude")
	udWant(t, got, "direct", Window{"5h", 53, five}, Window{"7d", 21.5, week})
	if got.AsOf != fixedNow.Format(time.RFC3339) || got.Note != "" {
		t.Fatalf("asOf is the time of the read and a reading has no note: %+v", got)
	}
	udWant(t, udRow(t, list, "claude-ann"), "direct", Window{"5h", 10, five}, Window{"7d", 21.5, week})

	probes := m.logLines("probe")
	if len(probes) != 2 {
		t.Fatalf("one probe per folder: %v", probes)
	}
	for _, p := range probes {
		if strings.Contains(p, "dir="+home+" ") && !strings.Contains(p, "cfgvar= ") {
			t.Fatalf("CLAUDE_CONFIG_DIR must be unset for the home folder: %s", p)
		}
		if strings.Contains(p, "dir="+own+" ") && !strings.Contains(p, "cfgvar=set") {
			t.Fatalf("CLAUDE_CONFIG_DIR must be set for another folder: %s", p)
		}
		if !strings.Contains(p, "entries=0") || strings.Contains(p, "cwd="+m.env.Home) {
			t.Fatalf("the probe runs in a fresh empty folder, not in the home: %s", p)
		}
	}
	// No status line and no session: nothing of the status line's was written.
	if _, err := os.Stat(snapshotPath(m.env, home)); !os.IsNotExist(err) {
		t.Fatalf("a direct read must not write a status line snapshot: %v", err)
	}
}

// Row 1 (shapes): a signed-out reply, garbage, an error reply and silence give
// source none with a fixed reason line, never a 0.
func TestUdClaudeFailuresGiveAReasonNotZero(t *testing.T) {
	cases := []struct{ name, body, reason string }{
		{"signed out", udSignedOut, "signed out"},
		{"rate_limits null while available", `{"rate_limits_available":true,"rate_limits":null}`, "version"},
		{"no window in the reply", `{"rate_limits_available":true,"rate_limits":{"five_hour":null,"seven_day":null}}`, "version"},
		{"garbage and another request id", "garbage", "version"},
		{"exit 1 and no output", "fail", "version"},
		{"error reply", "error", "error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := udMachine(t)
			dir := m.folder(".claude")
			m.usage(t, dir, c.body)
			got := udRow(t, m.windows(t, ""), "claude")
			if got.Source != "none" || got.Windows == nil || len(got.Windows) != 0 || !strings.Contains(got.Note, c.reason) {
				t.Fatalf("want none with a note naming %q: %+v", c.reason, got)
			}
			if strings.Contains(got.Note, "SECRET") {
				t.Fatalf("the reason is a fixed line, never the provider's text: %q", got.Note)
			}
			m.noBad(t)
		})
	}
}

// Unknown and missing keys are tolerated; a window with a null utilization is
// skipped; an ISO reset with an offset and a fraction becomes Unix seconds; the
// reply's other keys (a token among them) are never kept.
func TestUdClaudeToleratesUnknownAndMissingKeys(t *testing.T) {
	m := udMachine(t)
	dir := m.folder(".claude-ann")
	m.usage(t, dir, `{"subscription_type":"pro","rate_limits_available":true,"access_token":"SECRET-IN-REPLY","rate_limits":{"five_hour":{"utilization":null,"resets_at":"2026-11-11T14:00:00Z"},"seven_day":{"utilization":7,"resets_at":"2026-11-15T10:30:00.5+01:00","brand_new_key":{"x":1}},"future_window":{"utilization":99,"resets_at":"2026-11-12T00:00:00Z"}},"unknown_top":[1,2]}`)
	got := udRow(t, m.windows(t, ""), "claude-ann")
	udWant(t, got, "direct", Window{"7d", 7, udUnix(t, "2026-11-15T09:30:00Z")})

	m.usage(t, dir, `{"rate_limits_available":true,"rate_limits":{"five_hour":{"utilization":0,"resets_at":null}}}`)
	m.at(3 * time.Minute)
	got = udRow(t, m.windows(t, ""), "claude-ann")
	udWant(t, got, "direct", Window{"5h", 0, 0})

	_ = filepath.WalkDir(m.env.GridHome, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			data, _ := os.ReadFile(path)
			if strings.Contains(string(data), "SECRET") {
				t.Errorf("%s keeps a key of the reply: %s", path, data)
			}
		}
		return nil
	})
}

// A default account's sessions run on ~/.claude, so its reading is that folder's.
func TestUdDefaultAccountReadsTheHomeFolderDirectly(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	login(t, home, "ann@example.com")
	status(t, m.env, true)
	m.usage(t, home, udSignedIn)
	got := m.windows(t, "claude-ann")
	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	udWant(t, got[0], "direct", Window{"5h", 53, udUnix(t, "2026-11-11T14:00:00Z")}, Window{"7d", 21.5, udUnix(t, "2026-11-15T09:30:00Z")})
}

// Row 3: the order of sources.
func TestUdOrderDirectBeatsAnOlderNote(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	m.usage(t, home, udSignedIn)
	m.at(-time.Hour)
	if err := Note(m.env, strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":77,"resets_at":`+itoa(fixedNow.Add(time.Hour).Unix())+`}}}`), home); err != nil {
		t.Fatal(err)
	}
	m.at(0)
	udWant(t, udRow(t, m.windows(t, ""), "claude"), "direct", Window{"5h", 53, udUnix(t, "2026-11-11T14:00:00Z")}, Window{"7d", 21.5, udUnix(t, "2026-11-15T09:30:00Z")})
}

func TestUdOrderNewerNoteBeatsDirect(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	m.usage(t, home, udSignedIn)
	if got := udRow(t, m.windows(t, ""), "claude"); got.Source != "direct" {
		t.Fatalf("first read: %+v", got)
	}
	m.at(30 * time.Second)
	reset := fixedNow.Add(time.Hour).Unix()
	if err := Note(m.env, strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":77,"resets_at":`+itoa(reset)+`}}}`), home); err != nil {
		t.Fatal(err)
	}
	m.at(time.Minute)
	got := udRow(t, m.windows(t, ""), "claude")
	udWant(t, got, "statusline", Window{"5h", 77, reset})
	if m.probes() != 1 {
		t.Fatalf("inside the gap no second probe runs: %d", m.probes())
	}
}

func TestUdOrderNoteThenNoneWhenTheDirectReadFailed(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	m.usage(t, home, "fail")
	got := udRow(t, m.windows(t, ""), "claude")
	if got.Source != "none" || len(got.Windows) != 0 || got.Note == "" {
		t.Fatalf("no source: none with the reason: %+v", got)
	}
	reset := fixedNow.Add(time.Hour).Unix()
	if err := Note(m.env, strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":12,"resets_at":`+itoa(reset)+`}}}`), home); err != nil {
		t.Fatal(err)
	}
	udWant(t, udRow(t, m.windows(t, ""), "claude"), "statusline", Window{"5h", 12, reset})
}

// Row 3: a failed read keeps the last good with its asOf; the cache keeps the
// 2-minute gap and the 5-minute back-off.
func TestUdCacheGapBackOffAndLastGood(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	five, week := udUnix(t, "2026-11-11T14:00:00Z"), udUnix(t, "2026-11-15T09:30:00Z")
	good := []Window{{"5h", 53, five}, {"7d", 21.5, week}}
	m.usage(t, home, udSignedIn)

	udWant(t, udRow(t, m.windows(t, ""), "claude"), "direct", good...)
	m.at(90 * time.Second)
	udWant(t, udRow(t, m.windows(t, ""), "claude"), "direct", good...)
	if m.probes() != 1 {
		t.Fatalf("inside 2 minutes the cache answers: %d probes", m.probes())
	}

	m.usage(t, home, "fail")
	m.at(3 * time.Minute)
	got := udRow(t, m.windows(t, ""), "claude")
	udWant(t, got, "direct", good...)
	if m.probes() != 2 || got.AsOf != fixedNow.Format(time.RFC3339) {
		t.Fatalf("a failure keeps the last good with its own asOf: %d probes, %+v", m.probes(), got)
	}

	m.at(6 * time.Minute) // 3 minutes after the failure
	udWant(t, udRow(t, m.windows(t, ""), "claude"), "direct", good...)
	if m.probes() != 2 {
		t.Fatalf("5 minutes of back-off after a failure: %d probes", m.probes())
	}

	m.usage(t, home, strings.Replace(udSignedIn, `"utilization":53`, `"utilization":60`, 1))
	m.at(9 * time.Minute) // 6 minutes after the failure
	got = udRow(t, m.windows(t, ""), "claude")
	udWant(t, got, "direct", Window{"5h", 60, five}, Window{"7d", 21.5, week})
	if m.probes() != 3 || got.AsOf != fixedNow.Add(9*time.Minute).Format(time.RFC3339) {
		t.Fatalf("after the back-off a new read replaces the last good: %d probes, %+v", m.probes(), got)
	}
}

// A window that reset after the reading started over: used 0 for a direct reading too.
func TestUdResetPassedZeroesADirectWindow(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	m.usage(t, home, udSignedIn)
	m.windows(t, "")
	m.usage(t, home, "fail")
	m.at(3 * time.Hour) // past the 5h reset (+2h), before the 7d reset
	got := udRow(t, m.windows(t, ""), "claude")
	udWant(t, got, "direct", Window{"5h", 0, udUnix(t, "2026-11-11T14:00:00Z")}, Window{"7d", 21.5, udUnix(t, "2026-11-15T09:30:00Z")})
}

// Row 4: a probe that hangs is killed at the timeout, with its whole process
// group, and does not hold up the other folders.
func TestUdHangingProbeIsKilledAndDoesNotBlockOtherFolders(t *testing.T) {
	m := udMachine(t)
	m.env.ProbeTimeout = time.Second
	hangA, hangB, fine := m.folder(".claude-a"), m.folder(".claude-b"), m.folder(".claude-c")
	m.usage(t, hangA, "hang")
	m.usage(t, hangB, "hang")
	m.usage(t, fine, udSignedIn)

	start := time.Now()
	list := m.windows(t, "")
	elapsed := time.Since(start)
	if elapsed < time.Second || elapsed > 1800*time.Millisecond {
		t.Fatalf("two hung folders cost one timeout when read side by side, not two: %v", elapsed)
	}
	for _, id := range []string{"claude-a", "claude-b"} {
		got := udRow(t, list, id)
		if got.Source != "none" || !strings.Contains(got.Note, "timed out") {
			t.Fatalf("%s: %+v", id, got)
		}
	}
	if got := udRow(t, list, "claude-c"); got.Source != "direct" || len(got.Windows) != 2 {
		t.Fatalf("the folder that answers is not held up: %+v", got)
	}

	pids := strings.Fields(readFileOrEmpty(m.log + ".pids"))
	if len(pids) != 2 {
		t.Fatalf("each hung probe started a child of its own: %v", pids)
	}
	for _, p := range pids {
		var pid int
		if err := json.Unmarshal([]byte(p), &pid); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for udAlive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d of the hung probe is still running: the whole group must be killed", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func readFileOrEmpty(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func udAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// One probe per folder at a time: two readers of one folder start one probe,
// and the second takes the first's answer.
func TestUdOneProbePerFolderAtATime(t *testing.T) {
	m := udMachine(t)
	home := m.folder(".claude")
	m.usage(t, home, udSignedIn)
	write(t, filepath.Join(home, "fake-delay"), "0.4\n")

	var wg sync.WaitGroup
	results := make([]AccountWindows, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := Windows(m.env, "")
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = udRow(t, list, "claude")
		}()
	}
	wg.Wait()
	if m.probes() != 1 {
		t.Fatalf("two readers, one probe: %d", m.probes())
	}
	for i, r := range results {
		if r.Source != "direct" || len(r.Windows) != 2 {
			t.Fatalf("reader %d did not get the reading: %+v", i, r)
		}
	}
}

// --id limits the probes to that account.
func TestUdIdProbesOnlyThatAccount(t *testing.T) {
	m := udMachine(t)
	a, b := m.folder(".claude-a"), m.folder(".claude-b")
	m.usage(t, a, udSignedIn)
	m.usage(t, b, udSignedIn)
	got := m.windows(t, "claude-a")
	if len(got) != 1 || got[0].Source != "direct" || m.probes() != 1 || !strings.Contains(m.logLines("probe")[0], "dir="+a) {
		t.Fatalf("only claude-a is read: %+v, probes %v", got, m.logLines("probe"))
	}
}

// A provider that is not installed is a reason line, and no probe is recorded.
func TestUdNoClaudeOnPathSaysSo(t *testing.T) {
	env, _ := machine(t)
	write(t, filepath.Join(env.Home, ".claude", "keep"), "")
	got := udRow(t, mustWindows(t, env), "claude")
	if got.Source != "none" || !strings.Contains(got.Note, "not found") {
		t.Fatalf("%+v", got)
	}
}

func mustWindows(t *testing.T, env Env) []AccountWindows {
	t.Helper()
	list, err := Windows(env, "")
	if err != nil {
		t.Fatal(err)
	}
	return list
}
