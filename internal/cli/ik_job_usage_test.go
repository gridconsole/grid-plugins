package cli

import (
	"strings"
	"testing"
)

const ikJuCyclesLine = "       grid job cycles <items.json> [--json]"

func TestIkJuJobHelpExitLine(t *testing.T) {
	// grid job --help is the usage on stdout, exit 0; a bare grid job is a usage
	// error as a bare grid feed is: the same usage on stderr, exit 2 (P-r23-cli-help row 3).
	for _, args := range [][]string{{"job", "--help"}, {"job"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h, path := feedHarness(t)
			wantCode := 0
			if len(args) == 1 {
				wantCode = 2
			}
			if code := h.run(path, args...); code != wantCode {
				t.Fatalf("exit %d, want %d: %s %s", code, wantCode, h.out, h.errb)
			}
			usageText := h.out
			if wantCode == 2 {
				usageText = h.errb
			}
			lines := strings.Split(usageText.String(), "\n")
			var got []string
			prev := ""
			for i, l := range lines {
				if strings.HasPrefix(l, "Exit status:") {
					got = append(got, l)
					if i > 0 {
						prev = lines[i-1]
					}
				}
			}
			if len(got) != 1 || got[0] != strings.TrimSuffix(exitStatusHelp, "\n") {
				t.Fatalf("lines starting with Exit status: %q, want exactly %q\nout: %s", got, exitStatusHelp, h.out)
			}
			if prev != ikJuCyclesLine {
				t.Fatalf("line before Exit status: %q, want %q", prev, ikJuCyclesLine)
			}
		})
	}
}
