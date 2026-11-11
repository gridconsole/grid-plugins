package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// jzQwFeedLiterals parses one Go source and returns the feed.Store composite
// literals in it with at least one key, as "file:line", split into those that
// name a Warn key and those that do not. A zero-value feed.Store{} is not a
// store (it is the error return) and is skipped. The package may be imported
// under another name; a file that does not import internal/feed has none.
func jzQwFeedLiterals(t *testing.T, fset *token.FileSet, name string, src any) (keyed, missing []string) {
	t.Helper()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	local := ""
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasSuffix(path, "/internal/feed") {
			continue
		}
		local = "feed"
		if imp.Name != nil {
			local = imp.Name.Name
		}
	}
	if local == "" {
		return nil, nil
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || len(lit.Elts) == 0 {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Store" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != local {
			return true
		}
		at := fset.Position(lit.Pos())
		where := filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
		keyed = append(keyed, where)
		for _, e := range lit.Elts {
			if kv, ok := e.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Warn" {
					return true
				}
			}
		}
		missing = append(missing, where)
		return true
	})
	return keyed, missing
}

// TestJzQwEveryStoreLiteralSetsWarn walks every non-test file of this package:
// a feed.Store literal with keys that has no Warn key would print the fold's
// withdrawal-policy warning on the host's stderr instead of the call's.
func TestJzQwEveryStoreLiteralSetsWarn(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	seen := 0
	var missing []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		keyed, miss := jzQwFeedLiterals(t, fset, f, src)
		seen += len(keyed)
		missing = append(missing, miss...)
	}
	// The walk must see the literals it is about: nine on the base of this lane.
	if seen < 9 {
		t.Fatalf("the walk saw %d keyed feed.Store literals in internal/cli, want at least 9: it is not reading them", seen)
	}
	sort.Strings(missing)
	for _, where := range missing {
		t.Errorf("%s: feed.Store literal has no Warn key; set Warn: env.Stderr", where)
	}
}

// TestJzQwWalkTellsLiterals proves the walk on small sources: a keyed literal
// without Warn is named, one with Warn passes, a zero-value one is skipped, and
// another import name or another type called Store is judged correctly.
func TestJzQwWalkTellsLiterals(t *testing.T) {
	const head = "package p\nimport f \"github.com/gridconsole/grid-plugins/internal/feed\"\nimport \"other/internal/lease\"\nvar _ = lease.Store{}\n"
	cases := []struct {
		name, body     string
		keyed, missing []string
	}{
		{"no Warn", "var _ = f.Store{TaskDir: \"d\"}", []string{"x.go:5"}, []string{"x.go:5"}},
		{"Warn", "var _ = f.Store{TaskDir: \"d\", Warn: nil}", []string{"x.go:5"}, nil},
		{"address of", "var _ = &f.Store{TaskDir: \"d\"}", []string{"x.go:5"}, []string{"x.go:5"}},
		{"zero value", "var _ = f.Store{}", nil, nil},
		{"another Store", "var _ = lease.Store{TaskDir: \"d\"}", nil, nil},
	}
	for _, c := range cases {
		keyed, missing := jzQwFeedLiterals(t, token.NewFileSet(), "x.go", head+c.body)
		if strings.Join(keyed, ",") != strings.Join(c.keyed, ",") || strings.Join(missing, ",") != strings.Join(c.missing, ",") {
			t.Errorf("%s: keyed %v missing %v, want keyed %v missing %v", c.name, keyed, missing, c.keyed, c.missing)
		}
	}
}
