package question

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

var validID = regexp.MustCompile(`^Q-[1-9][0-9]*$`)

func (s Store) locked(create bool, fn func(string) error) error {
	if s.TaskDir == "" {
		return errors.New("task directory is required")
	}
	// Resolve ancestors, including macOS's symlinked temporary directory, once.
	task, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return err
	}
	dir := filepath.Join(task, "questions")
	if create {
		if err = os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) && !create {
		return fn("")
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("questions must be a real directory")
	}
	// Lock a stable inode, never the question file which rename replaces.
	f, err := fslock.OpenNoFollow(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("question lock must be a regular file")
	}
	if err = fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	return fn(dir)
}

// readAll skips Markdown files not named Q-n.md, such as the old plugin's
// per-recipient files, and names them in one warning per call.
func (s Store) readAll(dir string) ([]Record, error) {
	records := []Record{}
	if dir == "" {
		return records, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	skipped := []string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		if !validID.MatchString(id) {
			skipped = append(skipped, entry.Name())
			continue
		}
		r, e := read(dir, id)
		if e != nil {
			return nil, e
		}
		records = append(records, r)
	}
	if len(skipped) > 0 && s.Warn != nil {
		fmt.Fprintf(s.Warn, "question: skipping files in questions/ that are not Q-n.md: %s\n", strings.Join(skipped, ", "))
	}
	sort.Slice(records, func(i, j int) bool {
		a, _ := strconv.Atoi(records[i].ID[2:])
		b, _ := strconv.Atoi(records[j].ID[2:])
		return a < b
	})
	return records, nil
}

func read(dir, id string) (r Record, err error) {
	path := filepath.Join(dir, id+".md")
	info, err := os.Lstat(path)
	if err != nil {
		return r, err
	}
	if !info.Mode().IsRegular() {
		return r, errors.New("question file must be regular")
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return r, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return r, err
	}
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return r, fmt.Errorf("%s: missing question frontmatter", id)
	}
	body, _, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok {
		return r, fmt.Errorf("%s: incomplete question frontmatter", id)
	}
	if err = contract.Validate("question-file", body); err != nil {
		return r, fmt.Errorf("%s: %w", id, err)
	}
	if err = json.Unmarshal(body, &r); err != nil {
		return r, err
	}
	if r.ID != id {
		return r, errors.New("question id does not match filename")
	}
	return r, validateRecord(r)
}

func validateRecord(r Record) error {
	if !validID.MatchString(r.ID) {
		return errors.New("invalid question id")
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(r.ID, "Q-")); err != nil {
		return errors.New("question id exceeds the supported range")
	}
	if err := validateInput(Input{Question: r.Question, Who: r.Who, Why: r.Why, Options: r.Options, Recommended: r.Recommended, Default: r.Gate.Default, Applies: r.Gate.At.Time}); err != nil {
		return err
	}
	if r.AgeAfterSeconds <= 0 || r.AgeAfterSeconds > int64((1<<63-1)/int64(time.Second)/2) {
		return errors.New("invalid aging interval")
	}
	aging, defaults, answers := 0, 0, 0
	state, answer := "open", ""
	for _, e := range r.History {
		switch e.Kind {
		case "aged":
			if state != "open" || !r.Blocking {
				return errors.New("invalid aging history")
			}
			aging++
		case "defaulted":
			if state != "open" || e.Text != r.Gate.Default {
				return errors.New("invalid default history")
			}
			defaults++
			state = "defaulted"
			answer = e.Text
		case "answered":
			if state != "open" && state != "defaulted" {
				return errors.New("invalid answer history")
			}
			answers++
			state = "answered"
			answer = e.Text
		case "withdrawn":
			if state != "open" {
				return errors.New("invalid withdrawal history")
			}
			state = "withdrawn"
		}
	}
	if aging != r.Aging || defaults > 1 || answers > 1 || state != r.State || answer != r.Answer {
		return errors.New("question state does not match history")
	}
	return nil
}

func save(dir string, r Record) error {
	if err := validateRecord(r); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = contract.Validate("question-file", data); err != nil {
		return err
	}
	path := filepath.Join(dir, r.ID+".md")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return errors.New("question file must be regular")
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	f, err := os.CreateTemp(dir, ".question-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(data)
	out.WriteString("\n---\n\n")
	blocking := "not blocking"
	if r.Blocking {
		blocking = "blocking"
	}
	fmt.Fprintf(&out, "## %s | to: %s | %s | %s\n\n%s\n\n", r.ID, r.Who, blocking, r.State, r.Question)
	if r.Why != "" {
		fmt.Fprintf(&out, "Why: %s\n\n", r.Why)
	}
	for _, o := range r.Options {
		fmt.Fprintf(&out, "- %s", o)
		if o == r.Recommended {
			out.WriteString(" (Recommended)")
		}
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "\nDefault: %s\nApplies: %s\nAging: %d\nAnswer: %s\n", r.Gate.Default, r.Gate.At.UTC().Format(FeedTime), r.Aging, r.Answer)
	if _, err = f.Write(out.Bytes()); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
