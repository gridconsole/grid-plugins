package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/board/postfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
)

// BoardHome makes a Store the facade of a task's per-post board
// (docs/research/brain-board.md B1, Q2 and the Go API). The posts are files under
// <TaskDir>/board/, one per post (internal/board/postfile). The caches and locks are
// in the machine folder, tasks/<TaskID>/: board.lock, board.seq, board.cache.json,
// fold.lock, state.json and needs-you.json (decision point 6). Nothing of the task is
// written to feed/. The task id comes from task.md when TaskID is empty; task.md's
// home: is never read, so the home is the one the posts give (decision point 4).
//
// On a board, Post, Read, ReadOnly, State, SaveState, RefoldPublished,
// SaveStateLiveness, Home and PostUnless serve the board. The lease, gate and
// handover calls are the JSONL Feed's and are not served.
type BoardHome struct {
	// Rel is the task folder's path from the brain root, in slashes, for the links
	// in the body of each post.
	Rel string
	// By is <machine>:<login>, the by of every post this Store writes.
	By string
	// Login is the login of this process. When By is empty, a post's by is the machine
	// of its writer (the part of Writer.Worker after the @), a colon and Login, so a
	// caller that resolves the Store before it knows the writer sets this instead.
	Login string
}

var errNoMachineFolder = errors.New("a per-post board keeps its caches and locks in the machine folder, and it is not known")

// boardID is s with its TaskID read from task.md when empty.
func (s Store) boardID() (Store, error) {
	if s.TaskID == "" {
		id, err := TaskID(s.TaskDir)
		if err != nil {
			return s, err
		}
		s.TaskID = id
	}
	if !IsID(s.TaskID) {
		return s, fmt.Errorf("task id must be a ULID")
	}
	return s, nil
}

// boardCache is the task's folder in the machine folder.
func (s Store) boardCache() (string, error) {
	if s.Machine.Path == "" {
		return "", errNoMachineFolder
	}
	return s.Machine.Join("tasks", s.TaskID)
}

// boardDir is the task's board: board/ in the task folder and its cache folder.
func (s Store) boardDir() (postfile.Dir, error) {
	s, err := s.boardID()
	if err != nil {
		return postfile.Dir{}, err
	}
	cache, err := s.boardCache()
	if err != nil {
		return postfile.Dir{}, err
	}
	return postfile.Dir{Board: filepath.Join(s.TaskDir, "board"), Cache: cache, Rel: s.Board.Rel, MkdirAll: s.Machine.MkdirAll, WriteFile: s.Machine.WriteFile}, nil
}

// boardPost is a post file's header as the fold reads it: the by stands in the
// worker, and a board post has no lease, fence or wake.
func boardPost(h postfile.Header) contract.BoardPost {
	p := contract.Post{ID: h.ID, Task: h.Task, Type: h.Type, TypeV: h.TypeV, Author: Author{ID: h.Author.ID, Role: h.Author.Role}, Tier: contract.TierFor(h.Author.Role), Worker: h.By, Gen: h.Gen, Seq: h.Seq, At: h.At, After: h.After, Attempt: h.Attempt, Body: json.RawMessage(h.Body)}
	if h.Job != "" {
		p.Job = ptr(h.Job)
	}
	if h.Re != "" {
		p.Re = ptr(h.Re)
	}
	if p.After == nil {
		p.After = []string{}
	}
	return contract.BoardPost{Post: p, By: h.By, Legacy: h.Legacy}
}

// boardRecord is the record of a post file. Its worker is the by, the alias of
// brain-board.md Q3.
func boardRecord(h postfile.Header) (Record, error) {
	p := boardPost(h).Post
	var body map[string]any
	if err := decodeBody(p.Body, &body); err != nil || body == nil {
		return Record{}, fmt.Errorf("post body is not a JSON object")
	}
	return Record{ID: p.ID, Task: p.Task, Type: p.Type, TypeV: p.TypeV, Author: p.Author, Tier: p.Tier, Worker: p.Worker, Gen: p.Gen, Seq: p.Seq, At: p.At, Re: p.Re, After: p.After, Job: p.Job, Attempt: p.Attempt, Body: body}, nil
}

