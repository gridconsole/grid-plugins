package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSettingsStoreChokePoint(t *testing.T) {
	// Every production reference to settings.Store must be in the resolver
	// adapter. Inspect imports as well so renaming the package cannot bypass it.
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			pkg, err := strconv.Unquote(imp.Path.Value)
			if err != nil || pkg != "github.com/gridconsole/grid-plugins/internal/settings" {
				continue
			}
			alias := "settings"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if alias == "." {
				t.Errorf("%s: dot import hides settings.Store access", path)
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && path == filepath.Join(root, "internal", "cli", "settings.go") && fn.Name.Name == "settingsStore" {
					continue // Shared caller-aware resolver adapter, exercised below.
				}
				ast.Inspect(decl, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "Store" {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias {
							t.Errorf("%s: settings.Store bypasses settingsStore resolver adapter", path)
						}
					}
					return true
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Source restrictions alone cannot catch a branch that skips Resolve inside
	// the adapter itself. Exercise default writes and unregistered read fallback.
	h, cwd := settingsAccessFixture(t, "guest")
	unregistered := filepath.Join(h.root, "unregistered")
	h.mkdir(unregistered)
	for _, dir := range []string{cwd, unregistered, h.home} {
		h.env.Cwd = dir
		for _, listing := range []bool{false, true} {
			if listing && dir == cwd {
				continue // A registered project's effective settings remain readable.
			}
			for _, project := range []string{"", "personal"} {
				if _, err := settingsStore(h.env, project, listing); err == nil {
					t.Errorf("settingsStore(project=%q, listing=%t) bypassed Personal resolution", project, listing)
				}
			}
		}
	}
}
