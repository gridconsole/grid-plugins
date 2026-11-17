package settings

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// ClaudeRule identifies an entry by its settings array as well as its value.
// Edit path rules cover both Edit and Write in Claude Code; Write(path) does not.
type ClaudeRule struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// ClaudeBlock holds the Claude Code deny rules for what only grid writes:
// project tasks, personal tasks, project knowledge, home knowledge, and
// project.md. No Grid hook guard repeats them.
func ClaudeBlock(home, gridHome string) ([]ClaudeRule, error) {
	return claudeBlock(home, gridHome, "")
}

func claudeBlock(home, gridHome, projects string) ([]ClaudeRule, error) {
	rules, anchor, err := claudeAllowRules(home, gridHome)
	if err != nil {
		return nil, err
	}
	brainRules, err := claudeBrainRules(home, gridHome, os.Getenv)
	if err != nil {
		return nil, err
	}
	rules = append(rules, brainRules...)
	home, gridHome = gridhome.Real(home), gridhome.Real(gridHome)
	if projects == "" {
		projects = filepath.Join(gridHome, "projects")
	}
	projectAnchor, err := claudeProjectAnchor(home, gridhome.Real(projects))
	if err != nil {
		return nil, err
	}
	for _, path := range []string{
		projectAnchor + "/*/" + claudeFold("tasks") + "/[Tt]-[0-9]*/**",
		anchor + "/" + claudeFold("personal/tasks") + "/[Tt]-[0-9]*/**",
		anchor + "/*/" + claudeFold("tasks") + "/[Tt]-[0-9]*/**",
		projectAnchor + "/*/" + claudeFold("knowledge") + "/**",
		anchor + "/" + claudeFold("knowledge") + "/**",
		projectAnchor + "/*/" + claudeFold("project.md"),
	} {
		rules = append(rules, ClaudeRule{"permissions", "deny", "Edit(" + path + ")"})
	}
	for _, path := range claudeHostPaths(anchor, projectAnchor) {
		rules = append(rules, ClaudeRule{"permissions", "deny", "Edit(" + path + ")"})
	}
	return append(rules,
		ClaudeRule{"permissions", "deny", "Edit(~/.grid-person/**)"},
		ClaudeRule{"permissions", "deny", "Write(~/.grid-person/**)"},
		ClaudeRule{"permissions", "deny", "Edit(" + anchor + "/setup/**)"},
		ClaudeRule{"permissions", "deny", "Write(" + anchor + "/setup/**)"},
		ClaudeRule{"autoMode", "environment", "$defaults"},
		ClaudeRule{"autoMode", "environment", "The Grid home at " + anchor + " is trusted for Grid operations."}), nil
}

