package doctor

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactSettingsPath(t *testing.T) {
	home := t.TempDir()
	for _, file := range []string{
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(t.TempDir(), "custom-claude", "settings.json"),
	} {
		data, err := json.Marshal(map[string]string{"settings": file})
		if err != nil {
			t.Fatal(err)
		}
		got := Redact(string(data), home, file)
		if got != `{"settings":"[REDACTED]"}` {
			t.Fatalf("settings path not redacted: %s", got)
		}
	}
	file := "C:/" + strings.Join([]string{"config", "claude", "settings.json"}, "/")
	for _, variant := range []string{
		file, strings.ToLower(file), strings.ReplaceAll(file, "/", `\`),
		strings.ReplaceAll(file, "/", `\\`), "/c/" + strings.TrimPrefix(file, "C:/"),
	} {
		if got := Redact("settings="+variant+"\n", "", file); got != "settings=[REDACTED]\n" {
			t.Fatalf("Windows settings variant not redacted: %s", got)
		}
	}
}
