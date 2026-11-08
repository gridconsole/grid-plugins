package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestGateSquatMCP(t *testing.T) {
	for _, kind := range []string{"person", "timer"} {
		for _, allow := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/override=%t", kind, allow), func(t *testing.T) {
				s, store, writer := fixture(t)
				writer.Author = feed.Author{ID: "agent", Role: "agent"}
				writer.Worker = "agent@" + machineID
				if allow {
					if err := os.WriteFile(filepath.Join(store.TaskDir, "workflow.md"), []byte("---\ngate_post_roles:\n  "+kind+": [agent]\n---\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				// No question exists yet: reuse protection cannot stop this claim.
				args := `{"type":"question","body":{"question":"Open now","who":"owner","gate":{"id":"Q-1","kind":"` + kind + `","at":"2000-01-01T00:00:00.000Z"}}}`
				out := result(t, last(t, exchange(t, s, prelude+request("feed_post", args))))
				if allow {
					if out.IsError {
						t.Errorf("override: %+v", out)
					}
				} else if !out.IsError || !strings.Contains(out.Content[0].Text, "role agent may not post "+kind+" gates") {
					t.Errorf("squat: %+v", out)
				}
				r, err := store.Read(feed.Filter{})
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if allow {
					want = 1
				}
				if len(r.Records) != want {
					t.Errorf("records: got %d want %d", len(r.Records), want)
				}
				if !allow {
					if _, err := os.Stat(store.Dir()); !os.IsNotExist(err) {
						t.Errorf("refusal created Feed: %v", err)
					}
				}
			})
		}
	}
}
