package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// Stable error codes the CLI prints and the app maps to HTTP statuses.
const (
	codeChanged         = "changed"
	codeInvalidSettings = "invalid_settings"
	codeWriteFailed     = "write_failed"
	codeUnconfirmedKey  = "unconfirmed_key"
	codeNothingToUndo   = "nothing_to_undo"
)

// CodedError is a failure with a stable code; nothing was written when it is returned.
type CodedError struct {
	Code, Msg, Path string
	Err             error
}

func (e *CodedError) Error() string { return e.Msg }
func (e *CodedError) Unwrap() error { return e.Err }

func codedErr(code, path, format string, a ...any) *CodedError {
	return &CodedError{Code: code, Path: path, Msg: fmt.Sprintf(format, a...)}
}

// The five settings.json rows the budget panel lists, in panel order.
var claudeRows = []string{"autoCompactWindow", "autoCompactEnabled", "advisorModel", "permissions.defaultMode", "remoteControlAtStartup"}

// ClaudeRowNames returns the keys "Use this" may write.
func ClaudeRowNames() []string { return slices.Clone(claudeRows) }

// The environment Grid starts agents from; effects follow the spec's section 3.7.
var claudeEnvNames = []string{"CLAUDE_CODE_EFFORT_LEVEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "CLAUDE_CODE_SUBAGENT_MODEL",
	"CLAUDE_CODE_AUTO_COMPACT_WINDOW", "DISABLE_TELEMETRY", "DO_NOT_TRACK", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
	"DISABLE_GROWTHBOOK", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX"}

var permissionModes = []string{"default", "acceptEdits", "plan", "auto", "dontAsk", "bypassPermissions"}
var advisorNames = []string{"off", "fable", "opus", "sonnet"}
var codexRows = []string{"model_auto_compact_token_limit", "agents.default_subagent_model", "agents.default_subagent_reasoning_effort"}

// AdvisorBlocked names why the advisor cannot run here: telemetry or provider.
func AdvisorBlocked(getenv func(string) string) []string {
	out := []string{}
	for _, n := range []string{"DISABLE_TELEMETRY", "DO_NOT_TRACK", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "DISABLE_GROWTHBOOK"} {
		if getenv(n) != "" {
			out = append(out, "telemetry")
			break
		}
	}
	if getenv("CLAUDE_CODE_USE_BEDROCK") != "" || getenv("CLAUDE_CODE_USE_VERTEX") != "" {
		out = append(out, "provider")
	}
	return out
}

func (o *claudeObject) delete(key string) {
	*o = slices.DeleteFunc(*o, func(f claudeField) bool { return f.key == key })
}

func splitKey(key string) (string, string) {
	top, sub, _ := strings.Cut(key, ".")
	return top, sub
}

// canon is the compact form used to compare values; absent and null are one thing.
func canon(raw json.RawMessage) string {
	var b bytes.Buffer
	if len(bytes.TrimSpace(raw)) == 0 || json.Compact(&b, raw) != nil {
		return "null"
	}
	return b.String()
}

func getClaudePath(doc claudeObject, key string) (json.RawMessage, error) {
	top, sub := splitKey(key)
	raw := doc.get(top)
	if raw == nil || sub == "" || canon(raw) == "null" {
		return raw, nil
	}
	obj, err := parseClaudeObject(raw)
	if err != nil {
		return nil, err
	}
	return obj.get(sub), nil
}

func setClaudePath(doc *claudeObject, key string, value json.RawMessage) error {
	top, sub := splitKey(key)
	remove := canon(value) == "null"
	if sub == "" {
		if remove {
			doc.delete(top)
		} else {
			doc.set(top, value)
		}
		return nil
	}
	obj := claudeObject{}
	if raw := doc.get(top); raw != nil && canon(raw) != "null" {
		var err error
		if obj, err = parseClaudeObject(raw); err != nil {
			return err
		}
	}
	if remove {
		obj.delete(sub)
		if len(obj) == 0 {
			doc.delete(top)
			return nil
		}
	} else {
		obj.set(sub, value)
	}
	doc.set(top, obj.bytes())
	return nil
}

type keyEdit struct {
	Key   string
	Value json.RawMessage // nil or null removes the key
}

// applyClaudeEdits changes only the named keys of the parsed file: every other
// key, its order and its raw value stay as read. The bytes differ only in whitespace.
func applyClaudeEdits(file claudeFile, edits []keyEdit) (data []byte, from []json.RawMessage, changed bool, err error) {
	invalid := func() error {
		return codedErr(codeInvalidSettings, file.path, "Settings file %s is not valid JSON; fix it or remove it. Nothing written.", file.path)
	}
	doc := claudeObject{}
	if file.exists {
		if doc, err = parseClaudeObject(file.data); err != nil {
			return nil, nil, false, invalid()
		}
	}
	for _, e := range edits {
		cur, err := getClaudePath(doc, e.Key)
		if err != nil {
			return nil, nil, false, invalid()
		}
		from = append(from, cur)
		if canon(cur) == canon(e.Value) {
			continue
		}
		changed = true
		if err := setClaudePath(&doc, e.Key, e.Value); err != nil {
			return nil, nil, false, invalid()
		}
	}
	data, err = encodeClaudeJSON(json.RawMessage(doc.bytes()))
	return data, from, changed, err
}

func validateClaudeValue(key string, raw json.RawMessage) (json.RawMessage, error) {
	bad := func(want string) error { return codedErr(codeInvalidSettings, "", "%s must be %s.", key, want) }
	if canon(raw) == "null" {
		return nil, nil
	}
	value := json.RawMessage(canon(raw))
	d := json.NewDecoder(bytes.NewReader(value))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, bad("valid JSON")
	}
	switch key {
	case "autoCompactWindow":
		n, ok := v.(json.Number)
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if !ok || err != nil || i < 100000 || i > 1000000 {
			return nil, bad("a whole number from 100000 to 1000000")
		}
	case "autoCompactEnabled", "remoteControlAtStartup":
		if _, ok := v.(bool); !ok {
			return nil, bad("true or false")
		}
	case "advisorModel":
		if s, ok := v.(string); !ok || !slices.Contains(advisorNames, s) {
			return nil, bad("one of " + strings.Join(advisorNames, ", "))
		}
	case "permissions.defaultMode":
		if s, ok := v.(string); !ok || !slices.Contains(permissionModes, s) {
			return nil, bad("one of " + strings.Join(permissionModes, ", "))
		}
	}
	return value, nil
}

