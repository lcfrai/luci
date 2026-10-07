package api

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Only this session's home is mounted. Its parent and other sessions' homes
// stay outside the container, independently of the writable project mount.
const shellHomeDir = "/home/dev/.luci-shell"
const shellHomeParent = "h"

var shellSessionID = regexp.MustCompile(`^s-[a-f0-9]{8}$`)

// /etc/profile resets PATH for a login shell. Seed an ordinary, user-editable
// profile once so user-installed executables and the image's Go remain usable.
// Never source or copy the host user's profile, and never replace this on restart.
const shellInitialProfile = `# Luci terminal: tools installed in this private home survive Restart.
if [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
export PATH="$HOME/.local/bin:$HOME/.local/go/bin:$HOME/.cargo/bin:/usr/local/go/bin:$PATH"
`

func (a *API) shellHomePath(worktreeID, sessionID string) (string, error) {
	if !sessionStoreSegment.MatchString(worktreeID) || !shellSessionID.MatchString(sessionID) {
		return "", errors.New("invalid terminal home identity")
	}
	if !filepath.IsAbs(a.deps.ConversationsDir) {
		return "", errors.New("terminal state root must be absolute")
	}
	// A full digest keeps Windows paths short without collapsing account,
	// worktree or session identity. Each managed account owns a separate root.
	sum := sha256.Sum256([]byte("luci-shell-v1\x00" + sessionConversationOwner + "\x00" + worktreeID + "\x00" + sessionID))
	return filepath.Join(a.deps.ConversationsDir, shellHomeParent, base64.RawURLEncoding.EncodeToString(sum[:])), nil
}

// prepareShellHome is idempotent for Restart. New sessions have a new ID and
// receive an empty home, rather than another terminal's credentials or tools.
func (a *API) prepareShellHome(worktreeID, sessionID string) (string, error) {
	dir, err := a.shellHomePath(worktreeID, sessionID)
	if err != nil {
		return "", err
	}
	rootDir := filepath.Clean(a.deps.ConversationsDir)
	// Check the nearest existing ancestor before creating anything. A new
	// conversation root must not be created inside a project or through its link.
	for ancestor := rootDir; ; ancestor = filepath.Dir(ancestor) {
		if _, err = hostpath.ResolveOutside(a.projectsDir(), ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) || filepath.Dir(ancestor) == ancestor {
			return "", fmt.Errorf("terminal state root: %w", err)
		}
	}
	if err = os.MkdirAll(rootDir, 0700); err != nil {
		return "", err
	}
	if err = codexRegularPath(rootDir, rootDir); err != nil {
		return "", err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), rootDir); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	parent := filepath.Join(rootDir, shellHomeParent)
	if err = root.Mkdir(shellHomeParent, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err = shellPrivateDirectory(rootDir, parent); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(rootDir, dir)
	created := false
	if err = root.Mkdir(rel, 0700); err == nil {
		created = true
	} else if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err = shellPrivateDirectory(rootDir, dir); err != nil {
		return "", err
	}
	if !created {
		return dir, nil // user-owned contents are never rewritten on Restart
	}
	// The newly created directory has not been exposed to a sandbox yet.
	// Confined cleanup removes only that incomplete new home on setup failure.
	complete := false
	defer func() {
		if !complete {
			_ = root.RemoveAll(rel)
		}
	}()
	for _, child := range []string{".config", ".cache", ".local", ".local/bin", ".local/share", ".local/state"} {
		if err = root.MkdirAll(filepath.Join(rel, filepath.FromSlash(child)), 0700); err != nil {
			return "", err
		}
	}
	profile, err := root.OpenFile(filepath.Join(rel, ".bash_profile"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := profile.WriteString(shellInitialProfile)
	closeErr := profile.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	complete = true
	return dir, nil
}

func shellPrivateDirectory(root, dir string) error {
	if err := codexRegularPath(root, dir); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("terminal home must be a directory")
	}
	return codexPrivateDir(dir) // 0700 on Unix; private inherited ACL on Windows
}

func shellEnvironment(env map[string]string) {
	// These are installation/configuration locations, never provider access.
	// Workspace package installs keep their package manager's project defaults.
	for key, suffix := range map[string]string{
		"HOME": "", "XDG_CONFIG_HOME": "/.config", "XDG_DATA_HOME": "/.local/share",
		"XDG_STATE_HOME": "/.local/state", "XDG_CACHE_HOME": "/.cache",
		"npm_config_prefix": "/.local", "npm_config_cache": "/.cache/npm",
		"PIP_CACHE_DIR": "/.cache/pip", "GOCACHE": "/.cache/go-build",
		"GOMODCACHE": "/.cache/go-mod", "GOPATH": "/.local/go", "CARGO_HOME": "/.cargo",
	} {
		env[key] = shellHomeDir + suffix
	}
}

func shellMount(mounts []backend.Mount, dir string) []backend.Mount {
	return append(mounts, backend.Mount{HostPath: dir, ContainerPath: shellHomeDir})
}

// Called only after the session's container is gone. Kill retains the home;
// explicit Forget removes precisely the home named by this session's record.
func (a *API) forgetShellHome(sess registry.Session) error {
	if sess.Harness != string(harness.Shell) {
		return errors.New("only a terminal session can forget its terminal home")
	}
	dir, err := a.shellHomePath(sess.WorktreeID, sess.ID)
	if err != nil {
		return err
	}
	rootDir := filepath.Clean(a.deps.ConversationsDir)
	if err = codexRegularPath(rootDir, dir); errors.Is(err, os.ErrNotExist) {
		return nil // older sessions did not have persistent homes
	} else if err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return errors.New("terminal home must be a directory")
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), dir); err != nil {
		return err
	}
	if _, err = hostpath.Confine(filepath.Join(rootDir, shellHomeParent), dir); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	rel, _ := filepath.Rel(rootDir, dir)
	return root.RemoveAll(rel) // child links are removed, never followed
}
