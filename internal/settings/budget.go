package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"go.yaml.in/yaml/v4"
)

var budgetPresetNames = []string{"economic", "balanced", "high"}

// CodexPreset holds the Codex values of one preset; Codex is shown, never written.
type CodexPreset struct {
	Compact        int     `json:"compact" yaml:"compact"`
	SubagentModel  *string `json:"subagent_model" yaml:"subagent_model"`
	SubagentEffort string  `json:"subagent_effort" yaml:"subagent_effort"`
}

// BudgetPreset is one column of the preset table.
type BudgetPreset struct {
	Compact int              `json:"compact" yaml:"compact"`
	Advisor string           `json:"advisor" yaml:"advisor"`
	Agents  map[string]Agent `json:"agents" yaml:"agents"`
}

// BudgetData is the content of the "Agent budget" note.
type BudgetData struct {
	Default string
	Presets map[string]BudgetPreset
	Codex   map[string]CodexPreset
}

type budgetFront struct {
	Name    string                  `yaml:"name"`
	Title   string                  `yaml:"title"`
	Updated string                  `yaml:"updated"`
	Default string                  `yaml:"default"`
	Presets map[string]BudgetPreset `yaml:"presets"`
	Codex   map[string]CodexPreset  `yaml:"codex"`
}

// BudgetNote reports where the values came from: ok, missing (built-in values) or invalid.
type BudgetNote struct {
	Path  string  `json:"path"`
	State string  `json:"state"`
	Error *string `json:"error"`
}

func budgetNotePath(gridHome string) string {
	return filepath.Join(gridHome, "knowledge", "budget", "agent-budget.md")
}

// BuiltinBudget is the same table the note ships with; it is used only while the
// note is missing or invalid, and Apply never runs from it.
func BuiltinBudget() BudgetData {
	set := func(review, build Agent, reviewEffort string) map[string]Agent {
		review.Effort = reviewEffort
		return map[string]Agent{"grid-planner": review, "grid-reviewer": review, "grid-researcher": review, "grid-builder": build, "grid-designer": build}
	}
	econ := set(Agent{Model: "opus"}, Agent{"sonnet", "medium"}, "medium")
	bal := set(Agent{Model: "opus"}, Agent{"sonnet", "high"}, "medium")
	high := set(Agent{Model: "opus"}, Agent{"opus", "high"}, "high")
	return BudgetData{Default: "balanced",
		Presets: map[string]BudgetPreset{
			"economic": {Compact: 200000, Advisor: "off", Agents: econ},
			"balanced": {Compact: 250000, Advisor: "opus", Agents: bal},
			"high":     {Compact: 350000, Advisor: "fable", Agents: high}},
		Codex: map[string]CodexPreset{
			"economic": {Compact: 200000, SubagentEffort: "medium"},
			"balanced": {Compact: 250000, SubagentEffort: "medium"},
			"high":     {Compact: 350000, SubagentEffort: "high"}}}
}

func validateBudget(f budgetFront) error {
	if len(f.Presets) != len(budgetPresetNames) {
		return fmt.Errorf("presets must be exactly %s", strings.Join(budgetPresetNames, ", "))
	}
	for name, p := range f.Presets {
		if err := allowed("preset", name, budgetPresetNames); err != nil {
			return err
		}
		if p.Compact < 100000 || p.Compact > 1000000 {
			return fmt.Errorf("preset %s: compact must be a whole number from 100000 to 1000000", name)
		}
		if err := allowed("advisor", p.Advisor, advisorNames); err != nil {
			return fmt.Errorf("preset %s: %w", name, err)
		}
		if len(p.Agents) == 0 {
			return fmt.Errorf("preset %s lists no agents", name)
		}
		for n, ag := range p.Agents {
			if err := allowed("agent", n, AgentNames()); err != nil {
				return fmt.Errorf("preset %s: %w", name, err)
			}
			if ag.Model == "" || ag.Effort == "" {
				return fmt.Errorf("preset %s: %s needs model and effort", name, n)
			}
			if err := validateAgent(ag); err != nil {
				return fmt.Errorf("preset %s: %s: %w", name, n, err)
			}
		}
	}
	if f.Default != "" {
		if err := allowed("default preset", f.Default, budgetPresetNames); err != nil {
			return err
		}
	}
	for name, c := range f.Codex {
		if err := allowed("codex preset", name, budgetPresetNames); err != nil {
			return err
		}
		if c.Compact != 0 && (c.Compact < 100000 || c.Compact > 1000000) {
			return fmt.Errorf("codex %s: compact must be a whole number from 100000 to 1000000", name)
		}
	}
	return nil
}