// boardRead reads the board in (seq, id) order (F5). A post whose header cannot be
// read is bad for this pass; a second file with an id already read is left out. A
// post reads counted, unknown for a type or version this build does not know, or
// held while a post it names is not on the board. It answers, by id, the posts the
// fold reads: the counted and unknown ones, as a JSONL read gives the fold. save
// lets the names cache be updated; ReadOnly does not.
func (s Store) boardRead(filter Filter, save bool) (ReadResult, []contract.BoardPost, error) {
	out := ReadResult{Records: []Entry{}, Bad: map[string]int{}}
	d, err := s.boardDir()
	if err != nil {
		return out, nil, err
	}
	list := d.List
	if !save {
		list = d.Scan
	}
	listing, err := list()
	if err != nil {
		return out, nil, err
	}
	for name := range listing.Bad {
		out.Bad[name]++
	}
	sort.SliceStable(listing.Posts, func(i, j int) bool {
		a, b := listing.Posts[i].Header, listing.Posts[j].Header
		if a.Seq != b.Seq {
			return a.Seq < b.Seq
		}
		return a.ID < b.ID
	})
	seen := map[string]bool{}
	byID := map[string]contract.BoardPost{}
	for _, p := range listing.Posts {
		if seen[p.Header.ID] {
			continue
		}
		r, err := boardRecord(p.Header)
		if err != nil {
			out.Bad[p.Name]++
			continue
		}
		seen[r.ID] = true
		e := Entry{Record: r, File: p.Name, Verdict: "counted", Data: r.Data()}
		if t, known := contract.LookupType(r.Type); !known || t.Version != r.TypeV {
			e.Verdict, e.Reason = "unknown", "unknown type or type version"
		}
		out.Records = append(out.Records, e)
		byID[r.ID] = boardPost(p.Header)
	}
	posts := []contract.BoardPost{}
	for i := range out.Records {
		e := &out.Records[i]
		refs := append([]string{}, e.After...)
		if e.Re != nil {
			refs = append(refs, *e.Re)
		}
		for _, ref := range refs {
			if !seen[ref] {
				e.Waiting = append(e.Waiting, ref)
			}
		}
		if len(e.Waiting) > 0 {
			e.Verdict, e.Reason = "held", "waiting for record"
			continue
		}
		posts = append(posts, byID[e.ID])
	}
	read, err := applyFilter(out, filter)
	return read, posts, err
}

