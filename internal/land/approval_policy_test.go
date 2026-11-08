package land

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setNeedsApprove(t *testing.T, s Store, needs bool) {
	t.Helper()
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.Replace(string(b), "needs-approve: true", fmt.Sprintf("needs-approve: %t", needs), 1))
}

func TestStricterApprovalPolicyWins(t *testing.T) {
	for _, gate := range []string{"none", "plan", "ship"} {
		for _, needs := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/needs-approve=%t", gate, needs), func(t *testing.T) {
				s, dir, repo := fixture(t)
				setWorkflow(t, s, dir, gate)
				setNeedsApprove(t, s, needs)
				wantGate := gate
				if gate == "none" && needs {
					wantGate = "plan"
				}
				dry, err := s.Run(context.Background(), repo, "T-01", true)
				if err != nil || dry.Plan.Gate != wantGate {
					t.Errorf("dry-run gate = %q, want %q: %v", dry.Plan.Gate, wantGate, err)
				}
				_, err = s.Run(context.Background(), repo, "T-01", false)
				if wantGate == "none" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if !errors.Is(err, ErrRejected) {
						t.Errorf("missing approval was not rejected: %v", err)
					}
					if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
						t.Fatalf("command ran without approval: %v", err)
					}
					if wantGate == "ship" {
						b, err := os.ReadFile(filepath.Join(dir, "task.md"))
						if err != nil {
							t.Fatal(err)
						}
						write(t, filepath.Join(dir, "task.md"), strings.Replace(string(b), "state: waiting", "gate: ship\nstate: waiting", 1))
					}
					approve(t, s)
					if _, err := s.Run(context.Background(), repo, "T-01", false); err != nil {
						t.Fatalf("valid approval: %v", err)
					}
				}
				if b, err := os.ReadFile(filepath.Join(repo, "outcome")); err != nil || string(b) != "landed" {
					t.Fatalf("approved command outcome = %q: %v", b, err)
				}
			})
		}
	}
}