// claudeHostPaths lists the Grid settings, lock and workflows files and the host
// folder as Claude Code deny patterns. Edit rules cover Write, MultiEdit and
// NotebookEdit too. Claude Code enforces them; Grid runs no hook on these paths
// and refuses nothing itself. Each pattern carries:
//   - settings.json, .settings.lock and workflows.md in the Grid home, one folder
//     below it, at <home>/projects/<name>/, in <projects>/ and in
//     <projects>/<name>/, in lower and upper case;
//   - the host folder: <home>/.host/** and .host beside the projects folder (the
//     same folder in the default layout).
//
// The task gate files (approval*.json, questions/, task.md, workflow.md) and the
// task Feed sit under the task-folder deny rules in claudeBlock and are not
// repeated; that includes the gate files one level below the Grid home,
// <home>/<any>/tasks/T-n/, which the rule <home>/*/tasks/T-n/** in claudeBlock
// carries.
//
// Not carried, by decision (owner question Q4, default: not carried): the legacy
// .host two levels above the projects folder (~/.host in the default layout,
// outside the Grid home); settings.json, .settings.lock and workflows.md
// directly in, or one folder below, the parent of a projects folder that sits
// outside the Grid home; the same three at <projects>/projects/<name>/; and gate
// files under <parent>/<any>/tasks/T-n/ for a projects folder outside the Grid
// home. Grid reads nothing there (no Go file but this one names .host; the only
// reader of a projects folder's parent, internal/workflow/gates.go, reads the
// Grid home layout only), and those are the person's own folders.
func claudeHostPaths(anchor, projectAnchor string) []string {
	var paths []string
	add := func(p string) {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	// The parent of the projects folder, empty when it is only a root.
	parent := ""
	if i := strings.LastIndex(projectAnchor, "/"); i > 0 && projectAnchor[:i] != "/" && projectAnchor[:i] != "//" {
		parent = projectAnchor[:i]
	}
	// A projects folder directly in the Grid home is already <home>/*/.
	inHome := parent == anchor
	for _, name := range []string{"settings.json", ".settings.lock", "workflows.md"} {
		name = claudeFold(name)
		// <home>/<name>, <home>/<dir>/<name>, <home>/projects/<project>/<name>,
		// then <projects>/<name> and <projects>/<project>/<name>.
		add(anchor + "/" + name)
		add(anchor + "/*/" + name)
		add(anchor + "/" + claudeFold("projects") + "/*/" + name)
		if !inHome {
			add(projectAnchor + "/" + name)
		}
		add(projectAnchor + "/*/" + name)
	}
	add(anchor + "/" + claudeFold(".host") + "/**")
	// The host folder also sits beside the projects folder (the same folder in
	// the default layout).
	if parent != "" {
		add(parent + "/" + claudeFold(".host") + "/**")
	}
	return paths
}

// claudeAllowRules shares the allow entries and validated anchor with setup.
func claudeAllowRules(home, gridHome string) ([]ClaudeRule, string, error) {
	if home != "" {
		home = gridhome.Real(home)
	}
	if gridHome != "" {
		gridHome = gridhome.Real(gridHome)
	}
	rel, err := filepath.Rel(home, gridHome)
	if home == "" || gridHome == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
		return nil, "", errors.New("Grid home must be a directory under the user home for ~/ permission anchors")
	}
	anchor := "~/" + filepath.ToSlash(rel)
	if strings.ContainsAny(anchor, "*?[]()\n\r") {
		return nil, "", errors.New("Grid home contains permission pattern characters")
	}
	return []ClaudeRule{
		{"permissions", "allow", "Bash(grid *)"},
		{"permissions", "allow", "Read(" + anchor + "/**)"},
		{"permissions", "allow", "Edit(" + anchor + "/**)"},
	}, anchor, nil
}

// Project denies can live outside the user home; Claude's // anchor is
// filesystem-absolute (a single / would be relative to the settings source).
func claudeProjectAnchor(home, projects string) (string, error) {
	anchor := filepath.ToSlash(projects)
	rel, err := filepath.Rel(home, projects)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		anchor = "~"
		if rel != "." {
			anchor += "/" + filepath.ToSlash(rel)
		}
	} else {
		// Claude normalizes Windows drive paths to /c/path before matching.
		if volume := filepath.VolumeName(projects); len(volume) == 2 && volume[1] == ':' {
			anchor = "/" + strings.ToLower(volume[:1]) + anchor[len(volume):]
		}
		anchor = "//" + strings.TrimLeft(anchor, "/")
	}
	if strings.ContainsAny(anchor, "*?[]()\n\r") {
		return "", errors.New("Grid projects contains permission pattern characters")
	}
	parts := strings.Split(anchor, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "projects") {
			parts[i] = claudeFold("projects")
		}
	}
	return strings.Join(parts, "/"), nil
}

// The writer guard folds these reserved names even for not-yet-created files.
func claudeFold(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			fmt.Fprintf(&b, "[%c%c]", c, c-'a'+'A')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ClaudeSetupCommand separates the operand-free form without changing the
// project's existing flag parsing (including flags before the project).
func ClaudeSetupCommand(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--answers" || arg == "-answers" {
			i++
			continue
		}
		if arg == "--" {
			return i == len(args)-1
		}
		if !strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
}

// ClaudeSetupOptions supplies all paths and streams; tests never need a real
// home. terminal and beforeRename are per-call test seams, not global hooks.
type ClaudeSetupOptions struct {
	BeforeWrite               func() int // CLI person gate, after previews and confirmation but before any write
	Home, GridHome, ConfigDir string
	Projects                  string // absolute GRID_PROJECTS; empty selects <GridHome>/projects
	Yes, Remove, JSON         bool
	Sandbox                   bool // also switch on Claude Code's own sandbox (sandbox.enabled); a plain setup never writes or reads it
	Stdin                     io.Reader
	Stdout, Stderr            io.Writer
	terminal                  func(io.Reader) bool
	beforeRename              func()
}

type claudeResult struct {
	Path    string   `json:"path"`
	Added   []string `json:"added"`
	Present []string `json:"present"`
	Written bool     `json:"written"`
}

