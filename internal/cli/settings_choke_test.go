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
		if filepath.Dir(path) == filepath.Join(root, "internal", "settings") {
			for _, problem := range settingsPackageBypasses(filepath.Base(path), f) {
				t.Errorf("%s: %s", path, problem)
			}
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

// The package itself cannot import settings, so inspect its unqualified Store
// references too. Methods operate on the store supplied by the caller-aware
// constructor; the sole internal construction is the read-only hook adapter.
func settingsPackageBypasses(file string, f *ast.File) []string {
	var problems []string
	allowedReads := map[*ast.Ident]bool{}
	exports := map[string]string{
		"AgentNames": "catalog.go", "SkillNames": "catalog.go", "HookNames": "catalog.go",
		"ParseAnswers": "setup.go", "ReadHookLevels": "store.go", "ResolveHookLevel": "store.go",
		"ClaudeBlock": "claude.go", "ClaudeSetupCommand": "claude.go", "RunClaudeSetup": "claude.go", "claudeInvalid.Error": "claude.go",
		"ClaudeSettingsPath": "claude.go", "ReadClaudeAllowStatus": "claude_status.go", // Read-only setup diagnostics; no Store access.
		"AgentTemplatesPath": "agentsync.go", "ClaudeAgentsDir": "agentsync.go", "SyncAgents": "agentsync.go", // Agent files: SyncAgents reads the personal overlay and writes only ~/.claude/agents.
		"claudeReadError.Error": "claude.go", "claudeReadError.Unwrap": "claude.go", // Private read errors; no Store access.
		// Budget and Claude settings rows: the budget runs on a Store the CLI got from settingsStore
		// (Store.WithBudget); ClaudeSet and ClaudeShow touch the person's Claude files, not the Store.
		"Store.SetAgents": "store.go", "Store.WithBudget": "store.go", "BuiltinBudget": "budget.go", "ReadBudgetNote": "budget.go",
		"BudgetShow": "budget.go", "BudgetPreviewFor": "budget.go", "BudgetApply": "budget.go", "BudgetUndo": "budget.go",
		"ClaudeShow": "claude_user.go", "ClaudeSet": "claude_user.go", "ClaudeRowNames": "claude_user.go", "AdvisorBlocked": "claude_user.go",
		"CodedError.Error": "claude_user.go", "CodedError.Unwrap": "claude_user.go",
		"AgentForRole": "role.go", "Store.ModelFor": "role.go", // Read-only: the model and effort a spawned Job of a role runs on.
		"Store.Effective": "store.go", "Store.SetAgent": "store.go", "Store.SetSkill": "store.go", "Store.SetHook": "store.go", "Store.RecordSetup": "setup.go",
	}
	for _, decl := range f.Decls {
		var inspect ast.Node = decl
		if fn, ok := decl.(*ast.FuncDecl); ok {
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				recv := fn.Recv.List[0].Type
				if ptr, ok := recv.(*ast.StarExpr); ok {
					recv = ptr.X
				}
				if id, ok := recv.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			if ast.IsExported(fn.Name.Name) && exports[name] != file {
				problems = append(problems, "unreviewed settings export "+name)
			}
			// Inspect the signature without its receiver, and the body. A method may
			// use its supplied receiver, but cannot construct or return another Store.
			inspect = &ast.FuncLit{Type: fn.Type, Body: fn.Body}
			if (file == "store.go" && name == "ReadHookLevels" || file == "agentsync.go" && name == "SyncAgents") && fn.Recv == nil {
				// Allow only Store{...}.Effective(), never a local store that could write.
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Effective" {
						return true
					}
					expr := sel.X
					if paren, ok := expr.(*ast.ParenExpr); ok {
						expr = paren.X
					}
					lit, ok := expr.(*ast.CompositeLit)
					if !ok {
						return true
					}
					if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "Store" {
						allowedReads[id] = true
					}
					return true
				})
			}
		}
		ast.Inspect(inspect, func(n ast.Node) bool {
			if spec, ok := n.(*ast.ValueSpec); ok {
				for _, name := range spec.Names {
					if ast.IsExported(name.Name) {
						problems = append(problems, "unreviewed settings binding "+name.Name)
					}
				}
			}
			if spec, ok := n.(*ast.TypeSpec); ok && file == "store.go" && spec.Name.Name == "Store" {
				return false
			}
			if id, ok := n.(*ast.Ident); ok && id.Name == "Store" && !allowedReads[id] {
				problems = append(problems, "Store bypasses settingsStore resolver adapter")
			}
			return true
		})
	}
	return problems
}

func TestSettingsPackageChokeMutations(t *testing.T) {
	for _, source := range []string{
		`func Write(home string) error { return (Store{Home:home}).SetHook("home-allow", "off") }`,
		`func hidden(home string) error { var s Store; s.Home = home; return s.SetSkill("research", false) }`,
		`type Alias = Store`,
		`var store = new(Store)`,
		`func (s Store) Bypass() error { return nil }`,
		`func WriteDirect(path string) error { return os.WriteFile(path, nil, 0600) }`,
		`var Write = func(path string) error { return os.WriteFile(path, nil, 0600) }`,
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "probe.go", "package settings\n"+source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(settingsPackageBypasses("probe.go", f)) == 0 {
			t.Errorf("missed bypass: %s", source)
		}
	}
}
