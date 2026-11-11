package eval

import (
	"strconv"
	"strings"
)

// windowsTaskkillPath uses Windows separators on every host so it can be tested
// without a Windows runtime. An unset SystemRoot must never yield a relative path,
// so a taskkill.exe placed on PATH or in the working directory is never run.
func windowsTaskkillPath(systemRoot string) string {
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return strings.TrimRight(strings.ReplaceAll(systemRoot, "/", `\`), `\`) + `\System32\taskkill.exe`
}

// taskkillArgs ends pid and every process it started (/T) without asking (/F).
func taskkillArgs(pid int) []string {
	return []string{"/T", "/F", "/PID", strconv.Itoa(pid)}
}