// ClaudeSetOptions writes one key of the person's settings.json.
type ClaudeSetOptions struct {
	Home, ConfigDir string
	Key             string
	Value           json.RawMessage // nil or null removes the key
	Expect          json.RawMessage // the raw value the person saw; used when HasExpect
	HasExpect       bool
	beforeRename    func()
}

// ClaudeSetResult is the answer of one write; Written is false when nothing differed.
type ClaudeSetResult struct {
	Path    string          `json:"path"`
	Key     string          `json:"key"`
	From    json.RawMessage `json:"from"`
	To      json.RawMessage `json:"to"`
	Written bool            `json:"written"`
}

// ClaudeSet replaces, adds or removes one accepted key. Unknown keys and key
// order are kept; an invalid, duplicate-key or changed file is never written.
func ClaudeSet(o ClaudeSetOptions) (ClaudeSetResult, error) {
	if strings.HasPrefix(o.Key, "codex.") {
		return ClaudeSetResult{}, codedErr(codeUnconfirmedKey, "", "Codex key %s is read-only until its name is checked on a real install. Nothing written.", o.Key)
	}
	if !slices.Contains(claudeRows, o.Key) {
		return ClaudeSetResult{}, codedErr(codeInvalidSettings, "", "unknown key %q; allowed: %s. Nothing written.", o.Key, strings.Join(claudeRows, ", "))
	}
	value, err := validateClaudeValue(o.Key, o.Value)
	if err != nil {
		return ClaudeSetResult{}, err
	}
	path, err := ClaudeSettingsPath(o.Home, o.ConfigDir)
	if err != nil {
		return ClaudeSetResult{}, err
	}
	file, err := readClaudeFile(path)
	if err != nil {
		return ClaudeSetResult{}, err
	}
	data, from, changed, err := applyClaudeEdits(file, []keyEdit{{o.Key, value}})
	if err != nil {
		return ClaudeSetResult{}, err
	}
	res := ClaudeSetResult{Path: path, Key: o.Key, From: from[0], To: value}
	if o.HasExpect && canon(o.Expect) != canon(from[0]) {
		return res, codedErr(codeChanged, path, "Settings file changed since you looked. Nothing written.")
	}
	if !changed {
		return res, nil
	}
	stage, err := stageClaudeFile(file.target, data, file.mode)
	if err != nil {
		return res, &CodedError{Code: codeWriteFailed, Path: path, Msg: err.Error(), Err: err}
	}
	defer os.Remove(stage)
	if o.beforeRename != nil {
		o.beforeRename()
	}
	if !file.unchanged() {
		return res, codedErr(codeChanged, path, "%s", errClaudeChanged.Error())
	}
	if err := os.Rename(stage, file.target); err != nil {
		return res, &CodedError{Code: codeWriteFailed, Path: path, Msg: err.Error(), Err: err}
	}
	if err := atomicfile.SyncDir(filepath.Dir(file.target)); err != nil {
		return res, &CodedError{Code: codeWriteFailed, Path: path, Msg: err.Error(), Err: err}
	}
	res.Written = true
	return res, nil
}

