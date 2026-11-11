package cli

import (
	"errors"
	"reflect"
	"testing"
)

func openEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func openLook(path string, err error) func(string) (string, error) {
	return func(name string) (string, error) {
		if name != "xdg-open" {
			return "", errors.New("unexpected lookup " + name)
		}
		return path, err
	}
}

func TestOpenCommandPerOS(t *testing.T) {
	const target = "http://127.0.0.1:1/#/t/p/x/overview"
	none := openEnv(nil)
	cases := []struct {
		name string
		goos string
		env  map[string]string
		want []string
	}{
		{"darwin", "darwin", nil, []string{"open", target}},
		{"windows", "windows", nil, []string{"rundll32", "url.dll,FileProtocolHandler", target}},
		{"linux display", "linux", map[string]string{"DISPLAY": ":0"}, []string{"/usr/bin/xdg-open", target}},
		{"freebsd falls to xdg-open", "freebsd", map[string]string{"DISPLAY": ":0"}, []string{"/usr/bin/xdg-open", target}},
	}
	for _, c := range cases {
		env := none
		if c.env != nil {
			env = openEnv(c.env)
		}
		got, err := openCommand(c.goos, target, env, openLook("/usr/bin/xdg-open", nil))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: argv %q, want %q", c.name, got, c.want)
		}
	}
}

func TestOpenCommandWindowsKeepsTargetWhole(t *testing.T) {
	for _, target := range []string{
		"http://h/#/t/my project/x/overview",
		"http://h/?a=1&b=2#/t/p/x/overview",
	} {
		got, err := openCommand("windows", target, openEnv(nil), openLook("", errors.New("not used")))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[1] != "url.dll,FileProtocolHandler" || got[2] != target {
			t.Errorf("target %q: argv %q, want the target as the one item after the handler", target, got)
		}
		for _, a := range got {
			if a == "cmd" || a == "cmd.exe" || a == "/c" {
				t.Errorf("target %q: argv %q goes through a shell", target, got)
			}
		}
	}
}

func TestOpenCommandLinuxDisplayAndLookup(t *testing.T) {
	look := openLook("/usr/bin/xdg-open", nil)

	if _, err := openCommand("linux", "u", openEnv(nil), look); err == nil || err.Error() != "no display" {
		t.Errorf("no DISPLAY and no WAYLAND_DISPLAY: err %v, want \"no display\"", err)
	}
	if _, err := openCommand("linux", "u", openEnv(map[string]string{"DISPLAY": "", "WAYLAND_DISPLAY": ""}), look); err == nil || err.Error() != "no display" {
		t.Errorf("both empty: err %v, want \"no display\"", err)
	}

	missing := errors.New("executable file not found in $PATH")
	_, err := openCommand("linux", "u", openEnv(map[string]string{"DISPLAY": ":0"}), openLook("", missing))
	if !errors.Is(err, missing) {
		t.Errorf("xdg-open missing: err %v, want the lookPath error", err)
	}

	got, err := openCommand("linux", "u", openEnv(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}), look)
	if err != nil {
		t.Fatalf("WAYLAND_DISPLAY alone: %v", err)
	}
	if want := []string{"/usr/bin/xdg-open", "u"}; !reflect.DeepEqual(got, want) {
		t.Errorf("WAYLAND_DISPLAY alone: argv %q, want %q", got, want)
	}
}
