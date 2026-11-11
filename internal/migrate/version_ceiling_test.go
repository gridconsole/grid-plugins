package migrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// A source folder's own VERSION is read by sourceVersion: the other file, in the
// destination home, is read by versionAt and is covered by TestVersionGuards.
func TestSourceVersionChecks(t *testing.T) {
	current := strconv.Itoa(gridhome.FormatVersion)
	tests := []struct {
		name    string
		version string // text of the VERSION file; "" writes none
		dir     bool   // make VERSION a folder instead
		want    string // the exact error, or "" for a plan
	}{
		{name: "not a number", version: "abc\n", want: "invalid Grid VERSION"},
		{name: "empty", version: "\n", want: "invalid Grid VERSION"},
		{name: "negative", version: "-1\n", want: "invalid Grid VERSION"},
		{name: "newer format", version: strconv.Itoa(gridhome.FormatVersion+1) + "\n", want: gridhome.FormatProblem(gridhome.FormatVersion + 1)},
		{name: "a folder", dir: true, want: "VERSION must be a regular file"},
		{name: "current", version: current + "\n", want: "source is already a current Grid home"},
		{name: "current without a newline", version: current, want: "source is already a current Grid home"},
		{name: "no VERSION"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, home := fixture(t)
			switch {
			case tc.dir:
				if err := os.MkdirAll(filepath.Join(from, "VERSION"), 0700); err != nil {
					t.Fatal(err)
				}
			case tc.version != "":
				put(t, filepath.Join(from, "VERSION"), tc.version)
			}
			p, err := Scan(from, home)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("a source with no VERSION was refused: %v", err)
				}
				if p.Current {
					t.Fatal("a source with no VERSION was taken for a current home")
				}
				if len(p.Actions) == 0 {
					t.Fatal("a source with no VERSION gave an empty plan")
				}
				return
			}
			if err == nil {
				t.Fatalf("Scan accepted VERSION %q", tc.version)
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
			absent(t, home)
		})
	}
	t.Run("too large to read", func(t *testing.T) {
		from, home := fixture(t)
		put(t, filepath.Join(from, "VERSION"), strings.Repeat("1", maxRecord+1))
		_, err := Scan(from, home)
		if err == nil || !strings.Contains(err.Error(), "records over") {
			t.Fatalf("error = %v, want the size refusal of readSmall", err)
		}
	})
}

// The ceiling is where the walk up from a project folder to its git repo stops. The
// repo here sits above the ceiling, so a project below the ceiling is not in it.
func TestCeilingStopsTheRepoWalk(t *testing.T) {
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	repo := filepath.Join(root, "repo")
	sub := filepath.Join(repo, "sub")
	proj := filepath.Join(sub, "proj")
	if err := os.MkdirAll(proj, 0700); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, repo, "https://example.com/team/ceil.git", true)
	from, home := filepath.Join(root, "old"), filepath.Join(root, "new")
	put(t, filepath.Join(from, "projects", "proj", "project.md"), "---\nfolder: "+proj+"\n---\nbody\n")

	// found checks the repo recorded for the project: none, or the one at repo.
	found := func(t *testing.T, opts ...Option) {
		t.Helper()
		rec := parseProject(t, scan(t, from, home, opts...), "proj")
		if len(rec.Folders) != 1 || rec.Folders[0].Path != proj {
			t.Fatalf("the project folder changed: %+v", rec.Folders)
		}
		if len(rec.Repos) != 1 || filepath.Join(proj, filepath.FromSlash(rec.Repos[0].Path)) != repo {
			t.Fatalf("repos %+v, want the repo at %s", rec.Repos, repo)
		}
		if rec.Repos[0].Remote != "example.com/team/ceil" {
			t.Fatalf("repo remote %q", rec.Repos[0].Remote)
		}
	}
	notFound := func(t *testing.T, opts ...Option) {
		t.Helper()
		rec := parseProject(t, scan(t, from, home, opts...), "proj")
		if len(rec.Folders) != 1 || rec.Folders[0].Path != proj {
			t.Fatalf("the project folder changed: %+v", rec.Folders)
		}
		if len(rec.Repos) != 0 || len(rec.Keys) != 0 {
			t.Fatalf("the project was mapped to a repo above the ceiling: repos %+v keys %+v", rec.Repos, rec.Keys)
		}
	}

	t.Run("no ceiling reaches the repo above", func(t *testing.T) { found(t) })
	t.Run("ceiling below the repo", func(t *testing.T) { notFound(t, WithCeiling(sub)) })
	t.Run("ceiling at the project folder", func(t *testing.T) { notFound(t, WithCeiling(proj)) })
	t.Run("ceiling at the repo still finds it", func(t *testing.T) { found(t, WithCeiling(repo)) })
	t.Run("ceiling above the repo still finds it", func(t *testing.T) { found(t, WithCeiling(root)) })
	t.Run("ceiling spelled through a symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a symlink needs a privilege on Windows")
		}
		alias := filepath.Join(gridhome.Real(t.TempDir()), "ceiling")
		if err := os.Symlink(sub, alias); err != nil {
			t.Skip(err)
		}
		// Scan resolves the ceiling; the walk compares real paths.
		notFound(t, WithCeiling(alias))
	})
}

