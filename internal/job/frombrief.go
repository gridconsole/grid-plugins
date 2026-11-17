package job

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A brief file is the markdown a lane is started from: a "# <lane>: <title>" line, a
// line with "Repo: <repo>. ... from <base>. Model: <model>, <effort>. About <N> calls.",
// a "Why:" line, an optional "Skills:", "Touches:" and "Never:" line, and numbered
// acceptance rows. FromBrief maps it to a job-record as the brief-to-job-record
// convention says: Repo -> repo and base, Model -> model and effort, rows -> acceptance
// A1.., Touches -> write, Never -> out_of_scope, "About N calls" -> budget. The record
// points at the copied brief's lines instead of quoting it, except the row texts.

var (
	briefRepo   = regexp.MustCompile(`(?:^|\s)Repo:\s*([A-Za-z0-9_.-]+?)\.?(?:\s|$)`)
	briefBase   = regexp.MustCompile(`\bfrom\s+([A-Za-z0-9_./-]+?)\.?(?:\s|$)`)
	briefModel  = regexp.MustCompile(`(?:^|\s)Model:\s*([A-Za-z0-9_.-]+?)(?:,\s*([A-Za-z]+))?\.?(?:\s|$)`)
	briefCalls  = regexp.MustCompile(`\bAbout\s+([0-9]+)\s+calls\b`)
	briefRow    = regexp.MustCompile(`^\s*([0-9]+)\.\s+(\S.*)$`)
	briefPath   = regexp.MustCompile("[A-Za-z0-9_.*-]+(?:/[A-Za-z0-9_.*-]+)+")
	briefWrite  = regexp.MustCompile("\\bWrite\\s+`?((?:docs|research)/[^\\s`]+\\.md)")
	briefLaneID = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*-[A-Za-z0-9-]+$`)
	briefParens = regexp.MustCompile(`\([^)]*\)`)
)

// BriefTurns is the budget of a brief with no "About N calls" line: the builder
// runbook's default call budget.
const BriefTurns = 120

// FromBrief builds the job-record body of Job id from a brief whose text was copied
// into the task folder at input (a task-relative path such as inputs/b.md). repos are
// the project's repos: the brief's Repo is kept only when the project lists it, or
// lists none, so the record names no repo lint would refuse; the second answer says
// which repo was left out ("" when none was). A brief with no title line or no
// numbered rows is an error.
func FromBrief(id, input string, text []byte, repos []string) (map[string]any, string, error) {
	lines := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	ptr := func(from, to int) string {
		if from == to {
			return fmt.Sprintf("%s#L%d", input, from)
		}
		return fmt.Sprintf("%s#L%d-L%d", input, from, to)
	}
	label := func(name string) (int, string) {
		for i, l := range lines {
			if v, ok := strings.CutPrefix(strings.TrimSpace(l), name+":"); ok {
				return i + 1, strings.TrimSpace(v)
			}
		}
		return 0, ""
	}

	titleAt, title := 0, ""
	for i, l := range lines {
		if t, ok := strings.CutPrefix(l, "# "); ok && strings.TrimSpace(t) != "" {
			titleAt, title = i+1, strings.TrimSpace(t)
			break
		}
	}
	if titleAt == 0 {
		return nil, "", errors.New("the brief has no \"# <lane>: <title>\" line")
	}
	lane := ""
	if head, _, ok := strings.Cut(title, ":"); ok && briefLaneID.MatchString(head) {
		lane = head
	}

	from, to, rows := briefRows(lines)
	if len(rows) == 0 {
		return nil, "", errors.New("the brief has no numbered acceptance rows")
	}
	acceptance := []any{}
	for i, r := range rows {
		acceptance = append(acceptance, map[string]any{"id": "A" + strconv.Itoa(i+1), "text": r, "traces": "item"})
	}

	ownerAt, _ := label("Why")
	if ownerAt == 0 {
		ownerAt = titleAt
	}
	body := map[string]any{
		"id": id, "item": 1, "title": title, "role": "builder",
		"skills": []any{}, "risk": []any{},
		"intent": map[string]any{"owner": ptr(ownerAt, ownerAt), "item": ptr(titleAt, titleAt), "goal": ptr(from, to)},
		"links":  []any{}, "keep": []any{}, "decisions": []any{},
		"read":  []any{fmt.Sprintf("task:%s:1-%d", input, len(lines))},
		"write": []any{}, "out_of_scope": []any{},
		"acceptance": acceptance,
		"on_blocked": "ask-and-continue", "after": []any{}, "attempt": 1,
		"continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []any{"result", "question", "knowledge", "checkpoint"},
	}
	// A research lane plans and writes research; every other lane builds.
	if strings.HasPrefix(lane, "R-") {
		body["role"] = "researcher"
	}

	skipped := ""
	for _, l := range lines {
		if m := briefRepo.FindStringSubmatch(l); m != nil {
			if len(repos) == 0 || slices.Contains(repos, m[1]) {
				body["repo"] = m[1]
			} else {
				skipped = m[1]
			}
			if b := briefBase.FindStringSubmatch(l); b != nil {
				body["base"] = b[1]
			}
			break
		}
	}
	for _, l := range lines {
		if m := briefModel.FindStringSubmatch(l); m != nil {
			body["model"] = m[1]
			if m[2] != "" {
				body["effort"] = m[2]
			}
			break
		}
	}
	turns := BriefTurns
	for _, l := range lines {
		if m := briefCalls.FindStringSubmatch(l); m != nil {
			turns, _ = strconv.Atoi(m[1])
			break
		}
	}
	body["budget"] = map[string]any{"turns_estimate": max(turns, 1), "reads_before_first_edit": max(turns/4, 1)}

	if _, v := label("Skills"); v != "" {
		skills := []any{}
		for _, s := range strings.Split(briefParens.ReplaceAllString(v, ""), ",") {
			if s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ".`")); s != "" && !strings.ContainsAny(s, " \t") {
				skills = append(skills, s)
			}
		}
		body["skills"] = skills
	}
	if _, v := label("Touches"); v != "" {
		// The files the lane may change; a sentence that starts "No" or "Not" names files it may not.
		write, seen := []any{}, map[string]bool{}
		for _, sentence := range briefSentences(v) {
			if strings.HasPrefix(sentence, "No ") || strings.HasPrefix(sentence, "Not ") {
				continue
			}
			for _, p := range briefPath.FindAllString(sentence, -1) {
				if p = strings.TrimRight(p, "."); !seen[p] {
					seen[p] = true
					write = append(write, p)
				}
			}
		}
		body["write"] = write
	}
	if _, v := label("Never"); v != "" {
		// One entry per sentence. A colon reads as a repo prefix in out_of_scope, so it is written as a dash.
		never := []any{}
		for _, s := range briefSentences(v) {
			never = append(never, strings.ReplaceAll(s, ":", " -"))
		}
		body["out_of_scope"] = never
	}

	proof := ""
	if m := briefWrite.FindStringSubmatch(string(text)); m != nil {
		proof = m[1]
	} else if lane != "" {
		proof = "docs/lanes/" + lane + ".md"
	} else {
		proof = "docs/lanes/" + strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)) + ".md"
	}
	body["verify"] = map[string]any{"kind": "file-exists", "path": proof}
	return body, skipped, nil
}

