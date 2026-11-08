package env

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectEnvironmentConfig(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "projects", "example")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "project.md")
	for _, tc := range []struct {
		name, text string
		bad        bool
	}{
		{"configured", "environments:\n  command: serve --port {port}\n  health_url: http://127.0.0.1:{port}/health\n  timeout: 5s\n  port_range: [43000, 43199]\n", false},
		{"default range", "environments:\n  command: serve\n  health_url: http://127.0.0.1:{port}/health\n", false},
		{"bad range", "environments:\n  port_range: [3]\n", true},
		{"wrong shape", "environments: [dev]\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "---\nid: example-project\n" + tc.text + "---\nProject notes.\n"
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, rec, err := Load(home, "example")
			if tc.bad {
				if err == nil {
					t.Fatal("accepted invalid config")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rec.ID != "example-project" {
				t.Fatal(rec)
			}
			timeout, err := cfg.Validate()
			if err != nil {
				t.Fatal(err)
			}
			r, err := cfg.Ports()
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "configured" && (timeout != 5*time.Second || r.Start != 43000 || r.End != 43199) {
				t.Fatalf("%+v %v", r, timeout)
			}
			if tc.name == "default range" && (r.Start != 41000 || r.End != 41999) {
				t.Fatal(r)
			}
			after, _ := os.ReadFile(path)
			if string(after) != input {
				t.Fatal("config load wrote project.md")
			}
		})
	}
}
