package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// The sandbox git fix: every sandbox's environment carries git's
// safe.directory for the mount point, the user's commit identity, and — when
// the operator configured one — the extra CA bundle Node must trust.

// sandboxEnvLog is a log sink a test can read while the readiness goroutine may
// still be writing to it.
type sandboxEnvLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *sandboxEnvLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *sandboxEnvLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// sandboxEnvIdentity is a Deps.GitIdentity double that records every worktree
// path it was asked about.
type sandboxEnvIdentity struct {
	mu    sync.Mutex
	asked []string
	id    GitIdentity
	err   error
}

func (f *sandboxEnvIdentity) resolve(_ context.Context, worktreePath string) (GitIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, worktreePath)
	return f.id, f.err
}

func (f *sandboxEnvIdentity) set(id GitIdentity, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.id, f.err = id, err
}

func (f *sandboxEnvIdentity) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

func sandboxEnvNew(t *testing.T, tweak func(*Deps)) *sessionsEnv {
	t.Helper()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, tweak)
	env.st = st
	return env
}

// sandboxEnvCreate creates one session with body and returns the spec its
// sandbox was built from.
func sandboxEnvCreate(t *testing.T, env *sessionsEnv, body string) backend.Spec {
	t.Helper()
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", body), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	return env.be.spec(t, 0)
}

const sandboxEnvShellBody = `{"worktreeId":"w-4a77b1c9","harness":"shell"}`

// sandboxEnvGitConfig decodes GIT_CONFIG_COUNT/KEY_n/VALUE_n the way git
// does, and fails on a count that disagrees with the keys present: git reads
// exactly COUNT pairs and dies on a missing one.
func sandboxEnvGitConfig(t *testing.T, env map[string]string) map[string]string {
	t.Helper()
	n, err := strconv.Atoi(env["GIT_CONFIG_COUNT"])
	if err != nil {
		t.Fatalf("GIT_CONFIG_COUNT = %q, want a number: %v", env["GIT_CONFIG_COUNT"], err)
	}
	out := map[string]string{}
	for i := 0; i < n; i++ {
		k, okK := env["GIT_CONFIG_KEY_"+strconv.Itoa(i)]
		v, okV := env["GIT_CONFIG_VALUE_"+strconv.Itoa(i)]
		if !okK || !okV {
			t.Fatalf("GIT_CONFIG_COUNT=%d but pair %d is missing: %#v", n, i, env)
		}
		out[k] = v
	}
	if _, extra := env["GIT_CONFIG_KEY_"+strconv.Itoa(n)]; extra {
		t.Fatalf("GIT_CONFIG_KEY_%d is set beyond GIT_CONFIG_COUNT=%d, and git will never read it", n, n)
	}
	return out
}

// The mount point is written once, as sessionWorkDir. The sandbox's git is
// told that one path is safe — the path the worktree is mounted at and the
// directory the harness starts in — and nothing wider.
func TestEverySandboxTrustsTheMountPointAndNothingWider(t *testing.T) {
	for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
		t.Run(body, func(t *testing.T) {
			env := sandboxEnvNew(t, nil)
			spec := sandboxEnvCreate(t, env, body)
			cfg := sandboxEnvGitConfig(t, spec.Env)

			if got := cfg["safe.directory"]; got != sessionWorkDir {
				t.Fatalf("safe.directory = %q, want the mount point %q", got, sessionWorkDir)
			}
			if spec.WorkDir != cfg["safe.directory"] || spec.Mounts[0].ContainerPath != cfg["safe.directory"] {
				t.Errorf("safe.directory %q, WorkDir %q and the worktree mount %q must be one path",
					cfg["safe.directory"], spec.WorkDir, spec.Mounts[0].ContainerPath)
			}
			for k, v := range spec.Env {
				if strings.HasPrefix(k, "GIT_CONFIG_VALUE_") && v == "*" {
					t.Errorf("%s = \"*\" switches git's ownership check off for every repository", k)
				}
			}
			// No identity can mean an invented one: dev@<container>.
			if got := cfg["user.useConfigOnly"]; got != "true" {
				t.Errorf("user.useConfigOnly = %q, want \"true\" so git never invents an identity", got)
			}
		})
	}
}

