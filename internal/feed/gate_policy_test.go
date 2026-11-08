package feed

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGateRolePolicy(t *testing.T) {
	for _, role := range []string{"owner", "member", "lead", "cli", "agent", "worker"} {
		for _, kind := range []string{"person", "timer", "ci", "job"} {
			t.Run(role+"/"+kind, func(t *testing.T) {
				s, w := testStore(t), testWriter()
				w.Author.Role = role
				in := gatedQuestion("Q-1")
				gate := map[string]any{"id": "Q-1", "kind": kind}
				switch kind {
				case "timer":
					gate["at"] = "2000-01-01T00:00:00.000Z"
				case "ci":
					gate["check"] = "tests"
				case "job":
					gate["job"], gate["state"] = "J-1", "accepted"
				}
				in.Body["gate"] = gate
				denied := (role == "agent" || role == "worker") && (kind == "person" || kind == "timer")
				_, err := s.Post(in, w)
				if denied {
					var invalid *InvalidRecordError
					if !errors.As(err, &invalid) || !errors.Is(err, ErrInvalidRecord) {
						t.Fatalf("not final rejection: %v", err)
					}
					if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
						t.Fatalf("refusal wrote Feed: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				// Every kind can replace the default, including revoking person/CLI access.
				if err := os.WriteFile(filepath.Join(s.TaskDir, "workflow.md"), []byte("---\ngate_post_roles:\n  "+kind+": []\n---\n"), 0600); err != nil {
					t.Fatal(err)
				}
				gate["id"] = "Q-2"
				if _, err := s.Post(in, w); !errors.Is(err, ErrInvalidRecord) {
					t.Fatalf("empty policy admitted %s: %v", role, err)
				}
				// All roles admitted by the question schema can be enabled by workflow.
				if err := os.WriteFile(filepath.Join(s.TaskDir, "workflow.md"), []byte("---\ngate_post_roles:\n  "+kind+": ["+role+"]\n---\n"), 0600); err != nil {
					t.Fatal(err)
				}
				mustPost(t, s, in, w)
			})
		}
	}
}

func TestGatePolicyInvalidWritesNothing(t *testing.T) {
	s := testStore(t)
	if err := os.WriteFile(filepath.Join(s.TaskDir, "workflow.md"), []byte("---\ngate_post_roles: {person: typo}\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(gatedQuestion("Q-1"), testWriter()); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote Feed: %v", err)
	}
	// Ordinary questions do not consume a gate and retain their existing rules.
	in := gatedQuestion("")
	delete(in.Body, "gate")
	mustPost(t, s, in, testWriter())
}