// ClaudeShowOptions locates every file the read looks at; tests set all of them.
type ClaudeShowOptions struct {
	Home, ConfigDir string
	ProjectDir      string // folder holding .claude/settings.json and settings.local.json
	ManagedPath     string // empty selects the platform's managed file
	CodexHome       string // empty selects CODEX_HOME, then <Home>/.codex
	Getenv          func(string) string
	Now             func() time.Time
}

type ClaudeRow struct {
	Now        json.RawMessage `json:"now"`
	InForce    json.RawMessage `json:"inForce"`
	SetBy      *string         `json:"setBy"`
	ShadowedBy *string         `json:"shadowedBy,omitempty"`
	Blocked    []string        `json:"blocked,omitempty"`
}

type ClaudeMainModel struct {
	Value  *string `json:"value"`
	Window int     `json:"window"`
	From   string  `json:"from"`
}

type CodexRows struct {
	Path      string         `json:"path"`
	Exists    bool           `json:"exists"`
	Confirmed bool           `json:"confirmed"`
	Rows      map[string]any `json:"rows"`
}

type ClaudeShowResult struct {
	Path      string               `json:"path"`
	State     string               `json:"state"`
	Error     *string              `json:"error"`
	ReadAt    string               `json:"readAt"`
	MainModel ClaudeMainModel      `json:"mainModel"`
	Rows      map[string]ClaudeRow `json:"rows"`
	Env       map[string]*string   `json:"env"`
	Codex     CodexRows            `json:"codex"`
}

func defaultManagedPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode/managed-settings.json"
	case "windows":
		return `C:\Program Files\ClaudeCode\managed-settings.json`
	}
	return "/etc/claude-code/managed-settings.json"
}

