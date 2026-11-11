package version

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckCacheExpiryAndOffline(t *testing.T) {
	hits := 0
	fail := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if fail {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"version":"1.10.0","contract":"1","app_min":"0.2.0"}`)
	}))
	defer s.Close()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := CheckOptions{URL: s.URL, GridHome: t.TempDir(), Current: "1.9.0", Now: func() time.Time { return now }}
	r := Check(o)
	if r.Status != "update available" || r.Cached || hits != 1 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	now = now.Add(24*time.Hour - time.Second)
	r = Check(o)
	if !r.Cached || r.Stale || hits != 1 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	now = now.Add(time.Second)
	r = Check(o)
	if r.Cached || hits != 2 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	fail = true
	now = now.Add(24 * time.Hour)
	r = Check(o)
	if !r.Cached || !r.Stale || r.Release.Version != "1.10.0" || hits != 3 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
}

func TestCheckFailures(t *testing.T) {
	for _, body := range []string{`not json`, `null`, `{}`, `{"version":"banana"}`, `{"version":"1.0.0-01"}`, `{"version":"1.0.0"} {}`, strings.Repeat("x", (64<<10)+1)} {
		t.Run(fmt.Sprint(len(body), body[:min(len(body), 30)]), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer s.Close()
			if r := Check(CheckOptions{URL: s.URL, GridHome: t.TempDir(), Current: "1.0.0"}); r.Status != "unavailable" {
				t.Fatalf("%+v", r)
			}
		})
	}
	for _, address := range []string{"file:///manifest", "https://", "http://user:secret@example.com/manifest"} {
		if r := Check(CheckOptions{URL: address, GridHome: t.TempDir()}); r.Status != "unavailable" {
			t.Fatalf("%+v", r)
		}
	}
	if r := Check(CheckOptions{}); r.Status != "not configured" {
		t.Fatalf("%+v", r)
	}
}

func TestCheckTimeout(t *testing.T) {
	entered := make(chan struct{})
	gone := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(gone)
	}))
	defer s.Close()
	// The handler never answers, so Check returning is the proof the timeout ended the request.
	r := Check(CheckOptions{URL: s.URL, GridHome: t.TempDir(), Timeout: 20 * time.Millisecond})
	if r.Status != "unavailable" {
		t.Fatalf("%+v", r)
	}
	// Under load the timeout can fire before the request reaches the handler; then there is no request to wait for.
	select {
	case <-entered:
		select {
		case <-gone:
		case <-time.After(30 * time.Second):
			t.Fatal("request never ended")
		}
	default:
	}
}

func TestCheckCacheIsolationAndCorruption(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; fmt.Fprint(w, `{"version":"1.0.0"}`) }))
	defer s.Close()
	o := CheckOptions{URL: s.URL + "/one", GridHome: t.TempDir(), Current: "1.0.0"}
	Check(o)
	o.URL = s.URL + "/two"
	if r := Check(o); r.Cached || hits != 2 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	files, err := filepath.Glob(filepath.Join(o.GridHome, "cache", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("broken"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if r := Check(o); r.Cached || hits != 3 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	o.GridHome = filepath.Join(o.GridHome, "file")
	if err := os.WriteFile(o.GridHome, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if r := Check(o); r.Status != "up to date" {
		t.Fatalf("cache error changed result: %+v", r)
	}
}

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		want  int
		valid bool
	}{
		{"1.9.0", "1.10.0", -1, true}, {"v1.0.0+build", "1.0.0", 0, true},
		{"1.0.0-alpha.2", "1.0.0-alpha.10", -1, true}, {"1.0.0-alpha", "1.0.0", -1, true},
		{"1.0.0", "1.0.0-rc.1", 1, true}, {"1.0.0-2", "1.0.0-beta", -1, true},
		{"1.0.0-a", "1.0.0-a.1", -1, true}, {"2.0.0", "1.99.0", 1, true},
		{"dev", "1.0.0", 0, false}, {"01.0.0", "1.0.0", 0, false}, {"1.0.0-01", "1.0.0", 0, false},
	} {
		if got, ok := Compare(tc.a, tc.b); got != tc.want || ok != tc.valid {
			t.Errorf("Compare(%s,%s)=%d,%v", tc.a, tc.b, got, ok)
		}
	}
}

func TestAppWarningOncePerSession(t *testing.T) {
	home := t.TempDir()
	if w := AppWarning(home, "session-one", "0.0.1"); !strings.Contains(w, "Update the app.") {
		t.Fatal(w)
	}
	if w := AppWarning(home, "session-one", "0.0.1"); w != "" {
		t.Fatal(w)
	}
	if w := AppWarning(home, "session-two", "0.0.1"); w == "" {
		t.Fatal("new session lost warning")
	}
	if w := AppWarning(home, "session-three", AppMin); w != "" {
		t.Fatal(w)
	}
	if w := AppWarning(home, "session-four", ""); w != "" {
		t.Fatal(w)
	}
}

func TestCheckRefusesHTTPSToHTTPRedirect(t *testing.T) {
	plainHits := 0
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainHits++
		fmt.Fprint(w, `{"version":"9.0.0"}`)
	}))
	defer plain.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer secure.Close()
	saved := manifestClient.Transport
	manifestClient.Transport = secure.Client().Transport
	defer func() { manifestClient.Transport = saved }()
	r := Check(CheckOptions{URL: secure.URL, GridHome: t.TempDir(), Current: "1.0.0"})
	if r.Status != "unavailable" || plainHits != 0 {
		t.Fatalf("%+v plain hits=%d", r, plainHits)
	}
}