type claudeRecord struct {
	Path  string       `json:"path"`
	Rules []ClaudeRule `json:"rules"`
}

var errClaudeChanged = errors.New("Settings file changed while writing. Nothing written.")

type claudeInvalid struct{ path string }

func (e claudeInvalid) Error() string {
	return fmt.Sprintf("Settings file %s is not valid JSON; fix it or remove it. Nothing written.", e.path)
}

// opsDefaultsLine states the owner's ops defaults in the setup summary.
const opsDefaultsLine = "No telemetry. Crash reports and a daily copy (7 days) stay in your Grid home. Old sessions are not imported."

// RunClaudeSetup previews, confirms, and applies only entries owned by setup.
func RunClaudeSetup(o ClaudeSetupOptions) int {
	code, err := runClaudeSetup(o)
	if err != nil {
		fmt.Fprintln(o.Stderr, err)
		var invalid claudeInvalid
		if errors.As(err, &invalid) {
			return 2
		}
		return 1
	}
	return code
}

// ClaudeSettingsPath locates the setup destination without consulting XDG paths.
func ClaudeSettingsPath(home, configDir string) (string, error) {
	dir := configDir
	if dir == "" {
		if home == "" {
			return "", errors.New("Cannot locate user home. Nothing written.")
		}
		dir = filepath.Join(home, ".claude")
	}
	return filepath.Abs(filepath.Join(dir, "settings.json"))
}

