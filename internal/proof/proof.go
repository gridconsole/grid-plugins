// Package proof stores evidence per plan item and builds the Result projection.
package proof

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

const MaxBytes = 20 << 20

// Record describes evidence, not a Feed or task record. File is task-relative.
type Record struct {
	Kind     string `json:"kind"`
	By       string `json:"by"`
	At       string `json:"at"`
	SHA256   string `json:"sha256"`
	Note     string `json:"note"`
	File     string `json:"file"`
	PlanHash string `json:"plan_hash"`
}

type Item struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Level    string   `json:"level"`
	Proven   bool     `json:"proven"`
	Evidence []Record `json:"evidence"`
}

type Index struct {
	PlanHash string `json:"plan_hash"`
	Items    []Item `json:"items"`
}

type Store struct {
	TaskDir string
	Now     func() time.Time
}
type Input struct{ Item, File, Text, Kind, By, Note string }

func kindOK(kind string) bool {
	switch kind {
	case "screenshot", "video", "log", "check":
		return true
	}
	return false
}

func Level(item plan.Item) (string, error) {
	level := item.Proof
	if level == "" {
		level = "check"
	}
	if !kindOK(level) {
		return "", fmt.Errorf("item %s has unknown proof level %q", item.ID, level)
	}
	return level, nil
}

// Add keeps every addition, even when several processes add to the same item.
func (s Store) Add(in Input) (Record, error) {
	var zero Record
	if in.Kind == "" {
		in.Kind = "check"
	}
	if !kindOK(in.Kind) {
		return zero, errors.New("kind must be screenshot, video, log or check")
	}
	if strings.TrimSpace(in.By) == "" || strings.ContainsAny(in.By+in.Note, "\r\n\x00") {
		return zero, errors.New("by and note must be single-line values; by is required")
	}
	if (in.File == "") == (in.Text == "") {
		return zero, errors.New("supply one file or --text")
	}
	if in.File == "" && (in.Kind == "screenshot" || in.Kind == "video") {
		return zero, errors.New("screenshot and video proof need a file")
	}
	dir, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return zero, err
	}
	unlock, err := localdata.LockExisting(dir, ".proof.lock")
	if err != nil {
		return zero, err
	}
	defer unlock()
	text, err := (plan.Store{TaskDir: dir}).Read()
	if err != nil {
		return zero, err
	}
	items, err := plan.Items(text)
	if err != nil {
		return zero, err
	}
	id, _ := plan.ItemID(in.Item)
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			if _, err := Level(item); err != nil {
				return zero, err
			}
		}
	}
	if !found {
		return zero, fmt.Errorf("unknown plan item %q", in.Item)
	}
	data := []byte(in.Text)
	ext := ".txt"
	if in.File != "" {
		data, err = ReadEvidence(in.File)
		if err != nil {
			return zero, err
		}
		switch strings.ToLower(filepath.Ext(in.File)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".mp4", ".webm", ".txt", ".log", ".json":
			ext = strings.ToLower(filepath.Ext(in.File))
		default:
			ext = ".bin"
		}
	}
	if len(data) == 0 || len(data) > MaxBytes {
		return zero, errors.New("proof must contain 1 byte to 20 MiB")
	}
	if in.Kind == "screenshot" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return zero, errors.New("screenshot file must be png, jpg or webp")
	}
	if in.Kind == "video" && ext != ".mp4" && ext != ".webm" {
		return zero, errors.New("video file must be mp4 or webm")
	}
	folder, err := localdata.Safe(dir, "proof", id)
	if err != nil {
		return zero, err
	}
	if err = os.MkdirAll(folder, 0755); err != nil {
		return zero, err
	}
	metadata, err := localdata.Safe(dir, "proof", id, "proof.json")
	if err != nil {
		return zero, err
	}
	records, err := readRecords(metadata)
	if err != nil {
		return zero, err
	}
	unique, err := localdata.ID()
	if err != nil {
		return zero, err
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	r := Record{Kind: in.Kind, By: in.By, At: now.UTC().Format(time.RFC3339Nano), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Note: in.Note, File: filepath.ToSlash(filepath.Join("proof", id, unique+ext)), PlanHash: plan.Hash(text)}
	if err = atomicfile.Write(filepath.Join(dir, filepath.FromSlash(r.File)), data, 0644); err != nil {
		return zero, err
	}
	records = append(records, r)
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return zero, err
	}
	if err = atomicfile.Write(metadata, append(b, '\n'), 0644); err != nil {
		return zero, err
	}
	return r, nil
}

