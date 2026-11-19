package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// A project or home proposal is not in the task folder, so the curator Job reads
// only the task ones by path and points at grid brain keep for the rest.
func TestCuratorBodyNamesProjectAndHomeProposalsByKeepPath(t *testing.T) {
	ids := []string{
		proposalPointer(brain.WaitingProposal{ID: "a", Path: "task:brain-proposals/a.md"}),
		proposalPointer(brain.WaitingProposal{ID: "b", Path: "brain:project/brain-proposals/b.md"}),
		proposalPointer(brain.WaitingProposal{ID: "c", Path: "brain:home/brain-proposals/c.md"}),
	}
	body := curatorJobBody("J-1", "T-01", ids)
	if got, want := usCloseReads(t, body), []string{"brain-proposals/a.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reads %v, want %v", got, want)
	}
	acc, _ := body["acceptance"].([]any)
	text, _ := acc[0].(map[string]any)["text"].(string)
	for _, want := range []string{"brain:project/brain-proposals/b.md", "brain:home/brain-proposals/c.md", "grid brain keep --task T-01"} {
		if !strings.Contains(text, want) {
			t.Errorf("acceptance lacks %q: %s", want, text)
		}
	}
}
