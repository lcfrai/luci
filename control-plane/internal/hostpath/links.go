package hostpath

import (
	"errors"
	"fmt"
)

// ErrHardLinked is returned when a file has more than one name.
var ErrHardLinked = errors.New("hostpath: the file has another name, a hard link, that no walk of this name can see")

// OneName refuses a file that has more than one name, and a file whose link
// count cannot be read (X-3 review 4, I-C's second route).
//
// A hard link is a second directory entry for the same file, and making one
// needs no privilege on Windows, unlike a file symlink. When the host's
// ~/.gitconfig is a hard link to a file in a dotfiles project, the agent edits
// the in-project name in place and the host's name reads the edit, yet a walk
// of the host's name (ResolveOutside) passes no link and never touches the
// projects directory: measured, it returned the host's name with no error.
// Nothing on the file says where its other names are, so a count above one is
// refused outright rather than searched for. An agent cannot raise the count
// of a file outside its mounts, because it cannot see the file to link it, so
// a count of one read here stays one.
//
// Only a file whose CONTENT steers a mount needs this: a config file that sets
// a mounted value. A mounted file's own content is only what it shows the
// sandbox, read-only.
func OneName(p string) error {
	n, err := linkCount(p)
	if err != nil {
		return fmt.Errorf("hostpath: the link count of %q cannot be read, so whether it has another name is not known: %w", p, err)
	}
	if n != 1 {
		return fmt.Errorf("%w: %q has %d names", ErrHardLinked, p, n)
	}
	return nil
}
