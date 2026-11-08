// Package persongate checks the origin of person-only CLI writes.
package persongate

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Process struct {
	PID, PPID int
	Exe       string
	Args      []string
}
type Procs interface {
	Snapshot() (map[int]Process, error)
}
type SystemProcs struct{}
type Request struct {
	Verb, Argv []string
	Task       string
	Getenv     func(string) string
	// Environ supplies names for wildcard markers; nil uses os.Environ.
	Environ   func() []string
	Procs     Procs
	PID       int
	PersonDir func() (string, error)
	Clock     func() time.Time
}
type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
	Via    string `json:"via"`
}

// Floor is the single default verb table. Workflow policy can only add to it.
var floor = []string{"approve", "refusal decide", "decide", "answer", "question answer", "question withdraw", "task status", "task stop", "task close", "task reopen", "task delete", "task restore", "task rename", "task decline", "hook level", "agent set", "skill enable", "skill disable", "setup"}

func Floor() []string { return slices.Clone(floor) }

// PersonDir deliberately uses the account database, never HOME or XDG variables.
func PersonDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if err := accountHome(u); err != nil {
		return "", err
	}
	if u.HomeDir == "" || !filepath.IsAbs(u.HomeDir) {
		return "", errors.New("account has no absolute home")
	}
	return filepath.Join(u.HomeDir, ".grid-person"), nil
}

func name(exe string) string {
	exe = strings.ReplaceAll(exe, `\`, "/")
	return strings.TrimSuffix(strings.ToLower(filepath.Base(exe)), ".exe")
}
func agent(p Process) bool {
	n := name(p.Exe)
	if n == "claude" || n == "codex" || strings.HasPrefix(n, "codex-") || n == "grid-worker" {
		return true
	}
	if n == "node" || n == "bun" || n == "deno" {
		for _, a := range p.Args {
			a = strings.ToLower(strings.ReplaceAll(a, `\`, "/"))
			if strings.Contains(a, "claude-code") || strings.Contains(a, "@openai/codex") {
				return true
			}
		}
	}
	return false
}

var entries = strings.Split("login,sshd,tmux,screen,zellij,terminal,iterm2,ghostty,kitty,wezterm,alacritty,warp,gnome-terminal-server,konsole,xterm,windowsterminal,openconsole,explorer,code helper,cursor,jetbrains,grid-desktop,grid-server", ",")

func root(p Process) bool {
	return p.PID == 1 || slices.Contains([]string{"systemd", "cron", "crond", "atd", "atrun", "services", "taskeng", "taskhostw"}, name(p.Exe)) || (name(p.Exe) == "svchost" && slices.Contains(p.Args, "Schedule"))
}

func Check(r Request) (Decision, error) {
	refuse := func(reason string, err error) (Decision, error) { return Decision{Reason: reason}, err }
	getenv := r.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	environ := r.Environ
	if environ == nil {
		environ = os.Environ
	}
	keys := []string{"GRID_AGENT", "GRID_WORKER", "GRID_WORKER_ID", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_JOB", "CLAUDE_CODE_CHILD_SESSION"}
	for _, v := range environ() {
		k, _, _ := strings.Cut(v, "=")
		if strings.HasPrefix(k, "GRID_WORKER") || strings.HasPrefix(k, "GRID_LEASE") {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		if getenv(k) != "" {
			return refuse("agent_env", nil)
		}
	}
	role := strings.ToLower(strings.TrimSpace(getenv("GRID_ROLE")))
	if role != "" && role != "owner" && role != "member" && role != "cli" {
		return refuse("agent_env", nil)
	}
	procs := r.Procs
	if procs == nil {
		procs = SystemProcs{}
	}
	tree, err := procs.Snapshot()
	if err != nil {
		return refuse("proc_unreadable", err)
	}
	pid := r.PID
	if pid == 0 {
		pid = os.Getpid()
	}
	self, ok := tree[pid]
	if !ok {
		return refuse("proc_unreadable", errors.New("current process missing"))
	}
	chain := []Process{}
	seen := map[int]bool{pid: true}
	for pid = self.PPID; pid > 0 && !seen[pid]; {
		p, ok := tree[pid]
		if !ok {
			break
		}
		seen[pid] = true
		chain = append(chain, p)
		if agent(p) {
			return refuse("agent_ancestor", nil)
		}
		pid = p.PPID
	}
	resolve := r.PersonDir
	if resolve == nil {
		resolve = PersonDir
	}
	dir, dirErr := resolve()
	// Optional launcher names are read only from a verified account directory.
	launchers := slices.Clone(entries)
	if dirErr == nil {
		if names, e := readConfig(dir, "launchers"); e == nil {
			launchers = append(launchers, names...)
		} else if !errors.Is(e, os.ErrNotExist) {
			dirErr = e
		}
	}
	via := ""
	for _, p := range chain {
		if root(p) {
			break
		}
		if slices.Contains(launchers, name(p.Exe)) {
			via = "terminal"
			if name(p.Exe) == "grid-desktop" || name(p.Exe) == "grid-server" {
				via = "app"
			}
			break
		}
	}
	if via == "" {
		return refuse("detached", nil)
	}
	if dirErr != nil {
		return refuse("person_dir", dirErr)
	}
	now := r.Clock
	if now == nil {
		now = time.Now
	}
	argv, err := json.Marshal(r.Argv)
	if err != nil {
		return refuse("person_dir", err)
	}
	names := []string{}
	for _, p := range chain {
		names = append(names, name(p.Exe))
	}
	receipt := struct {
		Time       time.Time `json:"time"`
		Verb       string    `json:"verb"`
		Task       string    `json:"task"`
		ArgvSHA256 string    `json:"argv_sha256"`
		Via        string    `json:"via"`
		Chain      []string  `json:"chain"`
	}{now().UTC(), strings.Join(r.Verb, " "), r.Task, fmt.Sprintf("%x", sha256.Sum256(argv)), via, names}
	data, err := json.Marshal(receipt)
	if err != nil {
		return refuse("person_dir", err)
	}
	if err = appendReceipt(dir, append(data, '\n')); err != nil {
		return refuse("person_dir", err)
	}
	return Decision{Allow: true, Via: via}, nil
}

// Required checks the floor plus workflow additions. A policy line of the form
// "remove task close" is the only way to remove a floor verb. Unsafe policy is
// never used to relax the floor.
func Required(verb string, extra []string, resolve func() (string, error)) bool {
	if slices.Contains(extra, verb) {
		return true
	}
	if !slices.Contains(floor, verb) {
		return false
	}
	if resolve == nil {
		resolve = PersonDir
	}
	dir, err := resolve()
	if err != nil {
		return true
	}
	lines, err := readConfig(dir, "policy")
	if err != nil {
		return true
	}
	return !slices.Contains(lines, "remove "+verb)
}
