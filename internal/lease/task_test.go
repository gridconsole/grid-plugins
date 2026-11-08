package lease

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestTaskAndJobTenures(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
	w := feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}
	tenure, err := s.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = tenure.Fence
	if _, err := s.TakeTask(w, os.Getpid(), tenure.Since); err == nil {
		t.Fatal("replaced held task")
	}
	worker := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: "worker@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "J-1"}
	first, err := s.GrantJob(w, "J-1", worker.Worker, 1)
	if err != nil {
		t.Fatal(err)
	}
	worker.Fence = first.Fence
	n := 1
	if err := s.Feed.WithLease(worker, &n, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	second, err := s.GrantJob(w, "J-1", worker.Worker, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second.Fence <= first.Fence {
		t.Fatal("fence did not advance")
	}
	if err := s.Feed.WithLease(worker, &n, func() error { t.Fatal("old tenure admitted"); return nil }); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatal(err)
	}
	wrong := w
	wrong.Fence++
	if _, err := s.GrantJob(wrong, "J-2", worker.Worker, 1); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatal(err)
	}
	if _, err := s.Read("../task"); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestLeaseReadersRejectTheSameInvalidGrant(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir()}}
	path := filepath.Join(s.Feed.Dir(), "lease", "J-1.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../contract/fixtures/lease/valid/alternate.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, bytes.Replace(raw, []byte(`"J-1"`), []byte(`"J-2"`), 1), bytes.Replace(raw, []byte(`"lease": "J-1",`), nil, 1)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Read("J-1"); err == nil {
			t.Errorf("TaskStore accepted invalid grant %s", data)
		}
		if _, err := ReadJob(s.Feed.TaskDir, "J-1"); err == nil {
			t.Errorf("ReadJob accepted invalid grant %s", data)
		}
	}
}
