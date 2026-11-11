package cli

import (
	"strings"
	"testing"
)

const ikJuCyclesLine = "       grid job cycles <items.json> [--json]"

func TestIkJuJobHelpExitLine(t *testing.T) {
	for _, args := range [][]string{{"job", "--help"}, {"job"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h, path := feedHarness(t)
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
			lines := strings.Split(h.out.String(), "\n")
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
