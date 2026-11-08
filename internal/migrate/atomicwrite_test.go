package migrate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration writes its journal, receipt and pointer files through internal/atomicfile, the
// one atomic-write helper; it does not borrow another package's copy or keep its own.
func TestAtomicWritesUseAtomicfile(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "task")} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					if n.Sel.Name == "AtomicWrite" {
						t.Errorf("%s: calls an AtomicWrite helper; use atomicfile.Write", file)
					}
				case *ast.FuncDecl:
					if n.Name.Name == "AtomicWrite" {
						t.Errorf("%s: exports AtomicWrite; internal/atomicfile is the only atomic-write helper", file)
					}
				}
				return true
			})
		}
	}
}
