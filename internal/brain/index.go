package brain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type TopicRow struct {
	Topic       string `json:"topic"`
	Description string `json:"description"`
	Entries     int    `json:"entries"`
	LastChange  string `json:"last_change"`
	Invalid     int    `json:"invalid"`
}

type Index struct {
	Topics      []TopicRow        `json:"topics"`
	Generations map[string]uint64 `json:"generations"`
	Text        string            `json:"text"`
}

type Backlink struct {
	From    string `json:"from"`
	Rel     string `json:"rel"`
	Summary string `json:"summary"`
	Why     string `json:"why"`
}

type Backlinks struct {
	Generation uint64                `json:"generation"`
	Links      map[string][]Backlink `json:"links"`
}

func readBacklinks(root *os.Root) (Backlinks, error) {
	b, err := safefile.Read(root, "knowledge/backlinks.json")
	if errors.Is(err, fs.ErrNotExist) {
		return Backlinks{Links: map[string][]Backlink{}}, nil
	}
	if err != nil {
		return Backlinks{}, err
	}
	var links Backlinks
	if err := json.Unmarshal(b, &links); err != nil {
		return Backlinks{}, fmt.Errorf("invalid backlinks index: %w", err)
	}
	return links, nil
}

func rows(root *os.Root, level string) ([]TopicRow, []Note, error) {
	topics, err := scopeTopics(root, level)
	if err != nil {
		return nil, nil, err
	}
	notes, invalid, err := scan(root, level)
	if err != nil {
		return nil, nil, err
	}
	dropped, err := Dropped(root) // one read of the level's log
	if err != nil {
		return nil, nil, err
	}
	result := []TopicRow{}
	seen := map[string]bool{}
	for _, t := range topics {
		r := TopicRow{Topic: t.Name, Description: t.Description, Invalid: invalid[t.Name]}
		seen[t.Name] = true
		if t.Kind == "invalid" {
			r.Invalid++
		}
		if t.Kind == "config" {
			entries, err := fs.ReadDir(root.FS(), filepath.Join("knowledge", t.Name))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, nil, err
			}
			for _, entry := range entries {
				if entry.IsDir() || entry.Name() == "TOPIC.md" || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				data, err := safefile.Read(root, filepath.Join("knowledge", t.Name, entry.Name()))
				var fields map[string]any
				if err == nil {
					_, err = DecodeConfig(data, &fields)
				}
				if err != nil {
					r.Invalid++
				} else {
					r.Entries++
				}
			}
		}
		for _, n := range notes {
			if n.Topic == t.Name {
				// A superseded or dropped note leaves the count and stays on disk.
				if n.SupersededBy == "" && !dropped[n.Topic+"/"+n.Name] {
					r.Entries++
				}
				if n.Updated > r.LastChange {
					r.LastChange = n.Updated
				}
			}
		}
		result = append(result, r)
	}
	// Files placed in a built-in topic at the wrong level must still be visible.
	var extra []string
	for name := range invalid {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		result = append(result, TopicRow{Topic: name, Description: "Invalid topic level", Invalid: invalid[name]})
	}
	return result, notes, nil
}

// Index reads a fresh SessionStart view without creating any files. No note
// bodies or descriptions from untrusted proposals enter the table.
func (s Store) Index() (Index, error) {
	idx := Index{Topics: []TopicRow{}, Generations: map[string]uint64{}}
	positions := map[string]int{}
	for _, t := range BuiltinTopics() {
		positions[t.Name] = len(idx.Topics)
		idx.Topics = append(idx.Topics, TopicRow{Topic: t.Name, Description: t.Description})
	}
	for _, level := range s.levels() {
		root, err := s.scope(level, false)
		if errors.Is(err, fs.ErrNotExist) {
			idx.Generations[level] = 0
			continue
		}
		if err != nil {
			return Index{}, err
		}
		r, _, err := rows(root, level)
		if err != nil {
			root.Close()
			return Index{}, err
		}
		backlinks, err := readBacklinks(root)
		root.Close()
		if err != nil {
			return Index{}, err
		}
		idx.Generations[level] = backlinks.Generation
		for _, row := range r {
			if i, ok := positions[row.Topic]; ok {
				idx.Topics[i].Entries += row.Entries
				idx.Topics[i].Invalid += row.Invalid
				if row.LastChange > idx.Topics[i].LastChange {
					idx.Topics[i].LastChange = row.LastChange
				}
			} else {
				positions[row.Topic] = len(idx.Topics)
				idx.Topics = append(idx.Topics, row)
			}
		}
	}
	idx.Text = SessionStartText(idx.Topics)
	return idx, nil
}

