package gitx_test

// The §HGH tests (task-02 §HGH). Markers are written by a script in t.TempDir(), and each test
// asserts that the marker path does not exist after the call. Each layer is tested alone: a
// test of an H1 or H2 override runs with a Scope whose H3 rule admits the key.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// marker is a sh script that appends to a marker file, and the marker's path. git runs it by
// its absolute forward-slash path (Git for Windows reads the shebang).
func marker(t *testing.T, name string) (script, ran string) {
	t.Helper()
	dir := t.TempDir()
	ran = filepath.ToSlash(filepath.Join(dir, name+".ran"))
	script = filepath.ToSlash(filepath.Join(dir, name+".sh"))
	body := "#!/bin/sh\necho ran >> '" + ran + "'\ncat >/dev/null 2>&1\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, ran
}

func requireNoMarker(t *testing.T, ran string) {
	t.Helper()
	if _, err := os.Stat(ran); err == nil {
		t.Fatalf("marker %s exists: the repository's program ran", ran)
	}
}

func project(dir string, class gitx.Class, args ...string) gitx.Call {
	c := gitx.Call{Dir: dir, Args: args, Class: class, Scope: gitx.Project}
	if class == gitx.Remote {
		c.RemoteURL = "https://example.invalid/r.git"
	}
	return c
}

func runOK(t *testing.T, run gitx.Runner, c gitx.Call) string {
	t.Helper()
	res, err := run.Run(context.Background(), c)
	if err != nil {
		t.Fatalf("%v: %v", c.Args, err)
	}
	return strings.TrimSpace(string(res.Stdout))
}

var h1Pairs = []string{
	"core.fsmonitor=false", "core.sshCommand=", "protocol.allow=never", "protocol.file.allow=always",
	"core.askPass=", "commit.gpgSign=false", "tag.gpgSign=false", "log.showSignature=false",
	"submodule.recurse=false", "fetch.recurseSubmodules=false", "credential.helper=",
}

// prefixOf runs the helper process in argv mode and returns the -c values before Args[0].
func prefixOf(t *testing.T, x *gitx.Exec, c gitx.Call) (pairs []string, literal bool) {
	t.Helper()
	c.Env = map[string]string{"GITX_FAKE_MODE": "argv"}
	out := strings.Split(strings.ReplaceAll(runOK(t, x, c), "\r\n", "\n"), "\n")
	for i := 0; i < len(out) && out[i] != c.Args[0]; i++ {
		switch {
		case out[i] == "-c" && i+1 < len(out):
			pairs = append(pairs, out[i+1])
			i++
		case out[i] == "--literal-pathspecs":
			literal = true
		}
	}
	return pairs, literal
}

// TestHGHPrefixOnEveryClass: H1 on every class; https on a Remote call only; the machine's
// helpers on a Remote call or a Scope None call only (carried b01-2 Minor 2).
func TestHGHPrefixOnEveryClass(t *testing.T) {
	x, owned := fake(t)
	x.Hardening.Credential = []string{"credential.helper=machine"}
	dir := t.TempDir() // no .git: a Project call here is not pinned, and the fake lists no config
	for _, tc := range []struct {
		c              gitx.Call
		https, helpers bool
	}{
		{project(dir, gitx.Read, "status"), false, false},
		{project(dir, gitx.LocalWrite, "status"), false, false},
		{project(dir, gitx.Remote, "status"), true, true},
		{fakeCall(owned, gitx.Read, nil), false, true},
		{fakeCall(owned, gitx.Remote, nil), true, true},
	} {
		pairs, literal := prefixOf(t, x, tc.c)
		want := append([]string{"core.hooksPath=" + x.Hardening.HooksDir}, h1Pairs...)
		for _, p := range want {
			if !contains(pairs, p) {
				t.Errorf("class %d scope %d: prefix lacks -c %s", tc.c.Class, tc.c.Scope, p)
			}
		}
		if !literal {
			t.Errorf("class %d scope %d: prefix lacks --literal-pathspecs", tc.c.Class, tc.c.Scope)
		}
		if got := contains(pairs, "protocol.https.allow=always"); got != tc.https {
			t.Errorf("class %d scope %d: https allowed = %v, want %v", tc.c.Class, tc.c.Scope, got, tc.https)
		}
		if got := contains(pairs, "credential.helper=machine"); got != tc.helpers {
			t.Errorf("class %d scope %d: machine helper = %v, want %v", tc.c.Class, tc.c.Scope, got, tc.helpers)
		}
	}
}

