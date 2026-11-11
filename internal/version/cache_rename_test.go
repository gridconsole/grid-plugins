package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cacheFiles(t *testing.T, home string) (cache string, names []string) {
	t.Helper()
	cache = filepath.Join(home, "cache")
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return cache, names
}

func noTempFiles(t *testing.T, names []string) {
	t.Helper()
	for _, n := range names {
		if strings.HasPrefix(n, ".version-") {
			t.Fatalf("temp file left behind: %s in %v", n, names)
		}
	}
}

func TestCacheReplacedOnSecondCheck(t *testing.T) {
	served := "1.1.0"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":%q}`, served)
	}))
	defer s.Close()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := CheckOptions{URL: s.URL, GridHome: t.TempDir(), Current: "1.0.0", Now: func() time.Time { return now }}

	read := func() cacheEntry {
		cache, names := cacheFiles(t, o.GridHome)
		noTempFiles(t, names)
		if len(names) != 1 {
			t.Fatalf("want one cache file, got %v", names)
		}
		b, err := os.ReadFile(filepath.Join(cache, names[0]))
		if err != nil {
			t.Fatal(err)
		}
		var e cacheEntry
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		return e
	}

	if r := Check(o); r.Status != "update available" || r.Cached {
		t.Fatalf("%+v", r)
	}
	if e := read(); e.Release.Version != "1.1.0" {
		t.Fatalf("first cache: %+v", e)
	}

	served = "1.2.0"
	now = now.Add(25 * time.Hour)
	if r := Check(o); r.Cached || r.Release == nil || r.Release.Version != "1.2.0" {
		t.Fatalf("%+v", r)
	}
	e := read()
	if e.Release.Version != "1.2.0" || !e.Checked.Equal(now) {
		t.Fatalf("second cache not replaced: %+v", e)
	}
}

func TestCacheWriteOverFolderKeepsResult(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"version":"1.0.0"}`)
	}))
	defer s.Close()
	home := t.TempDir()
	o := CheckOptions{URL: s.URL, GridHome: home, Current: "1.0.0"}

	// Find the cache path by running one Check against a scratch home.
	scratch := t.TempDir()
	Check(CheckOptions{URL: s.URL, GridHome: scratch, Current: "1.0.0"})
	_, names := cacheFiles(t, scratch)
	if len(names) != 1 {
		t.Fatalf("scratch cache: %v", names)
	}
	dir := filepath.Join(home, "cache", names[0])
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(inner, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := Check(o)
	if r.Status != "up to date" || r.Release == nil || r.Release.Version != "1.0.0" {
		t.Fatalf("%+v", r)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("folder changed: %v %v", fi, err)
	}
	if b, err := os.ReadFile(inner); err != nil || string(b) != "keep" {
		t.Fatalf("folder contents changed: %q %v", b, err)
	}
	_, after := cacheFiles(t, home)
	noTempFiles(t, after)
	if len(after) != 1 {
		t.Fatalf("cache folder holds %v", after)
	}
}
