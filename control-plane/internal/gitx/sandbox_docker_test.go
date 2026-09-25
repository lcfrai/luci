//go:build docker

package gitx

// The sandbox half of plan task 01: git in panacea-app/sandbox:dev over a clone the host
// made, as a phase 3 lane session sees it. Run with -tags docker. Each call is one
// `docker run --rm` (containers named pa-p3-t01-*, no panacea-app.managed label, so no
// control plane's reaper counts them) with the clone bind-mounted at /home/dev/work and the
// entrypoint replaced by /bin/sh: sandboxd plays no part in what git does.

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	sandboxImage    = "panacea-app/sandbox:dev"
	sandboxWorkDir  = "/home/dev/work"        // phase 2's sessionWorkDir
	sandboxHooksDir = "/opt/panacea/githooks" // task 05's sessionHooksPath
	// Task 03's NoFetchURL and NoPushURL (task-03.md, step 4a).
	noFetchURL = "/panacea-app/no-remote/the-cockpit-fetches-origin-and-upstream-for-this-worktree"
	noPushURL  = "/panacea-app/no-remote/the-cockpit-lands-this-worktree-and-pushes-the-branch"
)

// sessionPairs is sessionEnv's GIT_CONFIG_KEY_n/VALUE_n pairs on feat/phase-2-cockpit, in its
// order (control-plane/internal/api/sessions.go:426-442 at 31576b7, which contains 5a4d0d8;
// re-read 2026-09-25). core.autocrlf and core.eol are the values a lane session is sent, task
// 03's step-4 constants (R4.2); the two file keys are sessionGitNoFile, what a host whose git
// reads no global excludes or attributes file sends. Written out once here because this task
// precedes gate G1 and gitx cannot import internal/api; task 05's
// TestSandboxLaneCloneChecksOutLF reads sessionEnv itself, so a drift is caught there.
func sessionPairs(autocrlf, eol string) [][2]string {
	return [][2]string{
		{"safe.directory", sandboxWorkDir},
		{"user.useConfigOnly", "true"},
		{"gc.worktreePruneExpire", "never"},
		{"gc.auto", "0"},
		{"core.autocrlf", autocrlf},
		{"core.eol", eol},
		{"core.excludesFile", "/dev/null"},
		{"core.attributesFile", "/dev/null"},
	}
}

// dropPairs is pairs without the named keys.
func dropPairs(pairs [][2]string, keys ...string) [][2]string {
	var out [][2]string
	for _, kv := range pairs {
		if !slices.Contains(keys, kv[0]) {
			out = append(out, kv)
		}
	}
	return out
}

const sandboxAuthor = "Sandbox Pin <sandbox-pin@example.invalid>"

// sandboxEnv is the session environment for pairs: GIT_CONFIG_COUNT, the numbered pairs, and
// the injected identity as GIT_AUTHOR_* and GIT_COMMITTER_*.
func sandboxEnv(pairs [][2]string) []string {
	env := []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(len(pairs))}
	for i, kv := range pairs {
		n := strconv.Itoa(i)
		env = append(env, "GIT_CONFIG_KEY_"+n+"="+kv[0], "GIT_CONFIG_VALUE_"+n+"="+kv[1])
	}
	return append(env,
		"GIT_AUTHOR_NAME=Sandbox Pin", "GIT_AUTHOR_EMAIL=sandbox-pin@example.invalid",
		"GIT_COMMITTER_NAME=Sandbox Pin", "GIT_COMMITTER_EMAIL=sandbox-pin@example.invalid")
}

// needSandbox skips, with a sentence, unless docker answers and the image exists. The test
// builds no image.
func needSandbox(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not on PATH, so the sandbox half of the git pins cannot run")
	}
	out, err := exec.Command("docker", "image", "inspect", "--format", "{{.Id}} {{.Created}}", sandboxImage).CombinedOutput()
	if err != nil {
		t.Skipf("the image %s is absent (docker image inspect: %v: %s); build the phase 2 sandbox image and rerun",
			sandboxImage, err, strings.TrimSpace(string(out)))
	}
	t.Logf("image %s: %s", sandboxImage, strings.TrimSpace(string(out)))
}

// hostPath is p with Windows 8.3 short names (AU-DEV~1) expanded, the form a bind mount's
// source is given in.
func hostPath(t *testing.T, p string) string {
	t.Helper()
	long, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return long
}

type bind struct {
	host, container string
	readOnly        bool
}

var containerSeq atomic.Int64

