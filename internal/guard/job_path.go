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
// resolves to. The reviewer and checker fence stays silent for such a write. Every
// failure to read the Job answers false, which is the fence's answer without it.
func (c context) jobNames(given, target string) bool {
	text := jobText(c.in.TranscriptPath, c.in.AgentTranscriptPath, c.in.AgentID)
	if text == "" {
		return false
	}
	for _, p := range unique([]string{given, resolve(c.in.Cwd, c.expand(given)), target}) {
		if namesPath(text, p) || namesPath(text, filepath.ToSlash(p)) {
			return true
		}
	}
	return false
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