// boardFold folds the board with the board's rules (contract.FoldBoard) and the
// questions as on a JSONL feed.
func (s Store) boardFold() (contract.State, contract.Loop, error) {
	read, posts, err := s.boardRead(Filter{}, true)
	if err != nil {
		return contract.State{}, contract.Loop{}, err
	}
	state, loop := contract.FoldBoard(posts)
	allowed := s.WithdrawAllowed
	if allowed == nil {
		allowed = QuestionWithdrawReadPolicy(s.TaskDir, s.Warn)
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	foldQuestions(read, &state, now, allowed)
	return state, loop, nil
}

// Home is the machine that holds the task now, as the fold derives it from the posts:
// the home of the highest gen they name (decision point 4). task.md's home: is a
// display copy and is not read. A JSONL feed has no home and answers "".
func (s Store) Home() (string, error) {
	if s.Board == nil {
		return "", nil
	}
	_, loop, err := s.boardFold()
	return loop.Home, err
}

// PostUnless is Post on a per-post board, unless a post for which held is true is on
// the board already: then it writes nothing and answers that post's id. The look and
// the write are one hold of the board lock, which every writer of the board on this
// machine takes, so of the callers in any of its processes one writes. It is the
// Claim of the board adapter (internal/board PerPost).
func (s Store) PostUnless(in Input, w Writer, held func(Record) bool) (Record, string, error) {
	if s.Board == nil {
		return Record{}, "", errors.New("PostUnless needs a per-post board")
	}
	return s.boardPublish(in, w, held)
}

// boardHeader is the header of a new post. Its seq is set when it is published.
func (s Store) boardHeader(in Input, w Writer) (postfile.Header, error) {
	invalid := func(format string, args ...any) (postfile.Header, error) {
		return postfile.Header{}, &InvalidRecordError{Err: fmt.Errorf(format, args...)}
	}
	by := s.Board.By
	if by == "" && s.Board.Login != "" {
		if _, machine, ok := strings.Cut(w.Worker, "@"); ok && machine != "" {
			by = machine + ":" + s.Board.Login
		}
	}
	if by == "" {
		return invalid("a board post needs its by, <machine>:<login>")
	}
	t, known := contract.LookupType(in.Type)
	if !known {
		return invalid("unknown post type %q", in.Type)
	}
	id := in.ID
	if id == "" {
		var err error
		if id, err = NewID(); err != nil {
			return postfile.Header{}, err
		}
	} else if !IsID(id) {
		return invalid("record id must be a ULID")
	}
	body := in.Body
	if body == nil {
		body = map[string]any{}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return invalid("post body: %v", err)
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	h := postfile.Header{ID: id, Task: s.TaskID, Type: in.Type, TypeV: t.Version, Job: in.Job, Gen: w.Gen, At: contract.FormatTime(now), Author: postfile.Author{ID: w.Author.ID, Role: w.Author.Role}, By: by, Re: in.Re, After: append([]string{}, in.After...), Body: data}
	if in.Attempt > 0 {
		attempt := in.Attempt
		h.Attempt = &attempt
	}
	return h, nil
}

// boardPublish writes one post file under the board lock: the next seq, a temp file
// linked to its name (postfile.Locked.Publish). A post the fold would not read as
// valid is refused, as a JSONL Post refuses it. A retry with an id on the board
// answers that post when it is the same request, and is refused otherwise.
func (s Store) boardPublish(in Input, w Writer, held func(Record) bool) (Record, string, error) {
	s, err := s.boardID()
	if err != nil {
		return Record{}, "", err
	}
	d, err := s.boardDir()
	if err != nil {
		return Record{}, "", err
	}
	h, err := s.boardHeader(in, w)
	if err != nil {
		return Record{}, "", err
	}
	check := boardPost(h)
	check.Seq = 1 // Publish sets the seq; the schema wants one.
	if err := contract.ValidBoardPost(check); err != nil {
		return Record{}, "", &InvalidRecordError{Err: err}
	}
	ctx := s.Context
	if ctx == nil {
		ctx = context.Background()
	}
	lock, err := d.Lock(ctx)
	if err != nil {
		return Record{}, "", err
	}
	defer lock.Unlock()
	listing, err := d.List()
	if err != nil {
		return Record{}, "", err
	}
	names := map[string]string{}
	for _, p := range listing.Posts {
		if _, ok := names[p.Header.ID]; !ok {
			names[p.Header.ID] = p.Name
		}
		if p.Header.ID == h.ID {
			if !sameBoardRequest(p.Header, h) {
				return Record{}, "", &InvalidRecordError{Err: &RecordIDUsedError{ID: h.ID}}
			}
			r, err := boardRecord(p.Header)
			return r, "", err
		}
		if held == nil {
			continue
		}
		if r, err := boardRecord(p.Header); err == nil && held(r) {
			return Record{}, r.ID, nil
		}
	}
	h, _, err = lock.Publish(h, names)
	if err != nil {
		return Record{}, "", err
	}
	r, err := boardRecord(h)
	return r, "", err
}

// sameBoardRequest says whether two headers of one id are the same post asked for
// twice: everything but the seq and the time the same.
func sameBoardRequest(a, b postfile.Header) bool {
	a.Seq, a.At, b.Seq, b.At = 0, "", 0, ""
	x, errA := a.Encode()
	y, errB := b.Encode()
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

// foldFiles is where saveState takes fold.lock and publishes state.json and
// needs-you.json, and how it writes them.
type foldFiles struct {
	lock, state, needsYou     string
	openLock                  func() (*os.File, error)
	writeState, writeNeedsYou func(path string, data []byte) error
}

// foldFiles on a JSONL feed are those of base: fold.lock in feed/.cache, state.json
// at Machine.StateFile, needs-you.json in feed/. On a board all three are in the
// task's folder in the machine folder, made with its modes.
func (s Store) foldFiles() (foldFiles, error) {
	if s.Board == nil {
		if err := s.prepare(); err != nil {
			return foldFiles{}, err
		}
		state, err := s.stateFile()
		if err != nil {
			return foldFiles{}, err
		}
		lock := filepath.Join(s.Dir(), ".cache", "fold.lock")
		return foldFiles{
			lock: lock, state: state, needsYou: filepath.Join(s.Dir(), "needs-you.json"),
			openLock: func() (*os.File, error) {
				if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
					return nil, err
				}
				return os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o600)
			},
			writeState:    s.Machine.WriteStateFile,
			writeNeedsYou: func(path string, data []byte) error { return atomicfile.Write(path, data, 0o600) },
		}, nil
	}
	s, err := s.boardID()
	if err != nil {
		return foldFiles{}, err
	}
	cache, err := s.boardCache()
	if err != nil {
		return foldFiles{}, err
	}
	lock := filepath.Join(cache, "fold.lock")
	return foldFiles{
		lock: lock, state: filepath.Join(cache, "state.json"), needsYou: filepath.Join(cache, "needs-you.json"),
		openLock:   func() (*os.File, error) { return s.Machine.OpenCreate(lock, os.O_RDWR) },
		writeState: s.Machine.WriteFile, writeNeedsYou: s.Machine.WriteFile,
	}, nil
}
