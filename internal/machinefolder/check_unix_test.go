//go:build unix

package machinefolder_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

const testGroup = "gridtest"

// opts answers the group questions from fixed values, so no test depends on the
// groups of the machine it runs on. The folder's own group is the test's gid.
func opts(e machinefolder.Env, groupGID string, member []string) machinefolder.CheckOptions {
	return machinefolder.CheckOptions{
		Env:   e,
		Login: "alice",
		LookupGroup: func(name string) (string, error) {
			if name == testGroup {
				return groupGID, nil
			}
			return "", errors.New("unknown group")
		},
		GroupName: func(gid string) (string, error) { return "g" + gid, nil },
		Groups:    func() ([]string, error) { return member, nil },
	}
}

func byName(rows []machinefolder.Finding, name string) *machinefolder.Finding {
	for i := range rows {
		if rows[i].Name == name {
			return &rows[i]
		}
	}
	return nil
}

func ownGID() string { return strconv.Itoa(os.Getgid()) }

func sharedSetup(t *testing.T) (machinefolder.Folder, machinefolder.Env) {
	t.Helper()
	f := sharedFolder(t)
	f.SharedGroup = testGroup
	e := machinefolder.Env{Home: t.TempDir(), GOOS: "linux", Getenv: func(string) string { return "" }}
	return f, e
}

func TestCheckSharedFolderAllHold(t *testing.T) {
	f, e := sharedSetup(t)
	rows := machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()}))
	for _, name := range []string{machinefolder.RowFolder, machinefolder.RowGroup, machinefolder.RowMode, machinefolder.RowLogin, machinefolder.RowLeftover} {
		r := byName(rows, name)
		if r == nil || !r.OK || r.Fix != "" {
			t.Errorf("row %q = %+v, want OK with no fix", name, r)
		}
	}
}

// Row 5: a wrong group, a missing setgid bit and a private leftover folder are each a
// finding that names the fix command.
func TestCheckWrongGroup(t *testing.T) {
	f, e := sharedSetup(t)
	rows := machinefolder.Check(f, opts(e, "424242", []string{"424242"}))
	r := byName(rows, machinefolder.RowGroup)
	if r == nil || r.OK {
		t.Fatalf("wrong group not found: %+v", rows)
	}
	if want := "sudo chgrp -R " + testGroup + " " + f.Path; r.Fix != want {
		t.Fatalf("fix = %q, want %q", r.Fix, want)
	}
	if !strings.Contains(r.Detail, testGroup) {
		t.Fatalf("detail does not name the wanted group: %q", r.Detail)
	}
}

func TestCheckMissingSetgid(t *testing.T) {
	f, e := sharedSetup(t)
	if err := os.Chmod(f.Path, 0o770); err != nil {
		t.Fatal(err)
	}
	rows := machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()}))
	r := byName(rows, machinefolder.RowMode)
	if r == nil || r.OK {
		t.Fatalf("missing setgid not found: %+v", rows)
	}
	if want := "sudo chmod 2770 " + f.Path; r.Fix != want {
		t.Fatalf("fix = %q, want %q", r.Fix, want)
	}
	if !strings.Contains(r.Detail, "0770") || !strings.Contains(r.Detail, "setgid") {
		t.Fatalf("detail = %q", r.Detail)
	}
	// Group access missing altogether is a finding too.
	if err := os.Chmod(f.Path, machinefolder.SharedDirMode&^0o070); err != nil {
		t.Fatal(err)
	}
	if r := byName(machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()})), machinefolder.RowMode); r == nil || r.OK || r.Fix == "" {
		t.Fatalf("a folder the group cannot enter: %+v", r)
	}
}

func TestCheckLoginNotInGroup(t *testing.T) {
	f, e := sharedSetup(t)
	rows := machinefolder.Check(f, opts(e, ownGID(), []string{"1"}))
	r := byName(rows, machinefolder.RowLogin)
	if r == nil || r.OK {
		t.Fatalf("login outside the group not found: %+v", rows)
	}
	if !strings.HasPrefix(r.Fix, "sudo usermod -aG "+testGroup+" alice") {
		t.Fatalf("fix = %q", r.Fix)
	}
}

func TestCheckMissingSharedFolderPrintsSetupCommands(t *testing.T) {
	f, e := sharedSetup(t)
	f.Path = filepath.Join(t.TempDir(), "nothing-here")
	rows := machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()}))
	r := byName(rows, machinefolder.RowFolder)
	if r == nil || r.OK {
		t.Fatalf("missing folder not found: %+v", rows)
	}
	for _, want := range []string{"sudo mkdir -p " + f.Path, "sudo chgrp " + testGroup + " " + f.Path, "sudo chmod 2770 " + f.Path, "sudo usermod -aG " + testGroup + " alice"} {
		if !strings.Contains(r.Fix, want) {
			t.Errorf("fix %q lacks %q", r.Fix, want)
		}
	}
	if _, err := os.Stat(f.Path); err == nil {
		t.Fatal("Check made the folder")
	}
}

