package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetupGitIdentityWritesOnlyExplicitGlobalIdentity(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	p := createJourneyProject(t, mux, "identity-app")
	if _, err := RunCommand(context.Background(), "git", []string{"config", "user.name", "Repository Impersonation"}, p.Path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(p.Path, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	w := journeyRequest(t, mux, "GET", "/v1/setup/git/identity", nil, 200)
	var identity GitIdentity
	_ = json.Unmarshal(w.Body.Bytes(), &identity)
	if identity.Name != "" || identity.Email != "" {
		t.Fatalf("repo-local or LUCI fixture identity leaked into global identity: %#v", identity)
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "tools")); !os.IsNotExist(err) {
		t.Fatalf("GET created a directory: %v", err)
	}
	requested := GitIdentity{Name: "A Real Person", Email: "real.person@example.com"}
	journeyRequest(t, mux, "POST", "/v1/setup/git/identity", requested, 200)
	w = journeyRequest(t, mux, "GET", "/v1/setup/git/identity", nil, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &identity)
	if identity != requested {
		t.Fatalf("identity did not round-trip through actual Git: %#v", identity)
	}
	after, _ := os.ReadFile(filepath.Join(p.Path, ".git", "config"))
	if !bytes.Equal(before, after) {
		t.Fatal("identity setup changed repository-local config")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/git/identity", GitIdentity{Name: "Injected\nName", Email: requested.Email}, 400)
	journeyRequest(t, mux, "POST", "/v1/setup/git/identity", GitIdentity{Name: requested.Name, Email: "Person <other@example.com>"}, 400)
	// This is an explicit host operation; a project cannot become its working directory.
	s.mu.Lock()
	s.config.ProjectsDir = s.stateDir
	s.mu.Unlock()
	journeyRequest(t, mux, "POST", "/v1/setup/git/identity", requested, 409)
}

func TestSetupSSHKeyIsRealReusableAndNeverOverwritesExistingKeys(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("OpenSSH is required to verify key generation")
	}
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	w := journeyRequest(t, mux, "POST", "/v1/setup/ssh/key", map[string]any{}, 200)
	var first SSHKey
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(first.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.PublicKey, "ssh-ed25519 ") || !strings.Contains(first.Fingerprint, "SHA256:") || strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Fatalf("bad or leaked key response: %s", w.Body.String())
	}
	w = journeyRequest(t, mux, "POST", "/v1/setup/ssh/key", map[string]any{}, 200)
	var second SSHKey
	_ = json.Unmarshal(w.Body.Bytes(), &second)
	if second != first {
		t.Fatalf("repeated setup rotated the key: %#v vs %#v", first, second)
	}
	if err := os.WriteFile(first.PrivateKeyPath+".pub", []byte("ssh-ed25519 AAAAwrong luci\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/setup/ssh/key", map[string]any{}, 409)
	after, _ := os.ReadFile(first.PrivateKeyPath)
	if !bytes.Equal(private, after) {
		t.Fatal("mismatched public key caused private key replacement")
	}
	if err := os.Remove(first.PrivateKeyPath); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/setup/ssh/key", map[string]any{}, 409)
	if _, err := os.Stat(first.PrivateKeyPath); !os.IsNotExist(err) {
		t.Fatalf("public-only existing key caused regeneration: %v", err)
	}
}

func runtimeSourceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, relative := range []string{"images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh", "sandboxd/go.mod", "sandboxd/vendor/modules.txt"} {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readRuntimeJob(t *testing.T, mux http.Handler) RuntimeJob {
	t.Helper()
	w := journeyRequest(t, mux, "GET", "/v1/setup/runtime/install", nil, 200)
	var job RuntimeJob
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	return job
}

func awaitRuntimeJob(t *testing.T, mux http.Handler) RuntimeJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job := readRuntimeJob(t, mux)
		if job.Status != "installing" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("runtime job did not finish")
	return RuntimeJob{}
}

func TestRuntimeInstallIsAsyncCachedDurableAndVerifiesImage(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	var calls atomic.Int32
	var buildStarted sync.Once
	started := make(chan struct{})
	release := make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		calls.Add(1)
		actualDir, actualErr := os.Stat(dir)
		expectedDir, expectedErr := os.Stat(s.runtimeDir)
		if exe != "docker" || actualErr != nil || expectedErr != nil || !os.SameFile(actualDir, expectedDir) {
			return "", errors.New("runtime command escaped the bundled source")
		}
		if args[0] == "build" {
			if !strings.Contains(strings.Join(args, "|"), "images/sandbox/Dockerfile|--tag|luci/sandbox:dev|.") {
				return "", errors.New("unexpected runtime build")
			}
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < 5*time.Minute {
				return "", errors.New("initial install was given a short probe timeout")
			}
			buildStarted.Do(func() { close(started) })
			select {
			case <-release:
				return "built", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		if strings.Join(args, " ") != "image inspect luci/sandbox:dev --format {{.Id}}" {
			return "", errors.New("unexpected image verification")
		}
		return "sha256:verified-fixture\n", nil
	}
	if readRuntimeJob(t, mux).Status != "idle" || calls.Load() != 0 {
		t.Fatal("GET ran an install or Docker probe")
	}
	w := journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	var accepted RuntimeJob
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatalf("async build did not start: %#v", readRuntimeJob(t, mux))
	}
	w = journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	var repeated RuntimeJob
	_ = json.Unmarshal(w.Body.Bytes(), &repeated)
	if accepted.ID == "" || repeated.ID != accepted.ID || readRuntimeJob(t, mux).Status != "installing" || calls.Load() != 1 {
		t.Fatal("pending install duplicated a job or GET called Docker")
	}
	// Simulate a second process reading the interrupted on-disk state. It must
	// not pretend the build succeeded, nor automatically launch a replacement.
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), RuntimeDir: s.runtimeDir, Runner: s.runner})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.runtimeJob.Status != "unknown" || calls.Load() != 1 {
		t.Fatal("restart fabricated readiness or restarted a job")
	}
	unblock.Do(func() { close(release) })
	final := awaitRuntimeJob(t, mux)
	if final.Status != "ready" || final.ID != accepted.ID || calls.Load() != 2 {
		t.Fatalf("build did not verify its image: %#v, calls=%d", final, calls.Load())
	}
	restarted, err = New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), RuntimeDir: s.runtimeDir})
	if err != nil || restarted.runtimeJob.Status != "ready" {
		t.Fatalf("result was not persisted: %v", err)
	}
}

func TestRuntimeInstallFailsHonestlyAndRejectsCallerSource(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 409)
	s.runtimeDir = runtimeSourceFixture(t)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]string{"sourceDir": t.TempDir()}, 400)
	s.runner = func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("Docker engine is not running")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	job := awaitRuntimeJob(t, mux)
	if job.Status != "failed" || !strings.Contains(job.Message, "Docker engine is not running") {
		t.Fatalf("failure was hidden: %#v", job)
	}
	s.runner = func(_ context.Context, _ string, args []string, _ string) (string, error) {
		if args[0] == "build" {
			return "claimed success", nil
		}
		return "", nil
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	if job := awaitRuntimeJob(t, mux); job.Status != "failed" {
		t.Fatalf("unverified image accepted: %#v", job)
	}
}
