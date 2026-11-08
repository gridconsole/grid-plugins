package routine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

const LabelPrefix = "com.gridconsole.routine."

type OSCommand struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
}
type OSRunner interface {
	Run(context.Context, OSCommand) error
}
type ExecOSRunner struct{}

func (ExecOSRunner) Run(ctx context.Context, c OSCommand) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, c.Program, c.Args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", c.Program, err, strings.TrimSpace(string(out)))
	}
	return nil
}

type TimerFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type TimerPlan struct {
	Label       string      `json:"label"`
	Project     string      `json:"project"`
	Name        string      `json:"name"`
	Files       []TimerFile `json:"files"`
	Install     []OSCommand `json:"install,omitempty"`
	Remove      []OSCommand `json:"remove,omitempty"`
	AfterRemove []OSCommand `json:"after_remove,omitempty"`
}

type TimerSpec struct{ Home, GridHome, Project, Binary, Name, Path string }

func (s TimerSpec) label() string { return timerLabel(s.GridHome, s.Project, s.Name) }
func (s TimerSpec) argv() []string {
	args := []string{s.Binary, "routine", "run", s.Name, "--due"}
	if s.Project != "" {
		args = append(args, "--project", s.Project)
	}
	return args
}
func timerLabel(gridHome, project, name string) string {
	return LabelPrefix + identity(gridHome, project, name)
}
func marker(label string) string { return "Managed by grid routine: " + label }

// Backend only renders a plan. The host-specific constructor is build tagged;
// all renderers can be exercised from any host without calling an OS scheduler.
type Backend interface {
	Plan(TimerSpec) (TimerPlan, error)
}
type Scheduler struct {
	Home, GridHome, Binary, Path string
	Backend                      Backend
	Runner                       OSRunner
}

type timerState struct {
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	// Hash is the fingerprint of the routine's run and every when the person
	// last ran schedule; timer runs refuse a routine that no longer matches.
	Hash string `json:"hash,omitempty"`
}