func TestRestoredFormat(t *testing.T) {
	t.Run("no VERSION", func(t *testing.T) {
		got, err := RestoredFormat(t.TempDir())
		if err != nil || got != "unversioned" {
			t.Fatalf("RestoredFormat = %q, %v; want unversioned", got, err)
		}
	})
	t.Run("a home that does not exist", func(t *testing.T) {
		got, err := RestoredFormat(filepath.Join(t.TempDir(), "missing"))
		if err != nil || got != "unversioned" {
			t.Fatalf("RestoredFormat = %q, %v; want unversioned", got, err)
		}
	})
	t.Run("trimmed text", func(t *testing.T) {
		home := t.TempDir()
		put(t, filepath.Join(home, "VERSION"), "2\n")
		got, err := RestoredFormat(home)
		if err != nil || got != "2" {
			t.Fatalf("RestoredFormat = %q, %v; want 2", got, err)
		}
	})
	t.Run("text that is not a number is returned as it is", func(t *testing.T) {
		home := t.TempDir()
		put(t, filepath.Join(home, "VERSION"), " draft \r\n")
		got, err := RestoredFormat(home)
		if err != nil || got != "draft" {
			t.Fatalf("RestoredFormat = %q, %v; want draft", got, err)
		}
	})
	t.Run("a folder", func(t *testing.T) {
		home := t.TempDir()
		if err := os.Mkdir(filepath.Join(home, "VERSION"), 0700); err != nil {
			t.Fatal(err)
		}
		got, err := RestoredFormat(home)
		if err == nil {
			t.Fatalf("RestoredFormat = %q with VERSION a folder, want an error", got)
		}
		if got != "" {
			t.Fatalf("RestoredFormat = %q beside an error, want empty", got)
		}
	})
}

func TestProcessStartOfThisProcess(t *testing.T) {
	pid := os.Getpid()
	first := ProcessStart(pid)
	if first == "" {
		t.Fatalf("ProcessStart(%d) is empty for a running process on %s", pid, runtime.GOOS)
	}
	if again := ProcessStart(pid); again != first {
		t.Fatalf("ProcessStart is not stable: %q then %q", first, again)
	}
	if runtime.GOOS == "linux" {
		// The shared helper writes proc:<boot>:<start>; migrate keeps the older proc:<start>.
		if !strings.HasPrefix(first, "proc:") || strings.Count(first, ":") != 1 {
			t.Fatalf("token %q, want proc:<start> with one colon", first)
		}
	}
}

// The pid of a process that has run and been waited for is not running. The code
// reads /proc/<pid>/stat on Linux and ps elsewhere, and both fail for it, so the
// answer is "" and not a token that could match a live process.
func TestProcessStartOfAFinishedProcess(t *testing.T) {
	exe, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no true command: ", err)
	}
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	live := ProcessStart(os.Getpid())
	got := ProcessStart(pid)
	if got != "" {
		t.Fatalf("ProcessStart(%d) = %q for a finished process, want empty (live token %q)", pid, got, live)
	}
	if got == live {
		t.Fatal("a finished process has the identity of a live one")
	}
}

