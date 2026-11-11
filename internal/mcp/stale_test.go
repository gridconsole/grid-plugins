package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeRoot(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	setDisk(t, root, version)
	return root
}

func setDisk(t *testing.T, root, version string) {
	t.Helper()
	body := `{"name":"grid","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// callTypes runs feed_types and returns the content texts.
func callTypes(t *testing.T, s Server) []string {
	t.Helper()
	r := result(t, last(t, exchange(t, s, prelude+request("feed_types", `{}`))))
	var texts []string
	for _, c := range r.Content {
		texts = append(texts, c.Text)
	}
	return texts
}

func withClock(t *testing.T) *time.Time {
	t.Helper()
	now := time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC)
	old := staleNow
	staleNow = func() time.Time { return now }
	t.Cleanup(func() { staleNow = old })
	return &now
}

const notice = "Grid plugin on disk is 2.0.0, this session runs test; restart the session to load it."

func TestStaleNoticeAppearsAndClears(t *testing.T) {
	now := withClock(t)
	root := fakeRoot(t, "2.0.0")
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	s := Server{Version: "test"}
	texts := callTypes(t, s)
	if got := texts[len(texts)-1]; got != notice {
		t.Fatalf("no notice: %q", texts)
	}
	setDisk(t, root, "test")
	*now = now.Add(61 * time.Second)
	for _, txt := range callTypes(t, s) {
		if strings.Contains(txt, "restart the session") {
			t.Fatalf("notice stayed: %q", txt)
		}
	}
}

func TestStaleNoticeReadAtMostOncePerMinute(t *testing.T) {
	now := withClock(t)
	root := fakeRoot(t, "2.0.0")
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	s := Server{Version: "test"}
	callTypes(t, s)
	setDisk(t, root, "test")
	*now = now.Add(30 * time.Second)
	texts := callTypes(t, s)
	if texts[len(texts)-1] != notice {
		t.Fatalf("file was re-read inside 60s: %q", texts)
	}
}

func TestStaleNoticeSilentCases(t *testing.T) {
	withClock(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	base := callTypes(t, Server{Version: "test"})
	if len(base) != 1 {
		t.Fatalf("unset root changed the result: %q", base)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", t.TempDir()) // no plugin.json
	if got := callTypes(t, Server{Version: "test"}); strings.Join(got, "|") != strings.Join(base, "|") {
		t.Fatalf("unreadable file changed the result: %q", got)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", fakeRoot(t, "test"))
	if got := callTypes(t, Server{Version: "test"}); strings.Join(got, "|") != strings.Join(base, "|") {
		t.Fatalf("same version changed the result: %q", got)
	}
}

func TestInitializeUnchangedByStaleRoot(t *testing.T) {
	withClock(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", fakeRoot(t, "2.0.0"))
	raw := string(exchange(t, Server{Version: "test"}, initialize+"\n"))
	if !strings.Contains(raw, `"serverInfo":{"name":"grid-feed","version":"test"}`) || strings.Contains(raw, "2.0.0") {
		t.Fatalf("initialize changed: %s", raw)
	}
}
