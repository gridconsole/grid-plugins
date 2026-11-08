package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// PostResult reports the durable log delta even when a notice could not be saved.
type PostResult struct {
	Summary
	Notices  []feed.Record `json:"notices"`
	Warnings []string      `json:"warnings"`
}

// Amounts preserves integer token budgets and baselines without float conversion.
// All entries must be JSON numbers; quoted numbers and null are invalid.
type Amounts map[string]json.Number

func (a *Amounts) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if raw == nil {
		return errors.New("amounts must be an object")
	}
	out := make(Amounts, len(raw))
	for unit, value := range raw {
		n, ok := value.(json.Number)
		if !ok {
			return errors.New("amounts must be JSON numbers")
		}
		out[unit] = n
	}
	*a = out
	return nil
}

// Configuration stores percentages, optional budgets, and task-lifetime notices.
// Baseline is the usage total when that unit's configuration was last changed:
// it prevents retroactive notices and permits retrying a crossing after a crash.
type Configuration struct {
	V                 int               `json:"v"`
	Dollars           []float64         `json:"dollars"`
	Minutes           []float64         `json:"minutes"`
	Tokens            []float64         `json:"tokens"`
	Budget            Amounts           `json:"budget"`
	Baseline          Amounts           `json:"baseline"`
	Fired             map[string]string `json:"fired"`
	TokenBudgetSource string            `json:"token_budget_source,omitempty"`
}

// Change replaces the supplied units' thresholds, or budgets when Budget is true.
// Nil leaves a unit unchanged; an empty slice explicitly clears it.
type Change struct {
	Budget           bool
	Dollars, Minutes []float64
	// Tokens changes percentages; BudgetTokens changes whole-token budgets.
	Tokens       []float64
	BudgetTokens []uint64
}

func defaults() Configuration {
	return Configuration{V: 1, Dollars: []float64{50, 80, 100}, Minutes: []float64{50, 80, 100}, Tokens: []float64{50, 80, 100},
		Budget: Amounts{}, Baseline: Amounts{"dollars": "0", "minutes": "0", "tokens": "0"}, Fired: map[string]string{}}
}

// Config returns effective settings without creating a usage folder.
func (s Store) Config() (Configuration, error) {
	c := defaults()
	err := s.readData(func(dir string) error {
		var err error
		c, err = readConfig(dir)
		return err
	})
	if err == nil {
		c = s.effectiveConfig(c)
	}
	return c, err
}

func normalize(values []float64, budget bool) ([]float64, error) {
	if values == nil {
		return nil, nil
	}
	values = append([]float64{}, values...)
	for _, value := range values {
		if !finite(value) {
			return nil, errors.New("values must be non-negative finite numbers")
		}
		if budget && value == 0 {
			return nil, errors.New("budget must be positive; use budget clear to remove it")
		}
		if !budget && value > 100 {
			return nil, errors.New("threshold is a percentage of budget; the schema limit is 100 percent")
		}
	}
	slices.Sort(values)
	values = slices.Compact(values)
	if budget && len(values) > 1 {
		return nil, errors.New("supply one budget per unit")
	}
	if !budget && len(values) > 8 {
		return nil, errors.New("at most 8 thresholds per unit")
	}
	return values, nil
}

// Configure snapshots current totals under the posting lock. Already issued
// notices remain fired even after a clear, budget change, or re-addition.
func (s Store) Configure(change Change) (Configuration, error) {
	if change.Dollars == nil && change.Minutes == nil && change.Tokens == nil && change.BudgetTokens == nil {
		return Configuration{}, errors.New("supply --dollars, --minutes or --tokens")
	}
	dollars, err := normalize(change.Dollars, change.Budget)
	if err != nil {
		return Configuration{}, err
	}
	minutes, err := normalize(change.Minutes, change.Budget)
	if err != nil {
		return Configuration{}, err
	}
	tokens, err := normalize(change.Tokens, false)
	if err != nil {
		return Configuration{}, err
	}
	if change.Budget && change.Tokens != nil || !change.Budget && change.BudgetTokens != nil {
		return Configuration{}, errors.New("token budgets and percentages must use the matching change kind")
	}
	if len(change.BudgetTokens) > 1 || len(change.BudgetTokens) == 1 && change.BudgetTokens[0] == 0 {
		return Configuration{}, errors.New("supply one positive token budget; use budget clear to remove it")
	}
	var c Configuration
	err = s.withData(func(dir string) error {
		var err error
		c, err = readConfig(dir)
		if err != nil {
			return err
		}
		summary, _, err := readLog(dir)
		if err != nil {
			return err
		}
		for _, unit := range []string{"dollars", "minutes"} {
			values, total := dollars, summary.Totals.Dollars
			if unit == "minutes" {
				values, total = minutes, summary.Totals.Minutes
			}
			if values == nil {
				continue
			}
			c.Baseline[unit] = json.Number(strconv.FormatFloat(total, 'g', -1, 64))
			if change.Budget {
				delete(c.Budget, unit)
				if len(values) > 0 {
					c.Budget[unit] = json.Number(strconv.FormatFloat(values[0], 'g', -1, 64))
				}
			} else if unit == "dollars" {
				c.Dollars = values
			} else {
				c.Minutes = values
			}
		}
		if tokens != nil || change.BudgetTokens != nil {
			c.Baseline["tokens"] = json.Number(strconv.FormatUint(summary.Totals.Tokens, 10))
			if change.Budget {
				delete(c.Budget, "tokens")
				if len(change.BudgetTokens) != 0 {
					c.Budget["tokens"] = json.Number(strconv.FormatUint(change.BudgetTokens[0], 10))
				}
			} else {
				c.Tokens = tokens
			}
		}
		return s.saveConfig(dir, c)
	})
	if err == nil {
		c = s.effectiveConfig(c)
	}
	return c, err
}

