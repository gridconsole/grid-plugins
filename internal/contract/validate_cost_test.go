package contract

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func costRecord(t testing.TB) string {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/ack/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// withSeq is the fixture with a different seq, so that no two calls see the same bytes.
func withSeq(rec string, n int) []byte {
	return []byte(strings.Replace(rec, `"seq": 1,`, fmt.Sprintf(`"seq": %d,`, n+1), 1))
}

func BenchmarkValidateNewRecord(b *testing.B) {
	rec := costRecord(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Validate("ack", withSeq(rec, i)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSeenRecord(b *testing.B) {
	data := withSeq(costRecord(b), 0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Validate("ack", data); err != nil {
			b.Fatal(err)
		}
	}
}

// A Feed read validates every counted record of every open task every second:
// a schema pattern must be compiled once, not once per string it checks.
func TestValidateCompilesPatternsOnce(t *testing.T) {
	rec := costRecord(t)
	if err := Validate("ack", withSeq(rec, 0)); err != nil {
		t.Fatal(err)
	}
	n := 0
	allocs := testing.AllocsPerRun(50, func() { n++; _ = Validate("ack", withSeq(rec, n)) })
	t.Logf("allocs per new record: %.0f", allocs)
	if allocs > 800 {
		t.Fatalf("Validate allocates %.0f times for a new record; compiling patterns per call is the cost", allocs)
	}
}

// The fold validates the records the reader already validated, on every pass.
func TestValidateRemembersASeenRecord(t *testing.T) {
	data := withSeq(costRecord(t), 7)
	first := Validate("ack", data)
	allocs := testing.AllocsPerRun(50, func() { _ = Validate("ack", data) })
	t.Logf("allocs per seen record: %.0f", allocs)
	if allocs > 20 {
		t.Fatalf("Validate allocates %.0f times for a record it has seen", allocs)
	}
	if got := Validate("ack", data); fmt.Sprint(got) != fmt.Sprint(first) {
		t.Fatalf("seen record: %v, first answer %v", got, first)
	}
}

// A refusal is remembered with its message, and an answer for one schema is not
// given for another.
func TestValidateMemoKeepsRefusalsAndSchemasApart(t *testing.T) {
	good := withSeq(costRecord(t), 0)
	bad := []byte(strings.Replace(string(good), `"type": "ack"`, `"type": "nope"`, 1))
	e1, e2 := Validate("ack", bad), Validate("ack", bad)
	if e1 == nil || e2 == nil || e1.Error() != e2.Error() {
		t.Fatalf("refusal differs between calls: %v / %v", e1, e2)
	}
	if err := Validate("ack", good); err != nil {
		t.Fatal(err)
	}
	if err := Validate("blocked", good); err == nil {
		t.Fatal("an ack record passed as blocked after it passed as ack")
	}
}
