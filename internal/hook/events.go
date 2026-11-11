package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Event describes one implemented dispatcher and its plugin registration.
type Event struct {
	Name    string
	Matcher string
	Timeout int
}

// This table is the source for dispatch, Handles and generated plugin hooks.
// Timeouts are seconds: Stop and SubagentStop leave room for a bounded verify.
var events = []struct {
	Event
	run func(Options) int
}{
	{Event{"SessionStart", "startup|resume|clear|compact|fork", 10}, quiet(func(o Options) { o.record("session start", func() { sessionStart(o) }) })},
	{Event{"UserPromptSubmit", "", 10}, remaining},
	{Event{"SubagentStart", "", 10}, remaining},
	{Event{"PreToolUse", "", 10}, quiet(toolUse)},
	{Event{"PostToolUse", "", 10}, remaining},
	{Event{"PreCompact", "", 10}, remaining},
	{Event{"Stop", "", 60}, remaining},
	{Event{"SubagentStop", "", 60}, remaining},
	{Event{"SessionEnd", "", 10}, remaining},
	{Event{"PostToolUseFailure", "", 10}, remaining},
	{Event{"PermissionRequest", "", 10}, quiet(toolUse)},
}

// Events returns registration metadata without exposing the dispatcher table.
func Events() []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		out = append(out, e.Event)
	}
	return out
}

func handler(name string) func(Options) int {
	for _, e := range events {
		if strings.EqualFold(name, e.Name) {
			return e.run
		}
	}
	return nil
}

func quiet(f func(Options)) func(Options) int {
	return func(o Options) int { f(o); return 0 }
}

var components = map[string]bool{"heartbeat": true, "stop-check": true, "worker-exit": true, "history-log": true, "usage-log": true, "brain-read": true, "brain-recall": true,
	"lib-agentname": true, "lib-gatelike": true, "lib-historyline": true, "lib-parsefrontmatter": true, "lib-workflowsteps": true, "lib-planstage": true}

func componentEvent(event string) bool { return components[strings.ToLower(event)] }
func str(v any) string                 { s, _ := v.(string); return s }
func (o Options) log(action string, err error) {
	if err != nil && o.Stderr != nil {
		fmt.Fprintf(o.Stderr, "grid hook: %s: %v\n", action, err)
	}
}

func remaining(o Options) int {
	in, large := hookio.Read(o.Stdin)
	if in == nil || large {
		return 0
	}
	if in.Cwd == "" {
		in.Cwd = o.Cwd
	}
	name := strings.ToLower(o.Event)
	if strings.HasPrefix(name, "lib-") {
		helper(o, *in)
		return 0
	}
	if !componentEvent(name) {
		for _, event := range []string{"PostToolUse", "PostToolUseFailure", "Stop", "SubagentStop", "SessionEnd", "UserPromptSubmit", "SubagentStart", "PreCompact"} {
			if strings.EqualFold(name, event) {
				in.HookEventName = event
			}
		}
	}
	switch name {
	case "heartbeat":
		o.record("heartbeat", func() { heartbeat(o, *in) })
	case "worker-exit":
		o.record("worker exit", func() { workerExit(o, *in) })
	case "history-log":
		o.record("history", func() { history(o, *in) })
	case "usage-log":
		o.record("session", func() { recordSession(o, *in) })
	case "brain-read":
		o.record("brain read", func() { brainRead(o, *in) })
	case "brain-recall":
		o.record("recall", func() { recall(o, *in) })
	case "stop-check":
		return stopCheck(o, *in)
	default:
		if name == "precompact" && o.Stdout != nil && o.hookEnabled("precompact", *in) {
			fmt.Fprintln(o.Stdout, "Keep the Job id, acceptance ids and open decisions in the summary.")
		}
		o.record("heartbeat", func() { heartbeat(o, *in) })
		o.record("history", func() { history(o, *in) })
		o.record("session", func() { recordSession(o, *in) })
		o.record("brain read", func() { brainRead(o, *in) })
		o.record("recall", func() { recall(o, *in) })
		if name == "stop" || name == "subagentstop" {
			if code := stopCheck(o, *in); code != 0 {
				return code
			}
		}
		if name == "sessionend" || name == "subagentstop" {
			o.record("worker exit", func() { workerExit(o, *in) })
		}
	}
	return 0
}

