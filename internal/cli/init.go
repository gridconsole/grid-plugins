package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/account"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// grid init asks the first-run questions of docs/research/first-run-config.md
// section 5 once and writes the answers to the one config every binary reads
// (gridconfig.Path). --detect fills each question from the machine and says where
// its default came from and whether it is asked; --answers - reads the answers and
// writes the config, keeping every key it does not know, and registers the agent
// accounts the person kept. A missing config is always an offer, never a refusal.

// initQuestion is one question of --detect.
type initQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Default  any    `json:"default"`
	// Source is where the default came from: flag, env, config, legacy, registry,
	// found, account, login or default.
	Source string `json:"source"`
	Asked  bool   `json:"asked"`
	Note   string `json:"note,omitempty"`
}

// initAccount is one agent account of the accounts question.
type initAccount struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Email  string `json:"email"`
	Dir    string `json:"dir"`
	Source string `json:"source"`
}

// initDetection is the --detect answer.
type initDetection struct {
	Config    string         `json:"config"`
	Found     bool           `json:"found"`
	Problem   string         `json:"problem,omitempty"`
	Workspace string         `json:"workspace"`
	Questions []initQuestion `json:"questions"`
}

// initResult is the --answers answer.
type initResult struct {
	Config    string   `json:"config"`
	Written   bool     `json:"written"`
	Workspace string   `json:"workspace"`
	Data      string   `json:"data"`
	Member    string   `json:"member"`
	Accounts  []string `json:"accounts"`
}

// initAnswers is what --answers reads. Every key may be left out: the detected
// default is used. Accounts left out keeps every account found; a list keeps only
// the folders it names.
type initAnswers struct {
	Workspace      string      `json:"workspace"`
	Data           string      `json:"data"`
	Brain          *string     `json:"brain"`
	Member         string      `json:"member"`
	Folder         string      `json:"folder"`
	Accounts       *[]initKeep `json:"accounts"`
	ClaudeSettings string      `json:"claudeSettings"`
	Sandbox        *bool       `json:"sandbox"`
}

// initKeep names one kept account by its folder or its login email.
type initKeep struct {
	Kind  string `json:"kind"`
	Dir   string `json:"dir"`
	Email string `json:"email"`
}

// initPlan is what the machine says before any question is answered.
type initPlan struct {
	path      string
	found     bool
	problem   string
	cfg       gridconfig.Config
	cfgSource string
	name      string
	data      string
	dataSrc   string
	dataAsk   bool
	brain     string
	brainSrc  string
	member    string
	memberSrc string
	login     string
}

func runInit(args []string, env Env) int {
	fs := newFlags("init", env)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: grid init [--detect] [--answers <file|->] [--workspace <name>] [--json]")
	}
	detect := fs.Bool("detect", false, "print each first-run question with its default, its source and whether it is asked")
	source := fs.String("answers", "", "answers JSON file relative to the current folder, or - for stdin; writes the config")
	workspace := fs.String("workspace", "", "the config entry to use")
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *detect && *source != "" {
		fmt.Fprintln(env.Stderr, "Usage: grid init --detect or grid init --answers <file|->, not both")
		return 2
	}
	if *workspace != "" && !gridconfig.ValidName(*workspace) {
		fmt.Fprintf(env.Stderr, "grid init: --workspace %q is not a plain name\n", *workspace)
		return 2
	}
	if *source == "" {
		d := initDetect(env, initPlanFor(env, *workspace))
		if *asJSON {
			return writeJSON(env.Stdout, d)
		}
		initPrintDetection(env.Stdout, d)
		return 0
	}
	data, err := initReadAnswers(env, *source)
	if err != nil {
		return commandError(env, "init", err)
	}
	var ans initAnswers
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans); err != nil {
		return commandError(env, "init", fmt.Errorf("answers: %w", err))
	}
	if ans.Workspace != "" && *workspace != "" && ans.Workspace != *workspace {
		fmt.Fprintf(env.Stderr, "grid init: --workspace %q and the answers' workspace %q differ\n", *workspace, ans.Workspace)
		return 2
	}
	name := *workspace
	if name == "" {
		name = ans.Workspace
	}
	res, err := initApply(env, initPlanFor(env, name), ans)
	if err != nil {
		return commandError(env, "init", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, res)
	}
	state := "unchanged"
	if res.Written {
		state = "written"
	}
	fmt.Fprintf(env.Stdout, "Config %s: %s\nWorkspace %s, data %s, member %s\n", state, res.Config, res.Workspace, res.Data, res.Member)
	if len(res.Accounts) > 0 {
		fmt.Fprintf(env.Stdout, "Accounts: %s\n", strings.Join(res.Accounts, ", "))
	}
	return 0
}

