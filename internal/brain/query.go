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

// Search ranks description matches ahead of title/summary and body matches.
// Equal scores prefer the project level; results never contain note bodies.
func (s Store) Search(query, topic string, limit int) ([]Hit, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	if topic != "" && !ValidName(topic) && !(s.Vault() && ValidID(topic)) {
		return nil, fmt.Errorf("topic must be a plain name")
	}
	entries, err := s.entries()
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
			hits = append(hits, Hit{ID: e.ID, Path: e.Path, Topic: n.Topic, Level: n.Level, Description: n.Description, Summary: n.Summary, Score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