func TestHGHPathspecsAreLiteral(t *testing.T) {
	r := gitxtest.NewRepo(t)
	r.Commit("two", map[string]string{"app/[slug]/page.tsx": "a\n", "app/s/page.tsx": "a\n"})
	r.Commit("both", map[string]string{"app/[slug]/page.tsx": "b\n", "app/s/page.tsx": "b\n"})
	got := runOK(t, r.Runner(), project(r.Dir, gitx.Read, "diff", "--name-only", "HEAD~1", "HEAD", "--", "app/[slug]/page.tsx"))
	if got != "app/[slug]/page.tsx" {
		t.Fatalf("diff listed %q, want exactly app/[slug]/page.tsx", got)
	}
}

func TestHGHFsmonitorNeverRuns(t *testing.T) {
	r := gitxtest.NewRepo(t)
	script, ran := marker(t, "fsmonitor")
	r.Git("config", "core.fsmonitor", script)
	runOK(t, r.Runner(), project(r.Dir, gitx.Read, "status", "--porcelain"))
	requireNoMarker(t, ran)
}

func TestHGHReferenceTransactionHookNeverRuns(t *testing.T) {
	r := gitxtest.NewRepo(t)
	_, ran := marker(t, "hook")
	hook := "#!/bin/sh\necho ran >> '" + ran + "'\n"
	if err := os.WriteFile(filepath.Join(r.Dir, ".git", "hooks", "reference-transaction"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	runOK(t, r.Runner(), project(r.Dir, gitx.LocalWrite, "update-ref", "refs/heads/moved", "HEAD"))
	requireNoMarker(t, ran)
	if a, b := r.Git("rev-parse", "refs/heads/moved"), r.Git("rev-parse", "HEAD"); a != b {
		t.Fatalf("refs/heads/moved = %s, want HEAD %s", a, b)
	}
}

func TestHGHUploadPackIsNeverTheRepositorys(t *testing.T) {
	src := gitxtest.NewRepo(t)
	r := gitxtest.NewRepo(t)
	script, ran := marker(t, "uploadpack")
	r.Git("remote", "add", "origin", filepath.ToSlash(src.Dir))
	r.Git("config", "remote.origin.uploadpack", script)
	runOK(t, r.Runner(), project(r.Dir, gitx.LocalWrite, "fetch", "-q", "origin"))
	requireNoMarker(t, ran)
	if got, want := r.Git("rev-parse", "refs/remotes/origin/main"), src.Git("rev-parse", "HEAD"); got != want {
		t.Fatalf("fetched origin/main = %s, want %s", got, want)
	}
}

func TestHGHReceivePackIsNeverTheRepositorys(t *testing.T) {
	r := gitxtest.NewRepo(t)
	bare := &gitxtest.Repo{T: t, Dir: t.TempDir(), Env: r.Env}
	bare.Git("init", "-q", "--bare")
	script, ran := marker(t, "receivepack")
	r.Git("remote", "add", "origin", filepath.ToSlash(bare.Dir))
	r.Git("config", "remote.origin.receivepack", script)
	runOK(t, r.Runner(), project(r.Dir, gitx.LocalWrite, "push", "-q", "origin", "main"))
	requireNoMarker(t, ran)
	if got, want := bare.Git("rev-parse", "refs/heads/main"), r.Git("rev-parse", "HEAD"); got != want {
		t.Fatalf("pushed main = %s, want %s", got, want)
	}
}

func requireUntrusted(t *testing.T, err error, key string) {
	t.Helper()
	var u *gitx.UntrustedConfig
	if !errors.As(err, &u) || !strings.EqualFold(u.Key, key) || u.Scope != gitx.Project {
		t.Fatalf("err = %v, want *UntrustedConfig{Key: %s, Scope: Project}", err, key)
	}
}

func TestHGHFilterDriverIsRefused(t *testing.T) {
	r := gitxtest.NewRepo(t)
	script, ran := marker(t, "filter")
	r.Git("config", "filter.x.clean", script)
	for name, body := range map[string]string{".gitattributes": "* filter=x\n", "README.md": "changed\n"} {
		if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := r.Runner().Run(context.Background(), project(r.Dir, gitx.Read, "status"))
	requireUntrusted(t, err, "filter.x.clean")
	requireNoMarker(t, ran)
}

func TestHGHProjectRefusalList(t *testing.T) {
	r := gitxtest.NewRepo(t)
	run := r.Runner()
	everyCall := [][2]string{
		{"include.path", "x.inc"}, {"includeIf.gitdir:/x/.path", "x.inc"}, {"filter.x.clean", "cat"},
		{"diff.x.textconv", "cat"}, {"diff.x.command", "cat"}, {"diff.external", "cat"},
		{"merge.x.driver", "cat"}, {"core.gitProxy", "cat"}, {"core.alternateRefsCommand", "cat"},
		{"extensions.worktreeConfig", "true"}, {"remote.origin.pushurl", "https://x.invalid/r"},
		{"url.https://x.invalid/.insteadOf", "https://y.invalid/"}, {"url.https://x.invalid/.pushInsteadOf", "https://y.invalid/"},
		{"remote.origin.proxy", "http://127.0.0.1:9"}, {"remote.origin.proxyAuthMethod", "basic"}, {"remote.origin.vcs", "x"},
	}
	remoteOnly := [][2]string{{"http.sslCAInfo", "x.pem"}, {"http.https://x.invalid/.proxy", "http://127.0.0.1:9"}, {"credential.trace", "x.log"}}
	admitted := [][2]string{{"core.autocrlf", "input"}, {"lfs.url", "https://x.invalid/lfs"}, {"remote.origin.url", "https://x.invalid/r"}}
	check := func(kv [2]string, refusedOn map[gitx.Class]bool) {
		r.Git("config", kv[0], kv[1])
		defer r.Git("config", "--unset-all", kv[0])
		for _, class := range classes {
			_, err := run.Run(context.Background(), project(r.Dir, class, "rev-parse", "HEAD"))
			if refusedOn[class] {
				var u *gitx.UntrustedConfig
				if !errors.As(err, &u) || !strings.EqualFold(u.Key, kv[0]) {
					t.Errorf("%s on class %d: err = %v, want *UntrustedConfig naming it", kv[0], class, err)
				}
			} else if err != nil {
				t.Errorf("%s on class %d: err = %v, want admitted", kv[0], class, err)
			}
		}
	}
	all := map[gitx.Class]bool{gitx.Read: true, gitx.LocalWrite: true, gitx.Remote: true}
	for _, kv := range everyCall {
		check(kv, all)
	}
	for _, kv := range remoteOnly {
		check(kv, map[gitx.Class]bool{gitx.Remote: true})
	}
	for _, kv := range admitted {
		check(kv, nil)
	}
}

// TestHGHProjectAdmitsRealConfig: this repository's own shape passes H3 on every class, and
// (carried b01-2 Minor 1) its repo-local helper is keyed at the test remote, which answers 401,
// so git does ask for a credential and the helper would run if H1's reset did not clear it.
func TestHGHProjectAdmitsRealConfig(t *testing.T) {
	r := gitxtest.NewRepo(t)
	var hits atomic.Int32
	url := gitxtest.HTTPSRemote(t, r, http401(&hits))
	script, ran := marker(t, "helper")
	for _, kv := range [][2]string{
		{"credential.usehttppath", "true"},
		{"credential.https://github.com/lcfrai/panaceaapp.helper", "!sh '" + script + "'"},
		{"credential." + url + ".helper", "!sh '" + script + "'"},
		{"remote.origin.gh-resolved", "base"}, {"core.longpaths", "true"},
	} {
		r.Git("config", kv[0], kv[1])
	}
	run := r.Runner()
	runOK(t, run, project(r.Dir, gitx.Read, "rev-parse", "HEAD"))
	runOK(t, run, project(r.Dir, gitx.LocalWrite, "update-ref", "refs/heads/t", "HEAD"))
	c := project(r.Dir, gitx.Remote, "ls-remote", url+"/r.git")
	c.RemoteURL = url + "/r.git"
	_, err := run.Run(context.Background(), c)
	if _, ok := gitx.AsError(err); !ok {
		t.Fatalf("Remote call = %v, want git's own failure (no credential), not a refusal", err)
	}
	if hits.Load() == 0 {
		t.Fatal("the remote saw no request: the helper was never asked for, so the test cannot fail")
	}
	requireNoMarker(t, ran)
}

func TestHGHConfigParseIsNULSafe(t *testing.T) {
	r := gitxtest.NewRepo(t)
	run := r.Runner()
	r.Git("config", "user.name", "a\nfilter.x.clean=y")
	runOK(t, run, project(r.Dir, gitx.Read, "rev-parse", "HEAD"))
	r.Git("config", "filter.x.clean", "cat")
	_, err := run.Run(context.Background(), project(r.Dir, gitx.Read, "rev-parse", "HEAD"))
	requireUntrusted(t, err, "filter.x.clean")
	r.Git("config", "--unset-all", "filter.x.clean")
	f, err := os.OpenFile(filepath.Join(r.Dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("[filter \"x\"]\n\trequired\n")
	f.Close()
	if got := r.Git("config", "--local", "--list", "-z"); !strings.Contains(got, "filter.x.required\x00") {
		t.Fatalf("fixture: the listing does not hold a valueless filter.x.required: %q", got)
	}
	_, err = run.Run(context.Background(), project(r.Dir, gitx.Read, "rev-parse", "HEAD"))
	requireUntrusted(t, err, "filter.x.required")
}

func TestHGHUntrustedConfigHidesTheValue(t *testing.T) {
	// The part's row names credential.helper, which H3's Project row admits (H1 clears it), so
	// the secret rides on two keys H3 does refuse (task 02 departure).
	r := gitxtest.NewRepo(t)
	run := r.Runner()
	for _, tc := range []struct {
		key   string
		class gitx.Class
	}{{"http.https://example.invalid/.extraHeader", gitx.Remote}, {"filter.x.clean", gitx.Read}} {
		r.Git("config", tc.key, "Authorization: Bearer s3cr3t")
		_, err := run.Run(context.Background(), project(r.Dir, tc.class, "rev-parse", "HEAD"))
		var u *gitx.UntrustedConfig
		if !errors.As(err, &u) {
			t.Fatalf("%s: err = %v, want *UntrustedConfig", tc.key, err)
		}
		if strings.Contains(err.Error(), "s3cr3t") || strings.Contains(u.Key+u.Dir, "s3cr3t") {
			t.Fatalf("%s: the value reached the refusal: %q", tc.key, err.Error())
		}
		r.Git("config", "--unset-all", tc.key)
	}
}

func TestHGHRemoteNeedsAURL(t *testing.T) {
	x, owned := fake(t)
	spawns := gitxtest.Spy(t, x)
	c := fakeCall(owned, gitx.Remote, nil)
	c.RemoteURL = ""
	if _, err := x.Run(context.Background(), c); !errors.Is(err, gitx.ErrCallShape) {
		t.Fatalf("err = %v, want ErrCallShape", err)
	}
	if len(*spawns) != 0 {
		t.Fatal("git started for a Remote call with no RemoteURL")
	}
}

// http401 answers every request 401 with a Basic challenge, so git asks for a credential.
func http401(hits *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="t"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
}

// clearProxyEnv keeps a machine proxy out of a loopback test: Hardening.Proxy is "" there.
func clearProxyEnv(t *testing.T) {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(k, "")
	}
}

func TestHGHRemoteOverridesBeatAURLScopedKey(t *testing.T) {
	clearProxyEnv(t)
	var served, proxied atomic.Int32
	o := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { served.Add(1) }))
	defer o.Close()
	p, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	go func() {
		for {
			c, err := p.Accept()
			if err != nil {
				return
			}
			proxied.Add(1)
			c.Close()
		}
	}()
	u, pURL := o.URL+"/r.git", "http://"+p.Addr().String()
	r := gitxtest.NewRepo(t)
	for _, kv := range [][2]string{{"remote.origin.url", u}, {"http." + u + ".sslVerify", "false"}, {"http." + u + ".proxy", pURL}} {
		r.Git("config", kv[0], kv[1])
	}
	if got := r.Git("config", "--get-urlmatch", "http.sslVerify", u); got != "false" {
		t.Fatalf("precondition: http.sslVerify for U = %q, want false", got)
	}
	if got := r.Git("config", "--get-urlmatch", "http.proxy", u); got != pURL {
		t.Fatalf("precondition: http.proxy for U = %q, want %q", got, pURL)
	}
	x := gitxtest.NewExec(t)
	x.Hardening.OwnedRoot = r.Dir // Scope None: H3 would refuse http.* on a Remote Project call
	c := gitx.Call{Dir: r.Dir, Args: []string{"ls-remote", "origin"}, Class: gitx.Remote, Scope: gitx.None, RemoteURL: u}
	_, err = r.RunnerOf(x).Run(context.Background(), c)
	if err == nil {
		t.Fatal("ls-remote succeeded: the repository's sslVerify=false beat H2")
	}
	if n := proxied.Load(); n != 0 {
		t.Fatalf("the repository's proxy saw %d connections", n)
	}
	if n := served.Load(); n != 0 {
		t.Fatalf("O served %d requests over an unverified certificate", n)
	}
}

func TestHGHRepoCredentialHelperNeverRuns(t *testing.T) {
	for _, key := range []string{"credential.helper", "credential.https://example.invalid.helper"} {
		t.Run(key, func(t *testing.T) {
			r := gitxtest.NewRepo(t)
			script, ran := marker(t, "helper")
			r.Git("config", key, "!sh '"+script+"'")
			x := gitxtest.NewExec(t)
			x.Hardening.OwnedRoot = r.Dir
			x.Hardening.Credential = []string{"credential.helper=!f() { echo username=machine; echo password=machinepw; }; f"}
			c := gitx.Call{Dir: r.Dir, Args: []string{"credential", "fill"}, Class: gitx.Read, Scope: gitx.None,
				Stdin: []byte("protocol=https\nhost=example.invalid\n\n")}
			out := runOK(t, r.RunnerOf(x), c)
			requireNoMarker(t, ran)
			if !strings.Contains(out, "username=machine") {
				t.Fatalf("credential fill = %q, want the machine helper's answer", out)
			}
		})
	}
}

func TestHGHCallShape(t *testing.T) {
	x, owned := fake(t)
	spawns := gitxtest.Spy(t, x)
	ok := fakeCall(owned, gitx.Read, nil)
	rows := map[string]gitx.Call{}
	c := ok
	c.Args = []string{"-c", "core.hooksPath=/x", "status"}
	rows["a caller's -c first"] = c
	c = ok
	c.Scope = gitx.Unset
	rows["Scope Unset"] = c
	c = ok
	c.Dir = t.TempDir()
	rows["Scope None outside OwnedRoot"] = c
	for _, k := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE"} {
		c = ok
		c.Env = map[string]string{k: owned}
		rows["Env "+k] = c
	}
	for name, c := range rows {
		if _, err := x.Run(context.Background(), c); !errors.Is(err, gitx.ErrCallShape) {
			t.Errorf("%s: err = %v, want ErrCallShape", name, err)
		}
	}
	if _, err := (&gitx.Exec{}).Run(context.Background(), ok); !errors.Is(err, gitx.ErrCallShape) {
		t.Errorf("Exec{} (no HooksDir): err = %v, want ErrCallShape", err)
	}
	if len(*spawns) != 0 {
		t.Fatalf("git started %d times for refused calls", len(*spawns))
	}
}

// copyDir copies a small directory tree (a test repository's .git).
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHGHCommondirCannotRedirect(t *testing.T) {
	r := gitxtest.NewRepo(t)
	r.Git("config", "user.name", "Real")
	evil := filepath.Join(t.TempDir(), "evil")
	copyDir(t, filepath.Join(r.Dir, ".git"), evil)
	script, ran := marker(t, "commondir")
	evilRepo := &gitxtest.Repo{T: t, Dir: r.Dir, Env: r.Env}
	evilRepo.Git("config", "--file", filepath.Join(evil, "config"), "user.name", "EVIL")
	evilRepo.Git("config", "--file", filepath.Join(evil, "config"), "filter.x.clean", script)
	for name, body := range map[string]string{".gitattributes": "* filter=x\n", "README.md": "changed\n",
		filepath.Join(".git", "commondir"): filepath.ToSlash(evil) + "\n"} {
		if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.Git("config", "user.name"); got != "EVIL" {
		t.Fatalf("precondition: raw git read user.name %q through commondir, want EVIL", got)
	}
	run := r.Runner()
	gotCommon := runOK(t, run, project(r.Dir, gitx.Read, "rev-parse", "--git-common-dir"))
	if !strings.EqualFold(filepath.Clean(gotCommon), filepath.Join(r.Dir, ".git")) {
		t.Fatalf("--git-common-dir = %q, want %q", gotCommon, filepath.Join(r.Dir, ".git"))
	}
	if got := runOK(t, run, project(r.Dir, gitx.Read, "config", "user.name")); got != "Real" {
		t.Fatalf("config user.name = %q, want Real", got)
	}
	runOK(t, run, project(r.Dir, gitx.Read, "status"))
	requireNoMarker(t, ran)
}

// TestHGHLogNeverVerifiesSignatures is carried b01-2 N1: a user's log.showSignature=true would
// run their gpg.program over an agent-authored signed commit on task 16's `log -1`.
func TestHGHLogNeverVerifiesSignatures(t *testing.T) {
	r := gitxtest.NewRepo(t)
	script, ran := marker(t, "gpg")
	r.Git("config", "log.showSignature", "true")
	r.Git("config", "gpg.program", script)
	obj := "tree " + r.Git("rev-parse", "HEAD^{tree}") + "\nparent " + r.Git("rev-parse", "HEAD") +
		"\nauthor A <a@x.invalid> 1700000000 +0000\ncommitter A <a@x.invalid> 1700000000 +0000\n" +
		"gpgsig -----BEGIN PGP SIGNATURE-----\n \n iQEzBAABCAAdFiEE\n -----END PGP SIGNATURE-----\n\nsigned\n"
	f := filepath.Join(t.TempDir(), "commit")
	if err := os.WriteFile(f, []byte(obj), 0o644); err != nil {
		t.Fatal(err)
	}
	sha := r.Git("hash-object", "-t", "commit", "-w", f)
	runOK(t, r.Runner(), project(r.Dir, gitx.Read, "log", "-1", "--format=%H %s", sha))
	requireNoMarker(t, ran)
	r.Git("log", "-1", "--format=%H", sha) // precondition, after the Exec call: raw git runs it
	if _, err := os.Stat(ran); err != nil {
		t.Fatalf("precondition: raw git log never ran gpg.program (%v), so this test cannot fail", err)
	}
}