// inSandbox runs script under /bin/sh in a fresh sandbox, with work bind-mounted at
// /home/dev/work, the working directory there, env set and binds added. The exit code is
// the docker client's, which `docker run` passes through from the container's process.
func inSandbox(t *testing.T, work string, env []string, binds []bind, script string) (stdout, stderr string, code int) {
	t.Helper()
	name := fmt.Sprintf("pa-p3-t01-%d-%d", time.Now().UnixNano()%1e9, containerSeq.Add(1))
	args := []string{"run", "--rm", "--name", name, "--entrypoint", "/bin/sh",
		"-w", sandboxWorkDir, "-v", hostPath(t, work) + ":" + sandboxWorkDir}
	for _, b := range binds {
		v := hostPath(t, b.host) + ":" + b.container
		if b.readOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	for _, kv := range env {
		args = append(args, "-e", kv)
	}
	args = append(args, sandboxImage, "-c", script)
	cmd := exec.Command("docker", args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("docker run: could not start: %v", err)
		}
	}
	code = cmd.ProcessState.ExitCode()
	if code == 125 {
		t.Fatalf("docker run itself failed (exit 125): %s", errb.String())
	}
	return out.String(), errb.String(), code
}

// sandboxFixture is a source repository of LF blobs, a symlink entry link -> t.txt, and an
// LFS pointer under `*.bin filter=lfs`, so one empty status in the sandbox covers review C2
// and Minors 13 and 15.
func sandboxFixture(t *testing.T) string {
	t.Helper()
	env := hermetic(t)
	src := newRepo(t, env, map[string]string{
		"a.txt": "x\ny\n", "sub/b.txt": "one\ntwo\n", "t.txt": "x\n", "a.bin": lfsPointer,
		".gitattributes": "*.bin filter=lfs -text\n",
	})
	// The link goes in last and by index only: a later `git add .` would stage its removal.
	oid := strings.TrimSpace(mustStdin(t, src, env, "t.txt", "hash-object", "-w", "--stdin"))
	must(t, src, env, "update-index", "--add", "--cacheinfo", "120000,"+oid+",link")
	must(t, src, env, "commit", "-q", "-m", "link")
	return src
}

// task03Argv is task 03 step 4's clone flags (task-03.md, Create step 4), with the
// line-ending pair given, so a test can run the I-1 mutant with true/crlf.
func task03Argv(autocrlf, eol string) []string {
	return []string{"--no-hardlinks",
		"--config", "core.autocrlf=" + autocrlf, "--config", "core.eol=" + eol,
		"--config", "core.checkStat=minimal", "--config", "core.trustctime=false",
		"--config", "core.longpaths=true", "--config", "core.symlinks=false",
		"--branch", "main", "--origin", "origin"}
}

// hostOfTheOwner is the production-shaped host: Git for Windows' core.autocrlf=true, plus
// core.symlinks=true and an LFS-shaped filter.lfs in the system file, cloning with task 03's
// NoFilterEnv.
func hostOfTheOwner(t *testing.T) gitEnv {
	t.Helper()
	system := lfsSystem(t, filepath.Join(t.TempDir(), "smudge-ran")) + "[core]\n\tsymlinks = true\n"
	return production(t, system).with(noFilterEnv...)
}

func lineCount(s string) int {
	return len(regexp.MustCompile(`(?m)^.+$`).FindAllString(s, -1))
}

// A clone made by task 03's argv on a production-shaped host has working git in the
// sandbox: an empty status (not just exit 0, review C2), an LF checkout, and a commit as the
// injected identity. The two controls run first.
func TestSandboxCloneHasWorkingGit(t *testing.T) {
	needSandbox(t)
	t.Parallel()
	src := sandboxFixture(t)
	host := hostOfTheOwner(t)

	// Control 1, C2's mechanism: a plain --local clone, and a session without the two
	// line-ending pairs.
	plain := clone(t, host, src, "--local")
	noEOL := sandboxEnv(dropPairs(sessionPairs("false", "lf"), "core.autocrlf", "core.eol"))
	out, errs, code := inSandbox(t, plain, noEOL, nil, "git status --porcelain")
	t.Logf("control 1 (plain --local clone, no line-ending pairs): exit %d, %d entries, stdout verbatim:\n%sstderr %q",
		code, lineCount(out), out, errs)
	if lineCount(out) == 0 {
		t.Logf("control 1: status is clean, so C2's mechanism did not reproduce here")
	}

	// Control 2, the I-1 mutant: task 03's clone with the project's answer true/crlf.
	mutant := clone(t, host, src, task03Argv("true", "crlf")...)
	out, errs, code = inSandbox(t, mutant, sandboxEnv(sessionPairs("true", "crlf")), nil, "git status --porcelain")
	onDisk := readFile(t, filepath.Join(mutant, "a.txt"))
	t.Logf("control 2 (true/crlf): status exit %d stdout %q stderr %q; a.txt on disk %q", code, out, errs, onDisk)
	if code != 0 || out != "" || onDisk != "x\r\ny\r\n" {
		t.Errorf("control 2: want an empty status and a CRLF a.txt, so that only the checkout below tells the two apart")
	}

	lane := clone(t, host, src, task03Argv("false", "lf")...)
	env := sandboxEnv(sessionPairs("false", "lf"))
	if out, errs, code := inSandbox(t, lane, env, nil, "git status --porcelain"); code != 0 || out != "" {
		t.Errorf("task 03 clone in the sandbox: status exit %d, stdout %q, stderr %q; want exit 0 and nothing", code, out, errs)
	}
	// The link's shape is asked in the sandbox too: the mutation check found that a real host
	// symlink (core.symlinks=true) also leaves status empty here, since Docker Desktop's bind
	// mount shows it as a Linux symlink, so only this line tells task 03's shape apart.
	out, errs, code = inSandbox(t, lane, env, nil,
		"rm a.txt && git checkout -- a.txt && if [ -L link ]; then echo symlink; else printf file: && cat link; fi")
	if code != 0 {
		t.Fatalf("rm a.txt && git checkout -- a.txt: exit %d, %q %q", code, out, errs)
	}
	if out != "file:t.txt" {
		t.Errorf("the sandbox sees link as %q; want file:t.txt, task 03's core.symlinks=false shape", out)
	}
	if got := readFile(t, filepath.Join(lane, "a.txt")); strings.Contains(got, "\r") {
		t.Errorf("the sandbox's checkout of a.txt wrote %q; want no \\r", got)
	}
	out, errs, code = inSandbox(t, lane, env, nil,
		"git commit -q --allow-empty -m x && git log -1 --format='%an <%ae>|%cn <%ce>'")
	if want := sandboxAuthor + "|" + sandboxAuthor + "\n"; code != 0 || out != want {
		t.Errorf("commit in the sandbox: exit %d, author|committer %q, stderr %q; want 0 and %q", code, out, errs, want)
	}
}

