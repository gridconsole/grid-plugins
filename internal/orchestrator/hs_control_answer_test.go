package orchestrator

import (
	"sort"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func hsCaKeys(body map[string]any) []string {
	var keys []string
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The accepting control of a leased answer carries the answer text, trimmed,
// as the holder's pass for an unleased answer already does.
func TestHsCaLeasedControlHasAnswer(t *testing.T) {
	f := setup(t)
	f.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, f.s.Writer)
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-90", "  Call it Lark. ", time.Now()); err != nil {
		t.Fatal(err)
	}
	body := acceptingControl(t, f, "J-90")
	if body == nil {
		t.Fatal("no accepting control")
	}
	if body["answer"] != "Call it Lark." {
		t.Errorf("control answer = %v, want %q", body["answer"], "Call it Lark.")
	}
	if want := "answered by " + f.s.Writer.Author.ID; body["reason"] != want {
		t.Errorf("control reason = %v, want %q", body["reason"], want)
	}
}

// The leased path and the holder's pass for an unleased answer write the same
// keys into the accepting control.
func TestHsCaBothPathsSameKeys(t *testing.T) {
	leased := setup(t)
	leased.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, leased.s.Writer)
	if _, err := AnswerPerson(leased.s.Feed, leased.s.Writer, "J-90", "Call it Lark.", time.Now()); err != nil {
		t.Fatal(err)
	}
	a := acceptingControl(t, leased, "J-90")
	if a == nil {
		t.Fatal("no leased accepting control")
	}

	unleased := setup(t)
	unleased.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, unleased.s.Writer)
	dana := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + machine}
	if _, err := AnswerPerson(unleased.s.Feed, dana, "J-90", "Call it Lark.", time.Now()); err != nil {
		t.Fatal(err)
	}
	unleased.tick(t)
	b := acceptingControl(t, unleased, "J-90")
	if b == nil {
		t.Fatal("the holder's pass did not accept the Job")
	}
	ka, kb := hsCaKeys(a), hsCaKeys(b)
	if len(ka) != len(kb) {
		t.Fatalf("leased keys %v, unleased keys %v", ka, kb)
	}
	for i := range ka {
		if ka[i] != kb[i] {
			t.Fatalf("leased keys %v, unleased keys %v", ka, kb)
		}
	}
	if a["answer"] != b["answer"] {
		t.Errorf("answers differ: %v and %v", a["answer"], b["answer"])
	}
}

// A field named "answer" does not replace the answer argument.
func TestHsCaFieldsDoNotOverwrite(t *testing.T) {
	f := setup(t)
	f.post(t, feed.Input{Type: "job", Job: "J-90", Body: approvefix.JobBody("J-90")}, f.s.Writer)
	fields := map[string]any{"note": "n", "answer": "other"}
	if _, err := AnswerPersonWith(f.s.Feed, f.s.Writer, "J-90", "Call it Lark.", fields, time.Now()); err != nil {
		t.Fatal(err)
	}
	body := acceptingControl(t, f, "J-90")
	if body == nil {
		t.Fatal("no accepting control")
	}
	if body["answer"] != "Call it Lark." {
		t.Errorf("control answer = %v, want the answer argument", body["answer"])
	}
	if body["note"] != "n" {
		t.Errorf("control note = %v, want n", body["note"])
	}
}
