package postfile

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var pbStUpdate = flag.Bool("pbst.update", false, "rewrite the render goldens")

func pbStInt(n int) *int { return &n }

func pbStHeader(id, job, typ string, body string) Header {
	return Header{
		ID: id, Task: "01JD7P0A1B2C3D4E5F6G7H8J9K", Type: typ, TypeV: 1, Job: job,
		Attempt: pbStInt(1), At: "2026-11-17T01:52:10.123Z",
		Author: Author{ID: "J-5", Role: "agent"}, By: "01JDMACHINE0000000000000000:grid3",
		After: []string{}, Body: []byte(body),
	}
}

func pbStDir(t *testing.T) Dir {
	t.Helper()
	root := t.TempDir()
	return Dir{
		Board: filepath.Join(root, "brain", "projects", "demo", "tasks", "T-7", "board"),
		Cache: filepath.Join(root, "machine", "tasks", "01JD7P0A1B2C3D4E5F6G7H8J9K"),
		Rel:   "projects/demo/tasks/T-7",
	}
}

func pbStPublish(t *testing.T, d Dir, h Header, names map[string]string) (Header, string) {
	t.Helper()
	l, err := d.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Unlock()
	h, name, err := l.Publish(h, names)
	if err != nil {
		t.Fatal(err)
	}
	return h, name
}

// V15: the header round-trips byte-exactly, with exit code 2.0 and a 64 KB body.
func TestPbStHeaderRoundTripsByteExactly(t *testing.T) {
	big := strings.Repeat("x", 64<<10)
	h := pbStHeader("01JD7Q4ZK0V8M2T6Y1N3B5C9XE", "J-5", "exit",
		`{"reason":"done","code":2.0,"log":"`+big+`","note":"a \"quoted\" line\nnext"}`)
	h.Author = Author{ID: "Ann Lee", Role: "person-team"}
	h.By = "01JDMACHINE0000000000000000:ann"
	h.Re = "01JD7Q1W3X5Y7Z9A2B4C6D8E0F"
	h.After = []string{"01JD7Q1W3X5Y7Z9A2B4C6D8E0A", "true", "1.5"}
	h.Legacy = "counted"
	h.Gen = 3
	h.Seq = 42
	enc, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	md := []byte("# anything\n\nbody text\n")
	got, rest, err := Decode(append(append([]byte{}, enc...), md...))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rest, md) {
		t.Fatalf("markdown = %q", rest)
	}
	again, err := got.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, enc) {
		t.Fatalf("re-encode differs:\n%s\n---\n%s", again[:200], enc[:200])
	}
	if !bytes.Contains(got.Body, []byte(`"code":2.0`)) || len(got.Body) < 64<<10 {
		t.Fatalf("body lost bytes: %d", len(got.Body))
	}
	if got.Legacy != "counted" || got.Author.ID != "Ann Lee" || len(got.After) != 3 || got.After[1] != "true" || *got.Attempt != 1 {
		t.Fatalf("decoded %+v", got)
	}
	// A header that is not in its written form is refused, so a decode always
	// re-encodes to the same bytes.
	bad := bytes.Replace(enc, []byte("gen: 3\n"), []byte("gen: 03\n"), 1)
	if _, _, err := Decode(bad); err == nil {
		t.Fatal("non-canonical header decoded")
	}
}

// Q8's example header is what Encode writes for it.
func TestPbStHeaderMatchesQ8Example(t *testing.T) {
	h := pbStQ8Header()
	enc, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := `---
id: 01JD7Q4ZK0V8M2T6Y1N3B5C9XE
task: 01JD7P0A1B2C3D4E5F6G7H8J9K
type: question
type_v: 1
job: J-5
attempt: 1
gen: 0
seq: 42
at: 2026-11-17T01:52:10.123Z
author: {id: J-5, role: agent}
by: grid2:grid3
for: null
re: 01JD7Q1W3X5Y7Z9A2B4C6D8E0F
after: []
body: {"question":"Should the inbox keep one section per machine?","options":["yes","no"],"why":"Two homes share one inbox note.","blocking":true}
---
`
	if string(enc) != want {
		t.Fatalf("header:\n%s", enc)
	}
}

