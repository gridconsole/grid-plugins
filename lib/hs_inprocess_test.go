package lib_test

import (
	"testing"

	"github.com/gridconsole/grid-plugins/lib"
)

func TestHsIpLibExport(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"worker", "next"}, false},
		{[]string{"lease", "take", "task", "--hold"}, false},
		{[]string{"mcp"}, false},
		{[]string{"version"}, true},
		{[]string{"feed", "state", "--json", "--task", "T-01"}, true},
		{[]string{"task", "--help"}, true},
		{[]string{}, false},
		{[]string{"nonsense"}, false},
		{[]string{"task", "nonsense"}, false},
	} {
		ok, why := lib.InProcess(tc.args)
		if ok != tc.want || why == "" {
			t.Errorf("lib.InProcess(%q) = %v %q, want %v with a why", tc.args, ok, why, tc.want)
		}
	}
}
