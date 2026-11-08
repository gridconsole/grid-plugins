package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHashGolden(t *testing.T) {
	// These expected values were produced by the original plan-hash.mjs.
	var cases []struct{ Name, Text, Hash string }
	if err := json.Unmarshal([]byte(fixture(t, "hashes.json")), &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if got := Hash(c.Text); got != c.Hash {
				t.Errorf("got %s, want %s", got, c.Hash)
			}
		})
	}
}

func TestHashStability(t *testing.T) {
	base := fixture(t, "plan.md")
	for name, changed := range map[string]string{
		"ticks":               strings.ReplaceAll(base, "[ ]", "[X]"),
		"CRLF":                strings.ReplaceAll(base, "\n", "\r\n"),
		"trailing whitespace": strings.ReplaceAll(base, "\n", " \t\n") + "\n\n",
		"status":              strings.ReplaceAll(base, "The store filter works. The toolbar and keyboard checks remain.", "Everything is ready."),
		"heading":             strings.ReplaceAll(base, "rev 3 · 000000", "rev 4 · abcdef"),
	} {
		t.Run(name, func(t *testing.T) {
			if Hash(base) != Hash(changed) || Digest(base) != Digest(changed) {
				t.Fatal("hash changed")
			}
		})
	}
	for name, changed := range map[string]string{
		"title":            strings.ReplaceAll(base, "tag filter", "topic filter"),
		"verification":     strings.ReplaceAll(base, "min_tests: 3", "min_tests: 4"),
		"after":            strings.ReplaceAll(base, "after: []", "after: [J-7]"),
		"internal blank":   strings.ReplaceAll(base, "## Risks\n", "## Risks\n\n"),
		"indentation":      strings.ReplaceAll(base, "  needs:", "needs:"),
		"outside evidence": base + "evidence: checks/R-3/proof.txt\n",
	} {
		t.Run(name, func(t *testing.T) {
			if Hash(base) == Hash(changed) || Digest(base) == Digest(changed) {
				t.Fatal("hash did not change")
			}
		})
	}
}

func TestItemsGolden(t *testing.T) {
	items, err := Items(fixture(t, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	items = WithLegacyState(items, fixture(t, "items.md"), []string{"---\nstatus: running\nitem: 2, 9\n---\nBuild the toolbar."})
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "plan-items", string(b)+"\n")
}

func TestItemsRefusals(t *testing.T) {
	for _, text := range []string{
		"- [ ] 1 One\n- [x] 01 Duplicate",
		"- [ ] 0 Zero",
		"- [ ] 9999999999999999999999999 Overflow",
		"- [ ] 1 One\n  write: [broken",
		"- [ ] 1 One\n  review: L1\n  review: L2",
		"- [ ] 1 One\n  verify: {command: one, command: two}",
	} {
		if _, err := Items(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

func TestItemsBoundaries(t *testing.T) {
	items, err := Items("# Plan\n- [X] 7 **Keep** `title`\n  Some prose.\n  jobs: []\n\n  - [ ] 8 Nested\n- [ ] 9 Last\n  jobs: []\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].N != 7 || items[0].Title != "Keep title" || items[0].State != "open" || items[0].Prose != "Some prose." {
		t.Fatalf("%+v", items)
	}
}

func TestDigestUsesFullCanonicalSHA256(t *testing.T) {
	text := "# Plan, rev 9 · ignored\r\n\r\n- [X] 1 Build it  \r\n  evidence: proof\r\n\r\n## Where it stands\r\nVolatile status.\r\n"
	sum := sha256.Sum256([]byte("- [ ] 1 Build it\n  evidence: proof\n"))
	want := hex.EncodeToString(sum[:])
	if got := Digest(text); got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}

func TestDigestIncludesStepMetadata(t *testing.T) {
	for _, section := range []string{"## Steps", "## Where it stands"} {
		for _, field := range []string{"evidence", "needs"} {
			t.Run(section+"/"+field, func(t *testing.T) {
				base := "# Plan\n" + section + "\n- [ ] 1 Build it\n  " + field + ": original\n"
				changed := strings.Replace(base, ": original", ": injected", 1)
				if Digest(base) == Digest(changed) {
					t.Fatal("worker-visible step metadata did not change digest")
				}
				if Hash(base) != Hash(changed) {
					t.Fatal("legacy short hash compatibility changed")
				}
			})
		}
	}
}
