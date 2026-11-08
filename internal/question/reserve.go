package question

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// ReserveGate shares the question-file allocator and its lock. A durable marker
// reserves the id before publishing to the Feed; a crash may leave a harmless
// gap, but neither writer can reuse the other's identity.
func (s Store) ReserveGate(minimum int, publish func(string) error) error {
	return s.locked(true, func(dir string) error {
		n, err := nextID(dir, minimum)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("Q-%d", n)
		if err := atomicfile.Write(filepath.Join(dir, id+".feed"), nil, 0600); err != nil {
			return err
		}
		return publish(id)
	})
}

func nextID(dir string, minimum int) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	next := max(1, minimum)
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext != ".md" && ext != ".feed" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ext)
		if !validID.MatchString(id) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(id, "Q-"))
		if err != nil || n == int(^uint(0)>>1) {
			return 0, errors.New("question id range exhausted")
		}
		next = max(next, n+1)
	}
	return next, nil
}
