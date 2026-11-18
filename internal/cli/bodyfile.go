package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// bodyFileUsage is the help line of --body-file on every verb that takes --body.
const bodyFileUsage = "file holding the body, relative to the current folder, instead of --body: an agent whose shell allows no redirect, pipe or here-document writes the file and names it here"

// readBodyFile reads the file --body-file names. A path that is not absolute is
// relative to env.Cwd. --body and --body-file together are refused: one body. A
// readMax above 0 stops the read one byte past it, so the caller can tell a file
// that is too long; 0 reads the whole file.
func readBodyFile(env Env, fs *flag.FlagSet, path string, readMax int64) (string, error) {
	bodyGiven := false
	fs.Visit(func(f *flag.Flag) { bodyGiven = bodyGiven || f.Name == "body" })
	if bodyGiven {
		return "", errors.New("--body and --body-file are both given; use one")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(env.Cwd, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("--body-file: %w", err)
	}
	defer f.Close()
	var r io.Reader = f
	if readMax > 0 {
		r = io.LimitReader(f, readMax)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("--body-file: %w", err)
	}
	return string(data), nil
}
