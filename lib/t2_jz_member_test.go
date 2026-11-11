package lib

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// A Feed write with no GRID_MEMBER and no GRID_ACTOR in the Call finds its member from the
// Call's own git config and USER, USERNAME; the host process's are not read.

var jzMbAuthor = regexp.MustCompile(`"author":\{"id":"([^"]*)"`)

// jzMbHostEnv makes the host process answer as someone else than the Call: its global git
// config names host@example.com and its account is hostuser. It runs after newHome, whose
// testgit.Isolate sets the same variables to the null device.
func jzMbHostEnv(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", jzMbGitConfig(t, dir, "host.gitconfig", "host@example.com"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("USER", "hostuser")
	t.Setenv("USERNAME", "hostuser")
}

// jzMbGitConfig writes a global git config file; an empty email writes one with no user.email.
func jzMbGitConfig(t *testing.T, dir, name, email string) string {
	t.Helper()
	text := "[user]\n\tname = Example\n"
	if email != "" {
		text = "[user]\n\temail = " + email + "\n"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// jzMbPlain is a folder outside any git repo, the Dir of the Call.
func jzMbPlain(t *testing.T, h home) string {
	t.Helper()
	dir := filepath.Join(h.root, "plain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// jzMbEnviron is a Call's whole environment with no member: no GRID_MEMBER, no GRID_ACTOR.
func jzMbEnviron(h home, extra ...string) []string {
	return append([]string{"HOME=" + filepath.Join(h.root, "home"), "GRID_ROLE=owner", "PATH=" + os.Getenv("PATH")}, extra...)
}

// jzMbFeed is every file under the task's feed folder, by path.
func jzMbFeed(t *testing.T, h home) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(h.task, "feed"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		files[path] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// jzMbNewAuthors is the author of each record written between the two snapshots.
func jzMbNewAuthors(before, after map[string]string) []string {
	var ids []string
	for path, text := range after {
		old := before[path]
		if text == old || !strings.HasSuffix(path, ".jsonl") {
			continue
		}
		if strings.HasPrefix(text, old) {
			text = text[len(old):]
		}
		for _, m := range jzMbAuthor.FindAllStringSubmatch(text, -1) {
			ids = append(ids, m[1])
		}
	}
	return ids
}

// jzMbChanged lists the files that were added, removed or changed between the two snapshots.
func jzMbChanged(before, after map[string]string) []string {
	var paths []string
	for path, text := range before {
		if got, ok := after[path]; !ok || got != text {
			paths = append(paths, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			paths = append(paths, path)
		}
	}
	return paths
}

// jzMbPost posts one message through lib.Run in dir with the given environment and returns
// the exit code, Stderr and the authors of the records it wrote.
func jzMbPost(t *testing.T, h home, dir string, environ []string) (int, string, []string) {
	t.Helper()
	id := haFdTaskID(t, h)
	before := jzMbFeed(t, h)
	code, _, errs := h.run(dir, environ, "feed", "post", id, "--type", "post", "--body", `{"ref":"task","kind":"message","text":"jz"}`, "--json")
	return code, errs, jzMbNewAuthors(before, jzMbFeed(t, h))
}

func jzMbWant(t *testing.T, code int, errs string, authors []string, want string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("post: exit %d, want 0: %s", code, errs)
	}
	if len(authors) != 1 || authors[0] != want {
		t.Errorf("author %q, want [%s]", authors, want)
	}
}

// Row 1: the Call's GIT_CONFIG_GLOBAL names the member, not the host's.
func TestJzMbEmailFromCallGitConfig(t *testing.T) {
	h := newHome(t, "Member from the call's git config")
	jzMbHostEnv(t, h.root)
	call := jzMbGitConfig(t, h.root, "call.gitconfig", "call@example.com")
	code, errs, authors := jzMbPost(t, h, jzMbPlain(t, h), jzMbEnviron(h, "GIT_CONFIG_GLOBAL="+call, "GIT_CONFIG_NOSYSTEM=1"))
	jzMbWant(t, code, errs, authors, "call@example.com")
}

// Row 2: no email in the Call's git config, so the Call's USER then USERNAME name the member.
func TestJzMbUserFromCall(t *testing.T) {
	for _, c := range []struct {
		name  string
		extra []string
		want  string
	}{
		{"USER", []string{"USER=ana"}, "ana"},
		{"USERNAME", []string{"USERNAME=bob"}, "bob"},
		{"USER wins over USERNAME", []string{"USERNAME=bob", "USER=ana"}, "ana"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHome(t, "Member from the call's USER")
			jzMbHostEnv(t, h.root)
			call := jzMbGitConfig(t, h.root, "call.gitconfig", "")
			environ := jzMbEnviron(h, append([]string{"GIT_CONFIG_GLOBAL=" + call, "GIT_CONFIG_NOSYSTEM=1"}, c.extra...)...)
			code, errs, authors := jzMbPost(t, h, jzMbPlain(t, h), environ)
			jzMbWant(t, code, errs, authors, c.want)
		})
	}
}

// Row 3: the Call names nobody, and the host's account is not written in its place.
func TestJzMbNoIdentityRefused(t *testing.T) {
	for _, hostEmail := range []bool{true, false} {
		name := "host git config has an email"
		if !hostEmail {
			name = "host git config has no email, so the host account is the one at stake"
		}
		t.Run(name, func(t *testing.T) {
			h := newHome(t, "No member in the call")
			jzMbHostEnv(t, h.root)
			if !hostEmail {
				t.Setenv("GIT_CONFIG_GLOBAL", jzMbGitConfig(t, h.root, "host-none.gitconfig", ""))
			}
			call := jzMbGitConfig(t, h.root, "call.gitconfig", "")
			id := haFdTaskID(t, h)
			before := jzMbFeed(t, h)
			code, _, errs := h.run(jzMbPlain(t, h), jzMbEnviron(h, "GIT_CONFIG_GLOBAL="+call, "GIT_CONFIG_NOSYSTEM=1"),
				"feed", "post", id, "--type", "post", "--body", `{"ref":"task","kind":"message","text":"jz"}`, "--json")
			if code == 0 {
				t.Errorf("exit 0, want non-zero")
			}
			if !strings.Contains(errs, "cannot find member identity; set GRID_MEMBER") {
				t.Errorf("stderr %q does not hold the refusal", errs)
			}
			if changed := jzMbChanged(before, jzMbFeed(t, h)); len(changed) != 0 {
				t.Errorf("the Feed changed: %q", changed)
			}
		})
	}
}

// A Call with no environment at all gives git an empty one, not the host's.
func TestJzMbEmptyEnvironIsNotTheHosts(t *testing.T) {
	h := newHome(t, "Empty environment")
	jzMbHostEnv(t, h.root)
	code, errs, authors := jzMbPost(t, h, jzMbPlain(t, h), nil)
	for _, a := range authors {
		if a == "host@example.com" || a == "hostuser" {
			t.Errorf("the host's identity %q was written (exit %d, %s)", a, code, errs)
		}
	}
	if code == 0 && len(authors) == 0 {
		t.Errorf("exit 0 and no record written")
	}
	if code != 0 && !strings.Contains(errs, "cannot find member identity; set GRID_MEMBER") {
		t.Errorf("exit %d, stderr %q does not hold the refusal", code, errs)
	}
}

// Two Calls of one host process name two members, each from its own git config.
func TestJzMbTwoCallsTwoMembers(t *testing.T) {
	type side struct {
		h       home
		want    string
		code    int
		errs    string
		authors []string
	}
	sides := []*side{{want: "ana@example.com"}, {want: "bob@example.com"}}
	envs := make([][]string, len(sides))
	dirs := make([]string, len(sides))
	ids := make([]string, len(sides))
	befores := make([]map[string]string, len(sides))
	for i, s := range sides {
		s.h = newHome(t, "Two members "+s.want)
		call := jzMbGitConfig(t, s.h.root, "call.gitconfig", s.want)
		envs[i] = jzMbEnviron(s.h, "GIT_CONFIG_GLOBAL="+call, "GIT_CONFIG_NOSYSTEM=1")
		dirs[i], ids[i] = jzMbPlain(t, s.h), haFdTaskID(t, s.h)
		befores[i] = jzMbFeed(t, s.h)
	}
	// The second newHome's testgit.Isolate reset the host's variables, so the host is set after both.
	jzMbHostEnv(t, sides[0].h.root)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, s := range sides {
		wg.Add(1)
		go func(s *side, dir, id string, environ []string) {
			defer wg.Done()
			<-start
			s.code, _, s.errs = s.h.run(dir, environ, "feed", "post", id, "--type", "post", "--body", `{"ref":"task","kind":"message","text":"jz"}`, "--json")
		}(s, dirs[i], ids[i], envs[i])
	}
	close(start)
	wg.Wait()
	for i, s := range sides {
		jzMbWant(t, s.code, s.errs, jzMbNewAuthors(befores[i], jzMbFeed(t, s.h)), s.want)
	}
}

// The binary (no Call) keeps its own reads: git sees the process's environment and the
// account is the system's.
func TestJzMbBinaryKeepsItsReads(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Binary member")
	jzMbHostEnv(t, h.root)
	id := haFdTaskID(t, h)
	post := []string{"feed", "post", id, "--type", "post", "--body", `{"ref":"task","kind":"message","text":"jz"}`, "--json"}
	binEnv := func(extra ...string) []string {
		return append(jzMbEnviron(h, "GRID_HOME="+h.grid, "GIT_CONFIG_NOSYSTEM=1"), extra...)
	}

	call := jzMbGitConfig(t, h.root, "call.gitconfig", "call@example.com")
	before := jzMbFeed(t, h)
	if code, _, errs := binary(bin, jzMbPlain(t, h), binEnv("GIT_CONFIG_GLOBAL="+call), post...); code != 0 {
		t.Fatalf("binary post: exit %d: %s", code, errs)
	}
	if got := jzMbNewAuthors(before, jzMbFeed(t, h)); len(got) != 1 || got[0] != "call@example.com" {
		t.Errorf("binary author %q, want [call@example.com]", got)
	}

	account, err := user.Current()
	if err != nil {
		t.Skipf("no system account: %v", err)
	}
	none := jzMbGitConfig(t, h.root, "none.gitconfig", "")
	before = jzMbFeed(t, h)
	if code, _, errs := binary(bin, jzMbPlain(t, h), binEnv("GIT_CONFIG_GLOBAL="+none, "USER=envuser"), post...); code != 0 {
		t.Fatalf("binary post with no email: exit %d: %s", code, errs)
	}
	if got := jzMbNewAuthors(before, jzMbFeed(t, h)); len(got) != 1 || got[0] != account.Username {
		t.Errorf("binary author %q, want [%s]", got, account.Username)
	}
}
