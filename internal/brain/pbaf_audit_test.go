package brain

import (
	"strings"
	"testing"
)

func pbAfFinding(res CheckResult, code, file, msg string) bool {
	for _, f := range res.Findings {
		if f.Code == code && f.File == file && strings.Contains(f.Message, msg) {
			return true
		}
	}
	return false
}

// Row 2: a person's edit that `check` adopts gives no drift finding, only the
// adopted count (5.3). The check is the one of audit_check.go.
func TestPbAfCheckAdoptsPersonEditWithoutDrift(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	pbAdEdit(t, s, "Notes/Plan.md", pbAdBase+"A person added this.\n")
	res, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if res.Adopted != 1 || len(res.Findings) != 0 {
		t.Fatalf("Check() = %+v", res)
	}
	if recs := pbAdOf(pbAdRecords(t, s), "edited", "Notes/Plan"); len(recs) != 1 || recs[0].OriginKind != "person" {
		t.Errorf("the edit is not recorded as a person's: %+v", recs)
	}
}

// Row 2: a change adoption cannot take in still gives a finding: a version whose
// bytes do not hash to its name, and a change record edited after it was written.
func TestPbAfCheckReportsUnadoptedGridChange(t *testing.T) {
	s := pbAdVault(t)
	if _, err := s.Adopt(); err != nil {
		t.Fatal(err)
	}
	if res, err := s.Check(); err != nil || len(res.Findings) != 0 {
		t.Fatalf("clean vault: %+v, %v", res, err)
	}
	vrel := versionRel("home", "Notes/Plan", NoteRev([]byte(pbAdBase)))
	pbVrWrite(t, s.BrainDir, vrel, "bytes that are not the version\n")
	res, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !pbAfFinding(res, CheckGridIntegrity, vrel, "do not hash to its file name") {
		t.Errorf("no finding for the changed version: %+v", res.Findings)
	}

	var rec recFile
	for _, r := range pbAdRecords(t, s) {
		if r.ID == "Notes/Other" {
			rec = r
		}
	}
	edited := strings.Replace(string(rec.raw), rec.Hash, strings.Repeat("0", len(rec.Hash)), 1)
	pbVrWrite(t, s.BrainDir, rec.name, edited)
	res, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !pbAfFinding(res, CheckGridIntegrity, rec.name, "change record was edited") {
		t.Errorf("no finding for the edited record: %+v", res.Findings)
	}
}
