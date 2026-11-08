package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

type entry struct {
	Data []byte
	Mode fs.FileMode
}
type tree map[string]entry

// snapshot never follows symlinks or reads devices, sockets, or named pipes.
func snapshot(root string) (tree, error) {
	out := tree{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file (symlink or special file): %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		e := entry{Mode: info.Mode()}
		if !d.IsDir() {
			e.Data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		out[filepath.ToSlash(rel)] = e
		return nil
	})
	if os.IsNotExist(err) && len(out) == 0 {
		return out, nil
	}
	return out, err
}

func equal(a, b tree) bool    { return reflect.DeepEqual(a, b) }
func same(a, b entry) bool    { return a.Mode.IsDir() == b.Mode.IsDir() && bytes.Equal(a.Data, b.Data) }
func checksum(e entry) string { h := sha256.Sum256(e.Data); return hex.EncodeToString(h[:]) }

func writeTree(root string, t tree) error {
	for _, rel := range sortedKeys(t) {
		e := t[rel]
		path := filepath.Join(root, filepath.FromSlash(rel))
		if e.Mode.IsDir() {
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := writeSynced(path, e.Data, e.Mode.Perm()&0777); err != nil {
			return err
		}
	}
	// Directory permissions are applied last, so read-only directories can be copied.
	for _, rel := range sortedKeys(t) {
		if e := t[rel]; e.Mode.IsDir() {
			if err := os.Chmod(filepath.Join(root, filepath.FromSlash(rel)), e.Mode.Perm()&0777); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSynced(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
