package workflow

import (
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
			thresholds, usageErr := UsageNoticeThresholds(dir)
			roles, gateErr := GatePostRoles(dir)
			if kind == "at limit" {
				if usageErr != nil || gateErr != nil || len(thresholds["tokens"]) != 2 || len(roles["person"]) != 1 || roles["person"][0] != "agent" {
					t.Fatalf("valid file refused: %v %v %v %v", thresholds, usageErr, roles, gateErr)
				}
			} else if usageErr == nil || gateErr == nil || thresholds != nil || roles != nil {
				t.Fatalf("unsafe policy accepted: thresholds=%v err=%v roles=%v err=%v", thresholds, usageErr, roles, gateErr)
			}
		})
	}
}
