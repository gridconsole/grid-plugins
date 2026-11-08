package job

import "testing"

func TestWindowsVerifyCommandLine(t *testing.T) {
	for _, tc := range []struct{ command, want string }{
		{`go test -v ./...`, `cmd.exe /d /s /c "go test -v ./..."`},
		{`go test -v -run "^TestExample$" ./...`, `cmd.exe /d /s /c "go test -v -run "^TestExample$" ./..."`},
		{`"C:\Program Files\Go\bin\go.exe" test -v ./...`, `cmd.exe /d /s /c ""C:\Program Files\Go\bin\go.exe" test -v ./..."`},
		{`echo "a b" && echo "c"`, `cmd.exe /d /s /c "echo "a b" && echo "c""`},
	} {
		if got := windowsVerifyCommandLine(tc.command); got != tc.want {
			t.Fatalf("command %q: got %q, want %q", tc.command, got, tc.want)
		}
	}
}
