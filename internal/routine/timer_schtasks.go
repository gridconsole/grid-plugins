package routine

import (
	"path/filepath"
	"strings"
)

// windowsArg is the quoting convention used by CommandLineToArgvW.
func windowsArg(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, r := range s {
		if r == '\\' {
			slashes++
			continue
		}
		if r == '"' {
			b.WriteString(strings.Repeat("\\", slashes*2+1))
			b.WriteRune(r)
		} else {
			b.WriteString(strings.Repeat("\\", slashes))
			b.WriteRune(r)
		}
		slashes = 0
	}
	b.WriteString(strings.Repeat("\\", slashes*2))
	b.WriteByte('"')
	return b.String()
}

type Schtasks struct{}

func (Schtasks) Plan(s TimerSpec) (TimerPlan, error) {
	label := s.label()
	path := filepath.Join(s.Home, ".grid-routine-timers", label+".xml")
	argv := s.argv()[1:]
	for i := range argv {
		argv[i] = windowsArg(argv[i])
	}
	// A tiny cmd wrapper would lose argv boundaries and invite shell expansion.
	// Instead --grid-home carries the local data location directly to this command.
	argv = append(argv, "--grid-home", windowsArg(s.GridHome))
	content := `<?xml version="1.0" encoding="UTF-16"?>` + "\n" + `<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">` +
		"<RegistrationInfo><Description>" + marker(label) + "</Description></RegistrationInfo>" +
		"<Triggers><TimeTrigger><Repetition><Interval>PT1M</Interval><StopAtDurationEnd>false</StopAtDurationEnd></Repetition><StartBoundary>2020-01-01T00:00:00Z</StartBoundary><Enabled>true</Enabled></TimeTrigger></Triggers>" +
		"<Principals><Principal id=\"Author\"><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>" +
		"<Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><ExecutionTimeLimit>PT11M</ExecutionTimeLimit></Settings>" +
		"<Actions Context=\"Author\"><Exec><Command>" + xmlText(s.Binary) + "</Command><Arguments>" + xmlText(strings.Join(argv, " ")) + "</Arguments><WorkingDirectory>" + xmlText(s.GridHome) + "</WorkingDirectory></Exec></Actions></Task>\n"
	return TimerPlan{Label: label, Project: s.Project, Name: s.Name, Files: []TimerFile{{Path: path, Content: content, Encoding: "UTF-16LE"}}, Install: []OSCommand{{"schtasks", []string{"/Create", "/TN", label, "/XML", path}}}, Remove: []OSCommand{{"schtasks", []string{"/Delete", "/TN", label, "/F"}}}}, nil
}
