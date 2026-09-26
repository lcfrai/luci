package backend_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// TestSeamBackendIsTheSevenMethodsBothImplementationsServe pins seam S8.1 in
// docs/SEAMS.md: the interface Phase 8's Kata backend plugs into, exactly as
// Phase 2 shipped it. dockerbe and katabe assert at compile time that they
// satisfy it; this pins the other direction — that the interface itself did not
// grow, shrink or change a signature while both implementations changed to
// match. If this fails because the seam was meant to change, change the want
// below and the S8.1 row in docs/SEAMS.md in the same commit.
func TestSeamBackendIsTheSevenMethodsBothImplementationsServe(t *testing.T) {
	want := map[string]string{
		"Attach":  "func(context.Context, string, string) (*websocket.Conn, error)",
		"Close":   "func() error",
		"Create":  "func(context.Context, backend.Spec) (backend.Sandbox, error)",
		"Destroy": "func(context.Context, string) error",
		"Get":     "func(context.Context, string) (backend.Sandbox, error)",
		"Kind":    "func() backend.Kind",
		"List":    "func(context.Context) ([]backend.Sandbox, error)",
	}
	typ := reflect.TypeOf((*backend.Backend)(nil)).Elem()
	got := make(map[string]string, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		got[m.Name] = m.Type.String()
	}
	if !reflect.DeepEqual(got, want) {
		names := make([]string, 0, len(got))
		for name := range got {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Fatalf("backend.Backend moved: seam S8.1 (docs/SEAMS.md) no longer matches.\nmethods now: %v\n got: %v\nwant: %v", names, got, want)
	}

	// Type.String() names a type by its package's NAME, not its import path, so
	// "*websocket.Conn" reads the same from coder/websocket, nhooyr.io/websocket or
	// gorilla/websocket. Without this, swapping the library under Attach — with both
	// implementations changed to match — passes the check above (mutant G1).
	wantPkg := map[string]string{
		"backend.Kind":    backendPkg,
		"backend.Sandbox": backendPkg,
		"backend.Spec":    backendPkg,
		"context.Context": "context",
		"websocket.Conn":  "github.com/coder/websocket",
	}
	gotPkg := map[string]string{}
	for i := 0; i < typ.NumMethod(); i++ {
		namedIn(typ.Method(i).Type, gotPkg)
	}
	if !reflect.DeepEqual(gotPkg, wantPkg) {
		t.Fatalf("backend.Backend moved: seam S8.1 (docs/SEAMS.md) names a type from another package.\n got: %v\nwant: %v", gotPkg, wantPkg)
	}
}

// backendPkg is the import path of package backend, where Spec, Sandbox and Kind live.
const backendPkg = "github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"

// namedIn records, by printed name, the import path of every named type t mentions.
// Predeclared types (error, string) have no import path and are skipped.
func namedIn(t reflect.Type, into map[string]string) {
	if t.PkgPath() != "" {
		into[t.String()] = t.PkgPath()
		return
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		namedIn(t.Elem(), into)
	case reflect.Map:
		namedIn(t.Key(), into)
		namedIn(t.Elem(), into)
	case reflect.Func:
		for i := 0; i < t.NumIn(); i++ {
			namedIn(t.In(i), into)
		}
		for i := 0; i < t.NumOut(); i++ {
			namedIn(t.Out(i), into)
		}
	}
}

// TestSeamBackendKindsAreTheTwoPersistedStrings pins the other half of S8.1.
// backend.Kind's doc comment says these strings are persisted in the registry
// and returned over REST, so renaming one strands every session recorded under it.
func TestSeamBackendKindsAreTheTwoPersistedStrings(t *testing.T) {
	if backend.KindDocker != "docker" || backend.KindKata != "kata" {
		t.Fatalf("Kind strings are %q and %q, want \"docker\" and \"kata\" (seam S8.1, docs/SEAMS.md)", backend.KindDocker, backend.KindKata)
	}

	// And there are exactly two. Reflection cannot list constants, so read them from
	// the package's own non-test source: every const declared with type Kind, or
	// as Kind(...). A third is a third backend, which is changing S8.1 (mutant G2).
	want := map[string]string{"KindDocker": `"docker"`, "KindKata": `"kata"`}
	if got := kindConstants(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("package backend's Kind constants moved: seam S8.1 (docs/SEAMS.md) names two.\n got: %v\nwant: %v", got, want)
	}
}

// kindConstants maps each Kind constant declared in package backend to its source text.
func kindConstants(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("listing package backend's source: %v (%d files)", err, len(files))
	}
	fset := token.NewFileSet()
	got := map[string]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					var v ast.Expr
					if i < len(vs.Values) {
						v = vs.Values[i]
					}
					typed := isIdent(vs.Type, "Kind")
					if call, ok := v.(*ast.CallExpr); ok && isIdent(call.Fun, "Kind") && len(call.Args) == 1 {
						typed, v = true, call.Args[0]
					}
					if !typed {
						continue
					}
					if lit, ok := v.(*ast.BasicLit); ok {
						got[n.Name] = lit.Value
					} else {
						got[n.Name] = "(not a literal)"
					}
				}
			}
		}
	}
	return got
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}
