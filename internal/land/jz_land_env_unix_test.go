//go:build unix

package land

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// jzLdLand runs one land of the fixture with a lands.md rule that names env
// [JZ_MARK] and returns what command wrote to outcome. The rule is written before
// the approval, which hashes it.
func jzLdLand(t *testing.T, command string, getenv func(string) string) string {
	t.Helper()
	s, _, repo := jzLdStore(t, command)
	s.Getenv = getenv
	approve(t, s)
	r, err := s.Run(context.Background(), repo, "T-01", false)
	if err != nil || r.Status != "landed" {
		t.Fatalf("result %+v: %v", r, err)
	}
	b, err := os.ReadFile(filepath.Join(repo, "outcome"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func jzLdStore(t *testing.T, command string) (Store, string, string) {
	t.Helper()
	s, dir, repo := fixture(t)
	// Quoting through JSON is also valid YAML string syntax.
	quoted, _ := json.Marshal(command)
	write(t, filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md"),
		"---\nlands:\n  target: local\n  branch: main\n  command: "+string(quoted)+"\n  needs-approve: true\n  env: [JZ_MARK]\n---\n")
	return s, dir, repo
}

const jzLdMarkCommand = `printf '%s' "$JZ_MARK" > outcome`

func TestJzLdLandEnvFromCall(t *testing.T) {
	t.Setenv("JZ_MARK", "host")
	call := func(name string) string {
		if name == "JZ_MARK" {
			return "call"
		}
		return os.Getenv(name)
	}
	if got := jzLdLand(t, jzLdMarkCommand, call); got != "call" {
		t.Fatalf("the command saw JZ_MARK=%q, want the call's value", got)
	}
}

func TestJzLdLandEnvNilReadsHost(t *testing.T) {
	t.Setenv("JZ_MARK", "host")
	if got := jzLdLand(t, jzLdMarkCommand, nil); got != "host" {
		t.Fatalf("the command saw JZ_MARK=%q, want the host's value", got)
	}
}

// The Call answers only JZ_MARK, so a variable the host has and the Call has not
// (HOME here) never reaches the command, though it is on the allow list.
func TestJzLdLandEnvHostValueDoesNotLeak(t *testing.T) {
	t.Setenv("JZ_MARK", "host")
	t.Setenv("HOME", "/jz/host-home")
	call := func(name string) string {
		if name == "JZ_MARK" {
			return "call"
		}
		return ""
	}
	if got := jzLdLand(t, `printf '%s|%s' "$HOME" "$JZ_MARK" > outcome`, call); got != "|call" {
		t.Fatalf("the command saw HOME|JZ_MARK = %q, want |call", got)
	}
}

// The land runs end to end with the Call's PATH, not the host's: sh itself is
// found through the process PATH, and the command reads what the Call gave it.
func TestJzLdLandEnvPathFromCall(t *testing.T) {
	t.Setenv("JZ_MARK", "host")
	call := func(name string) string {
		switch name {
		case "JZ_MARK":
			return "call"
		case "PATH":
			return "/jz/call-path"
		}
		return ""
	}
	if got := jzLdLand(t, `printf '%s|%s' "$PATH" "$JZ_MARK" > outcome`, call); got != "/jz/call-path|call" {
		t.Fatalf("the command saw PATH|JZ_MARK = %q", got)
	}
}
