package account

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Row 5: one folder, or one login, gives one account row. The two names the
// owner's Mac showed are claude-abdu-egal and claude-abdu-egal-gmail-com.

const (
	udIDShort = "claude-abdu-egal"
	udIDLong  = "claude-abdu-egal-gmail-com"
	udLogin   = "abdu.egal@gmail.com"
)

func udRecord(env Env, id, dir, email string) Record {
	return Record{ID: id, Kind: Claude, Name: email, Email: email, Dir: env.tilde(dir), Added: "person"}
}

func udSaveRegistry(t *testing.T, env Env, recs ...Record) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(registryPath(env)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveRecords(env, recs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(registryPath(env))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Two records for one login, each with a folder of its own, give one row in the
// account list and one in the windows.
func TestUdTwoRecordsOfOneLoginGiveOneRow(t *testing.T) {
	m := udMachine(t)
	short, long := m.folder("."+udIDShort), m.folder("."+udIDLong)
	login(t, short, udLogin)
	login(t, long, udLogin)
	before := udSaveRegistry(t, m.env, udRecord(m.env, udIDShort, short, udLogin), udRecord(m.env, udIDLong, long, udLogin))
	m.usage(t, long, udSignedIn)

	list := status(t, m.env, false)
	if len(list) != 1 || list[0].ID != udIDShort || list[0].Email != udLogin {
		t.Fatalf("one login is one account row: %+v", list)
	}
	rows := m.windows(t, "")
	if len(rows) != 1 || rows[0].ID != udIDShort {
		t.Fatalf("one login is one windows row: %+v", rows)
	}
	udWant(t, rows[0], "direct", Window{"5h", 53, udUnix(t, "2026-11-11T14:00:00Z")}, Window{"7d", 21.5, udUnix(t, "2026-11-15T09:30:00Z")})
	if after, _ := os.ReadFile(registryPath(m.env)); !bytes.Equal(before, after) {
		t.Fatalf("a read folds the rows and does not rewrite the registry:\n%s\n%s", before, after)
	}
	// The folded-away id still answers, with the one row.
	one := m.windows(t, udIDLong)
	if len(one) != 1 || one[0].Source != "direct" {
		t.Fatalf("the folded id resolves: %+v", one)
	}
}

// The record whose own folder is signed in is the one row, whatever its place in the registry.
func TestUdTheSignedInFolderRepresentsTheLogin(t *testing.T) {
	m := udMachine(t)
	short, long := m.folder("."+udIDShort), m.folder("."+udIDLong)
	write(t, filepath.Join(short, "keep"), "") // never signed in
	login(t, long, udLogin)
	udSaveRegistry(t, m.env, udRecord(m.env, udIDShort, short, udLogin), udRecord(m.env, udIDLong, long, udLogin))
	list := status(t, m.env, false)
	if len(list) != 1 || list[0].ID != udIDLong || list[0].SignedIn == nil || !*list[0].SignedIn {
		t.Fatalf("%+v", list)
	}
}

// A folder with the login of a registered account, that no record names, adds
// no row of its own: its reading is the account's.
func TestUdUnregisteredFolderOfARegisteredLoginAddsNoRow(t *testing.T) {
	m := udMachine(t)
	short, long := m.folder("."+udIDShort), m.folder("."+udIDLong)
	login(t, short, udLogin)
	login(t, long, udLogin)
	udSaveRegistry(t, m.env, udRecord(m.env, udIDShort, short, udLogin))
	m.usage(t, long, udSignedIn)

	rows := m.windows(t, "")
	if len(rows) != 1 || rows[0].ID != udIDShort || rows[0].Source != "direct" || len(rows[0].Windows) != 2 {
		t.Fatalf("one row, read from the other folder of the same login: %+v", rows)
	}
}

// Two records that name one folder give one row.
func TestUdTwoRecordsOfOneFolderGiveOneRow(t *testing.T) {
	m := udMachine(t)
	dir := m.folder("." + udIDShort)
	login(t, dir, udLogin)
	udSaveRegistry(t, m.env, udRecord(m.env, udIDShort, dir, udLogin), udRecord(m.env, udIDLong, dir, "other@example.com"))
	list := status(t, m.env, false)
	if len(list) != 1 || list[0].ID != udIDShort {
		t.Fatalf("one folder is one row: %+v", list)
	}
	if rows := m.windows(t, ""); len(rows) != 1 {
		t.Fatalf("%+v", rows)
	}
}

// Other logins, and the same email on another kind, stay apart.
func TestUdDifferentLoginsAndKindsStayApart(t *testing.T) {
	m := udMachine(t)
	m.env.ProbeTimeout = time.Second // the fake Codex here has nothing to answer with
	a, b := m.folder(".claude-a"), m.folder(".claude-b")
	login(t, a, "a@example.com")
	login(t, b, "b@example.com")
	write(t, filepath.Join(m.folder(".codex"), "auth.json"), `{"tokens":{"id_token":"`+idToken("a@example.com")+`"}}`)
	udSaveRegistry(t, m.env,
		udRecord(m.env, "claude-a", a, "a@example.com"),
		udRecord(m.env, "claude-b", b, "b@example.com"),
		Record{ID: "codex-a", Kind: Codex, Name: "a@example.com", Email: "a@example.com", Dir: m.env.tilde(m.folder(".codex")), Added: "auto"})
	if list := status(t, m.env, false); len(list) != 3 {
		t.Fatalf("three logins: %+v", list)
	}
	if rows := m.windows(t, ""); len(rows) != 3 {
		t.Fatalf("three rows: %+v", rows)
	}
}
