package backend_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
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

	// And there are exactly two, in the whole control plane. Reflection cannot list
	// constants, so type-check every package's non-test source and take every
	// constant whose type is Kind, however it is spelled and wherever it is
	// declared. A third is a third backend, which is changing S8.1 (mutant G2), and
	// a third backend's own package is where it would be declared (task 26 review 2
	// m5, mutant G-R2: `const KindKataWarm backend.Kind = "kata-warm"` in katabe).
	want := map[string]string{"internal/backend.KindDocker": `"docker"`, "internal/backend.KindKata": `"kata"`}
	named, values := kindConstants(t)
	if !reflect.DeepEqual(named, want) {
		t.Fatalf("the control plane's Kind constants moved: seam S8.1 (docs/SEAMS.md) names two.\n got: %v\nwant: %v", named, want)
	}
	// Nor a third value with no name: `backend.Kind("kata-warm")`, or an untyped
	// "kata-warm" assigned to or compared with a Kind, is the same third backend.
	var strays []string
	for at, v := range values {
		if v != want["internal/backend.KindDocker"] && v != want["internal/backend.KindKata"] {
			strays = append(strays, at+": "+v)
		}
	}
	if sort.Strings(strays); len(strays) > 0 {
		t.Fatalf("a Kind constant outside the two (seam S8.1, docs/SEAMS.md):\n%s", strings.Join(strays, "\n"))
	}
}

// kindConstants type-checks every package of the control plane's non-test Go.
// named maps each constant of type Kind, by package directory and name, to its
// value; values maps each constant expression of type Kind, by position, to its
// value. The type checker decides what has type Kind, not a reading of the
// syntax: a bare name repeating the previous spec in a const group, `const X =
// KindKata`, a constant of an alias of Kind and an untyped "x" that becomes a
// Kind are each a Kind by the language's rules, and a walk of the syntax has to
// re-implement every one (task 26 review m1, mutants G-M2 and G-M3).
func kindConstants(t *testing.T) (named, values map[string]string) {
	t.Helper()
	root := filepath.Join("..", "..") // this package is control-plane/internal/backend
	module := strings.TrimSuffix(backendPkg, "/internal/backend")
	fset := token.NewFileSet()
	// An import inside the control plane is resolved by checking that package
	// here, first, so a Kind reached through another package's alias is a Kind.
	// An import outside it is not resolved: no Kind constant needs one, and
	// resolving coder/websocket would tie this test to the module cache. The
	// control plane compiles, so every error the checker reports is an
	// unresolved import or its consequence, and each is ignored.
	checked := map[string]*types.Package{}
	infos := map[string]*types.Info{}
	var check func(dir string) *types.Package
	imp := importerFunc(func(path string) (*types.Package, error) {
		if dir, ok := strings.CutPrefix(path, module+"/"); ok {
			if pkg := check(dir); pkg != nil {
				return pkg, nil
			}
		}
		return nil, fmt.Errorf("%s: not resolved by kindConstants", path)
	})
	check = func(dir string) *types.Package {
		if pkg, done := checked[dir]; done {
			return pkg
		}
		checked[dir] = nil // an import cycle does not compile; this only stops a loop
		names, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		var files []*ast.File
		for _, name := range names {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, f)
		}
		if len(files) == 0 {
			return nil
		}
		info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
		conf := types.Config{Importer: imp, Error: func(error) {}}
		pkg, _ := conf.Check(strings.TrimSuffix(module+"/"+dir, "/."), fset, files, info)
		checked[dir], infos[dir] = pkg, info
		return pkg
	}
	backendTypes := check("internal/backend")
	if backendTypes == nil {
		t.Fatal("package backend has no non-test source")
	}
	kind, ok := backendTypes.Scope().Lookup("Kind").(*types.TypeName)
	if !ok {
		t.Fatal("package backend declares no type Kind")
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if p != root && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
			return filepath.SkipDir // as the go command does
		}
		rel, err := filepath.Rel(root, p)
		if err == nil {
			check(filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil || len(infos) < 10 {
		t.Fatalf("walking the control plane from %s: %v (%d packages)", root, err, len(infos))
	}
	named, values = map[string]string{}, map[string]string{}
	for dir, info := range infos {
		for id, obj := range info.Defs {
			if c, ok := obj.(*types.Const); ok && types.Identical(c.Type(), kind.Type()) {
				named[dir+"."+id.Name] = c.Val().ExactString()
			}
		}
		for expr, tv := range info.Types {
			if tv.Value != nil && types.Identical(tv.Type, kind.Type()) {
				values[fset.Position(expr.Pos()).String()] = tv.Value.ExactString()
			}
		}
	}
	return named, values
}

// importerFunc is a function as a types.Importer; see kindConstants.
type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }
