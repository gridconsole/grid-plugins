package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

func readBounded(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("hook input must be a regular file")
	}

	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if len(b) > 1<<20 {
		return "", fmt.Errorf("hook file exceeds read limit")
	}
	return string(b), err
}

// A transcript is evidence only for an unambiguous brief inside this task.
// File contents are never copied into output or session records.
var briefMention = regexp.MustCompile(`(?:^|[\s"'` + "`" + `])([^\s"'` + "`" + `]+[/\\]subagents[/\\][A-Za-z0-9_.-]+\.md)`)

func briefName(in hookio.Input, dir string) string {
	text, err := readBounded(in.AgentTranscriptPath)
	if err != nil {
		return ""
	}
	names := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		var row struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		var parts []string
		if s, ok := row.Message.Content.(string); ok {
			parts = append(parts, s)
		}
		if blocks, ok := row.Message.Content.([]any); ok {
			for _, b := range blocks {
				if m, ok := b.(map[string]any); ok {
					parts = append(parts, str(m["text"]))
					if ti, ok := m["input"].(map[string]any); ok {
						parts = append(parts, str(ti["file_path"]))
					}
				}
			}
		}
		for _, part := range parts {
			for _, m := range briefMention.FindAllStringSubmatch(" "+part, -1) {
				path := absolute(in.Cwd, m[1])
				if filepath.Dir(path) == filepath.Join(dir, "subagents") {
					names[filepath.Base(path)] = true
				}
			}
		}
	}
	if len(names) == 1 {
		for name := range names {
			return name
		}
	}
	return ""
}
func section(body, name string) string {
	lines := strings.Split(body, "\n")
	on := false
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if on {
				break
			}
			on = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == name
			continue
		}
		if on {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
func stopCheck(o Options, in hookio.Input) int {
	if in.HookEventName != "Stop" && in.HookEventName != "SubagentStop" {
		return 0
	}
	level, _ := o.hookLevel("stop-check", in)
	if level == "off" {
		return 0
	}
	if level != "warn" {
		return checkStop(o, in)
	}
	// Preserve the diagnostic while allowing the stop, including saved Job verification.
	var warning bytes.Buffer
	output := o.Stderr
	o.Stderr = &warning
	code := checkStop(o, in)
	if output != nil {
		if code == 2 {
			fmt.Fprint(output, "Warning: stop-check: ")
		}
		_, _ = output.Write(warning.Bytes())
	}
	return 0
}

func checkStop(o Options, in hookio.Input) int {
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("stop check", err)
		return 0
	}
	if dir == "" {
		return 0
	}
	if code, handled := verifyStop(o, in, dir); handled {
		return code
	}
	var problems []string
	if in.HookEventName == "SubagentStop" {
		who := guard.AgentName(in.AgentType)
		if who == "grid-scout" {
			return 0
		}
		name := briefName(in, dir)
		if name == "" {
			return 0
		}
		path := filepath.Join(dir, "subagents", name)
		text, err := readBounded(path)
		if err != nil {
			o.log("stop check", err)
			return 0
		}
		doc := taskfile.ParseFrontmatter(text)
		if doc == nil || doc.Odd {
			o.log("stop check", fmt.Errorf("invalid brief metadata"))
			return 0
		}
		roleReviewer := strings.Contains(str(doc.Data["role"]), "reviewer")
		mark := who != "grid-reviewer" || roleReviewer
		field := "Outcome"
		if who == "grid-reviewer" || roleReviewer {
			field = "Verdict"
		}
		if section(doc.Body, field) == "" {
			problems = append(problems, name+": write the "+field+" section with what was checked and what remains.")
			if in.StopHookActive && mark {
				o.log("mark brief", task.MarkBrief(dir, name, "blocked"))
			}
		} else if mark {
			o.log("mark brief", task.MarkBrief(dir, name, "done"))
		}
	} else {
		doc, err := task.Document(dir)
		if err != nil {
			o.log("stop check", err)
			return 0
		}
		// New tasks use title and intent; summary/next belong to the legacy contract.
		if _, legacy := doc.Data["status"]; legacy {
			for _, field := range []string{"summary", "next"} {
				if strings.TrimSpace(str(doc.Data[field])) == "" {
					problems = append(problems, "task.md: "+field+" is empty; record what was done and what happens next through grid.")
				}
			}
		}
		entries, err := os.ReadDir(filepath.Join(dir, "subagents"))
		if err != nil && !os.IsNotExist(err) {
			o.log("stop check", err)
			return 0
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			text, err := readBounded(filepath.Join(dir, "subagents", e.Name()))
			if err != nil {
				o.log("stop check", err)
				continue
			}
			d := taskfile.ParseFrontmatter(text)
			if d == nil {
				continue
			}
			if d.Data["status"] == "done" || d.Data["status"] == "blocked" {
				field := "Outcome"
				if strings.Contains(str(d.Data["role"]), "reviewer") {
					field = "Verdict"
				}
				if section(d.Body, field) == "" {
					problems = append(problems, e.Name()+": write the "+field+" section.")
				}
			}
		}
	}
	if in.StopHookActive || len(problems) == 0 {
		return 0
	}
	if o.Stderr != nil {
		fmt.Fprintf(o.Stderr, "Not finished yet:\n- %s\nFix this, then stop again.\n", strings.Join(problems, "\n- "))
	}
	return 2
}
