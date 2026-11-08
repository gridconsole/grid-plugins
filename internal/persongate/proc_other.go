//go:build !linux && !darwin && !windows

package persongate

import "errors"

func (SystemProcs) Snapshot() (map[int]Process, error) {
	return nil, errors.New("unsupported process snapshot")
}
