package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	// CodeNoSuchProject means the requested project is missing or inaccessible.
	CodeNoSuchProject = "no_such_project"
	// CodeNoSuchTask means the requested task is missing or inaccessible.
	CodeNoSuchTask = "no_such_task"
	// CodeNoSuchNote means the requested signal or inbox note does not exist.
	CodeNoSuchNote = "no_such_note"
	// CodePersonRequired means the existing person gate refused the request.
	CodePersonRequired = "person_required"
	// CodeInvalidArgument means the command rejected its arguments.
	CodeInvalidArgument = "invalid_argument"
	// CodeInvalidPolicy means a workflow policy value the command depends on is invalid.
	CodeInvalidPolicy = "invalid_policy"
	// CodeWorkflowChanged means the task's workflow now resolves to other steps
	// than the ones it was created with, so it is held.
	CodeWorkflowChanged = "workflow_changed"
	// CodeLinkedFile means a protected task file has more than one hard link.
	CodeLinkedFile = "linked_file"
)

func errorCode(err error) string {
	var bad *signal.BadInputError
	switch {
	case errors.Is(err, where.ErrProjectUnregistered):
		return CodeNoSuchProject
	case errors.Is(err, where.ErrTaskNotFound):
		return CodeNoSuchTask
	case errors.Is(err, signal.ErrNoNote):
		return CodeNoSuchNote
	case errors.Is(err, task.ErrWorkflowChanged):
		return CodeWorkflowChanged
	case errors.As(err, &bad), errors.Is(err, errTaskOperand):
		return CodeInvalidArgument
	case errors.Is(err, task.ErrLinkedFile):
		return CodeLinkedFile
	default:
		return ""
	}
}

// commandFailure tracks diagnostics without delaying stdout or stderr. Existing
// JSON producers keep their envelopes; only failures with no stdout get a new one.
type commandFailure struct {
	stderr bytes.Buffer
	stdout outputCount
	flags  *flag.FlagSet
	code   string
}

type outputCount struct {
	io.Writer
	written int
}

func (w *outputCount) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.written += n
	return n, err
}

func (e Env) recordError(err error) {
	if e.failure != nil {
		e.failure.code = errorCode(err)
	}
}

func (f *commandFailure) finish(args []string, exit int) {
	if exit == 0 || f.stdout.written != 0 || f.stderr.Len() == 0 {
		return
	}
	fs := f.flags
	if fs == nil {
		fs = flag.NewFlagSet("", flag.ContinueOnError)
		fs.Bool("json", false, "")
	}
	if !errorJSONRequested(fs, args) {
		return
	}
	code := f.code
	if code == "" && exit == 2 {
		code = CodeInvalidArgument
	}
	if code == "" {
		return
	}
	type detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	writeJSON(f.stdout.Writer, struct {
		Error detail `json:"error"`
	}{detail{code, strings.TrimSuffix(f.stderr.String(), "\n")}})
}

// withPersonErrorCode adds fields to the existing gate/probe JSON, preserving
// reason, hint and decision fields without changing the person-gate implementation.
// These calls emit at most one small object, so only their stdout is buffered.
func withPersonErrorCode(env *Env, run func(*Env) int) int {
	if env.failure == nil {
		return run(env)
	}
	stdout := env.Stdout
	start := env.failure.stderr.Len()
	var out bytes.Buffer
	env.Stdout = &out
	exit := run(env)
	env.Stdout = stdout
	if exit == 3 && out.Len() > 0 {
		var object map[string]json.RawMessage
		if json.Unmarshal(out.Bytes(), &object) == nil && string(object["error"]) == `"person_required"` {
			object["code"], _ = json.Marshal(CodePersonRequired)
			object["message"], _ = json.Marshal(strings.TrimSuffix(string(env.failure.stderr.Bytes()[start:]), "\n"))
			writeJSON(stdout, object)
			return exit
		}
	}
	_, _ = stdout.Write(out.Bytes())
	return exit
}

// Parsing can stop before --json; inspect flags without treating option values
// or arguments after -- as flags, and retain the last valid JSON setting.
func errorJSONRequested(fs *flag.FlagSet, args []string) bool {
	asJSON := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, value, equal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if name == "json" {
			if !equal {
				value = "true"
			}
			if enabled, err := strconv.ParseBool(value); err == nil {
				asJSON = enabled
			}
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); (!ok || !b.IsBoolFlag()) && !equal {
			i++
		}
	}
	return asJSON
}
