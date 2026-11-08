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
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// ClaudeRule identifies an entry by its settings array as well as its value.
// Edit path rules cover both Edit and Write in Claude Code; Write(path) does not.
type ClaudeRule struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// ClaudeBlock follows EvaluateOnlyWriter in internal/guard/writer.go: project
// tasks, personal tasks, project knowledge, home knowledge, and project.md.
// Keep its reserved kinds in step with that guard (see the parity test).
func ClaudeBlock(home, gridHome string) ([]ClaudeRule, error) {
	rel, err := filepath.Rel(home, gridHome)
	if home == "" || gridHome == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
		return nil, errors.New("Grid home must be a directory under the user home for ~/ permission anchors")
	}
	anchor := "~/" + filepath.ToSlash(rel)
	if strings.ContainsAny(anchor, "*?[]()\n\r") {
		return nil, errors.New("Grid home contains permission pattern characters")
	}
	rules := []ClaudeRule{
		{"permissions", "allow", "Bash(grid *)"},
		{"permissions", "allow", "Read(" + anchor + "/**)"},
		{"permissions", "allow", "Edit(" + anchor + "/**)"},
	}
	for _, path := range []string{
		"/projects/*/tasks/[Tt]-[0-9]*/**",
		"/personal/tasks/[Tt]-[0-9]*/**",
		"/projects/*/knowledge/**",
		"/knowledge/**",
		"/projects/*/project.md",
	} {
		rules = append(rules, ClaudeRule{"permissions", "deny", "Edit(" + anchor + path + ")"})
	}
	return append(rules,
		ClaudeRule{"autoMode", "environment", "$defaults"},
		ClaudeRule{"autoMode", "environment", "The Grid home at " + anchor + " is trusted for Grid operations."}), nil
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
	Home, GridHome, ConfigDir string
	Yes, Remove, JSON         bool
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

func runClaudeSetup(o ClaudeSetupOptions) (int, error) {
	dir := o.ConfigDir
	if dir == "" {
		if o.Home == "" {
			return 1, errors.New("Cannot locate user home. Nothing written.")
		}
		dir = filepath.Join(o.Home, ".claude")
	}
	path, err := filepath.Abs(filepath.Join(dir, "settings.json"))
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
			if !(r.Section == "permissions" && (r.Key == "allow" || r.Key == "deny") || r.Section == "autoMode" && r.Key == "environment") || r.Value == "" {
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
	rules := record.Rules
	if !o.Remove {
		rules, err = ClaudeBlock(o.Home, o.GridHome)
		if err != nil {
			return 1, err
		}
	}
	changes := []ClaudeRule{}
	for _, rule := range rules {
		values, exists, err := claudeValues(doc, rule)
		if err != nil {
			return 2, claudeInvalid{path}
		}
		// Preserve a user's intentional replacement of classifier defaults.
		if !o.Remove && rule.Section == "autoMode" && rule.Value == "$defaults" && exists && !slices.Contains(values, "$defaults") {
			continue
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
	for _, rule := range changes {
		if err := changeClaudeRule(&doc, rule, o.Remove); err != nil {
			return 1, err
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
		settingsStage, err = stageClaudeFile(file.target, append(doc.bytes(), '\n'), file.mode)
		if err != nil {
			return 1, err
		}
		defer os.Remove(settingsStage)
	}
	if !o.Remove {
		data, err := json.MarshalIndent(nextRecord, "", "  ")
		if err != nil {
			return 1, err
		}
		recordStage, err = stageClaudeFile(recordFile.target, append(data, '\n'), 0600)
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
		if err := os.Rename(settingsStage, file.target); err != nil {
			return 1, err
		}
		if err := atomicfile.SyncDir(filepath.Dir(file.target)); err != nil {
			return 1, err
		}
	}
	if o.Remove {
		err = os.Remove(recordFile.target)
	} else {
		err = os.Rename(recordStage, recordFile.target)
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
	cmd.Stdin = f
	return cmd.Run() == nil
}

type claudeFile struct {
	path, target string
	data         []byte
	mode         os.FileMode
	exists       bool
}

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
		if err != nil {
			return f, err
		}
		st, err = os.Stat(f.target)
		if err != nil {
			return f, err
		}
	}
	if !st.Mode().IsRegular() {
		return f, fmt.Errorf("Settings file %s is not a regular file. Nothing written.", path)
	}
	f.exists, f.mode = true, st.Mode().Perm()
	f.data, err = os.ReadFile(f.target)
	return f, err
}
func (f claudeFile) unchanged() bool {
	now, err := readClaudeFile(f.path)
	return err == nil && now.exists == f.exists && now.target == f.target && bytes.Equal(now.data, f.data)
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
		key, _ := json.Marshal(f.key)
		b.Write(key)
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
	var values []string
	err = json.Unmarshal(raw, &values)
	if err == nil && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		err = errors.New("expected array")
	}
	return values, true, err
}
func changeClaudeRule(doc *claudeObject, r ClaudeRule, remove bool) error {
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
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	obj.set(r.Key, data)
	doc.set(r.Section, obj.bytes())
	return nil
}
