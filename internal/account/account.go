// Package account finds the agent accounts on this machine and gives each login
// email its own config folder. Credentials stay where the provider keeps them
// (the macOS Keychain, or .credentials.json or auth.json in the provider's own
// folder): this package never copies, writes or logs a secret, and the Grid files
// it writes hold emails, folder names and use numbers only.
package account

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// The two kinds of account.
const (
	Claude = "claude"
	Codex  = "codex"
)

var kinds = []string{Claude, Codex}

// dirVars names the variable each provider reads its config folder from.
var dirVars = map[string]string{Claude: "CLAUDE_CONFIG_DIR", Codex: "CODEX_HOME"}

// configLinks is what a new folder links from the person's own folder, so one
// file serves every account. No credential or history file is ever in this list.
var configLinks = map[string][]string{
	Claude: {"settings.json", "CLAUDE.md", "agents", "commands", "skills"},
	Codex:  {"config.toml", "AGENTS.md"},
}

// Errors the app turns into its own refusal codes.
var (
	ErrNoAgent   = errors.New("no_agent")
	ErrBadFolder = errors.New("bad_folder")
)

// Env is what the package reads from the machine, so tests can set it.
type Env struct {
	Home     string // the person's home folder
	GridHome string // Grid's own folder; "" when it is not known
	Getenv   func(string) string
	Now      func() time.Time
	GOOS     string // the OS whose path rules apply; "" means the host's own
}

func (e Env) goos() string {
	if e.GOOS != "" {
		return e.GOOS
	}
	return runtime.GOOS
}

func (e Env) getenv(key string) string {
	if e.Getenv != nil {
		return e.Getenv(key)
	}
	return os.Getenv(key)
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Folder is one provider config folder found by Scan.
type Folder struct {
	Kind     string `json:"kind"`
	Dir      string `json:"dir"`
	Home     bool   `json:"home"` // ~/.claude or ~/.codex itself
	SignedIn *bool  `json:"signedIn"`
	Email    string `json:"email"`
	Plan     string `json:"plan"`
}

type folderRef struct {
	kind, dir string
	home      bool
}

func isDir(path string) bool {
	st, err := os.Stat(path) // follows a symlink
	return err == nil && st.IsDir()
}

// folderList names the folders of every kind: ~/.claude and ~/.claude-*, the same
// for Codex, and the folder the provider's own variable names when it is outside
// that list. Only directories count. A symlink is reported by its own path.
func folderList(env Env) []folderRef {
	var out []folderRef
	seen := map[string]bool{}
	add := func(kind, dir string, home bool) {
		dir = filepath.Clean(dir)
		key := env.pathKey(dir)
		if seen[key] || !isDir(dir) {
			return
		}
		seen[key] = true
		out = append(out, folderRef{kind, dir, home})
	}
	var names []string
	if entries, err := os.ReadDir(env.Home); err == nil {
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, kind := range kinds {
		base := "." + kind
		if env.Home != "" {
			add(kind, filepath.Join(env.Home, base), true)
			for _, n := range names {
				if strings.HasPrefix(n, base+"-") {
					add(kind, filepath.Join(env.Home, n), false)
				}
			}
		}
		if d := env.getenv(dirVars[kind]); d != "" {
			add(kind, d, false)
		}
	}
	return out
}

// Scan lists the folders with their login email and whether they are signed in.
// It never opens .credentials.json or the Keychain item; `claude auth status`
// may, which is Claude Code reading its own store.
func Scan(env Env) []Folder {
	refs := folderList(env)
	out := make([]Folder, 0, len(refs))
	for _, r := range refs {
		f := Folder{Kind: r.kind, Dir: r.dir, Home: r.home}
		switch r.kind {
		case Claude:
			f.Email, f.Plan, f.SignedIn = readClaude(env, r)
		case Codex:
			f.Email, f.Plan, f.SignedIn = readCodex(r)
		}
		out = append(out, f)
	}
	return out
}

func normEmail(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.Contains(s, "@") {
		return ""
	}
	return s
}

func boolPtr(b bool) *bool { return &b }

// runProvider runs the provider's own command with the folder variable set for a
// folder of its own, and unset for the home folder (dir ""). ran is false when
// the command is not installed.
func runProvider(bin string, args []string, dirVar, dir string, in io.Reader, out, errOut io.Writer) (code int, ran bool, err error) {
	path, lookErr := exec.LookPath(bin)
	if lookErr != nil {
		return 0, false, lookErr
	}
	ctx := context.Background()
	if in == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second) // a status probe has no stdin; a sign-in waits for the person
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, path, args...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, dirVar+"=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	if dir != "" {
		cmd.Env = append(cmd.Env, dirVar+"="+dir)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errOut
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), true, nil
		}
		return 0, false, err
	}
	return 0, true, nil
}

type captured struct{ b []byte }

func (c *captured) Write(p []byte) (int, error) { c.b = append(c.b, p...); return len(p), nil }

