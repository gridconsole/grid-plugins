package plan

import (
	"errors"
	"path/filepath"
)

// Migrate upgrades the installed legacy plan under the same lock used by Write
// and task approval. A plan that approved() names as approved requires yes. The
// approval stays untouched and its old hash no longer approves the new plan.
// approved returns the approved plan hash, or "" when there is none; nil means none.
func (s Store) Migrate(yes bool, approved func() string) (Written, string, error) {
	// Preserve the file-free no-op for current plans. The changing path reads
	// and converts again under the lock, never installing this first snapshot.
	text, err := s.Read()
	if err != nil {
		return Written{}, "", err
	}
	if _, err := Items(text); err == nil {
		return Written{}, "current", nil
	}
	dir, err := s.directory()
	if err != nil {
		return Written{}, "", err
	}
	var result Written
	var form string
	err = locked(dir, func() error {
		data, err := readRegular(filepath.Join(dir, "plan.md"))
		if err != nil {
			return err
		}
		current := string(data)
		converted, detected, err := Upgrade(current)
		form = detected
		if err != nil || form == "current" {
			return err
		}
		if !HasRecall(converted) {
			return errors.New("plan write requires Used: <note> or Recall: nothing found")
		}
		// The approved hash is the plan Job's result: task owns it and imports plan,
		// so the caller supplies it.
		if approved != nil && !yes {
			if hash := approved(); hash != "" && hash == Hash(current) {
				return errors.New("migration changes the approved plan; the plan will need approving again; use --yes to migrate")
			}
		}
		result, err = writeLocked(dir, converted, form == "C")
		return err
	})
	return result, form, err
}
