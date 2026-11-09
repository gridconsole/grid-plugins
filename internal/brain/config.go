package brain

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"go.yaml.in/yaml/v4"
)

// Config entries share the brain's scope, Markdown dialect and writer lock.
// The topic command owns the executable schema; the keeper cannot edit it.
func (s Store) configPath(root *os.Root, level, topic, name string) (string, error) {
	t, err := topicAt(root, topic, level)
	if err != nil {
		return "", err
	}
	if t.Kind != "config" || !ValidName(name) || name == "TOPIC" {
		return "", fmt.Errorf("config needs a config topic and a plain entry name")
	}
	return filepath.Join("knowledge", topic, name+".md"), nil
}

func (s Store) ReadConfig(level, topic, name string) ([]byte, error) {
	root, err := s.scope(level, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	path, err := s.configPath(root, level, topic, name)
	if err != nil {
		return nil, err
	}
	return safefile.Read(root, path)
}

func (s Store) ConfigNames(level, topic string) ([]string, error) {
	names := []string{}
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	path, err := s.configPath(root, level, topic, "entry")
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err := safefile.NoSymlinks(root, dir); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("config paths cannot be symbolic links")
		}
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "TOPIC.md" {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// EditConfig serializes read/modify/write with all brain writers. Missing input
// is nil; nil output deletes the entry. Callback errors never change the file.
func (s Store) EditConfig(level, topic, name string, edit func([]byte) ([]byte, error)) error {
	if err := localdata.Init(s.GridHome); err != nil {
		return err
	}
	root, err := s.scope(level, true)
	if err != nil {
		return err
	}
	defer root.Close()
	path, err := s.configPath(root, level, topic, name)
	if err != nil {
		return err
	}
	return locked(root, func() error {
		old, err := safefile.Read(root, path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		data, err := edit(old)
		if err != nil {
			return err
		}
		if data == nil {
			if err := root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			return nil
		}
		if len(data) > 1<<20 {
			return fmt.Errorf("config entry exceeds 1 MiB")
		}
		// Rewriting the same bytes is not a change: no version, no audit line.
		changed := old == nil || !bytes.Equal(old, data)
		version, hash := 0, ""
		if changed {
			if version, hash, err = recordVersion(root, topic, name, data); err != nil {
				return err
			}
		}
		if err := atomicWrite(root, path, data); err != nil {
			return err
		}
		if err := atomicfile.SyncDir(filepath.Join(root.Name(), filepath.Dir(path))); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		origin, err := SessionOrigin()
		if err != nil {
			return err
		}
		return logAudit(root, Change{At: s.stamp(), Status: "accepted", Topic: topic, Name: name,
			Version: version, Hash: hash, OriginKind: "config", OriginID: origin.ID})
	})
}

// DecodeConfig uses the same strict front matter parser as knowledge notes.
func DecodeConfig(data []byte, out any) (string, error) {
	front, body, err := mdfile.Split(data)
	if err != nil {
		return "", err
	}
	return body, mdfile.Load(front, out, true)
}

// EncodeConfig also preserves the entry's free-form notes on state updates.
func EncodeConfig(value any, body string) ([]byte, error) {
	front, err := yaml.Dump(value, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("---\n%s---\n%s\n", front, strings.TrimRight(body, "\n"))), nil
}
