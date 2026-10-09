package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
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

// sandboxEnvIdentity is a Deps.GitIdentity double that counts how often it
// was asked. It is given no worktree to be asked about (X-1).
type sandboxEnvIdentity struct {
	mu    sync.Mutex
	asked int
	id    GitIdentity
	err   error
}

func (f *sandboxEnvIdentity) resolve(context.Context) (GitIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	return f.id, f.err
}

func (f *sandboxEnvIdentity) set(id GitIdentity, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.id, f.err = id, err
}

func (f *sandboxEnvIdentity) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
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
			// The primary's .git/worktrees/<id>/gitdir files name host paths,
			// so every linked worktree looks prunable from in here, and an
			// automatic gc would delete their metadata on the host.
			if got := cfg["gc.worktreePruneExpire"]; got != "never" {
				t.Errorf("gc.worktreePruneExpire = %q, want \"never\" so no gc in the sandbox prunes a host worktree", got)
			}
			// With that metadata read-only, gc's reflog expiry cannot lock a
			// linked worktree's HEAD and gc exits 128. A detached auto gc that
			// fails writes .git/gc.log, and the HOST's git then skips its own
			// auto gc for a day (gc.logExpiry). So the sandbox never starts one.
			if got := cfg["gc.auto"]; got != "0" {
				t.Errorf("gc.auto = %q, want \"0\" so no auto gc in the sandbox leaves the host a gc.log", got)
			}
		})
	}
}

