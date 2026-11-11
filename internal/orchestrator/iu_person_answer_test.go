package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// iuPaAnswer writes one unleased answer to J-1 through a copy of the Feed store
// whose clock reads at, so the record's At is fixed and no test waits for time.
func iuPaAnswer(t *testing.T, f *fixture, w feed.Writer, text string, at time.Time) feed.Record {
	t.Helper()
	store := f.s.Feed
	store.Now = func() time.Time { return at }
	rec, err := AnswerPerson(store, w, "J-1", text, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// iuPaNotices ticks once, checks J-1 is accepted and returns the one message that
// says whose answer was taken and the one that says an answer was not used.
func iuPaNotices(t *testing.T, f *fixture) (taken, unused string) {
	t.Helper()
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s", got)
	}
	var takenAll, unusedAll []string
	for _, m := range messages(t, f) {
		switch {
		case strings.Contains(m, " answered J-1 at "):
			takenAll = append(takenAll, m)
		case strings.Contains(m, "was not used"):
			unusedAll = append(unusedAll, m)
		}
	}
	if len(takenAll) != 1 {
		t.Fatalf("want one message for the answer taken, got %q", takenAll)
	}
	if len(unusedAll) != 1 {
		t.Fatalf("want one message for the answer not used, got %q", unusedAll)
	}
	return takenAll[0], unusedAll[0]
}

// Two answers in the same second: the earlier millisecond wins, whatever order
// they were written in.
func TestIuPaSameSecondEarlierMillisecondWins(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	sam := feed.Writer{Author: feed.Author{ID: "sam", Role: "owner"}, Worker: "sam@" + machine}
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	late := iuPaAnswer(t, f, sam, "Call it Wren.", t0.Add(900*time.Millisecond))
	iuPaAnswer(t, f, personWriterOf("member"), "Call it Lark.", t0.Add(100*time.Millisecond))
	taken, unused := iuPaNotices(t, f)
	if !strings.HasPrefix(taken, "dana answered J-1 at ") || !strings.Contains(taken, "Call it Lark.") {
		t.Fatalf("want dana's answer taken: %s", taken)
	}
	for _, want := range []string{"The answer of sam to J-1", late.ID, "Call it Wren.", "dana answered first"} {
		if !strings.Contains(unused, want) {
			t.Fatalf("the not-used message lacks %q: %s", want, unused)
		}
	}
	if strings.Contains(unused, "Call it Lark.") {
		t.Fatalf("the not-used message names the answer that was used: %s", unused)
	}
}

// Two answers with the same At: the lower record ID wins. The IDs carry a random
// part, so the test reads which writer got the lower one and does not expect a
// fixed writer.
func TestIuPaSameTimeLowerRecordWins(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	sam := feed.Writer{Author: feed.Author{ID: "sam", Role: "owner"}, Worker: "sam@" + machine}
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	danaRec := iuPaAnswer(t, f, personWriterOf("member"), "Call it Lark.", t0)
	samRec := iuPaAnswer(t, f, sam, "Call it Wren.", t0)
	if danaRec.At != samRec.At {
		t.Fatalf("the answers have different times: %s and %s", danaRec.At, samRec.At)
	}
	if danaRec.ID == samRec.ID {
		t.Fatalf("both answers have record %s", danaRec.ID)
	}
	win, lose, winText, loseText := danaRec, samRec, "Call it Lark.", "Call it Wren."
	if samRec.ID < danaRec.ID {
		win, lose, winText, loseText = samRec, danaRec, "Call it Wren.", "Call it Lark."
	}
	taken, unused := iuPaNotices(t, f)
	if !strings.HasPrefix(taken, win.Author.ID+" answered J-1 at ") || !strings.Contains(taken, winText) {
		t.Fatalf("want %s's answer %s taken: %s", win.Author.ID, win.ID, taken)
	}
	for _, want := range []string{"The answer of " + lose.Author.ID + " to J-1", lose.ID, loseText, win.Author.ID + " answered first"} {
		if !strings.Contains(unused, want) {
			t.Fatalf("the not-used message lacks %q: %s", want, unused)
		}
	}
	if strings.Contains(unused, win.ID) || strings.Contains(unused, winText) {
		t.Fatalf("the not-used message names the answer that was used: %s", unused)
	}
}
