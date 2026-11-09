package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func workflowWriteArgs(rev string) []string {
	return []string{"workflow", "write", "--rev", rev, "--file", "-", "--json"}
}

func TestWorkflowWriteRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.env.Stdin = strings.NewReader(cliWorkflow)
	if code := h.run(h.root, workflowWriteArgs("none")...); code != 0 || h.errb.Len() != 0 {
		t.Fatalf("write exit %d: %s", code, h.errb)
	}
	var result map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantRev := fmt.Sprintf("%x", sha256.Sum256([]byte(cliWorkflow)))[:12]
	if len(result) != 2 || result["file"] != "workflows.md" || result["rev"] != wantRev {
		t.Fatalf("reply: %s", h.out)
	}
	if code := h.run(h.root, "workflow", "show", "--json"); code != 0 {
		t.Fatalf("show exit %d: %s", code, h.errb)
	}
	var shown workflow.Answer
	if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if len(shown.Files) != 1 || shown.Files[0].Rev == nil || *shown.Files[0].Rev != wantRev || shown.Files[0].Text != cliWorkflow || len(shown.Errors) != 0 {
		t.Fatalf("show: %s", h.out)
	}
	if code := h.run(h.root, "workflow", "show"); code != 0 || h.out.String() != "file: workflows.md\ntemplates: Default\nerrors: 0\n" {
		t.Fatalf("show golden: exit %d: %q", code, h.out)
	}
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	projectText := strings.ReplaceAll(cliWorkflow, "nothing else matches", "label alpha")
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), projectText)
	projectRev := fmt.Sprintf("%x", sha256.Sum256([]byte(projectText)))[:12]
	input := filepath.Join(h.root, "input.md")
	changed := strings.ReplaceAll(projectText, "review rounds: 2", "review rounds: 3")
	h.write(input, changed)
	if code := h.run(h.root, "workflow", "write", "--project", "id-alpha", "--rev", projectRev, "--file", input); code != 0 {
		t.Fatalf("project write exit %d: %s", code, h.errb)
	}
	wantRev = fmt.Sprintf("%x", sha256.Sum256([]byte(changed)))[:12]
	if h.out.String() != "alpha/workflows.md "+wantRev+"\n" {
		t.Fatalf("plain reply: %q", h.out)
	}
	data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"))
	if err != nil || string(data) != changed {
		t.Fatalf("project file: %q, %v", data, err)
	}
}

func TestWorkflowWriteRefusals(t *testing.T) {
	rev := fmt.Sprintf("%x", sha256.Sum256([]byte(cliWorkflow)))[:12]
	for _, tc := range []struct {
		name, rev, text, message string
		code                     int
	}{
		{"stale", "000000000000", cliWorkflow, "workflow changed since read", 3},
		{"none over existing", "none", cliWorkflow, "workflow changed since read", 3},
		{"parse", rev, strings.ReplaceAll(cliWorkflow, "review rounds: 2", "review rounds: 4"), "review rounds", 2},
		{"oversize", rev, strings.Repeat("x", 32*1024+1), "32768", 2},
		{"utf8", rev, cliWorkflow + "\xff", "UTF-8", 2},
		{"NUL", rev, cliWorkflow + "\x00", "NUL", 2},
		{"heading", rev, "# Other\n", "must start", 2},
		{"leading blank", rev, "\n" + cliWorkflow, "must start", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			h.write(filepath.Join(h.gridHome, "workflows.md"), cliWorkflow)
			h.env.Stdin = strings.NewReader(tc.text)
			if code := h.run(h.root, workflowWriteArgs(tc.rev)...); code != tc.code || !strings.Contains(h.errb.String(), tc.message) || strings.Contains(h.errb.String(), h.root) {
				t.Fatalf("exit %d: out=%s err=%s", code, h.out, h.errb)
			}
			errorCode := ""
			if tc.code == 2 {
				errorCode = CodeInvalidArgument
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), errorCode)
			data, err := os.ReadFile(filepath.Join(h.gridHome, "workflows.md"))
			if err != nil || string(data) != cliWorkflow {
				t.Fatalf("changed on refusal: %q, %v", data, err)
			}
		})
	}
}

func TestWorkflowWriteRoles(t *testing.T) {
	for _, role := range []string{"guest", "agent", "unknown", "member", "owner", ""} {
		t.Run(role, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(key string) string {
				if key == "GRID_ROLE" {
					return role
				}
				return ""
			}
			h.env.Stdin = strings.NewReader(cliWorkflow)
			code := h.run(h.root, workflowWriteArgs("none")...)
			allowed := role == "" || role == "owner" || role == "member"
			if (code == 0) != allowed {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if !allowed {
				entries, err := os.ReadDir(h.gridHome)
				if err != nil || len(entries) != 0 {
					t.Fatalf("refusal wrote files: %v, %v", entries, err)
				}
			}
		})
	}
}

