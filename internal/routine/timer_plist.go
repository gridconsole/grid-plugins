package routine

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/localdata"
)

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

type Launchd struct{ UID int }

func (b Launchd) Plan(s TimerSpec) (TimerPlan, error) {
	label := s.label()
	logDir, err := localdata.Safe(s.GridHome, "logs", "routines", label)
	if err != nil {
		return TimerPlan{}, err
	}
	path := filepath.Join(s.Home, "Library", "LaunchAgents", label+".plist")
	var args strings.Builder
	for _, arg := range s.argv() {
		args.WriteString("<string>" + xmlText(arg) + "</string>")
	}
	content := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		"<!-- " + marker(label) + " -->\n" + `<plist version="1.0"><dict>` +
		"<key>Label</key><string>" + label + "</string>" +
		"<key>ProgramArguments</key><array>" + args.String() + "</array>" +
		"<key>WorkingDirectory</key><string>" + xmlText(s.GridHome) + "</string>" +
		"<key>EnvironmentVariables</key><dict><key>GRID_HOME</key><string>" + xmlText(s.GridHome) + "</string><key>PATH</key><string>" + xmlText(s.Path) + "</string></dict>" +
		"<key>StandardOutPath</key><string>" + xmlText(filepath.Join(logDir, "stdout.log")) + "</string>" +
		"<key>StandardErrorPath</key><string>" + xmlText(filepath.Join(logDir, "stderr.log")) + "</string>" +
		"<key>StartInterval</key><integer>60</integer>" +
		"</dict></plist>\n"
	domain := fmt.Sprintf("gui/%d", b.UID)
	return TimerPlan{Label: label, Project: s.Project, Name: s.Name, LogDir: logDir, Files: []TimerFile{{path, content}}, Install: []OSCommand{{"launchctl", []string{"bootstrap", domain, path}}}, Remove: []OSCommand{{"launchctl", []string{"bootout", domain + "/" + label}}}}, nil
}
