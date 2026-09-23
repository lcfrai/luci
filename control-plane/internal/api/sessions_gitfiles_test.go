package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// gitfix-rereview I-A: the host's git reads a global excludes file even with
// core.excludesFile unset, $XDG_CONFIG_HOME/git/ignore or ~/.config/git/ignore,
// and a global attributes file the same way. The sandbox's HOME has neither, so
// an agent's `git add -A` committed .claude/settings.local.json, which the
// owner's host ignores there. Every sandbox's git now reads the host's own
// files, mounted read-only, and reads nothing where the host reads nothing.

// sandboxEnvGlobalFiles is a Deps.GitGlobalFiles double that records every
// worktree path it was asked about.
type sandboxEnvGlobalFiles struct {
	mu    sync.Mutex
	asked []string
	files GitGlobalFiles
	err   error
}

func (f *sandboxEnvGlobalFiles) resolve(_ context.Context, worktreePath string) (GitGlobalFiles, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, worktreePath)
	return f.files, f.err
}

func (f *sandboxEnvGlobalFiles) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

// sandboxEnvMountsAt is every mount of the sandbox at containerPath.
func sandboxEnvMountsAt(spec backend.Spec, containerPath string) []backend.Mount {
	var out []backend.Mount
	for _, m := range spec.Mounts {
		if m.ContainerPath == containerPath {
			out = append(out, m)
		}
	}
	return out
}

// sandboxEnvHostFile makes what a global-file answer names on the host, by
// kind, and returns the path the resolver answers with.
func sandboxEnvHostFile(t *testing.T, kind, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "git", name)
	switch kind {
	case "file":
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("**/.claude/settings.local.json\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case "link":
		// A dotfiles manager's ~/.config/git/ignore: a link to the real file.
		target := filepath.Join(dir, "dotfiles-ignore")
		if err := os.WriteFile(target, []byte("**/.claude/settings.local.json\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("this host cannot make a file symlink: %v", err)
		}
	case "missing":
		// git names the default path whether or not anything is there.
	case "dir":
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	case "none":
		return ""
	default:
		t.Fatalf("unknown kind %q", kind)
	}
	return path
}

// sandboxEnvWantGitFile checks one of the two global files in a spec: named at
// its mount path and mounted there read-only from the file hostPath is, links
// resolved, when want, else named as /dev/null and mounted nowhere.
func sandboxEnvWantGitFile(t *testing.T, spec backend.Spec, key, containerPath, hostPath string, want bool) {
	t.Helper()
	cfg := sandboxEnvGitConfig(t, spec.Env)
	got := sandboxEnvMountsAt(spec, containerPath)
	if want {
		if cfg[key] != containerPath {
			t.Errorf("%s = %q, want the mounted host file at %q", key, cfg[key], containerPath)
		}
		file, err := filepath.EvalSymlinks(hostPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].HostPath != file || !got[0].ReadOnly {
			t.Errorf("mounts at %s = %#v, want exactly %q, read-only", containerPath, got, file)
		}
		return
	}
	// Unset would let the sandbox's own ~/.config/git default count, which
	// the host never reads and an agent can write.
	if cfg[key] != sessionGitNoFile {
		t.Errorf("%s = %q, want %q: the host's git reads no such file here", key, cfg[key], sessionGitNoFile)
	}
	if len(got) != 0 {
		t.Errorf("mounts at %s = %#v, want none: the host's git reads no such file here", containerPath, got)
	}
}

// Each file the host's git would read reaches the sandbox read-only, at its one
// path, named by its key, for every harness. A path with nothing there, a
// directory, and no path at all are each "the host's git reads no patterns
// here", and the sandbox then reads none: /dev/null, no mount.
func TestTheHostsGlobalGitFilesReachTheSandboxReadOnlyAndOnlyWhenTheHostsGitReadsThem(t *testing.T) {
	for _, tc := range []struct {
		excludes, attributes string
	}{
		{"file", "file"},
		{"file", "missing"}, // the owner's host: ~/.config/git/ignore, no attributes
		{"missing", "file"},
		{"missing", "missing"},
		{"dir", "none"},
		{"none", "dir"},
		// Followed as git follows it, and mounted from the file itself, so
		// no Docker host is ever handed a link as a mount source.
		{"link", "missing"},
		{"missing", "link"},
	} {
		for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
			t.Run(tc.excludes+"/"+tc.attributes+" "+body, func(t *testing.T) {
				host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{
					Excludes:   sandboxEnvHostFile(t, tc.excludes, "ignore"),
					Attributes: sandboxEnvHostFile(t, tc.attributes, "attributes"),
				}}
				env := sandboxEnvNew(t, func(d *Deps) { d.GitGlobalFiles = host.resolve })
				spec := sandboxEnvCreate(t, env, body)

				if asked := host.calls(); len(asked) != 1 || asked[0] != env.dir {
					t.Fatalf("the global files were asked of %#v, want once, of the worktree's host path %q", asked, env.dir)
				}
				sandboxEnvWantGitFile(t, spec, "core.excludesFile", sessionGitExcludesPath, host.files.Excludes, tc.excludes == "file" || tc.excludes == "link")
				sandboxEnvWantGitFile(t, spec, "core.attributesFile", sessionGitAttributesPath, host.files.Attributes, tc.attributes == "file" || tc.attributes == "link")
				if spec.Mounts[0].ContainerPath != sessionWorkDir || spec.Mounts[0].ReadOnly {
					t.Errorf("Mounts[0] = %#v, want the worktree itself, writable", spec.Mounts[0])
				}
			})
		}
	}
}