// namedHomes writes an old root with one named home per name, each with one project
// of its own (a folder of its own too, so the plan has no duplicate identity). fill
// adds what the test is about to each home.
func namedHomes(t *testing.T, names []string, fill func(name, dir string)) (from, home string) {
	t.Helper()
	root := gridhome.Real(t.TempDir())
	from = filepath.Join(root, "old")
	for _, name := range names {
		dir := filepath.Join(from, name)
		put(t, filepath.Join(dir, "projects", "p-"+name, "project.md"), "---\nfolder: "+filepath.Join(root, "work-"+name)+"\n---\n")
		fill(name, dir)
	}
	return from, filepath.Join(root, "new")
}

// Two named homes both have knowledge/notes.md, which lands on one path in the new
// home. The plan takes it once when both are the same entry, and otherwise blocks.
func TestSameEntryOfTwoSources(t *testing.T) {
	const clash = "knowledge/notes.md: destination already has different data"

	t.Run("identical bytes", func(t *testing.T) {
		// Three homes: the third compares with the second, whose hash is kept.
		from, home := namedHomes(t, []string{"alpha", "beta", "gamma"}, func(name, dir string) {
			put(t, filepath.Join(dir, "knowledge", "notes.md"), "same words\n")
		})
		p := scan(t, from, home)
		if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
			t.Fatalf("identical files conflict: %v %v", p.Conflicts, p.Unmapped)
		}
		if got := string(read(t, from, "gamma/knowledge/notes.md")); got != "same words\n" {
			t.Fatalf("fixture changed: %q", got)
		}
	})
	t.Run("same size, different bytes", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			put(t, filepath.Join(dir, "knowledge", "notes.md"), name[:2]+name[:2]+"\n")
		})
		wantOneConflict(t, scan(t, from, home), clash)
	})
	t.Run("different size", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			put(t, filepath.Join(dir, "knowledge", "notes.md"), strings.Repeat("x", len(name))+"\n")
		})
		wantOneConflict(t, scan(t, from, home), clash)
	})
	t.Run("a file and a folder", func(t *testing.T) {
		for _, first := range []string{"alpha", "beta"} {
			t.Run(first+" has the file", func(t *testing.T) {
				from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
					if name == first {
						put(t, filepath.Join(dir, "knowledge", "notes.md"), "words\n")
					} else {
						put(t, filepath.Join(dir, "knowledge", "notes.md", "part.md"), "words\n")
					}
				})
				p := scan(t, from, home)
				if !hasConflict(p, clash) {
					t.Fatalf("a file and a folder at one path did not conflict: %v", p.Conflicts)
				}
			})
		}
	})
	t.Run("two folders", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			put(t, filepath.Join(dir, "knowledge", "notes", name+".md"), "words\n")
		})
		p := scan(t, from, home)
		if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
			t.Fatalf("two folders at one path conflict: %v %v", p.Conflicts, p.Unmapped)
		}
	})
	t.Run("symlinks to one target", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			link(t, "elsewhere.md", filepath.Join(dir, "knowledge", "notes.md"))
		})
		p := scan(t, from, home)
		if len(p.Conflicts) != 0 || len(p.Unmapped) != 0 {
			t.Fatalf("links to one target conflict: %v %v", p.Conflicts, p.Unmapped)
		}
	})
	t.Run("symlinks to two targets", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			link(t, name+".md", filepath.Join(dir, "knowledge", "notes.md"))
		})
		wantOneConflict(t, scan(t, from, home), clash)
	})
	t.Run("a symlink and a file", func(t *testing.T) {
		from, home := namedHomes(t, []string{"alpha", "beta"}, func(name, dir string) {
			if name == "alpha" {
				link(t, "elsewhere.md", filepath.Join(dir, "knowledge", "notes.md"))
			} else {
				put(t, filepath.Join(dir, "knowledge", "notes.md"), "words\n")
			}
		})
		wantOneConflict(t, scan(t, from, home), clash)
	})
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Skip(err)
	}
}

func hasConflict(p *Plan, line string) bool {
	for _, c := range p.Conflicts {
		if c == line {
			return true
		}
	}
	return false
}

func wantOneConflict(t *testing.T, p *Plan, line string) {
	t.Helper()
	if len(p.Conflicts) != 1 || !hasConflict(p, line) {
		t.Fatalf("conflicts %q, want exactly %q", p.Conflicts, line)
	}
	if len(p.Unmapped) != 0 {
		t.Fatalf("unmapped %q", p.Unmapped)
	}
}
