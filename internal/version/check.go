package version

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// Manifest describes one published release. Version is required.
type Manifest struct {
	Version  string `json:"version"`
	Contract string `json:"contract,omitempty"`
	AppMin   string `json:"app_min,omitempty"`
}

// CheckOptions configures a bounded, optional release lookup.
type CheckOptions struct {
	URL      string
	GridHome string
	Current  string
	Timeout  time.Duration
	Now      func() time.Time
}

// CheckResult is advisory: unavailable checks never stop the CLI.
type CheckResult struct {
	Status  string    `json:"status"`
	Release *Manifest `json:"release,omitempty"`
	Cached  bool      `json:"cached"`
	Stale   bool      `json:"stale"`
}

type cacheEntry struct {
	URL     string    `json:"url"`
	Checked time.Time `json:"checked"`
	Release Manifest  `json:"release"`
}

// Check uses a successful result for 24 hours, and retains it when offline.
// Cache failures are harmless and never prevent reporting a fetched release.
func Check(o CheckOptions) CheckResult {
	unavailable := CheckResult{Status: "unavailable"}
	if o.URL == "" {
		return CheckResult{Status: "not configured"}
	}
	u, err := url.Parse(o.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return unavailable
	}
	now := time.Now()
	if o.Now != nil {
		now = o.Now()
	}
	cachePath := ""
	if o.GridHome != "" {
		cachePath = filepath.Join(o.GridHome, "cache", fmt.Sprintf("version-%x.json", sha256.Sum256([]byte(o.URL))))
	}
	var cached cacheEntry
	b, err := os.ReadFile(cachePath)
	hasCache := err == nil && json.Unmarshal(b, &cached) == nil && cached.URL == o.URL && validManifest(cached.Release) && !cached.Checked.IsZero() && !cached.Checked.After(now)
	if hasCache && now.Sub(cached.Checked) < 24*time.Hour {
		return result(o.Current, cached.Release, true, false)
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	release, err := fetch(ctx, o.URL)
	if err != nil {
		if hasCache {
			return result(o.Current, cached.Release, true, true)
		}
		return unavailable
	}
	if cachePath != "" {
		saveCache(cachePath, cacheEntry{URL: o.URL, Checked: now, Release: release})
	}
	return result(o.Current, release, false, false)
}

// manifestClient follows redirects except a step from https down to http.
var manifestClient = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return errors.New("refusing a redirect from https to http")
	}
	return nil
}}

func fetch(ctx context.Context, address string) (Manifest, error) {
	var m Manifest
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return m, err
	}
	resp, err := manifestClient.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("manifest status %d", resp.StatusCode)
	}
	const limit = 64 << 10
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return m, err
	}
	if len(b) > limit {
		return m, fmt.Errorf("manifest too large")
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if !validManifest(m) {
		return m, fmt.Errorf("invalid release version")
	}
	return m, nil
}

func validManifest(m Manifest) bool {
	_, valid := Compare(m.Version, m.Version)
	if m.AppMin != "" {
		_, appValid := Compare(m.AppMin, m.AppMin)
		valid = valid && appValid
	}
	return valid
}

func result(current string, m Manifest, cached, stale bool) CheckResult {
	status := "unknown current version"
	if cmp, ok := Compare(current, m.Version); ok {
		status = "up to date"
		if cmp < 0 {
			status = "update available"
		}
	}
	return CheckResult{Status: status, Release: &m, Cached: cached, Stale: stale}
}

func saveCache(path string, entry cacheEntry) {
	b, err := json.Marshal(entry)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".version-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(b)
	closeErr := f.Close()
	if writeErr == nil && closeErr == nil {
		_ = atomicfile.Rename(f.Name(), path)
	}
}

var semver = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// Compare compares semantic versions, including prereleases and an optional v.
// Build metadata does not affect precedence; invalid versions are not compared.
func Compare(a, b string) (int, bool) {
	x, y := semver.FindStringSubmatch(a), semver.FindStringSubmatch(b)
	if x == nil || y == nil {
		return 0, false
	}
	for _, v := range []string{x[4], y[4]} {
		for _, id := range strings.Split(v, ".") {
			if numeric(id) && len(id) > 1 && id[0] == '0' {
				return 0, false
			}
		}
	}
	for i := 1; i <= 3; i++ {
		if c := numberCompare(x[i], y[i]); c != 0 {
			return c, true
		}
	}
	if x[4] == y[4] {
		return 0, true
	}
	if x[4] == "" {
		return 1, true
	}
	if y[4] == "" {
		return -1, true
	}
	xp, yp := strings.Split(x[4], "."), strings.Split(y[4], ".")
	for i := 0; i < len(xp) && i < len(yp); i++ {
		a, b := xp[i], yp[i]
		c := strings.Compare(a, b)
		switch {
		case numeric(a) && numeric(b):
			c = numberCompare(a, b)
		case numeric(a):
			c = -1
		case numeric(b):
			c = 1
		}
		if c != 0 {
			return c, true
		}
	}
	if len(xp) < len(yp) {
		return -1, true
	}
	return 1, true
}

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func numberCompare(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
