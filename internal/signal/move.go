package signal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/projects"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// ValidateMove rejects unsafe operands before acquiring the workspace lock.
// An empty target is invalid; the CLI translates its explicit inbox marker.
func ValidateMove(id, from, to string) error {
	if !validID(id) {
		return &BadInputError{Message: "invalid signal id"}
	}
	if to == "" {
		return &BadInputError{Message: "move requires --to <project|->"}
	}
	for _, p := range []string{from, to} {
		if err := projects.Name(p); err != nil {
			return &BadInputError{Message: "invalid project name"}
		}
	}
	return nil
}

// Move relocates an open note within one Grid home. Add and Apply share this
// lock, so the destination check and rename cannot overwrite another writer's
// note. Both notes folders must be on the same filesystem; there is no copy
// fallback. inbox_workspace defaults to personal, the only supported policy.
func (s Store) Move(id, from, to string) (Result, error) {
	if s.Resolver.Caller.IsGuest() {
		return Result{}, &RefusedError{Message: "guests cannot act on signals"}
	}
	if err := ValidateMove(id, from, to); err != nil {
		return Result{}, err
	}
	unlock, err := localdata.Lock(s.GridHome, ".signals.lock")
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	source, target := s, s
	source.Project, target.Project = from, to
	if _, err := source.moveNotes(); err != nil {
		return Result{}, err
	}
	dir, err := target.moveNotes()
	if err != nil {
		return Result{}, err
	}
	path, err := source.path(id)
	if err != nil {
		return Result{}, err
	}
	destination, err := target.path(id)
	if err != nil {
		return Result{}, err
	}
	n, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, &RefusedError{Message: "no such note"}
	}
	if err != nil {
		return Result{}, err
	}
	if !openStatus(n.Status) {
		return Result{}, &RefusedError{Message: fmt.Sprintf("this note is already decided (%s)", n.Status)}
	}
	if from == to {
		return Result{}, &RefusedError{Message: "the note is already there"}
	}
	if _, err := os.Lstat(destination); err == nil {
		return Result{}, &RefusedError{Message: "a note with this id is already there"}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	files, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	if len(files) >= 4000 {
		return Result{}, &RefusedError{Message: "notes already holds 4000 files"}
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	status := n.Status
	if to == where.Personal && status == "placed" {
		status = "new"
	} else if to != where.Personal && status == "new" {
		status = "placed"
	}
	data, err := relocateStatus(original, n.Status, status)
	if err != nil {
		return Result{}, err
	}
	if len(data) > MaxBytes {
		return Result{}, fmt.Errorf("signal note exceeds %d bytes", MaxBytes)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}
	// atomicfile preserves an existing file's mode; tighten it explicitly so
	// imported notes also move with the required private permissions.
	st, err := os.Stat(path)
	if err != nil {
		return Result{}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Result{}, err
	}
	restore := func(cause error) error {
		return errors.Join(cause, atomicfile.Write(path, original, 0o600), os.Chmod(path, st.Mode().Perm()))
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return Result{}, restore(err)
	}
	if err := os.Rename(path, destination); err != nil {
		return Result{}, restore(err)
	}
	n.Status = status
	got := result(target, n, "updated")
	got.From = from
	got.Became = status
	return got, nil
}

func (s Store) moveNotes() (string, error) {
	dir, err := s.notes()
	// notes owns registration and Personal access. Keep storage/path errors as
	// I/O failures, and translate only its registration refusals.
	return dir, moveNotesError(err)
}

func moveNotesError(err error) error {
	if errors.Is(err, where.ErrProjectUnregistered) || errors.Is(err, where.ErrPersonalRefused) {
		return &RefusedError{Message: "no such project"}
	}
	return err
}

// Patch the status scalar in the original header rather than re-encoding Note:
// that retains unknown fields, comments, timestamps and the uninterpreted body.
func relocateStatus(data []byte, old, status string) ([]byte, error) {
	if old == status {
		return data, nil
	}
	head, _, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok {
		return nil, fmt.Errorf("signal has no closing front matter")
	}
	var doc yaml.Node
	if err := yaml.Load(head, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("signal metadata is invalid")
	}
	fields := doc.Content[0].Content
	for i := 0; i+1 < len(fields); i += 2 {
		if fields[i].Value != "status" {
			continue
		}
		value := fields[i+1]
		lines := bytes.Split(head, []byte("\n"))
		offset := 4
		for _, line := range lines[:value.Line-1] {
			offset += len(line) + 1
		}
		offset += len(string([]rune(string(lines[value.Line-1]))[:value.Column-1]))
		// A scalar's source position can include a tag or anchor. Preserve
		// those properties while replacing only its value.
		for offset < len(data) && (data[offset] == '!' || data[offset] == '&') {
			for offset < len(data) && !strings.ContainsRune(" \t\r\n", rune(data[offset])) {
				offset++
			}
			for offset < len(data) && strings.ContainsRune(" \t\r\n", rune(data[offset])) {
				offset++
			}
		}
		end, replacement := offset, status
		switch data[offset] {
		case '\'', '"':
			quote := data[offset]
			end = offset + 1
			for end < len(data) {
				if quote == '"' && data[end] == '\\' {
					end += 2
					continue
				}
				if data[end] == quote {
					end++
					if quote == '\'' && end < len(data) && data[end] == quote {
						end++
						continue
					}
					break
				}
				end++
			}
			replacement = string(quote) + status + string(quote)
		case '*':
			for end < len(data) && !strings.ContainsRune(" \t\r\n,]}", rune(data[end])) {
				end++
			}
		case '|', '>':
			// An open status has no whitespace. A valid block scalar therefore
			// contains the complete status on one content line; keep its style,
			// indentation, chomping indicator and comments.
			lineEnd := bytes.IndexByte(data[offset:], '\n')
			if lineEnd < 0 {
				return nil, fmt.Errorf("signal status is invalid")
			}
			offset += lineEnd + 1
			for offset < len(data) && strings.ContainsRune(" \t\r\n", rune(data[offset])) {
				offset++
			}
			end = offset + len(old)
		default:
			end = offset + len(old)
		}
		if end > len(data) {
			return nil, fmt.Errorf("signal status is invalid")
		}
		out := append([]byte(nil), data[:offset]...)
		out = append(out, replacement...)
		out = append(out, data[end:]...)
		// Validate the patched header before writing even for unusual YAML
		// spellings, so malformed or ambiguous metadata fails closed.
		patched, _, ok := bytes.Cut(out[4:], []byte("\n---\n"))
		var check Note
		if !ok {
			return nil, fmt.Errorf("signal status is invalid")
		}
		if err := yaml.Load(patched, &check); err != nil {
			return nil, err
		}
		if check.Status != status {
			return nil, fmt.Errorf("signal status is invalid")
		}
		return out, nil
	}
	return nil, fmt.Errorf("signal status is missing")
}
