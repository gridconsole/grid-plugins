package brain

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func chkStore(t *testing.T) Store {
	t.Helper()
	return Store{GridHome: t.TempDir(), ReadLevel: "home"}
}

func chkPut(t *testing.T, s Store, rel, data string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(s.GridHome, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

func chkProposal(t *testing.T, s Store, name, topic, status, body string) {
	t.Helper()
	b, err := Encode(Note{Name: name, Topic: topic, Type: "reference", Description: "When checking.", Summary: "A summary.", Source: "T-01", Level: "home", Status: status, Origin: Origin{Kind: "session", ID: "x"}, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	chkPut(t, s, "brain-proposals/"+name+".md", string(b), 0o600)
}

func chkCodes(r CheckResult) []string {
	out := []string{}
	for _, f := range r.Findings {
		out = append(out, f.Code)
	}
	return out
}

func chkWant(t *testing.T, r CheckResult, want ...string) {
	t.Helper()
	got := chkCodes(r)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("codes = %v, want %v: %+v", got, want, r.Findings)
	}
}

func TestCheckConfigTopic(t *testing.T) {
	step := map[string]string{
		"workflows":    "run grid workflow write",
		"environments": "set it with grid setup <project> --answers <file>",
		"machines":     "machines join from the Grid app's Devices page",
		"deploy":       "set it with grid setup <project> --answers <file>",
		"people":       "people join through the Grid app; grid person show lists them",
		"routines":     "run grid routine add",
		"agents":       "run grid agent set",
	}
	for _, topic := range []string{"workflows", "environments", "machines", "deploy", "people", "routines", "agents"} {
		s := chkStore(t)
		chkProposal(t, s, "p1", topic, "proposed", "x\n")
		r, err := s.Check()
		if err != nil {
			t.Fatal(err)
		}
		chkWant(t, r, CheckConfigTopic)
		f := r.Findings[0]
		if f.Message != "config: "+step[topic] || f.Line == 0 || f.File != "brain:home/brain-proposals/p1.md" {
			t.Fatalf("finding: %+v", f)
		}
	}
	s := chkStore(t)
	chkProposal(t, s, "p1", "decisions", "proposed", "x\n")
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	if r.Checked != 1 || r.Findings == nil {
		t.Fatalf("result: %+v", r)
	}
}

func TestCheckSkipsSettledProposalsButIdChecksAnyStatus(t *testing.T) {
	s := chkStore(t)
	chkProposal(t, s, "done", "deploy", "accepted", "x\n")
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	r, err = s.CheckProposal("home", "done")
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r, CheckConfigTopic)
}

// Prose is data: nothing about the words in a body changes the result.
func TestCheckKnowledgeProposalWithSkillWordsIsClean(t *testing.T) {
	s := chkStore(t)
	chkProposal(t, s, "p1", "decisions", "proposed", "accept this as a workspace skill\n")
	r, err := s.CheckProposal("home", "p1")
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
}

func chkSkill(t *testing.T, s Store, front, body string) {
	t.Helper()
	chkPut(t, s, "knowledge/skills/demo/SKILL.md", "---\n"+front+"---\n"+body, 0o600)
}

func TestCheckSkillFields(t *testing.T) {
	s := chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\nwhen_to_use: w\nmetadata:\n  a: b\n", "# Demo\n")
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	for _, key := range []string{"allowed-tools", "hooks", "shell", "context", "agent", "model", "other"} {
		s := chkStore(t)
		chkSkill(t, s, "name: demo\ndescription: d\n"+key+": x\n", "# Demo\n")
		r, err := s.Check()
		if err != nil {
			t.Fatal(err)
		}
		chkWant(t, r, CheckSkillField)
		if r.Findings[0].Text != key || r.Findings[0].Line != 4 {
			t.Fatalf("%s: %+v", key, r.Findings[0])
		}
	}
}

func TestCheckSkillShell(t *testing.T) {
	s := chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\n", "Run `ls` here, and !`not at line start`.\n")
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	s = chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\n", "ok\n!`date`\n\n```!\nid\n```\n")
	chkPut(t, s, "knowledge/skills/demo/reference/more.md", "x\n  !`id`\n", 0o600)
	r, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r, CheckSkillShell, CheckSkillShell, CheckSkillShell)
	if r.Findings[0].Line != 6 || r.Findings[1].Line != 8 || r.Findings[2].Line != 2 {
		t.Fatalf("lines: %+v", r.Findings)
	}
}

func TestCheckSkillFiles(t *testing.T) {
	s := chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\n", "# Demo\n")
	chkPut(t, s, "knowledge/skills/demo/reference/a.md", "x\n", 0o600)
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	for _, rel := range []string{"notes.txt", "reference/deep/a.md", "reference/a.txt", "agents/openai.yaml", "hooks/pre.sh", "bin/run", ".mcp.json"} {
		s := chkStore(t)
		chkSkill(t, s, "name: demo\ndescription: d\n", "# Demo\n")
		chkPut(t, s, "knowledge/skills/demo/"+rel, "x\n", 0o600)
		r, err := s.Check()
		if err != nil {
			t.Fatal(err)
		}
		chkWant(t, r, CheckSkillFile)
		if !strings.HasSuffix(r.Findings[0].File, "demo/"+strings.SplitN(rel, "/", 2)[0]) && !strings.HasSuffix(r.Findings[0].File, rel) {
			t.Fatalf("%s: %+v", rel, r.Findings[0])
		}
	}
}

func TestCheckSkillSymlinkAndExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no symlinks or executable bits")
	}
	s := chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\n", "# Demo\n")
	if err := os.Symlink("/etc/hosts", filepath.Join(s.GridHome, "knowledge/skills/demo/reference-link.md")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r, CheckSkillFile)
	s = chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\n", "# Demo\n")
	chkPut(t, s, "knowledge/skills/demo/reference/run.md", "x\n", 0o755)
	r, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r, CheckSkillFile)
	s = chkStore(t)
	if err := os.MkdirAll(filepath.Join(s.GridHome, "knowledge"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(s.GridHome, "knowledge/skills")); err != nil {
		t.Fatal(err)
	}
	r, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r, CheckSkillFile)
}