func (s Scheduler) plan(project, name string) (TimerPlan, error) {
	if !brain.ValidName(name) || (project != "" && !brain.ValidName(project)) {
		return TimerPlan{}, fmt.Errorf("invalid routine or project name")
	}
	if s.Backend == nil || s.Runner == nil {
		return TimerPlan{}, fmt.Errorf("timer backend and OS runner are required")
	}
	if !filepath.IsAbs(s.Home) || !filepath.IsAbs(s.GridHome) || !filepath.IsAbs(s.Binary) {
		return TimerPlan{}, fmt.Errorf("timer home, Grid home and executable must be absolute")
	}
	for _, v := range []string{s.Home, s.GridHome, s.Binary, s.Path} {
		if strings.ContainsAny(v, "\r\n\x00") {
			return TimerPlan{}, fmt.Errorf("timer paths must be one line")
		}
	}
	return s.Backend.Plan(TimerSpec{Home: s.Home, GridHome: s.GridHome, Project: project, Binary: s.Binary, Name: name, Path: s.Path})
}
func (s Scheduler) statePath(label string) (string, error) {
	return localdata.Safe(s.GridHome, "local", "routine-timers", label+".json")
}
func (s Scheduler) writeState(p TimerPlan, installed bool, hash string) error {
	path, err := s.statePath(p.Label)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(timerState{Label: p.Label, Installed: installed, Project: p.Project, Name: p.Name, Hash: hash})
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(b, '\n'), 0o600)
}
func (s Scheduler) readState(p TimerPlan) (timerState, error) {
	path, err := s.statePath(p.Label)
	if err != nil {
		return timerState{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return timerState{}, nil
	}
	if err != nil {
		return timerState{}, err
	}
	var state timerState
	if err = json.Unmarshal(b, &state); err != nil {
		return state, err
	}
	if state.Label != p.Label || state.Name != p.Name || state.Project != p.Project {
		return state, fmt.Errorf("timer ownership record does not match")
	}
	return state, nil
}
func (s Scheduler) checkFiles(p TimerPlan) error {
	for _, f := range p.Files {
		rel, err := filepath.Rel(s.Home, f.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("timer file is outside the home folder")
		}
		// Dotfile managers symlink ~/.config and the like, so parent folders are
		// followed; only the timer file itself must not be a symlink.
		path := f.Path
		st, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink: %s", path)
		}
		if !st.Mode().IsRegular() || st.Size() > 1<<20 {
			return fmt.Errorf("timer file is not a small regular file")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), marker(p.Label)) {
			return fmt.Errorf("refusing timer file not created by grid routine: %s", filepath.Base(path))
		}
	}
	return nil
}
func (s Scheduler) commands(ctx context.Context, commands []OSCommand) error {
	for _, c := range commands {
		if err := s.Runner.Run(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (s Scheduler) Schedule(ctx context.Context, store Store) error {
	// The timer lock orders all install/remove operations; the entry lock also
	// serializes against add, remove and execution while its definition is read.
	unlock, err := localdata.Lock(s.GridHome, ".routine-timers.lock")
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := store.List()
	if err != nil {
		return err
	}
	machine := gridhome.Machine(s.GridHome)
	for _, entry := range entries {
		if entry.Machine != machine {
			continue
		}
		if err := s.install(ctx, store, entry.Name); err != nil {
			return err
		}
	}
	return nil
}
func (s Scheduler) install(ctx context.Context, store Store, name string) error {
	unlock, err := store.lock(name)
	if err != nil {
		return err
	}
	defer unlock()
	e, err := store.Get(name)
	if err != nil {
		return err
	}
	if e.Machine != gridhome.Machine(s.GridHome) {
		return fmt.Errorf("routine belongs to another machine")
	}
	p, err := s.plan(store.Project, name)
	if err != nil {
		return err
	}
	if err = s.checkFiles(p); err != nil {
		return err
	}
	state, err := s.readState(p)
	if err != nil {
		return err
	}
	hash := e.Fingerprint()
	if state.Installed {
		unchanged := true
		for _, f := range p.Files {
			b, err := os.ReadFile(f.Path)
			if err != nil || string(b) != f.Content {
				unchanged = false
			}
		}
		if unchanged {
			// The files do not carry the command, so a changed run or every
			// only needs the approval record refreshed.
			if state.Hash == hash {
				return nil
			}
			return s.writeState(p, true, hash)
		}
		if err = s.commands(ctx, p.Remove); err != nil {
			return err
		}
	}
	// Record ownership before OS calls, leaving enough evidence for recovery if
	// registration fails. Never overwrite a foreign file even with our prefix.
	if err = s.writeState(p, false, hash); err != nil {
		return err
	}
	for _, f := range p.Files {
		if err = os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
			return err
		}
		if err = atomicfile.Write(f.Path, []byte(f.Content), 0o600); err != nil {
			return err
		}
	}
	if err = s.commands(ctx, p.Install); err != nil {
		return err
	}
	return s.writeState(p, true, hash)
}

// Unschedule removes only entries with a local ownership record and our marker.
// An empty name removes all schedules in this scope, even for deleted entries.
func (s Scheduler) Unschedule(ctx context.Context, project, name string) error {
	unlock, err := localdata.Lock(s.GridHome, ".routine-timers.lock")
	if err != nil {
		return err
	}
	defer unlock()
	if name != "" {
		return s.remove(ctx, project, name)
	}
	// The manifest stores the scope/name separately to permit cleanup after a
	// definition disappears.
	dir, err := localdata.Safe(s.GridHome, "local", "routine-timers")
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), LabelPrefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path, err := localdata.Safe(dir, entry.Name())
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var state timerState
		if err = json.Unmarshal(b, &state); err != nil {
			return err
		}
		if state.Project != project {
			continue
		}
		if state.Name == "" {
			return fmt.Errorf("timer manifest has no routine name")
		}
		if err = s.remove(ctx, project, state.Name); err != nil {
			return err
		}
	}
	return nil
}
func (s Scheduler) remove(ctx context.Context, project, name string) error {
	p, err := s.plan(project, name)
	if err != nil {
		return err
	}
	state, err := s.readState(p)
	if err != nil {
		return err
	}
	if state.Label == "" {
		return nil
	}
	if err = s.checkFiles(p); err != nil {
		return err
	}
	// A failed registration can mean the OS already had someone else's label.
	// Only a confirmed installation authorizes removing an OS registration.
	if state.Installed {
		if err = s.commands(ctx, p.Remove); err != nil {
			return err
		}
	}
	for _, f := range p.Files {
		if err = os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if state.Installed {
		if err = s.commands(ctx, p.AfterRemove); err != nil {
			return err
		}
	}
	path, err := s.statePath(p.Label)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// RemoveRoutine closes the install/delete race by holding the timer lock through
// both OS deregistration and deletion of the config entry.
func (s Scheduler) RemoveRoutine(ctx context.Context, store Store, name string) error {
	unlock, err := localdata.Lock(s.GridHome, ".routine-timers.lock")
	if err != nil {
		return err
	}
	defer unlock()
	release, err := store.lock(name)
	if err != nil {
		return err
	}
	defer release()
	if err = s.remove(ctx, store.Project, name); err != nil {
		return err
	}
	return store.remove(name)
}

// Preview returns the native Windows fallback: complete XML and schtasks argv,
// without writing files, taking an unsupported lock or invoking schtasks.
func (s Scheduler) Preview(store Store, remove bool) ([]TimerPlan, error) {
	plans := []TimerPlan{}
	entries, err := store.List()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Machine != gridhome.Machine(s.GridHome) {
			continue
		}
		p, err := s.plan(store.Project, e.Name)
		if err != nil {
			return nil, err
		}
		if remove {
			p.Install = nil
		} else {
			p.Remove = nil
			p.AfterRemove = nil
		}
		plans = append(plans, p)
	}
	return plans, nil
}
