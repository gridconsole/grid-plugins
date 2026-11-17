// Command boardbench is the quick local measurement behind docs/research/brain-board.md
// question 2. It compares today's per-writer JSONL Feed with one markdown file per
// post, for 30 concurrent writers. Linux only (flock), stdlib only, a few seconds.
//
//	go run docs/research/testdata/boardbench/main.go
//
// It lives under testdata so ./... never builds it. It is evidence for a decision,
// not product code.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const writers = 30

func text(i int) string {
	return strings.Repeat("Checkpoint text with a [[link]] and some words. ", 6) + strconv.Itoa(i)
}

func header(seq, job, attempt int, typ, id string) string {
	return fmt.Sprintf("---\ntype: %s\njob: J-%d\nattempt: %d\nauthor: builder\nre: null\nafter: []\nseq: %d\nat: 2026-11-17T04:30:00.000Z\nid: %s\n---\n", typ, job, attempt, seq, id)
}

func writeJSONL(dir string, n int) {
	files := make([]*os.File, writers)
	for w := range files {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("w%02d@m.1.jsonl", w)))
		if err != nil {
			panic(err)
		}
		files[w] = f
	}
	for i := 0; i < n; i++ {
		rec := map[string]any{"seq": i/writers + 1, "type": "checkpoint", "job": fmt.Sprintf("J-%d", i%writers), "body": map[string]any{"text": text(i)}}
		b, _ := json.Marshal(rec)
		if _, err := files[i%writers].Write(append(b, '\n')); err != nil {
			panic(err)
		}
	}
	for _, f := range files {
		f.Close()
	}
}

func readJSONL(dir string) int {
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	n := 0
	for _, p := range files {
		data, _ := os.ReadFile(p)
		for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
			var r map[string]any
			if json.Unmarshal(line, &r) == nil {
				n++
			}
		}
	}
	return n
}

// post is one write as the decision proposes it: under the machine-folder board
// lock, read and bump the seq counter, write the file to a dot temp name, then
// publish it with link (fails if the name exists, like O_EXCL) and drop the temp.
func post(board, lockPath, counter string, job, attempt int, typ, body string, n int) error {
	l, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer l.Close()
	if err := syscall.Flock(int(l.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(l.Fd()), syscall.LOCK_UN)
	raw, _ := os.ReadFile(counter)
	seq, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	seq++
	id := fmt.Sprintf("01JD%022d", seq)
	name := fmt.Sprintf("%06d-J-%d-%s.md", seq, job, typ)
	tmp := filepath.Join(board, ".tmp", name)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(header(seq, job, attempt, typ, id) + "\n" + body + "\n"); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err := os.Link(tmp, filepath.Join(board, name)); err != nil {
		return err
	}
	os.Remove(tmp)
	return os.WriteFile(counter, []byte(strconv.Itoa(seq)), 0o600)
}

func writePosts(board, machine string, n int) (time.Duration, error) {
	if err := os.MkdirAll(filepath.Join(board, ".tmp"), 0o700); err != nil {
		return 0, err
	}
	lockPath := filepath.Join(machine, "board.lock")
	counter := filepath.Join(machine, "board.seq")
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	start := time.Now()
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < n/writers; i++ {
				if err := post(board, lockPath, counter, w, 1, "checkpoint", text(i), n); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	return time.Since(start), <-errs
}

func list(board string) []string {
	d, err := os.Open(board)
	if err != nil {
		panic(err)
	}
	names, _ := d.Readdirnames(-1)
	d.Close()
	out := names[:0]
	for _, n := range names {
		if strings.HasSuffix(n, ".md") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func readHeaders(board string, names []string) int {
	n := 0
	for _, name := range names {
		f, err := os.Open(filepath.Join(board, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		fields := map[string]string{}
		open := false
		for sc.Scan() {
			line := sc.Text()
			if line == "---" {
				if open {
					break
				}
				open = true
				continue
			}
			k, v, _ := strings.Cut(line, ": ")
			fields[k] = v
		}
		f.Close()
		if fields["type"] != "" {
			n++
		}
	}
	return n
}

// statAll is the cache check of a reader that keeps parsed posts by name and
// re-reads only a name it has not seen or whose size or mtime changed.
func statAll(board string, names []string) int {
	n := 0
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(board, name)); err == nil {
			n++
		}
	}
	return n
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func timeit(label string, reps int, fn func() int) {
	start := time.Now()
	got := 0
	for i := 0; i < reps; i++ {
		got = fn()
	}
	fmt.Printf("  %-46s %8.2f ms  (n=%d)\n", label, ms(time.Since(start))/float64(reps), got)
}

func main() {
	root, err := os.MkdirTemp("", "boardbench-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	for _, n := range []int{1200, 6000} {
		jdir := filepath.Join(root, fmt.Sprintf("feed-%d", n))
		board := filepath.Join(root, fmt.Sprintf("board-%d", n))
		machine := filepath.Join(root, fmt.Sprintf("machine-%d", n))
		for _, d := range []string{jdir, board, machine} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				panic(err)
			}
		}
		writeJSONL(jdir, n)
		took, err := writePosts(board, machine, n)
		if err != nil {
			panic(err)
		}
		names := list(board)
		seen := map[string]bool{}
		dup := 0
		for _, name := range names {
			if seen[name[:6]] {
				dup++
			}
			seen[name[:6]] = true
		}
		fmt.Printf("== %d posts by %d concurrent writers ==\n", n, writers)
		fmt.Printf("  %-46s %8.2f ms  (%.0f us/post, %d duplicate seq)\n", "write: lock + counter + temp + link", ms(took), float64(took.Microseconds())/float64(n), dup)
		timeit("today: read+parse 30 jsonl files", 5, func() int { return readJSONL(jdir) })
		timeit("posts: list board (names only)", 5, func() int { return len(list(board)) })
		timeit("posts: list + lstat every name (cache check)", 5, func() int { return statAll(board, list(board)) })
		timeit("posts: list + read+parse every header (cold)", 5, func() int { return readHeaders(board, list(board)) })
		timeit("posts: list + read 30 newest (warm cache)", 5, func() int { l := list(board); return readHeaders(board, l[len(l)-30:]) })
	}
}
