package task

import (
	"errors"
	"strings"
	"testing"
)

// The store's refusal for a role that may not delete or restore is MoveRefusal,
// the text the CLI's early guest refusal prints.
func TestTrashRefusalIsMoveRefusal(t *testing.T) {
	for _, restore := range []bool{false, true} {
		err := TrashOptions{Role: "guest"}.validate(restore)
		if !errors.Is(err, ErrEditRejected) || !strings.HasSuffix(err.Error(), MoveRefusal) {
			t.Fatalf("restore=%v: %v", restore, err)
		}
	}
}
