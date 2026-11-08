package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/where"
)

var shellWord = regexp.MustCompile(`^[\w@%+=:,./-]$`)
var flagWord = regexp.MustCompile(`^--?[A-Za-z][\w-]*$`)
var pathLike = regexp.MustCompile(`[/\\]|\.\.|^[.~]|^[\w@%+,-]+\.[\w-]+$`)
var httpURL = regexp.MustCompile(`^https?://[^\s]+$`)

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
	plugin, err = filepath.EvalSymlinks(resolve(c.root, plugin))
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

type homeAccess int

const (
	noHome homeAccess = iota
	projectHome
	sharedHome
	fullHome
)

// homeScope separates home access from Personal access. Member sessions get
// per-request approvals outside Personal; only guests need an active project.
func (c context) homeScope() (string, homeAccess) {
	r := callerResolver(c.env)
	role := contract.NormalizeRole(r.Caller.Role)
	if role == "member" || role == "lead" {
		return c.safeHome(), sharedHome
	}
	if r.PersonalOK() {
		return c.safeHome(), fullHome
	}
	if role != "guest" && role != "external" {
		return "", noHome
	}
	result, err := r.Resolve(where.Query{Cwd: c.in.Cwd, Project: c.env["GRID_PROJECT"], Task: c.env["GRID_TASK"]})
	if err != nil || !result.Record || result.Project == where.Personal {
		return "", projectHome
	}
	home := realNative(c.home, 0)
	dir := filepath.Join(home, "projects", result.Project)
	// A record must not redirect a session directory through a symlink.
	if realNative(dir, 0) != dir || !inside(filepath.Join(home, "projects"), dir) {
		return "", projectHome
	}
	return dir, projectHome
}

func (c context) homeRefused(given, scope string, access homeAccess) bool {
	if access == fullHome {
		return false
	}
	if access == noHome {
		return true
	}
	lexical := resolve(c.in.Cwd, c.expand(given))
	target := c.path(given)
	home := realNative(c.home, 0)
	personal := filepath.Join(home, where.Personal)
	if inside(filepath.Join(c.home, where.Personal), lexical) ||
		inside(personal, lexical) || inside(realNative(personal, 0), target) {
		return true
	}
	// Refuse broad directory reads/searches that include Personal as well.
	if inside(target, personal) {
		return true
	}
	return access == projectHome &&
		(inside(c.home, lexical) || inside(home, target)) &&
		(scope == "" || !inside(scope, target) || !inside(scope, realNative(lexical, 0)))
}

