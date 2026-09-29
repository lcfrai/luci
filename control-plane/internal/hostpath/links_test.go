package hostpath

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// X-3 review 4, I-C's second route, lifted from the reviewer's attack: the
// host's ~/.gitconfig is a HARD link to a file in a dotfiles project. The agent
// edits the in-project name in place and the host's name reads the edit, while
// the walk of the host's name passes no link and never touches the projects
// directory. The walk cannot see a second name, so the link count must refuse.
func TestAHardLinkedFileIsRefusedUnderEitherName(t *testing.T) {
	projects := t.TempDir()
	inProject := filepath.Join(projects, "dotfiles", ".gitconfig")
	writeFile(t, inProject, []byte("[user]\n\tname = me\n"))
	hostName := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.Link(inProject, hostName); err != nil {
		t.Skipf("no hard link here: %v", err)
	}
	f, err := os.OpenFile(inProject, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[core]\n\texcludesFile = C:/Users/x/.ssh/id_ed25519\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(hostName); len(b) == 0 {
		t.Fatalf("attack not set up: the host's name reads nothing")
	}
	// The premise the count exists for: the walk alone passes the host's name.
	if _, err := ResolveOutside(projects, hostName); err != nil {
		t.Fatalf("ResolveOutside(%q) = %v; the walk was expected to pass a hard link", hostName, err)
	}
	for _, name := range []string{hostName, inProject} {
		if err := OneName(name); !errors.Is(err, ErrHardLinked) {
			t.Errorf("OneName(%q) = %v, want ErrHardLinked: the file has a second name the agent writes", name, err)
		}
	}
}

// The count must not cost the normal case, and a file whose count cannot be
// read is refused, not passed: one name is accepted, and so is a file whose
// other name has been removed; a missing file is an error.
func TestAFileWithOneNameIsAcceptedAndAnUnreadableCountIsNot(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, ".gitconfig")
	writeFile(t, one, []byte("[user]\n\tname = me\n"))
	if err := OneName(one); err != nil {
		t.Errorf("OneName of a file with one name = %v, want nil", err)
	}
	other := filepath.Join(dir, "second")
	if err := os.Link(one, other); err == nil {
		if err := os.Remove(other); err != nil {
			t.Fatal(err)
		}
		if err := OneName(one); err != nil {
			t.Errorf("OneName after the second name is removed = %v, want nil", err)
		}
	}
	if err := OneName(filepath.Join(dir, "missing")); err == nil {
		t.Errorf("OneName of a missing file = nil, want an error: a count that cannot be read is not one")
	}
}
