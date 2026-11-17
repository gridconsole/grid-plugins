//go:build unix

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

const mfGroup = "gridtest"

// mfOpts answers the group questions from fixed values, so no test depends on the
// groups of the machine it runs on. gid is the group the name resolves to.
func mfOpts(gid string, member []string) machinefolder.CheckOptions {
	return machinefolder.CheckOptions{
		Login: "alice",
		LookupGroup: func(name string) (string, error) {
			if name == mfGroup {
				return gid, nil
			}
			return "", errors.New("unknown group")
		},
		GroupName: func(g string) (string, error) { return "g" + g, nil },
		Groups:    func() ([]string, error) { return member, nil },
	}
}

func mfOwnGID() string { return strconv.Itoa(os.Getgid()) }

// mfHarness is a harness whose environment is a map, so the process environment
// never reaches Grid and no check can touch a real machine folder.
type mfHarness struct {
	*harness
	vars   map[string]string
	folder string // the shared folder GRID_MACHINE_DIR names
}

func newMfHarness(t *testing.T) *mfHarness {
	t.Helper()
	h := &mfHarness{harness: newHarness(t), vars: map[string]string{}}
	h.env.Getenv = func(k string) string { return h.vars[k] }
	h.env.machineGOOS = "linux"
	h.env.machineCheck = mfOpts(mfOwnGID(), []string{mfOwnGID()})
	h.folder = filepath.Join(h.root, "shared")
	return h
}

// config writes the one config of the login.
func (h *mfHarness) config(text string) {
	h.t.Helper()
	h.write(filepath.Join(h.home, ".config", "grid", "config.json"), text)
}

// sharedFolder makes the shared folder as a person would: the login's own group,
// mode 2770, and GRID_MACHINE_DIR naming it.
func (h *mfHarness) sharedFolder(mode os.FileMode) {
	h.t.Helper()
	h.mkdir(h.folder)
	if err := os.Chmod(h.folder, mode); err != nil {
		h.t.Fatal(err)
	}
	h.vars[machinefolder.EnvDir] = h.folder
}

