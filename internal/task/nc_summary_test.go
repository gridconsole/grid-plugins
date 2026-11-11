package task

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ncSummaryListing is the sorted names in a task folder.
func ncSummaryListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func ncSummaryRead(t *testing.T, dir string) string {
	t.Helper()
	return readFile(t, filepath.Join(dir, "summary.md"))
}

func TestNcSummaryShapeAndReceipts(t *testing.T) {
	s, tk := writeFixture(t)
	before := ncSummaryListing(t, tk.Path)
	for _, name := range before {
		if name == "summary.md" {
			t.Fatal("a fresh task already holds summary.md")
		}
	}
	steps := []struct{ reader, text, want string }{
		{"customer", "  Plain words for the customer.  ", "## Owner\n\n## Developer\n\n## Customer\n\nPlain words for the customer.\n"},
		{"owner", "Owner view.", "## Owner\n\nOwner view.\n\n## Developer\n\n## Customer\n\nPlain words for the customer.\n"},
		{"customer", "Second line\nthird line", "## Owner\n\nOwner view.\n\n## Developer\n\n## Customer\n\nSecond line\nthird line\n"},
		{"Developer", "Dev view.", "## Owner\n\nOwner view.\n\n## Developer\n\nDev view.\n\n## Customer\n\nSecond line\nthird line\n"},
	}
	var first []string
	for i, step := range steps {
		event, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: step.reader, Text: step.text})
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if event.What != "summary" || event.Note != "summary · "+strings.ToLower(step.reader) || event.FromState != event.ToState {
			t.Fatalf("step %d receipt = %+v", i, event)
		}
		if got := ncSummaryRead(t, tk.Path); got != step.want {
			t.Fatalf("step %d summary.md = %q, want %q", i, got, step.want)
		}
		lines, err := s.History("", tk.ID)
		if err != nil || len(lines) != i+1 {
			t.Fatalf("step %d history = %v, %v", i, lines, err)
		}
		listing := ncSummaryListing(t, tk.Path)
		if i == 0 {
			// The first write creates summary.md and, on a task never written before,
			// the lock and the history file that holds the receipt.
			first = listing
			want := append([]string{}, before...)
			for _, name := range []string{"summary.md", ".task.lock", "history.md"} {
				if !strings.Contains(" "+strings.Join(before, " ")+" ", " "+name+" ") {
					want = append(want, name)
				}
			}
			sort.Strings(want)
			if strings.Join(first, " ") != strings.Join(want, " ") {
				t.Fatalf("listing after first write = %v, want %v", first, want)
			}
		} else if strings.Join(listing, " ") != strings.Join(first, " ") {
			t.Fatalf("step %d changed the folder: %v, want %v", i, listing, first)
		}
	}
}

