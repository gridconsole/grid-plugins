package packaging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// The plugin ships three agents and a set of skills. The eval runs them, so a
// renamed verb or flag would leave an agent calling a command that does not
// exist. These checks read the shipped .md files and compare what they quote
// against contract/commands.json.

type contractFlags map[string]bool

// contractSurface maps group -> verb ("" for a group that takes no verb) -> flags.
type contractSurface map[string]map[string]contractFlags

func loadContractSurface(t *testing.T) contractSurface {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json does not parse: %v", err)
	}
	out := contractSurface{}
	for _, c := range doc.Commands {
		verbs := map[string]contractFlags{}
		for _, s := range c.Subcommands {
			f := contractFlags{}
			for _, fl := range s.Flags {
				f[fl.Name] = true
			}
			verbs[s.Name] = f
		}
		out[c.Name] = verbs
	}
	return out
}

// frontMatter reads the block the way internal/settings/skill_files_test.go does:
// it opens with "---", closes at the next "---" line, and holds "key: value"
// lines (plus indented continuation lines such as "  - skill"). It is not a
// strict YAML parser: a ": " inside an unquoted description is fine.
func frontMatter(text string) (map[string]string, map[string][]string, error) {
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, fmt.Errorf("does not open with a --- line")
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 && strings.HasSuffix(rest, "\n---") {
		end = len(rest) - len("\n---")
	}
	if end < 0 {
		return nil, nil, fmt.Errorf("front matter is not closed by a --- line")
	}
	keys := map[string]string{}
	lists := map[string][]string{}
	last := ""
	for _, line := range strings.Split(rest[:end], "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			item := strings.TrimSpace(line)
			if last == "" || !strings.HasPrefix(item, "- ") {
				return nil, nil, fmt.Errorf("line %q is not a key: value line or a list item", line)
			}
			lists[last] = append(lists[last], strings.TrimSpace(strings.TrimPrefix(item, "- ")))
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			return nil, nil, fmt.Errorf("line %q is not a key: value line", line)
		}
		last = strings.TrimSpace(line[:i])
		keys[last] = strings.TrimSpace(line[i+1:])
	}
	return keys, lists, nil
}

