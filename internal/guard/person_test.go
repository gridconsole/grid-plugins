package guard

import "testing"

func TestHostBashPersonDirectory(t *testing.T) {
	for _, configured := range []bool{true, false} {
		f := setup(t)
		if !configured {
			delete(f.env, "GRID_PROJECTS")
			delete(f.env, "GRID_HOME")
		}
		for _, tc := range []struct{ command, want string }{
			{`printf 'remove approve\n' >> ~/.grid-person/policy`, "deny"},
			{`echo 'remove approve' | tee -a "$HOME/.grid-person/policy"`, "deny"},
			{`rm -rf ~/.grid-person`, "deny"},
			{`touch ~/.grid-person/launchers`, "deny"},
			{`echo x > ~/.grid-''person/policy`, "deny"},
			{`cd ~/.grid-person && echo x > policy`, "deny"},
			{`cat ~/.grid-person/policy`, ""},
			{`ls ~/.grid-person`, ""},
			{`echo x > .grid-person-notes`, ""},
		} {
			t.Run(tc.command, func(t *testing.T) {
				expect(t, EvaluateHost(f.input("Bash", tc.command), f.root, f.env), tc.want)
			})
		}
	}
}
