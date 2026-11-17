package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

func (s Store) entries() ([]Entry, error) {
	if s.Vault() {
		return s.vaultEntries()
	}
	entries := []Entry{}
	for _, level := range s.levels() {
		root, err := s.scope(level, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		notes, _, err := scan(root, level)
		var dropped map[string]bool
		if err == nil {
			dropped, err = Dropped(root) // one read of the level's log
		}
		root.Close()
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			e := s.entry(n, false)
			e.Dropped = dropped[n.Topic+"/"+n.Name]
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// Show resolves an unqualified id project-first, or an explicit brain:<level>/<id>.
func (s Store) Show(id string) (Entry, error) {
	if s.Vault() {
		return s.vaultShow(id)
	}
	level, name, err := splitID(id)
	if err != nil {
		return Entry{}, err
	}
	entries, err := s.entries()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range entries {
		if e.Note.Name == name && (level == "" || e.Note.Level == level) {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("accepted note %q not found", id)
}

func splitID(id string) (level, name string, err error) {
	name = id
	if strings.HasPrefix(id, "brain:") {
		var ok bool
		level, name, ok = strings.Cut(strings.TrimPrefix(id, "brain:"), "/")
		if !ok || (level != "home" && level != "project") {
			return "", "", fmt.Errorf("id must be brain:<home|project>/<name>")
		}
	}
	if !ValidName(name) {
		return "", "", fmt.Errorf("note id must be a plain name or brain:<home|project>/<name>")
	}
	return level, name, nil
}

type Hit struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Topic       string `json:"topic"`
	Level       string `json:"level"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
	Score       int    `json:"score"`
	// Kind is task, job, post or proposal. It is empty for a note, so a note
	// hit reads as it always did.
	Kind string `json:"kind,omitempty"`
	// Conflict names the person notes an agent note supersedes or duplicates; the
	// person note is listed before it (docs/research/brain-obsidian.md 6.3).
	Conflict []string `json:"conflict,omitempty"`
}

// The kinds of a search hit (docs/research/brain-board.md Q6). A note is
// knowledge; task, job and post are history; a proposal is a note waiting to be
// filed and is searched only when asked for.
const (
	KindNote     = "note"
	KindTask     = "task"
	KindJob      = "job"
	KindPost     = "post"
	KindProposal = "proposal"
)

// Kinds lists the values `--kind` takes, knowledge first.
var Kinds = []string{KindNote, KindTask, KindJob, KindPost, KindProposal}

// ValidKind reports whether kind is a value of --kind.
func ValidKind(kind string) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// postTypes are the board post types that are searched. Checkpoints, heartbeats
// and the other types are not indexed, so a long task adds few entries.
var postTypes = []string{"result", "question", "answer", "found"}

// entryKind says what a vault entry is by where it sits: tasks/<T>/board/ holds
// posts, tasks/<T>/jobs/ holds Job nodes, the rest of tasks/<T>/ is the task, and
// every other note is knowledge. indexed is false for a post of a type that is
// not searched. An entry of the old layout (no row) is always a note.
func entryKind(e Entry) (kind string, indexed bool) {
	if e.row == nil {
		return KindNote, true
	}
	parts := strings.Split(e.Note.Name, "/")
	if len(parts) < 3 || parts[0] != "tasks" {
		return KindNote, true
	}
	switch parts[2] {
	case "board":
		if len(parts) != 4 {
			return KindPost, false
		}
		for _, t := range postTypes {
			if strings.HasSuffix(parts[3], "-"+t) {
				return KindPost, true
			}
		}
		return KindPost, false
	case "jobs":
		return KindJob, true
	}
	return KindTask, true
}

// kindClass orders kinds: knowledge before history.
func kindClass(kind string) int {
	if kind == KindNote || kind == "" { // a hit's kind is empty for a note
		return 0
	}
	return 1
}

// proposalEntries are the notes waiting to be filed at the levels the store
// reads, as entries.
func (s Store) proposalEntries() ([]Entry, error) {
	waiting, err := s.Waiting("")
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, w := range waiting {
		// Path is brain:<level>/brain-proposals/<name>.md; ID is the name.
		level, _, ok := strings.Cut(strings.TrimPrefix(w.Path, "brain:"), "/")
		if !strings.HasPrefix(w.Path, "brain:") || !ok {
			continue
		}
		e, err := s.Proposal(level, w.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

var wordPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)
var stopWords = " a an the in on of to for and or is are was be how what why when where which do does did it this that with from by at as we i you our my "

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, word := range wordPattern.FindAllString(strings.ToLower(s), -1) {
		if len(word) < 2 || strings.Contains(stopWords, " "+word+" ") {
			continue
		}
		if len(word) > 3 {
			switch {
			case strings.HasSuffix(word, "ies"):
				word = strings.TrimSuffix(word, "ies") + "y"
			case strings.HasSuffix(word, "es"):
				word = strings.TrimSuffix(word, "s")
			case strings.HasSuffix(word, "s"):
				word = strings.TrimSuffix(word, "s")
			}
		}
		out[word] = true
	}
	return out
}

// Search is SearchKind over every kind but proposals.
func (s Store) Search(query, topic string, limit int) ([]Hit, error) {
	return s.SearchKind(query, topic, "", limit)
}

// SearchKind ranks description matches ahead of title/summary and body matches.
// Knowledge (notes) comes before history (tasks, Jobs, posts) whatever the score;
// within a class the higher score comes first, and equal scores prefer the
// project level. kind limits the hits to one kind; empty means every kind but
// proposals. Results never contain note bodies.
func (s Store) SearchKind(query, topic, kind string, limit int) ([]Hit, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	if topic != "" && !ValidName(topic) && !(s.Vault() && ValidID(topic)) {
		return nil, fmt.Errorf("topic must be a plain name")
	}
	if kind != "" && !ValidKind(kind) {
		return nil, fmt.Errorf("kind must be one of %s", strings.Join(Kinds, ", "))
	}
	var entries []Entry
	var err error
	if kind == KindProposal {
		entries, err = s.proposalEntries()
	} else {
		entries, err = s.entries()
	}
	if err != nil {
		return nil, err
	}
	q := words(query)
	hits := []Hit{}
	for _, e := range entries {
		n := e.Note
		if n.SupersededBy != "" || e.Dropped {
			continue
		}
		ek := KindProposal
		if kind != KindProposal {
			var indexed bool
			if ek, indexed = entryKind(e); !indexed || (kind != "" && ek != kind) {
				continue
			}
		}
		if topic != "" && n.Topic != topic {
			continue
		}
		desc, title, body := words(n.Description), words(n.Name+" "+n.Summary), words(n.Body)
		tags := map[string]bool{}
		if e.row != nil { // a vault note: its aliases and tags weigh like a description
			tags = words(strings.Join(append(append([]string{}, e.row.Aliases...), e.row.Tags...), " "))
		}
		score := 0
		for w := range q {
			if desc[w] || tags[w] {
				score += 4
			}
			if title[w] {
				score += 2
			}
			if body[w] {
				score++
			}
		}
		if score > 0 {
			if ek == KindNote {
				ek = ""
			}
			hits = append(hits, Hit{ID: e.ID, Path: e.Path, Topic: n.Topic, Level: n.Level, Description: n.Description, Summary: n.Summary, Score: score, Kind: ek})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if ci, cj := kindClass(hits[i].Kind), kindClass(hits[j].Kind); ci != cj {
			return ci < cj
		}
		return hits[i].Score > hits[j].Score
	})
	if s.Vault() {
		hits = personFirst(hits, entries)
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
