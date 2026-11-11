package migrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// jzMgHostData fills a plugin data folder beside the old home with one session pointer
// that names a task of the old home: the folder a process variable would point at.
func jzMgHostData(t *testing.T, from, name string) string {
	t.Helper()
	data := filepath.Join(filepath.Dir(from), name)
	put(t, filepath.Join(data, "sessions/hook.tasks"), filepath.Join(from, "alpha/projects/widget/tasks/T-07-first")+"\r\n")
	return data
}

// jzMgManifest reads the manifest of the backup an apply made.
func jzMgManifest(t *testing.T, backup string) manifest {
	t.Helper()
	var man manifest
	if err := json.Unmarshal([]byte(read(t, backup, "manifest.json")), &man); err != nil {
		t.Fatal(err)
	}
	return man
}

// Scan takes the plugin data folder only from WithPluginData. A process variable that
// names a folder with a session pointer in it adds nothing to the plan: no field, no
// action in the report, nothing in the manifest or the backup, and the folder is
// left as it was.
func TestJzMgScanReadsNoProcessVariable(t *testing.T) {
	from, _ := fixture(t)
	host := jzMgHostData(t, from, "host-plugin-data")
	t.Setenv("CLAUDE_PLUGIN_DATA", host)
	before := view(t, host)

	p := scan(t, from, from)
	if p.pluginData != "" || len(p.plugin.m) != 0 {
		t.Fatalf("the plan took the process variable: plugin data %q, %d files", p.pluginData, len(p.plugin.m))
	}
	var report bytes.Buffer
	if err := p.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report.String(), pluginSessionsPrefix) || strings.Contains(report.String(), host) {
		t.Fatalf("the --dry report names the host plugin data folder:\n%s", report.String())
	}

	res, err := p.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if man := jzMgManifest(t, res.Backup); man.PluginData != "" || len(man.PluginFiles) != 0 {
		t.Fatalf("the manifest names plugin data %q with %d files", man.PluginData, len(man.PluginFiles))
	}
	absent(t, filepath.Join(res.Backup, "plugin-sessions"))
	if !sameView(before, view(t, host)) {
		t.Fatal("the host plugin data folder changed")
	}
}

// The option is the one way in: with it the plan names that folder in the report and
// the manifest, whatever the process variable says, and a variable alone still adds nothing.
func TestJzMgWithPluginDataIsTheOnlyWayIn(t *testing.T) {
	from, _ := fixture(t)
	host := jzMgHostData(t, from, "host-plugin-data")
	chosen := jzMgHostData(t, from, "chosen-plugin-data")
	t.Setenv("CLAUDE_PLUGIN_DATA", host)
	hostBefore := view(t, host)

	p := scan(t, from, from, WithPluginData(chosen))
	var report bytes.Buffer
	if err := p.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), "copy: "+pluginSessionsPrefix+"/hook.tasks -> sessions/hook.tasks") {
		t.Fatalf("the report lacks the plugin data pointer:\n%s", report.String())
	}
	want := gridhome.Real(filepath.Join(chosen, "sessions"))
	if p.pluginData != want {
		t.Fatalf("plugin data is %q, want %q", p.pluginData, want)
	}
	res, err := p.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if man := jzMgManifest(t, res.Backup); man.PluginData != want || len(man.PluginFiles) == 0 {
		t.Fatalf("the manifest names plugin data %q with %d files, want %q", man.PluginData, len(man.PluginFiles), want)
	}
	if _, err := os.Stat(filepath.Join(res.Backup, "plugin-sessions", "hook.tasks")); err != nil {
		t.Fatal(err)
	}
	if !sameView(hostBefore, view(t, host)) {
		t.Fatal("the folder the process variable names changed")
	}
}
