package brain

import (
	"strings"
	"testing"
	"time"
)

func newestChange(t *testing.T, s Store) Change {
	t.Helper()
	got, err := s.Changes("project", time.Time{}, 0)
	if err != nil || len(got) == 0 {
		t.Fatalf("changes: %+v, %v", got, err)
	}
	return got[0]
}

func TestRevertWithReasonLogsIt(t *testing.T) {
	s := threeVersions(t)
	res, err := s.RevertWith("project", "conventions", "checks", 1, keeper, RevertOptions{Reason: "v2 claim was wrong"})
	if err != nil || !res.Wrote {
		t.Fatalf("RevertWith: %+v, %v", res, err)
	}
	c := newestChange(t, s)
	if c.Status != "reverted" || c.Version != res.Version || c.Reason != "revert to v1: v2 claim was wrong" {
		t.Fatalf("change: %+v", c)
	}
}

func TestRevertWithoutReasonKeepsTheText(t *testing.T) {
	for _, opts := range []RevertOptions{{}, {Reason: "  \n\t "}} {
		s := threeVersions(t)
		if _, err := s.RevertWith("project", "conventions", "checks", 1, keeper, opts); err != nil {
			t.Fatal(err)
		}
		if c := newestChange(t, s); c.Reason != "revert to v1" {
			t.Fatalf("reason %q for %+v", c.Reason, opts)
		}
	}
}

func TestRevertReasonIsOneCleanAuditLine(t *testing.T) {
	s := threeVersions(t)
	if _, err := s.RevertWith("project", "conventions", "checks", 1, keeper, RevertOptions{Reason: "-x | y\r\n\tz  w"}); err != nil {
		t.Fatal(err)
	}
	c := newestChange(t, s)
	if c.Reason != "revert to v1: -x / y z w" || c.Status != "reverted" || c.Topic != "conventions" || c.Name != "checks" ||
		c.OriginKind != keeper.Kind || c.OriginID != keeper.ID || !strings.HasPrefix(c.Hash, "sha256:") {
		t.Fatalf("change: %+v", c)
	}
	lines := strings.Split(strings.TrimSpace(readLog(t, s)), "\n")
	last := lines[len(lines)-1]
	if strings.Count(last, " | ") != 7 {
		t.Fatalf("not one 8-field line: %q", last)
	}
	got, _, ok := parseAudit(last)
	if !ok || got != c {
		t.Fatalf("parseAudit: %+v %v, want %+v", got, ok, c)
	}
}

func TestRevertWithReasonOnNoopWritesNothing(t *testing.T) {
	s := threeVersions(t)
	before := snapshot(t, s)
	res, err := s.RevertWith("project", "conventions", "checks", 3, keeper, RevertOptions{Reason: "no change"})
	if err != nil || res.Wrote {
		t.Fatalf("RevertWith: %+v, %v", res, err)
	}
	wantSame(t, s, before)
}
