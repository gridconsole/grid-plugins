package routine

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Systemd struct{}

// systemd expands percent specifiers and dollars even without a shell.
func systemdQuote(s string) string {
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "%", "%%", "\t", "\\t").Replace(s)
	return `"` + s + `"`
}

// WorkingDirectory= takes the rest of the line as one path and does not unquote
// it; only percent specifiers are expanded.
func systemdPath(s string) string { return strings.ReplaceAll(s, "%", "%%") }

func (Systemd) Plan(s TimerSpec) (TimerPlan, error) {
	label := s.label()
	dir := filepath.Join(s.Home, ".config", "systemd", "user")
	argv := s.argv()
	for i := range argv {
		argv[i] = systemdQuote(strings.ReplaceAll(argv[i], "$", "$$"))
	}
	service := "# " + marker(label) + "\n[Unit]\nDescription=Grid routine\n[Service]\nType=oneshot\nWorkingDirectory=" + systemdPath(s.GridHome) + "\nEnvironment=" + systemdQuote("GRID_HOME="+s.GridHome) + "\nEnvironment=" + systemdQuote("PATH="+s.Path) + "\nExecStart=" + strings.Join(argv, " ") + "\nTimeoutStartSec=660\n"
	timer := "# " + marker(label) + "\n[Unit]\nDescription=Grid routine schedule\n[Timer]\nOnCalendar=*-*-* *:*:00 UTC\nAccuracySec=1s\nPersistent=false\nUnit=" + label + ".service\n[Install]\nWantedBy=timers.target\n"
	return TimerPlan{Label: label, Project: s.Project, Name: s.Name, Files: []TimerFile{{filepath.Join(dir, label+".service"), service}, {filepath.Join(dir, label+".timer"), timer}}, Install: []OSCommand{{"systemctl", []string{"--user", "daemon-reload"}}, {"systemctl", []string{"--user", "enable", "--now", label + ".timer"}}}, Remove: []OSCommand{{"systemctl", []string{"--user", "disable", "--now", label + ".timer"}}}, AfterRemove: []OSCommand{{"systemctl", []string{"--user", "daemon-reload"}}}}, nil
}

// Unsupported is selected on platforms without one of the three timer systems.
type Unsupported struct{}

func (Unsupported) Plan(TimerSpec) (TimerPlan, error) {
	return TimerPlan{}, fmt.Errorf("OS routine timers are not supported on this platform")
}