// SessionStartText budgets 1,200 UTF-8 bytes (400 estimated tokens at three
// bytes/token) and always leaves room for an elision banner. The complete rows
// remain available in Index.Topics; topics lists every definition.
func SessionStartText(rows []TopicRow) string {
	const budget = 1200
	const readStep = "Read a note: grid brain search --topic <topic> <words>, then grid brain show <id>.\n"
	const reserve = 60 + len(readStep)
	var b strings.Builder
	b.WriteString("Brain: project knowledge first, then home knowledge.\n")
	b.WriteString("topic | description | entries | last_change | invalid\n")
	for i, row := range rows {
		date := row.LastChange
		if len(date) >= 10 {
			date = date[:10]
		}
		if date == "" {
			date = "-"
		}
		desc := strings.Join(strings.Fields(row.Description), " ")
		desc = strings.ReplaceAll(desc, "|", "/")
		if len(desc) > 80 {
			desc = truncate(desc, 77) + "..."
		}
		line := fmt.Sprintf("%s | %s | %d | %s | %d\n", row.Topic, desc, row.Entries, date, row.Invalid)
		// The reserve holds the cut banner (under 60 bytes) and the closing line.
		if b.Len()+len(line)+reserve > budget {
			fmt.Fprintf(&b, "%d topics cut; grid brain topics lists them.\n", len(rows)-i)
			break
		}
		b.WriteString(line)
	}
	b.WriteString(readStep)
	return b.String()
}

func truncate(s string, bytes int) string {
	for len(s) > bytes {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return s
}

// RebuildIndex publishes a separate index and backlink generation at each
// level. Only edges originating at that level enter its cache; personal notes
// and links are never copied to the project's syncable tree. The generation in
// index.md lets readers detect an interrupted pair of atomic renames.
func (s Store) RebuildIndex() (Index, error) {
	for _, level := range s.levels() {
		root, err := s.scope(level, true)
		if err != nil {
			return Index{}, err
		}
		err = locked(root, func() error {
			r, notes, err := rows(root, level)
			if err != nil {
				return err
			}
			old, err := readBacklinks(root)
			if err != nil {
				return err
			}
			if old.Generation == ^uint64(0) {
				return fmt.Errorf("backlink generation overflow")
			}
			backlinks := Backlinks{Generation: old.Generation + 1, Links: map[string][]Backlink{}}
			for _, n := range notes {
				for _, l := range n.Links {
					backlinks.Links[l.To] = append(backlinks.Links[l.To], Backlink{From: "brain:" + level + "/" + n.Name, Rel: l.Rel, Summary: n.Summary, Why: l.Why})
				}
			}
			b, err := json.MarshalIndent(backlinks, "", "  ")
			if err != nil {
				return err
			}
			// The cache stays in this brain root, per person, and does not go to the
			// machine folder (the owner's decision, lane P-shared-home-4): a home-level
			// brain is one person's, and a machine folder shared by several logins must
			// not show its summaries (index.md) and backlinks to the other logins.
			if err := atomicWrite(root, "knowledge/backlinks.json", append(b, '\n')); err != nil {
				return err
			}
			text := fmt.Sprintf("generation: %d\nlevel: %s\n", backlinks.Generation, level) + SessionStartText(r)
			return atomicWrite(root, "knowledge/index.md", []byte(text))
		})
		root.Close()
		if err != nil {
			return Index{}, err
		}
	}
	return s.Index()
}
