package mcp

import (
	"strings"
	"sync"
	"testing"
)

// jzMcProcessVersion is what the process root's manifest says in every test
// here. No row may ever show it: each answer must come from the server's Getenv
// (or, when that is nil, from the process root on purpose).
const jzMcProcessVersion = "9.9.9"

// jzMcGetenv is a fake environment that answers CLAUDE_PLUGIN_ROOT with root and
// every other key with "".
func jzMcGetenv(root string) func(string) string {
	return func(key string) string {
		if key == "CLAUDE_PLUGIN_ROOT" {
			return root
		}
		return ""
	}
}

// jzMcNotice is the stale line stale.go writes for a disk and a running version.
func jzMcNotice(disk, running string) string {
	return "Grid plugin on disk is " + disk + ", this session runs " + running + "; restart the session to load it."
}

// jzMcProcessRoot freezes the stale clock and points the process at a root whose
// manifest says 9.9.9. Reusing withClock, fakeRoot and callTypes from
// stale_test.go keeps one manifest writer.
func jzMcProcessRoot(t *testing.T) {
	t.Helper()
	withClock(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", fakeRoot(t, jzMcProcessVersion))
}

// jzMcStaleLines returns the texts that are a stale-plugin line.
func jzMcStaleLines(texts []string) []string {
	var lines []string
	for _, txt := range texts {
		if strings.Contains(txt, "restart the session") {
			lines = append(lines, txt)
		}
	}
	return lines
}

// jzMcOnlyNotice checks texts hold exactly the one wanted stale line, last.
func jzMcOnlyNotice(t *testing.T, texts []string, want string) {
	t.Helper()
	if lines := jzMcStaleLines(texts); len(lines) != 1 || lines[0] != want || texts[len(texts)-1] != want {
		t.Fatalf("want one notice %q last, got %d texts with stale lines %q", want, len(texts), lines)
	}
}

// Row 1: the command's environment answers the root, the process root is not read.
func TestJzMcGetenvRootBeatsTheProcessRoot(t *testing.T) {
	jzMcProcessRoot(t)
	other := fakeRoot(t, "1.2.3")
	texts := callTypes(t, Server{Version: "1.0.0", Getenv: jzMcGetenv(other)})
	jzMcOnlyNotice(t, texts, "Grid plugin on disk is 1.2.3, this session runs 1.0.0; restart the session to load it.")
	for _, line := range jzMcStaleLines(texts) {
		if strings.Contains(line, jzMcProcessVersion) {
			t.Fatalf("the process root leaked into the answer: %q", line)
		}
	}
}

// Row 2: a root at the running version, or no root at all, adds no notice, even
// though the process root is stale.
func TestJzMcGetenvSameVersionOrEmptyAddsNoNotice(t *testing.T) {
	jzMcProcessRoot(t)
	cases := []struct{ name, root string }{
		{"same version", fakeRoot(t, "1.0.0")},
		{"empty root", ""},
	}
	for _, c := range cases {
		if lines := jzMcStaleLines(callTypes(t, Server{Version: "1.0.0", Getenv: jzMcGetenv(c.root)})); len(lines) != 0 {
			t.Fatalf("%s: notice added: %q", c.name, lines)
		}
	}
}

// Row 3: no Getenv reads the process, as before.
func TestJzMcNilGetenvReadsTheProcessRoot(t *testing.T) {
	jzMcProcessRoot(t)
	texts := callTypes(t, Server{Version: "1.0.0"})
	jzMcOnlyNotice(t, texts, jzMcNotice(jzMcProcessVersion, "1.0.0"))
}

// The cache is keyed by root: servers with different roots, called one after the
// other inside one staleEvery window, each get the answer for their own root.
func TestJzMcServersWithDifferentRootsDoNotShareAnAnswer(t *testing.T) {
	jzMcProcessRoot(t)
	a := Server{Version: "1.0.0", Getenv: jzMcGetenv(fakeRoot(t, "1.2.3"))}
	b := Server{Version: "1.0.0", Getenv: jzMcGetenv(fakeRoot(t, "3.4.5"))}
	process := Server{Version: "1.0.0"}
	for round := 0; round < 2; round++ {
		jzMcOnlyNotice(t, callTypes(t, a), jzMcNotice("1.2.3", "1.0.0"))
		jzMcOnlyNotice(t, callTypes(t, b), jzMcNotice("3.4.5", "1.0.0"))
		jzMcOnlyNotice(t, callTypes(t, process), jzMcNotice(jzMcProcessVersion, "1.0.0"))
	}
}

// Calls from servers with different roots, released together, never see each
// other's answer. Goroutines only record; the test fails after they are joined.
func TestJzMcConcurrentServersKeepTheirOwnRoot(t *testing.T) {
	jzMcProcessRoot(t)
	servers := []struct {
		s    Server
		want string
	}{
		{Server{Version: "1.0.0", Getenv: jzMcGetenv(fakeRoot(t, "1.2.3"))}, jzMcNotice("1.2.3", "1.0.0")},
		{Server{Version: "1.0.0", Getenv: jzMcGetenv(fakeRoot(t, "3.4.5"))}, jzMcNotice("3.4.5", "1.0.0")},
		{Server{Version: "1.0.0"}, jzMcNotice(jzMcProcessVersion, "1.0.0")},
	}
	const workers, calls = 12, 100
	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
		wrong [workers]string
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := servers[w%len(servers)]
			<-start
			for i := 0; i < calls && wrong[w] == ""; i++ {
				got := c.s.withStaleNotice(Result{}).Content
				if len(got) != 1 || got[0].Text != c.want {
					wrong[w] = "worker " + string(rune('A'+w)) + " wanted " + c.want + ", got " + jzMcContentTexts(got)
				}
			}
		}(w)
	}
	close(start)
	wg.Wait()
	for _, msg := range wrong {
		if msg != "" {
			t.Fatal(msg)
		}
	}
}

func jzMcContentTexts(cs []Content) string {
	var texts []string
	for _, c := range cs {
		texts = append(texts, c.Text)
	}
	return strings.Join(texts, " | ")
}
