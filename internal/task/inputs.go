package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// InputsDir is the folder of a task that holds the files the person gave it.
const InputsDir = "inputs"

// checkInputs fails on the first path that is not a readable regular file, so
// New can refuse before it writes anything. The message names the path.
func checkInputs(paths []string) error {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("input: empty path")
		}
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("input %s: %w", p, unwrapPath(err))
		}
		if info.IsDir() {
			return fmt.Errorf("input %s: is a folder, not a file", p)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("input %s: is not a regular file", p)
		}
		if name := filepath.Base(p); name == "." || name == ".." || name == string(filepath.Separator) {
			return fmt.Errorf("input %s: has no file name", p)
		}
	}
	return nil
}

// unwrapPath drops the path the os error repeats; the caller names it already.
func unwrapPath(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// copyInputs copies each source into <stage>/inputs under its base name. A name
// already taken gets a number before the extension (a.png, a-2.png), never an
// overwrite. It makes no inputs folder when there is nothing to copy.
func copyInputs(stage string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	dir := filepath.Join(stage, InputsDir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	for _, p := range paths {
		if err := copyInput(dir, p); err != nil {
			return err
		}
	}
	return syncDir(dir)
}

func copyInput(dir, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("input %s: %w", src, unwrapPath(err))
	}
	defer in.Close()
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 1; ; n++ {
		name := base
		if n > 1 {
			name = stem + "-" + strconv.Itoa(n) + ext
		}
		out, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return fmt.Errorf("input %s: %w", src, err)
		}
		if err := out.Sync(); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}
