package processid

import (
	"errors"
	"testing"
	"time"
)

func TestParseLstart(t *testing.T) {
	est := time.FixedZone("EST", -5*3600)
	for _, tc := range []struct {
		name    string
		out     string
		loc     *time.Location
		want    time.Time
		wantErr bool
		gone    bool
	}{
		{name: "C locale line", out: "Tue Nov 10 09:15:30 2026", loc: time.UTC, want: time.Date(2026, 11, 10, 9, 15, 30, 0, time.UTC)},
		{name: "padded day", out: "Tue Nov  3 09:15:30 2026", loc: time.UTC, want: time.Date(2026, 11, 3, 9, 15, 30, 0, time.UTC)},
		{name: "ps newline", out: "Tue Nov  3 09:15:30 2026\n", loc: time.UTC, want: time.Date(2026, 11, 3, 9, 15, 30, 0, time.UTC)},
		{name: "BSD column padding", out: "  Tue Nov  3 09:15:30 2026  \n", loc: time.UTC, want: time.Date(2026, 11, 3, 9, 15, 30, 0, time.UTC)},
		{name: "named zone", out: "Tue Nov  3 04:15:30 2026", loc: est, want: time.Date(2026, 11, 3, 9, 15, 30, 0, time.UTC)},
		{name: "pid has gone", out: "", loc: time.UTC, wantErr: true, gone: true},
		{name: "pid has gone, newline only", out: "\n", loc: time.UTC, wantErr: true, gone: true},
		{name: "non C locale", out: "mar. nov. 10 09:15:30 2026", loc: time.UTC, wantErr: true},
		{name: "two lines", out: "Tue Nov  3 09:15:30 2026\nTue Nov  3 09:15:31 2026\n", loc: time.UTC, wantErr: true},
		{name: "truncated", out: "Tue Nov  3 09:15", loc: time.UTC, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLstart(tc.out, tc.loc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if errors.Is(err, ErrNoProcess) != tc.gone {
				t.Fatalf("ErrNoProcess = %v, want %v (err %v)", errors.Is(err, ErrNoProcess), tc.gone, err)
			}
			if !tc.wantErr && !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got.UTC(), tc.want)
			}
		})
	}
}

func TestProcStartTicks(t *testing.T) {
	for _, tc := range []struct {
		name, stat, want string
		ok               bool
	}{
		{"plain", statLine("42", "bash", "222"), "222", true},
		{"command with spaces and paren", statLine("42", "a) b (c", "333"), "333", true},
		{"no paren", "42 bash S 1", "", false},
		{"too short", "42 (bash) S 1 42 42", "", false},
		{"not a number", statLine("42", "bash", "x1"), "", false},
		{"empty", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ProcStartTicks(tc.stat)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("got %q,%v want %q,%v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestProcTokenRoundTrip(t *testing.T) {
	boot, ticks, ok := ParseProcToken(ProcToken("0f1e-boot", "222"))
	if !ok || boot != "0f1e-boot" || ticks != "222" {
		t.Fatalf("round trip: %q %q %v", boot, ticks, ok)
	}
	for _, tc := range []struct {
		token, boot, ticks string
		ok                 bool
	}{
		{"proc:222", "", "222", true}, // older hooks wrote no boot ID
		{"proc:", "", "", false},
		{"proc:boot:", "", "", false},
		{"Tue Nov  3 09:15:30 2026", "", "", false},
		{"2026-11-03T09:15:30.000Z", "", "", false},
		{"", "", "", false},
	} {
		boot, ticks, ok := ParseProcToken(tc.token)
		if boot != tc.boot || ticks != tc.ticks || ok != tc.ok {
			t.Errorf("ParseProcToken(%q) = %q,%q,%v want %q,%q,%v", tc.token, boot, ticks, ok, tc.boot, tc.ticks, tc.ok)
		}
	}
}

// statLine builds a /proc/<pid>/stat line whose 22nd field is ticks.
func statLine(pid, comm, ticks string) string {
	return pid + " (" + comm + ") S 1 42 42 0 -1 4194304 100 0 0 0 1 1 0 0 20 0 1 0 " + ticks + " 1000 100 18446744073709551615"
}
