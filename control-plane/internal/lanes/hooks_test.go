package lanes

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// hookValues is the part's table: each refused value, and the one copied.
var hookValues = []struct {
	v    string
	copy bool
}{
	{"/abs/hooks", false}, {"C:/hooks", false}, {"C:hooks", false}, {`\hooks`, false},
	{"~/hooks", false}, {"../x", false}, {"a/../../x", false}, {`.githooks\x`, false},
	{".git::$INDEX_ALLOCATION/hooks", false}, // carried b03-2 Minor 2: an NTFS name for .git
	{".githooks", true},
}

func TestCopyableHooksPath(t *testing.T) {
	for _, tc := range hookValues {
		if got := CopyableHooksPath(tc.v); got != tc.copy {
			t.Errorf("CopyableHooksPath(%q) = %v, want %v", tc.v, got, tc.copy)
		}
	}
	for _, v := range []string{"", ".husky/_\n"} {
		if CopyableHooksPath(v) {
			t.Errorf("CopyableHooksPath(%q) = true", v)
		}
	}
	if !CopyableHooksPath(".husky/_") {
		t.Error("CopyableHooksPath(.husky/_) = false")
	}
}

func TestCreateCopiesARelativeHooksPath(t *testing.T) {
	t.Parallel()
	t.Run("husky", func(t *testing.T) {
		t.Parallel()
		p := gitxtest.NewRepo(t)
		p.Git("config", "core.hooksPath", ".husky/_")
		writeFile(t, filepath.Join(p.Dir, ".git", "hooks", "pre-commit"), "#!/c/Program Files/Git/bin/sh\nexit 1\n")
		r := newRig(t, p)
		l := r.mustCreate("main", "husky")
		if got := r.lane(l).Git("config", "--local", "--get", "core.hooksPath"); got != ".husky/_" {
			t.Errorf("clone core.hooksPath = %q, want .husky/_", got)
		}
		if _, err := os.Lstat(filepath.Join(l.Path, ".git", "hooks", "pre-commit")); err == nil {
			t.Error("the project's .git/hooks/pre-commit reached the clone")
		}
	})
	for _, tc := range hookValues {
		t.Run(tc.v, func(t *testing.T) {
			t.Parallel()
			p := gitxtest.NewRepo(t)
			p.Git("config", "core.hooksPath", tc.v)
			r := newRig(t, p)
			l := r.mustCreate("main", "h")
			got, err := r.lane(l).GitErr("config", "--local", "--get", "core.hooksPath")
			switch {
			case tc.copy && got != tc.v:
				t.Errorf("clone core.hooksPath = %q, %v; want %q", got, err, tc.v)
			case !tc.copy && err == nil:
				t.Errorf("clone core.hooksPath = %q; want none", got)
			}
		})
	}
}

func huskyRepo(t *testing.T) *gitxtest.Repo {
	t.Helper()
	p := gitxtest.NewRepo(t)
	p.Commit("husky", map[string]string{".husky/pre-commit": "npm test\n"})
	p.Git("config", "core.hooksPath", ".husky/_")
	for name, body := range map[string]string{".gitignore": "*\n", "h": "#!/usr/bin/env sh\n. \"$(dirname \"$0\")/h\"\n", "pre-commit": "#!/usr/bin/env sh\n. \"$(dirname \"$0\")/h\"\n"} {
		writeFile(t, filepath.Join(p.Dir, ".husky", "_", name), body)
	}
	return p
}

func listDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, e := range ents {
		if !e.IsDir() {
			out[e.Name()] = readFile(t, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func TestCreateCopiesAGeneratedHooksDir(t *testing.T) {
	t.Parallel()
	p := huskyRepo(t)
	r := newRig(t, p)
	l := r.mustCreate("main", "gen")
	want := listDir(t, filepath.Join(p.Dir, ".husky", "_"))
	got := listDir(t, filepath.Join(l.Path, ".husky", "_"))
	if len(want) != 3 || len(got) != len(want) {
		t.Fatalf("clone .husky/_ = %v, want %v", keys(got), keys(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("clone .husky/_/%s = %q, want %q", k, got[k], v)
		}
	}
	if s := r.lane(l).Git("status", "--porcelain"); s != "" {
		t.Errorf("status --porcelain = %q, want empty", s)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
