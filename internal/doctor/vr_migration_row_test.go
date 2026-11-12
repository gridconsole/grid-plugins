package doctor

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// vrPendCheck returns the check of that name; the report must hold exactly one.
func vrPendCheck(t *testing.T, rep Report, name string) Check {
	t.Helper()
	var found []Check
	for _, c := range rep.Checks {
		if c.Name == name {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the report holds %d checks named %q: %+v", len(found), name, rep.Checks)
	}
	return found[0]
}

func vrPendNames(rep Report) []string {
	var names []string
	for _, c := range rep.Checks {
		names = append(names, c.Name)
	}
	return names
}

func TestVrPendDoctorShowsAnInterruptedMigrationWhenTheHomeIsAbsent(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, ".grid")
	if err := os.WriteFile(home+".migrate-journal", []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := Run(Options{GridHome: home, Label: "~/.grid", GitVersion: gitOK})
	c := vrPendCheck(t, rep, "migration")
	if c.OK {
		t.Errorf("migration check passes with a journal beside an absent home: %+v", c)
	}
	if want := "a migration of ~/.grid stopped part way"; c.Detail != want {
		t.Errorf("detail %q, want %q", c.Detail, want)
	}
	if want := "Run grid migrate --apply"; c.Fix != want {
		t.Errorf("fix %q, want %q", c.Fix, want)
	}
	if rep.OK {
		t.Error("the report is OK with an interrupted migration")
	}
	// The row sits in the gap, where the home is absent: right after "grid home".
	if names := vrPendNames(rep); len(names) < 2 || names[0] != "grid home" || names[1] != "migration" {
		t.Errorf("order %q; want migration right after grid home", names)
	}
}

func TestVrPendDoctorMigrationRowIsOKWithNoJournal(t *testing.T) {
	t.Run("home absent", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "absent")
		c := vrPendCheck(t, Run(Options{GridHome: home, Label: "~/.grid", GitVersion: gitOK}), "migration")
		if !c.OK || c.Detail != "no interrupted migration" || c.Fix != "" {
			t.Errorf("check %+v; want ok with detail %q and no fix", c, "no interrupted migration")
		}
	})
	t.Run("home present, after format version", func(t *testing.T) {
		home := t.TempDir()
		writeVersion(t, home, "1")
		rep := Run(Options{GridHome: home, Label: "~/.grid", GitVersion: gitOK})
		c := vrPendCheck(t, rep, "migration")
		if !c.OK || c.Detail != "no interrupted migration" {
			t.Errorf("check %+v; want ok with detail %q", c, "no interrupted migration")
		}
		want := []string{"grid home", "grid home writable", "format version", "migration", "git"}
		if names := vrPendNames(rep); len(names) < len(want) || !slices.Equal(names[:len(want)], want) {
			t.Errorf("order %q; want it to begin %q", names, want)
		}
	})
}

// A journal beside a home that exists is still an open migration (the copy phase).
func TestVrPendDoctorShowsTheJournalBesideAHomeThatExists(t *testing.T) {
	home := t.TempDir()
	writeVersion(t, home, "1")
	if err := os.WriteFile(home+".migrate-journal", []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := vrPendCheck(t, Run(Options{GridHome: home, Label: "~/.grid", GitVersion: gitOK}), "migration")
	if c.OK || c.Fix != "Run grid migrate --apply" {
		t.Errorf("check %+v; want a failure with the migrate fix", c)
	}
}
