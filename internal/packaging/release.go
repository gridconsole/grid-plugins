package packaging

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CommitVersion is the version a publish stamps into the plugin: 0.1.<commit count>.
// Every new commit raises it, so `claude plugin update` sees the release as newer.
func CommitVersion(commits int) string {
	return fmt.Sprintf("0.1.%d", commits)
}

// ReleaseVersion counts the commits reachable from HEAD in the repository at root.
// A shallow clone counts only the commits it holds, so publish from a full clone.
// make dist reads this once and Build stamps the result into every manifest.
func ReleaseVersion(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "rev-list", "--count", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("release version: git rev-list --count HEAD in %s: %w", root, err)
	}
	commits, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || commits < 1 {
		return "", fmt.Errorf("release version: cannot read the commit count %q", strings.TrimSpace(string(out)))
	}
	return CommitVersion(commits), nil
}
