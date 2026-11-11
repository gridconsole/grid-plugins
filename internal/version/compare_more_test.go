package version

import (
	"context"
	"net/http"
	"testing"
)

func compareMoreRequest(t *testing.T, address string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestCompareMoreNumbersByLength(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"10.0.0", "9.0.0", 1},
		{"9.0.0", "10.0.0", -1},
	}
	for _, c := range cases {
		if got, ok := Compare(c.a, c.b); !ok || got != c.want {
			t.Errorf("Compare(%q, %q) = %d, %v; want %d, true", c.a, c.b, got, ok, c.want)
		}
	}
}

func TestCompareMorePrereleaseBranches(t *testing.T) {
	cases := []struct {
		name, a, b string
		want       int
	}{
		{"numeric id on the right sorts lower", "1.0.0-alpha", "1.0.0-1", 1},
		{"left prerelease has more ids", "1.0.0-a.b", "1.0.0-a", 1},
	}
	for _, c := range cases {
		if got, ok := Compare(c.a, c.b); !ok || got != c.want {
			t.Errorf("%s: Compare(%q, %q) = %d, %v; want %d, true", c.name, c.a, c.b, got, ok, c.want)
		}
	}
}

func TestManifestClientRedirectTooMany(t *testing.T) {
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = compareMoreRequest(t, "http://a/")
	}
	err := manifestClient.CheckRedirect(compareMoreRequest(t, "http://b/"), via)
	if err == nil || err.Error() != "too many redirects" {
		t.Fatalf("CheckRedirect with 10 earlier requests = %v, want too many redirects", err)
	}
}

func TestManifestClientRedirectHTTPToHTTP(t *testing.T) {
	via := []*http.Request{compareMoreRequest(t, "http://a/")}
	if err := manifestClient.CheckRedirect(compareMoreRequest(t, "http://b/"), via); err != nil {
		t.Fatalf("CheckRedirect http to http = %v, want nil", err)
	}
}

func TestManifestClientRedirectHTTPSToHTTPContrast(t *testing.T) {
	via := []*http.Request{compareMoreRequest(t, "https://a/")}
	err := manifestClient.CheckRedirect(compareMoreRequest(t, "http://b/"), via)
	if err == nil || err.Error() != "refusing a redirect from https to http" {
		t.Fatalf("CheckRedirect https to http = %v, want the refusal", err)
	}
}

func TestFetchRefusesUnparseableAddress(t *testing.T) {
	// NewRequest rejects the address, so no request is sent.
	if _, err := fetch(context.Background(), "http://[::1"); err == nil {
		t.Fatal("fetch of an unparseable address returned no error")
	}
}
