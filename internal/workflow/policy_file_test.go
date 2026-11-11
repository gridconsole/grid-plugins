package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskPolicyFileLimits(t *testing.T) {
	for _, kind := range []string{"at limit", "oversize", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "workflow.md")
			text := "---\nusage_notice_thresholds: {tokens: [90, 100]}\ngate_post_roles: {person: [agent]}\n---\n"
			if strings.Contains(kind, "symlink") {
				target := filepath.Join(t.TempDir(), "policy.md")
				if kind == "symlink" {
					if err := os.WriteFile(target, []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			} else {
				size := 1 << 20
				if kind == "oversize" {
					size++
				}
				if err := os.WriteFile(path, []byte(text+strings.Repeat("x", size-len(text))), 0600); err != nil {
					t.Fatal(err)
				}
			}
						roles, gateErr := GatePostRoles(dir)
			if kind == "at limit" {
				if gateErr != nil || len(roles["person"]) != 1 || roles["person"][0] != "agent" {
					t.Fatalf("valid file refused: %v %v", roles, gateErr)
				}
			} else if gateErr == nil || roles != nil {
				t.Fatalf("unsafe policy accepted: roles=%v err=%v", roles, gateErr)
			} else {
				var pathErr *os.PathError
				if !errors.As(gateErr, &pathErr) {
					t.Fatalf("gate refusal lost filesystem error classification: %T", gateErr)
				}
			}
		})
	}
}