// With no answer, or a path that would be read against this process's
// directory rather than the worktree's, the create fails before anything
// exists: no record, and no sandbox whose `git add -A` commits what the host
// ignores.
func TestAGlobalFilesAnswerTheSandboxCannotUseFailsTheCreateAndLeavesNothing(t *testing.T) {
	for name, host := range map[string]*sandboxEnvGlobalFiles{
		"an error":                 {err: errors.New("git config --get --type=path core.excludesFile in C:\\work: exit status 128: fatal: bad config line 3")},
		"a relative excludes path": {files: GitGlobalFiles{Excludes: filepath.Join("git", "ignore")}},
		"a relative attributes":    {files: GitGlobalFiles{Attributes: filepath.Join("git", "attributes")}},
		"half and an error":        {files: GitGlobalFiles{Excludes: filepath.Join(t.TempDir(), "ignore")}, err: errors.New("git config: exit status 128")},
	} {
		t.Run(name, func(t *testing.T) {
			env := sandboxEnvNew(t, func(d *Deps) { d.GitGlobalFiles = host.resolve })
			rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
			}
			if n := sandboxEnvCreates(env); n != 0 {
				t.Errorf("backend.Create was called %d times", n)
			}
			if list, err := env.st.List(); err != nil || len(list) != 0 {
				t.Errorf("the registry holds %#v (err %v), want nothing", list, err)
			}
		})
	}
}

// Restart asks again, with the worktree's host path, and before the destroy:
// a file the user added since the first create is in the new sandbox, and a
// failure leaves the old sandbox alive and the record naming it.
func TestRestartAsksForTheGlobalGitFilesAgainBeforeTheDestroy(t *testing.T) {
	t.Run("answered", func(t *testing.T) {
		host := &sandboxEnvGlobalFiles{files: GitGlobalFiles{
			Excludes:   sandboxEnvHostFile(t, "file", "ignore"),
			Attributes: sandboxEnvHostFile(t, "missing", "attributes"),
		}}
		bundle := filepath.Join(t.TempDir(), "corp-root.pem")
		env := sandboxEnvNew(t, func(d *Deps) {
			d.GitGlobalFiles = host.resolve
			d.CABundle = bundle
		})
		sessionsStubReady(t, nil)
		done := sessionsWatchDone(t)
		sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

		sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
		sessionsAwaitWatch(t, done)
		spec := env.be.spec(t, 0)

		if asked := host.calls(); len(asked) != 1 || asked[0] != env.dir {
			t.Errorf("the restart asked for the global files of %#v, want once, of %q", asked, env.dir)
		}
		sandboxEnvWantGitFile(t, spec, "core.excludesFile", sessionGitExcludesPath, host.files.Excludes, true)
		sandboxEnvWantGitFile(t, spec, "core.attributesFile", sessionGitAttributesPath, host.files.Attributes, false)
		if ca := sandboxEnvMountsAt(spec, sessionCABundlePath); len(ca) != 1 || ca[0].HostPath != bundle || !ca[0].ReadOnly {
			t.Errorf("after a restart the CA mounts are %#v, want %q read-only", ca, bundle)
		}
		// Go contract §5.1 step 5 states the order: the worktree first, the
		// CA bundle, then the excludes file.
		order := map[string]int{}
		for i, m := range spec.Mounts {
			order[m.ContainerPath] = i
		}
		if !(order[sessionWorkDir] == 0 && order[sessionCABundlePath] < order[sessionGitExcludesPath]) {
			t.Errorf("mount order %#v, want the worktree first and the CA bundle before the excludes file", spec.Mounts)
		}
	})
	t.Run("an error", func(t *testing.T) {
		host := &sandboxEnvGlobalFiles{err: errors.New("exec: \"git\": executable file not found in %PATH%")}
		env := sandboxEnvNew(t, func(d *Deps) { d.GitGlobalFiles = host.resolve })
		seeded := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

		rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
		}
		if len(host.calls()) != 1 {
			t.Errorf("the restart asked for the global files %d times, want once", len(host.calls()))
		}
		if n := env.be.destroyCount(); n != 0 {
			t.Errorf("the failed restart destroyed %d sandboxes", n)
		}
		if n := sandboxEnvCreates(env); n != 0 {
			t.Errorf("the failed restart created %d sandboxes", n)
		}
		if after, err := env.st.Get("s-0000000b"); err != nil || after.SandboxID != seeded.SandboxID {
			t.Errorf("after the failed restart the record names %q (err %v), want the old sandbox %q", after.SandboxID, err, seeded.SandboxID)
		}
	})
}

// A wiring that forgot the resolver would reopen I-A silently, so New refuses
// it with a sentence naming the field.
func TestNewRefusesDepsWithNoGlobalGitFilesResolver(t *testing.T) {
	var deps Deps
	sandboxEnvNew(t, func(d *Deps) { deps = *d })
	deps.GitGlobalFiles = nil
	if _, err := New(deps); err == nil || !strings.Contains(err.Error(), "Deps.GitGlobalFiles") {
		t.Errorf("New with no GitGlobalFiles: err %v, want one naming Deps.GitGlobalFiles", err)
	}
}