// The identity is the resolver's answer, asked at the create with no worktree
// to read (X-1), and it reaches both halves git reads: author and committer.
func TestTheSandboxCommitsAsTheIdentityTheHostResolved(t *testing.T) {
	who := &sandboxEnvIdentity{id: GitIdentity{Name: "Ada Lovelace", Email: "ada@example.invalid"}}
	for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
		t.Run(body, func(t *testing.T) {
			env := sandboxEnvNew(t, func(d *Deps) { d.GitIdentity = who.resolve })
			before := who.calls()
			spec := sandboxEnvCreate(t, env, body)

			if asked := who.calls() - before; asked != 1 {
				t.Fatalf("the create asked the resolver %d times, want once", asked)
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
	const why = "no git identity: set LUCI_GIT_NAME and LUCI_GIT_EMAIL"
	// A half answer alongside the error, because Deps.GitIdentity's contract is
	// "an error means no identity" — not "an error means whatever came back".
	who := &sandboxEnvIdentity{id: GitIdentity{Name: "Half Resolved"}, err: errors.New(why)}
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
		{sandboxEnvShellBody, 3}, // worktree, read-only CA, private shell home; no provider store
	} {
		t.Run(tc.body, func(t *testing.T) {
			bundle := filepath.Join(t.TempDir(), "corp-root.pem")
			env := sandboxEnvNew(t, func(d *Deps) { d.CABundle = bundle })
			spec := sandboxEnvCreate(t, env, tc.body)

			if len(spec.Mounts) != tc.wantMounts {
				t.Fatalf("Mounts = %#v, want %d", spec.Mounts, tc.wantMounts)
			}
			caIndex := 2 // Claude conversation store precedes its CA mount.
			if tc.body == sandboxEnvShellBody {
				caIndex = 1 // Shell's private home is appended after the existing CA.
				stored, err := env.st.Get(spec.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				home := resumeAssertShellHomeIsolation(t, env, stored, spec)
				want := []backend.Mount{{HostPath: env.dir, ContainerPath: sessionWorkDir}, {HostPath: bundle, ContainerPath: sessionCABundlePath, ReadOnly: true}, {HostPath: home, ContainerPath: shellHomeDir}}
				if !slices.Equal(spec.Mounts, want) {
					t.Fatal("terminal lost exact worktree/CA/home mount isolation")
				}
			}
			ca := spec.Mounts[caIndex]
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
	host := &sandboxEnvLineEndings{eol: GitLineEndings{AutoCRLF: "true", EOL: "crlf"}}
	bundle := filepath.Join(t.TempDir(), "corp-root.pem")
	env := sandboxEnvNew(t, func(d *Deps) {
		d.GitIdentity = who.resolve
		d.GitLineEndings = host.resolve
		d.CABundle = bundle
	})
	admin := sandboxEnvPrimary(t, env.dir, true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	who.set(GitIdentity{Name: "Ada Lovelace", Email: "ada@example.invalid"}, nil)
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)
	spec := env.be.spec(t, 0)

	if asked := who.calls(); asked != 1 {
		t.Fatalf("the restart asked the resolver %d times, want once", asked)
	}
	if spec.Env["GIT_AUTHOR_NAME"] != "Ada Lovelace" || spec.Env["GIT_COMMITTER_EMAIL"] != "ada@example.invalid" {
		t.Errorf("the restarted sandbox commits as %q <%q>, want the identity resolved at restart",
			spec.Env["GIT_AUTHOR_NAME"], spec.Env["GIT_COMMITTER_EMAIL"])
	}
	cfg := sandboxEnvGitConfig(t, spec.Env)
	if got := cfg["safe.directory"]; got != sessionWorkDir {
		t.Errorf("safe.directory = %q after a restart", got)
	}
	if asked := host.calls(); asked != 1 {
		t.Errorf("the restart asked for the line endings %d times, want once", asked)
	}
	if cfg["core.autocrlf"] != "true" || cfg["core.eol"] != "crlf" || cfg["gc.worktreePruneExpire"] != "never" || cfg["gc.auto"] != "0" {
		t.Errorf("after a restart core.autocrlf %q, core.eol %q, gc.worktreePruneExpire %q, gc.auto %q; want true, crlf, never, 0",
			cfg["core.autocrlf"], cfg["core.eol"], cfg["gc.worktreePruneExpire"], cfg["gc.auto"])
	}
	if got := sandboxEnvAdminMounts(spec); len(got) != 1 || got[0].HostPath != admin || !got[0].ReadOnly {
		t.Errorf("after a restart the linked-worktree metadata mounts are %#v, want %q read-only", got, admin)
	}
	ca := spec.Mounts[len(spec.Mounts)-1]
	if ca.ContainerPath != sessionCABundlePath || !ca.ReadOnly || spec.Env["NODE_EXTRA_CA_CERTS"] != sessionCABundlePath {
		t.Errorf("the restarted sandbox lost its CA bundle: mounts %#v, NODE_EXTRA_CA_CERTS %q",
			spec.Mounts, spec.Env["NODE_EXTRA_CA_CERTS"])
	}
}

// sandboxEnvLineEndings is a Deps.GitLineEndings double that counts how often
// it was asked. It is given no worktree to be asked about (X-1).
type sandboxEnvLineEndings struct {
	mu    sync.Mutex
	asked int
	eol   GitLineEndings
	err   error
}

func (f *sandboxEnvLineEndings) resolve(context.Context) (GitLineEndings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	return f.eol, f.err
}

func (f *sandboxEnvLineEndings) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

// sandboxEnvCreates is how many sandboxes the fake backend was asked for.
func sandboxEnvCreates(env *sessionsEnv) int {
	env.be.mu.Lock()
	defer env.be.mu.Unlock()
	return len(env.be.specs)
}

// sandboxEnvAdminDir is where a primary worktree's linked-worktree metadata
// sits inside the sandbox: under the one mount point, never a second literal.
const sandboxEnvAdminDir = sessionWorkDir + "/.git/worktrees"

// sandboxEnvAdminMounts is every mount of the sandbox at sandboxEnvAdminDir.
func sandboxEnvAdminMounts(spec backend.Spec) []backend.Mount {
	var out []backend.Mount
	for _, m := range spec.Mounts {
		if m.ContainerPath == sandboxEnvAdminDir {
			out = append(out, m)
		}
	}
	return out
}

// sandboxEnvPrimary makes the test's worktree a primary one, whose .git is a
// directory, and returns the host path of its .git/worktrees. With linked it
// also holds one linked worktree's metadata, whose gitdir file names a host
// path, the way git on Windows writes it.
func sandboxEnvPrimary(t *testing.T, dir string, linked bool) string {
	t.Helper()
	admin := filepath.Join(dir, ".git", "worktrees")
	if !linked {
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		return admin
	}
	if err := os.MkdirAll(filepath.Join(admin, "linked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(admin, "linked", "gitdir"), []byte("C:/work/linked/.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return admin
}

// The host's answer, from its own config and asked about no worktree (X-1), is
// what the sandbox's git converts line endings with, for every harness. Three
// answers, so a sandbox that hard-coded any one of them passes at most one.
func TestTheSandboxConvertsLineEndingsAsTheHostsOwnConfigSays(t *testing.T) {
	for _, want := range []GitLineEndings{
		{AutoCRLF: "true", EOL: "crlf"},  // Git for Windows' installer default
		{AutoCRLF: "false", EOL: "crlf"}, // Windows, autocrlf off, core.eol native
		{AutoCRLF: "input", EOL: "lf"},
	} {
		for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
			t.Run(want.AutoCRLF+"/"+want.EOL+" "+body, func(t *testing.T) {
				host := &sandboxEnvLineEndings{eol: want}
				env := sandboxEnvNew(t, func(d *Deps) { d.GitLineEndings = host.resolve })
				spec := sandboxEnvCreate(t, env, body)

				if asked := host.calls(); asked != 1 {
					t.Fatalf("the line endings were asked for %d times, want once", asked)
				}
				cfg := sandboxEnvGitConfig(t, spec.Env)
				if cfg["core.autocrlf"] != want.AutoCRLF || cfg["core.eol"] != want.EOL {
					t.Errorf("core.autocrlf %q, core.eol %q; want the host's %q, %q",
						cfg["core.autocrlf"], cfg["core.eol"], want.AutoCRLF, want.EOL)
				}
			})
		}
	}
}

// With no answer, or one the sandbox's git would read differently from the
// host's, there is no safe guess: the create fails before anything exists. No
// record, and no sandbox whose git would rewrite line endings.
func TestALineEndingAnswerTheSandboxCannotUseFailsTheCreateAndLeavesNothing(t *testing.T) {
	for name, host := range map[string]*sandboxEnvLineEndings{
		"an error": {err: errors.New("git config in C:\\work: exit status 128: fatal: bad config line 3")},
		// native means CRLF to Git for Windows and LF to the sandbox's git.
		"native":      {eol: GitLineEndings{AutoCRLF: "false", EOL: "native"}},
		"no eol":      {eol: GitLineEndings{AutoCRLF: "true"}},
		"no autocrlf": {eol: GitLineEndings{EOL: "lf"}},
		"not a word":  {eol: GitLineEndings{AutoCRLF: "yes", EOL: "lf"}},
		// Half an answer alongside an error is still an error.
		"half and an error": {eol: GitLineEndings{AutoCRLF: "true", EOL: "crlf"}, err: errors.New("git config: exit status 1")},
	} {
		t.Run(name, func(t *testing.T) {
			env := sandboxEnvNew(t, func(d *Deps) { d.GitLineEndings = host.resolve })
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

// At restart the answer is asked for again, before the destroy, so a failure
// leaves the old sandbox alive and the record naming it.
func TestARestartThatCannotReadTheLineEndingsKeepsTheOldSandbox(t *testing.T) {
	host := &sandboxEnvLineEndings{err: errors.New("exec: \"git\": executable file not found in %PATH%")}
	env := sandboxEnvNew(t, func(d *Deps) { d.GitLineEndings = host.resolve })
	seeded := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if host.calls() != 1 {
		t.Errorf("the restart asked for the line endings %d times, want once", host.calls())
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
}

// A wiring that forgot the resolver would reopen the defect silently, so New
// refuses it with a sentence naming the field.
func TestNewRefusesDepsWithNoLineEndingResolver(t *testing.T) {
	var deps Deps
	sandboxEnvNew(t, func(d *Deps) { deps = *d })
	deps.GitLineEndings = nil
	if _, err := New(deps); err == nil || !strings.Contains(err.Error(), "Deps.GitLineEndings") {
		t.Errorf("New with no GitLineEndings: err %v, want one naming Deps.GitLineEndings", err)
	}
}

// A primary worktree's linked-worktree metadata is mounted over itself,
// read-only. The sandbox's git still sees every linked worktree, so it still
// refuses to check out a branch one of them holds. But every gitdir there
// names a host path, so each looks prunable from in here, and no prune, gc,
// repair or add run in the sandbox can now change it on the host. The
// directory is made when there is none yet, so a linked worktree added while
// the session runs is covered too.
func TestAPrimaryWorktreesLinkedWorktreeMetadataIsReadOnlyInTheSandbox(t *testing.T) {
	for _, linked := range []bool{true, false} {
		for _, body := range []string{sessionsCreateBody, sandboxEnvShellBody} {
			t.Run(strconv.FormatBool(linked)+" "+body, func(t *testing.T) {
				env := sandboxEnvNew(t, nil)
				admin := sandboxEnvPrimary(t, env.dir, linked)
				spec := sandboxEnvCreate(t, env, body)

				got := sandboxEnvAdminMounts(spec)
				if len(got) != 1 || got[0].HostPath != admin || !got[0].ReadOnly {
					t.Fatalf("linked-worktree metadata mounts = %#v, want exactly %q at %q, read-only", got, admin, sandboxEnvAdminDir)
				}
				if spec.Mounts[0].ContainerPath != sessionWorkDir || spec.Mounts[0].ReadOnly {
					t.Errorf("Mounts[0] = %#v, want the worktree itself, writable", spec.Mounts[0])
				}
				if info, err := os.Stat(admin); err != nil || !info.IsDir() {
					t.Errorf("the host's %s is not a directory (err %v); Docker would make one, root-owned, or fail", admin, err)
				}
				if linked {
					if b, err := os.ReadFile(filepath.Join(admin, "linked", "gitdir")); err != nil || string(b) != "C:/work/linked/.git\n" {
						t.Errorf("the linked worktree's gitdir is now %q (err %v)", b, err)
					}
				}
			})
		}
	}
}

// A linked worktree's .git is a file naming the host, and a directory with no
// .git is not a repository. Neither has linked-worktree metadata to protect,
// so neither gets a mount, and nothing is made in it. A .git that is a
// symlink is not followed either: nothing is made or mounted through a link
// whose target the sandbox resolves against its own filesystem.
func TestAWorktreeWhoseGitIsNotADirectoryGetsNoMetadataMount(t *testing.T) {
	for name, setup := range map[string]func(t *testing.T, dir string) string{
		"a linked worktree": func(t *testing.T, dir string) string {
			if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: C:/work/primary/.git/worktrees/linked\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(dir, ".git", "worktrees")
		},
		"no .git": func(t *testing.T, dir string) string { return filepath.Join(dir, ".git", "worktrees") },
		// A .git that is a symlink or a junction no longer "gets no metadata
		// mount": X-3 review 3 m-D fails the create for it. The pin moved to
		// TestAWorktreeWhoseGitIsAReparsePointFailsTheCreate.
	} {
		t.Run(name, func(t *testing.T) {
			env := sandboxEnvNew(t, nil)
			notMade := setup(t, env.dir)
			spec := sandboxEnvCreate(t, env, sessionsCreateBody)

			for _, m := range spec.Mounts {
				if strings.HasPrefix(m.ContainerPath, sessionWorkDir+"/") {
					t.Errorf("mount %#v inside the worktree of %s", m, name)
				}
			}
			if _, err := os.Stat(notMade); err == nil {
				t.Errorf("%s was made in %s", notMade, name)
			}
		})
	}
}
