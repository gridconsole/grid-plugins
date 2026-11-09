package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUsageNoticeThresholds(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       map[string][]float64
	}{
		{"absent file", "", nil},
		{"absent key", "---\nname: Ship\n---\n", nil},
		{"nested key", "---\npolicy:\n  usage_notice_thresholds: {tokens: [90, 100]}\n---\n", nil},
		{"body key", "---\nname: Ship\n---\nusage_notice_thresholds: {tokens: [90, 100]}\n", nil},
		{"one unit", "---\nusage_notice_thresholds: {tokens: [90, 100]}\n---\n", map[string][]float64{"tokens": {90, 100}}},
		{"three units", "---\nusage_notice_thresholds:\n  tokens: [90, 100]\n  dollars: [25.5, 75, 100]\n  minutes: [100]\n---\n", map[string][]float64{"tokens": {90, 100}, "dollars": {25.5, 75, 100}, "minutes": {100}}},
		{"crlf", "---\r\nusage_notice_thresholds: {minutes: [80, 100]}\r\n---\r\n", map[string][]float64{"minutes": {80, 100}}},
		{"empty map", "---\nusage_notice_thresholds: {}\n---\n", map[string][]float64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.text != "" {
				writeUsageWorkflow(t, dir, tc.text)
			}
			got, err := UsageNoticeThresholds(dir)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("thresholds=%v err=%v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestUsageNoticeThresholdsInvalidUnit(t *testing.T) {
	for _, tc := range []struct{ name, unit, value string }{
		{"unknown unit", "credits", "[80, 100]"},
		{"not a list", "tokens", "90"},
		{"not numbers", "tokens", "[ninety, 100]"},
		{"quoted number", "tokens", "['90', 100]"},
		{"boolean", "tokens", "[true, 100]"},
		{"null item", "tokens", "[null, 100]"},
		{"null list", "tokens", "null"},
		{"empty list", "tokens", "[]"},
		{"descending", "tokens", "[90, 80, 100]"},
		{"duplicate", "tokens", "[80, 80, 100]"},
		{"zero", "tokens", "[0, 100]"},
		{"negative", "tokens", "[-1, 100]"},
		{"missing 100", "tokens", "[80, 90]"},
		{"over 100", "tokens", "[80, 101]"},
		{"nan", "tokens", "[.nan, 100]"},
		{"infinity", "tokens", "[.inf, 100]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeUsageWorkflow(t, dir, "---\nusage_notice_thresholds:\n  dollars: [75, 100]\n  "+tc.unit+": "+tc.value+"\n---\n")
			got, err := UsageNoticeThresholds(dir)
			if err == nil || !strings.Contains(err.Error(), "usage_notice_thresholds") || !strings.Contains(err.Error(), tc.unit) || strings.Contains(err.Error(), dir) {
				t.Fatalf("want error naming key and unit without path; got %v", err)
			}
			if !reflect.DeepEqual(got, map[string][]float64{"dollars": {75, 100}}) {
				t.Fatalf("valid unit lost or invalid unit retained: %v", got)
			}
		})
	}
}

func TestUsageNoticeThresholdsInvalidFrontMatter(t *testing.T) {
	for _, text := range []string{
		"usage_notice_thresholds: {}\n",
		"---\nusage_notice_thresholds: {}\n",
		"---\nusage_notice_thresholds: [\n---\n",
		"---\nusage_notice_thresholds: null\n---\n",
		"---\nusage_notice_thresholds: [80, 100]\n---\n",
		"---\nusage_notice_thresholds: {tokens: [80, 100], tokens: [90, 100]}\n---\n",
	} {
		dir := t.TempDir()
		writeUsageWorkflow(t, dir, text)
		got, err := UsageNoticeThresholds(dir)
		if err == nil || !strings.Contains(err.Error(), "usage_notice_thresholds") || strings.Contains(err.Error(), dir) || got != nil {
			t.Fatalf("thresholds=%v err=%v; want path-free key error", got, err)
		}
	}
}

func TestUsageNoticeThresholdsReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "workflow.md"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := UsageNoticeThresholds(dir)
	if err == nil || !strings.Contains(err.Error(), "usage_notice_thresholds") || strings.Contains(err.Error(), dir) {
		t.Fatalf("want path-free read error, got %v", err)
	}
}

func writeUsageWorkflow(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
