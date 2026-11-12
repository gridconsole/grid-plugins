package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/intent"
)

func TestIntentReleaseCommand(t *testing.T) {
	for _, flag := range []string{"--token", "--claim", "environment"} {
		t.Run(flag, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			s := intent.Store{TaskDir: task}
			r, err := s.Enqueue("start", "task", nil, "cli", "")
			if err != nil {
				t.Fatal(err)
			}
			c, err := s.Claim(r.ID, "worker-a")
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"intent", "release", r.ID, "--task", task, "--json"}
			if flag == "environment" {
				h.env.Getenv = func(key string) string {
					if key == "GRID_INTENT_CLAIM" {
						return c.Token
					}
					return ""
				}
			} else {
				args = append(args, flag, c.Token)
			}
			if code := h.run(h.tmp, args...); code != 0 {
				t.Fatalf("release: exit %d: %s", code, h.errb)
			}
			var released struct {
				Token, Who string
				Created    time.Time
			}
			if err := json.Unmarshal(h.out.Bytes(), &released); err != nil {
				t.Fatal(err)
			}
			if released.Token != c.Token || released.Who != c.Who || released.Created.Before(c.Created) {
				t.Fatalf("release audit: %+v", released)
			}
			if code := h.run(task, "intent", "claim", r.ID, "--who", "worker-b", "--json"); code != 0 {
				t.Fatalf("immediate reclaim: exit %d: %s", code, h.errb)
			}
			var next intent.Claim
			if err := json.Unmarshal(h.out.Bytes(), &next); err != nil || next.Token == c.Token || next.Who != "worker-b" {
				t.Fatalf("new claim: %+v: %v", next, err)
			}
			if code := h.run(task, "intent", "release", r.ID, "--token", next.Token); code != 0 || !strings.Contains(h.out.String(), "state: pending") {
				t.Fatal(code, h.out, h.errb)
			}
		})
	}
}

func TestIntentReleaseCommandRefusals(t *testing.T) {
	for _, scenario := range []string{"wrong", "expired", "done", "released", "replaced"} {
		t.Run(scenario, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			s := intent.Store{TaskDir: task}
			if scenario == "expired" {
				s.Now = func() time.Time { return time.Now().Add(-intent.ClaimTimeout) }
			}
			r, err := s.Enqueue("start", "task", nil, "cli", "")
			if err != nil {
				t.Fatal(err)
			}
			c, err := s.Claim(r.ID, "worker-a")
			if err != nil {
				t.Fatal(err)
			}
			token := c.Token
			switch scenario {
			case "wrong":
				token = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			case "done":
				if _, err := s.Done(r.ID, token, ""); err != nil {
					t.Fatal(err)
				}
			case "released", "replaced":
				if code := h.run(task, "intent", "release", r.ID, "--token", token); code != 0 {
					t.Fatal(code, h.errb)
				}
				if scenario == "replaced" {
					if _, err := s.Claim(r.ID, "worker-b"); err != nil {
						t.Fatal(err)
					}
				}
			}
			dir := filepath.Join(task, "intents")
			before := intentFiles(t, dir)
			want := 3
			if scenario == "done" {
				want = 5
			}
			if code := h.run(task, "intent", "release", r.ID, "--token", token); code != want {
				t.Fatalf("refusal: exit %d: %s", code, h.errb)
			}
			after := intentFiles(t, dir)
			if !bytes.Equal(before, after) {
				t.Fatalf("refused release changed records:\nbefore: %s\nafter: %s", before, after)
			}
		})
	}
}

func intentFiles(t *testing.T, dir string) []byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		// feed/.cache holds fold.lock once a post refolds; only record files are compared.
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestIntentReleaseUsage(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	for _, args := range [][]string{
		{"intent", "release"},
		{"intent", "release", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"intent", "release", "01ARZ3NDEKTSV4RRFFQ69G5FAV", "--token", "x", "--error=oops"},
	} {
		if code := h.run(task, args...); code != 2 {
			t.Fatal(args, code, h.errb)
		}
	}
	if code := h.run(task, "intent", "release", "--help"); code != 0 {
		t.Fatal(code, h.errb)
	}
}
