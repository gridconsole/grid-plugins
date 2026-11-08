package intent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReleasePreservesRequestAndAudit(t *testing.T) {
	s := testStore(t)
	now := s.now()
	s.Now = func() time.Time { return now }
	r := enqueue(t, s, "start", nil)
	path := filepath.Join(s.TaskDir, "intents", r.ID+".json")
	// Intent records currently have no built-in attempt counter. Preserve any
	// worker-owned attempt metadata, including fields unknown to this reader.
	request := readReleaseTestFile(t, path)
	request = bytes.Replace(request, []byte(`"verb":`), []byte(`"attempt":7,"verb":`), 1)
	if err := os.WriteFile(path, request, 0o600); err != nil {
		t.Fatal(err)
	}
	claim, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	released, err := s.Release(r.ID, claim.Token)
	if err != nil {
		t.Fatal(err)
	}
	if released.Token != claim.Token || released.Who != claim.Who || !released.Created.Equal(now) {
		t.Fatalf("release: %+v", released)
	}
	auditPath := filepath.Join(s.TaskDir, "intents", r.ID+".release.json")
	audit := readReleaseTestFile(t, auditPath)
	var stored Release
	if err := json.Unmarshal(audit, &stored); err != nil || stored != released {
		t.Fatalf("stored release: %+v: %v", stored, err)
	}
	if !bytes.Contains(audit, []byte(`"created":"2030-01-01T00:01:00.000Z"`)) {
		t.Fatalf("timestamp format: %s", audit)
	}
	rows, err := s.List(true)
	if err != nil || len(rows) != 1 || rows[0].State != "pending" || rows[0].Claim != nil {
		t.Fatalf("released intent: %+v: %v", rows, err)
	}
	if _, err := s.Renew(r.ID, claim.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("released holder renewed: %v", err)
	}
	if _, err := s.Done(r.ID, claim.Token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("released holder completed: %v", err)
	}
	next, err := s.Claim(r.ID, "worker-b")
	if err != nil || next.Token == claim.Token {
		t.Fatalf("immediate reclaim: %+v: %v", next, err)
	}
	if !bytes.Equal(audit, readReleaseTestFile(t, auditPath)) {
		t.Fatal("reclaim changed release audit")
	}
	if _, err := s.Renew(r.ID, next.Token); err != nil {
		t.Fatalf("new claim blocked by previous release: %v", err)
	}
	now = now.Add(time.Minute)
	if _, err := s.Release(r.ID, next.Token); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if !bytes.Equal(request, readReleaseTestFile(t, path)) {
		t.Fatal("release or reclaim rewrote the request or its attempt metadata")
	}
}

func readReleaseTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReleaseRefusals(t *testing.T) {
	for _, scenario := range []string{"wrong", "expired", "done", "released", "replaced", "unclaimed", "malformed", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			s := testStore(t)
			r := enqueue(t, s, "start", nil)
			c, err := s.Claim(r.ID, "worker-a")
			if err != nil {
				t.Fatal(err)
			}
			token, want := c.Token, ErrClaim
			switch scenario {
			case "wrong":
				token = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			case "malformed":
				token = "invalid"
			case "empty":
				token = ""
			case "expired":
				s.Now = func() time.Time { return c.Until }
			case "done":
				if _, err := s.Done(r.ID, token, ""); err != nil {
					t.Fatal(err)
				}
				want = ErrDone
			case "released", "replaced":
				if _, err := s.Release(r.ID, token); err != nil {
					t.Fatal(err)
				}
				if scenario == "replaced" {
					if _, err := s.Claim(r.ID, "worker-b"); err != nil {
						t.Fatal(err)
					}
				}
			case "unclaimed":
				r = enqueue(t, s, "start", nil)
			}
			dir := filepath.Join(s.TaskDir, "intents")
			before := releaseTestSnapshot(t, dir)
			if _, err := s.Release(r.ID, token); !errors.Is(err, want) {
				t.Fatalf("release: %v, want %v", err, want)
			}
			if after := releaseTestSnapshot(t, dir); !bytes.Equal(before, after) {
				t.Fatalf("refused release changed files:\n%s\n%s", before, after)
			}
		})
	}
}

func releaseTestSnapshot(t *testing.T, dir string) []byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		files[entry.Name()] = string(readReleaseTestFile(t, filepath.Join(dir, entry.Name())))
	}
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReleaseRacingClaim(t *testing.T) {
	s := testStore(t)
	r := enqueue(t, s, "start", nil)
	first, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	type result struct {
		claim Claim
		err   error
	}
	gate := make(chan struct{})
	results := make(chan result, workers)
	release := make(chan error, 1)
	go func() {
		<-gate
		_, err := s.Release(r.ID, first.Token)
		release <- err
	}()
	for i := range workers {
		go func() {
			<-gate
			c, err := s.Claim(r.ID, fmt.Sprintf("worker-%d", i))
			results <- result{c, err}
		}()
	}
	close(gate)
	var winners []Claim
	for range workers {
		res := <-results
		if res.err == nil {
			winners = append(winners, res.claim)
		} else if !errors.Is(res.err, ErrClaimed) {
			t.Errorf("claim: %v", res.err)
		}
	}
	if err := <-release; err != nil {
		t.Fatal(err)
	}
	if len(winners) == 0 {
		// Every contender can acquire the lock before the releaser.
		c, err := s.Claim(r.ID, "worker-after-release")
		if err != nil {
			t.Fatal(err)
		}
		winners = append(winners, c)
	}
	if len(winners) != 1 || winners[0].Token == first.Token {
		t.Fatalf("holders after release: %+v", winners)
	}
	rows, err := s.List(false)
	if err != nil || len(rows) != 1 || rows[0].Claim == nil || rows[0].Claim.Token != winners[0].Token {
		t.Fatalf("winner not current: %+v: %v", rows, err)
	}
	if _, err := s.Release(r.ID, first.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("old holder released winner: %v", err)
	}
	if _, err := s.Done(r.ID, first.Token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("old holder completed winner: %v", err)
	}
	if _, err := s.Renew(r.ID, winners[0].Token); err != nil {
		t.Fatalf("winner lost its claim: %v", err)
	}
}

func TestReleaseRacingDone(t *testing.T) {
	s := testStore(t)
	r := enqueue(t, s, "start", nil)
	c, err := s.Claim(r.ID, "worker")
	if err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	released, done := make(chan error, 1), make(chan error, 1)
	go func() {
		<-gate
		_, err := s.Release(r.ID, c.Token)
		released <- err
	}()
	go func() {
		<-gate
		_, err := s.Done(r.ID, c.Token, "")
		done <- err
	}()
	close(gate)
	releaseErr, doneErr := <-released, <-done
	if releaseErr == nil && errors.Is(doneErr, ErrClaim) {
		if _, err := s.Claim(r.ID, "next"); err != nil {
			t.Fatal(err)
		}
	} else if doneErr == nil && errors.Is(releaseErr, ErrDone) {
		if _, err := s.Claim(r.ID, "next"); !errors.Is(err, ErrDone) {
			t.Fatal(err)
		}
	} else {
		t.Fatalf("release: %v; done: %v", releaseErr, doneErr)
	}
}

func TestReleaseMissingDirectory(t *testing.T) {
	s := testStore(t)
	if _, err := s.Release("01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAW"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