func readConfig(dir string) (Configuration, error) {
	if dir == "" {
		return defaults(), nil
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "thresholds.json"), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return defaults(), nil
	}
	if err != nil {
		return Configuration{}, err
	}
	defer f.Close()
	c := Configuration{Tokens: []float64{50, 80, 100}}
	dec := json.NewDecoder(f)
	// New optional fields must remain readable by older version-1 readers.
	// Known fields are still type-checked here and validated below.
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("thresholds.json: %w", err)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return c, errors.New("thresholds.json: expected one JSON object")
	}
	if c.V != 1 || c.Dollars == nil || c.Minutes == nil || c.Tokens == nil || c.Budget == nil || c.Baseline == nil || c.Fired == nil {
		return c, errors.New("thresholds.json: invalid version or missing fields")
	}
	for _, values := range [][]float64{c.Dollars, c.Minutes, c.Tokens} {
		normal, err := normalize(values, false)
		if err != nil || !slices.Equal(normal, values) {
			return c, errors.New("thresholds.json: invalid, unsorted or duplicate percentages")
		}
	}
	for unit, value := range c.Budget {
		if !validAmount(unit, value, true) {
			return c, errors.New("thresholds.json: invalid budget")
		}
	}
	// Version 1 settings written before tokens have just the two older baselines.
	if _, ok := c.Baseline["tokens"]; !ok {
		c.Baseline["tokens"] = "0"
	}
	if len(c.Baseline) != 3 || c.Baseline["dollars"] == "" || c.Baseline["minutes"] == "" {
		return c, errors.New("thresholds.json: missing baseline")
	}
	for unit, value := range c.Baseline {
		if !validAmount(unit, value, false) {
			return c, errors.New("thresholds.json: invalid baseline")
		}
	}
	for key, id := range c.Fired {
		unit, percent, _ := strings.Cut(key, ":")
		value, err := strconv.ParseFloat(percent, 64)
		if err != nil || !finite(value) || value > 100 || (unit != "dollars" && unit != "minutes" && unit != "tokens") || !feed.IsID(id) {
			return c, errors.New("thresholds.json: invalid fired entry")
		}
	}
	c.TokenBudgetSource = ""
	return c, nil
}

func (s Store) saveConfig(dir string, c Configuration) error {
	if s.beforeSave != nil {
		if err := s.beforeSave(); err != nil {
			return err
		}
	}
	path, err := localdata.Safe(dir, "thresholds.json")
	if err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}

func noticeKey(unit string, percent float64) string {
	return unit + ":" + strconv.FormatFloat(percent, 'g', -1, 64)
}

func (s Store) notices(dir string, out *PostResult) {
	if err := s.postNotices(dir, out); err != nil {
		out.Warnings = append(out.Warnings, "usage recorded; notice: "+err.Error())
	}
}

