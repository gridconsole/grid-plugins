package account

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// foldsCase reports whether the volume under dir treats .claude and .CLAUDE as
// one folder (a Mac's default volume). It leaves dir as it found it.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, ".fold-probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Stat(filepath.Join(dir, ".FOLD-PROBE"))
	return err == nil
}

func TestSignInFoldsTheAskedSpellingOnMacOS(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	env.GOOS = "darwin"
	own := filepath.Join(env.Home, ".claude-ann")
	write(t, filepath.Join(own, "keep"), "")
	var out bytes.Buffer
	asked := filepath.Join(env.Home, ".Claude-ann")
	code, err := SignIn(env, Claude, asked, strings.NewReader(""), &out, io.Discard)
	if err != nil || code != 0 {
		t.Fatalf("asked %s: code %d err %v", asked, code, err)
	}
	if !strings.Contains(out.String(), "login on "+own) {
		t.Fatalf("the provider must run on the listed spelling %s, got %q", own, out.String())
	}
	if strings.Contains(out.String(), "login on "+asked) {
		t.Fatalf("the provider ran on the asked spelling: %q", out.String())
	}
}

func TestSignInKeepsCaseApartOnLinux(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	env.GOOS = "linux"
	write(t, filepath.Join(env.Home, ".claude-ann", "keep"), "")
	asked := filepath.Join(env.Home, ".Claude-ann")
	code, err := SignIn(env, Claude, asked, strings.NewReader(""), io.Discard, io.Discard)
	if !errors.Is(err, ErrBadFolder) || code == 0 {
		t.Fatalf("linux compares as spelled: code %d err %v", code, err)
	}
}

func TestFolderListFoldsTheVariablesSpellingOnMacOS(t *testing.T) {
	env, _ := machine(t)
	env.GOOS = "darwin"
	write(t, filepath.Join(env.Home, ".claude", "keep"), "")
	variable := filepath.Join(env.Home, ".CLAUDE")
	if !foldsCase(t, env.Home) {
		write(t, filepath.Join(variable, "keep"), "")
	}
	env.Getenv = func(k string) string {
		if k == dirVars[Claude] {
			return variable
		}
		return ""
	}
	var claude []folderRef
	for _, r := range folderList(env) {
		if r.kind == Claude {
			claude = append(claude, r)
		}
	}
	if len(claude) != 1 {
		t.Fatalf("one claude folder, got %+v", claude)
	}
	if !claude[0].home || claude[0].dir != filepath.Join(env.Home, ".claude") {
		t.Fatalf("the first spelling and its home flag are kept: %+v", claude[0])
	}
}

func TestFolderListKeepsCaseApartOnLinux(t *testing.T) {
	env, _ := machine(t)
	env.GOOS = "linux"
	write(t, filepath.Join(env.Home, ".claude", "keep"), "")
	variable := filepath.Join(env.Home, ".CLAUDE")
	if foldsCase(t, env.Home) {
		t.Skip("this volume folds case, so .claude and .CLAUDE cannot both be made")
	}
	write(t, filepath.Join(variable, "keep"), "")
	env.Getenv = func(k string) string {
		if k == dirVars[Claude] {
			return variable
		}
		return ""
	}
	n := 0
	for _, r := range folderList(env) {
		if r.kind == Claude {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("two folders that differ only in case stay two on linux, got %d", n)
	}
}
