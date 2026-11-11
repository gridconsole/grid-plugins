package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// staleEvery bounds how often tools/call re-reads plugin.json.
const staleEvery = 60 * time.Second

// staleNow is the clock for the stale check; tests replace it.
var staleNow = time.Now

var stale struct {
	sync.Mutex
	root    string
	checked time.Time
	onDisk  string
}

// diskVersion returns the plugin version on disk, or "" when there is none to
// report: CLAUDE_PLUGIN_ROOT unset, file unreadable or without a version.
func diskVersion() string {
	root := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if root == "" {
		return ""
	}
	stale.Lock()
	defer stale.Unlock()
	now := staleNow()
	if stale.root == root && !stale.checked.IsZero() && now.Sub(stale.checked) >= 0 && now.Sub(stale.checked) < staleEvery {
		return stale.onDisk
	}
	stale.root, stale.checked, stale.onDisk = root, now, ""
	raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &manifest) == nil {
		stale.onDisk = manifest.Version
	}
	return stale.onDisk
}

// withStaleNotice appends one text line when the plugin on disk differs from
// the running build. Otherwise the result is returned untouched.
func (s Server) withStaleNotice(r Result) Result {
	disk := diskVersion()
	if disk == "" || disk == s.Version {
		return r
	}
	content := append(append([]Content(nil), r.Content...), Content{"text",
		"Grid plugin on disk is " + disk + ", this session runs " + s.Version + "; restart the session to load it."})
	r.Content = content
	return r
}
