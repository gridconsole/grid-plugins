package routine

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// LimitPolicy retains YAML types and distinguishes an omitted key from null.
// Only explicit overrides are written back to the routine's front matter.
type LimitPolicy struct {
	RoutineTimeout  yaml.Node `json:"-" yaml:"routine_timeout,omitempty"`
	RoutineMaxTurns yaml.Node `json:"-" yaml:"routine_max_turns,omitempty"`
	RoutineBudget   yaml.Node `json:"-" yaml:"routine_budget,omitempty"`
}

// Limits is the effective policy reported for each routine.
type Limits struct {
	Timeout  string  `json:"routine_timeout"`
	MaxTurns int     `json:"routine_max_turns"`
	Budget   float64 `json:"routine_budget"`
}

// LimitError never includes the supplied value, which may contain private data.
type LimitError struct{ Key, Routine string }

func (e *LimitError) Error() string {
	if e.Routine != "" {
		return "routine " + e.Routine + ": invalid " + e.Key
	}
	return "invalid " + e.Key
}

func (p LimitPolicy) explicit() bool {
	return p.RoutineTimeout.Kind != 0 || p.RoutineMaxTurns.Kind != 0 || p.RoutineBudget.Kind != 0
}

// EffectiveLimits overlays entry keys on the workflow snapshot. The snapshot
// stays separate so saving run state cannot turn inherited defaults into keys.
func (e Entry) EffectiveLimits() (Limits, error) {
	p := e.workflowPolicy
	for _, d := range limitDefinitions {
		if n := *d.field(&e.LimitPolicy); n.Kind != 0 {
			*d.field(&p) = n
		}
	}
	return p.EffectiveLimits()
}

func (l Limits) fingerprint() string {
	duration, _ := time.ParseDuration(l.Timeout) // validated by EffectiveLimits
	return strconv.FormatInt(int64(duration), 10) + "\x00" + strconv.Itoa(l.MaxTurns) + "\x00" + strconv.FormatFloat(l.Budget, 'f', -1, 64)
}

var decimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)
var integer = regexp.MustCompile(`^[+-]?[0-9]+$`)

type limitDefinition struct {
	key, fallback, bounds string
	field                 func(*LimitPolicy) *yaml.Node
	apply                 func(*Limits, yaml.Node) bool
}

// The defaults and permitted ranges live in one table for validation and help.
var limitDefinitions = []limitDefinition{
	{"routine_timeout", "10m", "Go duration from 1m to 2h", func(p *LimitPolicy) *yaml.Node { return &p.RoutineTimeout }, func(l *Limits, n yaml.Node) bool {
		d, err := time.ParseDuration(n.Value)
		if n.Tag != "!!str" || err != nil || d < time.Minute || d > 2*time.Hour {
			return false
		}
		l.Timeout = n.Value
		return true
	}},
	{"routine_max_turns", "10", "integer from 1 to 100", func(p *LimitPolicy) *yaml.Node { return &p.RoutineMaxTurns }, func(l *Limits, n yaml.Node) bool {
		v, err := strconv.Atoi(n.Value)
		if n.Tag != "!!int" || !integer.MatchString(n.Value) || err != nil || v < 1 || v > 100 {
			return false
		}
		l.MaxTurns = v
		return true
	}},
	{"routine_budget", "1", "decimal greater than 0 and at most 50", func(p *LimitPolicy) *yaml.Node { return &p.RoutineBudget }, func(l *Limits, n yaml.Node) bool {
		v, err := strconv.ParseFloat(n.Value, 64)
		if (n.Tag != "!!int" && n.Tag != "!!float") || !decimal.MatchString(n.Value) || err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 50 {
			return false
		}
		// Check the cap before float rounding can hide a value just above it.
		exactText := n.Value
		if !strings.Contains(exactText, ".") {
			exactText += ".0"
		}
		amount, exact := new(big.Rat).SetString(exactText)
		if !exact || amount.Sign() <= 0 || amount.Cmp(big.NewRat(50, 1)) > 0 {
			return false
		}
		l.Budget = v
		return true
	}},
}

// SetText accepts CLI values, using the same type and range checks as YAML.
func (p *LimitPolicy) SetText(key, value string) error {
	for _, d := range limitDefinitions {
		if d.key != key {
			continue
		}
		var n yaml.Node
		if err := yaml.Load([]byte(value), &n, yaml.WithSingleDocument()); err != nil || len(n.Content) != 1 {
			return &LimitError{Key: key}
		}
		n = *n.Content[0]
		var limits Limits
		if n.Kind != yaml.ScalarNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 || !d.apply(&limits, n) {
			return &LimitError{Key: key}
		}
		*d.field(p) = n
		return nil
	}
	return &LimitError{Key: key}
}

func (p LimitPolicy) EffectiveLimits() (Limits, error) {
	var limits Limits
	for _, d := range limitDefinitions {
		n := *d.field(&p)
		if n.Kind == 0 {
			var doc yaml.Node
			// Table defaults are constants with the same validation as overrides.
			if err := yaml.Load([]byte(d.fallback), &doc); err != nil {
				return limits, &LimitError{Key: d.key}
			}
			n = *doc.Content[0]
		}
		if n.Kind != yaml.ScalarNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 || !d.apply(&limits, n) {
			return limits, &LimitError{Key: d.key}
		}
	}
	return limits, nil
}

func LimitsHelp() string {
	var parts []string
	for _, d := range limitDefinitions {
		parts = append(parts, fmt.Sprintf("%s (default %s; %s)", d.key, d.fallback, d.bounds))
	}
	return "Routine front matter policy: " + strings.Join(parts, "; ") + ". Each default may be changed by the owner."
}