// The identity is the resolver's answer for THIS worktree, asked with the
// host path, and it reaches both halves git reads: author and committer.
func TestTheSandboxCommitsAsTheIdentityResolvedForItsWorktree(t *testing.T) {
	who := &sandboxEnvIdentity{id: GitIdentity{Name: "Ada Lovelace", Email: "ada@example.invalid"}}
	for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
		t.Run(body, func(t *testing.T) {
			env := sandboxEnvNew(t, func(d *Deps) { d.GitIdentity = who.resolve })
			spec := sandboxEnvCreate(t, env, body)

			asked := who.calls()
			if len(asked) == 0 || asked[len(asked)-1] != env.dir {
				t.Fatalf("the resolver was asked about %#v, want the worktree's host path %q", asked, env.dir)
			}
			for k, want := range map[string]string{
				"GIT_AUTHOR_NAME":     "Ada Lovelace",
				"GIT_COMMITTER_NAME":  "Ada Lovelace",
				"GIT_AUTHOR_EMAIL":    "ada@example.invalid",
				"GIT_COMMITTER_EMAIL": "ada@example.invalid",
			} {
				if got := spec.Env[k]; got != want {
					t.Errorf("%s = %q, want %q", k, got, want)
				}
			}
		})
	}
}

// Neither an explicit identity nor one the host's git can name: the session
// still starts, the sandbox gets no identity at all, git in it is left unable
// to guess one, and the resolver's sentence is what the log says.
func TestNoIdentityStartsTheSessionAndLogsTheResolversSentence(t *testing.T) {
	const why = "no git identity: set PANACEA_APP_GIT_NAME and PANACEA_APP_GIT_EMAIL"
	who := &sandboxEnvIdentity{err: errors.New(why)}
	logs := &sandboxEnvLog{}
	env := sandboxEnvNew(t, func(d *Deps) {
		d.GitIdentity = who.resolve
		d.Log = slog.New(slog.NewTextHandler(logs, nil))
	})
	spec := sandboxEnvCreate(t, env, sessionsCreateBody)

	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if v, ok := spec.Env[k]; ok {
			t.Errorf("%s = %q with no identity resolved; git would commit as it", k, v)
		}
	}
	if got := sandboxEnvGitConfig(t, spec.Env)["user.useConfigOnly"]; got != "true" {
		t.Errorf("user.useConfigOnly = %q with no identity: git would invent dev@<container>", got)
	}
	if !strings.Contains(logs.String(), why) {
		t.Errorf("the log does not carry the resolver's sentence %q:\n%s", why, logs.String())
	}
}

// A nil resolver is a hand-built API's zero value, and it means no identity —
// not a panic at the first create.
func TestANilIdentityResolverMeansNoIdentity(t *testing.T) {
	env := sandboxEnvNew(t, nil)
	spec := sandboxEnvCreate(t, env, sessionsCreateBody)
	if v, ok := spec.Env["GIT_AUTHOR_NAME"]; ok {
		t.Errorf("GIT_AUTHOR_NAME = %q with no resolver", v)
	}
}

// sandboxEnvForbidden are the two settings that would make the CA fix a
// security regression: one replaces the system bundle, the other stops Node
// verifying anything.
var sandboxEnvForbidden = []string{"SSL_CERT_FILE", "NODE_TLS_REJECT_UNAUTHORIZED"}

// With no CA bundle configured the sandbox is exactly what it was before this
// fix: no extra mount and no NODE_EXTRA_CA_CERTS.
func TestNoCABundleMountsNothingAndSetsNothing(t *testing.T) {
	env := sandboxEnvNew(t, nil)
	spec := sandboxEnvCreate(t, env, sessionsCreateBody)

	if v, ok := spec.Env["NODE_EXTRA_CA_CERTS"]; ok {
		t.Errorf("NODE_EXTRA_CA_CERTS = %q with no bundle configured", v)
	}
	for _, m := range spec.Mounts {
		if m.ContainerPath == sessionCABundlePath {
			t.Errorf("a CA mount %#v with no bundle configured", m)
		}
	}
	if len(spec.Mounts) != 2 {
		t.Errorf("Mounts = %#v, want only the worktree and its conversation store", spec.Mounts)
	}
	for _, k := range sandboxEnvForbidden {
		if v, ok := spec.Env[k]; ok {
			t.Errorf("%s = %q", k, v)
		}
	}
}

