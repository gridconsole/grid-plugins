package session

import (
	"strconv"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// tokenFixed is the one instant these tests build their tokens from, with a
// sub-second part so a read that kept it would show.
var tokenFixed = time.Date(2026, time.November, 3, 9, 15, 30, 123456789, time.UTC)

// The bare integer is the token `grid task attach` wrote on Windows before it
// went through processid.Start. The probe cannot read it, so an attached
// session stayed Unknown.
func TestTokenTimeBareNanosecondsIsNotRead(t *testing.T) {
	token := strconv.FormatInt(tokenFixed.UnixNano(), 10)
	if got, ok := tokenTime(token); ok {
		t.Fatalf("tokenTime(%q) = %v, true; want not ok", token, got)
	}
}

// The lstart line is what processid.Start writes on Windows now: the probe
// reads it back at second precision.
func TestTokenTimeLstartLineIsRead(t *testing.T) {
	token := tokenFixed.UTC().Format(processid.LstartLayout)
	got, ok := tokenTime(token)
	if !ok {
		t.Fatalf("tokenTime(%q) not ok; want ok", token)
	}
	if want := tokenFixed.Truncate(time.Second); !got.Equal(want) {
		t.Fatalf("tokenTime(%q) = %v; want %v", token, got, want)
	}
}

func TestTokenTimeRFC3339NanoReadsAtSecondPrecision(t *testing.T) {
	cases := []struct {
		name  string
		token string
	}{
		{"utc", tokenFixed.Format(time.RFC3339Nano)},
		{"offset", tokenFixed.In(time.FixedZone("plus1", 3600)).Format(time.RFC3339Nano)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := tokenTime(c.token)
			if !ok {
				t.Fatalf("tokenTime(%q) not ok; want ok", c.token)
			}
			if want := tokenFixed.Truncate(time.Second); !got.Equal(want) {
				t.Fatalf("tokenTime(%q) = %v; want %v", c.token, got, want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("tokenTime(%q) is in %v; want UTC", c.token, got.Location())
			}
		})
	}
}

func TestTokenTimeUnreadableTokensAreNotOK(t *testing.T) {
	for _, token := range []string{"", "proc:1:2", "garbage"} {
		if got, ok := tokenTime(token); ok {
			t.Errorf("tokenTime(%q) = %v, true; want not ok", token, got)
		}
	}
}
