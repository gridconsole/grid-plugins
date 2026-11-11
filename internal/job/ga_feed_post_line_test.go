package job_test

import (
	"strings"
	"testing"
)

func TestGaFeedPostLine(t *testing.T) {
	for _, role := range []string{"builder", "reviewer"} {
		got := renderRole(t, role)
		_, after, ok := strings.Cut(got.Brief, "### Instructions\n")
		if !ok {
			t.Fatalf("%s brief has no Instructions section", role)
		}
		for _, want := range []string{"feed_post", "grid job result"} {
			if !strings.Contains(after, want) {
				t.Fatalf("%s: Instructions do not name %s", role, want)
			}
		}
	}
}