// With origin set as task 03's step 4a sets it, fetch, pull and push in the sandbox fail
// with a sentence naming who does the job. The control records what the Windows path git
// clone wrote does instead (review Minor 14).
func TestSandboxCloneCannotFetch(t *testing.T) {
	needSandbox(t)
	t.Parallel()
	src := sandboxFixture(t)
	host := hostOfTheOwner(t)
	lane := clone(t, host, src, task03Argv("false", "lf")...)
	env := sandboxEnv(sessionPairs("false", "lf"))

	origin := line(t, lane, host, "config", "--get", "remote.origin.url")
	out, errs, code := inSandbox(t, lane, env, nil, "git fetch")
	t.Logf("control: origin %q, git fetch in the sandbox: exit %d, stdout %q, stderr verbatim:\n%s", origin, code, out, errs)
	if code == 0 {
		t.Errorf("control: git fetch of a Windows-path origin exited 0 in the sandbox")
	}

	must(t, lane, host, "config", "remote.origin.url", noFetchURL)
	must(t, lane, host, "config", "remote.origin.pushurl", noPushURL)
	for _, c := range []struct{ script, names string }{
		{"git fetch", noFetchURL},
		{"git pull", noFetchURL},
		{"git push origin HEAD", noPushURL},
	} {
		out, errs, code := inSandbox(t, lane, env, nil, c.script)
		t.Logf("%s in the sandbox: exit %d, stdout %q, stderr verbatim:\n%s", c.script, code, out, errs)
		if code == 0 || !strings.Contains(errs, c.names) {
			t.Errorf("%s: exit %d, stderr %q; want non-zero and a sentence naming %s", c.script, code, errs, c.names)
		}
	}
}

// core.hooksPath appended after the fix's pairs, as task 05 appends it, points the sandbox's
// git at a bind-mounted hooks directory: a commit carries the hook's line.
func TestSandboxGitHonoursEnvHooksPath(t *testing.T) {
	needSandbox(t)
	t.Parallel()
	src := sandboxFixture(t)
	host := hostOfTheOwner(t)
	lane := clone(t, host, src, task03Argv("false", "lf")...)
	hooks := t.TempDir()
	writeFile(t, filepath.Join(hooks, "prepare-commit-msg"),
		"#!/bin/sh\nprintf '\\nPanacea-Session: s-sandbox\\n' >> \"$1\"\n")
	pairs := append(sessionPairs("false", "lf"), [2]string{"core.hooksPath", sandboxHooksDir})
	out, errs, code := inSandbox(t, lane, sandboxEnv(pairs),
		[]bind{{host: hooks, container: sandboxHooksDir, readOnly: true}},
		"git --version && ls -l "+sandboxHooksDir+" && git commit -q --allow-empty -m x && git log -1 --format=%B")
	t.Logf("sandbox: exit %d, stdout verbatim:\n%sstderr %q", code, out, errs)
	if code != 0 || !strings.Contains(out, "Panacea-Session: s-sandbox") {
		t.Errorf("commit under an env core.hooksPath: exit %d, stdout %q, stderr %q; want the hook's line", code, out, errs)
	}
}