func pbStQ8Header() Header {
	h := pbStHeader("01JD7Q4ZK0V8M2T6Y1N3B5C9XE", "J-5", "question",
		`{"question":"Should the inbox keep one section per machine?","options":["yes","no"],"why":"Two homes share one inbox note.","blocking":true}`)
	h.By = "grid2:grid3"
	h.Seq = 42
	h.Re = "01JD7Q1W3X5Y7Z9A2B4C6D8E0F"
	return h
}

// V15: the body under the header is Q8's rendering; the Q8 example renders as written.
func TestPbStRenderMatchesQ8Example(t *testing.T) {
	got := Render(pbStQ8Header(), "projects/demo/tasks/T-7/board/000042-J-5-question.md",
		map[string]string{"01JD7Q1W3X5Y7Z9A2B4C6D8E0F": "000041-J-5-result.md"})
	want := `# J-5 question: Should the inbox keep one section per machine?

- Job: [[projects/demo/tasks/T-7/jobs/J-5|J-5]], attempt 1
- From: agent J-5, by grid2:grid3, 2026-11-17 01:52 UTC
- About: [[projects/demo/tasks/T-7/board/000041-J-5-result|J-5 result]]

Two homes share one inbox note.

- yes
- no

Blocking: yes

## Links

- [[projects/demo/tasks/T-7/task|T-7]] - related: the task this board belongs to
- [[projects/demo/tasks/T-7/jobs/J-5|J-5]] - continues: the Job this question comes from
- [[projects/demo/tasks/T-7/board/000041-J-5-result|J-5 result]] - continues: the result the question is about
`
	if string(got) != want {
		t.Fatalf("render:\n%s", got)
	}
}

// V15: a golden per post type, with path-qualified links to task.md, the Job node and
// the re post; rendering twice from the same inputs gives the same bytes.
func TestPbStRenderGoldenPerType(t *testing.T) {
	const (
		assign   = "01JDASSIGN00000000000000000"
		question = "01JDQUESTION000000000000000"
		result   = "01JDRESULT00000000000000000"
	)
	names := map[string]string{
		assign:   "000001-J-5-control.md",
		question: "000007-J-5-question.md",
		result:   "000006-J-5-result.md",
	}
	cases := []struct {
		name, job, typ, re, body string
		after                    []string
		gen                      int64
	}{
		{"assign", "J-5", "control", "", `{"action":"assign","to":"builder@01JDMACHINE0000000000000000","attempt":1,"start":"now","retry_of":null}`, nil, 0},
		{"started", "J-5", "started", assign, `{"firing":"f1"}`, nil, 0},
		{"exit", "J-5", "exit", assign, `{"reason":"done","code":2.0,"cost":{"usd":0.42}}`, nil, 0},
		{"result", "J-5", "result", assign, `{"outcome":"Built the board store.\n## Links\nwritten by hand","changed":["a.go","b.go"],"acceptance":["row 1 HOLDS"],"must_fix":[],"proof":[{"path":"shot.png"}]}`, nil, 0},
		{"checkpoint", "J-5", "checkpoint", assign, `{"step":"Half way there","commit":"abc123","acceptance":["row 2"]}`, nil, 0},
		{"question", "J-5", "question", result, `{"question":"Keep one section?","options":["yes","no"],"why":"Two homes share one note.","who":"owner","blocking":false}`, nil, 0},
		{"answer", "J-5", "answer", question, `{"answer":"yes","note":"Keep one.","reason":"simpler"}`, nil, 0},
		{"waiting", "J-5", "waiting", assign, `{"unmet":["after:J-4"],"measured":{"load":0.5}}`, nil, 0},
		{"cancel", "J-5", "cancel", assign, `{"reason":"the person asked"}`, []string{result}, 0},
		{"task-move", "", "task-move", "", `{"from":"01JDMACHINEA000000000000000","to":"01JDMACHINEB000000000000000"}`, nil, 1},
		{"post", "", "post", question, `{"kind":"note","text":"First line\nsecond line"}`, nil, 0},
		{"other", "J-5", "heartbeat", "", `{"at":"now","n":3}`, nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := pbStHeader("01JD7Q4ZK0V8M2T6Y1N3B5C9XE", c.job, c.typ, c.body)
			h.Re = c.re
			h.After = c.after
			h.Gen = c.gen
			h.Seq = 9
			own := "projects/demo/tasks/T-7/board/" + Name(9, c.job, c.typ)
			got := Render(h, own, names)
			if again := Render(h, own, names); !bytes.Equal(got, again) {
				t.Fatal("two renders differ")
			}
			golden := filepath.Join("testdata", "render", c.name+".md")
			if *pbStUpdate {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("render differs from %s:\n%s", golden, got)
			}
			if !strings.Contains(string(got), "[[projects/demo/tasks/T-7/task|T-7]] - related:") {
				t.Fatal("no task.md link")
			}
			if c.job != "" && !strings.Contains(string(got), "[[projects/demo/tasks/T-7/jobs/J-5|J-5]] - continues:") {
				t.Fatal("no Job node link")
			}
			if c.re != "" && !strings.Contains(string(got), "[[projects/demo/tasks/T-7/board/"+strings.TrimSuffix(names[c.re], ".md")+"|") {
				t.Fatal("no re link")
			}
			if strings.Count(string(got), "\n## Links\n") != 1 {
				t.Fatal("not one Links section")
			}
		})
	}
}

