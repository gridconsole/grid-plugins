package packaging

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The owner's default is no telemetry. This fails when the module names a
// telemetry or analytics host, client library or endpoint (sentry, segment,
// posthog and the like). Docs, lane reports and this file are not scanned.
func TestModuleNamesNoTelemetryHost(t *testing.T) {
	root := moduleRoot(t)
	banned := telemetryPattern
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || rel == "docs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || rel == "internal/packaging/no_telemetry_test.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.IndexByte(string(data[:min(len(data), 512)]), 0) >= 0 {
			return nil // binary
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			if m := banned.FindString(line); m != "" {
				t.Errorf("%s:%d names a telemetry host or library: %q", rel, i+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 50 {
		t.Fatalf("scanned only %d files under %s; the walk is broken", scanned, root)
	}
}

var telemetryPattern = regexp.MustCompile(`(?i)(\.|//|/)(` +
	`[a-z0-9-]*sentry[a-z0-9.-]*\.(io|com|net|dev|app)|` +
	`[a-z0-9-]*posthog[a-z0-9.-]*\.(io|com|net|dev|app)|` +
	`[a-z0-9-]*segment\.(io|com)|` +
	`(google-analytics|googletagmanager|mixpanel|amplitude|heap-analytics|hotjar)\.(com|io)` +
	`)|getsentry|segmentio|posthog/posthog|sentry-go|sentry\.dsn|api\.segment\.`)

// The scan must hit real telemetry hosts and pass plain words.
func TestTelemetryPatternSamples(t *testing.T) {
	for _, hit := range []string{
		"https://o123.ingest.sentry.io/456", "https://us.i.posthog.com/capture", "https://api.segment.io/v1/track",
		"https://cdn.segment.com/analytics.js", "github.com/getsentry/sentry-go v0.1", "github.com/segmentio/analytics-go",
	} {
		if !telemetryPattern.MatchString(hit) {
			t.Errorf("pattern missed %q", hit)
		}
	}
	for _, ok := range []string{"DISABLE_TELEMETRY", "path segment", "a segment.length check", "No telemetry."} {
		if telemetryPattern.MatchString(ok) {
			t.Errorf("pattern flagged %q", ok)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test folder")
		}
		dir = parent
	}
}
