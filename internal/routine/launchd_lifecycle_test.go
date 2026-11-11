package routine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Install then remove on macOS through a fake launchctl: the commands run in
// order, the plist is on disk when launchctl is called, and nothing is left in
// the temporary LaunchAgents folder or the ownership record afterwards.
func TestLaunchdInstallThenRemoveLeavesNoPlist(t *testing.T) {
	for _, mode := range []string{"unschedule", "remove-routine", "failed-bootstrap"} {
		t.Run(mode, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			gh := filepath.Join(home, ".grid")
			store := Store{GridHome: gh}
			if _, err := store.Add("daily", "0 9 * * *", "grid version"); err != nil {
				t.Fatal(err)
			}
			s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "grid"), Backend: Launchd{UID: 501}, Runner: &fakeOS{}}
			p, err := s.plan("", "daily")
			if err != nil {
				t.Fatal(err)
			}
			agents := filepath.Join(home, "Library", "LaunchAgents")
			plist := filepath.Join(agents, p.Label+".plist")

			// Each entry records the call and whether the plist was on disk then.
			var calls []string
			launchctl := &fakeOS{fail: mode == "failed-bootstrap"}
			launchctl.check = func(c OSCommand) {
				_, err := os.Stat(plist)
				calls = append(calls, c.Program+" "+strings.Join(c.Args, " ")+" plist-present="+map[bool]string{true: "yes", false: "no"}[err == nil])
			}
			s.Runner = launchctl

			err = s.Schedule(context.Background(), store)
			if mode == "failed-bootstrap" {
				if err == nil {
					t.Fatal("a failed launchctl bootstrap must surface")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if st, err := os.Stat(plist); err != nil || st.Mode().Perm() != 0o600 {
				t.Fatalf("plist after install: %v %v", st, err)
			}
			wantInstall := "launchctl bootstrap gui/501 " + plist + " plist-present=yes"
			if !reflect.DeepEqual(calls, []string{wantInstall}) {
				t.Fatalf("install calls: %q, want %q", calls, wantInstall)
			}

			calls = nil
			launchctl.fail = false
			switch mode {
			case "remove-routine":
				err = s.RemoveRoutine(context.Background(), store, "daily")
			default:
				err = s.Unschedule(context.Background(), "", "daily")
			}
			if err != nil {
				t.Fatal(err)
			}
			// A bootstrap that failed never confirmed an installation, so remove
			// does not deregister a label that may belong to someone else.
			wantRemove := []string{"launchctl bootout gui/501/" + p.Label + " plist-present=yes"}
			if mode == "failed-bootstrap" {
				wantRemove = nil
			}
			if !reflect.DeepEqual(calls, wantRemove) {
				t.Fatalf("remove calls: %q, want %q", calls, wantRemove)
			}
			left, err := os.ReadDir(agents)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range left {
				t.Fatalf("LaunchAgents still holds %s", entry.Name())
			}
			if _, err := os.Stat(filepath.Join(gh, "local", "routine-timers", p.Label+".json")); !os.IsNotExist(err) {
				t.Fatalf("ownership record remains: %v", err)
			}
			if mode == "remove-routine" {
				if _, err := store.Get("daily"); err == nil {
					t.Fatal("routine definition remains after remove")
				}
			}
		})
	}
}
