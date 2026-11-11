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

// allowedKeysFor is the allow list: the fixed names for goos, then the rule's own
// names. Credentials, including SSH_AUTH_SOCK, require an explicit lands.env entry.
func allowedKeysFor(goos string, extra []string) []string {
	keys := []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG"}
	switch goos {
	case "windows":
		keys = append(keys, "COMSPEC", "PATHEXT", "APPDATA", "LOCALAPPDATA", "ProgramData", "SystemDrive", "windir")
	case "darwin":
		keys = append(keys, "TMPDIR", "USER")
	}
	return append(keys, extra...)
}

// allowedEnvironmentGetenv builds the allowed environment from getenv, the Call's
// own environment, instead of filtering a process environment. Each allowed name
// is asked once, in allow-list order, and passed as NAME=value when the answer is
// not empty. The name keeps the allow list's spelling, and a rule name that folds
// to one already asked (PATH and Path on windows) is not asked again.
func allowedEnvironmentGetenv(goos string, getenv func(string) string, extra []string) []string {
	asked := make(map[string]bool)
	env := []string{}
	for _, key := range allowedKeysFor(goos, extra) {
		folded := envKeyFor(goos, key)
		if asked[folded] {
			continue
		}
		asked[folded] = true
		if value := getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// allowedEnvironmentFrom reads the Call's environment through getenv; nil reads
// the host, as the binary does.
func allowedEnvironmentFrom(getenv func(string) string, extra []string) []string {
	if getenv == nil {
		return allowedEnvironment(extra)
	}
	return allowedEnvironmentGetenv(runtime.GOOS, getenv, extra)
}

func allowedEnvironmentFor(goos string, environ, extra []string) []string {
	allowed := make(map[string]bool)
	for _, key := range allowedKeysFor(goos, extra) {
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

func landEnvironment(p Plan, getenv func(string) string) []string {
	env := allowedEnvironmentFrom(getenv, p.Rule.Env)
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
