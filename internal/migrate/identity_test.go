package migrate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestMigratedTaskIdentity(t *testing.T) {
	const stable = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "allocate", true: "preserve"}[existing], func(t *testing.T) {
			from, home := fixture(t)
			if existing {
				name := "alpha/projects/widget/tasks/T-07-first/task.md"
				data := read(t, from, name)
				put(t, filepath.Join(from, filepath.FromSlash(name)), strings.Replace(data, "id: T-07", "id: T-07\nulid: "+stable, 1))
			}
			p := scan(t, from, home)
			if _, err := p.Apply(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(home, "projects", "widget", "tasks", "T-07-first")
			id, err := feed.TaskID(dir)
			if err != nil || !feed.IsID(id) || existing && id != stable {
				t.Fatalf("migrated identity: %q, %v", id, err)
			}
			if _, err := scan(t, from, home).Apply(); err != nil {
				t.Fatal(err)
			}
			again, err := feed.TaskID(dir)
			if err != nil || again != id {
				t.Fatalf("second migration changed identity: %q, %v", again, err)
			}
		})
	}
}
