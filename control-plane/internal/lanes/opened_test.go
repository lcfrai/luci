package lanes

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// countingSource reads the real file, then endless bytes, counting every byte read, while its
// Stat answers the real file's, so SameFile and the size check both pass.
type countingSource struct {
	f    *os.File
	r    io.Reader
	read *atomic.Int64
}

func (c countingSource) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.read.Add(int64(n))
	return n, err
}
func (c countingSource) Close() error               { return c.f.Close() }
func (c countingSource) Stat() (fs.FileInfo, error) { return c.f.Stat() }

type endless struct{}

func (endless) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 'x'
	}
	return len(b), nil
}

// Not parallel: it swaps the package variable openForCopy.
func TestCopyGeneratedHooksChecksTheOpenedFile(t *testing.T) {
	setup := func(t *testing.T) (projectTop, cloneTop string) {
		projectTop, cloneTop = t.TempDir(), t.TempDir()
		huskyFiles(t, filepath.Join(projectTop, ".husky", "_"), 2, 10)
		return
	}
	defer func(orig func(string) (copySource, error)) { openForCopy = orig }(openForCopy)

	t.Run("a different file", func(t *testing.T) {
		projectTop, cloneTop := setup(t)
		other := filepath.Join(t.TempDir(), "other")
		writeFile(t, other, strings.Repeat("y", 10))
		openForCopy = func(string) (copySource, error) { return os.Open(other) }
		n, err := LocalFS{}.CopyGeneratedHooks(context.Background(), projectTop, cloneTop, ".husky/_")
		if n != 0 || err == nil || !strings.Contains(err.Error(), filepath.Join(projectTop, ".husky", "_")) {
			t.Errorf("= %d, %v; want 0 and an error naming the path", n, err)
		}
		if _, err := os.Lstat(filepath.Join(cloneTop, ".husky", "_")); err == nil {
			t.Error("the refused copy left files in the clone")
		}
		// review m-4: .husky did not exist before the copy, so the copy's cleanup removes it too.
		if _, err := os.Lstat(filepath.Join(cloneTop, ".husky")); err == nil {
			t.Error("the refused copy left the parent it created in the clone")
		}
	})
	t.Run("larger than its Lstat", func(t *testing.T) {
		projectTop, cloneTop := setup(t)
		var read atomic.Int64
		openForCopy = func(name string) (copySource, error) {
			f, err := os.Open(name)
			if err != nil {
				return nil, err
			}
			return countingSource{f: f, r: io.MultiReader(f, endless{}), read: &read}, nil
		}
		n, err := LocalFS{}.CopyGeneratedHooks(context.Background(), projectTop, cloneTop, ".husky/_")
		if n != 0 || err == nil {
			t.Errorf("= %d, %v; want 0 and a refusal", n, err)
		}
		if got := read.Load(); got > maxHookBytes {
			t.Errorf("read %d bytes, want at most %d", got, maxHookBytes)
		}
	})
	t.Run("grown after its Lstat", func(t *testing.T) {
		projectTop, cloneTop := setup(t)
		openForCopy = func(name string) (copySource, error) {
			if err := os.WriteFile(name, []byte(strings.Repeat("z", 2*maxHookBytes)), 0o644); err != nil {
				return nil, err
			}
			return os.Open(name)
		}
		if n, err := (LocalFS{}).CopyGeneratedHooks(context.Background(), projectTop, cloneTop, ".husky/_"); n != 0 || err == nil {
			t.Errorf("= %d, %v; want 0 and a refusal", n, err)
		}
	})
}
