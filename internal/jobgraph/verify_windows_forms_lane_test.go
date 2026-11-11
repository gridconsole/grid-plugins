package jobgraph

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// A verify command written for cmd.exe (program names with .exe/.cmd/.bat/.com,
// backslash paths) is read the same way on every host.
func TestVerifyWindowsForms(t *testing.T) {
	for _, tc := range []struct {
		command string
		valid   bool
	}{
		// Names a test: extension dropped, name lower-cased, backslashes plain.
		{`go.exe test ./...`, true},
		{`GO.EXE test ./...`, true},
		{`npm.cmd test`, true},
		{`C:\Go\bin\go.exe test ./...`, true},
		{`"C:\Program Files\Go\bin\go.exe" test ./...`, true},
		{`cargo.exe test`, true},
		{`node.exe --test tests\a.test.mjs`, true},
		{`C:\Go\bin\GO.EXE test ./...`, true},
		{`PYTEST.EXE tests`, true},
		{`scripts\run-tests.bat`, true},
		{`go.exe build ./... && C:\Go\bin\go.exe test ./...`, true},
		// Still refused.
		{`go.exe build ./...`, false},
		{`echo.exe test`, false},
		{`C:\tools\make.exe check`, false},
		{`go.exe vet ./...`, false},
		{`$(go test)`, false},
		{`node.exe -e "test"`, false},
		{`C:\Go\bin\go.exe test $(x)`, false},
		{`C:\Go\bin\go.exe test (x)`, false},
		// A POSIX escape keeps the first reading's answer.
		{`go\ test`, false},
		{`echo hello\; go test ./...`, false},
		{`echo C:\\Go\\bin\\go.exe test`, false},
		// No extension: the case is kept, as before.
		{`GO test ./...`, false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			err := ValidateVerify(job.Job{Verify: job.Verify{Command: tc.command, MinTests: 1}})
			if (err == nil) != tc.valid {
				t.Fatalf("got %v, valid %v", err, tc.valid)
			}
			if err != nil && !strings.Contains(err.Error(), "must name a test") {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}
