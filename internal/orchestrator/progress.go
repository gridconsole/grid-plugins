package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// fileProgress scans metadata once per pass and hashes only changed work
// products. The persisted cache also avoids rereading large artifacts when the
// next pass runs in another CLI process. Mechanical files never count.
func (s Store) fileProgress(m *memory) (string, error) {
	files := map[string]progressFile{}
	h := sha256.New()
	err := filepath.WalkDir(s.Feed.TaskDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != s.Feed.TaskDir {
				switch d.Name() {
				case "feed", "spawned", "questions", ".git", "brain-proposals":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.Feed.TaskDir, path)
		if err != nil {
			return err
		}
		cached, ok := m.Files[rel]
		if !ok || cached.Size != info.Size() || cached.Mtime != info.ModTime().UnixNano() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			cached = progressFile{Size: info.Size(), Mtime: info.ModTime().UnixNano(), Hash: hex.EncodeToString(sum[:])}
		}
		files[rel] = cached
		fmt.Fprintf(h, "%s\x00%s\n", rel, cached.Hash)
		return nil
	})
	if err != nil {
		return "", err
	}
	m.Files = files
	return hex.EncodeToString(h.Sum(nil)), nil
}
