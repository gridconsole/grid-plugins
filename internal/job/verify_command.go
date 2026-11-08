package job

// cmd.exe /s /c removes the outer quotes and interprets the command itself.
// MSVC argv escaping (in particular backslash-quote) is not cmd.exe syntax.
// Keep this builder portable so the exact Windows command line is tested on Unix.
func windowsVerifyCommandLine(command string) string {
	return `cmd.exe /d /s /c "` + command + `"`
}
