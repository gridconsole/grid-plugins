package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

var shellWord = regexp.MustCompile(`^[\w@%+=:,./-]$`)
var flagWord = regexp.MustCompile(`^--?[A-Za-z][\w-]*$`)
var pathLike = regexp.MustCompile(`[/\\]|\.\.|^[.~]|^[\w@%+,-]+\.[\w-]+$`)
var httpURL = regexp.MustCompile(`^https?://[^\s]+$`)
var globEscape = regexp.MustCompile(`^~|^[A-Za-z]:|(^|[\\/])\.\.([\\/]|$)`)

func (c context) safeHome() string {
	if !filepath.IsAbs(c.in.Cwd) || !filepath.IsAbs(c.home) {
		return ""
	}
	at := realNative(c.home, 0)
	if at == filepath.Dir(at) || inside(at, realNative(c.in.Cwd, 0)) {
		return ""
	}
	if c.env["HOME"] == "" || inside(at, realNative(c.env["HOME"], 0)) {
		return ""
	}
	return at
}
func words(text string) ([]string, bool) {
	out := []string{}
	cur := ""
	active := false
	for i := 0; i < len(text); {
		ch := text[i]
		if ch == ' ' {
			if active {
				out = append(out, cur)
			}
			cur = ""
			active = false
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			end := strings.IndexByte(text[i+1:], ch)
			if end < 0 {
				return nil, false
			}
			end += i + 1
			body := text[i+1 : end]
			for _, r := range body {
				if r == '$' || r == '`' || r == '\\' || r < 32 || r == 127 {
					return nil, false
				}
			}
			cur += body
			active = true
			i = end + 1
			continue
		}
		if !shellWord.MatchString(string(ch)) {
			return nil, false
		}
		cur += string(ch)
		active = true
		i++
	}
	if active {
		out = append(out, cur)
	}
	return out, true
}
func (c context) ownScript(command string) (string, []string) {
	if !strings.HasPrefix(command, "node ") {
		return "", nil
	}
	tokens, ok := words(strings.TrimLeft(command[5:], " "))
	if !ok || len(tokens) == 0 || !filepath.IsAbs(tokens[0]) {
		return "", nil
	}
	script, err := filepath.EvalSymlinks(tokens[0])
	if err != nil {
		return "", nil
	}
	info, err := os.Stat(script)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil
	}
	// The launcher supplies its installation root; never infer it from tool input.
	plugin := c.env["CLAUDE_PLUGIN_ROOT"]
	if plugin == "" {
		return "", nil
	}
	plugin, err = filepath.EvalSymlinks(plugin)
	if err != nil {
		return "", nil
	}
	parts := strings.Split(relative(plugin, script), "/")
	if len(parts) != 4 || parts[0] != "skills" || parts[2] != "scripts" || !strings.HasSuffix(parts[3], ".mjs") || parts[1] == "" {
		return "", nil
	}
	for _, p := range parts {
		if p == ".." {
			return "", nil
		}
	}
	return parts[1] + "/" + parts[3], tokens[1:]
}
func isJSON(s string) bool {
	if !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
		return false
	}
	var v any
	if json.Unmarshal([]byte(s), &v) != nil {
		return false
	}
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}
func pathsIn(args []string) []string {
	out := []string{}
	for _, a := range args {
		if isJSON(a) || flagWord.MatchString(a) {
			continue
		}
		pieces := []string{a}
		pieces = append(pieces, strings.FieldsFunc(a, func(r rune) bool { return r == '=' || r == ',' || r == ':' })...)
		if at := strings.IndexByte(a, '/'); at >= 0 {
			pieces = append(pieces, a[at:])
		}
		for _, p := range unique(pieces) {
			if pathLike.MatchString(p) {
				out = append(out, p)
			}
		}
	}
	return out
}
func proofAddPaths(args []string) []string {
	judged := []string{}
	for i := 0; i < len(args); i += 2 {
		flag := args[i]
		judged = append(judged, flag)
		if i+1 < len(args) {
			value := args[i+1]
			if flag != "--caption" && flag != "--title" && !(flag == "--url" && httpURL.MatchString(value)) {
				judged = append(judged, value)
			}
		}
	}
	return pathsIn(judged)
}

// EvaluateHome only grants permission; an empty decision means normal permission
// handling, never denial. Combine it with other guards using hookio.Strongest.
func EvaluateHome(in hookio.Input, root string, env map[string]string) hookio.Decision {
	c := newContext(in, root, env)
	// Do not reinterpret a missing cwd as an absolute hook-supplied directory.
	c.in.Cwd = in.Cwd
	reason := ""
	if in.ToolName == "Bash" {
		script, args := c.ownScript(str(in.ToolInput["command"]))
		if script == "" {
			return hookio.Decision{}
		}
		paths := pathsIn(args)
		if script == "deliver/proof-add.mjs" {
			paths = proofAddPaths(args)
		}
		home := c.safeHome()
		for _, p := range paths {
			if home == "" || strings.HasPrefix(p, "~") || !inside(home, realNative(resolve(in.Cwd, p), 0)) {
				return hookio.Decision{}
			}
		}
		reason = "runs one of the Grid plugin's own scripts"
	} else {
		key := ""
		switch in.ToolName {
		case "Read", "Write", "Edit", "MultiEdit":
			key = "file_path"
		case "NotebookEdit":
			key = "notebook_path"
		case "Glob", "Grep":
			key = "path"
		default:
			return hookio.Decision{}
		}
		home := c.safeHome()
		given := str(in.ToolInput[key])
		if home == "" || !filepath.IsAbs(given) {
			return hookio.Decision{}
		}
		if in.ToolName == "Glob" {
			p := str(in.ToolInput["pattern"])
			if filepath.IsAbs(p) || globEscape.MatchString(p) {
				return hookio.Decision{}
			}
		}
		if !inside(home, realNative(given, 0)) {
			return hookio.Decision{}
		}
		reason = in.ToolName + " stays inside the Grid home"
	}
	if in.HookEventName == "PermissionRequest" {
		return hookio.PermissionAllow(c.safeHome())
	}
	if in.HookEventName != "PreToolUse" {
		return hookio.Decision{}
	}
	return hookio.Allow("Grid: " + reason + ".")
}
