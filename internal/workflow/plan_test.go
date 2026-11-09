package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanHashToken(t *testing.T) {
	for _, tc := range []struct {
		name, text, want string
	}{
		{"missing", "", "any"},
		{"absent-key", "---\nname: review\n---\n", "any"},
		{"any", "---\nplan_hash_token: any\n---\n", "any"},
		{"digest", "---\nplan_hash_token: digest\n---\n", "digest"},
		{"crlf", "---\r\nplan_hash_token: digest\r\n---\r\n", "digest"},
		{"case", "---\nplan_hash_token: Digest\n---\n", ""},
		{"algorithm", "---\nplan_hash_token: sha256\n---\n", ""},
		{"empty", "---\nplan_hash_token:\n---\n", ""},
		{"quoted-empty", "---\nplan_hash_token: ''\n---\n", ""},
		{"boolean", "---\nplan_hash_token: true\n---\n", ""},
		{"list", "---\nplan_hash_token: [digest]\n---\n", ""},
		{"broken", "---\nplan_hash_token: [\n---\n", ""},
		{"unclosed", "---\nplan_hash_token: digest\n", ""},
		{"no-frontmatter", "# Workflow\n", ""},
		{"duplicate", "---\nplan_hash_token: digest\nplan_hash_token: any\n---\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.text != "" {
				if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte(tc.text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := PlanHashToken(dir)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("PlanHashToken = %q, %v; want %q", got, err, tc.want)
			}
			if err != nil && (err.Error() != "workflow.md: plan_hash_token must be any or digest" || strings.Contains(err.Error(), dir)) {
				t.Fatalf("unsafe or unclear error: %v", err)
			}
		})
	}
}

func TestPlanHashTokenReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "workflow.md"), 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := PlanHashToken(dir); got != "" || err == nil || err.Error() != "workflow.md: plan_hash_token must be any or digest" {
		t.Fatalf("read error must fail closed without a path: %q, %v", got, err)
	}
}
