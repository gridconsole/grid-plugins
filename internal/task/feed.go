package task

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// FeedIdentity reads the stable Feed id and optional owner from an already
// resolved task folder. Task.Read still serves the legacy display-id record;
// both id: ULID and id: T-NN with ulid: ULID use the existing Feed reader.
func FeedIdentity(dir string) (id, owner string, err error) {
	id, err = feed.TaskID(dir)
	if err != nil {
		return
	}
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return "", "", err
	}
	if fm := taskfile.ScalarFrontmatter(string(b)); fm != nil {
		owner, _ = fm.Data["owner"].(string)
	}
	return
}
