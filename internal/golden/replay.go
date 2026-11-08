package golden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Command preserves the legacy invocation identity; Argv, when present, is the
// Go argv excluding the executable. Old recordings encode argv as Hook + Call.
type Command struct {
	Hook string   `json:"hook"`
	Call string   `json:"call"`
	Argv []string `json:"argv"`
	// Event is the hook_event_name of the case's input.json, set by Replay so a
	// driver can tell which legacy event a script was recorded for.
	Event string `json:"-"`
}

func (c Command) String() string {
	if len(c.Argv) > 0 {
		return "grid " + strings.Join(c.Argv, " ")
	}
	if c.Call != "" {
		return c.Hook + " (" + c.Call + ")"
	}
	return c.Hook
}

type setup struct {
	Command
	Dirs  []string           `json:"dirs"`
	Files map[string]*string `json:"files"`
	Links map[string]string  `json:"links"`
	Pre   []struct {
		Command
		Input json.RawMessage `json:"input"`
	} `json:"pre"`
	PreStdout bool               `json:"pre_stdout"`
	AfterPre  map[string]*string `json:"after_pre"`
}

// Invocation is isolated from the real process environment. The CLI adapter
// should supply Getenv from Env, never mutate os.Environ or change process cwd.
type Invocation struct {
	Argv                   []string
	Env                    map[string]string
	Stdin, Cwd, Home, Root string
}

type Output struct {
	Stdout, Stderr string
	Exit           int
}

// Driver separates the test harness from cli to avoid import cycles in packages
// that already use golden.Assert. Resolve must reject unsupported subcommands;
// silent success from an unknown hook event is not evidence of implementation.
type Driver struct {
	Resolve func(Command) ([]string, bool)
	Run     func(Invocation) Output
}

type Result struct {
	Command     string
	Skip        string
	Artifacts   map[string]string
	Differences []string
}

// Cases discovers recorded cases, in stable order, without Python or Node.
func Cases(root string) ([]string, error) {
	var cases []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "setup.json" {
			cases = append(cases, filepath.Dir(path))
		}
		return nil
	})
	sort.Strings(cases)
	return cases, err
}

