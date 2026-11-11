package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// objectKeys returns the top-level keys of one JSON object in the order printed.
func objectKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("stdout %q is not an object: %v", raw, err)
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	if _, err := dec.Token(); err != io.EOF {
		t.Fatalf("stdout %q holds more than one JSON value", raw)
	}
	return keys
}

func TestGlJobWaitingCode(t *testing.T) {
	h, path, store, text := waitingApproveHarness(t)
	code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json")
	if code != 3 {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out.String(), h.errb.String())
	}
	if got := strings.Join(objectKeys(t, h.out.Bytes()), ","); got != "error,code,job,waiting_for,reason" {
		t.Fatalf("keys %s, stdout %s", got, h.out.String())
	}
	var got struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		Job        string `json:"job"`
		WaitingFor string `json:"waiting_for"`
		Reason     string `json:"reason"`
	}
	dec := json.NewDecoder(bytes.NewReader(h.out.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("stdout %q: %v", h.out.String(), err)
	}
	if got.Error != "job_waiting" || got.Code != "job_waiting" || got.Job != "J-2" || got.WaitingFor != "J-1" || got.Reason != "waiting for J-1 to be accepted" {
		t.Fatalf("stdout %s", h.out.String())
	}
	if strings.TrimRight(h.out.String(), "\n") != fu2WaitingBody {
		t.Fatalf("stdout %q, want %s", h.out.String(), fu2WaitingBody)
	}
	requireNothingAnswered(t, path, store)
}
