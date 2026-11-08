//go:build linux || darwin

package lease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadJobIsReadOnlyAndRequiresMatchingBeat(t *testing.T) {
	task := t.TempDir()
	s, err := ReadJob(task, "J-1")
	if err != nil || s.Known || s.Expired(0, time.Now()) {
		t.Fatalf("missing: %+v %v", s, err)
	}
	entries, err := os.ReadDir(task)
	if err != nil || len(entries) != 0 {
		t.Fatal("read created storage")
	}
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Use the contract fixtures as the authoritative wire shapes.
	grant, err := os.ReadFile("../../contract/fixtures/lease/valid/alternate.json")
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	path := filepath.Join(task, "lease", "J-1.json")
	write(path, grant)
	s, err = ReadJob(task, "J-1")
	if err != nil || !s.Known || s.Expired(2, time.Now()) {
		t.Fatalf("missing beat: %+v %v", s, err)
	}
	if !s.Expired(1, time.Now()) {
		t.Fatal("superseded fence was not expired")
	}
	beat, err := os.ReadFile("../../contract/fixtures/heartbeat/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(task, ".live", "worker@01K9J000000000000000000002.2.json")
	write(live, beat)
	s, err = ReadJob(task, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Expired(2, s.Until.Add(-time.Second)) || !s.Expired(2, s.Until) {
		t.Fatal("incorrect expiry boundary")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(grant) {
		t.Fatal("read changed the grant")
	}
	after, _ = os.ReadFile(live)
	if string(after) != string(beat) {
		t.Fatal("read changed the heartbeat")
	}
	if err := json.Unmarshal(beat, &fields); err != nil {
		t.Fatal(err)
	}
	fields["fence"] = 3
	beat, _ = json.Marshal(fields)
	write(live, beat)
	if _, err := ReadJob(task, "J-1"); err == nil {
		t.Fatal("accepted another tenure's heartbeat")
	}
	// Malformed records and symlinks fail closed; they never become free leases.
	write(path, []byte("{"))
	if _, err := ReadJob(task, "J-1"); err == nil {
		t.Fatal("accepted malformed grant")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(task, "target")
	write(target, grant)
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadJob(task, "J-1"); err == nil {
		t.Fatal("followed grant symlink")
	}
}
