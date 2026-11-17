// Package postfile is the per-post board of a task: one immutable markdown file per
// post under the task's board/ folder, named <seq>-<job|task>-<type>.md. The file is a
// header between two "---" lines, then the markdown rendering of the post. The fold
// reads the header only; the markdown is display and links (docs/research/brain-board.md
// Q8).
//
// The package is a leaf: it knows the header, the file name, the renderer and the
// folder, and nothing of the Feed, so internal/feed can use it as its board mode.
package postfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Author is who wrote a post and in which role.
type Author struct {
	ID   string
	Role string
}

// Header is the header of one post file. Job, For and Re are null when empty, and
// Legacy is written only when set. Body is the type's JSON body on one line, kept as
// written, so a number such as 2.0 reads back as 2.0.
type Header struct {
	ID      string
	Task    string
	Type    string
	TypeV   int
	Job     string
	Attempt *int
	Gen     int64
	Seq     int64
	At      string
	Author  Author
	By      string
	For     string
	Re      string
	After   []string
	Legacy  string
	Body    []byte
}

// ErrUnterminated is a file whose header has no closing "---" line. A reader counts
// it bad for this pass and reads it again on the next (F3, F12).
var ErrUnterminated = errors.New("post header has no closing ---")

// Encode writes the header with its two "---" lines. Decode of the result gives the
// same Header, and Encode of that gives the same bytes.
func (h Header) Encode() ([]byte, error) {
	if !json.Valid(h.Body) || bytes.ContainsAny(h.Body, "\r\n") {
		return nil, errors.New("post body is not one line of JSON")
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	put := func(k, v string) { b.WriteString(k + ": " + v + "\n") }
	put("id", scalar(h.ID))
	put("task", scalar(h.Task))
	put("type", scalar(h.Type))
	put("type_v", strconv.Itoa(h.TypeV))
	put("job", nullable(h.Job))
	if h.Attempt == nil {
		put("attempt", "null")
	} else {
		put("attempt", strconv.Itoa(*h.Attempt))
	}
	put("gen", strconv.FormatInt(h.Gen, 10))
	put("seq", strconv.FormatInt(h.Seq, 10))
	put("at", scalar(h.At))
	put("author", "{id: "+scalar(h.Author.ID)+", role: "+scalar(h.Author.Role)+"}")
	put("by", scalar(h.By))
	put("for", nullable(h.For))
	put("re", nullable(h.Re))
	after := make([]string, len(h.After))
	for i, a := range h.After {
		after[i] = scalar(a)
	}
	put("after", "["+strings.Join(after, ", ")+"]")
	if h.Legacy != "" {
		put("legacy", scalar(h.Legacy))
	}
	put("body", string(h.Body))
	b.WriteString("---\n")
	return b.Bytes(), nil
}

// Decode reads the header at the start of a post file and answers it with the
// markdown below it. It is strict: a header that Encode would not write the same way,
// byte for byte, is an error.
func Decode(data []byte) (Header, []byte, error) {
	var h Header
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return h, nil, errors.New("post file does not start with ---")
	}
	end := bytes.Index(data[3:], []byte("\n---\n"))
	if end < 0 {
		return h, nil, ErrUnterminated
	}
	head := data[:3+end+5]
	lines := strings.Split(string(data[4:3+end+1]), "\n")
	lines = lines[:len(lines)-1]
	keys := []string{"id", "task", "type", "type_v", "job", "attempt", "gen", "seq", "at", "author", "by", "for", "re", "after", "legacy", "body"}
	i := 0
	for _, key := range keys {
		if i >= len(lines) {
			return h, nil, fmt.Errorf("post header has no %s", key)
		}
		v, ok := strings.CutPrefix(lines[i], key+": ")
		if !ok {
			if key == "legacy" {
				continue
			}
			return h, nil, fmt.Errorf("post header line %d is not %s", i+1, key)
		}
		i++
		var err error
		switch key {
		case "id":
			h.ID, err = unscalar(v)
		case "task":
			h.Task, err = unscalar(v)
		case "type":
			h.Type, err = unscalar(v)
		case "type_v":
			h.TypeV, err = strconv.Atoi(v)
		case "job":
			h.Job, err = unnullable(v)
		case "attempt":
			if v != "null" {
				var n int
				n, err = strconv.Atoi(v)
				h.Attempt = &n
			}
		case "gen":
			h.Gen, err = strconv.ParseInt(v, 10, 64)
		case "seq":
			h.Seq, err = strconv.ParseInt(v, 10, 64)
		case "at":
			h.At, err = unscalar(v)
		case "author":
			h.Author, err = unauthor(v)
		case "by":
			h.By, err = unscalar(v)
		case "for":
			h.For, err = unnullable(v)
		case "re":
			h.Re, err = unnullable(v)
		case "after":
			h.After, err = unlist(v)
		case "legacy":
			h.Legacy, err = unscalar(v)
		case "body":
			h.Body = []byte(v)
		}
		if err != nil {
			return h, nil, fmt.Errorf("post header %s: %w", key, err)
		}
	}
	if i != len(lines) {
		return h, nil, fmt.Errorf("post header has an unknown line %d", i+1)
	}
	again, err := h.Encode()
	if err != nil {
		return h, nil, err
	}
	if !bytes.Equal(again, head) {
		return h, nil, errors.New("post header is not in its written form")
	}
	return h, data[len(head):], nil
}