// record runs one function that only records (history, usage, heartbeat, brain
// reads, session end). Verdict v-15: it never blocks or denies. A Go panic exits
// the process with code 2, which Claude Code reads as a block, so a panic here
// goes to the hook log and the hook carries on and exits 0.
func (o Options) record(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			o.log(name, fmt.Errorf("panic: %v", r))
		}
	}()
	f()
}
func reader(in hookio.Input) *strings.Reader {
	b, _ := json.Marshal(in)
	return strings.NewReader(string(b))
}

func taskDir(o Options, in hookio.Input) (string, error) {
	if o.Resolver.GridHome == "" {
		return "", nil
	}
	explicit := o.Getenv("GRID_TASK")
	if explicit == "" {
		explicit = o.Getenv("GRID_FEED_TASK")
	}
	if explicit != "" {
		r, err := o.Resolver.Resolve(where.Query{Cwd: in.Cwd, Task: explicit})
		if err != nil {
			return "", err
		}
		return task.FolderAt(o.Resolver, r.TaskPath)
	}
	return task.FolderAt(o.Resolver, in.Cwd)
}
func activityStore(o Options, in hookio.Input) (session.Store, error) {
	dir, err := taskDir(o, in)
	if err != nil {
		return session.Store{}, err
	}
	if dir == "" {
		dir = o.Resolver.GridHome
	}
	return session.Store{TaskDir: dir}, nil
}
func writer(in hookio.Input) string {
	if in.AgentType == "" {
		return "claude"
	}
	return guard.AgentName(in.AgentType)
}
func safeReference(s string) string {
	if len(s) > 1000 || strings.ContainsAny(s, "\x00\r\n") {
		return ""
	}
	return s
}

func recordSession(o Options, in hookio.Input) {
	if in.SessionID == "" || o.Resolver.GridHome == "" {
		return
	}
	if in.HookEventName != "SessionStart" && in.HookEventName != "SubagentStop" && !(in.HookEventName == "PostToolUse" && (in.ToolName == "Agent" || in.ToolName == "Task")) {
		return
	}
	if !o.hookEnabled("usage-log", in) {
		return
	}
	store, err := activityStore(o, in)
	if err != nil {
		o.log("session", err)
		return
	}
	if in.HookEventName == "PostToolUse" {
		res := in.ToolResponse
		usage, _ := res["usage"].(map[string]any)
		num := func(m map[string]any, k string) float64 {
			n, _ := m[k].(float64)
			if n < 0 {
				return 0
			}
			return n
		}
		u := session.Usage{Input: num(usage, "input_tokens"), Output: num(usage, "output_tokens"), CacheRead: num(usage, "cache_read_input_tokens"), CacheWrite: num(usage, "cache_creation_input_tokens"), Total: num(res, "totalTokens"), Milliseconds: num(res, "totalDurationMs")}
		if in.ToolUseID != "" {
			o.log("usage", store.RecordUsage(in.SessionID, in.ToolUseID, u))
		}
		return
	}
	o.log("session", store.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		if in.HookEventName == "SubagentStop" {
			if p := safeReference(in.AgentTranscriptPath); p != "" && in.AgentID != "" {
				if a.Transcripts == nil {
					a.Transcripts = map[string]string{}
				}
				a.Transcripts[in.AgentID] = p
			}
		} else if p := safeReference(in.TranscriptPath); p != "" {
			a.Transcript = p
		}
		return nil
	}))
}

func heartbeat(o Options, in hookio.Input) {
	if in.SessionID == "" || (in.HookEventName != "PostToolUse" && in.HookEventName != "Stop" && in.HookEventName != "PreCompact") {
		return
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("heartbeat", err)
		return
	}
	if dir == "" {
		return
	}
	machine := o.Resolver.Machine
	if machine == "" {
		machine = gridhome.Machine(o.Resolver.GridHome)
	}
	if machine == "" {
		return
	}
	err = lease.HookActivity(session.Store{TaskDir: dir}, session.Key{SessionID: in.SessionID, MachineID: machine}, safeReference(in.TranscriptPath))
	if !errors.Is(err, os.ErrNotExist) {
		o.log("heartbeat", err)
	}
}

