package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// ClaudeStatusLineChange decides the status line command. It gets the person's
// current command (present false when the file has no statusLine) and answers
// the command to write, or remove true to take the key out.
type ClaudeStatusLineChange func(cmd string, present bool) (next string, remove bool)

// EditClaudeStatusLine changes the statusLine key of the Claude settings file at
// path through the same read, stage and rename path setup uses, so every other
// key keeps its value and its order. Only the command of an existing statusLine
// object is replaced; its other fields stay. It writes nothing, and answers
// false, when the result is the file's present state. A statusLine that is not a
// command status line is refused and left alone.
func EditClaudeStatusLine(path string, change ClaudeStatusLineChange) (bool, error) {
	file, err := readClaudeFile(path)
	if err != nil {
		return false, err
	}
	doc, _ := parseClaudeObject([]byte("{}"))
	if file.exists {
		if doc, err = parseClaudeObject(file.data); err != nil {
			return false, claudeInvalid{path}
		}
	}
	cmd, line, present, err := claudeStatusLineOf(doc)
	if err != nil {
		return false, err
	}
	next, remove := change(cmd, present)
	switch {
	case remove && !present, !remove && present && next == cmd:
		return false, nil
	case remove:
		doc = deleteClaudeKey(doc, "statusLine")
	default:
		quoted, err := encodeClaudeJSON(next)
		if err != nil {
			return false, err
		}
		quoted = quoted[:len(quoted)-1] // Encode ends with a newline
		if !present {
			line, _ = parseClaudeObject([]byte("{}"))
			line.set("type", []byte(`"command"`))
		}
		line.set("command", quoted)
		doc.set("statusLine", line.bytes())
	}
	data, err := encodeClaudeJSON(json.RawMessage(doc.bytes()))
	if err != nil {
		return false, err
	}
	stage, err := stageClaudeFile(file.target, data, file.mode)
	if err != nil {
		return false, err
	}
	defer os.Remove(stage)
	if !file.unchanged() {
		return false, errClaudeChanged
	}
	if err := atomicfile.Rename(stage, file.target); err != nil {
		return false, err
	}
	if err := atomicfile.SyncDir(filepath.Dir(file.target)); err != nil {
		return false, err
	}
	return true, nil
}

// claudeStatusLineOf answers the command, the parsed object and whether the
// document has a statusLine. A statusLine that is not a command status line is
// refused. EditClaudeStatusLine and ReadClaudeStatusLine share it.
func claudeStatusLineOf(doc claudeObject) (cmd string, line claudeObject, present bool, err error) {
	raw := doc.get("statusLine")
	if raw == nil {
		return "", nil, false, nil
	}
	if line, err = parseClaudeObject(raw); err != nil {
		return "", nil, true, errors.New("statusLine is not an object; left alone")
	}
	if t := line.get("type"); t != nil {
		var typ string
		if json.Unmarshal(t, &typ) != nil || typ != "command" {
			return "", nil, true, errors.New("statusLine is not a command status line; left alone")
		}
	}
	if json.Unmarshal(line.get("command"), &cmd) != nil {
		return "", nil, true, errors.New("statusLine has no command string; left alone")
	}
	return cmd, line, true, nil
}

// ReadClaudeStatusLine answers the statusLine command of the Claude settings
// file at path and whether the file has a statusLine. It writes nothing and
// creates nothing: no file, or a file with no statusLine, answers "", false. It
// refuses what EditClaudeStatusLine refuses, with the same reasons.
func ReadClaudeStatusLine(path string) (cmd string, present bool, err error) {
	file, err := readClaudeFile(path)
	if err != nil {
		return "", false, err
	}
	if !file.exists {
		return "", false, nil
	}
	doc, err := parseClaudeObject(file.data)
	if err != nil {
		return "", false, claudeInvalid{path}
	}
	cmd, _, present, err = claudeStatusLineOf(doc)
	if err != nil {
		return "", false, err
	}
	return cmd, present, nil
}

func deleteClaudeKey(doc claudeObject, key string) claudeObject {
	out := make(claudeObject, 0, len(doc))
	for _, f := range doc {
		if f.key != key {
			out = append(out, f)
		}
	}
	return out
}
