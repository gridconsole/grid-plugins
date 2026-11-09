package doctor

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)-----BEGIN [A-Za-z0-9 /_-]+-----.*?(?:-----END [A-Za-z0-9 /_-]+-----|\z)`),
	regexp.MustCompile(`(?i)\bbearer[ \t\r\n]+[^\s"'<>;,]+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`),
}

// JSON JWS headers start with eyJ. Requiring that prefix preserves ordinary
// hostnames and dotted filenames while still covering detached signatures.
var jwsPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]{2,}`)

// Redact removes support-bundle secrets and emails and replaces the user's home
// with ~. Explicit private paths (such as the Claude settings file) are removed
// before home substitution. Windows paths match case and separator variants.
func Redact(text, home string, privatePaths ...string) string {
	for _, path := range privatePaths {
		if path != "" {
			text = regexp.MustCompile(redactPathPattern(path)).ReplaceAllString(text, `[REDACTED]${1}`)
		}
	}
	if home != "" {
		text = regexp.MustCompile(redactPathPattern(home)).ReplaceAllString(text, `~${1}`)
	}
	for _, pattern := range secretPatterns {
		text = pattern.ReplaceAllString(text, "[REDACTED]")
	}
	return jwsPattern.ReplaceAllString(text, "[REDACTED]")
}

func redactPathPattern(home string) string {
	pattern := regexp.QuoteMeta(home)
	normalized := strings.ReplaceAll(home, `\`, "/")
	if len(normalized) >= 3 && normalized[1:3] == ":/" {
		variants := []string{normalized, "/" + normalized[:1] + normalized[2:]}
		for i, variant := range variants {
			parts := strings.Split(variant, "/")
			for j, part := range parts {
				parts[j] = regexp.QuoteMeta(part)
			}
			variants[i] = strings.Join(parts, `(?:\\{1,2}|/)`)
		}
		pattern = `(?i)(?:` + strings.Join(variants, "|") + `)`
	}
	// Preserve the delimiter without matching a longer filename or home.
	return pattern + `($|[^A-Za-z0-9_-])`
}