// ReadEvidence refuses symlinks, special files and overlarge inputs, and hashes
// in the caller only after reading the actual bytes from the opened file.
func ReadEvidence(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, errors.New("proof source must be a regular file of at most 20 MiB")
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("proof source must be a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if len(b) > MaxBytes {
		return nil, errors.New("proof exceeds 20 MiB")
	}
	return b, err
}

func readRecords(path string) ([]Record, error) {
	records := []Record{}
	b, err := ReadEvidence(path)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &records); err != nil {
		return nil, fmt.Errorf("invalid proof.json: %w", err)
	}
	if records == nil {
		return nil, errors.New("proof.json must be an array")
	}
	return records, nil
}

// Build reads original evidence, never trusting the generated index. A saved
// record only meets the gate for the plan hash it was collected against.
func (s Store) Build() (Index, error) {
	out := Index{Items: []Item{}}
	dir, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return out, err
	}
	text, err := (plan.Store{TaskDir: dir}).Read()
	if err != nil {
		return out, err
	}
	items, err := plan.Items(text)
	if err != nil {
		return out, err
	}
	out.PlanHash = plan.Hash(text)
	for _, item := range items {
		level, err := Level(item)
		if err != nil {
			return out, err
		}
		path, err := localdata.Safe(dir, "proof", item.ID, "proof.json")
		if err != nil {
			return out, err
		}
		records, err := readRecords(path)
		if err != nil {
			return out, err
		}
		row := Item{ID: item.ID, Title: item.Title, Level: level, Evidence: records}
		for _, r := range records {
			if !kindOK(r.Kind) || strings.TrimSpace(r.By) == "" || strings.ContainsAny(r.By+r.Note, "\r\n\x00") {
				return out, fmt.Errorf("invalid proof metadata for %s", item.ID)
			}
			if _, err := time.Parse(time.RFC3339Nano, r.At); err != nil {
				return out, fmt.Errorf("invalid proof time for %s", item.ID)
			}
			rel := filepath.FromSlash(r.File)
			if !filepath.IsLocal(rel) || filepath.Dir(rel) != filepath.Join("proof", item.ID) || filepath.Base(rel) == "proof.json" {
				return out, fmt.Errorf("invalid proof file for %s", item.ID)
			}
			path, err := localdata.Safe(dir, rel)
			if err != nil {
				return out, err
			}
			b, err := ReadEvidence(path)
			if err != nil {
				return out, err
			}
			if len(b) == 0 || fmt.Sprintf("%x", sha256.Sum256(b)) != r.SHA256 {
				return out, fmt.Errorf("proof checksum mismatch for %s", item.ID)
			}
			if r.PlanHash == out.PlanHash && (r.Kind == level || level == "check" && r.Kind == "log") {
				row.Proven = true
			}
		}
		sort.Slice(row.Evidence, func(i, j int) bool {
			a, b := row.Evidence[i], row.Evidence[j]
			if a.At != b.At {
				return a.At < b.At
			}
			return a.File < b.File
		})
		out.Items = append(out.Items, row)
	}
	return out, nil
}

func (s Store) Rebuild() (Index, error) {
	dir, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return Index{}, err
	}
	unlock, err := localdata.LockExisting(dir, ".proof.lock")
	if err != nil {
		return Index{}, err
	}
	defer unlock()
	out, err := s.Build()
	if err != nil {
		return out, err
	}
	path, err := localdata.Safe(dir, "proof", "index.json")
	if err != nil {
		return out, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return out, err
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return out, err
	}
	return out, atomicfile.Write(path, append(b, '\n'), 0644)
}

func (idx Index) Check() error {
	if len(idx.Items) == 0 {
		return errors.New("plan has no items to prove")
	}
	for _, item := range idx.Items {
		if !item.Proven {
			return fmt.Errorf("plan item %s lacks %s proof; run grid proof add %s", item.ID, item.Level, item.ID)
		}
	}
	return nil
}
