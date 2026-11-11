package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/intent"
)

func TestExitRefusalClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{intent.ErrClaimed, 5}, {intent.ErrDone, 5}, {intent.ErrClaim, 3},
		{errors.New(intent.ErrClaimed.Error()), 1},
	} {
		for depth := 0; depth < 3; depth++ {
			if got := exitCode(tc.err); got != tc.want {
				t.Errorf("%v: exit %d, want %d", tc.err, got, tc.want)
			}
			tc.err = fmt.Errorf("wrapped: %w", tc.err)
		}
	}
}

func TestExitRefusalTwoClaims(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	r, err := (intent.Store{TaskDir: task}).Enqueue("start", "task", nil, "cli", "")
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 5} {
		code := h.run(task, "intent", "claim", r.ID, "--who", fmt.Sprintf("worker-%d", i), "--json")
		if code != want {
			t.Errorf("claim %d: exit %d, want %d: %s", i+1, code, want, h.errb)
		}
		if i == 1 && (h.errb.String() != "grid: intent is already claimed\n" || h.out.Len() != 0) {
			t.Fatalf("refusal stdout %q stderr %q", h.out, h.errb)
		}
	}
}

func TestExitRefusalIntent(t *testing.T) {
	for _, action := range []string{"claim", "renew", "release", "done"} {
		for _, state := range []string{"done", "wrong", "expired", "released", "replaced", "unclaimed"} {
			if action == "claim" && state != "done" {
				continue
			}
			t.Run(action+"/"+state, func(t *testing.T) {
				h := newHarness(t)
				task := livenessProject(h)
				s := intent.Store{TaskDir: task}
				if state == "expired" {
					s.Now = func() time.Time { return time.Now().Add(-2 * intent.ClaimTimeout) }
				}
				r, err := s.Enqueue("start", "task", nil, "cli", "")
				if err != nil {
					t.Fatal(err)
				}
				token := "01ARZ3NDEKTSV4RRFFQ69G5FAZ"
				if state != "unclaimed" {
					c, err := s.Claim(r.ID, "worker-a")
					if err != nil {
						t.Fatal(err)
					}
					if state != "wrong" {
						token = c.Token
					}
				}
				want, diagnostic := 3, "grid: a current claim token is required\n"
				switch state {
				case "done":
					if _, err := s.Done(r.ID, token, ""); err != nil {
						t.Fatal(err)
					}
					// A matching done replay succeeds; a different token is refused.
					token = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
					want, diagnostic = 5, "grid: intent is already done\n"
				case "released", "replaced":
					if _, err := s.Release(r.ID, token); err != nil {
						t.Fatal(err)
					}
					if state == "replaced" {
						if _, err := s.Claim(r.ID, "worker-b"); err != nil {
							t.Fatal(err)
						}
					}
				}
				before := intentFiles(t, filepath.Join(task, "intents"))
				args := []string{"intent", action, r.ID}
				if action != "claim" {
					args = append(args, "--token", token)
				}
				if code := h.run(task, args...); code != want {
					t.Errorf("exit %d, want %d: %s", code, want, h.errb)
				}
				if h.errb.String() != diagnostic || h.out.Len() != 0 {
					t.Errorf("stdout %q stderr %q, want %q", h.out, h.errb, diagnostic)
				}
				if after := intentFiles(t, filepath.Join(task, "intents")); !bytes.Equal(before, after) {
					t.Fatal("refusal changed intent files")
				}
			})
		}
	}
}

func TestExitRefusalStorageAndScope(t *testing.T) {
	for _, action := range []string{"claim", "renew", "release", "done"} {
		for _, failure := range []string{"store", "scope"} {
			t.Run(action+"/"+failure, func(t *testing.T) {
				h := newHarness(t)
				task := livenessProject(h)
				r, err := (intent.Store{TaskDir: task}).Enqueue("start", "task", nil, "cli", "")
				if err != nil {
					t.Fatal(err)
				}
				// A directory in place of the record is unreadable even as root.
				path := filepath.Join(task, "intents", r.ID+".json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				h.mkdir(path)
				if failure == "scope" {
					h.env.GridHome = ""
				}
				args := []string{"intent", action, r.ID}
				if action != "claim" {
					args = append(args, "--token", "01ARZ3NDEKTSV4RRFFQ69G5FAZ")
				}
				if code := h.run(task, args...); code != 1 || h.errb.Len() == 0 || h.out.Len() != 0 {
					t.Fatalf("exit %d stdout %q stderr %q", code, h.out, h.errb)
				}
			})
		}
	}
}

func TestExitRefusalJobSide(t *testing.T) {
	for _, tc := range []struct{ kind, lease, message string }{
		{"ack", "", "Job-side writes require grid job post or grid job ack"},
		{"submit", "", "Job-side writes require grid job post or grid job submit"},
		{"result", "", "Job-side writes require grid job post or grid job result"},
		{"checkpoint", "", "Job-side writes require grid job post or grid job checkpoint"},
		{"worker-exit", "", "Job-side writes require grid job post or grid job worker-exit"},
		{"integrate", "", "Job-side writes require grid job post or grid job integrate"},
		{"post", "J-1", "Job lease writes require grid job post"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			err := refuseJobSide(tc.kind, tc.lease)
			if !errors.Is(err, feed.ErrInvalidRecord) || err.Error() != tc.message {
				t.Errorf("refusal: %v", err)
			}
			h, task := feedHarness(t)
			if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"seed"}`); code != 0 {
				t.Fatal(code, h.errb)
			}
			before := intentFiles(t, filepath.Join(task, "feed"))
			if code := h.run(task, "feed", "post", "--type", tc.kind, "--lease", tc.lease, "--body", `{}`); code != 3 {
				t.Errorf("exit %d, want 3: %s", code, h.errb)
			}
			if h.errb.String() != "grid feed: "+tc.message+"\n" || h.out.Len() != 0 {
				t.Errorf("stdout %q stderr %q", h.out, h.errb)
			}
			if after := intentFiles(t, filepath.Join(task, "feed")); !bytes.Equal(before, after) {
				t.Fatal("refusal changed Feed bytes")
			}
		})
	}
}

func TestExitRefusalHelpGolden(t *testing.T) {
	// Inline golden keeps this narrow help contract in the lane's test file.
	const want = "Exit status: 0 done, 1 failed, 2 usage, 3 rejected (stale lease, invalid record or intent claim token; do not retry), 4 disk full, 5 taken (intent already claimed or done; skip), 6 stale rev (read again), 7 not versioned (nothing written)."
	h := newHarness(t)
	if code := h.run(h.tmp, "help"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var lines []string
	for _, line := range strings.Split(h.out.String(), "\n") {
		if strings.HasPrefix(line, "Exit status:") {
			lines = append(lines, line)
		}
	}
	got, _ := json.Marshal(lines)
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("help exit status golden: got %s, want %q", got, want)
	}
}
