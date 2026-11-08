package cli

import (
	"encoding/json"
	"testing"
)

func TestBrainCommandRegistered(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "brain", "index", "--level", "home", "--read-only", "--json"); code != 0 {
		t.Fatalf("brain command exit %d: %s", code, h.errb)
	}
	if !json.Valid(h.out.Bytes()) {
		t.Fatalf("invalid brain JSON: %s", h.out)
	}
}