var (
	backtickSpan = regexp.MustCompile("`([^`\n]*)`")
	quotedText   = regexp.MustCompile(`"[^"]*"`)
	flagToken    = regexp.MustCompile(`(?:^|\s)(--[a-z][a-z0-9-]*)`)
	verbToken    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// checkPluginFiles returns one problem per bad thing, each naming the file and
// the bad name. agents and skills map a file's repo path to its text; skillDirs
// lists the folders under skills/ that hold a SKILL.md. checked counts the
// `grid ...` command spans that were compared with the contract.
func checkPluginFiles(agents, skills map[string]string, skillDirs map[string]bool, surface contractSurface) (problems []string, checked int) {
	return checkPluginFilesWith(agents, skills, nil, skillDirs, surface)
}

// pipedGrid finds a grid command after a pipe, as in `printf ... | grid signal add`.
var pipedGrid = regexp.MustCompile(`\|\s*(grid\s.*)$`)

// commandSpans returns the `grid ...` command texts one line holds: each
// backticked span, or the whole line inside a fenced block, and the command
// after a pipe.
func commandSpans(line string, inFence bool) []string {
	var cands []string
	if inFence {
		cands = []string{strings.TrimSpace(line)}
	} else {
		for _, m := range backtickSpan.FindAllStringSubmatch(line, -1) {
			cands = append(cands, strings.TrimSpace(m[1]))
		}
	}
	var out []string
	for _, c := range cands {
		if c == "grid" || strings.HasPrefix(c, "grid ") {
			out = append(out, c)
		} else if m := pipedGrid.FindStringSubmatch(c); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// checkPluginFilesWith is checkPluginFiles plus docs: files that hold no front
// matter rules (the topics) and are read for their grid commands only.
func checkPluginFilesWith(agents, skills, docs map[string]string, skillDirs map[string]bool, surface contractSurface) (problems []string, checked int) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	type file struct {
		path, want, text string
		agent, doc       bool
	}
	var files []file
	for p, text := range docs {
		files = append(files, file{path: p, text: text, doc: true})
	}
	for p, text := range agents {
		files = append(files, file{path: p, want: strings.TrimSuffix(filepath.Base(p), ".md"), text: text, agent: true})
	}
	for p, text := range skills {
		files = append(files, file{path: p, want: filepath.Base(filepath.Dir(p)), text: text})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	for _, f := range files {
		keys, lists, err := frontMatter(f.text)
		if f.doc {
			// no front matter rules for a doc
		} else if err != nil {
			add("%s: front matter: %v", f.path, err)
		} else {
			if keys["name"] != f.want {
				add("%s: name is %q, want %q", f.path, keys["name"], f.want)
			}
			desc, has := keys["description"]
			if !has || desc == "" || desc == ">" || desc == "|" {
				add("%s: description is missing, empty or not on one line", f.path)
			}
			if f.agent {
				for _, k := range []string{"model", "tools"} {
					if keys[k] == "" {
						add("%s: agent does not set %s", f.path, k)
					}
				}
				names := lists["skills"]
				if v := keys["skills"]; v != "" {
					for _, n := range strings.Split(v, ",") {
						names = append(names, strings.TrimSpace(n))
					}
				}
				for _, n := range names {
					if !skillDirs[n] {
						add("%s: skills: names %q, but skills/%s/SKILL.md does not exist", f.path, n, n)
					}
				}
			}
		}

		inFence := false
		for ln, line := range strings.Split(f.text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			for _, span := range commandSpans(line, inFence) {
				fields := strings.Fields(strings.TrimPrefix(span, "grid"))
				if len(fields) == 0 || !verbToken.MatchString(fields[0]) {
					continue // "grid" alone, or a placeholder such as `grid <verb> --help`
				}
				checked++
				where := fmt.Sprintf("%s:%d", f.path, ln+1)
				group := fields[0]
				verbs, ok := surface[group]
				if !ok {
					add("%s: `%s` names group %q, which commands.json does not have", where, span, group)
					continue
				}
				verb := ""
				if len(fields) > 1 && verbToken.MatchString(fields[1]) {
					if _, named := verbs[fields[1]]; named {
						verb = fields[1]
					} else if _, bare := verbs[""]; !bare {
						add("%s: `%s` names verb %q, which is not a subcommand of grid %s", where, span, fields[1], group)
						continue
					}
				}
				flags, ok := verbs[verb]
				if !ok {
					continue // the group alone, as in "`grid task` opens the task skill"; no verb given
				}
				for _, fm := range flagToken.FindAllStringSubmatch(quotedText.ReplaceAllString(span, ""), -1) {
					if !flags[fm[1]] {
						add("%s: `%s` uses flag %s, which grid %s %s does not have", where, span, fm[1], group, verb)
					}
				}
			}
		}
	}
	return problems, checked
}

func readPluginFiles(t *testing.T) (agents, skills map[string]string, skillDirs map[string]bool) {
	t.Helper()
	agents, skills, skillDirs = map[string]string{}, map[string]string{}, map[string]bool{}
	paths, err := filepath.Glob(filepath.Join("..", "..", "agents", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no agents/*.md found: %v", err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		agents["agents/"+filepath.Base(p)] = string(data)
	}
	dirs, err := os.ReadDir(filepath.Join("..", "..", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "skills", d.Name(), "SKILL.md"))
		if err != nil {
			t.Errorf("skills/%s has no SKILL.md: %v", d.Name(), err)
			continue
		}
		skills["skills/"+d.Name()+"/SKILL.md"] = string(data)
		skillDirs[d.Name()] = true
	}
	return agents, skills, skillDirs
}

// Rows 1-3: every shipped agent and skill is well formed, names only skills that
// exist, and quotes only grid commands and flags that commands.json has.
func TestShippedAgentsAndSkillsAreWellFormed(t *testing.T) {
	agents, skills, skillDirs := readPluginFiles(t)
	problems, checked := checkPluginFiles(agents, skills, skillDirs, loadContractSurface(t))
	for _, p := range problems {
		t.Error(p)
	}
	if checked == 0 {
		t.Fatal("no `grid ...` commands were found in agents or skills; the scan is broken")
	}
	t.Logf("%d agents, %d skills, %d backticked grid commands checked against contract/commands.json", len(agents), len(skills), checked)
}

// Row 4: a deliberately broken fixture makes the checks fail, naming the file
// and the bad name, so the test is shown to bite.
func TestPluginFileChecksBiteOnBrokenFixture(t *testing.T) {
	surface := loadContractSurface(t)
	agents := map[string]string{
		"agents/broken.md": "---\nname: broken\ndescription: Does a thing\nmodel: sonnet\ntools: Read\nskills:\n  - nope\n---\nRun `grid job fetch <job>` now.\n",
		"agents/noname.md": "---\nname: other\ndescription:\n---\n",
	}
	skills := map[string]string{
		"skills/ok/SKILL.md":   "---\nname: ok\ndescription: fine: with a colon\n---\nSee `grid job show <task> <job>` and `grid job show --nosuchflag`, `grid nogroup x`.\n",
		"skills/open/SKILL.md": "no front matter\n",
	}
	problems, checked := checkPluginFiles(agents, skills, map[string]bool{"ok": true}, surface)
	all := strings.Join(problems, "\n")
	for _, want := range []string{
		`agents/broken.md: skills: names "nope"`,
		"agents/broken.md:9: `grid job fetch <job>` names verb \"fetch\"",
		"agents/noname.md: name is \"other\", want \"noname\"",
		"agents/noname.md: description is missing, empty or not on one line",
		"agents/noname.md: agent does not set model",
		"skills/ok/SKILL.md:5: `grid job show --nosuchflag` uses flag --nosuchflag",
		"skills/ok/SKILL.md:5: `grid nogroup x` names group \"nogroup\"",
		"skills/open/SKILL.md: front matter: does not open with a --- line",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("broken fixture did not produce %q; problems were:\n%s", want, all)
		}
	}
	if strings.Contains(all, "skills/ok/SKILL.md: name") || strings.Contains(all, "skills/ok/SKILL.md: description") {
		t.Errorf("a well-formed skill with ': ' in its description was flagged:\n%s", all)
	}
	if checked != 4 {
		t.Errorf("fixture checked %d commands, want 4", checked)
	}
	t.Logf("fixture problems:\n%s", all)
}
