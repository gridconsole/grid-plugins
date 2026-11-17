package machinefolder_test

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// gridconfig.MachineDirFor (the config reader) and machinefolder.DefaultPath (the
// resolver) both name the OS standard machine folder. They must give one answer.
func TestDefaultPathAgreesWithGridconfig(t *testing.T) {
	appdata := func(k string) string {
		if k == "LOCALAPPDATA" {
			return `C:\U\s\AppData\Local`
		}
		return ""
	}
	none := func(string) string { return "" }
	cases := []struct {
		goos   string
		getenv func(string) string
		home   string
	}{
		{"linux", none, "/home/s"},
		{"darwin", none, "/Users/s"},
		{"windows", appdata, `C:\U\s`},
		{"windows", none, `C:\U\s`},
		{"linux", none, ""},
		{"darwin", none, ""},
	}
	for _, c := range cases {
		want := gridconfig.MachineDirFor(c.goos, c.getenv, c.home)
		got := machinefolder.DefaultPath(machinefolder.Env{Getenv: c.getenv, Home: c.home, GOOS: c.goos})
		if got != want {
			t.Errorf("%s home=%q: machinefolder.DefaultPath = %q, gridconfig.MachineDirFor = %q", c.goos, c.home, got, want)
		}
	}
}
