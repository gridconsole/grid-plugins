package gridhome

import (
	"os"
	"path/filepath"
	"testing"
)

// linkedHome makes a real folder in one temp folder and a link to it in another,
// the way a home reached through /var -> /private/var is. It skips when the link
// cannot be made (Windows without the right).
func linkedHome(t *testing.T) (real, link string) {
	t.Helper()
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	return real, link
}

func TestTildeUnderALinkedHome(t *testing.T) {
	real, link := linkedHome(t)
	other := filepath.Join(filepath.Dir(real), "elsewhere")
	cases := []struct{ name, path, home, want string }{
		{"resolved path under linked home", filepath.Join(real, "x"), link, "~/x"},
		{"resolved home itself", real, link, "~"},
		{"literal path under linked home", filepath.Join(link, "x"), link, "~/x"},
		{"literal home itself", link, link, "~"},
		{"path under neither", filepath.Join(other, "x"), link, filepath.Join(other, "x")},
		{"empty home", filepath.Join(real, "x"), "", filepath.Join(real, "x")},
		{"sibling with a shared name prefix", real + "-other" + string(filepath.Separator) + "x", link, real + "-other" + string(filepath.Separator) + "x"},
		{"deeper resolved path", filepath.Join(real, "a", "b"), link, "~/a/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Tilde(c.path, c.home); got != c.want {
				t.Fatalf("Tilde(%q, %q) = %q, want %q", c.path, c.home, got, c.want)
			}
		})
	}
}
