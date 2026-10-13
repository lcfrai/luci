//go:build linux

package microvmguest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

// InitializeRepository gives upstream engines their normal independent Git
// workspace without importing host .git/config, hooks, credentials or history.
// This is a synthetic guest snapshot, explicitly distinct from the original
// host commit. Result import must bind the original transfer/source digest.
func InitializeRepository(ctx context.Context, p Prepared) (string, error) {
	if e := validatePrepared(p); e != nil {
		return "", e
	}
	if os.Geteuid() != 0 {
		return "", ErrRefused
	}
	source := filepath.Join(p.directory, "source")
	if _, e := os.Lstat(filepath.Join(source, ".git")); !os.IsNotExist(e) {
		return "", ErrRefused
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	git := func(args ...string) (string, error) {
		argv := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "init.templateDir=", "-c", "commit.gpgSign=false"}, args...)
		command := exec.CommandContext(ctx, "/usr/bin/git", argv...)
		command.Dir = source
		command.Env = []string{"PATH=/usr/bin:/bin", "HOME=/home/dev", "LANG=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_NAME=Luci", "GIT_AUTHOR_EMAIL=guest@luci.invalid", "GIT_COMMITTER_NAME=Luci", "GIT_COMMITTER_EMAIL=guest@luci.invalid"}
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1001, Gid: 1001}}
		output, e := command.Output()
		if e != nil {
			return "", e
		}
		if len(output) > 4096 {
			return "", ErrRefused
		}
		return strings.TrimSpace(string(output)), nil
	}
	for _, args := range [][]string{{"init", "--initial-branch=main", "."}, {"add", "--force", "--all", "--", "."}, {"commit", "--no-gpg-sign", "--allow-empty", "-m", "Reviewed Luci task input snapshot"}} {
		if _, e := git(args...); e != nil {
			return "", e
		}
	}
	commit, e := git("rev-parse", "HEAD")
	if e != nil {
		return "", e
	}
	if !regexp.MustCompile(`^[a-f0-9]{40,64}$`).MatchString(commit) {
		return "", ErrRefused
	}
	if e = writeJSON(filepath.Join(p.directory, "guest-repository.json"), struct{ Kind, Commit, OriginalSourceDigest, ArchiveSHA256 string }{"synthetic-guest-snapshot", commit, p.manifest.Binding.SourceDigest, p.archiveSHA256}); e != nil {
		return "", e
	}
	return commit, nil
}

// ExposeReviewedContext preserves the existing upstream prompt path with a
// read-only bind mount. A replaceable symlink under the writable dev home would
// not preserve this boundary. Missing mount capability is a refusal, no fallback.
// The trusted template must precreate the destination root-owned and empty.
func ExposeReviewedContext(p Prepared) error {
	if e := validatePrepared(p); e != nil {
		return e
	}
	if os.Geteuid() != 0 {
		return ErrRefused
	}
	target := harness.ReviewedContextDirectory
	// The bind mount itself is read-only, but its pathname is not immutable if
	// the upstream user can replace an ancestor. The guest template and its
	// UID bootstrap must keep every ancestor root-owned and non-writable.
	if _, e := privateHierarchy(target, 0); e != nil {
		return ErrRefused
	}
	info, e := os.Lstat(target)
	if e != nil {
		return e
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 {
		return ErrRefused
	}
	entries, e := os.ReadDir(target)
	if e != nil {
		return e
	}
	if len(entries) != 0 {
		return ErrRefused
	}
	if e = syscall.Mount(filepath.Join(p.directory, "context"), target, "", syscall.MS_BIND, ""); e != nil {
		return e
	}
	return syscall.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
}
