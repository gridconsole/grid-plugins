package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// heJpRefused runs args and asserts the unknown-sub answer: exit 2 and the message
// on stderr. With --json the same refusal is also the invalid_argument envelope on
// stdout; without it stdout stays empty.
func heJpRefused(t *testing.T, h *harness, cwd, message string, args ...string) {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	code := h.run(cwd, args...)
	asJSON := false
	for _, a := range args {
		asJSON = asJSON || a == "--json"
	}
	envelope := strings.Contains(h.out.String(), `"code":"invalid_argument"`) && strings.Contains(h.out.String(), "unknown")
	if code != 2 || !strings.Contains(h.errb.String(), message) || envelope != asJSON || (!asJSON && h.out.Len() != 0) {
		t.Fatalf("%v: exit %d, want 2 with %q\nstdout: %s\nstderr: %s", args, code, message, h.out, h.errb)
	}
}

// TestHeJpRemovedSubs holds the removal of `grid job order` and `grid person list`
// (docs/needed.md:41): both are unknown subs; the order control record posted with
// `job post --type control` still reorders the queue, and `person show` still answers.
func TestHeJpRemovedSubs(t *testing.T) {
	t.Run("job order is an unknown job command", func(t *testing.T) {
		h, path, _ := orchestratorHarness(t)
		t.Setenv("GRID_ROLE", "member")
		const message = `job: unknown job command "order"; use render, lint or show`
		heJpRefused(t, h, path, message, "job", "order", "J-1", "3")
		heJpRefused(t, h, path, message, "job", "order")
		// With --json grid job answers its own {"ok":false,"error":...} shape on stdout.
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(path, "job", "order", "J-1", "3", "--json"); code != 2 || !strings.Contains(h.out.String(), `"error":"unknown job command \"order\"; use render, lint or show"`) {
			t.Fatalf("job order --json: exit %d\nstdout: %s\nstderr: %s", code, h.out, h.errb)
		}
	})

	t.Run("person list is an unknown command", func(t *testing.T) {
		h := newHarness(t)
		const message = `person: unknown command "list"`
		heJpRefused(t, h, h.home, message, "person", "list")
		heJpRefused(t, h, h.home, message, "person", "list", "--json")
		if code := h.run(h.home, "person", "--help"); code != 0 || strings.Contains(h.out.String(), "person list") {
			t.Fatalf("person --help: exit %d: %s", code, h.out)
		}
	})

	t.Run("the order control record still reorders the queue", func(t *testing.T) {
		h, path, store := orchestratorHarness(t)
		t.Setenv("GRID_ROLE", "member")
		// J-1 has no order; J-2 and J-3 take one through the --order flag of job post, which stays.
		for _, posted := range [][2]string{{"J-2", "5"}, {"J-3", "6"}} {
			id, order := posted[0], posted[1]
			if code := h.run(path, "job", "post", id, "--type", "job", "--body", jobBodyWith(t, id, nil), "--order", order); code != 0 {
				t.Fatalf("post %s: exit %d: %s", id, code, h.errb)
			}
		}
		// The control record moves J-3 ahead of J-2 (5) and of the Job without an order.
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(path, "job", "post", "J-3", "--type", "control", "--body", `{"target":"J-3","order":0}`, "--json"); code != 0 {
			t.Fatalf("control post: exit %d: %s", code, h.errb)
		}
		state, err := store.State()
		if err != nil {
			t.Fatal(err)
		}
		if o := state.Jobs["J-3"].Order; o == nil || *o != 0 {
			t.Fatalf("J-3 order after the control record: %v", o)
		}
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"id":"J-3"`) {
			t.Fatalf("job next should offer J-3 first: exit %d: %s %s", code, h.out, h.errb)
		}
	})

	t.Run("person show still answers", func(t *testing.T) {
		h := newHarness(t)
		h.write(filepath.Join(h.gridHome, "knowledge", "people", "gus.md"), "---\nhandle: gus\nname: Gus Example\nrole: owner\n---\n")
		if code := h.run(h.home, "person", "show", "gus"); code != 0 || !strings.Contains(h.out.String(), "gus  Gus Example  owner") {
			t.Fatalf("person show: exit %d: %s %s", code, h.out, h.errb)
		}
	})
}
