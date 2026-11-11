package fslock_test

import (
	"os"
	"strings"
	"testing"
)

func TestFcContractWindowsLocks(t *testing.T) {
	b, err := os.ReadFile("../../contract/README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(b)), " ")
	for _, stale := range []string{
		"not supported until the Windows Feed is decided",
		"refuses operations requiring locks",
	} {
		if strings.Contains(text, stale) {
			t.Errorf("contract/README.md still holds stale phrase %q", stale)
		}
	}
	for _, want := range []string{"`flock` on Unix and `LockFileEx` on Windows", "compiled by `make cross`"} {
		if !strings.Contains(text, want) {
			t.Errorf("contract/README.md lacks %q", want)
		}
	}
}
