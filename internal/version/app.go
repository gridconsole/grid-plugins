package version

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// AppWarning reports incompatibility with this installed plugin, once per named
// session. A release manifest's app_min describes that release, not this binary.
func AppWarning(home, session, app string) string {
	cmp, ok := Compare(app, AppMin)
	if !ok || cmp >= 0 {
		return ""
	}
	if home != "" && session != "" {
		dir := filepath.Join(home, "cache", "version-sessions")
		if os.MkdirAll(dir, 0o700) == nil {
			name := fmt.Sprintf("%x", sha256.Sum256([]byte(session)))
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if os.IsExist(err) {
				return ""
			}
			if err == nil {
				_ = f.Close()
			}
		}
	}
	return fmt.Sprintf("This plugin needs Grid app %s or newer; you have %s. Update the app.", AppMin, app)
}
