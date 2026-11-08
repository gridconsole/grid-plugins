package routine

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

type fakeOS struct {
	calls []OSCommand
	fail  bool
	check func(OSCommand)
}

func (f *fakeOS) Run(_ context.Context, c OSCommand) error {
	f.calls = append(f.calls, c)
	if f.check != nil {
		f.check(c)
	}
	if f.fail {
		return errors.New("OS failure")
	}
	return nil
}

func TestLaunchdIntervalAndLogs(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	store := Store{GridHome: filepath.Join(home, "data & space")}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: Launchd{UID: 501}, Runner: &fakeOS{}}
	p, err := s.plan("", "daily")
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(store.GridHome, "logs", "routines", p.Label)
	if p.LogDir != wantDir {
		t.Fatalf("log folder: %s", p.LogDir)
	}
	want := map[string]string{
		"StartInterval":     "60",
		"StandardOutPath":   filepath.Join(wantDir, "stdout.log"),
		"StandardErrorPath": filepath.Join(wantDir, "stderr.log"),
	}
	decoder := xml.NewDecoder(strings.NewReader(p.Files[0].Content))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "key" {
			continue
		}
		var key string
		if err := decoder.DecodeElement(&key, &start); err != nil {
			t.Fatal(err)
		}
		if key == "StartCalendarInterval" {
			t.Fatal("calendar trigger remains")
		}
		if expected, ok := want[key]; ok {
			token, err := decoder.Token()
			if err != nil {
				t.Fatal(err)
			}
			start, ok := token.(xml.StartElement)
			kind := "string"
			if key == "StartInterval" {
				kind = "integer"
			}
			if !ok || start.Name.Local != kind {
				t.Fatalf("%s must be %s: %v", key, kind, token)
			}
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil || value != expected {
				t.Fatalf("%s: %q, want %q: %v", key, value, expected, err)
			}
			delete(want, key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing plist settings: %v", want)
	}
	s.Runner = &fakeOS{check: func(OSCommand) {
		if st, err := os.Stat(wantDir); err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
			t.Fatalf("log folder unavailable at registration: %v %v", st, err)
		}
	}}
	if err := s.Schedule(context.Background(), store); err != nil {
		t.Fatal(err)
	}
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

func TestTimerRunRefusesRoutineChangedSinceSchedule(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	now := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	store := Store{GridHome: filepath.Join(home, ".grid"), Project: "example", Now: func() time.Time { return now }}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	runner := executorFunc(func(context.Context, Store, Entry) error { runs.Add(1); return nil })
	var signals int
	signalErr := errors.New("signal unavailable")
	run := func(due bool) error {
		_, err := store.Execute(context.Background(), "daily", due, runner, func(e Entry, err error) error {
			signals++
			if !errors.Is(err, ErrScheduleChanged) || e.LastRun.Equal(now) {
				t.Fatalf("wrong refusal signal: %+v %v", e, err)
			}
			return signalErr
		})
		return err
	}
	// Without a schedule nothing was approved, so there is nothing to compare.
	if err := run(true); err != nil || runs.Load() != 1 {
		t.Fatalf("unscheduled timer run: %v %d", err, runs.Load())
	}
	calls := &fakeOS{}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: Systemd{}, Runner: calls}
	if err := s.Schedule(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	if err := run(true); err != nil || runs.Load() != 2 {
		t.Fatalf("scheduled timer run: %v %d", err, runs.Load())
	}
	for _, edit := range []func(*Entry){
		func(e *Entry) { e.Run = "grid brain index" },
		func(e *Entry) { e.Every = "1m" },
	} {
		e, err := store.Get("daily")
		if err != nil {
			t.Fatal(err)
		}
		edit(&e)
		if err = store.save(e); err != nil {
			t.Fatal(err)
		}
		before := runs.Load()
		beforeSignals := signals
		now = now.Add(2 * time.Hour)
		err = run(true)
		if !errors.Is(err, ErrScheduleChanged) || !errors.Is(err, signalErr) || runs.Load() != before {
			t.Fatalf("changed routine ran or signal error lost: %v %d", err, runs.Load())
		}
		if signals != beforeSignals+1 {
			t.Fatalf("refusal signals: %d", signals-beforeSignals)
		}
		// A manual run is the person's own request.
		if err = run(false); err != nil || runs.Load() != before+1 {
			t.Fatalf("manual run refused: %v %d", err, runs.Load())
		}
		// The files do not hold the command, so approving it again makes no OS call.
		osCalls := len(calls.calls)
		if err = s.Schedule(context.Background(), store); err != nil || len(calls.calls) != osCalls {
			t.Fatalf("reschedule: %v %d", err, len(calls.calls)-osCalls)
		}
		now = now.Add(2 * time.Hour)
		if err = run(true); err != nil || runs.Load() != before+2 {
			t.Fatalf("approved routine refused: %v %d", err, runs.Load())
		}
	}
}

func TestScheduleFollowsSymlinkedParentFolders(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	// A dotfile manager keeps the real ~/.config elsewhere.
	target := filepath.Join(home, "dotfiles", "config")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, ".config")); err != nil {
		t.Fatal(err)
	}
	store := Store{GridHome: filepath.Join(home, ".grid")}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	calls := &fakeOS{}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: Systemd{}, Runner: calls}
	p, err := s.plan("", "daily")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Schedule(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(target, "systemd", "user", filepath.Base(p.Files[0].Path))); err != nil {
		t.Fatalf("timer not written through the symlinked folder: %v", err)
	}
	if err = s.Unschedule(context.Background(), "", "daily"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(p.Files[0].Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("timer remains: %v", err)
	}
	// The timer file itself is still never followed.
	if err = os.Symlink(filepath.Join(home, "elsewhere"), p.Files[0].Path); err != nil {
		t.Fatal(err)
	}
	if err = s.Schedule(context.Background(), store); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("timer file symlink accepted: %v", err)
	}
}