// workerExit records how a Grid Worker's session ended. It acts only for a
// Worker (GRID_FEED_TASK, GRID_JOB and GRID_WORKER set) whose Job has no result
// yet, as the old hook did. Claude Code's own SessionEnd reasons are never a
// crash: only an explicit Grid reason of crash or loop ends the record Errored.
func workerExit(o Options, in hookio.Input) {
	if in.HookEventName != "SessionEnd" && in.HookEventName != "SubagentStop" {
		return
	}
	jobID := o.Getenv("GRID_JOB")
	if in.SessionID == "" || o.Getenv("GRID_FEED_TASK") == "" || jobID == "" || o.Getenv("GRID_WORKER") == "" {
		return
	}
	if !o.hookEnabled("session-end", in) {
		return
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("worker exit", err)
		return
	}
	if dir == "" {
		return
	}
	if done, err := jobHasResult(dir, jobID, o.Getenv("GRID_ATTEMPT")); err != nil {
		o.log("worker exit", err)
		return
	} else if done {
		return
	}
	reason := exitReason(in)
	code := exitCode(in)
	store := session.Store{TaskDir: dir}
	key := "exit:" + in.AgentID + ":" + jobID + ":" + o.Getenv("GRID_LEASE_EPOCH") + ":" + o.Getenv("GRID_ATTEMPT")
	o.log("worker exit", store.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		if !a.Seen[key] {
			a.ExitReason = reason
			if a.Exits == nil {
				a.Exits = map[string]string{}
			}
			a.Exits[key] = reason
			a.ExitCode = code
			if a.ExitCodes == nil {
				a.ExitCodes = map[string]string{}
			}
			a.ExitCodes[key] = code
			if a.Seen == nil {
				a.Seen = map[string]bool{}
			}
			a.Seen[key] = true
		}
		return nil
	}))
	machine := o.Resolver.Machine
	if machine == "" {
		machine = gridhome.Machine(o.Resolver.GridHome)
	}
	if in.HookEventName == "SessionEnd" && machine != "" {
		state := session.Finished
		switch reason {
		case "crash", "loop":
			state = session.Errored
		case "cancelled":
			state = session.StoppedByYou
		}
		_, err = store.End(session.Key{SessionID: in.SessionID, MachineID: machine}, state)
		if !errors.Is(err, os.ErrNotExist) {
			o.log("end session", err)
		}
	}
}

// exitCode is the raw end reason kept beside the mapped one: what Claude Code
// sent, subagent_stop for a SubagentStop, none when SessionEnd gave no reason.
func exitCode(in hookio.Input) string {
	if in.HookEventName == "SubagentStop" {
		return "subagent_stop"
	}
	if in.Reason == "" {
		return "none"
	}
	return in.Reason
}

// exitReason maps a SessionEnd reason to the contract's worker-exit reasons.
// Claude Code sends clear, logout, prompt_input_exit and other (and more); none
// of them is a crash signal, so anything Grid did not name is a normal end.
func exitReason(in hookio.Input) string {
	if in.HookEventName == "SubagentStop" {
		return "done"
	}
	switch in.Reason {
	case "done", "crash", "usage_limit", "cancelled", "loop":
		return in.Reason
	case "clear", "logout":
		return "cancelled"
	}
	return "done"
}

// jobHasResult reports a result or an earlier worker-exit for the Job's attempt.
// A task without a Feed has none.
func jobHasResult(dir, jobID, attempt string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, "feed")); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	// A task with no stable id has no Feed this store can read.
	id, err := feed.TaskID(dir)
	if err != nil {
		return false, nil
	}
	read, err := feed.Store{TaskDir: dir, TaskID: id}.Read(feed.Filter{Job: jobID})
	if err != nil {
		return false, err
	}
	n, _ := strconv.Atoi(attempt)
	for _, e := range read.Records {
		if (e.Type == "result" || e.Type == "worker-exit") && (n == 0 || e.Attempt == nil || *e.Attempt == n) {
			return true, nil
		}
	}
	return false, nil
}

// Helper events expose the same pure functions the handlers use. They are
// migration diagnostics, not runtime hook subscriptions.
func helper(o Options, in hookio.Input) {
	arg := func(i int) string {
		if i < len(in.Args) {
			return str(in.Args[i])
		}
		return ""
	}
	var out any
	switch strings.ToLower(o.Event) {
	case "lib-agentname":
		out = guard.AgentName(arg(0))
	case "lib-gatelike":
		out = guard.GateLike(arg(0))
	case "lib-historyline":
		out = taskfile.HistoryLine(arg(0), arg(1), arg(2), arg(3), arg(4))
	case "lib-parsefrontmatter":
		out = taskfile.ParseFrontmatter(arg(0))
	case "lib-workflowsteps":
		out = guard.WorkflowSteps(arg(0))
	case "lib-planstage":
		out = guard.PlanStage(arg(0), arg(1))
	}
	if o.Stdout != nil {
		o.log("helper output", json.NewEncoder(o.Stdout).Encode(out))
	}
}

func absolute(cwd, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}