// With one, every harness gets it read-only at the one in-sandbox path, and
// NODE_EXTRA_CA_CERTS names that path — the constant, not the host path, which
// means nothing inside a Linux container.
func TestACABundleIsMountedReadOnlyAndNamedByNodeExtraCACerts(t *testing.T) {
	for _, tc := range []struct {
		body       string
		wantMounts int
	}{
		{sessionsCreateBody, 3},  // worktree, conversation store, CA
		{sandboxEnvShellBody, 2}, // a shell has no conversation store
	} {
		t.Run(tc.body, func(t *testing.T) {
			bundle := filepath.Join(t.TempDir(), "corp-root.pem")
			env := sandboxEnvNew(t, func(d *Deps) { d.CABundle = bundle })
			spec := sandboxEnvCreate(t, env, tc.body)

			if len(spec.Mounts) != tc.wantMounts {
				t.Fatalf("Mounts = %#v, want %d", spec.Mounts, tc.wantMounts)
			}
			ca := spec.Mounts[len(spec.Mounts)-1]
			if ca.HostPath != bundle || ca.ContainerPath != sessionCABundlePath {
				t.Errorf("CA mount = %#v, want %q at %q", ca, bundle, sessionCABundlePath)
			}
			if !ca.ReadOnly {
				t.Error("the CA bundle is mounted writable; an agent could rewrite what the sandbox trusts")
			}
			if spec.Mounts[0].ReadOnly {
				t.Error("the worktree became read-only; ReadOnly belongs to the CA mount alone")
			}
			if got := spec.Env["NODE_EXTRA_CA_CERTS"]; got != sessionCABundlePath {
				t.Errorf("NODE_EXTRA_CA_CERTS = %q, want %q", got, sessionCABundlePath)
			}
			for _, k := range sandboxEnvForbidden {
				if v, ok := spec.Env[k]; ok {
					t.Errorf("%s = %q", k, v)
				}
			}
		})
	}
}

// Restart rebuilds the spec, so the environment is rebuilt too: the resolver is
// asked again — a user who fixed their identity after the first create gets a
// sandbox that can commit — and the CA mount and safe.directory are still there.
func TestRestartAsksForTheIdentityAgainAndKeepsTheCAAndSafeDirectory(t *testing.T) {
	who := &sandboxEnvIdentity{err: errors.New("no identity yet")}
	bundle := filepath.Join(t.TempDir(), "corp-root.pem")
	env := sandboxEnvNew(t, func(d *Deps) {
		d.GitIdentity = who.resolve
		d.CABundle = bundle
	})
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	who.set(GitIdentity{Name: "Ada Lovelace", Email: "ada@example.invalid"}, nil)
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)
	spec := env.be.spec(t, 0)

	if asked := who.calls(); len(asked) != 1 || asked[0] != env.dir {
		t.Fatalf("the restart asked the resolver about %#v, want once, about %q", asked, env.dir)
	}
	if spec.Env["GIT_AUTHOR_NAME"] != "Ada Lovelace" || spec.Env["GIT_COMMITTER_EMAIL"] != "ada@example.invalid" {
		t.Errorf("the restarted sandbox commits as %q <%q>, want the identity resolved at restart",
			spec.Env["GIT_AUTHOR_NAME"], spec.Env["GIT_COMMITTER_EMAIL"])
	}
	if got := sandboxEnvGitConfig(t, spec.Env)["safe.directory"]; got != sessionWorkDir {
		t.Errorf("safe.directory = %q after a restart", got)
	}
	ca := spec.Mounts[len(spec.Mounts)-1]
	if ca.ContainerPath != sessionCABundlePath || !ca.ReadOnly || spec.Env["NODE_EXTRA_CA_CERTS"] != sessionCABundlePath {
		t.Errorf("the restarted sandbox lost its CA bundle: mounts %#v, NODE_EXTRA_CA_CERTS %q",
			spec.Mounts, spec.Env["NODE_EXTRA_CA_CERTS"])
	}
}
