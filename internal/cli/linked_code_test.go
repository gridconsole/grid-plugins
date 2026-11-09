package cli

import (
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestLinkedFileHasAStableCode(t *testing.T) {
	if got := errorCode(fmt.Errorf("grid task show: %w", task.ErrLinkedFile)); got != "linked_file" {
		t.Fatalf("code = %q, want linked_file", got)
	}
}
