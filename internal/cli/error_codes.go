package cli

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

const (
	// CodeNoSuchProject means the requested project is missing or inaccessible.
	CodeNoSuchProject = "no_such_project"
	// CodeNoSuchTask means the requested task is missing or inaccessible.
	CodeNoSuchTask = "no_such_task"
	// CodeNoSuchNote means the requested signal or inbox note does not exist.
	CodeNoSuchNote = "no_such_note"
	// CodeInvalidArgument means the command rejected its arguments.
	CodeInvalidArgument = "invalid_argument"
	// CodeInvalidPolicy means a workflow policy value the command depends on is invalid.
	CodeInvalidPolicy = "invalid_policy"
	// CodeWorkflowFileChanged means a workflow file changed since the caller read
	// its revision; nothing was written.
	CodeWorkflowFileChanged = "workflow_file_changed"
	// CodeNoteState means a signal note's status does not allow the act or move.
	CodeNoteState = "note_state"
	// CodeNoteExists means the note is already in the target project.
	CodeNoteExists = "note_exists"
	// CodeNotesFull means the target notes folder already holds 4000 files.
	CodeNotesFull = "notes_full"
	// CodeGuestRefused means the caller is refused as a guest; nothing was written.
	CodeGuestRefused = "guest_refused"
)

func errorCode(err error) string {
	var bad *signal.BadInputError
	var coded *settings.CodedError
	var refused *signal.RefusedError
	switch {
	case errors.As(err, &coded):
		return coded.Code
	case errors.Is(err, where.ErrProjectUnregistered):
		return CodeNoSuchProject
	case errors.Is(err, where.ErrTaskNotFound):
		return CodeNoSuchTask
	case errors.Is(err, signal.ErrNoNote):
		return CodeNoSuchNote
	case errors.As(err, &bad), errors.Is(err, errTaskOperand):
		return CodeInvalidArgument
	case errors.Is(err, workflow.ErrChanged):
		return CodeWorkflowFileChanged
	case errors.Is(err, ilGuestRefused):
		return CodeGuestRefused
	case errors.As(err, &refused) && refused.Code != "":
		return refused.Code
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

func jsonRequested(args []string) bool {
	return slices.Contains(args, "--json") || slices.Contains(args, "-json") || slices.Contains(args, "--json=true") || slices.Contains(args, "-json=true")
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
