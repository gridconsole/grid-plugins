package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
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
	// Preserve the diagnostic while allowing the stop.
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

// checkStop is the one stop check: an agent that holds a Job has posted the
// Job's result on the Feed (jobHasResult). If not, it is told once, in one
// line, to post it; with stop_hook_active it stops. No Job (a plain session, a
// scout) stops at once and prints nothing.
func checkStop(o Options, in hookio.Input) int {
	jobID := o.Getenv("GRID_JOB")
	if jobID == "" {
		return 0
	}
	if in.HookEventName == "SubagentStop" && contract.NormalizeRole(guard.AgentName(in.AgentType)) == "scout" {
		return 0
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("stop check", err)
		return 0
	}
	if dir == "" {
		return 0
	}
	posted, err := jobHasResult(dir, jobID, o.Getenv("GRID_ATTEMPT"))
	if err != nil {
		o.log("stop check", err)
		return 0
	}
	if posted || in.StopHookActive {
		return 0
	}
	if o.Stderr != nil {
		fmt.Fprintf(o.Stderr, "Post the result of Job %s on the Feed, then stop.\n", jobID)
	}
	return 2
}