func runClaudeSetup(o ClaudeSetupOptions) (int, error) {
	path, err := ClaudeSettingsPath(o.Home, o.ConfigDir)
	if err != nil {
		return 1, err
	}
	if o.GridHome == "" {
		return 1, errors.New("Cannot locate Grid home. Nothing written.")
	}
	recordPath := filepath.Join(o.GridHome, "setup", "claude-block.json")
	recordFile, err := readClaudeFile(recordPath)
	if err != nil {
		return 1, err
	}
	record := claudeRecord{Path: path}
	if recordFile.exists {
		if err := json.Unmarshal(recordFile.data, &record); err != nil || record.Path != path || len(record.Rules) == 0 {
			return 1, errors.New("Setup record is invalid or belongs to another settings file. Nothing written.")
		}
		for _, r := range record.Rules {
			if o.Remove {
				continue // Removal reports every non-block entry below.
			}
			if !(r.Section == "permissions" && (r.Key == "allow" || r.Key == "deny") || r.Section == "autoMode" && r.Key == "environment" || r == claudeSandboxRule) || r.Value == "" {
				return 1, errors.New("Setup record contains an invalid rule. Nothing written.")
			}
		}
	}
	result := claudeResult{Path: path, Added: []string{}, Present: []string{}}
	finish := func(message string) (int, error) {
		if o.JSON {
			return 0, json.NewEncoder(o.Stdout).Encode(result)
		}
		if message != "" {
			fmt.Fprintln(o.Stdout, message)
		}
		return 0, nil
	}
	if o.Remove && !recordFile.exists {
		return finish("Nothing to remove.")
	}
	file, err := readClaudeFile(path)
	if err != nil {
		return 1, err
	}
	doc, err := parseClaudeObject(file.data)
	if !file.exists {
		doc, err = parseClaudeObject([]byte("{}"))
	}
	if err != nil {
		return 2, claudeInvalid{path}
	}
	block, err := claudeBlock(o.Home, o.GridHome, o.Projects)
	if err != nil {
		return 1, err
	}
	// The sandbox entry is only written on request. Removal lists it so that an
	// entry recorded by an earlier --sandbox run can be taken out; it is read
	// then only when the record names it.
	if o.Sandbox || o.Remove {
		block = append(block, claudeSandboxRule)
	}
	rules := block
	if o.Remove {
		// The writable record is evidence of installation, never authority to
		// remove arbitrary settings. Match section, key, and value exactly,
		// using the same path normalization and block builder as installation.
		rules = nil
		for _, rule := range record.Rules {
			if slices.Contains(block, rule) {
				rules = append(rules, rule)
			} else {
				fmt.Fprintf(o.Stderr, "not a Grid rule, left in place: %s\n", rule.Value)
			}
		}
	}
	changes := []ClaudeRule{}
	for _, rule := range rules {
		if rule == claudeSandboxRule {
			// A boolean, not an array entry: on means already true.
			on, err := claudeSandboxOn(doc)
			if err != nil {
				return 2, claudeInvalid{path}
			}
			label := claudeLabel(rule)
			switch {
			case o.Remove && on:
				changes = append(changes, rule)
				result.Added = append(result.Added, label)
			case o.Remove:
			case on:
				result.Present = append(result.Present, label)
			default:
				changes = append(changes, rule)
				result.Added = append(result.Added, label)
			}
			continue
		}
		values, exists, err := claudeValues(doc, rule)
		if err != nil {
			return 2, claudeInvalid{path}
		}
		// Preserve a user's intentional replacement of classifier defaults.
		if !o.Remove && rule.Section == "autoMode" && rule.Value == "$defaults" && exists && !slices.Contains(values, "$defaults") {
			continue
		}
		if o.Remove && rule == (ClaudeRule{"autoMode", "environment", "$defaults"}) {
			// Keep the defaults alongside any context the person will retain.
			personal := slices.ContainsFunc(values, func(value string) bool {
				return !slices.Contains(rules, ClaudeRule{"autoMode", "environment", value})
			})
			if personal {
				continue
			}
		}
		present := slices.Contains(values, rule.Value)
		if o.Remove {
			if present {
				changes = append(changes, rule)
				result.Added = append(result.Added, rule.Value)
			}
		} else if present {
			result.Present = append(result.Present, rule.Value)
		} else {
			changes = append(changes, rule)
			result.Added = append(result.Added, rule.Value)
		}
	}
	if !o.JSON {
		prefix := "+"
		if o.Remove {
			prefix = "-"
		}
		for _, value := range result.Added {
			fmt.Fprintf(o.Stdout, "%s %s\n", prefix, value)
		}
		for _, value := range result.Present {
			fmt.Fprintf(o.Stdout, "= %s\n", value)
		}
		block, _ := parseClaudeObject([]byte("{}"))
		for _, rule := range rules {
			if rule.Value == "$defaults" && !slices.Contains(result.Added, rule.Value) && !slices.Contains(result.Present, rule.Value) {
				continue
			}
			if err := changeClaudeRule(&block, rule, false); err != nil {
				return 1, err
			}
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, block.bytes(), "", "  "); err != nil {
			return 1, err
		}
		fmt.Fprintln(o.Stdout, pretty.String())
		fmt.Fprintln(o.Stdout, opsDefaultsLine)
	}
	if len(changes) == 0 && !o.Remove {
		return finish("Nothing written.")
	}
	if !o.Yes {
		terminal := o.terminal
		if terminal == nil {
			terminal = claudeTerminal
		}
		if o.JSON || !terminal(o.Stdin) {
			return finish("Nothing written.")
		}
		prompt := "Write these? [y/N] "
		if o.Remove {
			prompt = "Remove these? [y/N] "
		}
		fmt.Fprint(o.Stdout, prompt)
		answer, _ := bufio.NewReader(o.Stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
		default:
			return finish("Nothing written.")
		}
	}
	if o.BeforeWrite != nil {
		if code := o.BeforeWrite(); code != 0 {
			return code, nil
		}
	}
	for _, rule := range changes {
		if err := changeClaudeRule(&doc, rule, o.Remove); err != nil {
			return 1, err
		}
	}
	// Removing our $defaults entry must restore an absent environment array,
	// rather than leave [] (which disables the classifier's built-in context).
	if o.Remove && slices.Contains(changes, ClaudeRule{"autoMode", "environment", "$defaults"}) {
		values, exists, err := claudeValues(doc, ClaudeRule{"autoMode", "environment", ""})
		if err != nil {
			return 1, err
		}
		if exists && len(values) == 0 {
			obj, err := parseClaudeObject(doc.get("autoMode"))
			if err != nil {
				return 1, err
			}
			obj = slices.DeleteFunc(obj, func(f claudeField) bool { return f.key == "environment" })
			doc.set("autoMode", obj.bytes())
		}
	}
	// Prepare both files before committing either. Ownership survives no-op runs;
	// later installs retain earlier ownership and add only newly missing entries.
	nextRecord := record
	if !o.Remove {
		nextRecord.Rules = slices.Clone(record.Rules)
		for _, rule := range changes {
			if !slices.Contains(nextRecord.Rules, rule) {
				nextRecord.Rules = append(nextRecord.Rules, rule)
			}
		}
	}
	var settingsStage, recordStage string
	if len(changes) > 0 {
		data, encodeErr := encodeClaudeJSON(json.RawMessage(doc.bytes()))
		if encodeErr != nil {
			return 1, encodeErr
		}
		settingsStage, err = stageClaudeFile(file.target, data, file.mode)
		if err != nil {
			return 1, err
		}
		defer os.Remove(settingsStage)
	}
	if !o.Remove {
		data, err := encodeClaudeJSON(nextRecord)
		if err != nil {
			return 1, err
		}
		recordStage, err = stageClaudeFile(recordFile.target, data, 0600)
		if err != nil {
			return 1, err
		}
		defer os.Remove(recordStage)
	}
	if o.beforeRename != nil {
		o.beforeRename()
	}
	// No writes or user interaction between this read and the final rename.
	if !recordFile.unchanged() || !file.unchanged() {
		return 1, errClaudeChanged
	}
	if settingsStage != "" {
		if err := atomicfile.Rename(settingsStage, file.target); err != nil {
			return 1, err
		}
		if err := atomicfile.SyncDir(filepath.Dir(file.target)); err != nil {
			return 1, err
		}
	}
	if o.Remove {
		err = os.Remove(recordFile.target)
	} else {
		err = atomicfile.Rename(recordStage, recordFile.target)
	}
	if err != nil {
		return 1, err
	}
	if err := atomicfile.SyncDir(filepath.Dir(recordFile.target)); err != nil {
		return 1, err
	}
	result.Written = true
	return finish("")
}