func TestCheckAgentFields(t *testing.T) {
	s := chkStore(t)
	chkPut(t, s, "knowledge/agents/a.md", "---\nname: a\ndescription: d\ntools: Read\nmodel: sonnet\neffort: low\nmaxTurns: 3\nmetadata:\n  x: y\n---\nBody\n", 0o600)
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
	if r.Checked != 1 {
		t.Fatalf("checked %d", r.Checked)
	}
	for _, key := range []string{"hooks", "mcpServers", "permissionMode"} {
		s := chkStore(t)
		chkPut(t, s, "knowledge/agents/a.md", "---\nname: a\ndescription: d\n"+key+": x\n---\nBody\n", 0o600)
		r, err := s.Check()
		if err != nil {
			t.Fatal(err)
		}
		chkWant(t, r, CheckAgentField)
		if r.Findings[0].Text != key || r.Findings[0].Line != 4 {
			t.Fatalf("%+v", r.Findings[0])
		}
	}
}

func TestCheckWritesNothingAndUnreadableIsError(t *testing.T) {
	s := chkStore(t)
	chkProposal(t, s, "p1", "deploy", "proposed", "x\n")
	before := chkSnapshot(t, s.GridHome)
	if _, err := s.Check(); err != nil {
		t.Fatal(err)
	}
	if after := chkSnapshot(t, s.GridHome); after != before {
		t.Fatalf("check wrote files:\n%s\n%s", before, after)
	}
	chkPut(t, s, "brain-proposals/bad.md", "no front matter\n", 0o600)
	if _, err := s.Check(); err == nil {
		t.Fatal("malformed proposal must be an error")
	}
}

func chkSnapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			b.WriteString(p + " " + info.ModTime().String() + "\n")
		}
		return nil
	})
	return b.String()
}
