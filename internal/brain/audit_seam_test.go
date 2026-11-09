package brain

import "testing"

// The host policy decides per level: false gives no audit_hash finding there.
func TestAuditHashCheckSeamIsPerLevel(t *testing.T) {
	real := AuditHashCheck
	t.Cleanup(func() { AuditHashCheck = real })
	s := testStore(t)
	for _, level := range []string{"home", "project"} {
		chkPut(t, Store{GridHome: s.GridHome}, levelDir(level)+"knowledge/decisions/e1.md", "current bytes\n", 0o600)
		chkPut(t, Store{GridHome: s.GridHome}, levelDir(level)+"knowledge/log.md", "# Brain log\n\n"+accLine(auditAt, 1, "sha256:0123456789abcdef"), 0o600)
	}
	AuditHashCheck = func(level string) bool { return level != "project" }
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	got := auditCodes(r, "audit_hash")
	if len(got) != 1 || got[0].File != "brain:home/knowledge/decisions/e1.md" {
		t.Fatalf("with project off: %+v", got)
	}
	AuditHashCheck = real
	r, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if got := auditCodes(r, "audit_hash"); len(got) != 2 {
		t.Fatalf("default checks both levels: %+v", got)
	}
}

func levelDir(level string) string {
	if level == "project" {
		return "projects/example/"
	}
	return ""
}
