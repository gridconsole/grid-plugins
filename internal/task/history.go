package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

// AppendHistory is the single history.md writer. A persistent kernel lock
// serializes processes; replacing the file keeps readers from seeing torn lines.
func AppendHistory(dir, line string, at time.Time) error {
	if strings.TrimSpace(line) == "" || strings.ContainsAny(line, "\r\n\x00") {
		return errors.New("history needs one nonempty line")
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	unlock, err := localdata.LockExisting(dir, ".history.lock")
	if err != nil {
		return err
	}
	defer unlock()
	path, err := localdata.Safe(dir, "history.md")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, fmt.Sprintf("- %s · %s\n", at.UTC().Format(time.RFC3339), line)...)
	return atomicfile.Write(path, data, 0o644)
}
