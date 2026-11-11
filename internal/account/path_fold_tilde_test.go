package account

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// foldFixture is a machine where the registry record says ~/.claude-ann and the
// person is signed in on a folder spelled <Home>/.Claude-ann, which the variable
// names (the home scan only takes names starting .claude-).
func foldFixture(t *testing.T, goos string) (Env, string) {
	t.Helper()
	env, bin := machine(t)
	installFakeClaude(t, bin)
	env.GOOS = goos
	spelled := filepath.Join(env.Home, ".Claude-ann")
	login(t, spelled, "ann@example.com")
	t.Setenv("CLAUDE_CONFIG_DIR", spelled)
	write(t, registryPath(env), `{"accounts":[{"id":"claude-ann","kind":"claude","name":"ann@example.com","email":"ann@example.com","dir":"~/.claude-ann","added":"auto"}]}`)
	return env, spelled
}

func TestFoldDescribeMatchesTheFolderWhateverItsCase(t *testing.T) {
	for goos, want := range map[string]bool{"darwin": true, "windows": true, "linux": false} {
		t.Run(goos, func(t *testing.T) {
			env, _ := foldFixture(t, goos)
			a := byID(t, status(t, env, false), "claude-ann")
			if a.SignedIn == nil || *a.SignedIn != want {
				t.Fatalf("signedIn on %s: want %v, got %v", goos, want, a.SignedIn)
			}
			if a.Dir != filepath.Join(env.Home, ".claude-ann") {
				t.Fatalf("the account keeps the record's own spelling: %q", a.Dir)
			}
		})
	}
}

func TestFoldRegisterNewGivesANewEmailItsOwnFolderWhenTheCaseOnlyDiffers(t *testing.T) {
	for goos, ownFolder := range map[string]bool{"darwin": true, "windows": true, "linux": false} {
		t.Run(goos, func(t *testing.T) {
			env, bin := machine(t)
			installFakeClaude(t, bin)
			env.GOOS = goos
			login(t, filepath.Join(env.Home, ".claude-ann"), "bob@example.com")
			write(t, registryPath(env), `{"accounts":[{"id":"claude-ann","kind":"claude","name":"ann@example.com","email":"ann@example.com","dir":"~/.Claude-ann","added":"auto"}]}`)

			got := status(t, env, true)
			if len(got) != 2 {
				t.Fatalf("two records: %+v", got)
			}
			bob := byID(t, got, "claude-bob")
			wantDir := filepath.Join(env.Home, ".claude-ann")
			if ownFolder {
				wantDir = filepath.Join(env.Home, ".claude-bob")
				if st, err := os.Stat(wantDir); err != nil || !st.IsDir() {
					t.Fatalf("MakeFolder did not run: %v", err)
				}
			}
			if bob.Dir != wantDir || got[0].Dir == bob.Dir {
				t.Fatalf("on %s want bob on %s apart from ann: %+v", goos, wantDir, got)
			}
			before, _ := os.ReadFile(registryPath(env))
			status(t, env, true)
			after, _ := os.ReadFile(registryPath(env))
			if !bytes.Equal(before, after) {
				t.Fatalf("a second identical call rewrote the registry:\n%s\n%s", before, after)
			}
		})
	}
}

func TestFoldExpandTakesTildeBackslashOnWindowsOnly(t *testing.T) {
	home := "/home/ada"
	for _, c := range []struct{ goos, in, want string }{
		{"windows", `~\.claude-ann`, filepath.Join(home, ".claude-ann")},
		{"windows", `~\a\b`, filepath.Join(home, "a", "b")},
		{"windows", `~\a/b`, filepath.Join(home, "a", "b")},
		{"windows", "~", home},
		{"windows", "~/x", filepath.Join(home, "x")},
		{"linux", `~\.claude-ann`, filepath.Clean(`~\.claude-ann`)},
		{"darwin", `~\.claude-ann`, filepath.Clean(`~\.claude-ann`)},
		{"darwin", "~", home},
		{"linux", "~/x", filepath.Join(home, "x")},
		{"linux", "/a/b/../c", "/a/c"},
	} {
		if got := (Env{Home: home, GOOS: c.goos}).expand(c.in); got != c.want {
			t.Errorf("%s expand(%q) = %q, want %q", c.goos, c.in, got, c.want)
		}
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		if got := (Env{Home: home, GOOS: goos}).tilde(filepath.Join(home, ".claude-ann")); got != "~/.claude-ann" {
			t.Errorf("%s tilde = %q", goos, got)
		}
	}
}

func TestFoldWindowsReadsTheScannedSpellingUnderTheAccount(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			env, spelled := foldFixture(t, goos)
			folded := goos != "linux"
			ids := func() map[string]AccountWindows {
				got, err := Windows(env, "")
				if err != nil {
					t.Fatal(err)
				}
				m := map[string]AccountWindows{}
				for _, w := range got {
					m[w.ID] = w
				}
				return m
			}
			m := ids()
			_, hasFolderRow := m["Claude-ann"]
			if _, ok := m["claude-ann"]; !ok || hasFolderRow == folded {
				t.Fatalf("%s entries: %+v", goos, m)
			}

			future := fixedNow.Add(time.Hour).Unix()
			line := `{"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":` + strconv.FormatInt(future, 10) + `}}}`
			if err := Note(env, strings.NewReader(line), spelled); err != nil {
				t.Fatal(err)
			}
			m = ids()
			acct, folder := m["claude-ann"], m["Claude-ann"]
			if folded {
				if acct.Source != "statusline" || len(acct.Windows) != 1 || acct.Windows[0].Used != 23.5 {
					t.Fatalf("%s account entry: %+v", goos, acct)
				}
				if _, ok := m["Claude-ann"]; ok {
					t.Fatalf("%s must not add a row named after the folder: %+v", goos, m)
				}
			} else if acct.Source != "none" || folder.Source != "statusline" || len(folder.Windows) != 1 {
				t.Fatalf("linux as on main: account %+v folder %+v", acct, folder)
			}
		})
	}
}

