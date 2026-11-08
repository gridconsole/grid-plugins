package question

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

var jobID = regexp.MustCompile(`^J-[1-9][0-9]*(\.[1-9][0-9]*)*$`)

func (s Store) feed() (feed.Store, error) {
	id := s.TaskID
	if id == "" {
		var err error
		id, err = feed.TaskID(s.TaskDir)
		if err != nil {
			return feed.Store{}, err
		}
	}
	return feed.Store{TaskDir: s.TaskDir, TaskID: id, Now: s.Now}, nil
}

func (s Store) publishQuestion(dir string, r *Record) error {
	f, err := s.feed()
	if err != nil {
		return err
	}
	body := map[string]any{"question": r.Question, "who": r.Who, "blocking": r.Blocking, "gate": r.Gate, "age_after_seconds": r.AgeAfterSeconds}
	if len(r.Options) > 0 {
		body["options"] = r.Options
	}
	if r.Recommended != "" {
		body["recommended"] = r.Recommended
	}
	if r.Why != "" {
		body["why"] = r.Why
	}
	if r.Job != "" {
		body["job"] = r.Job
	}
	if r.Item != 0 {
		body["item"] = r.Item
	}
	w := s.Writer
	w.Lease, w.Fence, w.Gen = "", 0, 0
	post, err := f.Post(feed.Input{Type: "question", Body: body, Job: r.Job}, w)
	if err != nil {
		return err
	}
	r.Post = post.ID
	r.Asked.Time, err = time.Parse(FeedTime, post.At)
	if err != nil {
		return fmt.Errorf("invalid Feed timestamp: %w", err)
	}
	return save(dir, *r)
}

func (s Store) publishEvents(dir string, r *Record, before int) error {
	f, err := s.feed()
	if err != nil {
		return err
	}
	for i := before; i < len(r.History); i++ {
		e := &r.History[i]
		in := feed.Input{Re: r.Post, Job: r.Job}
		w := s.Writer
		if e.Kind == "answered" {
			if contract.NormalizeRole(w.Author.Role) == "agent" || contract.NormalizeRole(w.Author.Role) == "cli" {
				return fmt.Errorf("only a person may answer a person gate")
			}
			w.Lease, w.Fence, w.Gen = "", 0, 0
			in.Type, in.Body = "answer", map[string]any{"answer": e.Text}
		} else {
			w = s.ControlWriter
			if e.Kind == "withdrawn" && w.Lease != "task" {
				w = s.Writer
				w.Lease, w.Fence, w.Gen = "", 0, 0
			} else if w.Lease != "task" || w.Fence <= 0 {
				return fmt.Errorf("%s requires the task lease to post its control record", e.Kind)
			}
			in.Type, in.Body = "control", map[string]any{"target": r.Post, "question_event": e.Kind, "text": e.Text}
			if e.Kind == "aged" {
				level := 0
				for _, h := range r.History[:i+1] {
					if h.Kind == "aged" {
						level++
					}
				}
				in.Body["level"] = level
			}
		}
		switch contract.NormalizeRole(w.Author.Role) {
		case "owner", "member", "guest", "lead", "agent", "cli":
		default:
			return fmt.Errorf("unsupported question author role %q", w.Author.Role)
		}
		if i > 0 {
			in.After = []string{r.History[i-1].Post}
		}
		post, err := f.Post(in, w)
		if err != nil {
			return err
		}
		e.Post, e.Who, e.Role = post.ID, post.Author.ID, post.Author.Role
		e.At.Time, err = time.Parse(FeedTime, post.At)
		if err != nil {
			return fmt.Errorf("invalid Feed timestamp: %w", err)
		}
	}
	return save(dir, *r)
}

// Sync repairs missing or changed projections from the admitted Feed. The same
// task question lock covers sync, id allocation and publication, so a crash
// after the append is repaired before a retry can publish another decision.
func (s Store) Sync() (records []Record, err error) {
	if err = s.requireWritePolicy(); err != nil {
		return
	}
	err = s.locked(true, func(dir string) error {
		var e error
		records, e = s.project(dir, true)
		return e
	})
	return
}