func TestCheckPrivateLeftoverFolder(t *testing.T) {
	home := t.TempDir()
	getenv := func(k string) string {
		if k == machinefolder.EnvDir {
			return filepath.Join(home, "elsewhere")
		}
		return ""
	}
	e := machinefolder.Env{Home: home, GOOS: "linux", Getenv: getenv}
	old := machinefolder.DefaultPath(e)
	if err := os.MkdirAll(filepath.Join(old, "launcher"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "launcher", "pid"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := machinefolder.Resolve(e)
	if err := os.MkdirAll(f.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	rows := machinefolder.Check(f, opts(e, ownGID(), nil))
	r := byName(rows, machinefolder.RowLeftover)
	if r == nil || r.OK {
		t.Fatalf("private leftover not found: %+v", rows)
	}
	if !strings.Contains(r.Detail, old) || !strings.Contains(r.Fix, "cp -a") || !strings.Contains(r.Fix, f.Path) {
		t.Fatalf("leftover row: %+v", r)
	}
	if strings.Contains(r.Fix, "chgrp") {
		t.Fatalf("a private target needs no group commands: %q", r.Fix)
	}
	// The old folder is where it was; nothing moved.
	if _, err := os.Stat(filepath.Join(old, "launcher", "pid")); err != nil {
		t.Fatalf("the old folder was changed: %v", err)
	}

	// In shared mode the fix carries the group and mode commands for the new folder.
	f.SharedGroup = testGroup
	r = byName(machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()})), machinefolder.RowLeftover)
	if r == nil || r.OK || !strings.Contains(r.Fix, "chgrp -R "+testGroup) || !strings.Contains(r.Fix, "2770") {
		t.Fatalf("shared leftover fix: %+v", r)
	}

	// An empty old folder, or the old folder in use, is nothing left behind.
	if err := os.Remove(filepath.Join(old, "launcher", "pid")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(old, "launcher")); err != nil {
		t.Fatal(err)
	}
	if r := byName(machinefolder.Check(f, opts(e, ownGID(), []string{ownGID()})), machinefolder.RowLeftover); r == nil || !r.OK {
		t.Fatalf("an empty old folder is not a finding: %+v", r)
	}
}

// Every finding that fails names its fix, and Check changes nothing.
func TestCheckEveryFailureHasAFixAndChangesNothing(t *testing.T) {
	f, e := sharedSetup(t)
	if err := os.Chmod(f.Path, 0o750); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(f.Path)
	rows := machinefolder.Check(f, opts(e, "424242", []string{"1"}))
	failed := 0
	for _, r := range rows {
		if !r.OK {
			failed++
			if strings.TrimSpace(r.Fix) == "" || strings.TrimSpace(r.Detail) == "" {
				t.Errorf("failing row %q has no detail or fix: %+v", r.Name, r)
			}
		}
	}
	if failed < 3 {
		t.Fatalf("expected the group, mode and login rows to fail, got %+v", rows)
	}
	after, _ := os.Stat(f.Path)
	if after.Mode() != before.Mode() {
		t.Fatalf("Check changed the folder mode: %v to %v", before.Mode(), after.Mode())
	}
	if entries, _ := os.ReadDir(f.Path); len(entries) != 0 {
		t.Fatalf("Check left %d entries in the folder", len(entries))
	}
}

// grid setup --shared-group looks first, writes nothing, and when the folder is not
// ready it has the exact commands.
func TestPlanShared(t *testing.T) {
	home := t.TempDir()
	missing := filepath.Join(t.TempDir(), "grid")
	e := machinefolder.Env{Home: home, GOOS: "linux", Getenv: func(k string) string {
		if k == machinefolder.EnvDir {
			return missing
		}
		return ""
	}}
	o := opts(e, ownGID(), nil)
	p := machinefolder.PlanShared(e, testGroup, o)
	if p.Ready || p.Path != missing || p.Why == "" {
		t.Fatalf("plan = %+v", p)
	}
	want := []string{"sudo mkdir -p " + missing, "sudo chgrp " + testGroup + " " + missing, "sudo chmod 2770 " + missing, "sudo usermod -aG " + testGroup + " alice"}
	if strings.Join(p.Commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(p.Commands, "\n"), strings.Join(want, "\n"))
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("PlanShared made the folder")
	}
	// A group that does not exist yet starts with groupadd.
	p = machinefolder.PlanShared(e, "newgroup", o)
	if len(p.Commands) == 0 || p.Commands[0] != "sudo groupadd newgroup" {
		t.Fatalf("commands = %v", p.Commands)
	}

	if err := os.Mkdir(missing, 0o700); err != nil {
		t.Fatal(err)
	}
	p = machinefolder.PlanShared(e, testGroup, o)
	if !p.Ready || len(p.Commands) != 0 {
		t.Fatalf("a folder the login can write is ready: %+v", p)
	}
	if entries, _ := os.ReadDir(missing); len(entries) != 0 {
		t.Fatalf("PlanShared left %d entries in the folder", len(entries))
	}
}
