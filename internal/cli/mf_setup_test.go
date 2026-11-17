//go:build unix

package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// mfSnapshot records every path under root: a file by mode, size and time, a folder
// by mode only (a folder's own time moves when a child is made).
func mfSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			snap[rel] = "dir " + info.Mode().String()
		} else {
			snap[rel] = fmt.Sprintf("file %s %d %s", info.Mode(), info.Size(), info.ModTime())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// mfChanged lists the paths that were added, removed or changed between two snapshots.
func mfChanged(before, after map[string]string) []string {
	var out []string
	for p, v := range after {
		if before[p] != v {
			out = append(out, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// fakeSudo puts programs named like the ones setup prints on the PATH. Each one
// leaves a marker, so a setup that ran any of them is caught.
func fakeSudo(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"sudo", "usermod", "groupadd", "chgrp", "chmod", "mkdir"} {
		script := "#!/bin/sh\necho \"" + name + " $*\" >> '" + marker + "'\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func (h *mfHarness) readConfig() map[string]any {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.home, ".config", "grid", "config.json"))
	if err != nil {
		h.t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		h.t.Fatalf("config is not JSON: %v\n%s", err, b)
	}
	return doc
}

// Row 3: with the folder missing, setup prints the commands, runs none of them, and
// writes the config and nothing else.
func TestMfSetupSharedGroupPrintsCommandsAndWritesOnlyTheConfig(t *testing.T) {
	h := newMfHarness(t)
	marker := fakeSudo(t)
	h.config(`{"theme":"dark"}`)
	h.vars[machinefolder.EnvDir] = h.folder // the folder does not exist
	before := mfSnapshot(t, h.root)

	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 0 {
		t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
	}
	out := h.out.String()
	want := []string{
		"sudo mkdir -p " + h.folder,
		"sudo chgrp " + mfGroup + " " + h.folder,
		"sudo chmod 2770 " + h.folder,
		"sudo usermod -aG " + mfGroup + " alice",
	}
	last := -1
	for _, c := range want {
		i := strings.Index(out, c)
		if i < 0 || i < last {
			t.Fatalf("output lacks %q in order:\n%s", c, out)
		}
		last = i
	}
	if !strings.Contains(out, "Grid runs no sudo") {
		t.Errorf("output does not say that Grid runs nothing:\n%s", out)
	}

	if _, err := os.Stat(marker); err == nil {
		b, _ := os.ReadFile(marker)
		t.Fatalf("setup ran a command: %s", b)
	}
	if _, err := os.Stat(h.folder); err == nil {
		t.Fatal("setup made the shared folder")
	}
	changed := mfChanged(before, mfSnapshot(t, h.root))
	// The config is written through gridconfig: the new file and the .v1 backup of the
	// old one, and no other path.
	wantChanged := []string{
		filepath.Join("home", ".config", "grid", "config.json"),
		filepath.Join("home", ".config", "grid", "config.json.v1"),
	}
	if !reflect.DeepEqual(changed, wantChanged) {
		t.Fatalf("setup changed %q, want only %q", changed, wantChanged)
	}
	backup, err := os.ReadFile(filepath.Join(h.home, ".config", "grid", "config.json.v1"))
	if err != nil || string(backup) != `{"theme":"dark"}` {
		t.Fatalf("the .v1 backup is not the old config: %q %v", backup, err)
	}

	cfg := h.readConfig()
	if cfg["v"] != float64(3) || cfg["workspaces"] == nil {
		t.Fatalf("config is not a version 3 gridconfig file: %v", cfg)
	}
	if cfg["theme"] != "dark" || cfg[machinefolder.KeySharedGroup] != mfGroup || cfg[machinefolder.KeyMachineFolder] != h.folder {
		t.Fatalf("config = %v", cfg)
	}
	// A second run changes nothing: the config is the same bytes.
	first, _ := os.ReadFile(filepath.Join(h.home, ".config", "grid", "config.json"))
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 0 {
		t.Fatalf("second run exit %d: %s", code, h.errb)
	}
	second, _ := os.ReadFile(filepath.Join(h.home, ".config", "grid", "config.json"))
	if string(first) != string(second) {
		t.Fatalf("config changed on the second run:\n%s\n%s", first, second)
	}
}

// With no config yet the folder of GRID_MACHINE_DIR and the group are recorded, and
// the config's folder is made for it.
func TestMfSetupSharedGroupMakesTheConfigWhenThereIsNone(t *testing.T) {
	h := newMfHarness(t)
	h.vars[machinefolder.EnvDir] = h.folder
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	// gridconfig writes a version 3 file: v and workspaces beside the two keys. There
	// was no old config, so there is no .v1 backup and no other file in the folder.
	cfg := h.readConfig()
	if len(cfg) != 4 || cfg["v"] != float64(3) || cfg["workspaces"] == nil ||
		cfg[machinefolder.KeySharedGroup] != mfGroup || cfg[machinefolder.KeyMachineFolder] != h.folder {
		t.Fatalf("config = %v", cfg)
	}
	if entries, err := os.ReadDir(filepath.Join(h.home, ".config", "grid")); err != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("config folder = %v %v, want only config.json", entries, err)
	}
	// A config that setup makes is private to the login.
	if info, err := os.Stat(filepath.Join(h.home, ".config", "grid", "config.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode: %v %v", info, err)
	}
	// Doctor then reads the same folder and group.
	if f := machinefolder.Resolve(machinefolder.Env{Home: h.home, Getenv: func(string) string { return "" }}); f.Path != h.folder || f.SharedGroup != mfGroup {
		t.Fatalf("resolved %+v", f)
	}
}

// A folder that exists and that the login can write in needs no commands.
func TestMfSetupSharedGroupFolderReady(t *testing.T) {
	h := newMfHarness(t)
	marker := fakeSudo(t)
	h.sharedFolder(os.ModeSetgid | 0o770)
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	out := h.out.String()
	if strings.Contains(out, "sudo mkdir") || strings.Contains(out, "sudo chgrp") {
		t.Fatalf("a ready folder was given make commands:\n%s", out)
	}
	if !strings.Contains(out, "grid doctor") {
		t.Errorf("output does not point at grid doctor:\n%s", out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("setup ran a command")
	}
	if entries, _ := os.ReadDir(h.folder); len(entries) != 0 {
		t.Fatalf("setup left %d entries in the folder", len(entries))
	}
}

// A group that does not exist starts the list with groupadd.
func TestMfSetupSharedGroupNewGroupStartsWithGroupadd(t *testing.T) {
	h := newMfHarness(t)
	h.vars[machinefolder.EnvDir] = h.folder
	if code := h.run(h.tmp, "setup", "--shared-group", "newgroup"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), "sudo groupadd newgroup") {
		t.Fatalf("no groupadd:\n%s", h.out)
	}
}

// With no GRID_MACHINE_DIR and no machine_folder the folder is the login's own
// standard place; setup says so, because a login's own folder is not shared.
func TestMfSetupSharedGroupSaysWhenTheFolderIsTheLoginsOwn(t *testing.T) {
	h := newMfHarness(t)
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), machinefolder.EnvDir) {
		t.Fatalf("output does not point at %s:\n%s", machinefolder.EnvDir, h.out)
	}
}

func TestMfSetupSharedGroupJSON(t *testing.T) {
	h := newMfHarness(t)
	h.vars[machinefolder.EnvDir] = h.folder
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, h.out)
	}
	for _, key := range []string{"group", "path", "config", "ready", "commands"} {
		if _, ok := got[key]; !ok {
			t.Errorf("JSON lacks %q: %v", key, got)
		}
	}
	if got["group"] != mfGroup || got["path"] != h.folder || got["ready"] != false {
		t.Fatalf("JSON = %v", got)
	}
	cmds, _ := got["commands"].([]any)
	if len(cmds) != 4 || cmds[0] != "sudo mkdir -p "+h.folder {
		t.Fatalf("commands = %v", got["commands"])
	}
}

