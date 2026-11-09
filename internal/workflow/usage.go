package workflow

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// UsageNoticeThresholds reads task-local usage_notice_thresholds overrides.
// Missing files or keys return nil. Invalid units are omitted from the returned
// map, with errors joined so callers can retain valid units and report once.
func UsageNoticeThresholds(taskDir string) (map[string][]float64, error) {
	raw, err := readTaskPolicy(taskDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		// Preserve the cause without exposing the task's filesystem path.
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return nil, fmt.Errorf("usage_notice_thresholds: cannot read workflow.md: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, errors.New("usage_notice_thresholds: workflow.md needs YAML front matter")
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return nil, errors.New("usage_notice_thresholds: workflow.md front matter is not closed")
	}
	var front map[string]any
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
		return nil, fmt.Errorf("usage_notice_thresholds: workflow.md front matter: %w", err)
	}
	value, exists := front["usage_notice_thresholds"]
	if !exists {
		return nil, nil
	}
	overrides, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("usage_notice_thresholds must be a map of units to number lists")
	}
	units := make([]string, 0, len(overrides))
	for unit := range overrides {
		units = append(units, unit)
	}
	slices.Sort(units)
	out := make(map[string][]float64, len(units))
	var errs []error
	for _, unit := range units {
		values, err := usageThresholdList(unit, overrides[unit])
		if err != nil {
			errs = append(errs, fmt.Errorf("usage_notice_thresholds.%s: %w", unit, err))
			continue
		}
		out[unit] = values
	}
	return out, errors.Join(errs...)
}

func usageThresholdList(unit string, value any) ([]float64, error) {
	if unit != "tokens" && unit != "dollars" && unit != "minutes" {
		return nil, errors.New("unknown unit")
	}
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("must be a nonempty list of numbers")
	}
	values := make([]float64, 0, len(list))
	var previous float64
	for _, item := range list {
		var n float64
		switch v := item.(type) {
		case int:
			n = float64(v)
		case int64:
			n = float64(v)
		case uint64:
			n = float64(v)
		case float64:
			n = v
		default:
			return nil, errors.New("must be a list of numbers")
		}
		if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
			return nil, errors.New("values must be positive finite numbers")
		}
		if n <= previous {
			return nil, errors.New("values must be strictly rising")
		}
		values = append(values, n)
		previous = n
	}
	if previous != 100 {
		return nil, errors.New("last value must be 100")
	}
	return values, nil
}
