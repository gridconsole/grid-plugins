package land

import (
	"os"
	"runtime"
	"strings"
)

func envKey(key string) string {
	return envKeyFor(runtime.GOOS, key)
}

func envKeyFor(goos, key string) string {
	if goos == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func allowedEnvironment(extra []string) []string {
	return allowedEnvironmentFor(runtime.GOOS, os.Environ(), extra)
}

func allowedEnvironmentFor(goos string, environ, extra []string) []string {
	keys := []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG"}
	switch goos {
	case "windows":
		keys = append(keys, "COMSPEC", "PATHEXT", "APPDATA", "LOCALAPPDATA", "ProgramData", "SystemDrive", "windir")
	case "darwin":
		keys = append(keys, "TMPDIR", "USER")
	}
	// Credentials, including SSH_AUTH_SOCK, require an explicit lands.env entry.
	allowed := make(map[string]bool)
	for _, key := range append(keys, extra...) {
		allowed[envKeyFor(goos, key)] = true
	}
	env := []string{}
	for _, value := range environ {
		key, _, _ := strings.Cut(value, "=")
		if allowed[envKeyFor(goos, key)] {
			env = append(env, value)
		}
	}
	return env
}

func landEnvironment(p Plan) []string {
	env := allowedEnvironment(p.Rule.Env)
	for _, pair := range []struct{ key, value string }{
		{"GRID_LAND_TASK", p.Task}, {"GRID_LAND_TARGET", p.Rule.Target},
		{"GRID_LAND_BRANCH", p.Rule.Branch}, {"GRID_PLAN_HASH", p.Hash},
	} {
		next := env[:0]
		for _, value := range env {
			key, _, _ := strings.Cut(value, "=")
			if envKey(key) != envKey(pair.key) {
				next = append(next, value)
			}
		}
		env = append(next, pair.key+"="+pair.value)
	}
	return env
}
