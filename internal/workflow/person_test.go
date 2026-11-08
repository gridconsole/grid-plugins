package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPersonVerbs(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       map[string][]string
		bad        bool
	}{
		{"legacy", "# Workflow", nil, false},
		{"list", "---\nperson_verbs: [deliver, task attach]\n---", map[string][]string{"deliver": nil, "task attach": nil}, false},
		{"roles", "---\nperson_verbs: {deliver: [owner], task attach: []}\n---", map[string][]string{"deliver": {"owner"}, "task attach": {}}, false},
		{"agent", "---\nperson_verbs: {deliver: [agent]}\n---", nil, true},
		{"wrong_type", "---\nperson_verbs: false\n---", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := PersonVerbs(dir)
			if (err != nil) != tc.bad || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}
