package version

import (
	"errors"
	"strconv"
	"testing"
)

// describeTag (describe.go:10) is
//
//	^(v?)(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+?)??(?:-(\d+)-g([0-9a-f]+))?(-dirty)?$
//
// so "v1.2.99999999999999999999-3-gabc1234" matches with patch
// 99999999999999999999, 3 commits and sha abc1234; strconv.Atoi then fails.
func TestFromDescribePatchPastIntRange(t *testing.T) {
	const input = "v1.2.99999999999999999999-3-gabc1234"
	if describeTag.FindStringSubmatch(input) == nil {
		t.Fatalf("describeTag does not match %q", input)
	}
	got, err := FromDescribe(input)
	if got != "" || err == nil {
		t.Fatalf("FromDescribe(%q) = %q, %v; want \"\" and an error", input, got, err)
	}
	if !errors.Is(err, strconv.ErrRange) {
		t.Fatalf("FromDescribe error = %v, want a strconv range error", err)
	}
}
