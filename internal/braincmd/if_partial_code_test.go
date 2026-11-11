package braincmd

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// ifBeCode is the stable code of contract/README.md that a partial success
// carries: the change was written, rebuilding index.md failed.
const ifBeCode = "index_rebuild_failed"

// ifBeObject decodes data as one JSON object of strings and fails unless its
// keys are exactly want.
func ifBeObject(t *testing.T, name string, data []byte, want ...string) map[string]string {
	t.Helper()
	var obj map[string]string
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("%s %q: %v", name, data, err)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, want) {
		t.Fatalf("%s keys %v, want %v: %s", name, keys, want, data)
	}
	return obj
}

// A revert whose index rebuild fails has written the version: under --json both
// the stdout object and the stderr object carry the code, the message is the
// store's, and the exit is 1.
func TestIfBeRevertPartialCode(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	vlBreakIndex(t, h)
	h.out.Reset()
	h.err.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home", "--json"); code != 1 {
		t.Fatalf("exit %d, want 1: %q %q", code, &h.out, &h.err)
	}
	var out struct {
		Version *int   `json:"version"`
		Noop    *bool  `json:"noop"`
		Error   string `json:"error"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil || out.Version == nil || *out.Version != 4 || out.Noop == nil || *out.Noop {
		t.Fatalf("stdout %q (%v), want version 4 and noop false", &h.out, err)
	}
	if out.Code != ifBeCode {
		t.Fatalf("stdout code %q, want %q: %s", out.Code, ifBeCode, &h.out)
	}
	e := ifBeObject(t, "stderr", h.err.Bytes(), "code", "error")
	if e["code"] != ifBeCode {
		t.Fatalf("stderr code %q, want %q", e["code"], ifBeCode)
	}
	if !strings.HasPrefix(e["error"], "reverted to v1 as v4, but rebuilding the index failed: ") || e["error"] != out.Error {
		t.Fatalf("stderr error %q, stdout error %q", e["error"], out.Error)
	}

	// The human output keeps its plain line and carries no code.
	h = newHarness(t)
	h.threeVersions(t)
	vlBreakIndex(t, h)
	h.out.Reset()
	h.err.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 1 {
		t.Fatalf("text exit %d, want 1: %q %q", code, &h.out, &h.err)
	}
	if h.out.String() != "v4\n" || !strings.HasPrefix(h.err.String(), "brain: reverted to v1 as v4, but rebuilding the index failed: ") || strings.Contains(h.err.String(), ifBeCode) {
		t.Fatalf("text stdout %q stderr %q", &h.out, &h.err)
	}
}

// A supersede whose index rebuild fails has written both notes: stdout keeps the
// SupersedeResult unchanged, stderr carries the code beside the unchanged
// message, and the exit is 1.
func TestIfBeSupersedePartialCode(t *testing.T) {
	args := []string{"supersede", "brain:home/c1", "--by", "brain:home/c2", "--reason", svReason, "--level", "home"}
	h := newHarness(t)
	h.svNotes(t, "c1", "c2")
	vlBreakIndex(t, h)
	h.out.Reset()
	h.err.Reset()
	if code := h.run(append(args, "--json")...); code != 1 {
		t.Fatalf("exit %d, want 1: %q %q", code, &h.out, &h.err)
	}
	var res brain.SupersedeResult
	dec := json.NewDecoder(bytes.NewReader(h.out.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&res); err != nil || !res.Wrote || res.Old != "brain:home/c1" || res.New != "brain:home/c2" || res.Versions.Old != 2 || res.Versions.New != 2 {
		t.Fatalf("stdout %q (%v), want the SupersedeResult with wrote true", &h.out, err)
	}
	e := ifBeObject(t, "stderr", h.err.Bytes(), "code", "error")
	if e["code"] != ifBeCode {
		t.Fatalf("stderr code %q, want %q", e["code"], ifBeCode)
	}
	if !strings.HasPrefix(e["error"], "brain:home/c1 is superseded by brain:home/c2, but rebuilding the index failed: ") {
		t.Fatalf("stderr error %q", e["error"])
	}

	// The human output keeps its plain line and carries no code.
	h = newHarness(t)
	h.svNotes(t, "c1", "c2")
	vlBreakIndex(t, h)
	h.out.Reset()
	h.err.Reset()
	if code := h.run(args...); code != 1 {
		t.Fatalf("text exit %d, want 1: %q %q", code, &h.out, &h.err)
	}
	if h.out.String() != "brain:home/c1 superseded by brain:home/c2 (v2, v2)\n" || !strings.HasPrefix(h.err.String(), "brain: brain:home/c1 is superseded by") || strings.Contains(h.err.String(), ifBeCode) {
		t.Fatalf("text stdout %q stderr %q", &h.out, &h.err)
	}
}
