package brain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigScopesAndWriter(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Project: "example"}
	for _, level := range []string{"home", "project"} {
		t.Run(level, func(t *testing.T) {
			if err := s.EditConfig(level, "routines", "daily", func(old []byte) ([]byte, error) {
				if old != nil {
					t.Fatal("unexpected entry")
				}
				return EncodeConfig(map[string]string{"name": "daily"}, "Notes.")
			}); err != nil {
				t.Fatal(err)
			}
			b, err := s.ReadConfig(level, "routines", "daily")
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Name string `yaml:"name"`
			}
			body, err := DecodeConfig(b, &got)
			if err != nil || got.Name != "daily" || body != "Notes.\n" {
				t.Fatalf("decode: %+v %q %v", got, body, err)
			}
			stop := errors.New("stop")
			if err := s.EditConfig(level, "routines", "daily", func([]byte) ([]byte, error) { return nil, stop }); !errors.Is(err, stop) {
				t.Fatal(err)
			}
			names, err := s.ConfigNames(level, "routines")
			if err != nil || len(names) != 1 {
				t.Fatalf("names: %v %v", names, err)
			}
		})
	}
	for _, tc := range []struct{ level, topic, name string }{{"home", "decisions", "entry"}, {"home", "routines", "../escape"}, {"project", "environments", "TOPIC"}} {
		if err := s.EditConfig(tc.level, tc.topic, tc.name, func([]byte) ([]byte, error) { return []byte("bad"), nil }); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range idx.Topics {
		if row.Topic == "routines" && (row.Entries != 2 || row.Invalid != 0) {
			t.Fatalf("config index: %+v", row)
		}
	}
	notes, err := s.Search("daily", "", 10)
	if err != nil || len(notes) != 0 {
		t.Fatalf("config leaked into knowledge search: %v %v", notes, err)
	}

	path := filepath.Join(s.GridHome, "knowledge", "routines", "linked.md")
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadConfig("home", "routines", "linked"); err == nil {
		t.Fatal("read symlink")
	}
	if err := s.EditConfig("home", "routines", "linked", func([]byte) ([]byte, error) { return nil, nil }); err == nil {
		t.Fatal("edited symlink")
	}
}
