package version

import "testing"

func TestFromDescribe(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"bded019", "0.0.0-dev+bded019"},
		{"bded019-dirty", "0.0.0-dev+bded019.dirty"},
		{"v0.2.0", "v0.2.0"},
		{"v0.2.0-3-gabc1234", "v0.2.1-dev.3+abc1234"},
		{"v0.2.0-3-gabc1234-dirty", "v0.2.1-dev.3+abc1234.dirty"},
		{"v0.2.0-dirty", "v0.2.1-dev.0+dirty"},
		{"1.0.0-rc.1-2-gabc1234", "1.0.0-rc.1.dev.2+abc1234"},
	} {
		got, err := FromDescribe(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("FromDescribe(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "release", "v1.2-3-gabc"} {
		if got, err := FromDescribe(in); err == nil {
			t.Errorf("FromDescribe(%q) = %q, want error", in, got)
		}
	}
}

func TestFromDescribeCompares(t *testing.T) {
	// Each version sorts strictly above the one before it, as Compare reads them.
	order := []string{"0.0.0-dev+bded019", "0.1.0", "v0.2.0", "v0.2.1-dev.3+abc1234", "v0.2.1", "1.0.0-rc.1", "1.0.0-rc.1.dev.2+abc1234", "1.0.0"}
	for i := 1; i < len(order); i++ {
		if c, ok := Compare(order[i-1], order[i]); !ok || c != -1 {
			t.Errorf("Compare(%q, %q) = %d, %v; want -1", order[i-1], order[i], c, ok)
		}
	}
	for _, d := range []string{"bded019", "bded019-dirty", "v0.2.0-3-gabc1234", "1.0.0-rc.1-2-gabc1234-dirty"} {
		v, err := FromDescribe(d)
		if err != nil {
			t.Fatal(err)
		}
		if c, ok := Compare(v, "0.1.0"); !ok {
			t.Errorf("Compare(%q) from %q is not comparable (%d)", v, d, c)
		}
	}
	if v, _ := FromDescribe("v0.2.0-3-gabc1234"); true {
		if c, _ := Compare(v, "v0.2.0"); c != 1 {
			t.Errorf("%q does not sort above v0.2.0", v)
		}
	}
	if v, _ := FromDescribe("bded019"); true {
		if c, _ := Compare(v, "0.1.0"); c != -1 {
			t.Errorf("%q should sort below 0.1.0 (app plugin_min)", v)
		}
	}
}
