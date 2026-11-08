package doctor

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/version"
)

const bundleFileLimit int64 = 5 << 20
const bundleLogLimit = 20

// BundleOptions controls a local archive. Report comes from the existing Run
// checks. Now is optional. Out defaults to GridHome/support; existing output
// files are refused so a bundle cannot accidentally replace another document.
type BundleOptions struct {
	GridHome string
	Home     string
	Out      string
	Report   Report
	Now      func() time.Time
}

// WriteReport prints the same plain diagnostics used by doctor and doctor.txt.
func WriteReport(w io.Writer, rep Report) error {
	for _, c := range rep.Checks {
		status := "ok"
		if !c.OK {
			status = "fail"
		}
		if _, err := fmt.Fprintf(w, "%s %s: %s\n", status, c.Name, c.Detail); err != nil {
			return err
		}
		if !c.OK {
			if _, err := fmt.Fprintf(w, "fix: %s\n", c.Fix); err != nil {
				return err
			}
		}
	}
	return nil
}

// Bundle writes a redacted zip without contacting a network service. It takes
// the newest 20 regular logs and the newest crash-*.txt report, keeping at most
// the last 5 MiB of each. Symlinks and special files are not collected.
func Bundle(o BundleOptions) (string, error) {
	if o.GridHome == "" {
		return "", errors.New("cannot find the Grid home; set GRID_HOME")
	}
	now := time.Now()
	if o.Now != nil {
		now = o.Now()
	}
	out := o.Out
	if out == "" {
		out = filepath.Join(o.GridHome, "support", "grid-support-"+now.UTC().Format("20060102T150405.000000000Z")+".zip")
	}
	var err error
	out, err = filepath.Abs(out)
	if err != nil {
		return "", err
	}
	files, err := bundleFiles(filepath.Join(o.GridHome, "logs"), out)
	if err != nil {
		return "", err
	}
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	names := map[string]bool{}
	add := func(name, content string) error {
		name = Redact(name, o.Home)
		if names[name] {
			return errors.New("redacted archive names collide")
		}
		names[name] = true
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		content = Redact(content, o.Home)
		if strings.HasPrefix(name, "logs/") && int64(len(content)) > bundleFileLimit {
			content = content[int64(len(content))-bundleFileLimit:]
		}
		_, err = io.WriteString(writer, content)
		return err
	}
	var report strings.Builder
	if err = WriteReport(&report, o.Report); err != nil {
		return "", err
	}
	if err = add("doctor.txt", report.String()); err != nil {
		return "", err
	}
	versions := fmt.Sprintf("grid %s\ncontract %s\n", version.Version, version.Contract)
	if version.AppMin != "" {
		versions += fmt.Sprintf("app_min %s\n", version.AppMin)
	}
	if err = add("versions.txt", versions); err != nil {
		return "", err
	}
	for _, file := range files {
		content, err := readBundleTail(file.path)
		if err != nil {
			return "", fmt.Errorf("collect %s: %w", file.name, err)
		}
		if err = add("logs/"+filepath.ToSlash(file.name), content); err != nil {
			return "", err
		}
	}
	if err = archive.Close(); err != nil {
		return "", err
	}
	dir := filepath.Dir(out)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".grid-support.lock"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return "", err
	}
	defer fslock.Unlock(lock)
	if _, err = os.Lstat(out); err == nil {
		return "", errors.New("support bundle output already exists; choose another --out path")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err = atomicfile.Write(out, data.Bytes(), 0600); err != nil {
		return "", err
	}
	return out, nil
}

type bundleFile struct {
	path, name string
	modified   time.Time
}

func bundleFiles(root, out string) ([]bundleFile, error) {
	st, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("logs must be a directory, not a symlink")
	}
	var logs, crashes []bundleFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || path == out {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Backslashes in Unix names can become traversal separators on extraction.
		if strings.ContainsAny(filepath.ToSlash(name), "\\\r\n") {
			return nil
		}
		file := bundleFile{path: path, name: name, modified: info.ModTime()}
		if strings.HasPrefix(d.Name(), "crash-") && strings.HasSuffix(d.Name(), ".txt") {
			crashes = append(crashes, file)
		} else {
			logs = append(logs, file)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	newest := func(files []bundleFile) {
		sort.Slice(files, func(i, j int) bool {
			if files[i].modified.Equal(files[j].modified) {
				return files[i].name < files[j].name
			}
			return files[i].modified.After(files[j].modified)
		})
	}
	newest(logs)
	newest(crashes)
	logs = logs[:min(len(logs), bundleLogLimit)]
	if len(crashes) > 0 {
		logs = append(logs, crashes[0])
	}
	return logs, nil
}

func readBundleTail(path string) (string, error) {
	f, err := openBundleLog(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("log is not a regular file")
	}
	start := max(int64(0), st.Size()-bundleFileLimit)
	// Track PEM delimiters in the discarded prefix. Otherwise a key starting
	// before the retained tail could leak after its BEGIN line was truncated.
	inPEM := false
	carry := ""
	buf := make([]byte, 32*1024)
	for remaining := start; remaining > 0; {
		n, err := io.ReadFull(f, buf[:min(int64(len(buf)), remaining)])
		if err != nil {
			return "", err
		}
		chunk := carry + string(buf[:n])
		begin, end := strings.LastIndex(chunk, "-----BEGIN "), strings.LastIndex(chunk, "-----END ")
		if begin >= 0 || end >= 0 {
			inPEM = begin > end
		}
		carry = chunk[max(0, len(chunk)-16):]
		remaining -= int64(n)
	}
	b, err := io.ReadAll(io.LimitReader(f, bundleFileLimit))
	if err != nil {
		return "", err
	}
	if start > 0 {
		// Drop a partial first line rather than leaking a cut bearer/JWS/key token.
		if at := bytes.IndexByte(b, '\n'); at >= 0 {
			first := carry + string(b[:at])
			begin, end := strings.LastIndex(first, "-----BEGIN "), strings.LastIndex(first, "-----END ")
			if begin >= 0 || end >= 0 {
				inPEM = begin > end
			}
			b = b[at+1:]
		} else {
			b = nil
		}
	}
	if inPEM {
		return "-----BEGIN REDACTED-----\n" + string(b), nil
	}
	return string(b), nil
}
