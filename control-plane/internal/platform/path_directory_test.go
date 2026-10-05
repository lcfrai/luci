package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

func TestPlatformPrivateDirectoryReuseRetainsFileAndReparseGuards(t *testing.T) {
	s := fixture(t, nil)
	root, e := s.path("stack-regression")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(root, "volumes", "db"), 0700); e != nil {
		t.Fatal(e)
	}
	// On native Linux, directory link counts include children. File-only
	// OneName checks must not reject this previously prepared stack directory.
	if again, e := s.path("stack-regression"); e != nil || again != root {
		t.Fatalf("prepared data directory cannot be reused: %v", e)
	}
	file := filepath.Join(s.dir, "private-record")
	if e = os.WriteFile(file, []byte(`{"value":"private"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Link(file, filepath.Join(s.dir, "second-record-name")); e != nil {
		t.Fatal(e)
	}
	if _, e = s.path("private-record"); !errors.Is(e, hostpath.ErrHardLinked) {
		t.Fatal("regular hard-linked private state was admitted")
	}
	link := filepath.Join(s.dir, "linked-stack")
	if e = os.Symlink(root, link); e == nil {
		if _, e = s.path("linked-stack"); e == nil {
			t.Fatal("linked private stack directory was admitted")
		}
	}
}