// V15: an unknown re name renders as the id, with no link.
func TestPbStRenderUnknownReIsTheId(t *testing.T) {
	h := pbStQ8Header()
	got := string(Render(h, "projects/demo/tasks/T-7/board/000042-J-5-question.md", nil))
	if !strings.Contains(got, "- About: 01JD7Q1W3X5Y7Z9A2B4C6D8E0F\n") || !strings.Contains(got, "- re: 01JD7Q1W3X5Y7Z9A2B4C6D8E0F - continues:") {
		t.Fatalf("unknown re:\n%s", got)
	}
	if strings.Contains(got, "/board/") {
		t.Fatalf("unknown re has a link:\n%s", got)
	}
}

// V15: 30 concurrent goroutines give unique names, published by link.
func TestPbStThirtyGoroutinesUniqueNamesByLink(t *testing.T) {
	d := pbStDir(t)
	var links atomic.Int64
	defer func(old func(string, string) error) { link = old }(link)
	link = func(o, n string) error { links.Add(1); return os.Link(o, n) }
	var wg sync.WaitGroup
	got := make([]string, 30)
	errs := make([]error, 30)
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := d.Lock(context.Background())
			if err != nil {
				errs[i] = err
				return
			}
			defer l.Unlock()
			_, got[i], errs[i] = l.Publish(pbStHeader(pbStID(i), "J-5", "checkpoint", `{"step":"s"}`), nil)
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	for i, n := range got {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if seen[n] {
			t.Fatalf("name %s twice", n)
		}
		seen[n] = true
	}
	if links.Load() != 30 {
		t.Fatalf("links = %d, want 30", links.Load())
	}
	listing, err := d.List()
	if err != nil || len(listing.Posts) != 30 || len(listing.Bad) != 0 {
		t.Fatalf("listing %d posts, bad %v, err %v", len(listing.Posts), listing.Bad, err)
	}
	for i, p := range listing.Posts {
		if p.Header.Seq != int64(i+1) || p.Name != Name(int64(i+1), "J-5", "checkpoint") {
			t.Fatalf("post %d is %s seq %d", i, p.Name, p.Header.Seq)
		}
	}
	if tmps, _ := os.ReadDir(filepath.Join(d.Board, ".tmp")); len(tmps) != 0 {
		t.Fatalf("temps left: %d", len(tmps))
	}
}

// pbStID is a distinct 26-character id for post i.
func pbStID(i int) string {
	s := "01JDPOST" + strings.Repeat("0", 18)
	n := []byte(s)
	for p := len(n) - 1; i > 0; p-- {
		n[p] = "0123456789ABCDEFGHJKMNPQRS"[i%26]
		i /= 26
	}
	return string(n)
}

