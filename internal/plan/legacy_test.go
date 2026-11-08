package plan_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func legacyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "legacy", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeLegacyFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeLegacyGolden(t *testing.T) {
	for _, tc := range []struct{ name, form string }{
		{"form-a", "A"}, {"form-b", "B"}, {"form-b-revised", "B"}, {"form-c", "C"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, form, err := plan.Upgrade(legacyFixture(t, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			if form != tc.form {
				t.Fatalf("form %q, want %q", form, tc.form)
			}
			want := legacyFixture(t, tc.name+".expected")
			if got != want {
				t.Fatalf("converted bytes differ:\n%s\nwant:\n%s", got, want)
			}
			if _, err := plan.Items(got); err != nil {
				t.Fatal(err)
			}
			again, form, err := plan.Upgrade(got)
			if err != nil || again != got || form != "current" {
				t.Fatalf("not idempotent: %s %v", form, err)
			}
		})
	}
}

func TestUpgradeCurrentUnchanged(t *testing.T) {
	for _, text := range []string{"", legacyFixture(t, "form-b.expected"), strings.ReplaceAll(legacyFixture(t, "form-c.expected"), "\n", "\r\n")} {
		out, form, err := plan.Upgrade(text)
		if err != nil || form != "current" || out != text {
			t.Fatalf("current changed: %s %v", form, err)
		}
	}
}

func TestUpgradeLegacyErrors(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"Job without item", strings.Replace(legacyFixture(t, "form-c"), "item: 1", "item: 2", 1), "line 31"},
		{"Job without section", strings.Replace(legacyFixture(t, "form-c"), "## Step 1 {#step-1}", "## Notes", 1), "line 3"},
		{"unknown step field", "# Plan\n- [ ] 1 One\n  mystery: value\n", "line 3"},
		{"mixed current", "# Plan\n- [ ] 1 One\n  verify: sentence\n  jobs: []\n", "line 4"},
		{"malformed YAML", "# Plan\n- [ ] 1 One\n  write: [broken\n", "line 3"},
		{"blank in step", "# Plan\n- [ ] 1 One\n  First paragraph.\n\n  Second paragraph.\n  verify: inspect it\n", "line 4"},
		{"orphan field", "# Plan\n  write: []\n", "line 2"},
		{"unknown front matter", strings.Replace(legacyFixture(t, "form-c"), "items:", "extra: true\nitems:", 1), "line 2"},
		{"mixed front matter", strings.Replace(legacyFixture(t, "form-c"), "## Step 1 {#step-1}", "- [ ] 1 One", 1), "line 31"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := plan.Upgrade(tc.text)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v, want %s", err, tc.want)
			}
		})
	}
}

func TestUpgradeLegacyOptionalProse(t *testing.T) {
	text := "# Plan\n- [ ] 7 One\n  host: local\n  verify: inspect it\n  needs: none\n  evidence: checks/proof.txt\n\nVerify: It works.\n"
	out, _, err := plan.Upgrade(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"  host: local", "  Verified by: inspect it", "  needs: none", "  evidence: checks/proof.txt", "  jobs: []", "## Verify\n\nIt works."} {
		if !strings.Contains(out, line) {
			t.Errorf("lost %q", line)
		}
	}
}

func TestUpgradeLegacyJobFields(t *testing.T) {
	old := legacyFixture(t, "form-c")
	doc, err := frontmatter.Parse(old)
	if err != nil {
		t.Fatal(err)
	}
	var expected job.Job
	raw := doc.Data["items"].([]any)[0]
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	expected.Kind = "non-code"
	expected.Defaults()
	converted, _, err := plan.Upgrade(old)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeLegacyFile(t, filepath.Join(dir, "plan.md"), converted)
	actual, err := job.LoadPlan(dir, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("got %+v\nwant %+v", actual, expected)
	}
}

