package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/projectenv"
)

// grid project env add --kind deploy writes a deploy target; list reads it back,
// and both answers pass cmd-project-env.json.
func TestPdProjectEnvDeployKind(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	answer := lnEnvCliRun(t, h, "add", "alpha", "--id", "staging", "--name", "staging", "--kind", "deploy",
		"--where", "https://staging.example.test", "--runs", "v2.8.1-rc2, deployed Fri 23 Oct 09:20",
		"--how", "make deploy-staging", "--who", "ann", "--who", "bo", "--secret", "HEALTH_CHECK_KEY:missing", "--json")
	want := map[string]any{"id": "staging", "name": "staging", "kind": "deploy", "where": "https://staging.example.test",
		"runs": "v2.8.1-rc2, deployed Fri 23 Oct 09:20", "how": "make deploy-staging", "who": []any{"ann", "bo"},
		"secrets": []any{map[string]any{"name": "HEALTH_CHECK_KEY", "set": false}}}
	if !reflect.DeepEqual(answer, map[string]any{"state": "applied", "item": want}) {
		t.Fatalf("add answer = %v", answer)
	}
	entry := projectenv.Entry{ID: "staging", Name: "staging", Kind: "deploy", Where: "https://staging.example.test",
		Runs: "v2.8.1-rc2, deployed Fri 23 Oct 09:20", How: "make deploy-staging", Who: []string{"ann", "bo"},
		Secrets: []projectenv.Secret{{Name: "HEALTH_CHECK_KEY"}}}
	if raw, ok := lnEnvCliRaw(t, h, "staging"); !ok || raw != lnEnvCliRender(t, entry) || !strings.Contains(raw, "\nkind: deploy\n") {
		t.Fatalf("entry file:\n%s\nwant projectenv.Render:\n%s", raw, lnEnvCliRender(t, entry))
	}
	if got := lnEnvCliRun(t, h, "list", "alpha", "--json"); !reflect.DeepEqual(got, map[string]any{"items": []any{want}}) {
		t.Fatalf("list = %v", got)
	}
	// An existing environment can become a deploy target and go back.
	lnEnvCliRun(t, h, "add", "alpha", "--id", "box", "--name", "Box", "--kind", "remote", "--json")
	for _, k := range []string{"deploy", "remote"} {
		got := lnEnvCliRun(t, h, "edit", "alpha", "--id", "box", "--kind", k, "--json")
		if item, _ := got["item"].(map[string]any); item["kind"] != k {
			t.Fatalf("edit --kind %s answer = %v", k, got)
		}
	}
}

func TestPdProjectEnvUsageNamesDeploy(t *testing.T) {
	if !strings.Contains(projectEnvUsage, "--kind local|remote|cloud|deploy") {
		t.Fatalf("usage: %s", projectEnvUsage)
	}
}