// initReadAnswers reads the answers from stdin for "-", else from the file.
func initReadAnswers(env Env, source string) ([]byte, error) {
	var r io.Reader = env.Stdin
	if source != "-" {
		path := source
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	if r == nil {
		return nil, errors.New("no answers input")
	}
	const limit = 1 << 20
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, fmt.Errorf("answers exceed %d bytes", limit)
	}
	return b, nil
}

// initLoginName is the OS login name: USER, else USERNAME, else LOGNAME.
func initLoginName(env Env) string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := strings.TrimSpace(env.getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// initPlanFor reads the config and fills the data folder, brain and member for the
// entry workspace names (else the config's current one). GRID_HOME and --grid-home
// win over the config for the data folder.
func initPlanFor(env Env, workspace string) initPlan {
	p := initPlan{path: gridconfig.Path(env.getenv, env.Home), login: initLoginName(env), cfgSource: string(gridhome.SourceConfig)}
	l := gridconfig.Load(p.path, gridconfig.OldPath(env.Home))
	p.found, p.cfg = l.Found, l.Config
	if l.Err != nil {
		p.problem = l.Err.Error()
		p.cfg = gridconfig.Config{}
	} else if p.cfg.Migrate(env.Home, gridconfig.HandleFromLogin(p.login)) {
		p.cfgSource = string(gridhome.SourceLegacy)
	}
	root := gridconfig.DataRoot(env.Home)
	p.name = workspace
	if p.name == "" {
		p.name = p.cfg.Current
	}
	entry := p.cfg.Entry(p.name)
	gh := env.getenv("GRID_HOME")
	if gh != "" && workspace == "" {
		// The entry that names the GRID_HOME folder, as gridhome.Resolve picks it.
		entry, p.name = nil, ""
		for i := range p.cfg.Workspaces {
			if d := p.cfg.Workspaces[i].DataFolder(env.Home); d != "" && gridhome.Real(d) == gridhome.Real(gh) {
				entry, p.name = &p.cfg.Workspaces[i], p.cfg.Workspaces[i].Name
				break
			}
		}
	}
	candidates := initCandidates(env.Home, p.cfg)
	switch {
	case gh != "":
		p.data, p.dataSrc = filepath.Clean(gh), string(gridhome.SourceEnv)
		if env.typedGlobal["GRID_HOME"] {
			p.dataSrc = string(gridhome.SourceFlag)
		}
	case entry != nil && entry.DataFolder(env.Home) != "":
		p.data, p.dataSrc = entry.DataFolder(env.Home), p.cfgSource
	case workspace != "" && root != "":
		// A name the config does not hold yet: a new entry is offered.
		p.data, p.dataSrc = filepath.Join(root, workspace), string(gridhome.SourceDefault)
	default:
		if best := initMostProjects(candidates); best != "" {
			p.data, p.dataSrc = best, "found"
		} else {
			p.data, p.dataSrc = root, string(gridhome.SourceDefault)
		}
	}
	p.dataAsk = p.dataSrc != string(gridhome.SourceEnv) && p.dataSrc != string(gridhome.SourceFlag) && len(candidates) != 1
	p.brain, p.brainSrc = p.data, string(gridhome.SourceDefault)
	if entry != nil && entry.BrainFolder() != "" {
		p.brain, p.brainSrc = entry.BrainFolder(), p.cfgSource
	}
	switch {
	case entry != nil && entry.Member != "":
		p.member, p.memberSrc = entry.Member, p.cfgSource
	default:
		if name, src := gridhome.DefaultMember(env.getenv, env.Home); src == gridhome.SourceAccount && gridconfig.HandleFromLogin(name) != "" {
			p.member, p.memberSrc = gridconfig.HandleFromLogin(name), string(gridhome.SourceAccount)
		} else if h := gridconfig.HandleFromLogin(p.login); h != "" {
			p.member, p.memberSrc = h, string(gridhome.SourceLogin)
		} else {
			p.memberSrc = string(gridhome.SourceDefault)
		}
	}
	return p
}

// initCandidates are the data folders the person may pick: each config entry's,
// ~/.grid itself when it holds data, and every ~/.grid/* folder that holds data.
func initCandidates(userHome string, cfg gridconfig.Config) []string {
	var out []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" || seen[gridhome.Real(dir)] {
			return
		}
		seen[gridhome.Real(dir)] = true
		out = append(out, dir)
	}
	for _, w := range cfg.Workspaces {
		add(w.DataFolder(userHome))
	}
	root := gridconfig.DataRoot(userHome)
	if root == "" {
		return out
	}
	if initHoldsData(root) {
		add(root)
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && gridconfig.ValidName(e.Name()) && initHoldsData(filepath.Join(root, e.Name())) {
			add(filepath.Join(root, e.Name()))
		}
	}
	return out
}

