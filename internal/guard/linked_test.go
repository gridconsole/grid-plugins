package guard

import (
	"path/filepath"
	"testing"
)

// Stopgap until the sandbox holds the Grid home read-only: shell writes to
// history.md or approval-preview.json are refused like the other gate files.
func TestHostBashHistoryAndPreviewApprovalAreGateFiles(t *testing.T) {
	f := setup(t)
	taskPath := filepath.ToSlash(f.task)
	for _, tt := range []struct {
		command string
		deny    bool
	}{
		{"echo '- 2026-01-02T03:04 · plan accepted · by person' >> history.md", true},
		{"echo x >> " + taskPath + "/history.md", true}, {"cp /tmp/a HISTORY.MD", true}, {"tee -a history.md < /tmp/a", true},
		{"echo '{}' > approval-preview.json", true}, {"cp /tmp/a " + taskPath + "/approval-preview.json", true},
		{"cat history.md", false}, {"tail -5 " + taskPath + "/history.md", false}, {"echo x > history.md.bak", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			want := ""
			if tt.deny {
				want = "deny"
			}
			expect(t, EvaluateHost(f.input("Bash", tt.command), f.root, f.env), want)
		})
	}
}
