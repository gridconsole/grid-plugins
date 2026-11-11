package land

import (
	"reflect"
	"strings"
	"testing"
)

// jzLdFake answers from a map the way os.Getenv does on the host it models: exact
// names on unix, case-blind names on windows. It records every name it is asked.
func jzLdFake(caseBlind bool, answers map[string]string) (func(string) string, *[]string) {
	table := map[string]string{}
	for name, value := range answers {
		if caseBlind {
			name = strings.ToLower(name)
		}
		table[name] = value
	}
	asked := &[]string{}
	return func(name string) string {
		*asked = append(*asked, name)
		if caseBlind {
			name = strings.ToLower(name)
		}
		return table[name]
	}, asked
}

func TestJzLdAllowedEnvironmentFromGetenv(t *testing.T) {
	for _, tc := range []struct {
		goos      string
		caseBlind bool
		answers   map[string]string
		extra     []string
		want      []string
		asked     []string
	}{
		{
			goos: "linux",
			answers: map[string]string{
				"PATH": "/jz/bin", "HOME": "/jz/home", "LANG": "C.UTF-8", "JZ_MARK": "call", "SECRET": "s3cret",
				"USERPROFILE": "", "USER": "person", "TMPDIR": "/jz/tmp", "path": "/wrong-case", "ComSpec": "cmd",
			},
			extra: []string{"JZ_MARK"},
			want:  []string{"PATH=/jz/bin", "HOME=/jz/home", "LANG=C.UTF-8", "JZ_MARK=call"},
			asked: []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG", "JZ_MARK"},
		},
		{
			goos: "darwin",
			answers: map[string]string{
				"PATH": "/jz/bin", "HOME": "/jz/home", "LANG": "C.UTF-8", "JZ_MARK": "call", "SECRET": "s3cret",
				"USER": "person", "TMPDIR": "/jz/tmp", "ComSpec": "cmd",
			},
			extra: []string{"JZ_MARK"},
			want:  []string{"PATH=/jz/bin", "HOME=/jz/home", "LANG=C.UTF-8", "TMPDIR=/jz/tmp", "USER=person", "JZ_MARK=call"},
			asked: []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG", "TMPDIR", "USER", "JZ_MARK"},
		},
		{
			goos:      "windows",
			caseBlind: true,
			answers: map[string]string{
				"Path": `C:\jz\bin`, "systemroot": `C:\JzWindows`, "HOME": `C:\jz\home`, "LANG": "C.UTF-8", "JZ_MARK": "call", "SECRET": "s3cret",
				"ComSpec": `C:\JzWindows\System32\cmd.exe`, "windir": `C:\JzWindows`, "USER": "person", "TMPDIR": "/jz/tmp",
			},
			// Path and SYSTEMROOT fold onto names the fixed list already asked.
			extra: []string{"JZ_MARK", "Path", "SYSTEMROOT"},
			want:  []string{`PATH=C:\jz\bin`, `HOME=C:\jz\home`, `SystemRoot=C:\JzWindows`, "LANG=C.UTF-8", `COMSPEC=C:\JzWindows\System32\cmd.exe`, `windir=C:\JzWindows`, "JZ_MARK=call"},
			asked: []string{
				"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG",
				"COMSPEC", "PATHEXT", "APPDATA", "LOCALAPPDATA", "ProgramData", "SystemDrive", "windir", "JZ_MARK",
			},
		},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			getenv, asked := jzLdFake(tc.caseBlind, tc.answers)
			got := allowedEnvironmentGetenv(tc.goos, getenv, tc.extra)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environment = %q, want %q", got, tc.want)
			}
			for _, pair := range got {
				if strings.HasPrefix(pair, "SECRET=") {
					t.Errorf("a name outside the allow list reached the environment: %q", pair)
				}
			}
			// Each allowed name is asked once, with the allow list's spelling, and nothing else is asked.
			if !reflect.DeepEqual(*asked, tc.asked) {
				t.Errorf("getenv was asked %q, want %q", *asked, tc.asked)
			}
		})
	}
}

// With a Getenv the Call's answer wins over the host's, and landEnvironment still
// puts the plan's own GRID_LAND_ names last, once each.
func TestJzLdLandEnvironmentUsesGetenv(t *testing.T) {
	t.Setenv("JZ_MARK", "host")
	p := Plan{Task: "T-01", Hash: "hash", Rule: Rule{Target: "local", Branch: "main", Env: []string{"JZ_MARK", "GRID_LAND_BRANCH"}}}
	call := func(name string) string {
		switch name {
		case "JZ_MARK":
			return "call"
		case "GRID_LAND_BRANCH":
			return "from-the-call"
		}
		return ""
	}
	got := landEnvironment(p, call)
	want := []string{"JZ_MARK=call", "GRID_LAND_TASK=T-01", "GRID_LAND_TARGET=local", "GRID_LAND_BRANCH=main", "GRID_PLAN_HASH=hash"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("environment = %q, want %q", got, want)
	}
	// Nil reads the host: the os.Environ filter, unchanged.
	host := landEnvironment(p, nil)
	found := false
	for _, pair := range host {
		if pair == "JZ_MARK=host" {
			found = true
		}
		if pair == "JZ_MARK=call" {
			t.Errorf("a nil Getenv returned the call's value: %q", host)
		}
	}
	if !found {
		t.Errorf("a nil Getenv did not read the host's JZ_MARK: %q", host)
	}
}
