package usage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestUsageNoticeInitializesTaskIdentity(t *testing.T) {
	s := noticeStore(t)
	if err := os.WriteFile(filepath.Join(s.TaskPath, "task.md"), []byte("---\nid: T-01\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	configure(t, s, Change{Budget: true, Dollars: []float64{200}})
	out := post(t, s, 100, 0)
	id, err := feed.TaskID(s.TaskPath)
	if err != nil || !feed.IsID(id) || len(out.Notices) != 1 || out.Notices[0].Task != id || out.Notices[0].Body["task"] != id {
		t.Fatalf("notice identity: %q, %+v, %v", id, out.Notices, err)
	}
}
