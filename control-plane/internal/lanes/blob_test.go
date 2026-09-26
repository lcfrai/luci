package lanes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

const lfsPointer = "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 12345\n"

// blobRepo is a project with a symlink entry link -> t.txt (mode 120000, written through the
// index, so no link is made on disk here) and a.bin under `*.bin filter=lfs`, committed before
// any filter is configured, so a.bin's blob is the pointer text.
func blobRepo(t *testing.T) *gitxtest.Repo {
	t.Helper()
	p := gitxtest.NewRepo(t)
	p.Commit("files", map[string]string{"t.txt": "target\n", "a.bin": lfsPointer, ".gitattributes": "*.bin filter=lfs diff=lfs merge=lfs -text\n"})
	writeFile(t, filepath.Join(p.Dir, "linktarget.tmp"), "t.txt")
	sha := p.Git("hash-object", "-w", "--no-filters", "linktarget.tmp")
	if err := os.Remove(filepath.Join(p.Dir, "linktarget.tmp")); err != nil {
		t.Fatal(err)
	}
	p.Git("update-index", "--add", "--cacheinfo", "120000,"+sha+",link")
	p.Git("commit", "-q", "-m", "link")
	return p
}

// TestCreateWritesBlobBytes (review Minors 13, 15; carried t01-D11). "marker" is the part's
// LFS-shaped fixture: a smudge that writes a marker, clean = cat, required, no process. "git-lfs"
// is git-lfs's real shape, `process = git-lfs filter-process` and required, with git-lfs absent
// on this machine: a checkout that tried the driver would die, so only process= and
// required=false let it through. Its status runs with no system config, as the sandbox's git
// has none: under this one the host's own `status` cannot run the absent clean filter.
func TestCreateWritesBlobBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, filter string
		hostStatus   bool
	}{
		{"marker", "[filter \"lfs\"]\n\tsmudge = touch '%MARKER%' && cat\n\tclean = cat\n\trequired = true\n", true},
		{"git-lfs", "[filter \"lfs\"]\n\tclean = git-lfs clean -- %f\n\tsmudge = git-lfs smudge -- %f\n\tprocess = git-lfs filter-process\n\trequired = true\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := blobRepo(t)
			marker := filepath.ToSlash(filepath.Join(t.TempDir(), "smudged"))
			body := gitxtest.WindowsInstallerSystem + "\tsymlinks = true\n" + strings.ReplaceAll(tc.filter, "%MARKER%", marker)
			p.UseSystemConfig(body)
			r := newRig(t, p)
			l := r.mustCreate("main", "blob")
			c := r.lane(l)
			for k, want := range map[string]string{"core.symlinks": "false", "core.longpaths": "true"} {
				if got := c.Git("config", "--local", "--get", k); got != want {
					t.Errorf("clone config %s = %q, want %q", k, got, want)
				}
			}
			if out, err := c.GitErr("config", "--local", "--get-regexp", `^filter\.`); err == nil {
				t.Errorf("clone local config holds filter keys: %q", out)
			}
			fi, err := os.Lstat(filepath.Join(l.Path, "link"))
			if err != nil || !fi.Mode().IsRegular() {
				t.Fatalf("link: Lstat = %v, %v; want a regular file", fi, err)
			}
			if got := readFile(t, filepath.Join(l.Path, "link")); got != "t.txt" {
				t.Errorf("link holds %q, want its target's text t.txt", got)
			}
			if got := readFile(t, filepath.Join(l.Path, "a.bin")); got != lfsPointer {
				t.Errorf("a.bin = %q, want the pointer blob byte for byte", got)
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the smudge filter ran (marker %s: %v)", marker, err)
			}
			if !tc.hostStatus {
				env := map[string]string{}
				for k, v := range c.Env {
					env[k] = v
				}
				delete(env, "GIT_CONFIG_SYSTEM")
				env["GIT_CONFIG_NOSYSTEM"] = "1"
				c = &gitxtest.Repo{T: t, Dir: l.Path, Env: env}
			}
			if got := c.Git("status", "--porcelain"); got != "" {
				t.Errorf("status --porcelain = %q, want empty", got)
			}
		})
	}
}