// ClaudeShow reads the user file, the project, local and managed layers above
// it, the environment Grid starts agents from, and Codex's file (read only).
func ClaudeShow(o ClaudeShowOptions) (ClaudeShowResult, error) {
	getenv, now := o.Getenv, o.Now
	if getenv == nil {
		getenv = os.Getenv
	}
	if now == nil {
		now = time.Now
	}
	path, err := ClaudeSettingsPath(o.Home, o.ConfigDir)
	if err != nil {
		return ClaudeShowResult{}, err
	}
	res := ClaudeShowResult{Path: path, State: "ok", ReadAt: now().UTC().Format(time.RFC3339), Rows: map[string]ClaudeRow{}, Env: map[string]*string{}}
	note := func(state string, err error) {
		res.State = state
		msg := err.Error()
		res.Error = &msg
	}
	var user claudeObject
	file, err := readClaudeFile(path)
	switch {
	case err != nil:
		note("unreadable", err)
	case !file.exists:
		res.State = "missing"
	default:
		if user, err = parseClaudeObject(file.data); err != nil {
			note("invalid", claudeInvalid{path})
			user = nil
		}
	}
	type layer struct {
		name string
		doc  claudeObject
	}
	layers := []layer{}
	load := func(name, p string) {
		if p == "" {
			return
		}
		if f, err := readClaudeFile(p); err == nil && f.exists {
			if doc, err := parseClaudeObject(f.data); err == nil {
				layers = append(layers, layer{name, doc})
			}
		}
	}
	managed := o.ManagedPath
	if managed == "" {
		managed = defaultManagedPath()
	}
	load("managed", managed)
	if o.ProjectDir != "" {
		load("local", filepath.Join(o.ProjectDir, ".claude", "settings.local.json"))
		load("project", filepath.Join(o.ProjectDir, ".claude", "settings.json"))
	}
	layers = append(layers, layer{"user", user})
	for _, n := range claudeEnvNames {
		if v := getenv(n); v != "" {
			res.Env[n] = &v
		} else {
			res.Env[n] = nil
		}
	}
	blocked := AdvisorBlocked(getenv)
	defaults := map[string]json.RawMessage{"autoCompactEnabled": json.RawMessage("true"), "advisorModel": json.RawMessage(`"off"`), "permissions.defaultMode": json.RawMessage(`"default"`)}
	for _, key := range claudeRows {
		row := ClaudeRow{InForce: defaults[key]}
		for _, l := range layers {
			v, err := getClaudePath(l.doc, key)
			if err != nil || canon(v) == "null" {
				continue
			}
			if l.name == "user" {
				row.Now = v
			}
			if row.SetBy == nil {
				by := l.name
				row.SetBy, row.InForce = &by, v
			}
		}
		if key == "advisorModel" && len(blocked) > 0 {
			row.Blocked, row.InForce = blocked, json.RawMessage(`"off"`)
		}
		if key == "autoCompactWindow" {
			if mm := user.get("modelSettings"); mm != nil {
				if models, err := parseClaudeObject(mm); err == nil {
					for _, m := range models {
						if per, err := parseClaudeObject(m.value); err == nil && per.get("autoCompactWindow") != nil {
							by := "modelSettings." + m.key + ".autoCompactWindow"
							row.ShadowedBy = &by
							break
						}
					}
				}
			}
		}
		res.Rows[key] = row
	}
	res.MainModel = ClaudeMainModel{Window: 200000, From: "default"}
	for _, l := range layers {
		var s string
		if raw := l.doc.get("model"); raw != nil && json.Unmarshal(raw, &s) == nil && s != "" {
			res.MainModel.Value, res.MainModel.From = &s, "settings"
			break
		}
	}
	if v := getenv("ANTHROPIC_MODEL"); res.MainModel.Value == nil && v != "" {
		res.MainModel.Value, res.MainModel.From = &v, "env"
	}
	if res.MainModel.Value != nil && strings.Contains(*res.MainModel.Value, "[1m]") {
		res.MainModel.Window = 1000000
	}
	codexHome := o.CodexHome
	if codexHome == "" {
		codexHome = getenv("CODEX_HOME")
	}
	if codexHome == "" && o.Home != "" {
		codexHome = filepath.Join(o.Home, ".codex")
	}
	res.Codex = readCodex(filepath.Join(codexHome, "config.toml"))
	return res, nil
}

// readCodex reads three keys of Codex's config.toml and writes nothing.
func readCodex(path string) CodexRows {
	out := CodexRows{Path: path, Rows: map[string]any{}}
	for _, k := range codexRows {
		out.Rows[k] = nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	out.Exists = true
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			section = strings.Trim(line, "[] ")
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name := strings.Trim(strings.TrimSpace(k), `"`)
		if section != "" {
			name = section + "." + name
		}
		if _, want := out.Rows[name]; !want {
			continue
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, `"`) {
			if end := strings.Index(v[1:], `"`); end >= 0 {
				out.Rows[name] = v[1 : end+1]
			}
			continue
		}
		v, _, _ = strings.Cut(v, "#")
		v = strings.TrimSpace(v)
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			out.Rows[name] = n
		} else {
			out.Rows[name] = v
		}
	}
	return out
}
