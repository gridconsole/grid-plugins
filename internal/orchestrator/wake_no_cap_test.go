package orchestrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func wakeSnap() Snapshot {
	return Snapshot{State: contract.State{Jobs: map[string]*contract.Job{}, Wakes: []string{}}, Records: map[string]feed.Record{}}
}

// ack records the Judge's answer to a wake, as a post carrying its key.
func wakeAck(snap Snapshot, out *Wake) {
	key := fmt.Sprintf("%s/%d", out.Event, out.N)
	snap.Records["ack-"+out.Event] = feed.Record{Wake: &key}
}

// T-11 on 18 Nov: twenty wakes went out in nine minutes, then three results came in (J-15, J-5.1,
// J-12.1). The Judge acts only on a wake, so no review started and every Job behind them waited for
// the first stamp to leave the hour. The shape below is that task's .orchestrator.json.
func TestWakeIsNotHeldWhenTwentyWentOutThisHour(t *testing.T) {
	f := setup(t)
	snap := wakeSnap()
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}, WakeVersion: 1, Overflow: true}
	for i := 0; i < 20; i++ {
		m.Wakes = append(m.Wakes, f.now.Add(-time.Duration(20-i)*30*time.Second))
	}
	for i, job := range []string{"J-15", "J-5.1", "J-12.1"} {
		id := fmt.Sprintf("event-%d", i)
		snap.State.Wakes = append(snap.State.Wakes, id)
		snap.Records[id] = feed.Record{Body: map[string]any{"job": job}}
	}
	for i, job := range []string{"J-15", "J-5.1", "J-12.1"} {
		out := f.s.wake(snap, &m)
		if out == nil || out.Job != job || out.Event != fmt.Sprintf("event-%d", i) {
			t.Fatalf("wake %d held back or out of order: %+v", i, out)
		}
		if len(out.Events) != 0 {
			t.Fatal("a wake is delivered by itself, not as a summary", out.Events)
		}
		wakeAck(snap, out)
	}
	if out := f.s.wake(snap, &m); out != nil || len(m.Pending) != 0 {
		t.Fatal("wakes left over after all three were answered", out, m.Pending)
	}
}

// Every wake goes out, one at a time, however many came this hour: nothing is held back, summarized or dropped.
func TestWakeDeliversEveryEventInOneHour(t *testing.T) {
	f := setup(t)
	snap := wakeSnap()
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}}
	for n := 0; n < 25; n++ {
		id := fmt.Sprintf("event-%02d", n)
		snap.State.Wakes = append(snap.State.Wakes, id)
		snap.Records[id] = feed.Record{Body: map[string]any{"job": "J-1"}}
		out := f.s.wake(snap, &m)
		if out == nil || out.Event != id || len(out.Events) != 0 {
			t.Fatalf("wake %d not delivered on its own: %+v", n, out)
		}
		wakeAck(snap, out)
	}
	if out := f.s.wake(snap, &m); out != nil || len(m.Pending) != 0 {
		t.Fatal("wakes left over", out, m.Pending)
	}
}