// A name that appears between the seq and the link (a post that synced in) makes the
// post take the next seq.
func TestPbStExistingNameTakesNextSeq(t *testing.T) {
	d := pbStDir(t)
	defer func(old func(string, string) error) { link = old }(link)
	first := true
	link = func(o, n string) error {
		if first {
			first = false
			if err := os.WriteFile(n, []byte("synced\n"), 0o600); err != nil {
				return err
			}
		}
		return os.Link(o, n)
	}
	h, name := pbStPublish(t, d, pbStHeader(pbStID(1), "J-5", "started", `{}`), nil)
	if h.Seq != 2 || name != "000002-J-5-started.md" {
		t.Fatalf("published %s seq %d", name, h.Seq)
	}
}

// F12: with no hard links the post is created with O_EXCL, written and synced.
func TestPbStNoHardLinksFallsBackToExclusiveCreate(t *testing.T) {
	d := pbStDir(t)
	defer func(old func(string, string) error) { link = old }(link)
	link = func(string, string) error { return &os.LinkError{Op: "link", Err: errors.ErrUnsupported} }
	pbStPublish(t, d, pbStHeader(pbStID(1), "J-5", "started", `{}`), nil)
	pbStPublish(t, d, pbStHeader(pbStID(2), "J-5", "started", `{}`), nil)
	listing, err := d.List()
	if err != nil || len(listing.Posts) != 2 || listing.Posts[1].Header.Seq != 2 {
		t.Fatalf("listing %+v %v", listing, err)
	}
}

