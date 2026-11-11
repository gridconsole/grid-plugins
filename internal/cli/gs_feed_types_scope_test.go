package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

type gsTypesResult struct {
	code        int
	out, errout string
}

func gsTypes(t *testing.T, h *harness, args ...string) gsTypesResult {
	t.Helper()
	code := h.run(h.home, append([]string{"feed"}, args...)...)
	return gsTypesResult{code, h.out.String(), h.errb.String()}
}

func gsErrCode(t *testing.T, out string) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not an error envelope: %q: %v", out, err)
	}
	return env.Error.Code
}

func TestGsTypes(t *testing.T) {
	t.Run("no scope prints the list", func(t *testing.T) {
		h, _ := feedHarness(t)
		r := gsTypes(t, h, "types", "--json")
		var want bytes.Buffer
		writeJSON(&want, struct {
			Types []contract.PostType `json:"types"`
		}{contract.PostTypes()})
		if r.code != 0 || r.out != want.String() {
			t.Fatalf("code %d out %q want %q err %q", r.code, r.out, want.String(), r.errout)
		}
	})
	t.Run("unknown task", func(t *testing.T) {
		h, _ := feedHarness(t)
		r := gsTypes(t, h, "types", "--task", "T-99", "--json")
		h2, _ := feedHarness(t)
		read := gsTypes(t, h2, "read", "--task", "T-99", "--json")
		if r.code == 0 || r.code != read.code {
			t.Fatalf("types exit %d, read exit %d", r.code, read.code)
		}
		if got := gsErrCode(t, r.out); got != "no_such_task" {
			t.Fatalf("code %q out %q", got, r.out)
		}
	})
	t.Run("unknown project", func(t *testing.T) {
		h, _ := feedHarness(t)
		r := gsTypes(t, h, "types", "--project", "nope", "--json")
		h2, _ := feedHarness(t)
		read := gsTypes(t, h2, "read", "--project", "nope", "--json")
		if r.code == 0 || r.code != read.code {
			t.Fatalf("types exit %d, read exit %d", r.code, read.code)
		}
		if got := gsErrCode(t, r.out); got != "no_such_project" {
			t.Fatalf("code %q out %q", got, r.out)
		}
	})
	t.Run("shorthand unknown task", func(t *testing.T) {
		h, _ := feedHarness(t)
		r := gsTypes(t, h, "types", "T-99")
		if r.code == 0 || r.out != "" || !strings.HasPrefix(r.errout, "grid feed: ") {
			t.Fatalf("code %d out %q err %q", r.code, r.out, r.errout)
		}
	})
	t.Run("existing task prints the list", func(t *testing.T) {
		h, task := feedHarness(t)
		r := gsTypes(t, h, "types", "--task", task, "--json")
		if r.code != 0 || !strings.HasPrefix(r.out, `{"types":[`) {
			t.Fatalf("code %d out %q err %q", r.code, r.out, r.errout)
		}
	})
}
