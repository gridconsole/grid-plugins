package feed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestInvalidRecordRejections(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T, Store, Writer) error
		text string
	}{
		{"unregistered", func(t *testing.T, s Store, w Writer) error {
			_, e := s.Post(Input{Type: "example", Body: map[string]any{}}, w)
			return e
		}, `unregistered post type "example"`},
		{"schema", func(t *testing.T, s Store, w Writer) error {
			_, e := s.Post(Input{Type: "post", Body: map[string]any{}}, w)
			return e
		}, "$.body: oneOf matched 0 branches: $.body: missing required ref"},
		{"identity", func(t *testing.T, s Store, w Writer) error {
			w.Author.ID = "other"
			_, e := s.Post(message("example"), w)
			return e
		}, "author and worker disagree"},
		{"body encoding", func(t *testing.T, s Store, w Writer) error {
			in := message("example")
			in.Body["text"] = make(chan int)
			_, e := s.Post(in, w)
			return e
		}, "json: unsupported type: chan int"},
		{"validate encoding", func(t *testing.T, s Store, w Writer) error {
			r, e := s.record(message("example"), w)
			if e != nil {
				t.Fatal(e)
			}
			r.Body["text"] = make(chan int)
			return validate(r)
		}, "json: unsupported type: chan int"},
		{"body decoding", func(t *testing.T, s Store, w Writer) error {
			in := message("example")
			in.Body["text"] = json.RawMessage("1e1000")
			_, e := s.Post(in, w)
			return e
		}, "json: cannot unmarshal number 1e1000 into Go struct field .text of type float64"},
		{"same tenure id", func(t *testing.T, s Store, w Writer) error {
			r := mustPost(t, s, message("first"), w)
			in := message("changed")
			in.ID = r.ID
			_, e := s.Post(in, w)
			return e
		}, "record id already used for a different post"},
		{"other tenure id", func(t *testing.T, s Store, w Writer) error {
			r := mustPost(t, s, message("first"), w)
			in := message("changed")
			in.ID = r.ID
			in.Wake = testTask + "/1"
			w.Worker = "other@" + testMachine
			w.Author.ID = "other"
			_, e := s.Post(in, w)
			return e
		}, "record id already used for a different post"},
		{"assignment", func(t *testing.T, s Store, w Writer) error {
			w.Author.Role = "worker"
			w.Lease = "J-1"
			w.Fence = 2
			a := 1
			testLease(t, s, w, &a)
			_, e := s.Post(Input{Type: "ack", Job: "J-1", Attempt: 1, After: []string{testTask}, Body: map[string]any{}}, w)
			return e
		}, "ack requires its counted assignment in after"},
		{"guest", func(t *testing.T, s Store, w Writer) error {
			w.Author.Role = "guest"
			_, e := s.Post(Input{Type: "answer", Re: testTask, Body: map[string]any{"answer": "yes"}}, w)
			return e
		}, "guest answer must name a question addressed to that guest"},
		{"first task record", func(t *testing.T, s Store, w Writer) error {
			w.Lease = "task"
			w.Fence = 1
			testLease(t, s, w, nil)
			_, e := s.Post(message("example"), w)
			return e
		}, "first record of a tenure must be handover"},
		{"first job record", func(t *testing.T, s Store, w Writer) error {
			w.Lease = "J-1"
			w.Fence = 2
			w.Author.Role = "worker"
			a := 1
			testLease(t, s, w, &a)
			_, e := s.Post(Input{Type: "worker-exit", Job: "J-1", Attempt: 1, Body: map[string]any{}}, w)
			return e
		}, "first record of a tenure must be ack"},
		{"envelope", func(t *testing.T, s Store, w Writer) error {
			in := message("example")
			for i := 0; i < 1200; i++ {
				in.After = append(in.After, fmt.Sprintf("%026d", i))
			}
			_, e := s.Post(in, w)
			return e
		}, "envelope exceeds 32 KB even with body reference"},
		{"spilled body conflict", func(t *testing.T, s Store, w Writer) error {
			in := Input{Type: "knowledge", Body: map[string]any{"title": "Example", "text": strings.Repeat("x", MaxLine)}}
			in.ID = testTask
			if e := s.saveBody(in.ID, []byte(`{}`)); e != nil {
				t.Fatal(e)
			}
			_, e := s.Post(in, w)
			return e
		}, "body id already used"},
		{"handover lease", func(t *testing.T, s Store, w Writer) error { _, e := s.Handover(Input{}, w); return e }, "handover requires an already granted task lease"},
		{"handover fence", func(t *testing.T, s Store, w Writer) error {
			w.Lease = "task"
			_, e := s.Handover(Input{}, w)
			return e
		}, "handover requires an already granted task lease"},
		{"handover propagated", func(t *testing.T, s Store, w Writer) error {
			w.Lease = "task"
			w.Fence = 1
			_, e := s.Handover(Input{ID: "invalid"}, w)
			return e
		}, "$.id: pattern ^[0-7][0-9A-HJKMNP-TV-Z]{25}$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			err := tc.run(t, s, testWriter())
			if !errors.Is(err, ErrInvalidRecord) {
				t.Errorf("not a validation rejection: %v", err)
			}
			if err == nil || err.Error() != tc.text {
				t.Errorf("error = %v, want %q", err, tc.text)
			}
		})
	}
}

func TestInvalidRecordUnwrap(t *testing.T) {
	cause := errors.New("original diagnostic")
	err := fmt.Errorf("outer: %w", &InvalidRecordError{Err: cause})
	if !errors.Is(err, ErrInvalidRecord) || !errors.Is(err, cause) {
		t.Fatal(err)
	}
	var rejection *InvalidRecordError
	if !errors.As(err, &rejection) || rejection.Error() != cause.Error() {
		t.Fatal(err)
	}
}

func TestRecordOperationalErrors(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	w.Lease = "task"
	w.Fence = 1
	if _, err := s.Handover(Input{}, w); !errors.Is(err, ErrStaleLease) || errors.Is(err, ErrInvalidRecord) {
		t.Fatal(err)
	}
	s = testStore(t)
	if err := os.WriteFile(s.Dir(), []byte("obstruction"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(message("example"), testWriter()); err == nil || errors.Is(err, ErrInvalidRecord) {
		t.Fatal(err)
	}
}

func TestRecordDiskFull(t *testing.T) {
	s := testStore(t)
	f, err := os.Create(filepath.Join(s.TaskDir, "record.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = appendRecord(f, []byte("{}\n"), func(*os.File, []byte) (int, error) { return 0, syscall.ENOSPC })
	if !errors.Is(err, ErrDiskFull) || errors.Is(err, ErrInvalidRecord) {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil || info.Size() != 0 {
		t.Fatal(info, err)
	}
}

func TestRecordPreRejectionFixture(t *testing.T) {
	// This existing fixture predates typed rejection errors; no wire field changed.
	raw, err := os.ReadFile("../../contract/fixtures/post/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	s := testStore(t)
	s.TaskID = r.Task
	if err := os.MkdirAll(s.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	// Compact the preexisting fixture without rebuilding or adding envelope fields.
	var line bytes.Buffer
	if err := json.Compact(&line, raw); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), writerFile(r.Worker, r.Fence)), append(line.Bytes(), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 1 || len(read.Bad) != 0 || read.Records[0].ID != r.ID {
		t.Fatalf("%+v", read)
	}
}
