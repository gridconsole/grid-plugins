package machinefolder

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A SecretHit names one path under the machine folder that holds, or is named
// like, a secret. It never carries the secret itself.
type SecretHit struct {
	Path   string // relative to the scanned folder, slash separated
	Reason string // "file name", "folder name" or "content"
}

// secretFiles are file names that hold credentials; secretDirs are the Claude and
// Codex config folders. None of them belongs in the machine folder: each login
// keeps them in its own home.
var (
	secretFiles = map[string]bool{
		".credentials.json": true, "credentials.json": true, ".claude.json": true,
		"auth.json": true, "id_rsa": true, "id_ecdsa": true, "id_ed25519": true, ".netrc": true,
	}
	secretDirs = map[string]bool{".claude": true, ".codex": true, ".ssh": true}
	secretExts = map[string]bool{".pem": true, ".key": true, ".p12": true, ".pfx": true}
)

// scanLimit bounds how much of each file is read; a video is binary and larger.
const scanLimit = 1 << 20

// ScanSecrets walks root and reports each file or folder named like a credential
// or a Claude config folder, and each text file whose content looksSecret says
// holds a secret. Symbolic links are not followed. Binary files (a NUL in the
// first 8 KiB) are matched by name only. looksSecret is the caller's secrets
// check (the note checker's, brain.LooksSecret); passing it in keeps this package
// free of the brain. The hits are sorted by path.
func ScanSecrets(root string, looksSecret func(string) bool) ([]SecretHit, error) {
	var hits []SecretHit
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // an unreadable entry is not a finding
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		name := d.Name()
		lower := strings.ToLower(name)
		switch {
		case d.IsDir():
			if secretDirs[lower] || strings.HasPrefix(lower, ".claude-") {
				hits = append(hits, SecretHit{rel, "folder name"})
				return fs.SkipDir
			}
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			return nil
		case secretFiles[lower] || secretExts[filepath.Ext(lower)] || lower == ".env" || strings.HasPrefix(lower, ".env."):
			hits = append(hits, SecretHit{rel, "file name"})
			return nil
		case !d.Type().IsRegular():
			return nil
		}
		if looksSecret != nil && fileLooksSecret(path, looksSecret) {
			hits = append(hits, SecretHit{rel, "content"})
		}
		return nil
	})
	sort.Slice(hits, func(i, j int) bool { return hits[i].Path < hits[j].Path })
	return hits, err
}

func fileLooksSecret(path string, looksSecret func(string) bool) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, scanLimit))
	if err != nil {
		return false
	}
	head := b
	if len(head) > 8192 {
		head = head[:8192]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	return looksSecret(string(b))
}