func TestWorkflowWriteUsageAndPaths(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--rev", "none"}, {"--file", "-"}, {"--rev", "", "--file", "-"},
		{"--rev", "bad", "--file", "-"}, {"--rev", "00000000000z", "--file", "-"},
		{"--rev", "none", "--file", "-", "--project="},
		{"--rev", "none", "--file", "-", "extra"},
	} {
		h := newHarness(t)
		if code := h.run(h.root, append([]string{"workflow", "write"}, args...)...); code != 2 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	for _, selector := range []string{"missing", "../alpha", filepath.Join(h.root, "private")} {
		h.env.Stdin = strings.NewReader(cliWorkflow)
		code := h.run(h.root, append(workflowWriteArgs("none"), "--project", selector)...)
		if (code != 1 && code != 2) || h.out.Len() != 0 || strings.Contains(h.errb.String(), h.root) {
			t.Fatalf("selector: exit %d: %s %s", code, h.out, h.errb)
		}
	}
	if code := h.run(h.root, "workflow", "write", "--rev", "none", "--file", filepath.Join(h.root, "secret", "missing")); code == 0 || strings.Contains(h.errb.String(), h.root) {
		t.Fatalf("input path: exit %d: %s", code, h.errb)
	}
	if code := h.run(h.root, "workflow", "--help"); code != 0 || !strings.Contains(h.out.String(), "workflow write") {
		t.Fatalf("help: exit %d: %s", code, h.out)
	}
}

func TestWorkflowWriteInvalidTargets(t *testing.T) {
	for _, kind := range []string{"target link", "internal target link", "directory", "project link"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			outside := filepath.Join(h.root, "outside")
			if kind == "internal target link" {
				outside = filepath.Join(h.gridHome, "other")
			}
			untouched := filepath.Join(outside, "workflows.md")
			h.write(untouched, "unchanged")
			args := workflowWriteArgs("none")
			text := cliWorkflow
			target := filepath.Join(h.gridHome, "workflows.md")
			switch kind {
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			case "project link":
				h.write(filepath.Join(outside, "project.md"), "---\nid: id-alpha\n---\n")
				if err := os.MkdirAll(filepath.Join(h.gridHome, "projects"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(h.gridHome, "projects", "alpha")); err != nil {
					t.Skip(err)
				}
				args = append(args, "--project", "alpha")
				text = strings.ReplaceAll(text, "nothing else matches", "label alpha")
			default:
				if err := os.Symlink(untouched, target); err != nil {
					t.Skip(err)
				}
			}
			h.env.Stdin = strings.NewReader(text)
			if code := h.run(h.root, args...); code != 2 || !strings.Contains(h.errb.String(), "workflow path is unsafe or inaccessible") && !strings.Contains(h.errb.String(), "workflow target is not a regular file") || strings.Contains(h.errb.String(), h.root) {
				t.Fatalf("invalid target: exit %d: %s", code, h.errb)
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), CodeInvalidArgument)
			data, err := os.ReadFile(untouched)
			if err != nil || string(data) != "unchanged" {
				t.Fatalf("target changed: %q, %v", data, err)
			}
		})
	}
}

func TestWorkflowWriteConcurrent(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "workflows.md"), cliWorkflow)
	rev := fmt.Sprintf("%x", sha256.Sum256([]byte(cliWorkflow)))[:12]
	type result struct {
		code     int
		out, err string
		text     string
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, rounds := range []string{"3", "5"} {
		text := strings.ReplaceAll(cliWorkflow, "review rounds: 2", "review rounds: "+rounds)
		go func() {
			var out, errb bytes.Buffer
			env := h.env
			env.Stdout, env.Stderr, env.Stdin = &out, &errb, strings.NewReader(text)
			<-start
			code := Run(workflowWriteArgs(rev), env)
			results <- result{code, out.String(), errb.String(), text}
		}()
	}
	close(start)
	wins, conflicts := 0, 0
	var winner string
	for range 2 {
		r := <-results
		switch r.code {
		case 0:
			wins++
			winner = r.text
		case 3:
			conflicts++
			if r.out != "" || !strings.Contains(r.err, "workflow changed since read") {
				t.Fatalf("conflict: %+v", r)
			}
		default:
			t.Fatalf("writer: %+v", r)
		}
	}
	data, err := os.ReadFile(filepath.Join(h.gridHome, "workflows.md"))
	if wins != 1 || conflicts != 1 || err != nil || string(data) != winner {
		t.Fatalf("wins=%d conflicts=%d file=%q err=%v", wins, conflicts, data, err)
	}
}