// readClaude reads the email by `claude auth status`, then by the account details
// file: ~/.claude.json for the home folder, <dir>/.claude.json for another.
func readClaude(env Env, r folderRef) (email, plan string, signedIn *bool) {
	child := r.dir
	if r.home {
		child = ""
	}
	var out captured
	code, ran, _ := runProvider("claude", []string{"auth", "status"}, dirVars[Claude], child, nil, &out, io.Discard)
	if ran {
		var st struct {
			LoggedIn         *bool  `json:"loggedIn"`
			Email            string `json:"email"`
			SubscriptionType string `json:"subscriptionType"`
		}
		_ = json.Unmarshal(out.b, &st)
		in := code == 0 && (st.LoggedIn == nil || *st.LoggedIn)
		signedIn = boolPtr(in)
		if !in {
			return "", "", signedIn // signed out: no email, so no default tag
		}
		email, plan = normEmail(st.Email), st.SubscriptionType
	}
	if email == "" {
		details := filepath.Join(r.dir, ".claude.json")
		if r.home {
			details = filepath.Join(env.Home, ".claude.json")
		}
		email = claudeDetails(details)
	}
	return email, plan, signedIn
}

func claudeDetails(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var d struct {
		OAuthAccount struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(data, &d) != nil {
		return ""
	}
	return normEmail(d.OAuthAccount.EmailAddress)
}

// readCodex takes the email from auth.json: the email claim in the payload of
// tokens.id_token. Only the email and plan claims are kept; the token is not.
// With no auth.json (the login is in the OS keyring), the email is ” and
// signedIn comes from `codex login status`.
func readCodex(r folderRef) (email, plan string, signedIn *bool) {
	if data, err := os.ReadFile(filepath.Join(r.dir, "auth.json")); err == nil {
		email, plan = codexIdentity(data)
		if email != "" {
			return email, plan, boolPtr(true)
		}
	}
	child := r.dir
	if r.home {
		child = ""
	}
	if code, ran, _ := runProvider("codex", []string{"login", "status"}, dirVars[Codex], child, nil, io.Discard, io.Discard); ran {
		signedIn = boolPtr(code == 0)
	}
	return "", "", signedIn
}

func codexIdentity(authJSON []byte) (email, plan string) {
	var a struct {
		Tokens struct {
			IDToken string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(authJSON, &a) != nil {
		return "", ""
	}
	parts := strings.Split(a.Tokens.IDToken, ".")
	if len(parts) != 3 {
		return "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return "", ""
	}
	var claims struct {
		Email string `json:"email"`
		Auth  struct {
			Plan string `json:"chatgpt_plan_type"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return "", ""
	}
	return normEmail(claims.Email), claims.Auth.Plan
}

// FolderResult is what MakeFolder answers.
type FolderResult struct {
	Dir     string `json:"dir"`
	Created bool   `json:"created"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// MakeFolder makes ~/.<id> (mode 0700) and, off Windows, links the person's config
// from ~/.<kind> into it. It is idempotent: a second run gives created:false and
// adds only links that are missing. It never copies or links a credential,
// .claude.json, auth.json, history or projects: the person signs in there once
// and the provider writes its own credential.
func MakeFolder(env Env, kind, id string) (FolderResult, error) {
	if _, ok := dirVars[kind]; !ok {
		return FolderResult{}, fmt.Errorf("kind %q is not claude or codex", kind)
	}
	if !idPattern.MatchString(id) || !strings.HasPrefix(id, kind+"-") {
		return FolderResult{}, fmt.Errorf("id %q must be %s-<name>, in lowercase letters, digits and dashes", id, kind)
	}
	if env.Home == "" {
		return FolderResult{}, errors.New("cannot find the home folder")
	}
	dir := filepath.Join(env.Home, "."+id)
	res := FolderResult{Dir: dir}
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return res, err
		}
		res.Created = true
	} else if err != nil {
		return res, err
	} else if !isDir(dir) {
		return res, fmt.Errorf("%s: %w", dir, ErrBadFolder)
	}
	if runtime.GOOS == "windows" {
		return res, nil // symlinks need developer mode; grid setup writes that folder's settings
	}
	source := filepath.Join(env.Home, "."+kind)
	for _, name := range configLinks[kind] {
		src, dst := filepath.Join(source, name), filepath.Join(dir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			return res, err
		}
	}
	return res, nil
}

// SignIn runs the provider's login with the folder's own environment and streams
// its output. The provider writes its credential itself: the Keychain on macOS,
// a file in that folder on Linux and Windows. It returns the provider's exit code.
func SignIn(env Env, kind, dir string, in io.Reader, out, errOut io.Writer) (int, error) {
	if _, ok := dirVars[kind]; !ok {
		return 1, fmt.Errorf("kind %q is not claude or codex", kind)
	}
	dir = filepath.Clean(dir)
	var ref *folderRef
	for _, r := range folderList(env) {
		if r.kind == kind && env.pathKey(r.dir) == env.pathKey(dir) {
			r := r
			ref = &r
		}
	}
	if ref == nil {
		return 1, fmt.Errorf("%s: %w", dir, ErrBadFolder)
	}
	args := []string{"login"}
	if kind == Claude {
		args = []string{"auth", "login"}
	}
	child := ref.dir
	if ref.home {
		child = ""
	}
	code, ran, err := runProvider(kind, args, dirVars[kind], child, in, out, errOut)
	if !ran {
		return 1, fmt.Errorf("%s is not installed or cannot start (%v): %w", kind, err, ErrNoAgent)
	}
	return code, nil
}