func (s Store) postNotices(dir string, out *PostResult) error {
	c, err := readConfig(dir)
	if err != nil {
		return err
	}
	type crossing struct {
		unit    string
		percent float64
		total   any
	}
	var pending []crossing
	for _, unit := range []string{"dollars", "minutes"} {
		budgetValue, ok := c.Budget[unit]
		if !ok {
			continue
		}
		thresholds, total, totalMicro := c.Dollars, out.Totals.Dollars, out.Totals.dollarsMicro
		if unit == "minutes" {
			thresholds, total, totalMicro = c.Minutes, out.Totals.Minutes, out.Totals.minutesMicro
		}
		base, _ := c.Baseline[unit].Float64()
		budget, _ := budgetValue.Float64()
		baseMicro := int64(math.Round(base * microPerUnit))
		for _, percent := range thresholds {
			// Whole micro-units make "exactly equal fires" hold for decimal amounts.
			amountMicro := int64(math.Round(budget * percent / 100 * microPerUnit))
			if baseMicro < amountMicro && amountMicro <= totalMicro && c.Fired[noticeKey(unit, percent)] == "" {
				pending = append(pending, crossing{unit, percent, total})
			}
		}
	}
	if budget, _ := s.tokenBudget(c); budget != nil {
		base, _ := strconv.ParseUint(string(c.Baseline["tokens"]), 10, 64)
		for _, percent := range c.Tokens {
			if tokenCrossing(base, out.Totals.Tokens, *budget, percent) && c.Fired[noticeKey("tokens", percent)] == "" {
				pending = append(pending, crossing{"tokens", percent, out.Totals.Tokens})
			}
		}
	}
	if len(pending) == 0 {
		return nil
	}
	taskID, err := feed.EnsureTaskID(s.TaskPath)
	if err != nil {
		return err
	}
	store := feed.Store{TaskDir: s.TaskPath, TaskID: taskID}
	// The contract state fold intentionally discards chatter. Read supplies the
	// same admitted, validated entries used by that fold, including notices.
	read, err := store.Read(feed.Filter{Type: "usage-notice"})
	if err != nil {
		return err
	}
	existing := map[string]string{}
	for _, entry := range read.Records {
		if entry.Verdict != "counted" || entry.Task != taskID || entry.Body["task"] != taskID {
			continue
		}
		percent, ok := entry.Body["threshold"].(float64)
		if !ok {
			continue
		}
		for _, unit := range []string{"dollars", "minutes", "tokens"} {
			if _, ok := entry.Body[unit]; ok {
				existing[noticeKey(unit, percent)] = entry.ID
			}
		}
	}
	var writer *feed.Writer
	for _, p := range pending {
		key := noticeKey(p.unit, p.percent)
		id := existing[key]
		if id == "" {
			if writer == nil {
				if s.Writer == nil {
					return errors.New("no CLI Feed writer configured")
				}
				w, err := s.Writer()
				if err != nil {
					return err
				}
				writer = &w
			}
			record, err := store.Post(feed.Input{Type: "usage-notice", Body: map[string]any{"task": taskID, "threshold": p.percent, p.unit: p.total}}, *writer)
			if err != nil {
				return err
			}
			out.Notices = append(out.Notices, record)
			id = record.ID
		}
		c.Fired[key] = id
		if err := s.saveConfig(dir, c); err != nil {
			return err
		}
	}
	return nil
}

// ParseTokens accepts only ASCII decimal digits in the uint64 range.
func ParseTokens(value string) (uint64, error) {
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, errors.New("tokens must be a non-negative integer in the uint64 range")
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("tokens must be a non-negative integer in the uint64 range")
	}
	return n, nil
}

func validAmount(unit string, value json.Number, budget bool) bool {
	if unit == "tokens" {
		n, err := ParseTokens(string(value))
		return err == nil && (!budget || n > 0)
	}
	n, err := value.Float64()
	return (unit == "dollars" || unit == "minutes") && err == nil && finite(n) && (!budget || n > 0)
}

// tokenCrossing compares exact integer counts to the configured decimal percent;
// converting counts to float64 would round large budgets and crossings.
func tokenCrossing(base, total, budget uint64, percent float64) bool {
	p, _ := new(big.Rat).SetString(strconv.FormatFloat(percent, 'g', -1, 64))
	amount := new(big.Rat).Mul(new(big.Rat).SetInt(new(big.Int).SetUint64(budget)), p)
	amount.Quo(amount, big.NewRat(100, 1))
	before := new(big.Rat).SetInt(new(big.Int).SetUint64(base))
	after := new(big.Rat).SetInt(new(big.Int).SetUint64(total))
	return before.Cmp(amount) < 0 && after.Cmp(amount) >= 0
}

func (s Store) tokenBudget(c Configuration) (*uint64, string) {
	if value, ok := c.Budget["tokens"]; ok {
		n, _ := ParseTokens(string(value)) // readConfig/Configure validated it.
		return &n, "task"
	}
	if n := s.workspaceTokens(); n != nil {
		return n, "workspace"
	}
	return nil, "none"
}

func (s Store) effectiveConfig(c Configuration) Configuration {
	n, source := s.tokenBudget(c)
	c.TokenBudgetSource = source
	if n != nil {
		c.Budget["tokens"] = json.Number(strconv.FormatUint(*n, 10))
	}
	return c
}

// workspaceTokens reads the replaceable default tokens_per_task from workspace.md
// using the existing front-matter reader. Missing, linked, non-regular, unreadable,
// oversized, invalid UTF-8 or non-positive/non-digit settings mean no default.
func (s Store) workspaceTokens() *uint64 {
	if s.GridHome == "" {
		return nil
	}
	path := filepath.Join(s.GridHome, "workspace.md")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxWorkspaceBytes {
		return nil
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxWorkspaceBytes+1))
	if err != nil || len(data) > maxWorkspaceBytes || !utf8.Valid(data) {
		return nil
	}
	front := taskfile.ParseFrontmatter(string(data))
	if front == nil {
		return nil
	}
	value, ok := front.Data["tokens_per_task"].(string)
	if !ok {
		return nil
	}
	n, err := ParseTokens(value)
	if err != nil || n == 0 {
		return nil
	}
	return &n
}

const maxWorkspaceBytes = 64 * 1024