func TestNcSummaryKeepsEveryOtherByte(t *testing.T) {
	for _, tc := range []struct{ name, have, reader, text, want string }{
		{"crlf and a line before the first heading",
			"Written by hand\r\n## Owner\r\nold owner\r\n\r\n## Developer\r\ndev stays\r\n\r\n## Customer\r\ncust stays\r\n",
			"owner", "new owner\nsecond line",
			"Written by hand\r\n## Owner\r\n\r\nnew owner\r\nsecond line\r\n\r\n## Developer\r\ndev stays\r\n\r\n## Customer\r\ncust stays\r\n"},
		{"last section, crlf",
			"## Owner\r\no\r\n\r\n## Customer\r\nold\r\n",
			"customer", "new",
			"## Owner\r\no\r\n\r\n## Customer\r\n\r\nnew\r\n"},
		{"heading in another case and with spaces",
			"## Owner\n\no\n\n##  CUSTOMER \n\nold\n\n## Notes\n\nkeep me\n",
			"customer", "new",
			"## Owner\n\no\n\n##  CUSTOMER \n\nnew\n\n## Notes\n\nkeep me\n"},
		{"the heading the app reads last is replaced",
			"## Owner\n\nfirst\n\n## Owner\n\nlast\n",
			"owner", "new",
			"## Owner\n\nfirst\n\n## Owner\n\nnew\n"},
		{"a missing section is added at the end",
			"intro\n\n## Owner\n\no\n",
			"developer", "dev",
			"intro\n\n## Owner\n\no\n\n## Developer\n\ndev\n"},
		{"a heading with no line end",
			"## Owner\n\no\n\n## Customer",
			"customer", "new",
			"## Owner\n\no\n\n## Customer\n\nnew\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, tk := writeFixture(t)
			write(t, filepath.Join(tk.Path, "summary.md"), tc.have)
			if _, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: tc.reader, Text: tc.text}); err != nil {
				t.Fatal(err)
			}
			if got := ncSummaryRead(t, tk.Path); got != tc.want {
				t.Fatalf("summary.md = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNcSummaryJournalReplay(t *testing.T) {
	_, tk := writeFixture(t)
	history := "- {\"what\":\"summary\"}\n"
	journal, err := json.Marshal(map[string][]byte{"history.md": []byte(history), "summary.md": []byte("## Owner\n\nx\n")})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(tk.Path, journalName), string(journal))
	if err := recoverWrite(tk.Path); err != nil {
		t.Fatal(err)
	}
	if got := ncSummaryRead(t, tk.Path); got != "## Owner\n\nx\n" {
		t.Fatalf("summary.md = %q", got)
	}
	if got := readFile(t, filepath.Join(tk.Path, "history.md")); got != history {
		t.Fatalf("history.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(tk.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal not removed")
	}
	// Any other name is still refused.
	bad, _ := json.Marshal(map[string][]byte{"history.md": []byte(history), "summary.txt": []byte("x")})
	write(t, filepath.Join(tk.Path, journalName), string(bad))
	if err := recoverWrite(tk.Path); err == nil || !strings.Contains(err.Error(), "invalid journal file") {
		t.Fatalf("unknown journal file: %v", err)
	}
}

func TestNcSummaryRefusals(t *testing.T) {
	s, tk := writeFixture(t)
	for _, tc := range []struct{ name, reader, text string }{
		{"unknown reader", "boss", "words"},
		{"empty reader", "", "words"},
		{"blank text", "owner", "   \n\t "},
		{"heading line", "owner", "a\n## Customer\nb"},
		{"heading first", "customer", "## Owner\nb"},
		{"control character", "owner", "a\x1bb"},
		{"carriage return", "owner", "a\r\nb"},
	} {
		_, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: tc.reader, Text: tc.text})
		if !errors.Is(err, ErrEditUsage) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if names := ncSummaryListing(t, tk.Path); strings.Contains(strings.Join(names, " "), "summary.md") || strings.Contains(strings.Join(names, " "), ".task.lock") {
		t.Fatalf("a refusal wrote something: %v", names)
	}
	// A heading-like line that is not at the start of a line is plain words.
	if _, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: "owner", Text: "a ## b\n#### c"}); err != nil {
		t.Fatal(err)
	}
	kept := ncSummaryRead(t, tk.Path)
	for _, reader := range []string{"boss", "owner"} {
		if _, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: reader, Text: "x\n## Developer\ny"}); !errors.Is(err, ErrEditUsage) {
			t.Fatalf("%s: %v", reader, err)
		}
	}
	if got := ncSummaryRead(t, tk.Path); got != kept {
		t.Fatalf("a refusal changed summary.md: %q", got)
	}
}

func TestNcSummaryRolePolicy(t *testing.T) {
	for _, role := range []string{"", "owner", "member", "guest", "worker"} {
		want := role == "" || role == "owner" || role == "member"
		if SummaryAllowed(role) != want {
			t.Fatalf("SummaryAllowed(%q) != %t", role, want)
		}
	}
	s, tk := writeFixture(t)
	_, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: "owner", Text: "words", Role: "guest"})
	if err == nil || err.Error() != "scope: role cannot write task summaries" || errors.Is(err, ErrEditUsage) {
		t.Fatalf("guest: %v", err)
	}
	if names := ncSummaryListing(t, tk.Path); strings.Contains(strings.Join(names, " "), "summary.md") || strings.Contains(strings.Join(names, " "), ".task.lock") {
		t.Fatalf("a role refusal wrote something: %v", names)
	}
	original := SummaryAllowed
	t.Cleanup(func() { SummaryAllowed = original })
	SummaryAllowed = func(role string) bool { return role == "guest" }
	if _, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: "owner", Text: "words", Role: "guest"}); err != nil {
		t.Fatalf("custom policy: %v", err)
	}
}

func TestNcSummaryClosedTaskAndState(t *testing.T) {
	s, tk := writeFixture(t)
	if _, err := s.Change(WriteOptions{ID: tk.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	taskBefore := readFile(t, filepath.Join(tk.Path, "task.md"))
	if _, err := s.Summary(SummaryOptions{WriteOptions: WriteOptions{ID: tk.ID}, Reader: "customer", Text: "All done."}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(tk.Path, "task.md")); got != taskBefore {
		t.Fatal("a summary changed task.md")
	}
}
