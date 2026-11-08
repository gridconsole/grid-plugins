package land

import "testing"

func TestWindowsTaskkillPath(t *testing.T) {
	for _, tc := range []struct{ root, want string }{
		{"", `C:\Windows\System32\taskkill.exe`},
		{`C:\Windows`, `C:\Windows\System32\taskkill.exe`},
		{`D:\Windows`, `D:\Windows\System32\taskkill.exe`},
		{`D:\Windows\`, `D:\Windows\System32\taskkill.exe`},
		{`D:/Windows/`, `D:\Windows\System32\taskkill.exe`},
	} {
		t.Run(tc.root, func(t *testing.T) {
			if got := windowsTaskkillPath(tc.root); got != tc.want {
				t.Errorf("taskkill path = %q, want %q", got, tc.want)
			}
		})
	}
}
