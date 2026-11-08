package persongate

import (
	"errors"
	"io"
	"os"
	"strings"
)

func openDir(path string, create bool) (*os.Root, error) {
	if create {
		if err := makeDir(path); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe person directory")
	}
	if err := private(before); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	after, err := r.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		r.Close()
		return nil, errors.New("person directory changed")
	}
	if err := checkDir(r); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}
func safeFile(r *os.Root, name string, flags int) (*os.File, error) {
	before, err := r.Lstat(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && (!before.Mode().IsRegular() || private(before) != nil) {
		return nil, errors.New("unsafe person file")
	}
	f, err := openFile(r, name, flags)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || private(after) != nil || (before != nil && !os.SameFile(before, after)) {
		f.Close()
		return nil, errors.New("unsafe person file")
	}
	current, err := r.Lstat(name)
	if err != nil || !os.SameFile(current, after) || current.Mode()&os.ModeSymlink != 0 {
		f.Close()
		return nil, errors.New("person file changed")
	}
	return f, nil
}
func readConfig(dir, name string) ([]string, error) {
	r, err := openDir(dir, false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := safeFile(r, name, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return nil, err
	}
	if len(b) > 65536 {
		return nil, errors.New("person config too large")
	}
	var lines []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
func appendReceipt(dir string, data []byte) error {
	r, err := openDir(dir, true)
	if err != nil {
		return err
	}
	defer r.Close()
	lock, err := safeFile(r, "gate.lock", os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = lockFile(lock); err != nil {
		return err
	}
	defer unlockFile(lock)
	f, err := safeFile(r, "gate.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
