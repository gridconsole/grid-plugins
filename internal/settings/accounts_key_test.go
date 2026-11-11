package settings

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// appAccountsFile is the settings.json shape the app's account reads write into the
// Grid home: the "accounts" array, one record per signed-in login.
const appAccountsFile = `{"accounts":[{"id":"claude-ann","kind":"claude","name":"ann@example.com","email":"ann@example.com","dir":"~/.claude-ann","added":"auto"},{"id":"codex-lab","kind":"codex","name":"Lab","email":"","dir":"~/.codex-lab","added":"person"}]}`

func accountsOf(t *testing.T, path string) json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(contents(t, path), &m); err != nil {
		t.Fatalf("%s is not JSON: %v", path, err)
	}
	return m["accounts"]
}

func sameJSON(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return string(xb) == string(yb)
}

func TestSettingsWithAccountsKeyReadsAndShowsBudget(t *testing.T) {
	o, _ := budgetFixture(t)
	put(t, filepath.Join(o.GridHome, "settings.json"), []byte(appAccountsFile), 0o644)

	eff, err := (Store{Home: o.GridHome}).Effective()
	if err != nil {
		t.Fatalf("a settings.json with the accounts key must read: %v", err)
	}
	if out, _ := json.Marshal(eff); eff.Agents["grid-builder"].Model != "sonnet" || strings.Contains(string(out), "ann@example.com") {
		t.Fatalf("the defaults stay and the records are not part of the effective values: %+v", eff)
	}
	shown, err := BudgetShow(o)
	if err != nil {
		t.Fatalf("agent_budget show on a home with accounts: %v", err)
	}
	if out, _ := json.Marshal(shown); strings.Contains(string(out), "ann@example.com") {
		t.Fatalf("the account records must not reach the budget read: %s", out)
	}
	if _, err := ReadHookLevels(o.GridHome, ""); err != nil {
		t.Fatalf("the hook levels read the same file: %v", err)
	}
}

func TestBudgetApplyUndoAndSetKeepTheAccountsKey(t *testing.T) {
	o, _ := budgetFixture(t)
	path := filepath.Join(o.GridHome, "settings.json")
	put(t, path, []byte(appAccountsFile), 0o644)

	o.Preset = "high"
	if _, err := BudgetApply(o); err != nil {
		t.Fatalf("apply on a home with accounts: %v", err)
	}
	overlay := contents(t, path)
	if !strings.Contains(string(overlay), `"budget": "high"`) || !sameJSON(t, accountsOf(t, path), json.RawMessage(strings.TrimSuffix(strings.TrimPrefix(appAccountsFile, `{"accounts":`), `}`))) {
		t.Fatalf("apply must keep the account records it did not write:\n%s", overlay)
	}
	must(t, (Store{Home: o.GridHome}).SetSkill("browser", false))
	if !strings.Contains(string(accountsOf(t, path)), "claude-ann") {
		t.Fatalf("a settings change dropped the account records:\n%s", contents(t, path))
	}
	if _, err := BudgetUndo(o); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if !strings.Contains(string(accountsOf(t, path)), "codex-lab") {
		t.Fatalf("undo dropped the account records:\n%s", contents(t, path))
	}
}

func TestUnknownSettingsKeysAreStillRefused(t *testing.T) {
	o, _ := budgetFixture(t)
	path := filepath.Join(o.GridHome, "settings.json")
	for name, body := range map[string]string{
		"unknown top key":          `{"surprise":1}`,
		"app key with the records": `{"accounts":[],"accountSelection":{"mode":"use","acct":"","fullAt":95}}`,
		"accounts not an array":    `{"accounts":{"id":"claude-ann"}}`,
		"accounts null":            `{"accounts":null}`,
		"accounts twice":           `{"accounts":[],"accounts":[]}`,
		"unknown agent key":        `{"accounts":[],"agents":{"grid-nobody":{"model":"opus"}}}`,
	} {
		put(t, path, []byte(body), 0o644)
		if _, err := (Store{Home: o.GridHome}).Effective(); err == nil {
			t.Errorf("%s: must still be refused", name)
		}
		if _, err := BudgetShow(o); err == nil {
			t.Errorf("%s: budget show must still refuse", name)
		}
	}
}
