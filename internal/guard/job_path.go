package guard

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// jobTextLimit is how much of a transcript the Job reader looks at.
const jobTextLimit = 64 << 10

// jobNames reports that the calling agent's own Job spells the target path: the
// path as the call gave it, as the absolute path it names, or as the real path it
// resolves to. The reviewer and checker fence stays silent for such a write. A Job
// that points at a brief file (an absolute path ending in .md) also speaks through
// that file's text, read once and not followed further (hopText); a folder that
// text names with a trailing slash covers the files directly in it. Every failure
// to read the Job or the file answers as the fence does without it.
func (c context) jobNames(given, target string) bool {
	text := jobText(c.in.TranscriptPath, c.in.AgentTranscriptPath, c.in.AgentID)
	if text == "" {
		return false
	}
	paths := unique([]string{given, resolve(c.in.Cwd, c.expand(given)), target})
	if namesAny(text, paths) {
		return true
	}
	hop := hopText(text)
	if hop == "" {
		return false
	}
	if namesAny(hop, paths) {
		return true
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			continue
		}
		dir := filepath.Dir(p)
		if filepath.Dir(dir) == dir {
			continue // a lone slash in prose is not the root folder
		}
		if namesPath(hop, dir+string(filepath.Separator)) || namesPath(hop, filepath.ToSlash(dir)+"/") {
			return true
		}
	}
	return false
}

// namesAny reports that text names one of paths, as written or with slashes.
func namesAny(text string, paths []string) bool {
	for _, p := range paths {
		if namesPath(text, p) || namesPath(text, filepath.ToSlash(p)) {
			return true
		}
	}
	return false
}

// hopText is the text of the brief files the Job names: each absolute path in job
// that ends in .md and is a regular file (a symlink is not) of fewer than
// jobTextLimit bytes, once each and in the order the Job names them. It reads those
// files only; a path inside one is not followed. A file that cannot be read adds
// nothing.
func hopText(job string) string {
	var parts []string
	seen := map[string]bool{}
	for _, p := range hopPaths(job) {
		if seen[p] {
			continue
		}
		seen[p] = true
		if s, ok := hopRead(p); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// hopPaths lists the absolute paths in text that end in .md. A path is a run of
// path characters that does not start inside a longer one, so zz/a/b.md names no
// /a/b.md; a full stop that ends a sentence is not part of the name.
func hopPaths(text string) []string {
	var out []string
	for i := 0; i < len(text); {
		if !pathChar(text[i]) {
			i++
			continue
		}
		j := i + 1
		if drive := text[i] | 0x20; drive >= 'a' && drive <= 'z' && j+1 < len(text) && text[j] == ':' && (text[j+1] == '\\' || text[j+1] == '/') {
			j++ // C:\dir\brief.md
		}
		for j < len(text) && pathChar(text[j]) {
			j++
		}
		p := strings.TrimRight(text[i:j], ".")
		if strings.HasSuffix(p, ".md") && len(p) <= 1000 && filepath.IsAbs(p) {
			out = append(out, p)
		}
		i = j
	}
	return out
}

// hopRead returns the text of the regular file at path when it is under
// jobTextLimit bytes.
func hopRead(path string) (string, bool) {
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() || info.Size() >= jobTextLimit {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, jobTextLimit))
	if err != nil || len(b) >= jobTextLimit {
		return "", false
	}
	return string(b), true
}

// jobTranscript names the file that holds the caller's Job as its first message.
// A subagent's Job is the first message of its own transcript: the hook names it,
// or it sits beside the session's as subagents/agent-<id>.jsonl. The session's
// first message is a person's prompt, so a subagent never falls back to it.
func jobTranscript(session, own, agentID string) string {
	switch {
	case own != "":
		return own
	case agentID == "":
		return session
	case session == "" || strings.ContainsAny(agentID, "/\\\x00"):
		return ""
	}
	name := "agent-" + agentID + ".jsonl"
	if filepath.Base(session) == name {
		return session
	}
	return filepath.Join(strings.TrimSuffix(session, ".jsonl"), "subagents", name)
}

// jobText returns the text of the first user message of the caller's transcript,
// read from the start of the file and no further than jobTextLimit bytes. A
// transcript that is missing, is not a regular file, does not start with a whole
// user row inside the limit, or is not JSON lines gives "".
func jobText(session, own, agentID string) string {
	path := jobTranscript(session, own, agentID)
	if path == "" || len(path) > 1000 || !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return ""
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, jobTextLimit))
	if err != nil {
		return ""
	}
	lines := bytes.Split(b, []byte("\n"))
	if len(b) >= jobTextLimit {
		lines = lines[:len(lines)-1] // a row cut at the limit is not a whole row
	}
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Type    string `json:"type"`
			IsMeta  bool   `json:"isMeta"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &row) != nil {
			return ""
		}
		role := row.Message.Role
		if role == "" {
			role = row.Type
		}
		if role != "user" || row.IsMeta {
			continue
		}
		var s string
		if json.Unmarshal(row.Message.Content, &s) == nil {
			return s
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(row.Message.Content, &blocks) != nil {
			return ""
		}
		parts := []string{}
		for _, blk := range blocks {
			if blk.Type == "text" {
				parts = append(parts, blk.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// namesPath reports that text holds p as a whole path: the characters around it
// are not ones a longer path would continue with, so /a/b.md is not named by
// /x/a/b.md, /a/b.md.bak or /a/b.mdx. A full stop or colon ends a sentence, not a
// name, unless a file name goes on after it.
func namesPath(text, p string) bool {
	for from := 0; p != "" && from < len(text); {
		i := strings.Index(text[from:], p)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(p)
		if (i == 0 || !pathChar(text[i-1])) && !continues(text, end) {
			return true
		}
		from = i + 1
	}
	return false
}

// pathChar reports a byte a path can run on through.
func pathChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.IndexByte("_-./\\~", b) >= 0
}

// continues reports that the path in text goes on at end.
func continues(text string, end int) bool {
	if end >= len(text) {
		return false
	}
	if text[end] == '.' {
		return end+1 < len(text) && text[end+1] != '.' && pathChar(text[end+1])
	}
	return pathChar(text[end])
}
