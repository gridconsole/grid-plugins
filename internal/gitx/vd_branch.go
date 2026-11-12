package gitx

// CurrentBranch returns the branch HEAD points at in the repo holding dir. It
// answers false outside a repo and on a detached HEAD.
func CurrentBranch(dir string) (string, bool) {
	out, err := run(dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}

// MergeBase returns the commit sha that a and b branched from. It answers false
// outside a repo, when a ref does not exist, or when the two share no history.
func MergeBase(dir, a, b string) (string, bool) {
	out, err := run(dir, "merge-base", a, b)
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}
