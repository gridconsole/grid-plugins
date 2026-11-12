package routine

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interval accepts durations of at least a minute or five numeric cron fields.
// Cron is evaluated in UTC, including the usual day-of-month/day-of-week OR.
// The OS owns the minute tick; no Grid process keeps a clock between calls.
type Interval struct {
	Duration time.Duration
	fields   [5]map[int]bool
	wildcard [5]bool
}

func ParseInterval(value string) (Interval, error) {
	var s Interval
	if d, err := time.ParseDuration(value); err == nil {
		if d < time.Minute || d%time.Second != 0 {
			return s, fmt.Errorf("duration must be whole seconds and at least one minute")
		}
		s.Duration = d
		return s, nil
	}
	parts := strings.Fields(value)
	if len(parts) != 5 {
		return s, fmt.Errorf("every must be a duration or five-field numeric cron expression")
	}
	lows, highs := [5]int{0, 0, 1, 1, 0}, [5]int{59, 23, 31, 12, 7}
	for i, field := range parts {
		s.wildcard[i] = strings.HasPrefix(field, "*")
		s.fields[i] = map[int]bool{}
		for _, part := range strings.Split(field, ",") {
			base, stepText, hasStep := strings.Cut(part, "/")
			step := 1
			if hasStep {
				var err error
				step, err = strconv.Atoi(stepText)
				if err != nil || step < 1 || step > highs[i]-lows[i]+1 {
					return s, fmt.Errorf("invalid cron step %q", part)
				}
			}
			lo, hi := lows[i], highs[i]
			if base != "*" {
				a, b, hasRange := strings.Cut(base, "-")
				var err error
				lo, err = strconv.Atoi(a)
				if err != nil {
					return s, fmt.Errorf("invalid cron field %q", part)
				}
				hi = lo
				if hasRange {
					hi, err = strconv.Atoi(b)
					if err != nil {
						return s, fmt.Errorf("invalid cron range %q", part)
					}
				} else if hasStep {
					hi = highs[i]
				}
			}
			if lo < lows[i] || hi > highs[i] || hi < lo {
				return s, fmt.Errorf("cron field out of range: %q", part)
			}
			for n := lo; n <= hi; n += step {
				if i == 4 && n == 7 {
					s.fields[i][0] = true
				} else {
					s.fields[i][n] = true
				}
			}
		}
	}
	return s, nil
}

func (s Interval) Due(now, last time.Time) bool {
	if s.Duration != 0 {
		return last.IsZero() || !now.Before(last.Add(s.Duration))
	}
	now = now.UTC()
	if !last.IsZero() && !now.Truncate(time.Minute).After(last.UTC().Truncate(time.Minute)) {
		return false
	}
	return s.matches(now)
}

// matches reports whether the UTC minute of t satisfies the five cron fields.
// Due and Next both decide with it, so they cannot drift apart.
func (s Interval) matches(t time.Time) bool {
	return s.fields[0][t.Minute()] && s.fields[1][t.Hour()] && s.fields[3][int(t.Month())] && s.dayMatches(t)
}

// dayMatches is the usual day-of-month/day-of-week OR: when both fields are
// restricted either may match, otherwise both must.
func (s Interval) dayMatches(t time.Time) bool {
	day, week := s.fields[2][t.Day()], s.fields[4][int(t.Weekday())]
	if !s.wildcard[2] && !s.wildcard[4] {
		return day || week
	}
	return day && week
}

// nextWindow is how far Next looks for a minute that matches.
const nextWindow = 366 * 24 * time.Hour

// Next is the next time the OS timer would start the routine after the instant
// after, given the last run. A duration counts from last (or is due now when it
// never ran or is overdue). A cron expression is walked minute by minute with the
// rules of Due: the minute must be later than the minute of last. It reports false
// when no minute matches within 366 days of after.
func (s Interval) Next(after, last time.Time) (time.Time, bool) {
	after = after.UTC()
	if s.Duration != 0 {
		if last.IsZero() {
			return after, true
		}
		if next := last.UTC().Add(s.Duration); next.After(after) {
			return next, true
		}
		return after, true
	}
	start := after.Truncate(time.Minute).Add(time.Minute)
	if !last.IsZero() {
		if first := last.UTC().Truncate(time.Minute).Add(time.Minute); first.After(start) {
			start = first
		}
	}
	end := after.Add(nextWindow)
	for t := start; !t.After(end); {
		switch {
		case !s.fields[3][int(t.Month())]:
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		case !s.dayMatches(t):
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
		case !s.fields[1][t.Hour()]:
			t = t.Truncate(time.Hour).Add(time.Hour)
		case s.matches(t):
			return t, true
		default:
			t = t.Add(time.Minute)
		}
	}
	return time.Time{}, false
}
