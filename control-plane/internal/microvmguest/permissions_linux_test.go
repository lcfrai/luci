//go:build linux

package microvmguest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProtectedGuestContextCannotBeReplacedByUpstreamUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("actual Linux ownership enforcement requires root fixture")
	}
	parent, e := os.MkdirTemp("/tmp", "luci-owned-guest-permission-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(parent)
	info, e := os.Lstat(parent)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		t.Fatal("unexpected fixture ownership")
	}
	if e = os.Chmod(parent, 0755); e != nil {
		t.Fatal(e)
	}
	data, binding, digest := archive(t)
	p, e := Prepare(context.Background(), filepath.Join(parent, "attempt"), bytes.NewReader(data), binding, digest)
	if e != nil {
		t.Fatal(e)
	}
	if e = Protect(p, 1001, 1001); e != nil {
		t.Fatal(e)
	}
	commit, e := InitializeRepository(context.Background(), p)
	if e != nil || len(commit) != 40 {
		t.Fatal("independent Git workspace", commit, e)
	}
	if _, e = InitializeRepository(context.Background(), p); e == nil {
		t.Fatal("reinitialized existing Git source")
	}
	for path, want := range map[string]uint32{"source": 1001, "source/AGENTS.md": 1001, "context": 0, "context/reviewed.json": 0, ".": 0} {
		info, e := os.Lstat(filepath.Join(p.Directory(), path))
		if e != nil {
			t.Fatal(e)
		}
		if info.Sys().(*syscall.Stat_t).Uid != want {
			t.Fatal(path, "wrong owner")
		}
	}
	command := exec.Command("/bin/sh", "-c", `test -r "$1/context/reviewed.json" && printf 'updated' > "$1/source/AGENTS.md" && test -x "$1/source/build.sh" && ! (printf 'tamper' > "$1/context/reviewed.json") && ! mv "$1/context" "$1/renamed-context" && ! rm "$1/prepared.json"`, "guest-fixture", p.Directory())
	command.Dir = parent
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1001, Gid: 1001, Groups: []uint32{}}}
	if out, e := command.CombinedOutput(); e != nil {
		t.Fatalf("unprivileged boundary: %v %s", e, out)
	}
	if b, _ := os.ReadFile(filepath.Join(p.Directory(), "context/reviewed.json")); string(b) != "{\"revision\":7}" {
		t.Fatal("protected context changed")
	}
}
