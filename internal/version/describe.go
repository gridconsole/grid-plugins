package version

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	describeTag = regexp.MustCompile(`^(v?)(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+?)??(?:-(\d+)-g([0-9a-f]+))?(-dirty)?$`)
	describeSHA = regexp.MustCompile(`^([0-9a-f]{4,})(-dirty)?$`)
)

// FromDescribe turns `git describe --tags --always --dirty` output into a version Compare
// accepts. A bare sha becomes 0.0.0-dev+<sha>. Commits after a release tag become the next
// patch's pre-release (v0.2.0-3-gabc to v0.2.1-dev.3+abc), which sorts above the tag and
// below the next release; after a pre-release tag they append dev.N. Build metadata does
// not affect precedence.
func FromDescribe(describe string) (string, error) {
	if m := describeSHA.FindStringSubmatch(describe); m != nil {
		return "0.0.0-dev+" + m[1] + dirtyMeta(m[2]), nil
	}
	m := describeTag.FindStringSubmatch(describe)
	if m == nil {
		return "", fmt.Errorf("version: cannot read git describe output %q", describe)
	}
	v, pre, commits, sha, dirty := m[1], m[5], m[6], m[7], m[8]
	if commits == "" && dirty == "" {
		return describe, nil
	}
	if commits == "" {
		commits, sha = "0", "dirty"
		dirty = ""
	}
	core := m[2] + "." + m[3] + "."
	if pre == "" {
		patch, err := strconv.Atoi(m[4])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%s%d-dev.%s+%s%s", v, core, patch+1, commits, sha, dirtyMeta(dirty)), nil
	}
	return fmt.Sprintf("%s%s%s%s.dev.%s+%s%s", v, core, m[4], pre, commits, sha, dirtyMeta(dirty)), nil
}

func dirtyMeta(dirty string) string {
	if dirty == "" {
		return ""
	}
	return ".dirty"
}