// A character device alone is insufficient (/dev/null is one too). Let the
// host's terminal test inspect the actual inherited descriptor; fail closed.
func claudeTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	cmd := exec.Command("test", "-t", "0")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "if ([Console]::IsInputRedirected) { exit 1 }")
	}
	cmd.Stdin = f
	return cmd.Run() == nil
}

type claudeFile struct {
	path, target string
	data         []byte
	mode         os.FileMode
	exists       bool
}

// Keep setup's detailed errors while giving doctor a path-free reason.
type claudeReadError struct {
	reason string
	cause  error
}

func (e claudeReadError) Error() string { return e.cause.Error() }
func (e claudeReadError) Unwrap() error { return e.cause }

// Shared by setup (including its unchanged check) and read-only diagnostics.
func readClaudeFile(path string) (claudeFile, error) {
	f := claudeFile{path: path, target: path, mode: 0600}
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		f.target, err = filepath.EvalSymlinks(path)
		if errors.Is(err, os.ErrNotExist) {
			return f, claudeReadError{"Claude settings file is a dangling symlink", err}
		}
		if err != nil {
			return f, err
		}
		st, err = os.Stat(f.target)
		if err != nil {
			return f, err
		}
	}
	if !st.Mode().IsRegular() {
		return f, claudeReadError{"Claude settings file is not a regular file",
			fmt.Errorf("Settings file %s is not a regular file. Nothing written.", path)}
	}
	// Resolve supported links first, then reject replacement of the final target
	// by a link and verify the opened file before reading it.
	opened, err := fslock.OpenNoFollow(f.target, os.O_RDONLY, 0)
	if err != nil {
		return f, err
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		return f, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(st, info) {
		return f, claudeReadError{"Claude settings file changed while reading",
			errors.New("Settings file changed while reading. Nothing written.")}
	}
	f.exists, f.mode = true, info.Mode().Perm()
	f.data, err = io.ReadAll(opened)
	return f, err
}
func (f claudeFile) unchanged() bool {
	now, err := readClaudeFile(f.path)
	return err == nil && now.exists == f.exists && now.target == f.target && now.mode == f.mode && bytes.Equal(now.data, f.data)
}
func stageClaudeFile(target string, data []byte, mode os.FileMode) (string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".grid-claude-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err = f.Close(); err == nil {
		err = os.Chmod(name, mode)
	}
	if err == nil {
		err = atomicfile.Write(name, data, mode)
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// Ordered raw values preserve unknown keys, number precision, and object order.
// Duplicate keys are refused instead of guessing which value Claude will use.
type claudeField struct {
	key   string
	value json.RawMessage
}
type claudeObject []claudeField

func parseClaudeObject(data []byte) (claudeObject, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, errors.New("expected JSON object")
	}
	obj := claudeObject{}
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok || obj.get(key) != nil {
			return nil, errors.New("duplicate or invalid key")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		obj = append(obj, claudeField{key, raw})
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return obj, nil
}
func (o claudeObject) get(key string) json.RawMessage {
	for _, f := range o {
		if f.key == key {
			return f.value
		}
	}
	return nil
}
func (o *claudeObject) set(key string, value []byte) {
	for i := range *o {
		if (*o)[i].key == key {
			(*o)[i].value = value
			return
		}
	}
	*o = append(*o, claudeField{key, value})
}
func (o claudeObject) bytes() []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := encodeClaudeJSON(f.key)
		b.Write(bytes.TrimSpace(key))
		b.WriteByte(':')
		b.Write(f.value)
	}
	b.WriteByte('}')
	return b.Bytes()
}
func claudeValues(doc claudeObject, r ClaudeRule) ([]string, bool, error) {
	raw := doc.get(r.Section)
	if raw == nil {
		return nil, false, nil
	}
	obj, err := parseClaudeObject(raw)
	if err != nil {
		return nil, false, err
	}
	raw = obj.get(r.Key)
	if raw == nil {
		return nil, false, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, true, err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true, errors.New("expected array")
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		var value string
		if len(entry) == 0 || entry[0] != '"' {
			return nil, true, errors.New("expected string rule")
		}
		if err := json.Unmarshal(entry, &value); err != nil {
			return nil, true, err
		}
		values = append(values, value)
	}
	return values, true, nil
}