// F2: the next poster removes temps older than an hour and keeps newer ones.
func TestPbStPosterRemovesOldTemps(t *testing.T) {
	d := pbStDir(t)
	tmp := filepath.Join(d.Board, ".tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	old, young := filepath.Join(tmp, "000001-J-5-started.md.1"), filepath.Join(tmp, "000001-J-5-started.md.2")
	for _, p := range []string{old, young} {
		if err := os.WriteFile(p, []byte("---\nid: half"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	pbStPublish(t, d, pbStHeader(pbStID(1), "J-5", "started", `{}`), nil)
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old temp kept: %v", err)
	}
	if _, err := os.Stat(young); err != nil {
		t.Fatalf("young temp removed: %v", err)
	}
	listing, _ := d.List()
	if len(listing.Posts) != 1 || len(listing.Bad) != 0 {
		t.Fatalf("a temp was listed: %+v", listing)
	}
}

// F3: an unterminated header is bad for this pass and not cached, so the next pass
// reads the file again.
func TestPbStUnterminatedHeaderIsReadAgain(t *testing.T) {
	d := pbStDir(t)
	h := pbStHeader(pbStID(1), "J-5", "started", `{}`)
	h.Seq = 1
	full, err := d.file(h, "000001-J-5-started.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(d.Board, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d.Board, "000001-J-5-started.md")
	if err := os.WriteFile(p, full[:bytes.Index(full, []byte("\n---\n"))], 0o600); err != nil {
		t.Fatal(err)
	}
	listing, err := d.List()
	if err != nil || len(listing.Posts) != 0 || !strings.Contains(listing.Bad["000001-J-5-started.md"], "closing") {
		t.Fatalf("first pass %+v %v", listing, err)
	}
	if err := os.WriteFile(p, full, 0o600); err != nil {
		t.Fatal(err)
	}
	listing, err = d.List()
	if err != nil || len(listing.Posts) != 1 || len(listing.Bad) != 0 {
		t.Fatalf("second pass %+v %v", listing, err)
	}
}

// F6: an edited post is a finding and is read as it is now; a deleted post is a
// finding and drops out. Editing only the markdown leaves the header as it was.
func TestPbStEditedAndDeletedPostsAreFindings(t *testing.T) {
	d := pbStDir(t)
	_, a := pbStPublish(t, d, pbStHeader(pbStID(1), "J-5", "started", `{}`), nil)
	_, b := pbStPublish(t, d, pbStHeader(pbStID(2), "J-5", "checkpoint", `{"step":"one"}`), nil)
	if _, err := d.List(); err != nil {
		t.Fatal(err)
	}
	pa := filepath.Join(d.Board, a)
	data, _ := os.ReadFile(pa)
	head := data[:bytes.Index(data, []byte("\n---\n"))+5]
	if err := os.WriteFile(pa, append(append([]byte{}, head...), "edited by hand\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(d.Board, b)); err != nil {
		t.Fatal(err)
	}
	found, err := d.Check()
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{{Name: a, Kind: "edited post"}, {Name: b, Kind: "deleted post"}}
	if len(found) != 2 || found[0] != want[0] || found[1] != want[1] {
		t.Fatalf("findings %+v", found)
	}
	listing, _ := d.List()
	if len(listing.Posts) != 1 || listing.Posts[0].Name != a || listing.Posts[0].Header.ID != pbStID(1) {
		t.Fatalf("listing %+v", listing)
	}
}

// V15: an F4 renumber rewrites the links inside the renumbered posts and inside every
// unaccepted post that links to them.
func TestPbStRenumberRewritesLinks(t *testing.T) {
	d := pbStDir(t)
	h1, n1 := pbStPublish(t, d, pbStHeader(pbStID(1), "J-5", "result", `{"outcome":"done"}`), nil)
	h2 := pbStHeader(pbStID(2), "J-5", "question", `{"question":"why?"}`)
	h2.Re = h1.ID
	h2, n2 := pbStPublish(t, d, h2, map[string]string{h1.ID: n1})
	h3 := pbStHeader(pbStID(3), "J-5", "answer", `{"answer":"because"}`)
	h3.Re = h2.ID
	h3, n3 := pbStPublish(t, d, h3, map[string]string{h1.ID: n1, h2.ID: n2})
	h4 := pbStHeader(pbStID(4), "J-5", "checkpoint", `{"step":"next"}`)
	h4.After = []string{h3.ID}
	_, n4 := pbStPublish(t, d, h4, map[string]string{h3.ID: n3})
	before1, _ := os.ReadFile(filepath.Join(d.Board, n1))
	l, err := d.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	moved, err := l.Renumber([]string{n2, n3}, []string{n2, n3, n4})
	l.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if moved[n2] != "000005-J-5-question.md" || moved[n3] != "000006-J-5-answer.md" {
		t.Fatalf("moved %v", moved)
	}
	read := func(n string) (Header, string) {
		data, err := os.ReadFile(filepath.Join(d.Board, n))
		if err != nil {
			t.Fatal(err)
		}
		h, md, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		return h, string(md)
	}
	g2, md2 := read(moved[n2])
	g3, md3 := read(moved[n3])
	g4, md4 := read(n4)
	if g2.ID != h2.ID || g2.Seq != 5 || g3.ID != h3.ID || g3.Seq != 6 || g4.Seq != 4 {
		t.Fatalf("headers %+v %+v %+v", g2, g3, g4)
	}
	if !strings.Contains(md2, "board/000001-J-5-result|J-5 result]]") {
		t.Fatalf("moved question lost its link:\n%s", md2)
	}
	if !strings.Contains(md3, "board/000005-J-5-question|J-5 question]] - decides:") {
		t.Fatalf("moved answer links the old name:\n%s", md3)
	}
	if !strings.Contains(md4, "board/000006-J-5-answer|J-5 answer]] - depends-on:") {
		t.Fatalf("unaccepted post links the old name:\n%s", md4)
	}
	for _, old := range []string{n2, n3} {
		if _, err := os.Stat(filepath.Join(d.Board, old)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old name %s kept", old)
		}
	}
	if after1, _ := os.ReadFile(filepath.Join(d.Board, n1)); !bytes.Equal(before1, after1) {
		t.Fatal("an accepted post changed")
	}
	if found, err := d.Check(); err != nil || len(found) != 0 {
		t.Fatalf("renumber left findings %+v %v", found, err)
	}
}