func (s Store) project(dir string, write bool) ([]Record, error) {
	if write && s.TaskID == "" {
		var err error
		s.TaskID, err = feed.EnsureTaskID(s.TaskDir)
		if err != nil {
			return nil, err
		}
	}
	f, err := s.feed()
	if err != nil {
		return nil, err
	}
	read, err := f.ReadOnly(feed.Filter{})
	if err != nil {
		return nil, err
	}
	allowed, err := s.withdrawPolicy()
	if err != nil {
		return nil, err
	}
	_, decisions := feed.FoldReadAt(read, s.now(), allowed)
	records := []Record{}
	ids := map[string]bool{}
	for _, d := range decisions {
		body, err := json.Marshal(d.Question.Body)
		if err != nil {
			return nil, err
		}
		var r Record
		if err = json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		r.ID, r.Post = r.Gate.ID, d.Question.ID
		// Person gates posted without a local Q-id are not question files.
		if !validID.MatchString(r.ID) {
			continue
		}
		if ids[r.ID] {
			return nil, fmt.Errorf("Feed has conflicting question id %s", r.ID)
		}
		ids[r.ID] = true
		if r.Job == "" && d.Question.Job != nil {
			r.Job = *d.Question.Job
		}
		r.Asked.Time, err = time.Parse(FeedTime, d.Question.At)
		if err != nil {
			return nil, fmt.Errorf("invalid Feed timestamp: %w", err)
		}
		r.State, r.Answer, r.Aging = d.State, d.Answer, d.Aging
		if r.AgeAfterSeconds == 0 {
			r.AgeAfterSeconds = int64(DefaultAgeAfter / time.Second)
		}
		r.History = []Event{}
		for _, p := range d.Events {
			kind, text := "answered", ""
			if p.Type == "answer" {
				text, _ = p.Body["answer"].(string)
			} else {
				kind, _ = p.Body["question_event"].(string)
				text, _ = p.Body["text"].(string)
			}
			at, err := time.Parse(FeedTime, p.At)
			if err != nil {
				return nil, fmt.Errorf("invalid Feed event timestamp: %w", err)
			}
			r.History = append(r.History, Event{Post: p.ID, Role: p.Author.Role, Who: p.Author.ID, Kind: kind, Text: text, At: Stamp{at}})
		}
		records = append(records, r)
	}
	// Check history invariants before changing any files.
	for _, r := range records {
		if err := validateRecord(r); err != nil {
			return nil, err
		}
	}
	var entries []os.DirEntry
	if dir != "" {
		entries, err = os.ReadDir(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	skipped := []string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		if !validID.MatchString(id) {
			skipped = append(skipped, e.Name())
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("question file must be regular")
		}
		if !ids[id] {
			// Keep even unreadable or legacy contents intact. The filename reserves
			// the id; no migration or schema interpretation is needed to list it.
			if _, err := strconv.Atoi(id[2:]); err != nil {
				return nil, fmt.Errorf("question id exceeds the supported range: %s", id)
			}
			records = append(records, Record{ID: id, State: "not in the Feed"})
			if s.Warn != nil {
				fmt.Fprintf(s.Warn, "question: %s is not in the Feed; keeping file and reserving its id\n", e.Name())
			}
		}
	}
	if len(skipped) > 0 && s.Warn != nil {
		fmt.Fprintf(s.Warn, "question: skipping files in questions/ that are not Q-n.md: %s\n", strings.Join(skipped, ", "))
	}
	if write {
		for _, r := range records {
			if r.Post == "" {
				continue
			}
			if err = save(dir, r); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(records, func(i, j int) bool {
		a, _ := strconv.Atoi(records[i].ID[2:])
		b, _ := strconv.Atoi(records[j].ID[2:])
		return a < b
	})
	return records, nil
}
