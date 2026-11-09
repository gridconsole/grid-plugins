package usage

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
)

// HoldSettings are the named policy keys behind the hold, kept in one place
// until a workflow file can set them.
type HoldSettings struct {
	// AtPercent is the key usage_hold_at_percent: hold when spent reaches this
	// share of the budget in any unit that has a budget. Zero, negative or
	// non-finite values mean the default, 100.
	AtPercent float64
	// Tools is the key usage_hold_tools: the tool calls that start an agent.
	Tools []string
}

// DefaultHoldSettings hold at the budget and cover the tools that start an agent.
var DefaultHoldSettings = HoldSettings{AtPercent: 100, Tools: []string{"Agent", "Task"}}

// HoldBudget is the task's effective budget per unit; a nil unit has no budget.
type HoldBudget struct {
	Dollars, Minutes *float64
	Tokens           *uint64
}

func (b HoldBudget) none() bool { return b.Dollars == nil && b.Minutes == nil && b.Tokens == nil }

// HoldSummary is what a HoldPolicy judges: what the task spent, as `grid usage
// show` folds it, and the budget per unit. Summary has no budget of its own.
type HoldSummary struct {
	Summary
	Budget HoldBudget
}

// HoldPolicy is the one seam of the hold: it judges a task that has a budget
// and returns whether to hold, with the reason shown to the session.
type HoldPolicy func(HoldSummary) (hold bool, reason string)

// DefaultHoldPolicy judges with DefaultHoldSettings.
var DefaultHoldPolicy HoldPolicy = DefaultHoldSettings.Judge

// HoldDecision is the answer to Hold. Reason is set only when Hold is true.
type HoldDecision struct {
	Hold   bool
	Reason string
}

// HoldsTool reports whether a call of the tool starts an agent, so the hold applies to it.
func HoldsTool(tool string) bool { return slices.Contains(DefaultHoldSettings.Tools, tool) }

// Hold judges the task's spend against its budget. A task with no budget never
// holds. When the usage data or the budget cannot be read the answer is no
// hold and the error: the budget is a spending convenience, not a security gate.
func (s Store) Hold() (HoldDecision, error) {
	c, err := s.Config()
	if err != nil {
		return HoldDecision{}, err
	}
	budget, err := holdBudget(c)
	if err != nil {
		return HoldDecision{}, err
	}
	if budget.none() {
		return HoldDecision{}, nil
	}
	spent, err := s.Show()
	if err != nil {
		return HoldDecision{}, err
	}
	policy := s.HoldPolicy
	if policy == nil {
		policy = DefaultHoldPolicy
	}
	hold, reason := policy(HoldSummary{Summary: spent, Budget: budget})
	if !hold {
		return HoldDecision{}, nil
	}
	return HoldDecision{Hold: true, Reason: reason}, nil
}

func holdBudget(c Configuration) (HoldBudget, error) {
	var out HoldBudget
	for unit, target := range map[string]**float64{"dollars": &out.Dollars, "minutes": &out.Minutes} {
		value, ok := c.Budget[unit]
		if !ok {
			continue
		}
		n, err := value.Float64()
		if err != nil || !finite(n) {
			return HoldBudget{}, fmt.Errorf("%s budget is malformed", unit)
		}
		*target = &n
	}
	if value, ok := c.Budget["tokens"]; ok {
		n, err := ParseTokens(string(value))
		if err != nil {
			return HoldBudget{}, errors.New("tokens budget is malformed")
		}
		out.Tokens = &n
	}
	return out, nil
}

// Judge holds when spent has reached AtPercent of the budget in any unit that
// has one. Dollars and minutes compare in whole micro-units and tokens as
// integers, so a decimal amount that equals the limit holds.
func (h HoldSettings) Judge(in HoldSummary) (bool, string) {
	percent := h.AtPercent
	if !(percent > 0) || math.IsInf(percent, 0) {
		percent = 100
	}
	var reasons []string
	for _, u := range []struct {
		name       string
		budget     *float64
		spentMicro int64
		spent      float64
	}{
		{"dollars", in.Budget.Dollars, in.Totals.dollarsMicro, in.Totals.Dollars},
		{"minutes", in.Budget.Minutes, in.Totals.minutesMicro, in.Totals.Minutes},
	} {
		if u.budget == nil {
			continue
		}
		limit := math.Round(*u.budget * percent / 100 * microPerUnit)
		if limit < float64(maxMicro) && u.spentMicro >= int64(limit) {
			reasons = append(reasons, holdReason(u.name, formatAmount(u.spent), formatAmount(*u.budget), percent))
		}
	}
	if b := in.Budget.Tokens; b != nil && tokensAtLeast(in.Totals.Tokens, *b, percent) {
		reasons = append(reasons, holdReason("tokens", strconv.FormatUint(in.Totals.Tokens, 10), strconv.FormatUint(*b, 10), percent))
	}
	return len(reasons) > 0, strings.Join(reasons, "; ")
}

func holdReason(unit, spent, budget string, percent float64) string {
	reason := unit + ": " + spent + " spent of a budget of " + budget
	if percent != 100 {
		reason += " (hold at " + formatAmount(percent) + "%)"
	}
	return reason
}

func formatAmount(n float64) string { return strconv.FormatFloat(n, 'f', -1, 64) }

// tokensAtLeast compares exact integers to the decimal percent, as tokenCrossing does.
func tokensAtLeast(spent, budget uint64, percent float64) bool {
	p, ok := new(big.Rat).SetString(strconv.FormatFloat(percent, 'g', -1, 64))
	if !ok {
		return false
	}
	limit := new(big.Rat).Mul(new(big.Rat).SetInt(new(big.Int).SetUint64(budget)), p)
	limit.Quo(limit, big.NewRat(100, 1))
	return new(big.Rat).SetInt(new(big.Int).SetUint64(spent)).Cmp(limit) >= 0
}
