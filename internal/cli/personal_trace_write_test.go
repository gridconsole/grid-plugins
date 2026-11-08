package cli

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestJobTracePersonalCallerPolicy(t *testing.T) {
	for _, role := range []string{"member", "owner"} {
		for _, selector := range []string{"", "personal", "01PERSONAL"} {
			for _, json := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/json=%t", role, selector, json), func(t *testing.T) {
					h, cwd, personal := bypassFixture(t)
					h.record(project.Record{Name: "personal", ID: "01PERSONAL"})
					h.write(filepath.Join(personal, "jobs", "J-01", "inputs.json"), `{"sources":[{"pointer":"secret.md"}]}`)
					h.env.Getenv = func(key string) string {
						return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "sam"}[key]
					}
					run := func(pointer, project string) (int, string, string) {
						args := []string{"job", "trace", pointer}
						if project != "" {
							args = append(args, "--project", project)
						}
						if json {
							args = append(args, "--json")
						}
						code := h.run(cwd, args...)
						return code, h.out.String(), h.errb.String()
					}
					missingSelector := ""
					if selector != "" {
						missingSelector = "unknown"
					}
					missingCode, missingOut, missingErr := run("unknown.md", missingSelector)
					code, out, stderr := run("secret.md", selector)
					if role == "member" {
						if code != missingCode || out != missingOut || stderr != missingErr {
							t.Fatalf("member Personal trace differs from unknown: exit=%d want=%d out=%q err=%q", code, missingCode, out, stderr)
						}
					} else if code != 0 || stderr != "" || !strings.Contains(out, "J-01") || !strings.Contains(out, "T-01") {
						t.Fatalf("owner cannot trace Personal task: exit=%d out=%q err=%q", code, out, stderr)
					}
				})
			}
		}
	}
}

func TestWorkflowWritePersonalCallerPolicy(t *testing.T) {
	for _, role := range []string{"member", "owner"} {
		for _, selector := range []string{"personal", "01PERSONAL"} {
			t.Run(role+"/"+selector, func(t *testing.T) {
				h := newHarness(t)
				h.record(project.Record{Name: "personal", ID: "01PERSONAL"})
				text := strings.ReplaceAll(cliWorkflow, "nothing else matches", "label personal")
				h.write(filepath.Join(h.gridHome, "projects", "personal", "workflows.md"), text)
				rev := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))[:12]
				changed := strings.ReplaceAll(text, "review rounds: 2", "review rounds: 3")
				before := bypassTree(t, h.gridHome)
				h.env.Getenv = func(key string) string {
					return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "sam"}[key]
				}
				run := func(project string) (int, string, string) {
					h.env.Stdin = strings.NewReader(changed)
					code := h.run(h.root, append(workflowWriteArgs(rev), "--project", project)...)
					return code, h.out.String(), h.errb.String()
				}
				missingCode, missingOut, missingErr := run("unknown")
				code, out, stderr := run(selector)
				if role == "member" {
					if code != 1 || code != missingCode || out != missingOut || stderr != missingErr {
						t.Errorf("member Personal write differs from unknown: exit=%d want=%d out=%q err=%q", code, missingCode, out, stderr)
					}
					if !reflect.DeepEqual(before, bypassTree(t, h.gridHome)) {
						t.Error("member changed Personal workflow files")
					}
				} else {
					if code != 0 || stderr != "" {
						t.Fatalf("owner cannot write Personal workflow: exit=%d out=%q err=%q", code, out, stderr)
					}
					files := bypassTree(t, h.gridHome)
					if files[filepath.Join(h.gridHome, "projects", "personal", "workflows.md")] != changed {
						t.Error("owner workflow change was not saved")
					}
				}
			})
		}
	}
}
