package projectenv

import (
	"reflect"
	"strings"
	"testing"
)

// The design's deploy target card (grid-design/project.html:3645, the staging
// record): where it deploys in where, what is deployed and when in runs, and
// the Deploy block's How and Who may deploy in how and who.
const pdDeployExample = `---
name: staging
---
## Environments
### staging
name: staging
kind: deploy
where: https://staging.example.test
runs: v2.8.1-rc2, deployed Fri 23 Oct 09:20
test: make smoke
how: make deploy-staging
who:
  - ann
  - bo
secrets:
  - HEALTH_CHECK_KEY (missing)
approval: no
`

func pdDeployStaging() Entry {
	return Entry{ID: "staging", Name: "staging", Kind: "deploy", Where: "https://staging.example.test",
		Runs: "v2.8.1-rc2, deployed Fri 23 Oct 09:20", Test: "make smoke", How: "make deploy-staging",
		Who: []string{"ann", "bo"}, Secrets: []Secret{{Name: "HEALTH_CHECK_KEY"}}, Approval: lnEnvBool(false)}
}

func TestPdDeployKindRenderParse(t *testing.T) {
	got, err := Render(pdDeployStaging())
	if err != nil {
		t.Fatalf("render kind deploy: %v", err)
	}
	if string(got) != pdDeployExample {
		t.Fatalf("bytes differ:\n%q\nwant\n%q", got, pdDeployExample)
	}
	e, err := Parse("staging", []byte(pdDeployExample))
	if err != nil {
		t.Fatalf("parse kind deploy: %v", err)
	}
	if !reflect.DeepEqual(e, pdDeployStaging()) {
		t.Fatalf("parse: %+v", e)
	}
}

func TestPdDeployKindSaveList(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, pdDeployStaging()); err != nil {
		t.Fatalf("save kind deploy: %v", err)
	}
	if got := string(lnEnvRead(t, s, "staging")); got != pdDeployExample {
		t.Fatalf("stored bytes:\n%q\nwant\n%q", got, pdDeployExample)
	}
	list, err := List(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list, []Entry{pdDeployStaging()}) {
		t.Fatalf("list: %+v", list)
	}
}

// A deploy target needs no how or who: the design draws "Not set".
func TestPdDeployKindBare(t *testing.T) {
	in := Entry{ID: "prod", Name: "Production", Kind: "deploy"}
	b, err := Render(in)
	if err != nil {
		t.Fatalf("render bare deploy: %v", err)
	}
	out, err := Parse("prod", b)
	if err != nil || !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip: %+v %v", out, err)
	}
}

// The old kinds read and write as before; an unknown kind is still refused and
// the message names every kind.
func TestPdDeployOldKindsUnchanged(t *testing.T) {
	for _, k := range []string{"", "local", "remote", "cloud"} {
		in := Entry{ID: "env", Name: "Env", Kind: k}
		b, err := Render(in)
		if err != nil {
			t.Fatalf("kind %q: %v", k, err)
		}
		if line := "kind: " + k + "\n"; (k != "") != strings.Contains(string(b), line) {
			t.Fatalf("kind %q bytes: %q", k, b)
		}
		out, err := Parse("env", b)
		if err != nil || !reflect.DeepEqual(out, in) {
			t.Fatalf("kind %q round trip: %+v %v", k, out, err)
		}
	}
	for _, k := range []string{"Deploy", "deployment", "ci"} {
		_, err := Render(Entry{ID: "env", Name: "Env", Kind: k})
		if err == nil || err.Error() != "environment kind must be local, remote, cloud or deploy" {
			t.Fatalf("kind %q: %v", k, err)
		}
	}
}
