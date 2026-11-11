package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopicLoadsFromTopicsPath(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/topics/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: project\ndescription: Test guidance\n---\nLocal test advice.\n")
	topics, err := s.Topics()
	if err != nil || len(topics) != 9 || topics[8].Name != "testing" || !topics[8].Custom || topics[8].Description != "Test guidance" {
		t.Fatalf("topic from topics/: %+v %v", topics, err)
	}
	n := sample()
	n.Topic = "testing"
	accepted(t, s, n)
}

func TestTopicsPathWinsOverLegacyPath(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/knowledge/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: project\ndescription: Old words\n---\n")
	putFile(t, s, "projects/example/topics/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: project\ndescription: New words\n---\n")
	topics, err := s.Topics()
	if err != nil || len(topics) != 9 || topics[8].Description != "New words" {
		t.Fatalf("topics path must win and list once: %+v %v", topics, err)
	}
}

func TestTopicBrief(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "topics", "delivering", "TOPIC.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", 400)
	text := "---\nname: delivering\nkind: knowledge\nlevel: home\ndescription: Deliver only with proof.\n---\n# Delivering\nIgnored prose.\n- brain:home/a-note: why one matters here\n- " + long + "\n"
	for i := 0; i < 10; i++ {
		text += "- pointer: why it helps\n"
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, err := TopicBrief(root, "delivering")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(got, "\n")
	if lines[0] != "Brain topic delivering: Deliver only with proof." || lines[1] != "- brain:home/a-note: why one matters here" {
		t.Fatalf("summary and first pointer: %q", lines)
	}
	if len(lines) != 1+briefPointers || strings.Contains(got, "Ignored prose") || len([]rune(lines[2])) > briefLineRunes+2 {
		t.Fatalf("brief not short: %d lines: %s", len(lines), got)
	}
	if _, err := TopicBrief(root, "nothing"); err == nil {
		t.Fatal("missing topic gave a brief")
	}
	if _, err := TopicBrief(root, "../x"); err == nil {
		t.Fatal("path accepted as topic name")
	}
}