// ReadBudgetNote reads the preset values from the note <home>/knowledge/budget/agent-budget.md
// with the strict front matter parser. A missing or invalid note yields the built-in
// values and says so; the raw bytes go into the Apply digest.
func ReadBudgetNote(gridHome string) (BudgetData, BudgetNote, []byte) {
	path := budgetNotePath(gridHome)
	note := BudgetNote{Path: path, State: "ok"}
	fail := func(state string, err error) (BudgetData, BudgetNote, []byte) {
		msg := err.Error()
		note.State, note.Error = state, &msg
		return BuiltinBudget(), note, nil
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fail("missing", errors.New("no Agent budget note"))
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return fail("invalid", errors.New("the Agent budget note must be a regular file under 1 MiB"))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fail("invalid", err)
	}
	front, _, err := mdfile.Split(raw)
	if err != nil {
		return fail("invalid", err)
	}
	var f budgetFront
	if err := mdfile.Load(front, &f, true); err != nil {
		return fail("invalid", err)
	}
	if err := validateBudget(f); err != nil {
		return fail("invalid", err)
	}
	d := BudgetData{Default: f.Default, Presets: f.Presets, Codex: f.Codex}
	if d.Default == "" {
		d.Default = "balanced"
	}
	return d, note, raw
}

// ---- the agent files ----

type agentFile struct {
	file          claudeFile
	state         string // grid, missing or yours
	model, effort string
}

type agentFront struct {
	Model    string `yaml:"model"`
	Effort   string `yaml:"effort"`
	Metadata struct {
		Grid struct {
			Owner string `yaml:"owner"`
		} `yaml:"grid"`
	} `yaml:"metadata"`
}

func readAgentFile(path string) (agentFile, error) {
	f, err := readClaudeFile(path)
	if err != nil {
		return agentFile{}, err
	}
	af := agentFile{file: f, state: "missing"}
	if !f.exists {
		return af, nil
	}
	af.state = "yours"
	front, _, err := mdfile.Split(f.data)
	var fm agentFront
	if err == nil && yaml.Load(front, &fm) == nil && fm.Metadata.Grid.Owner == "grid" {
		af.state, af.model, af.effort = "grid", fm.Model, fm.Effort
	}
	return af, nil
}

// rewriteAgentFile replaces only the model: and effort: lines of the front matter.
func rewriteAgentFile(data []byte, model, effort string) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return data
	}
	eol := "\n"
	if strings.HasSuffix(lines[0], "\r\n") {
		eol = "\r\n"
	}
	set := func(key, val string) {
		for i := 1; i < end; i++ {
			if strings.HasPrefix(lines[i], key+":") {
				lines[i] = key + ": " + val + lines[i][len(strings.TrimRight(lines[i], "\r\n")):]
				return
			}
		}
		lines = slices.Insert(lines, end, key+": "+val+eol)
		end++
	}
	set("model", model)
	set("effort", effort)
	return []byte(strings.Join(lines, ""))
}

func fileSHA(f claudeFile) string {
	if !f.exists {
		return ""
	}
	return shaOf(f.data)
}
func shaOf(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func tilde(home, p string) string {
	if home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(p[len(home)+1:])
	}
	return filepath.ToSlash(p)
}

// ---- state, show and preview ----