// Name is the file name of a post: the seq zero-padded to six digits, the Job or
// "task", and the type.
func Name(seq int64, job, typ string) string {
	if job == "" {
		job = "task"
	}
	return fmt.Sprintf("%06d-%s-%s.md", seq, job, typ)
}

// NameSeq is the seq a post file name starts with.
func NameSeq(name string) (int64, bool) {
	digits, _, ok := strings.Cut(name, "-")
	if !ok || digits == "" || !strings.HasSuffix(name, ".md") {
		return 0, false
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	return n, err == nil && n >= 0
}

var plain = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.:@/+-]*$`)

// scalar writes a string plain when YAML reads it back as the same string, and in
// JSON quotes otherwise.
func scalar(s string) string {
	if plain.MatchString(s) {
		switch strings.ToLower(s) {
		case "null", "true", "false", "yes", "no", "on", "off", "y", "n":
		default:
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return s
			}
		}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

func nullable(s string) string {
	if s == "" {
		return "null"
	}
	return scalar(s)
}

func unscalar(v string) (string, error) {
	s, rest, err := readScalar(v)
	if err == nil && rest != "" {
		err = errors.New("trailing text")
	}
	return s, err
}

func unnullable(v string) (string, error) {
	if v == "null" {
		return "", nil
	}
	return unscalar(v)
}

// readScalar reads one scalar from the start of v: a JSON string, or plain text up to
// a flow delimiter.
func readScalar(v string) (string, string, error) {
	if strings.HasPrefix(v, `"`) {
		for i := 1; i < len(v); i++ {
			switch v[i] {
			case '\\':
				i++
			case '"':
				var s string
				err := json.Unmarshal([]byte(v[:i+1]), &s)
				return s, v[i+1:], err
			}
		}
		return "", "", errors.New("unterminated string")
	}
	i := strings.IndexAny(v, ",]}")
	if i < 0 {
		i = len(v)
	}
	return v[:i], v[i:], nil
}

func unauthor(v string) (Author, error) {
	var a Author
	rest, ok := strings.CutPrefix(v, "{id: ")
	if !ok {
		return a, errors.New("not {id, role}")
	}
	var err error
	if a.ID, rest, err = readScalar(rest); err != nil {
		return a, err
	}
	if rest, ok = strings.CutPrefix(rest, ", role: "); !ok {
		return a, errors.New("no role")
	}
	if a.Role, rest, err = readScalar(rest); err != nil {
		return a, err
	}
	if rest != "}" {
		return a, errors.New("not {id, role}")
	}
	return a, nil
}

func unlist(v string) ([]string, error) {
	rest, ok := strings.CutPrefix(v, "[")
	if !ok {
		return nil, errors.New("not a list")
	}
	out := []string{}
	if rest == "]" {
		return out, nil
	}
	for {
		s, r, err := readScalar(rest)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
		if r == "]" {
			return out, nil
		}
		if rest, ok = strings.CutPrefix(r, ", "); !ok {
			return nil, errors.New("not a list")
		}
	}
}
