package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Migrate upgrades the installed legacy plan under the same lock used by Write
// and task approval. A matching plan or ship approval requires yes. Approval
// records remain untouched and their old hashes no longer approve the new plan.
func (s Store) Migrate(yes bool) (Written, string, error) {
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
		for _, name := range []string{"approval.json", "approval-ship.json"} {
			data, err := readRegular(filepath.Join(dir, name))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			// Read only the hash projection: task owns the receipt schema and
			// imports plan, so plan cannot depend on task.Event.
			var receipt map[string]json.RawMessage
			if err := json.Unmarshal(data, &receipt); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			var hash string
			if raw, ok := receipt["plan_hash"]; ok {
				if err := json.Unmarshal(raw, &hash); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
			if hash == Hash(current) && !yes {
				return errors.New("migration changes the approved plan; the plan will need approving again; use --yes to migrate")
			}
		}
		result, err = writeLocked(dir, converted, form == "C")
		return err
	})
	return result, form, err
}
