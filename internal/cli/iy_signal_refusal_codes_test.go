package cli

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/signal"
)

// TestIySigCLI runs refused signal verbs with and without --json: the exit
// and stderr do not change, and stdout is exactly error, message and code.
func TestIySigCLI(t *testing.T) {
	for _, tc := range []struct {
		name, status, role, message, code string
		args                              []string
	}{
		{"guest", "new", "guest", "guests cannot act on signals", CodeGuestRefused, actArgs("note")},                                                                          // signal.go:67
		{"wake-new", "new", "", "this note is not snoozed", CodeNoteState, actArgs("wake")},                                                                                   // act.go:127
		{"note-done", "done", "", "this note is already decided (done)", CodeNoteState, actArgs("note")},                                                                      // act.go:133
		{"move-same-project", "new", "", "the note is already there", CodeNoteExists, []string{"signal", "move", actID, "--project", "example", "--to", "example", "--json"}}, // move.go:78
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(args []string) (int, string, string) {
				h, _, _ := actFixture(t, tc.status)
				h.env.Getenv = func(key string) string {
					if key == "GRID_ROLE" {
						return tc.role
					}
					return ""
				}
				exit := h.run(h.root, args...)
				return exit, h.out.String(), h.errb.String()
			}
			plainExit, plainOut, plainErr := run(tc.args[:len(tc.args)-1])
			exit, out, stderr := run(tc.args)
			if exit != 3 || plainExit != 3 {
				t.Fatalf("exit %d (plain %d), want 3: %s", exit, plainExit, stderr)
			}
			if stderr != plainErr || !strings.Contains(stderr, tc.message) {
				t.Fatalf("stderr %q, plain stderr %q", stderr, plainErr)
			}
			if plainOut != "" {
				t.Fatalf("plain stdout %q", plainOut)
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("stdout %q: %v", out, err)
			}
			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, []string{"code", "error", "message"}) || got["error"] != "refused" || got["message"] != tc.message || got["code"] != tc.code {
				t.Fatalf("stdout %s, want error refused, message %q, code %q", out, tc.message, tc.code)
			}
		})
	}
}

// iySigRefusedType reports whether a composite literal type is RefusedError
// or signal.RefusedError.
func iySigRefusedType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "RefusedError"
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		return ok && x.Name == "signal" && e.Sel.Name == "RefusedError"
	}
	return false
}

// iySigWalk returns the position of every RefusedError composite literal in
// file that has no Code field or an empty string Code, and how many it saw.
// A Code that is not a string literal (a constant) counts as set; the
// constants are checked by TestIySigConstants.
func iySigWalk(fset *token.FileSet, file *ast.File) (missing []string, seen int) {
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !iySigRefusedType(lit.Type) {
			return true
		}
		seen++
		var code ast.Expr
		for i, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Code" {
					code = kv.Value
				}
			} else if i == 1 { // unkeyed: Message, Code, Cause
				code = elt
			}
		}
		set := code != nil
		if bl, ok := code.(*ast.BasicLit); ok && bl.Kind == token.STRING {
			v, _ := strconv.Unquote(bl.Value)
			set = v != ""
		}
		if !set {
			missing = append(missing, fset.Position(lit.Pos()).String())
		}
		return true
	})
	return missing, seen
}

func TestIySigWalk(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		fset := token.NewFileSet()
		total := 0
		for _, root := range []string{filepath.Join("..", "signal"), filepath.Join("..", "cli")} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return err
				}
				file, err := parser.ParseFile(fset, path, nil, 0)
				if err != nil {
					return err
				}
				missing, seen := iySigWalk(fset, file)
				total += seen
				for _, at := range missing {
					t.Errorf("%s: RefusedError literal without a Code", at)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if total == 0 {
			t.Fatal("no RefusedError literal found; the walk reads nothing")
		}
	})
	t.Run("reports", func(t *testing.T) {
		const src = `package x

func f(n int) error {
	switch n {
	case 0:
		return &signal.RefusedError{Message: "a", Code: "c"}
	case 1:
		return &signal.RefusedError{Message: "b"}
	case 2:
		return &RefusedError{Message: "c", Code: ""}
	}
	return &RefusedError{Message: "d", Code: CodeNoteState}
}
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "iy.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		missing, seen := iySigWalk(fset, file)
		if want := []string{"iy.go:8:11", "iy.go:10:11"}; seen != 4 || !slices.Equal(missing, want) {
			t.Fatalf("walk saw %d, reported %v; want 4 and %v", seen, missing, want)
		}
	})
}

// TestIySigConstants keeps the internal/signal codes equal to their
// internal/cli twins and the four new codes in the README code table once.
func TestIySigConstants(t *testing.T) {
	for _, c := range []struct{ name, signal, cli string }{
		{"guest_refused", signal.CodeGuestRefused, CodeGuestRefused},
		{"note_state", signal.CodeNoteState, CodeNoteState},
		{"note_exists", signal.CodeNoteExists, CodeNoteExists},
		{"notes_full", signal.CodeNotesFull, CodeNotesFull},
		{"no_such_note", signal.CodeNoSuchNote, CodeNoSuchNote},
		{"no_such_task", signal.CodeNoSuchTask, CodeNoSuchTask},
		{"no_such_project", signal.CodeNoSuchProject, CodeNoSuchProject},
	} {
		if c.signal != c.name || c.cli != c.name {
			t.Errorf("%s: signal %q, cli %q", c.name, c.signal, c.cli)
		}
	}
	doc := hfFsDocCodes(t)
	for _, code := range []string{CodeGuestRefused, CodeNoteState, CodeNoteExists, CodeNotesFull} {
		count := 0
		for _, c := range doc {
			if c == code {
				count++
			}
		}
		if count != 1 {
			t.Errorf("README code table lists %q %d times, want once", code, count)
		}
	}
}
