package gitx

// HeadSHA returns the full commit sha HEAD points at in the repo holding dir. It
// answers false outside a repo and in a repo with no commit yet.
func HeadSHA(dir string) (string, bool) {
	out, err := run(dir, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}