// Replay builds one case in an empty t.TempDir supplied by its caller, executes
// preludes, captures a raw baseline, then compares all four recorded artifacts.
// It never updates expectations. go-diff.md must cite doc:line and provide a
// complete go-expected directory; missing artifacts are errors, not skips.
func Replay(fixtures, caseDir, box string, driver Driver) (Result, error) {
	var result Result
	raw, err := os.ReadFile(filepath.Join(caseDir, "setup.json"))
	if err != nil {
		return result, err
	}
	var s setup
	if err = json.Unmarshal(raw, &s); err != nil {
		return result, err
	}
	result.Command = s.Command.String()
	box, err = filepath.EvalSymlinks(box)
	if err != nil {
		return result, err
	}
	home := filepath.Join(box, "demo")
	workspace := filepath.Join(box, "workspace", "demo")
	user := filepath.Join(box, "user")
	// Reference/recorder tokens expand to inert directories inside the case. No
	// files from either runtime are needed or executed by the Go harness.
	tokens := map[string]string{home: "$GRID_HOME", filepath.Join(box, "old-plugin"): "$OLD_PLUGIN", filepath.Join(box, "recorder"): "$RECORDER"}
	n := newNormalizer(box, tokens)
	config := ".config"
	if runtime.GOOS == "darwin" {
		config = "Library/Application Support"
	}
	expand := func(value string) string {
		value = strings.ReplaceAll(value, "$APP_CONFIG", config)
		for actual, token := range tokens {
			value = strings.ReplaceAll(value, token, actual)
		}
		return value
	}
	for _, path := range []string{home, workspace, user} {
		if err = os.MkdirAll(path, 0o755); err != nil {
			return result, err
		}
	}
	err = filepath.WalkDir(fixtures, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtures, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(home, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture symlinks must be declared in setup.json: %s", rel)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err = os.WriteFile(dest, []byte(expand(string(content))), info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chmod(dest, info.Mode().Perm())
	})
	if err != nil {
		return result, err
	}
	safe := func(value string) (string, error) { return safePath(box, expand(value)) }
	patch := func(files map[string]*string) error {
		for _, path := range sortedKeys(files) {
			at, err := safe(path)
			if err != nil {
				return err
			}
			if files[path] == nil {
				err = os.Remove(at)
				if os.IsNotExist(err) {
					err = nil
				}
				if err != nil {
					return err
				}
				continue
			}
			if err = os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
				return err
			}
			_, statErr := os.Stat(at)
			if err = os.WriteFile(at, []byte(expand(*files[path])), 0o644); err != nil {
				return err
			}
			if os.IsNotExist(statErr) {
				if err = os.Chmod(at, 0o644); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, path := range s.Dirs {
		at, e := safe(path)
		if e != nil {
			return result, e
		}
		if e = os.MkdirAll(at, 0o755); e != nil {
			return result, e
		}
	}
	if err = patch(s.Files); err != nil {
		return result, err
	}
	for _, path := range sortedKeys(s.Links) {
		at, e := safe(path)
		if e != nil {
			return result, e
		}
		target, e := safe(s.Links[path])
		if e != nil {
			return result, e
		}
		if e = os.MkdirAll(filepath.Dir(at), 0o755); e != nil {
			return result, e
		}
		if e = os.Symlink(target, at); e != nil {
			return result, e
		}
	}
	envBytes, err := os.ReadFile(filepath.Join(caseDir, "env.txt"))
	if err != nil {
		return result, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(envBytes), "\n"), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return result, fmt.Errorf("invalid env line %q", line)
		}
		env[key] = expand(value)
	}
	input, err := os.ReadFile(filepath.Join(caseDir, "input.json"))
	if err != nil {
		return result, err
	}
	expected := caseDir
	note, err := os.ReadFile(filepath.Join(caseDir, "go-diff.md"))
	if err == nil {
		if !docLine.Match(note) {
			return result, fmt.Errorf("go-diff.md needs a doc:line reference")
		}
		expected = filepath.Join(caseDir, "go-expected")
	} else if !os.IsNotExist(err) {
		return result, err
	}
	wants := map[string]string{}
	for _, name := range artifactNames {
		b, e := os.ReadFile(filepath.Join(expected, name))
		if e != nil {
			return result, e
		}
		wants[name] = string(b)
	}
	var event struct {
		Name string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(input, &event)
	s.Command.Event = event.Name
	argv, ok := driver.Resolve(s.Command)
	if !ok {
		result.Skip = "command not implemented: " + s.Command.String()
		return result, nil
	}
	result.Command = "grid " + strings.Join(argv, " ")
	// Check all prelude capabilities before executing any, so skipped cases never
	// report partial runs as successful comparisons.
	preArgs := make([][]string, len(s.Pre))
	for i, step := range s.Pre {
		preArgs[i], ok = driver.Resolve(step.Command)
		if !ok {
			result.Skip = "setup command not implemented: " + step.Command.String()
			return result, nil
		}
	}
	invoke := func(args []string, input string) Output {
		expanded := make([]string, len(args))
		for i, a := range args {
			expanded[i] = expand(a)
		}
		return driver.Run(Invocation{expanded, env, expand(input), workspace, user, box})
	}
	for i, step := range s.Pre {
		value := string(step.Input)
		var literal string
		if json.Unmarshal(step.Input, &literal) == nil {
			value = literal
		} else {
			var formatted bytes.Buffer
			if err := json.Indent(&formatted, step.Input, "", "  "); err != nil {
				return result, err
			}
			value = formatted.String() + "\n"
		}
		out := invoke(preArgs[i], value)
		if out.Exit != 0 || out.Stderr != "" || (!s.PreStdout && out.Stdout != "") {
			return result, fmt.Errorf("prelude %s failed: exit %d stdout %q stderr %q", step.Command.String(), out.Exit, n.text(out.Stdout), n.text(out.Stderr))
		}
	}
	if err = patch(s.AfterPre); err != nil {
		return result, err
	}
	before, err := takeSnapshot(box)
	if err != nil {
		return result, err
	}
	out := invoke(argv, string(input))
	after, err := takeSnapshot(box)
	if err != nil {
		return result, err
	}
	// Match the recorder's ID allocation order: files, sorted env, then streams.
	diff := folderDiff(before, after, n)
	for _, key := range sortedKeys(env) {
		n.text(env[key])
	}
	result.Artifacts = map[string]string{"files.diff": diff, "stdout": n.text(out.Stdout), "stderr": n.text(out.Stderr), "exit": strconv.Itoa(out.Exit) + "\n"}
	for _, name := range artifactNames {
		if result.Artifacts[name] != wants[name] {
			result.Differences = append(result.Differences, name+" differs:\n"+unifiedDiff(wants[name], result.Artifacts[name], "expected/"+name, "actual/"+name))
		}
	}
	return result, nil
}

var artifactNames = []string{"stdout", "stderr", "exit", "files.diff"}
var docLine = regexp.MustCompile(`[A-Za-z0-9_./-]+\.(?:md|mjs|go|py):[1-9][0-9]*\b`)

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Resolve existing ancestors, including symlinks, before allowing a fixture
// write. Resolve the temporary root itself too (macOS commonly symlinks /var).
func safePath(box, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("fixture path must be absolute: %s", path)
	}
	path = filepath.Clean(path)
	within := func(p string) bool {
		rel, err := filepath.Rel(box, p)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	}
	if !within(path) || path == box {
		return "", fmt.Errorf("fixture path escapes case: %s", path)
	}
	at := path
	for {
		_, err := os.Lstat(at)
		if err == nil {
			real, err := filepath.EvalSymlinks(at)
			if err != nil {
				return "", err
			}
			if !within(real) {
				return "", fmt.Errorf("fixture symlink escapes case: %s", path)
			}
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(at)
		if parent == at {
			return "", fmt.Errorf("no fixture ancestor: %s", path)
		}
		at = parent
	}
}
