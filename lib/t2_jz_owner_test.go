package lib

import (
	"bytes"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// jzOwResult is what one Run call answered.
type jzOwResult struct {
	code        int
	out, errout string
}

// jzOwEnviron is a call's whole environment: the fixture's HOME and what the
// test adds, never GRID_WHO, GRID_ACTOR or GRID_MEMBER.
func jzOwEnviron(h home, extra ...string) []string {
	return append([]string{"HOME=" + filepath.Join(h.root, "home")}, extra...)
}

// jzOwNew makes one `task new` call in the fixture home with that Environ alone.
func jzOwNew(h home, title string, environ []string) jzOwResult {
	var out, errb bytes.Buffer
	code := Run(Call{Args: []string{"task", "new", title, "--project", "example", "--json"},
		GridHome: h.grid, Dir: h.folder, Environ: environ, Stdout: &out, Stderr: &errb})
	return jzOwResult{code: code, out: out.String(), errout: errb.String()}
}

// jzOwOwner reads the owner a new task recorded in its task.md front matter.
func jzOwOwner(t *testing.T, r jzOwResult) string {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("task new exited %d: %s", r.code, r.errout)
	}
	var made struct{ ID, Path string }
	if err := json.Unmarshal([]byte(r.out), &made); err != nil || made.Path == "" {
		t.Fatalf("task new output: %v %s", err, r.out)
	}
	text, err := os.ReadFile(filepath.Join(made.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(text), "\n") {
		if name, ok := strings.CutPrefix(line, "owner: "); ok {
			return name
		}
	}
	t.Fatalf("task.md has no owner line: %s", text)
	return ""
}

// jzOwHostAccount is the account the host process runs as.
func jzOwHostAccount() string {
	if u, err := user.Current(); err == nil {
		return strings.TrimSpace(u.Username)
	}
	return strings.TrimSpace(os.Getenv("USER"))
}

// Two calls for two people in one process, started together, each record their own USER.
func TestJzOwTwoCallsTwoOwners(t *testing.T) {
	people := []struct {
		name string
		h    home
		env  []string
		res  jzOwResult
	}{
		{name: "ana", h: newHome(t, "Fixture for ana")},
		{name: "ben", h: newHome(t, "Fixture for ben")},
	}
	if people[0].h.grid == people[1].h.grid {
		t.Fatalf("the two calls share the Grid home %s", people[0].h.grid)
	}
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	for i := range people {
		p := &people[i]
		p.env = jzOwEnviron(p.h, "USER="+p.name)
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			p.res = jzOwNew(p.h, "Owner of "+p.name, p.env)
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	for _, p := range people {
		if got := jzOwOwner(t, p.res); got != p.name {
			t.Errorf("the call with USER=%s recorded owner %q (the host account is %q)", p.name, got, jzOwHostAccount())
		}
	}
}

// USERNAME names the owner when there is no USER (the Windows name), and a call
// with neither names "owner": the host's account is never read.
func TestJzOwUsernameAndNobody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		environ []string
		want    string
	}{
		{"USERNAME only", []string{"USERNAME=ben"}, "ben"},
		{"USER wins over USERNAME", []string{"USERNAME=ben", "USER=ana"}, "ana"},
		{"no USER and no USERNAME", nil, "owner"},
		{"blank USER and USERNAME", []string{"USER=  ", "USERNAME="}, "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHome(t, "Fixture")
			got := jzOwOwner(t, jzOwNew(h, "Owner from the call", jzOwEnviron(h, tc.environ...)))
			if got != tc.want {
				t.Fatalf("owner %q, want %q (the host account is %q)", got, tc.want, jzOwHostAccount())
			}
		})
	}
}
