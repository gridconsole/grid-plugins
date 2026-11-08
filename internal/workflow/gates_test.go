package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGatePostRoles(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
		bad        bool
	}{
		{"missing", "", []string{"cli", "owner", "member", "lead"}, false},
		{"omitted", "---\nname: Ship\n---\n", []string{"cli", "owner", "member", "lead"}, false},
		{"override", "---\ngate_post_roles:\n  person: [agent, worker]\n---\n", []string{"agent", "worker"}, false},
		{"empty", "---\ngate_post_roles:\n  person: []\n---\n", []string{}, false},
		{"crlf", "---\r\ngate_post_roles: {person: [agent]}\r\n---\r\n", []string{"agent"}, false},
		{"no front matter", "gate_post_roles: {}", nil, true},
		{"unclosed", "---\ngate_post_roles: {}", nil, true},
		{"syntax", "---\ngate_post_roles: [\n---\n", nil, true},
		{"null map", "---\ngate_post_roles: null\n---\n", nil, true},
		{"scalar map", "---\ngate_post_roles: agent\n---\n", nil, true},
		{"unknown kind", "---\ngate_post_roles: {typo: [agent]}\n---\n", nil, true},
		{"unknown role", "---\ngate_post_roles: {person: [typo]}\n---\n", nil, true},
		{"schema role", "---\ngate_post_roles: {person: [guest]}\n---\n", nil, true},
		{"scalar roles", "---\ngate_post_roles: {person: agent}\n---\n", nil, true},
		{"null roles", "---\ngate_post_roles: {person: null}\n---\n", nil, true},
		{"number role", "---\ngate_post_roles: {person: [1]}\n---\n", nil, true},
		{"duplicate kind", "---\ngate_post_roles:\n  person: [agent]\n  person: []\n---\n", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.text != "" {
				if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte(tc.text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := GatePostRoles(dir)
			if (err != nil) != tc.bad {
				t.Fatalf("roles=%v err=%v", got, err)
			}
			if !tc.bad {
				if !reflect.DeepEqual(got["person"], tc.want) {
					t.Errorf("person=%v want %v", got["person"], tc.want)
				}
				if !reflect.DeepEqual(got["ci"], []string{"cli", "owner", "member", "lead", "agent", "worker"}) {
					t.Errorf("omitted kind lost defaults: %v", got)
				}
			}
		})
	}
}