// A bad call exits 2 and writes nothing.
func TestMfSetupSharedGroupRefusals(t *testing.T) {
	for _, args := range [][]string{
		{"setup", "--shared-group", ""},
		{"setup", "--shared-group", "two words"},
		{"setup", "--shared-group", "grid;reboot"},
		{"setup", "--shared-group", "$(id)"},
		{"setup", "--shared-group", "-grid"},
		{"setup", "--shared-group", mfGroup, "example"},
		{"setup", "--shared-group", mfGroup, "--yes"},
		{"setup", "--shared-group", mfGroup, "--remove"},
		{"setup", "--shared-group", mfGroup, "--answers", "-"},
		{"setup", "--shared-group"},
	} {
		h := newMfHarness(t)
		h.vars[machinefolder.EnvDir] = h.folder
		before := mfSnapshot(t, h.root)
		if code := h.run(h.tmp, args...); code != 2 {
			t.Errorf("%q exit %d, want 2: %s%s", args, code, h.out, h.errb)
		}
		if changed := mfChanged(before, mfSnapshot(t, h.root)); len(changed) != 0 {
			t.Errorf("%q wrote %q", args, changed)
		}
	}
}

// A config that is not a JSON object is a person's edit: setup stops and keeps it.
func TestMfSetupSharedGroupKeepsABrokenConfig(t *testing.T) {
	h := newMfHarness(t)
	h.config("{broken")
	h.vars[machinefolder.EnvDir] = h.folder
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, h.out)
	}
	if b, _ := os.ReadFile(filepath.Join(h.home, ".config", "grid", "config.json")); string(b) != "{broken" {
		t.Fatalf("config changed: %q", b)
	}
}

func TestMfSetupSharedGroupHelp(t *testing.T) {
	h := newMfHarness(t)
	if code := h.run(h.tmp, "setup", "--shared-group", mfGroup, "--help"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if !strings.HasPrefix(h.out.String(), "Usage: grid setup --shared-group <group>") {
		t.Fatalf("usage = %q", h.out)
	}
}
