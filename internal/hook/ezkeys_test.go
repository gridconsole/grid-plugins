package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// Round 4 fault F3: the app writes agentSandbox into settings.json; the hook used
// to refuse the whole file and read no hook level from it.
func TestSettingsFileWithAppKeysSetsTheHookLevel(t *testing.T) {
	f := eventSetup(t)
	body := `{"agentSandbox":false,"claudePath":"claude","maxTurns":7,"hooks":{"session-start":"off"}}`
	if err := os.WriteFile(filepath.Join(f.home, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out, log, code := f.run(t, "SessionStart", hookio.Input{})
	if code != 0 || strings.Contains(out, "Grid tracks tasks") {
		t.Fatalf("the session-start off level in the file was not read: %d %q %q", code, out, log)
	}
	if strings.Contains(log, "hook settings") {
		t.Fatalf("app keys must give no diagnostic: %q", log)
	}
}

func TestSettingsFileWithAnUnknownKeyWarnsAndIsStillRead(t *testing.T) {
	f := eventSetup(t)
	body := `{"surprise":1,"hooks":{"session-start":"off"}}`
	if err := os.WriteFile(filepath.Join(f.home, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out, log, code := f.run(t, "SessionStart", hookio.Input{})
	if code != 0 || strings.Contains(out, "Grid tracks tasks") {
		t.Fatalf("the rest of the file was not read: %d %q %q", code, out, log)
	}
	if !strings.Contains(log, `unknown key "surprise"`) || strings.Contains(log, "Cannot read hook settings") {
		t.Fatalf("want a warning, not a refusal: %q", log)
	}
}
