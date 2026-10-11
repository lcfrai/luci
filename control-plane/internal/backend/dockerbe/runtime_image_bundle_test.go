package dockerbe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the actual bundled Dockerfile and vendored source, rather than only
// a fixture that could drift away from the production packaging contract.
// Docker observation is controlled; this does not pull, build, tag or execute.
func TestRuntimeImageAcceptsCurrentRepositoryBundleReadOnly(t *testing.T) {
	b, f, spec := newRuntimeImageProbe(t)
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	spec.SourceDirectory = root
	prepared, err := b.PrepareRuntimeImage(context.Background(), spec)
	if err != nil {
		t.Fatal("current bundled sources rejected", err)
	}
	files := runtimeFixtureTar(t, prepared.PreparedContext)
	for _, name := range []string{"sandboxd/go.mod", "sandboxd/vendor/modules.txt", "images/sandbox/entrypoint.sh"} {
		original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || files[name] != string(original) {
			t.Fatalf("bundled input %s changed or missing: %v", name, err)
		}
	}
	if files["images/sandbox/Dockerfile"] == "" || prepared.ContextDigest == "" || prepared.ContextDigest != prepared.Spec.Identity.SourceDigest {
		t.Fatal("runtime source identity was not bound")
	}
	assertRuntimeNoMutation(t, f)
	t.Logf("read-only bundled context: files=%d bytes=%d sourceDigest=%s", len(files), len(prepared.PreparedContext), prepared.ContextDigest)
}
