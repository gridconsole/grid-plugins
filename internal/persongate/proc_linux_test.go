//go:build linux

package persongate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLinuxExeFallback(t *testing.T) {
	root := t.TempDir()
	pid := os.Getpid()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(fmt.Sprintf("%d (grid with ) parens) S 0 0 0", pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte("codex\x00run\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	tree, err := linuxSnapshot(root)
	if err != nil || tree[pid].Exe != "codex" || len(tree[pid].Args) != 2 {
		t.Fatalf("%+v %v", tree, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := linuxSnapshot(root); err == nil {
		t.Fatal("unreadable exe without cmdline passed")
	}
}

func TestLinuxSSHSession(t *testing.T) {
	for _, tc := range []struct {
		name, reason, via string
		commReadable      bool
	}{
		{name: "comm", commReadable: true, via: "terminal"},
		{name: "unreadable_comm", reason: "detached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			procRoot := t.TempDir()
			pid := os.Getpid()
			for i, p := range []struct {
				name, cmdline string
			}{
				{"grid", "grid\x00person\x00check\x00--json\x00"},
				{"bash", "bash\x00"},
				{"sshd-session", "sshd-session: grid@notty\x00"},
				{"sshd", "sshd: listener\x00"},
				{"systemd", "systemd\x00"},
			} {
				id, parent := pid+i, pid+i+1
				if i == 3 {
					parent = 1
				} else if i == 4 {
					id, parent = 1, 0
				}
				dir := filepath.Join(procRoot, strconv.Itoa(id))
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				for file, data := range map[string]string{
					"stat":    fmt.Sprintf("%d (%s) S %d 0 0", id, p.name, parent),
					"cmdline": p.cmdline,
				} {
					if err := os.WriteFile(filepath.Join(dir, file), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				comm := filepath.Join(dir, "comm")
				if !tc.commReadable && (i == 2 || i == 3) {
					// A directory reliably makes ReadFile fail, even when run as root.
					if err := os.Mkdir(comm, 0700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(comm, []byte(p.name+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			tree, err := linuxSnapshot(procRoot)
			if err != nil {
				t.Fatal(err)
			}
			session := tree[pid+2]
			wantExe := "sshd-session: grid@notty"
			if tc.commReadable {
				wantExe = "sshd-session"
			}
			if session.Exe != wantExe {
				t.Errorf("session exe = %q, want %q", session.Exe, wantExe)
			}
			if len(session.Args) != 1 || session.Args[0] != "sshd-session: grid@notty" {
				t.Fatalf("session args = %q", session.Args)
			}
			r := request(t, "grid")
			r.PID, r.Procs = pid, fakeProcs{tree: tree}
			d, err := Check(r)
			if err != nil || d.Allow != (tc.reason == "") || d.Reason != tc.reason || d.Via != tc.via {
				t.Fatalf("decision %+v error %v; want reason=%q via=%q", d, err, tc.reason, tc.via)
			}
		})
	}
}