func TestLegacyDiffArchiveImmutable(t *testing.T) {
	dir := t.TempDir()
	archiveDir := filepath.Join(dir, "plan-revisions")
	if err := os.Mkdir(archiveDir, 0700); err != nil {
		t.Fatal(err)
	}
	before := strings.Replace(legacyFixture(t, "form-b"), "rev 3", "rev 1", 1)
	after := strings.Replace(legacyFixture(t, "form-b-revised.expected"), "rev 4", "rev 2", 1)
	archive := filepath.Join(archiveDir, "1.md")
	writeLegacyFile(t, archive, before)
	writeLegacyFile(t, filepath.Join(dir, "plan.md"), after)
	store := plan.Store{TaskDir: dir}
	for _, tc := range []struct{ from, to string }{{"1", "2"}, {"2", "1"}, {"1", "1"}} {
		t.Run(tc.from+" to "+tc.to, func(t *testing.T) {
			diff, err := store.Diff(tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if tc.from != tc.to && len(diff.Changed) == 0 {
				t.Fatal("missing changes")
			}
			data, err := os.ReadFile(archive)
			if err != nil || string(data) != before {
				t.Fatal("archive changed")
			}
		})
	}
}

func TestLegacyWithHash(t *testing.T) {
	data, err := os.ReadFile("testdata/hashes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Text, Hash string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tc := range cases {
		if !strings.HasPrefix(tc.Name, "migrated ") {
			continue
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			name := strings.TrimPrefix(tc.Name, "migrated ")
			text, _, err := plan.Upgrade(legacyFixture(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if text != tc.Text {
				t.Fatal("hash corpus text differs from converted fixture")
			}
			dir := t.TempDir()
			writeLegacyFile(t, filepath.Join(dir, "plan.md"), text)
			if err := (plan.Store{TaskDir: dir}).WithHash(func(hash string) error {
				if hash != tc.Hash {
					t.Errorf("hash %s, want %s", hash, tc.Hash)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 3 {
		t.Fatalf("found %d migrated hash cases", count)
	}
}

func TestUpgradeLegacyProse(t *testing.T) {
	for _, verification := range []string{"inspect it", "inspect it\n    and check the result", "inspect it\n  and check the result"} {
		t.Run(verification, func(t *testing.T) {
			old := "# Plan\n- [ ] 1 One\n  write: [docs/example.md]\n  verify: " + verification + "\n- [ ] 2 Two\n  verify: " + verification + "\n- [ ] 3 Three\n  Only prose.\n"
			out, form, err := plan.Upgrade(old)
			if err != nil || form != "B" {
				t.Fatalf("form %q: %v", form, err)
			}
			items, err := plan.Items(out)
			if err != nil || len(items) != 3 || len(items[0].Jobs) != 1 || len(items[1].Jobs) != 0 || len(items[2].Jobs) != 0 {
				t.Fatalf("items: %+v, %v", items, err)
			}
			want := "Verified by: " + strings.ReplaceAll(verification, "\n    ", "\n  ")
			want = strings.ReplaceAll(want, "\n  ", "\n")
			if items[0].Prose != want {
				t.Fatalf("prose %q, want %q", items[0].Prose, want)
			}
			dir := t.TempDir()
			writeLegacyFile(t, filepath.Join(dir, "plan.md"), out)
			// A prose-only step has no executable Job, including when wrapped.
			if _, err := job.LoadPlan(dir, "2"); err == nil || !strings.Contains(err.Error(), `expected one item "2", found 0`) {
				t.Fatalf("prose became an executable Job: %v", err)
			}
		})
	}
}

func TestUpgradeLegacyFencedVerify(t *testing.T) {
	for _, fence := range []string{"```", "~~~~"} {
		old := "# Plan\n- [ ] 1 One\n  verify: inspect it\n\n" + fence + "text\nVerify: leave this example alone\n" + fence + "\n\nVerify: the real verification\n"
		out, _, err := plan.Upgrade(old)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, fence+"text\nVerify: leave this example alone\n"+fence) || !strings.Contains(out, "## Verify\n\nthe real verification") {
			t.Fatalf("fence changed: %s", out)
		}
	}
}

func TestUpgradeLegacyCRLF(t *testing.T) {
	for _, name := range []string{"form-a", "form-b", "form-c"} {
		out, _, err := plan.Upgrade(strings.ReplaceAll(legacyFixture(t, name), "\n", "\r\n"))
		if err != nil || out != legacyFixture(t, name+".expected") {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	}
}
