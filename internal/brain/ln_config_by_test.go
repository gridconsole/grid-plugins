package brain

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// lnCfgByChanges is the project level's audit lines, newest first.
func lnCfgByChanges(t *testing.T, s Store) []Change {
	t.Helper()
	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

func lnCfgByWrite(text string) func([]byte) ([]byte, error) {
	return func([]byte) ([]byte, error) { return EncodeConfig(map[string]string{"name": "dev"}, text) }
}

func lnCfgByDelete(old []byte) ([]byte, error) { return nil, nil }

func TestLnCfgByPersonOriginIsOnTheAuditLine(t *testing.T) {
	s := testStore(t)
	bo := Origin{Kind: "person", ID: "bo"}
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByWrite("First.")); err != nil {
		t.Fatal(err)
	}
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByWrite("Second.")); err != nil {
		t.Fatal(err)
	}
	changes := lnCfgByChanges(t, s)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want 2", changes)
	}
	for i, c := range changes {
		if c.Status != "accepted" || c.OriginKind != "person" || c.OriginID != "bo" || c.Topic != "environments" || c.Name != "dev" || c.Version != 2-i {
			t.Fatalf("change %d = %+v, want accepted by person:bo, v%d", i, c, 2-i)
		}
	}
	// The same bytes again are not a change: no version and no line, as EditConfig.
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByWrite("Second.")); err != nil {
		t.Fatal(err)
	}
	if got := lnCfgByChanges(t, s); len(got) != 2 {
		t.Fatalf("an unchanged write logged: %+v", got)
	}
}

func TestLnCfgByEditConfigStillStampsTheSession(t *testing.T) {
	s := testStore(t)
	if err := s.EditConfig("project", "environments", "dev", lnCfgByWrite("First.")); err != nil {
		t.Fatal(err)
	}
	changes := lnCfgByChanges(t, s)
	// config.go on main writes the CLI session's random id with the "config" kind.
	if len(changes) != 1 || changes[0].OriginKind != "config" || len(changes[0].OriginID) != 24 || changes[0].Status != "accepted" {
		t.Fatalf("EditConfig changes = %+v, want one accepted config:<session id> line", changes)
	}
	// A delete through EditConfig still writes no line.
	if err := s.EditConfig("project", "environments", "dev", lnCfgByDelete); err != nil {
		t.Fatal(err)
	}
	if got := lnCfgByChanges(t, s); len(got) != 1 {
		t.Fatalf("an EditConfig delete logged: %+v", got)
	}
}

func TestLnCfgByDeleteWritesOneDroppedLine(t *testing.T) {
	s := testStore(t)
	bo := Origin{Kind: "person", ID: "bo"}
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByWrite("First.")); err != nil {
		t.Fatal(err)
	}
	stored, err := s.ReadConfig("project", "environments", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByDelete); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "knowledge", "environments", "dev.md")); !os.IsNotExist(err) {
		t.Fatalf("the entry is still there: %v", err)
	}
	changes := lnCfgByChanges(t, s)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want the add and one drop", changes)
	}
	drop := changes[0]
	if drop.Status != "dropped" || drop.OriginKind != "person" || drop.OriginID != "bo" || drop.Topic != "environments" || drop.Name != "dev" || drop.Version != 1 || drop.Hash != auditHash(stored) {
		t.Fatalf("drop line = %+v, want dropped by person:bo at v1 with the removed bytes' hash", drop)
	}
	// A delete of an entry that is not there writes nothing.
	if err := s.EditConfigBy("project", "environments", "dev", bo, lnCfgByDelete); err != nil {
		t.Fatal(err)
	}
	if err := s.EditConfigBy("project", "environments", "never", bo, lnCfgByDelete); err != nil {
		t.Fatal(err)
	}
	if got := lnCfgByChanges(t, s); len(got) != 2 {
		t.Fatalf("a delete of a missing entry logged: %+v", got)
	}
}

func TestLnCfgByRefusesAnEmptyAuthor(t *testing.T) {
	s := testStore(t)
	for _, o := range []Origin{{}, {Kind: "person"}, {Kind: "per son", ID: "bo"}, {Kind: "person", ID: "b\no"}} {
		if err := s.EditConfigBy("project", "environments", "dev", o, lnCfgByWrite("First.")); err == nil {
			t.Fatalf("origin %+v was accepted", o)
		}
	}
	if _, err := s.ReadConfig("project", "environments", "dev"); !os.IsNotExist(err) {
		t.Fatalf("a refused author wrote the entry: %v", err)
	}
}
