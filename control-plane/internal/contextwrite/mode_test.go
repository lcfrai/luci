package contextwrite_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryWriteIsShared is TestFileIsWorldReadable's half that runs on
// Windows too, where NTFS has no 0o644 to observe and that test skips
// (mutation P1): every statepath.WriteAtomic in this package's source passes
// statepath.Shared, so the brief is never published 0o600 - unreadable to
// the sandbox's user through the bind mount (task 01).
func TestEveryWriteIsShared(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	calls := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSel(call.Fun, "statepath", "WriteAtomic") {
				return true
			}
			calls++
			if len(call.Args) != 3 || !isSel(call.Args[2], "statepath", "Shared") {
				t.Errorf("%s: statepath.WriteAtomic without statepath.Shared", fset.Position(call.Pos()))
			}
			return true
		})
	}
	if calls == 0 {
		t.Fatal("no statepath.WriteAtomic call found: the guard reads the wrong files")
	}
}

func isSel(e ast.Expr, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != name {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == pkg
}
