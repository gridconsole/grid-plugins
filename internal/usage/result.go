package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
)

// resultLine is the part of a stream-json "result" line that books usage.
type resultLine struct {
	Type         string   `json:"type"`
	SessionID    string   `json:"session_id"`
	TotalCostUSD *float64 `json:"total_cost_usd"`
	DurationMS   *float64 `json:"duration_ms"`
	Usage        *struct {
		Input         uint64 `json:"input_tokens"`
		Output        uint64 `json:"output_tokens"`
		CacheRead     uint64 `json:"cache_read_input_tokens"`
		CacheCreation uint64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	ModelUsage map[string]struct {
		CostUSD float64 `json:"costUSD"`
	} `json:"modelUsage"`
}

// modelSuffix is a context-window tag such as "[1m]" that a model name may carry in a log.
var modelSuffix = regexp.MustCompile(`\[[^\]]*\]$`)

// ParseResult reads a Claude stream-json attempt log, takes its last line of type
// "result" and returns the usage it reports with the once-only key "result:<session_id>".
// Lines that are not JSON, such as trailing text after the result line, are skipped.
// The Job is not in the log; the caller sets it.
func ParseResult(r io.Reader) (Input, string, error) {
	var last []byte
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if t := bytes.TrimSpace(line); len(t) > 0 && t[0] == '{' && bytes.Contains(t, []byte(`"result"`)) {
			var head struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(t, &head) == nil && head.Type == "result" {
				last = t
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Input{}, "", err
		}
	}
	if last == nil {
		return Input{}, "", errors.New(`no line with "type":"result" (the run may have been killed)`)
	}
	var res resultLine
	if err := json.Unmarshal(last, &res); err != nil {
		return Input{}, "", errors.New("result line is not usable: " + err.Error())
	}
	if res.TotalCostUSD == nil || math.IsNaN(*res.TotalCostUSD) || *res.TotalCostUSD < 0 {
		return Input{}, "", errors.New("result line has no total_cost_usd or it is negative")
	}
	if res.SessionID == "" || !sessionPattern.MatchString(res.SessionID) {
		return Input{}, "", errors.New("result line has no usable session_id")
	}
	in := Input{Session: res.SessionID, Dollars: res.TotalCostUSD}
	if res.DurationMS != nil {
		if *res.DurationMS < 0 || math.IsNaN(*res.DurationMS) {
			return Input{}, "", errors.New("result line has a negative duration_ms")
		}
		minutes := *res.DurationMS / 60000
		in.Minutes = &minutes
	}
	if u := res.Usage; u != nil {
		total := u.Input
		for _, n := range []uint64{u.Output, u.CacheRead, u.CacheCreation} {
			if n > math.MaxUint64-total {
				return Input{}, "", errors.New("result line token counts overflow")
			}
			total += n
		}
		in.Tokens = &total
	}
	in.Model = dearestModel(res.ModelUsage)
	in.Key = "result:" + res.SessionID
	return in, in.Key, nil
}

// dearestModel returns the model with the largest costUSD, the lower name on a tie,
// or "" when the log names none a post would accept.
func dearestModel(m map[string]struct {
	CostUSD float64 `json:"costUSD"`
}) string {
	best, bestCost := "", -1.0
	for name, v := range m {
		name = strings.TrimSpace(modelSuffix.ReplaceAllString(strings.TrimSpace(name), ""))
		if !modelPattern.MatchString(name) {
			continue
		}
		if v.CostUSD > bestCost || (v.CostUSD == bestCost && name < best) {
			best, bestCost = name, v.CostUSD
		}
	}
	return best
}