// briefRows finds the acceptance rows: the numbered list under a line or heading that
// starts with "Acceptance", else under one that starts with "Questions" (a research
// brief), else the first numbered list of the brief. It answers the 1-based first and
// last line of the rows and their texts.
func briefRows(lines []string) (int, int, []string) {
	for _, head := range []string{"acceptance", "questions", ""} {
		for i, l := range lines {
			h := strings.ToLower(strings.TrimSpace(strings.TrimLeft(l, "# ")))
			if head != "" && !strings.HasPrefix(h, head) {
				continue
			}
			start := i + 1
			if head == "" {
				start = i
			}
			from, to, rows := 0, 0, []string{}
			for j := start; j < len(lines); j++ {
				m := briefRow.FindStringSubmatch(lines[j])
				if m == nil {
					if len(rows) > 0 || strings.HasPrefix(strings.TrimSpace(lines[j]), "#") {
						break
					}
					if head == "" {
						break
					}
					continue
				}
				if from == 0 {
					from = j + 1
				}
				to = j + 1
				rows = append(rows, strings.TrimSpace(m[2]))
			}
			if len(rows) > 0 {
				return from, to, rows
			}
		}
	}
	return 0, 0, nil
}

// briefSentences splits a line into its sentences, each without its closing full stop.
func briefSentences(v string) []string {
	out := []string{}
	for _, s := range regexp.MustCompile(`\.\s+`).Split(strings.TrimSpace(v), -1) {
		if s = strings.TrimSpace(strings.TrimSuffix(s, ".")); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CopyBrief copies the brief at src into the task's inputs folder under its base
// name and answers its task-relative path and its bytes. A file of that name with the
// same bytes is the copy already made and is reused; one with other bytes is kept,
// and the brief gets a number before the extension (b.md, b-2.md), never an overwrite.
func CopyBrief(taskDir, src string) (string, []byte, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", nil, fmt.Errorf("--from-brief %s: %w", src, err)
	}
	dir := filepath.Join(taskDir, "inputs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 1; ; n++ {
		name := base
		if n > 1 {
			name = stem + "-" + strconv.Itoa(n) + ext
		}
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil {
			if bytes.Equal(old, data) {
				return "inputs/" + name, data, nil
			}
			continue
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return "", nil, err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return "", nil, err
		}
		return "inputs/" + name, data, f.Close()
	}
}
