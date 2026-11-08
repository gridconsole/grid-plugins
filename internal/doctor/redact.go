package doctor

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN [A-Za-z0-9 /_-]+-----.*?(?:-----END [A-Za-z0-9 /_-]+-----|\z)`),
	regexp.MustCompile(`(?i)\bbearer[ \t]+[^\s"'<>;,]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),
}

var jwsPattern = regexp.MustCompile(`[A-Za-z0-9_-]{2,}\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]{2,}`)

// Redact removes support-bundle secrets and replaces the user's home with ~.
// JWS recognition is deliberately conservative: any three base64url-like parts
// are removed, including ordinary text that has that shape.
func Redact(text, home string) string {
	// Home first, since a dotted home path could otherwise match the JWS pattern.
	if home != "" {
		text = strings.ReplaceAll(text, home, "~")
		if strings.Contains(home, `\`) {
			text = strings.ReplaceAll(text, strings.ReplaceAll(home, `\`, "/"), "~")
		}
	}
	for _, pattern := range secretPatterns {
		text = pattern.ReplaceAllString(text, "[REDACTED]")
	}
	text = jwsPattern.ReplaceAllStringFunc(text, func(s string) string {
		if strings.Trim(s, "0123456789.") == "" {
			return s
		} // Numeric versions are not JWS headers.
		return "[REDACTED]"
	})
	return text
}
