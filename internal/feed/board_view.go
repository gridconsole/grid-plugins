package feed

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/board/postfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
)

// BoardPresent says whether the task folder holds a per-post board: a board/ folder.
// Nothing on a JSONL home makes one, so until a task is migrated (B3) the answer is
// false and every caller keeps the JSONL Feed.
func BoardPresent(taskDir string) bool {
	info, err := os.Stat(filepath.Join(taskDir, "board"))
	return err == nil && info.IsDir()
}

// Boarded is s on its per-post board when the task folder has one (BoardPresent), and
// s unchanged otherwise. rel and login are the BoardHome's: the task folder's path
// from the brain root, and the login that writes. A reader passes neither.
func (s Store) Boarded(rel, login string) Store {
	if s.Board == nil && BoardPresent(s.TaskDir) {
		s.Board = &BoardHome{Rel: rel, Login: login}
	}
	return s
}

// BoardRecord is a record as the CLI and MCP print it for a per-post board
// (docs/research/brain-board.md Q3). It keeps id, type, seq, attempt, author and body,
// gains by, and loses lease, fence and gen; worker stays for one release as an alias of
// by. A JSONL Feed prints Record and Entry as before, so this is used on a board only.
type BoardRecord struct {
	ID      string             `json:"id"`
	Task    string             `json:"task"`
	Type    string             `json:"type"`
	TypeV   int                `json:"type_v"`
	Author  Author             `json:"author"`
	Tier    contract.TrustTier `json:"tier"`
	By      string             `json:"by"`
	Worker  string             `json:"worker"`
	Seq     int64              `json:"seq"`
	At      string             `json:"at"`
	Re      *string            `json:"re"`
	After   []string           `json:"after"`
	Job     *string            `json:"job"`
	Attempt *int               `json:"attempt"`
	Wake    *string            `json:"wake"`
	Body    Body               `json:"body"`
}

// BoardEntry is an Entry as the CLI and MCP print it for a board: the BoardRecord, the
// reader's verdict and the post's file name. It has no end, which is a JSONL offset.
type BoardEntry struct {
	BoardRecord
	Arrived string   `json:"arrived"`
	File    string   `json:"file"`
	Verdict string   `json:"verdict"`
	Waiting []string `json:"waiting"`
	Data    bool     `json:"data"`
	Reason  string   `json:"reason"`
}

// BoardReadResult is a ReadResult as the CLI and MCP print it for a board.
type BoardReadResult struct {
	Records []BoardEntry   `json:"records"`
	Bad     map[string]int `json:"bad"`
}

// BoardView is r as Q3 gives it. The record of a board has its by in Worker.
func (r Record) BoardView() BoardRecord {
	return BoardRecord{ID: r.ID, Task: r.Task, Type: r.Type, TypeV: r.TypeV, Author: r.Author, Tier: r.Tier, By: r.Worker, Worker: r.Worker, Seq: r.Seq, At: r.At, Re: r.Re, After: r.After, Job: r.Job, Attempt: r.Attempt, Wake: r.Wake, Body: r.Body}
}

// BoardFile is the file name of the post r on a board.
func (r Record) BoardFile() string { return postfile.Name(r.Seq, value(r.Job), r.Type) }

// BoardView is e as Q3 gives it. An entry that has no file name gets the one its seq,
// Job and type make.
func (e Entry) BoardView() BoardEntry {
	file := e.File
	if file == "" {
		file = e.BoardFile()
	}
	return BoardEntry{BoardRecord: e.Record.BoardView(), Arrived: e.Arrived, File: file, Verdict: e.Verdict, Waiting: e.Waiting, Data: e.Data, Reason: e.Reason}
}

// BoardView is r as Q3 gives it.
func (r ReadResult) BoardView() BoardReadResult {
	out := BoardReadResult{Records: make([]BoardEntry, 0, len(r.Records)), Bad: r.Bad}
	for _, e := range r.Records {
		out.Records = append(out.Records, e.BoardView())
	}
	return out
}