// initHoldsData reports a folder that holds Grid data: VERSION, projects/ or knowledge/.
func initHoldsData(dir string) bool {
	for _, name := range []string{"VERSION", "projects", "knowledge"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// initMostProjects is the candidate with the most project folders, the first one on
// a tie; "" when no candidate exists on disk.
func initMostProjects(candidates []string) string {
	best, most := "", -1
	for _, dir := range candidates {
		if !initHoldsData(dir) {
			continue
		}
		n := 0
		entries, _ := os.ReadDir(filepath.Join(dir, "projects"))
		for _, e := range entries {
			if e.IsDir() {
				n++
			}
		}
		if n > most {
			best, most = dir, n
		}
	}
	return best
}

func initAccountEnv(env Env, data string) account.Env {
	return account.Env{Home: env.Home, GridHome: data, Getenv: env.getenv}
}

// initAccounts lists the registered accounts and the signed-in folders the registry
// does not hold yet; asked is true when there is such a folder.
func initAccounts(env Env, data string) (list []initAccount, asked bool, problem string) {
	list = []initAccount{}
	aenv := initAccountEnv(env, data)
	var known []account.Account
	if data != "" {
		var err error
		if known, err = account.Status(aenv, false); err != nil {
			problem = err.Error()
		}
	}
	for _, a := range known {
		list = append(list, initAccount{ID: a.ID, Kind: a.Kind, Email: a.Email, Dir: a.Dir, Source: "registry"})
	}
	for _, f := range account.Scan(aenv) {
		if f.Email == "" {
			continue
		}
		registered := false
		for _, a := range known {
			registered = registered || a.Kind == f.Kind && strings.EqualFold(a.Email, f.Email)
		}
		if !registered {
			list = append(list, initAccount{Kind: f.Kind, Email: f.Email, Dir: f.Dir, Source: "found"})
			asked = true
		}
	}
	return list, asked, problem
}

// initChoice reads one choices key as a string or a bool.
func initChoice(cfg gridconfig.Config, key string, into any) bool {
	raw, ok := cfg.Choices[key]
	return ok && json.Unmarshal(raw, into) == nil
}

// initDetect fills every section 5 question from the plan and the machine.
func initDetect(env Env, p initPlan) initDetection {
	d := initDetection{Config: p.path, Found: p.found, Problem: p.problem, Workspace: p.name}
	data := initQuestion{ID: "data", Question: "Data folder", Default: p.data, Source: p.dataSrc, Asked: p.dataAsk}
	if p.dataSrc == string(gridhome.SourceEnv) || p.dataSrc == string(gridhome.SourceFlag) {
		data.Note = "GRID_HOME or --grid-home wins over the config"
	}
	accounts, asked, problem := initAccounts(env, p.data)
	acctSrc := string(gridhome.SourceDefault)
	for _, a := range accounts {
		if a.Source == "found" || acctSrc == string(gridhome.SourceDefault) {
			acctSrc = a.Source
		}
	}
	acctQ := initQuestion{ID: "accounts", Question: "Agent accounts", Default: accounts, Source: acctSrc, Asked: asked,
		Note: "all kept by default"}
	if problem != "" {
		acctQ.Note = problem
	}
	claude, claudeSrc := "yes", string(gridhome.SourceDefault)
	if initChoice(p.cfg, "claudeSettings", &claude) {
		claudeSrc = p.cfgSource
	}
	sandbox, sandboxSrc := false, string(gridhome.SourceDefault)
	if initChoice(p.cfg, "sandbox", &sandbox) {
		sandboxSrc = p.cfgSource
	}
	test, testSrc := "", string(gridhome.SourceDefault)
	if initChoice(p.cfg, "testDefault", &test) {
		testSrc = p.cfgSource
	}
	var packs any = []any{}
	packsSrc := string(gridhome.SourceDefault)
	if initChoice(p.cfg, "packs", &packs) {
		packsSrc = p.cfgSource
	}
	d.Questions = []initQuestion{
		data,
		{ID: "brain", Question: "Where the brain lives", Default: p.brain, Source: p.brainSrc, Note: "settled; the brain stays in the data folder unless the config names another"},
		{ID: "member", Question: "Name in Grid", Default: p.member, Source: p.memberSrc, Asked: true, Note: "shown once, prefilled, to confirm"},
		acctQ,
		{ID: "claudeSettings", Question: "Claude Code settings entries", Default: claude, Source: claudeSrc, Asked: true, Note: "grid setup previews and writes the entries"},
		{ID: "sandbox", Question: "Sandbox", Default: sandbox, Source: sandboxSrc, Note: "not asked; grid setup --sandbox adds it"},
		{ID: "testDefault", Question: "Test default", Default: test, Source: testSrc, Note: "asked only in the app"},
		{ID: "packs", Question: "Packs", Default: packs, Source: packsSrc, Note: "asked only in the app"},
	}
	return d
}

func initPrintDetection(w io.Writer, d initDetection) {
	state := "found"
	if !d.Found {
		state = "none yet; grid init --answers - writes it"
	}
	fmt.Fprintf(w, "Config: %s (%s)\n", d.Config, state)
	if d.Problem != "" {
		fmt.Fprintf(w, "Config problem: %s\n", d.Problem)
	}
	for _, q := range d.Questions {
		mark := "settled"
		if q.Asked {
			mark = "asked"
		}
		def := q.Default
		if list, ok := def.([]initAccount); ok {
			var names []string
			for _, a := range list {
				names = append(names, a.Kind+" "+a.Email)
			}
			def = strings.Join(names, ", ")
		}
		fmt.Fprintf(w, "%s: %v (%s, %s)\n", q.Question, def, q.Source, mark)
	}
}

// initApply writes the answers to the config and registers the kept accounts. The
// file is written only when its bytes change, so a second run with the same
// answers changes no byte; setupAt is stamped only on a write.
func initApply(env Env, p initPlan, ans initAnswers) (initResult, error) {
	if p.problem != "" {
		return initResult{}, fmt.Errorf("%s cannot be read (%s); fix or move it, then run grid init again", p.path, p.problem)
	}
	if p.path == "" {
		return initResult{}, errors.New("cannot find the config folder: the home folder is unknown")
	}
	data := p.data
	if ans.Data != "" {
		if !filepath.IsAbs(ans.Data) {
			return initResult{}, fmt.Errorf("data %q is not an absolute folder", ans.Data)
		}
		data = filepath.Clean(ans.Data)
	}
	member := p.member
	if ans.Member != "" {
		member = ans.Member
	}
	if !gridconfig.ValidName(member) {
		return initResult{}, fmt.Errorf("member %q is not a plain name", member)
	}
	name := p.name
	if name == "" {
		name = gridconfig.HandleFromLogin(member)
	}
	if name == "" {
		name = gridconfig.FallbackName
	}
	cfg := p.cfg
	entry := cfg.Entry(name)
	if entry == nil {
		cfg.Workspaces = append(cfg.Workspaces, gridconfig.Workspace{Name: name, Setup: "started"})
		entry = &cfg.Workspaces[len(cfg.Workspaces)-1]
	}
	if data == "" {
		return initResult{}, errors.New("cannot find a data folder: the home folder is unknown")
	}
	if entry.DataFolder(env.Home) != data {
		entry.Data = data
		if data == filepath.Join(gridconfig.DataRoot(env.Home), name) {
			entry.Data = ""
		}
	}
	if ans.Brain != nil {
		if *ans.Brain != "" && !filepath.IsAbs(*ans.Brain) {
			return initResult{}, fmt.Errorf("brain %q is not an absolute folder", *ans.Brain)
		}
		entry.Brain = *ans.Brain
	}
	if ans.Folder != "" {
		if !filepath.IsAbs(ans.Folder) {
			return initResult{}, fmt.Errorf("folder %q is not an absolute folder", ans.Folder)
		}
		entry.Folder = filepath.Clean(ans.Folder)
	}
	entry.Member = member
	cfg.Current = name
	if ans.ClaudeSettings != "" || ans.Sandbox != nil {
		if cfg.Choices == nil {
			cfg.Choices = map[string]json.RawMessage{}
		}
	}
	if ans.ClaudeSettings != "" {
		if ans.ClaudeSettings != "yes" && ans.ClaudeSettings != "no" {
			return initResult{}, fmt.Errorf("claudeSettings %q is not yes or no", ans.ClaudeSettings)
		}
		cfg.Choices["claudeSettings"], _ = json.Marshal(ans.ClaudeSettings)
	}
	if ans.Sandbox != nil {
		cfg.Choices["sandbox"], _ = json.Marshal(*ans.Sandbox)
	}
	b, err := cfg.Marshal()
	if err != nil {
		return initResult{}, err
	}
	res := initResult{Config: p.path, Workspace: name, Data: data, Member: member, Accounts: []string{}}
	if old, err := os.ReadFile(p.path); err != nil || !bytes.Equal(old, b) {
		cfg.SetupAt = time.Now().UTC().Format(time.RFC3339)
		if err := gridconfig.Write(p.path, cfg); err != nil {
			return initResult{}, err
		}
		res.Written = true
	}
	var keep func(account.Folder) bool
	if ans.Accounts != nil {
		kept := *ans.Accounts
		keep = func(f account.Folder) bool {
			for _, k := range kept {
				if k.Kind != "" && k.Kind != f.Kind {
					continue
				}
				if k.Dir != "" && gridhome.Real(k.Dir) == gridhome.Real(f.Dir) || k.Email != "" && strings.EqualFold(k.Email, f.Email) {
					return true
				}
			}
			return false
		}
	}
	accts, err := account.RegisterKept(initAccountEnv(env, data), keep)
	if err != nil {
		return res, fmt.Errorf("config %s saved; registering the accounts: %w", p.path, err)
	}
	for _, a := range accts {
		res.Accounts = append(res.Accounts, a.ID)
	}
	sort.Strings(res.Accounts)
	return res, nil
}