// BudgetOptions locates everything Apply, Undo and the reads touch.
type BudgetOptions struct {
	Home, GridHome, ConfigDir, ProjectDir string
	Getenv                                func(string) string
	Now                                   func() time.Time
	Preset                                string
	With                                  []string // compact and/or advisor
	Expect                                string   // digest from the preview; empty skips the check
	beforeRename                          func()
	store                                 budgetStore // bound by Store.WithBudget, the caller-aware store
}

// budgetStore is the part of a Store the budget needs; the package builds no Store itself.
type budgetStore struct {
	directory func() (string, error)
	effective func() (Document, error)
	locked    func(func() error) error
}

func (o BudgetOptions) lock(f func() error) error {
	if o.store.locked == nil {
		return errors.New("budget options are not bound to a settings store")
	}
	return o.store.locked(f)
}

type budgetState struct {
	data       BudgetData
	note       BudgetNote
	noteRaw    []byte
	names      []string
	eff        Document
	explicit   Document
	overlay    claudeFile
	agentsDir  string
	agents     map[string]agentFile
	settings   claudeFile
	getenv     func(string) string
	overlayDir string
}

func (o BudgetOptions) env() func(string) string {
	if o.Getenv != nil {
		return o.Getenv
	}
	return os.Getenv
}

func (o BudgetOptions) load() (*budgetState, error) {
	if o.store.directory == nil {
		return nil, errors.New("budget options are not bound to a settings store")
	}
	dir, err := o.store.directory()
	if err != nil {
		return nil, err
	}
	st := &budgetState{getenv: o.env(), overlayDir: dir, agents: map[string]agentFile{}}
	if st.eff, err = o.store.effective(); err != nil {
		return nil, err
	}
	if st.explicit, err = readDocument(dir); err != nil {
		return nil, err
	}
	st.data, st.note, st.noteRaw = ReadBudgetNote(o.GridHome)
	if st.overlay, err = readClaudeFile(filepath.Join(dir, "settings.json")); err != nil {
		return nil, err
	}
	settingsPath, err := ClaudeSettingsPath(o.Home, o.ConfigDir)
	if err != nil {
		return nil, err
	}
	if st.settings, err = readClaudeFile(settingsPath); err != nil {
		return nil, err
	}
	st.agentsDir = filepath.Join(filepath.Dir(settingsPath), "agents")
	for _, n := range AgentNames() {
		for _, p := range st.data.Presets {
			if _, ok := p.Agents[n]; ok && !slices.Contains(st.names, n) {
				st.names = append(st.names, n)
			}
		}
	}
	for _, n := range st.names {
		if st.agents[n], err = readAgentFile(filepath.Join(st.agentsDir, n+".md")); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func (st *budgetState) projectFile(o BudgetOptions, name string) bool {
	if o.ProjectDir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(o.ProjectDir, ".claude", "agents", name+".md"))
	return err == nil && info.Mode().IsRegular()
}

func (st *budgetState) digest(o BudgetOptions) string {
	h := sha256.New()
	add := func(label string, f claudeFile) {
		fmt.Fprintf(h, "%s|%t|%d|%s\n", label, f.exists, len(f.data), shaOf(f.data))
	}
	fmt.Fprintf(h, "%s|%s|%s\n", o.Preset, strings.Join(o.With, ","), shaOf(st.noteRaw))
	add("overlay", st.overlay)
	for _, n := range st.names {
		add(n, st.agents[n].file)
	}
	add("settings", st.settings)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// BudgetAgentRow is one agent in the read.
type BudgetAgentRow struct {
	Model         string `json:"model"`
	Effort        string `json:"effort"`
	File          string `json:"file"`
	State         string `json:"state"`
	ChangedByHand bool   `json:"changedByHand"`
}

type BudgetEnv struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

type BudgetUndoInfo struct {
	At     string `json:"at"`
	Preset string `json:"preset"`
	From   string `json:"from"`
}

type BudgetShowResult struct {
	Note         BudgetNote                `json:"note"`
	Presets      map[string]BudgetPreset   `json:"presets"`
	CodexPresets map[string]CodexPreset    `json:"codexPresets"`
	Agents       map[string]BudgetAgentRow `json:"agents"`
	Applied      *string                   `json:"applied"`
	Closest      string                    `json:"closest"`
	Custom       []string                  `json:"custom"`
	Env          []BudgetEnv               `json:"env"`
	Undo         *BudgetUndoInfo           `json:"undo"`
}

func matches(eff Document, p BudgetPreset) (n int, diff []string) {
	diff = []string{}
	for _, name := range AgentNames() {
		want, ok := p.Agents[name]
		if !ok {
			continue
		}
		if eff.Agents[name] == want {
			n++
		} else {
			diff = append(diff, name)
		}
	}
	return n, diff
}

// BudgetShow reads the note, the overlay, the agent files and the environment.
func BudgetShow(o BudgetOptions) (BudgetShowResult, error) {
	st, err := o.load()
	if err != nil {
		return BudgetShowResult{}, err
	}
	res := BudgetShowResult{Note: st.note, Presets: st.data.Presets, CodexPresets: st.data.Codex,
		Agents: map[string]BudgetAgentRow{}, Env: []BudgetEnv{}, Custom: []string{}}
	for _, n := range st.names {
		af, eff := st.agents[n], st.eff.Agents[n]
		row := BudgetAgentRow{Model: eff.Model, Effort: eff.Effort, File: tilde(o.Home, af.file.path), State: af.state,
			ChangedByHand: af.state == "grid" && (af.model != eff.Model || af.effort != eff.Effort)}
		if st.projectFile(o, n) {
			row.State = "project"
		}
		res.Agents[n] = row
	}
	best := -1
	for _, name := range append([]string{st.data.Default}, budgetPresetNames...) {
		p, ok := st.data.Presets[name]
		if !ok {
			continue
		}
		n, diff := matches(st.eff, p)
		if n > best {
			best, res.Closest, res.Custom = n, name, diff
		}
		if len(diff) == 0 && res.Applied == nil {
			applied := name
			if st.eff.Budget != "" && st.eff.Budget == name {
				res.Applied = &applied
				break
			}
			res.Applied = &applied
		}
	}
	if res.Applied != nil {
		res.Custom = []string{}
	}
	effects := map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "overrides-effort", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "forces-model", "CLAUDE_CODE_SUBAGENT_MODEL": "fills-gaps"}
	for _, n := range []string{"CLAUDE_CODE_EFFORT_LEVEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "CLAUDE_CODE_SUBAGENT_MODEL"} {
		if v := st.getenv(n); v != "" {
			res.Env = append(res.Env, BudgetEnv{n, v, effects[n]})
		}
	}
	if snap, err := readSnapshot(o); err == nil {
		res.Undo = &BudgetUndoInfo{At: snap.At, Preset: snap.Preset, From: snap.From}
	}
	return res, nil
}

type BudgetFileChange struct {
	Agent          string `json:"agent"`
	Path           string `json:"path"`
	From           Agent  `json:"from"`
	To             Agent  `json:"to"`
	ReplacesByHand bool   `json:"replacesByHand"`
	ProjectWins    bool   `json:"projectWins,omitempty"`
}

type BudgetSkip struct {
	Agent  string `json:"agent"`
	Reason string `json:"reason"`
}

type BudgetSettingChange struct {
	File    string          `json:"file"`
	Key     string          `json:"key"`
	From    json.RawMessage `json:"from"`
	To      json.RawMessage `json:"to"`
	Blocked *string         `json:"blocked"`
}

type BudgetCodexChange struct {
	File      string `json:"file"`
	Key       string `json:"key"`
	From      any    `json:"from"`
	To        any    `json:"to"`
	Confirmed bool   `json:"confirmed"`
}

type BudgetPreview struct {
	Preset    string                `json:"preset"`
	Files     []BudgetFileChange    `json:"files"`
	Unchanged int                   `json:"unchanged"`
	Skipped   []BudgetSkip          `json:"skipped"`
	Settings  []BudgetSettingChange `json:"settings"`
	Codex     []BudgetCodexChange   `json:"codex"`
	Digest    string                `json:"digest"`
}

type agentWrite struct {
	name       string
	file       agentFile
	data       []byte
	from, to   Agent
	overlayOld *Agent
}

type budgetWork struct {
	preview         BudgetPreview
	overlayData     []byte
	overlayBudget   string
	writes          []agentWrite
	settingsData    []byte
	settingsEdits   []keyEdit
	settingsFrom    []json.RawMessage
	settingsChanged bool
}

func (st *budgetState) plan(o BudgetOptions) (*budgetWork, error) {
	if st.note.State == "invalid" {
		return nil, codedErr(codeInvalidSettings, st.note.Path, "The Agent budget note is invalid: %s. Nothing applied.", *st.note.Error)
	}
	p, ok := st.data.Presets[o.Preset]
	if !ok {
		return nil, codedErr(codeInvalidSettings, "", "unknown preset %q; allowed: %s", o.Preset, strings.Join(budgetPresetNames, ", "))
	}
	for _, w := range o.With {
		switch w {
		case "compact", "advisor":
		case "codex":
			return nil, codedErr(codeUnconfirmedKey, "", "Codex keys are read-only until checked on a real install. Nothing written.")
		default:
			return nil, codedErr(codeInvalidSettings, "", "unknown setting %q; allowed: compact, advisor", w)
		}
	}
	w := &budgetWork{overlayBudget: o.Preset}
	pv := &w.preview
	*pv = BudgetPreview{Preset: o.Preset, Files: []BudgetFileChange{}, Skipped: []BudgetSkip{}, Settings: []BudgetSettingChange{}, Codex: []BudgetCodexChange{}}
	doc := st.explicit
	if doc.Agents == nil {
		doc.Agents = map[string]Agent{}
	}
	doc.Agents = cloneAgents(doc.Agents)
	for _, n := range st.names {
		want, ok := p.Agents[n]
		if !ok {
			continue
		}
		cur := st.eff.Agents[n]
		doc.Agents[n] = want
		af := st.agents[n]
		switch {
		case af.state == "missing" || af.state == "yours":
			pv.Skipped = append(pv.Skipped, BudgetSkip{n, af.state})
		case af.model == want.Model && af.effort == want.Effort:
			if cur == want {
				pv.Unchanged++
			}
		default:
			pv.Files = append(pv.Files, BudgetFileChange{Agent: n, Path: tilde(o.Home, af.file.path), From: Agent{af.model, af.effort}, To: want,
				ReplacesByHand: af.model != cur.Model || af.effort != cur.Effort, ProjectWins: st.projectFile(o, n)})
			var oldp *Agent
			if old, ok := st.explicit.Agents[n]; ok {
				c := old
				oldp = &c
			}
			w.writes = append(w.writes, agentWrite{name: n, file: af, data: rewriteAgentFile(af.file.data, want.Model, want.Effort), from: Agent{af.model, af.effort}, to: want, overlayOld: oldp})
		}
	}
	doc.Budget = o.Preset
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	w.overlayData = append(b, '\n')
	blocked := AdvisorBlocked(st.getenv)
	for _, with := range o.With {
		edit := keyEdit{Key: "autoCompactWindow", Value: json.RawMessage(fmt.Sprint(p.Compact))}
		if with == "advisor" {
			raw, _ := json.Marshal(p.Advisor)
			edit = keyEdit{Key: "advisorModel", Value: raw}
		}
		row := BudgetSettingChange{File: tilde(o.Home, st.settings.path), Key: edit.Key, To: edit.Value}
		if with == "advisor" && len(blocked) > 0 {
			row.Blocked = &blocked[0]
			pv.Settings = append(pv.Settings, row)
			continue
		}
		w.settingsEdits = append(w.settingsEdits, edit)
		pv.Settings = append(pv.Settings, row)
	}
	if len(w.settingsEdits) > 0 {
		if w.settingsData, w.settingsFrom, w.settingsChanged, err = applyClaudeEdits(st.settings, w.settingsEdits); err != nil {
			return nil, err
		}
		i := 0
		for k := range pv.Settings {
			if pv.Settings[k].Blocked == nil {
				pv.Settings[k].From = w.settingsFrom[i]
				i++
			}
		}
	}
	codex := readCodex(filepath.Join(codexDir(o), "config.toml"))
	if c, ok := st.data.Codex[o.Preset]; ok {
		add := func(key string, to any) {
			if to != nil && fmt.Sprint(codex.Rows[key]) != fmt.Sprint(to) {
				pv.Codex = append(pv.Codex, BudgetCodexChange{File: tilde(o.Home, codex.Path), Key: key, From: codex.Rows[key], To: to})
			}
		}
		if c.Compact > 0 {
			add("model_auto_compact_token_limit", int64(c.Compact))
		}
		if c.SubagentModel != nil {
			add("agents.default_subagent_model", *c.SubagentModel)
		}
		if c.SubagentEffort != "" {
			add("agents.default_subagent_reasoning_effort", c.SubagentEffort)
		}
	}
	pv.Digest = st.digest(o)
	return w, nil
}

func codexDir(o BudgetOptions) string {
	if d := o.env()("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(o.Home, ".codex")
}

func cloneAgents(m map[string]Agent) map[string]Agent {
	out := make(map[string]Agent, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// BudgetPreviewFor is the dry run: what would change, per file, and a digest of what was read.
func BudgetPreviewFor(o BudgetOptions) (BudgetPreview, error) {
	st, err := o.load()
	if err != nil {
		return BudgetPreview{}, err
	}
	w, err := st.plan(o)
	if err != nil {
		return BudgetPreview{}, err
	}
	return w.preview, nil
}

// ---- apply and undo ----

type snapFile struct {
	Path     string `json:"path"`
	Existed  bool   `json:"existed"`
	Before   []byte `json:"before"`
	AfterSHA string `json:"afterSha"`
}

type snapAgent struct {
	Agent         string `json:"agent"`
	File          snapFile
	OverlayBefore *Agent `json:"overlayBefore"`
	OverlayAfter  Agent  `json:"overlayAfter"`
}

type snapSetting struct {
	Key    string          `json:"key"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

type budgetSnapshot struct {
	At           string        `json:"at"`
	Preset       string        `json:"preset"`
	From         string        `json:"from"`
	Overlay      snapFile      `json:"overlay"`
	BudgetBefore string        `json:"budgetBefore"`
	Agents       []snapAgent   `json:"agents"`
	SettingsFile snapFile      `json:"settingsFile"`
	Settings     []snapSetting `json:"settings"`
}

func snapshotPath(o BudgetOptions) string {
	return filepath.Join(o.GridHome, "setup", "agent-budget-undo.json")
}

func readSnapshot(o BudgetOptions) (budgetSnapshot, error) {
	var s budgetSnapshot
	data, err := os.ReadFile(snapshotPath(o))
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	return s, err
}

func snapOf(f claudeFile, after []byte) snapFile {
	return snapFile{Path: f.path, Existed: f.exists, Before: f.data, AfterSHA: shaOf(after)}
}

type BudgetApplyResult struct {
	Applied  string `json:"applied"`
	At       string `json:"at"`
	Files    int    `json:"files"`
	Settings int    `json:"settings"`
	Undo     bool   `json:"undo"`
}

type staged struct {
	target, tmp string
	src         *claudeFile
	before      []byte
	existed     bool
	mode        os.FileMode
}

// BudgetApply writes the overlay, the model and effort lines of Grid's own agent
// files and, when asked, settings.json: every target is staged first, the undo
// snapshot is written, and only then are the files renamed, under the settings lock.
func BudgetApply(o BudgetOptions) (BudgetApplyResult, error) {
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	var res BudgetApplyResult
	err := o.lock(func() error {
		st, err := o.load()
		if err != nil {
			return err
		}
		w, err := st.plan(o)
		if err != nil {
			return err
		}
		if o.Expect != "" && o.Expect != w.preview.Digest {
			return codedErr(codeChanged, "", "The files changed since the preview. Look at the preview again. Nothing written.")
		}
		snap := budgetSnapshot{At: now().UTC().Format(time.RFC3339), Preset: o.Preset, From: st.eff.Budget, BudgetBefore: st.explicit.Budget}
		var stages []staged
		cleanup := func() {
			for _, s := range stages {
				os.Remove(s.tmp)
			}
		}
		defer cleanup()
		stage := func(f claudeFile, data []byte, mode os.FileMode) error {
			tmp, err := stageClaudeFile(f.target, data, mode)
			if err != nil {
				return &CodedError{Code: codeWriteFailed, Path: f.path, Msg: err.Error(), Err: err}
			}
			src := f
			stages = append(stages, staged{f.target, tmp, &src, f.data, f.exists, mode})
			return nil
		}
		for _, wr := range w.writes {
			if err := stage(wr.file.file, wr.data, wr.file.file.mode); err != nil {
				return err
			}
			snap.Agents = append(snap.Agents, snapAgent{Agent: wr.name, File: snapOf(wr.file.file, wr.data), OverlayBefore: wr.overlayOld, OverlayAfter: wr.to})
		}
		if w.settingsChanged {
			if err := stage(st.settings, w.settingsData, st.settings.mode); err != nil {
				return err
			}
			snap.SettingsFile = snapOf(st.settings, w.settingsData)
			for i, e := range w.settingsEdits {
				snap.Settings = append(snap.Settings, snapSetting{e.Key, w.settingsFrom[i], e.Value})
			}
		}
		overlayMode := os.FileMode(0o644)
		if st.overlay.exists {
			overlayMode = st.overlay.mode
		}
		if err := stage(st.overlay, w.overlayData, overlayMode); err != nil {
			return err
		}
		snap.Overlay = snapOf(st.overlay, w.overlayData)
		snapData, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		snapFileState, err := readClaudeFile(snapshotPath(o))
		if err != nil {
			return err
		}
		snapFileState.target = snapshotPath(o)
		if err := stage(snapFileState, append(snapData, '\n'), 0o600); err != nil {
			return err
		}
		if o.beforeRename != nil {
			o.beforeRename()
		}
		for _, s := range stages {
			if s.src != nil && !s.src.unchanged() {
				return codedErr(codeChanged, s.src.path, "%s", errClaudeChanged.Error())
			}
		}
		done := 0
		for ; done < len(stages); done++ {
			s := stages[done]
			if err = os.Rename(s.tmp, s.target); err != nil {
				break
			}
		}
		if err != nil {
			for _, s := range stages[:done] { // put back what was already replaced
				if s.existed {
					_ = atomicfile.Write(s.target, s.before, s.mode)
				} else {
					_ = os.Remove(s.target)
				}
			}
			return &CodedError{Code: codeWriteFailed, Path: stages[done].target, Msg: err.Error(), Err: err}
		}
		for _, s := range stages {
			_ = atomicfile.SyncDir(filepath.Dir(s.target))
		}
		res = BudgetApplyResult{Applied: o.Preset, At: snap.At, Files: len(w.writes), Settings: len(snap.Settings), Undo: true}
		return nil
	})
	return res, err
}

type BudgetUndoResult struct {
	Back     *string      `json:"back"`
	Restored []string     `json:"restored"`
	Skipped  []BudgetSkip `json:"skipped"`
	Settings []string     `json:"settings"`
}

// BudgetUndo restores the state before the last Apply, one level: a value goes back
// only where it still equals what Apply wrote; then the snapshot is deleted.
func BudgetUndo(o BudgetOptions) (BudgetUndoResult, error) {
	res := BudgetUndoResult{Restored: []string{}, Skipped: []BudgetSkip{}, Settings: []string{}}
	err := o.lock(func() error {
		snap, err := readSnapshot(o)
		if err != nil {
			return codedErr(codeNothingToUndo, snapshotPath(o), "There is no Apply to undo.")
		}
		restore := func(f snapFile, cur claudeFile) error {
			if !f.Existed {
				return os.Remove(cur.target)
			}
			return atomicfile.Write(cur.target, f.Before, cur.mode)
		}
		restored := map[string]bool{}
		for _, ag := range snap.Agents {
			cur, err := readClaudeFile(ag.File.Path)
			if err != nil {
				return err
			}
			if fileSHA(cur) != ag.File.AfterSHA {
				res.Skipped = append(res.Skipped, BudgetSkip{ag.Agent, "changed after apply"})
				continue
			}
			if err := restore(ag.File, cur); err != nil {
				return &CodedError{Code: codeWriteFailed, Path: ag.File.Path, Msg: err.Error(), Err: err}
			}
			restored[ag.Agent] = true
		}
		ov, err := readClaudeFile(snap.Overlay.Path)
		if err != nil {
			return err
		}
		if fileSHA(ov) == snap.Overlay.AfterSHA {
			if err := restore(snap.Overlay, ov); err != nil {
				return &CodedError{Code: codeWriteFailed, Path: ov.path, Msg: err.Error(), Err: err}
			}
		} else if dir, err := o.store.directory(); err == nil { // the overlay moved on: put back only Apply's own entries
			d, err := readDocument(dir)
			if err != nil {
				return err
			}
			for _, ag := range snap.Agents {
				if d.Agents[ag.Agent] == ag.OverlayAfter {
					if ag.OverlayBefore == nil {
						delete(d.Agents, ag.Agent)
					} else {
						d.Agents[ag.Agent] = *ag.OverlayBefore
					}
					restored[ag.Agent] = true
				}
			}
			if d.Budget == snap.Preset {
				d.Budget = snap.BudgetBefore
			}
			b, err := json.MarshalIndent(d, "", "  ")
			if err != nil {
				return err
			}
			if err := atomicfile.Write(ov.target, append(b, '\n'), 0o644); err != nil {
				return &CodedError{Code: codeWriteFailed, Path: ov.path, Msg: err.Error(), Err: err}
			}
		}
		for _, ag := range snap.Agents {
			if restored[ag.Agent] {
				res.Restored = append(res.Restored, ag.Agent)
			}
		}
		if len(snap.Settings) > 0 {
			cur, err := readClaudeFile(snap.SettingsFile.Path)
			if err != nil {
				return err
			}
			if fileSHA(cur) == snap.SettingsFile.AfterSHA {
				if err := restore(snap.SettingsFile, cur); err != nil {
					return &CodedError{Code: codeWriteFailed, Path: cur.path, Msg: err.Error(), Err: err}
				}
				for _, s := range snap.Settings {
					res.Settings = append(res.Settings, s.Key)
				}
			} else {
				edits := []keyEdit{}
				for _, s := range snap.Settings {
					doc, perr := parseClaudeObject(append([]byte(nil), orEmpty(cur.data)...))
					if perr != nil {
						return codedErr(codeInvalidSettings, cur.path, "Settings file %s is not valid JSON. Settings left as they are.", cur.path)
					}
					if now, _ := getClaudePath(doc, s.Key); canon(now) == canon(s.After) {
						edits = append(edits, keyEdit{s.Key, s.Before})
						res.Settings = append(res.Settings, s.Key)
					}
				}
				if len(edits) > 0 {
					data, _, changed, err := applyClaudeEdits(cur, edits)
					if err != nil {
						return err
					}
					if changed {
						if err := atomicfile.Write(cur.target, data, cur.mode); err != nil {
							return &CodedError{Code: codeWriteFailed, Path: cur.path, Msg: err.Error(), Err: err}
						}
					}
				}
			}
		}
		if snap.From != "" {
			back := snap.From
			res.Back = &back
		}
		return os.Remove(snapshotPath(o))
	})
	return res, err
}

func orEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}
