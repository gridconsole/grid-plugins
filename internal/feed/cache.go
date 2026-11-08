package feed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

// Only arrival metadata is clock-based. Offsets always stop just after a newline.
// Parsed lines are cached before fencing and hold decisions, so every pass can
// rejudge a past record after a takeover or the arrival of a missing dependency.
type readerCache struct {
	Version int                    `json:"version"`
	Task    string                 `json:"task"`
	Files   map[string]*cachedFile `json:"files"`
}
type cachedFile struct {
	Offset  int64   `json:"offset"`
	LastSeq int64   `json:"last_seq"`
	Bad     int     `json:"bad"`
	Records []Entry `json:"records"`
}

func (s Store) scan() (ReadResult, error) {
	out := ReadResult{Records: []Entry{}, Bad: map[string]int{}}
	if err := CheckFilesystem(s.Dir()); err != nil {
		return out, err
	}
	if _, err := os.Stat(s.Dir()); errors.Is(err, os.ErrNotExist) {
		return out, nil
	} else if err != nil {
		return out, err
	}
	cacheDir := filepath.Join(s.Dir(), ".cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return out, err
	}
	guard, err := os.OpenFile(filepath.Join(cacheDir, "reader.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return out, err
	}
	defer guard.Close()
	if err := lock(guard, true); err != nil {
		return out, err
	}
	defer unlock(guard)
	cache := readerCache{Version: 1, Task: s.TaskID, Files: map[string]*cachedFile{}}
	data, err := os.ReadFile(filepath.Join(cacheDir, "reader.json"))
	if err == nil {
		if json.Unmarshal(data, &cache) != nil || cache.Version != 1 || cache.Task != s.TaskID || cache.Files == nil {
			return out, fmt.Errorf("invalid reader cache; remove feed/.cache/reader.json to rebuild")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return out, err
	}
	files, err := filepath.Glob(filepath.Join(s.Dir(), "*.jsonl"))
	if err != nil {
		return out, err
	}
	arrived := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for _, file := range files {
		name := filepath.Base(file)
		cf := cache.Files[name]
		if cf == nil {
			cf = &cachedFile{Records: []Entry{}}
			cache.Files[name] = cf
		}
		f, err := os.Open(file)
		if err != nil {
			return out, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return out, err
		}
		if info.Size() < cf.Offset {
			f.Close()
			return out, fmt.Errorf("tenure file was truncated: %s", name)
		}
		if _, err := f.Seek(cf.Offset, io.SeekStart); err != nil {
			f.Close()
			return out, err
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return out, err
		}
		cut := bytes.LastIndexByte(data, '\n')
		if cut < 0 {
			continue
		}
		end := cf.Offset
		for _, line := range completeLines(data) {
			end += int64(len(line) + 1)
			var r Record
			if len(line)+1 > MaxLine || !utf8.Valid(line) || json.Unmarshal(line, &r) != nil || !wireValid(line, r, s.TaskID) || writerFile(r.Worker, r.Fence) != name || r.Seq <= cf.LastSeq {
				cf.Bad++
				continue
			}
			cf.LastSeq = r.Seq
			cf.Records = append(cf.Records, Entry{Record: r, File: name, End: end, Arrived: arrived, Verdict: "counted", Waiting: []string{}})
		}
		cf.Offset += int64(cut + 1)
	}
	for name, cf := range cache.Files {
		if _, err := os.Stat(filepath.Join(s.Dir(), name)); err != nil {
			return out, fmt.Errorf("cached tenure file is missing: %s", name)
		}
		out.Records = append(out.Records, cf.Records...)
		if cf.Bad > 0 {
			out.Bad[name] = cf.Bad
		}
	}
	if err := writeProjection(cacheDir, "reader.json", cache); err != nil {
		return out, err
	}
	return out, nil
}