type mfRow struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// doctor runs grid doctor --json and returns the rows by name and in order.
func (h *mfHarness) doctor() (map[string]mfRow, []string, int) {
	h.t.Helper()
	code := h.run(h.tmp, "doctor", "--json")
	var rep struct {
		OK     bool    `json:"ok"`
		Checks []mfRow `json:"checks"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &rep); err != nil {
		h.t.Fatalf("doctor --json is not JSON (exit %d): %v\n%s%s", code, err, h.out, h.errb)
	}
	if rep.OK != (code == 0) {
		h.t.Fatalf("doctor says ok=%v with exit %d", rep.OK, code)
	}
	rows := map[string]mfRow{}
	var names []string
	for _, c := range rep.Checks {
		rows[c.Name] = c
		names = append(names, c.Name)
	}
	return rows, names, code
}

var mfDefaultRows = []string{"grid home", "grid home writable", "format version", "migration", "git", "grid on PATH", "plugin hooks", "allow block", "deny block", "sandbox"}

// Row 2: with nothing configured the doctor lists the 10 rows of the README and no
// machine folder row, even when a folder of the OS standard place holds files.
func TestMfDoctorUnconfiguredHasNoMachineFolderRows(t *testing.T) {
	h := newMfHarness(t)
	std := machinefolder.DefaultPath(machinefolder.Env{Home: h.home, Getenv: h.env.Getenv})
	h.mkdir(filepath.Join(std, "launcher"))
	h.write(filepath.Join(std, "launcher", "pid"), "1\n")
	_, names, _ := h.doctor()
	if !reflect.DeepEqual(names, mfDefaultRows) {
		t.Fatalf("unconfigured doctor rows = %q, want %q", names, mfDefaultRows)
	}
}

// Row 1: a seeded shared folder with the right group and mode adds the rows, all
// holding and none naming a fix, after the 10 rows.
func TestMfDoctorConfiguredRowsFollowTheDefaultRows(t *testing.T) {
	h := newMfHarness(t)
	h.config(`{"shared_group":"` + mfGroup + `"}`)
	h.sharedFolder(os.ModeSetgid | 0o770)
	rows, names, _ := h.doctor()
	if !reflect.DeepEqual(names[:len(mfDefaultRows)], mfDefaultRows) {
		t.Fatalf("the default rows moved: %q", names)
	}
	want := []string{machinefolder.RowFolder, machinefolder.RowGroup, machinefolder.RowMode, machinefolder.RowLogin, machinefolder.RowLeftover}
	if !reflect.DeepEqual(names[len(mfDefaultRows):], want) {
		t.Fatalf("machine folder rows = %q, want %q", names[len(mfDefaultRows):], want)
	}
	for _, n := range want {
		if r := rows[n]; !r.OK || r.Fix != "" {
			t.Errorf("row %q = %+v, want OK with no fix", n, r)
		}
	}
}

// Row 1: a wrong group, a missing setgid bit and a private leftover folder are each
// a failing doctor row that names its fix command, and doctor exits 1.
func TestMfDoctorFindsWrongGroupMissingSetgidAndPrivateLeftover(t *testing.T) {
	t.Run("wrong group", func(t *testing.T) {
		h := newMfHarness(t)
		h.config(`{"shared_group":"` + mfGroup + `"}`)
		h.sharedFolder(os.ModeSetgid | 0o770)
		h.env.machineCheck = mfOpts("424242", []string{"424242"}) // the group is not the folder's
		rows, _, code := h.doctor()
		r := rows[machinefolder.RowGroup]
		if code != 1 || r.OK {
			t.Fatalf("exit %d, row %+v", code, r)
		}
		if want := "sudo chgrp -R " + mfGroup + " " + h.folder; r.Fix != want {
			t.Fatalf("fix = %q, want %q", r.Fix, want)
		}
		if !strings.Contains(r.Detail, mfGroup) {
			t.Fatalf("detail does not name the group: %q", r.Detail)
		}
	})
	t.Run("missing setgid", func(t *testing.T) {
		h := newMfHarness(t)
		h.config(`{"shared_group":"` + mfGroup + `"}`)
		h.sharedFolder(0o770)
		rows, _, code := h.doctor()
		r := rows[machinefolder.RowMode]
		if code != 1 || r.OK {
			t.Fatalf("exit %d, row %+v", code, r)
		}
		if want := "sudo chmod 2770 " + h.folder; r.Fix != want {
			t.Fatalf("fix = %q, want %q", r.Fix, want)
		}
		if !strings.Contains(r.Detail, "setgid") || !strings.Contains(r.Detail, "0770") {
			t.Fatalf("detail = %q", r.Detail)
		}
	})
	t.Run("private leftover", func(t *testing.T) {
		h := newMfHarness(t)
		h.config(`{"shared_group":"` + mfGroup + `"}`)
		h.sharedFolder(os.ModeSetgid | 0o770)
		old := machinefolder.DefaultPath(machinefolder.Env{Home: h.home, Getenv: h.env.Getenv})
		h.mkdir(filepath.Join(old, "launcher"))
		h.write(filepath.Join(old, "launcher", "pid"), "1\n")
		rows, _, code := h.doctor()
		r := rows[machinefolder.RowLeftover]
		if code != 1 || r.OK {
			t.Fatalf("exit %d, row %+v", code, r)
		}
		for _, want := range []string{"cp -a", h.folder, "chgrp -R " + mfGroup, "2770"} {
			if !strings.Contains(r.Fix, want) {
				t.Errorf("fix %q lacks %q", r.Fix, want)
			}
		}
		if !strings.Contains(r.Detail, old) {
			t.Errorf("detail %q does not name the old folder", r.Detail)
		}
		if _, err := os.Stat(filepath.Join(old, "launcher", "pid")); err != nil {
			t.Fatalf("doctor moved or removed the old folder: %v", err)
		}
	})
}

// The plain output prints each failing row with its fix line, and doctor changes
// nothing in the folder but the probe file it removes at once.
func TestMfDoctorPlainOutputPrintsTheFixAndChangesNothing(t *testing.T) {
	h := newMfHarness(t)
	h.config(`{"shared_group":"` + mfGroup + `"}`)
	h.sharedFolder(0o750)
	h.env.machineCheck = mfOpts("424242", []string{"1"})
	before, err := os.Stat(h.folder)
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.tmp, "doctor"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	out := h.out.String()
	for _, want := range []string{
		"fail machine folder group: ", "fix: sudo chgrp -R " + mfGroup + " " + h.folder + "\n",
		"fail machine folder mode: ", "fix: sudo chmod 2770 " + h.folder + "\n",
		"fail machine folder login: ", "fix: sudo usermod -aG " + mfGroup + " alice",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	after, err := os.Stat(h.folder)
	if err != nil || after.Mode() != before.Mode() {
		t.Fatalf("doctor changed the folder: %v %v", after, err)
	}
	if entries, _ := os.ReadDir(h.folder); len(entries) != 0 {
		t.Fatalf("doctor left %d entries in the folder", len(entries))
	}
}
