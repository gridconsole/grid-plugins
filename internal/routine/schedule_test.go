package routine

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

type fakeOS struct {
	calls []OSCommand
	fail  bool
}

func (f *fakeOS) Run(_ context.Context, c OSCommand) error {
	f.calls = append(f.calls, c)
	if f.fail {
		return errors.New("OS failure")
	}
	return nil
}

func TestScheduleRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		backend Backend
	}{{"linux", Systemd{}}, {"darwin", Launchd{UID: 501}}, {"windows", Schtasks{}}} {
		t.Run(tc.name, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			gh := filepath.Join(home, "data & space")
			store := Store{GridHome: gh, Project: "example"}
			if _, err := store.Add("daily", "0 9 * * *", "grid version"); err != nil {
				t.Fatal(err)
			}
			calls := &fakeOS{}
			s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "bin & space", "grid"), Backend: tc.backend, Runner: calls}
			p, err := s.plan(store.Project, "daily")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Schedule(context.Background(), store); err != nil {
				t.Fatal(err)
			}
			count := len(calls.calls)
			if count == 0 {
				t.Fatal("OS registration missing")
			}
			if err := s.Schedule(context.Background(), store); err != nil || len(calls.calls) != count {
				t.Fatalf("repeat schedule: %v %d", err, len(calls.calls))
			}
			for _, f := range p.Files {
				b, err := os.ReadFile(f.Path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(b), "--due") && !strings.HasSuffix(f.Path, ".timer") {
					t.Fatal("timer bypasses interval guard")
				}
				if strings.HasSuffix(f.Path, ".xml") || strings.HasSuffix(f.Path, ".plist") {
					decoder := xml.NewDecoder(strings.NewReader(string(b)))
					for {
						_, err := decoder.Token()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatalf("bad XML: %v", err)
						}
					}
				}
			}
			// Removal still works if an external edit deleted the definition first.
			if err := store.Remove("daily"); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(filepath.Dir(p.Files[0].Path), "unrelated.timer")
			if err := os.WriteFile(foreign, []byte("unrelated"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := s.Unschedule(context.Background(), store.Project, ""); err != nil {
				t.Fatal(err)
			}
			for _, f := range p.Files {
				if _, err := os.Stat(f.Path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("timer remains: %s %v", f.Path, err)
				}
			}
			if _, err := os.Stat(foreign); err != nil {
				t.Fatal("removed unrelated timer")
			}
			count = len(calls.calls)
			if err := s.Unschedule(context.Background(), store.Project, ""); err != nil || len(calls.calls) != count {
				t.Fatal("repeat removal called OS")
			}
		})
	}
}

func TestTimerOwnershipAndFailure(t *testing.T) {
	for _, mode := range []string{"foreign", "symlink", "failure", "other-machine"} {
		t.Run(mode, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			gh := filepath.Join(home, ".grid")
			store := Store{GridHome: gh}
			if _, err := store.Add("daily", "1h", "grid version"); err != nil {
				t.Fatal(err)
			}
			calls := &fakeOS{}
			s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "grid"), Backend: Systemd{}, Runner: calls}
			p, _ := s.plan("", "daily")
			if err := os.MkdirAll(filepath.Dir(p.Files[0].Path), 0o700); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "foreign":
				if err := os.WriteFile(p.Files[0].Path, []byte("foreign"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(home, "elsewhere"), p.Files[0].Path); err != nil {
					t.Fatal(err)
				}
			case "failure":
				calls.fail = true
			case "other-machine":
				if err := os.WriteFile(filepath.Join(gh, "machine"), []byte("01ARZ3NDEKTSV4RRFFQ69G5FAV\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := s.Schedule(context.Background(), store)
			if mode == "other-machine" {
				if err != nil || len(calls.calls) != 0 {
					t.Fatalf("foreign owner scheduled: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted invalid installation")
			}
			if mode != "failure" && len(calls.calls) != 0 {
				t.Fatal("OS touched before ownership check")
			}
			if mode == "failure" {
				state, err := s.readState(p)
				if err != nil || state.Installed || state.Label == "" {
					t.Fatalf("recovery state: %+v %v", state, err)
				}
				calls.fail = false
				if err := s.Schedule(context.Background(), store); err != nil {
					t.Fatal(err)
				}
				calls.fail = true
				if err := s.Unschedule(context.Background(), "", "daily"); err == nil {
					t.Fatal("OS removal failure hidden")
				}
				if _, err := os.Stat(p.Files[0].Path); err != nil {
					t.Fatal("deleted file after OS failure")
				}
			}
		})
	}
}

func TestPreviewDoesNotWriteOrInvokeOS(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	gh := filepath.Join(home, ".grid")
	store := Store{GridHome: gh}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	calls := &fakeOS{}
	scheduler := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "grid.exe"), Backend: Schtasks{}, Runner: calls}
	for _, remove := range []bool{false, true} {
		plans, err := scheduler.Preview(store, remove)
		if err != nil || len(plans) != 1 {
			t.Fatalf("preview: %v %v", plans, err)
		}
		if len(calls.calls) != 0 {
			t.Fatal("preview invoked OS")
		}
		if _, err := os.Stat(plans[0].Files[0].Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("preview created a timer file")
		}
		if remove && (len(plans[0].Install) != 0 || len(plans[0].Remove) != 1) {
			t.Fatal("wrong removal plan")
		}
	}
}

func TestTimerEscaping(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	spec := TimerSpec{Home: home, GridHome: filepath.Join(home, "$data% & spaces"), Binary: filepath.Join(home, "grid $tool%"), Name: "daily", Path: "/example/bin:$literal%"}
	p, err := (Systemd{}).Plan(spec)
	if err != nil {
		t.Fatal(err)
	}
	service := p.Files[0].Content
	if !strings.Contains(service, `$data%% & spaces`) || !strings.Contains(service, `grid $$tool%%`) || !strings.Contains(service, `PATH=/example/bin:$literal%%`) {
		t.Fatalf("systemd escaping: %s", service)
	}
	// systemd does not unquote WorkingDirectory=, and it expands only percent specifiers.
	if want := "\nWorkingDirectory=" + strings.ReplaceAll(spec.GridHome, "%", "%%") + "\n"; !strings.Contains(service, want) {
		t.Fatalf("systemd working directory line %q missing: %s", want, service)
	}
	for _, tc := range []struct{ in, want string }{{`a b`, `"a b"`}, {`a"b`, `"a\"b"`}, {`a\`, `"a\\"`}} {
		if got := windowsArg(tc.in); got != tc.want {
			t.Fatalf("Windows argv %q: %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestFailedRegistrationNeverAuthorizesOSRemoval(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	store := Store{GridHome: filepath.Join(home, ".grid")}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	calls := &fakeOS{fail: true}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: Launchd{UID: 501}, Runner: calls}
	if err := s.Schedule(context.Background(), store); err == nil {
		t.Fatal("expected registration error")
	}
	calls.fail = false
	before := len(calls.calls)
	if err := s.Unschedule(context.Background(), "", "daily"); err != nil {
		t.Fatal(err)
	}
	if len(calls.calls) != before {
		t.Fatal("removed OS label after failed registration")
	}
}