// claudeSandboxRule switches on Claude Code's own sandbox: "sandbox": {"enabled": true}.
// It is the only sandbox key setup writes (never allowUnsandboxedCommands or
// failIfUnavailable). It is a boolean, so it takes its own paths, not the array ones.
var claudeSandboxRule = ClaudeRule{"sandbox", "enabled", "true"}

// claudeLabel is how a rule is shown in previews and in the JSON result.
func claudeLabel(r ClaudeRule) string {
	if r == claudeSandboxRule {
		return "sandbox.enabled: true"
	}
	return r.Value
}

// claudeSandboxOn reports whether sandbox.enabled is true. An absent key is off;
// a sandbox that is not an object or an enabled that is not a boolean is invalid.
func claudeSandboxOn(doc claudeObject) (bool, error) {
	raw := doc.get("sandbox")
	if raw == nil {
		return false, nil
	}
	obj, err := parseClaudeObject(raw)
	if err != nil {
		return false, err
	}
	switch value := string(bytes.TrimSpace(obj.get("enabled"))); value {
	case "":
		return false, nil
	case "true", "false":
		return value == "true", nil
	}
	return false, errors.New("expected boolean")
}

func changeClaudeSandbox(doc *claudeObject, remove bool) error {
	raw := doc.get("sandbox")
	if raw == nil {
		if remove {
			return nil
		}
		raw = []byte("{}")
	}
	obj, err := parseClaudeObject(raw)
	if err != nil {
		return err
	}
	if !remove {
		obj.set("enabled", []byte("true"))
	} else if obj = slices.DeleteFunc(obj, func(f claudeField) bool { return f.key == "enabled" }); len(obj) == 0 {
		// Setup made the object; leave no empty one behind.
		*doc = slices.DeleteFunc(*doc, func(f claudeField) bool { return f.key == "sandbox" })
		return nil
	}
	doc.set("sandbox", obj.bytes())
	return nil
}

func changeClaudeRule(doc *claudeObject, r ClaudeRule, remove bool) error {
	if r == claudeSandboxRule {
		return changeClaudeSandbox(doc, remove)
	}
	raw := doc.get(r.Section)
	if raw == nil {
		raw = []byte("{}")
	}
	obj, err := parseClaudeObject(raw)
	if err != nil {
		return err
	}
	values, _, err := claudeValues(*doc, r)
	if err != nil {
		return err
	}
	if remove {
		values = slices.DeleteFunc(values, func(s string) bool { return s == r.Value })
	} else if !slices.Contains(values, r.Value) {
		values = append(values, r.Value)
	}
	if values == nil {
		values = []string{}
	}
	data, err := encodeClaudeJSON(values)
	if err != nil {
		return err
	}
	obj.set(r.Key, data)
	doc.set(r.Section, obj.bytes())
	return nil
}

// Use one encoder for generated strings, records, and the final ordered document.
// RawMessage retains number precision and key order while indentation is refreshed.
func encodeClaudeJSON(value any) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	return b.Bytes(), err
}
