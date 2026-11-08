package land

import (
	"os"
	"runtime"
	"strings"
)

func envKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func allowedEnvironment(extra []string) []string {
	allowed := make(map[string]bool)
	for _, key := range append([]string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "LANG"}, extra...) {
		allowed[envKey(key)] = true
	}
	env := []string{}
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if allowed[envKey(key)] {
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