// EvaluateHome grants only within the caller's scope and refuses home paths
// outside it. Scope refusals cannot be disabled by home-allow settings.
func EvaluateHome(in hookio.Input, root string, env map[string]string) (decision hookio.Decision) {
	defer func() {
		if decision.Behavior != "deny" {
			decision = configuredLevel("home-allow", in, root, env, decision)
		}
	}()
	c := newContext(in, root, env)
	// Do not reinterpret a missing cwd as an absolute hook-supplied directory.
	c.in.Cwd = in.Cwd
	scope, access := c.homeScope()
	reason := ""
	if in.ToolName == "Bash" {
		script, args := c.ownScript(str(in.ToolInput["command"]))
		if script == "" {
			return hookio.Decision{}
		}
		// Guests cannot establish a project scope for implicit script access.
		if access == projectHome || access == noHome {
			return hookio.Deny("This caller cannot authorize legacy scripts with implicit Personal access.")
		}
		paths := pathsIn(args)
		if script == "deliver/proof-add.mjs" {
			paths = proofAddPaths(args)
		}
		home := c.safeHome()
		for _, p := range paths {
			if c.homeRefused(p, scope, access) {
				return hookio.Deny("This caller cannot access Personal through a script argument.")
			}
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
		scopedPath := given
		if scopedPath == "" && key == "path" {
			scopedPath = in.Cwd
		}
		if c.homeRefused(scopedPath, scope, access) {
			return hookio.Deny("This caller cannot access this Grid home scope.")
		}
		if in.ToolName == "Glob" {
			pattern := str(in.ToolInput["pattern"])
			if !c.homeGlobAllowed(pattern, scope, access) {
				// Owners and machine sessions retain normal permission handling
				// outside home-rooted patterns, without an automatic home grant.
				if access == fullHome && !c.homeGlobRooted(pattern) {
					return hookio.Decision{}
				}
				return hookio.Deny("This glob pattern cannot stay inside the allowed home scope.")
			}
		}
		if home == "" || !filepath.IsAbs(given) {
			return hookio.Decision{}
		}
		if !inside(home, realNative(given, 0)) || scope == "" || !inside(scope, realNative(given, 0)) {
			return hookio.Decision{}
		}
		reason = in.ToolName + " stays inside the Grid home"
	}
	if in.HookEventName == "PermissionRequest" {
		if access == sharedHome {
			// A directory grant cannot express the Personal exclusion.
			return hookio.PermissionAllow("")
		}
		return hookio.PermissionAllow(scope)
	}
	if in.HookEventName != "PreToolUse" {
		return hookio.Decision{}
	}
	return hookio.Allow("Grid: " + reason + ".")
}

// homeGlobRooted recognizes literal home roots before cleaning can erase a
// traversal. A mixed brace pattern is home-rooted if any alternative is.
func (c context) homeGlobRooted(pattern string) bool {
	if !filepath.IsAbs(c.home) {
		return false
	}
	patterns, ok := expandHomeBraces(pattern)
	if !ok {
		patterns = []string{pattern}
	}
	for _, root := range []string{c.home, realNative(c.home, 0)} {
		if !filepath.IsAbs(root) {
			continue
		}
		root = strings.TrimRight(strings.ReplaceAll(root, `\`, "/"), "/")
		for _, p := range patterns {
			p = strings.ReplaceAll(p, `\`, "/")
			if p == root || strings.HasPrefix(p, root+"/") {
				return true
			}
		}
	}
	return false
}

// homeGlobAllowed checks every brace alternative structurally, before cleaning
// paths could erase traversal elements. Absolute patterns need a literal prefix
// inside the allowed root, checked both lexically and through existing symlinks.
func (c context) homeGlobAllowed(pattern, scope string, access homeAccess) bool {
	patterns, ok := expandHomeBraces(pattern)
	if !ok {
		return false
	}
	for _, p := range patterns {
		p = strings.ReplaceAll(p, `\`, "/")
		for _, part := range strings.Split(p, "/") {
			if part == ".." {
				return false
			}
		}
		if strings.HasPrefix(p, "~") {
			return false
		}
		// A leading separator is rooted even without a Windows volume.
		// Drive-relative and foreign drive paths must also face the prefix
		// check, never inherit the tool's allowed path as relative patterns.
		if !strings.HasPrefix(p, "/") && !(len(p) >= 2 && p[1] == ':') {
			continue
		}
		prefix := p
		if at := strings.IndexAny(prefix, "*?["); at >= 0 {
			prefix = prefix[:at]
		}
		if scope == "" || !inside(scope, prefix) || !inside(scope, realNative(prefix, 0)) || c.homeRefused(prefix, scope, access) {
			return false
		}
	}
	return true
}

// Bound expansion work and refuse malformed or excessive patterns. Expanding
// innermost braces first also catches traversal assembled across brace edges.
func expandHomeBraces(pattern string) ([]string, bool) {
	if len(pattern) > 16384 {
		return nil, false
	}
	patterns := []string{pattern}
	for i := 0; i < len(patterns); {
		p := patterns[i]
		end := strings.IndexByte(p, '}')
		if end < 0 {
			if strings.ContainsRune(p, '{') {
				return nil, false
			}
			i++
			continue
		}
		start := strings.LastIndexByte(p[:end], '{')
		if start < 0 {
			return nil, false
		}
		parts := strings.Split(p[start+1:end], ",")
		if len(parts) < 2 || len(patterns)+len(parts)-1 > 256 {
			return nil, false
		}
		patterns[i] = p[:start] + parts[0] + p[end+1:]
		for _, part := range parts[1:] {
			patterns = append(patterns, p[:start]+part+p[end+1:])
		}
	}
	return patterns, true
}
