package brain

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// Concurrent O_CREATE opens of a not-yet-existing file through os.Root can
// spuriously fail with ENOENT on Darwin; locked() must never surface that.
func TestLockCreateRaceNeverFails(t *testing.T) {
	const writers, rounds = 64, 4
	for round := 0; round < rounds; round++ {
		dir := t.TempDir() // fresh folder, no lock file yet
		start := make(chan struct{})
		var inside, failed atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				root, err := os.OpenRoot(dir)
				if err != nil {
					t.Errorf("open root: %v", err)
					return
				}
				defer root.Close()
				<-start
				err = locked(root, func() error {
					if inside.Add(1) != 1 {
						failed.Add(1)
					}
					inside.Add(-1)
					return nil
				})
				if err != nil {
					t.Errorf("locked on a fresh folder: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
		if n := failed.Load(); n != 0 {
			t.Fatalf("%d writers held the lock together", n)
		}
	}
}
