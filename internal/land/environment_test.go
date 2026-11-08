package land

import (
	"reflect"
	"testing"
)

func TestAllowedEnvironmentByOS(t *testing.T) {
	base := []string{"PATH=/bin", "HOME=/home/person", "USERPROFILE=C:\\Users\\person", "SystemRoot=C:\\Windows", "TEMP=/tmp", "TMP=/tmp", "LANG=en_US.UTF-8"}
	windows := []string{"ComSpec=C:\\Windows\\System32\\cmd.exe", "PATHEXT=.COM;.EXE;.BAT;.CMD", "APPDATA=roaming", "LOCALAPPDATA=local", "ProgramData=C:\\ProgramData", "SystemDrive=C:", "windir=C:\\Windows"}
	darwin := []string{"TMPDIR=/var/folders/private", "USER=person"}
	environ := append(append(append(append([]string{}, base...), windows...), darwin...), "SSH_AUTH_SOCK=/private/agent", "DEPLOY_TOKEN=secret=token", "UNLISTED_SECRET=secret", "path=/wrong-case", "ssh_auth_sock=/lowercase-agent")
	for _, tc := range []struct {
		goos     string
		platform []string
	}{
		{"linux", nil},
		{"darwin", darwin},
		{"windows", windows},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			for _, explicit := range []bool{false, true} {
				name := "defaults"
				var extra []string
				want := append(append([]string{}, base...), tc.platform...)
				if explicit {
					name = "explicit secrets"
					extra = []string{"SSH_AUTH_SOCK", "DEPLOY_TOKEN"}
					want = append(want, "SSH_AUTH_SOCK=/private/agent", "DEPLOY_TOKEN=secret=token")
				}
				if tc.goos == "windows" {
					want = append(want, "path=/wrong-case")
					if explicit {
						want = append(want, "ssh_auth_sock=/lowercase-agent")
					}
				}
				t.Run(name, func(t *testing.T) {
					if got := allowedEnvironmentFor(tc.goos, environ, extra); !reflect.DeepEqual(got, want) {
						t.Errorf("environment = %q, want %q", got, want)
					}
				})
			}
		})
	}
}
