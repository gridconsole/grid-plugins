package hook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

func TestPermissionForwardAndGuardPrecedence(t *testing.T) {
	f := eventSetup(t)
	var calls atomic.Int32
	answer := map[string]any{"behavior": "allow"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/hooks/permission" || r.Method != "POST" {
			t.Error("wrong request")
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	defer server.Close()
	f.env["GRID_URL"] = server.URL
	in := hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(filepath.Dir(f.home), "outside.md")}}
	out, log, code := f.run(t, "PermissionRequest", in)
	if code != 0 || log != "" || !strings.Contains(out, `"behavior":"allow"`) {
		t.Fatalf("allow: %d %s %s", code, out, log)
	}
	in.ToolName = "Write"
	in.ToolInput["file_path"] = filepath.Join(f.dir, "task.md")
	out, _, _ = f.run(t, "PermissionRequest", in)
	if calls.Load() != 1 || !strings.Contains(out, `"behavior":"deny"`) {
		t.Fatalf("local deny lost: calls=%d %s", calls.Load(), out)
	}
	in.ToolInput["file_path"] = filepath.Join(filepath.Dir(f.home), "outside.md")
	answer = map[string]any{"behavior": "allow", "updatedInput": map[string]any{"file_path": filepath.Join(f.dir, "task.md"), "content": "bad"}}
	out, _, _ = f.run(t, "PermissionRequest", in)
	if !strings.Contains(out, `"behavior":"deny"`) || strings.Contains(out, "updatedInput") {
		t.Fatalf("server bypassed guard: %s", out)
	}
}
func TestPermissionTimeoutAndInvalidResponseAllow(t *testing.T) {
	for _, mode := range []string{"timeout", "invalid", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			f := eventSetup(t)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "timeout":
					time.Sleep(25 * time.Millisecond)
				case "invalid":
					_, _ = w.Write([]byte("{bad"))
				case "redirect":
					http.Redirect(w, r, "https://example.com/permission", http.StatusTemporaryRedirect)
				}
			}))
			defer s.Close()
			f.env["GRID_URL"] = s.URL
			f.env["GRID_PERMISSION_TIMEOUT_MS"] = "5"
			out, log, code := f.run(t, "PermissionRequest", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(filepath.Dir(f.home), "outside.md")}})
			if code != 0 || out != "" || !strings.Contains(log, "grid hook:") {
				t.Fatalf("fallback: %d %s %s", code, out, log)
			}
		})
	}
}

func TestPermissionTimeoutDefaultOverrideAndCap(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want time.Duration
	}{
		{"", 85 * time.Second},
		{"junk", 85 * time.Second},
		{"-5", 85 * time.Second},
		{"0", 85 * time.Second},
		{"500", 500 * time.Millisecond},
		{"95000", 95 * time.Second},
		{"600000", 95 * time.Second},
	} {
		got := permissionTimeout(func(string) string { return tc.env })
		if got != tc.want {
			t.Errorf("GRID_PERMISSION_TIMEOUT_MS=%q: %v, want %v", tc.env, got, tc.want)
		}
	}
}

func TestPermissionSlowAnswerWithinWindowIsUsed(t *testing.T) {
	f := eventSetup(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"behavior": "allow"})
	}))
	defer s.Close()
	f.env["GRID_URL"] = s.URL
	out, log, code := f.run(t, "PermissionRequest", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(filepath.Dir(f.home), "outside.md")}})
	if code != 0 || log != "" || !strings.Contains(out, `"behavior":"allow"`) {
		t.Fatalf("slow answer lost: %d %s %s", code, out, log)
	}
}

func TestPermissionForwardCannotApprovePersonal(t *testing.T) {
	for _, level := range []string{"off", "warn", "block"} {
		t.Run(level, func(t *testing.T) {
			f := eventSetup(t)
			f.env["GRID_ROLE"], f.env["GRID_MEMBER"] = "guest", "gus"
			if err := os.WriteFile(filepath.Join(f.home, "settings.json"), []byte(`{"hooks":{"home-allow":"`+level+`"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			personal := filepath.Join(f.dir, "task.md")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_ = json.NewEncoder(w).Encode(map[string]any{"behavior": "allow", "updatedInput": map[string]any{"file_path": personal}})
			}))
			defer server.Close()
			f.env["GRID_URL"] = server.URL
			outside := filepath.Join(filepath.Dir(f.home), "outside.md")
			for _, path := range []string{personal, outside} {
				out, _, code := f.run(t, "PermissionRequest", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": path}})
				if code != 0 || !strings.Contains(out, `"behavior":"deny"`) || strings.Contains(out, "updatedPermissions") || strings.Contains(out, personal) {
					t.Fatalf("Personal approved or disclosed: %d %s", code, out)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("server calls = %d, want only the outside request", calls.Load())
			}
		})
	}
}
