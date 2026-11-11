package brain

// LooksSecret reports whether s matches one of the note checker's secret
// patterns. It never returns the matched text, so a caller can refuse a value
// without echoing it.
func LooksSecret(s string) bool {
	for _, p := range secretPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}
