package api

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func shellStateMount(t *testing.T, spec backend.Spec) string {
	t.Helper()
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == shellHomeDir {
			if mount.ReadOnly {
				t.Fatal("terminal home is not writable")
			}
			return mount.HostPath
		}
	}
	t.Fatal("terminal has no private home")
	return ""
}

func TestShellHomeRetainsToolsAndUserProfileButIsolatesSessionWorktreeAndAccount(t *testing.T) {
	env := sessionsNewEnv(t)
	home, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001")
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, home, ".local/bin/my-tool", "#!/bin/sh\nprintf tool-result")
	nativeWrite(t, home, ".config/my-tool/config", "private setting")
	nativeWrite(t, home, ".bash_profile", "# user's profile must survive")
	if again, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001"); err != nil || again != home {
		t.Fatalf("restart lost its home: %v", err)
	}
	for name, want := range map[string]string{
		".local/bin/my-tool":     "#!/bin/sh\nprintf tool-result",
		".config/my-tool/config": "private setting",
		".bash_profile":          "# user's profile must survive",
	} {
		got, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Fatalf("restart changed %s: %v", name, err)
		}
	}
	otherAccount := sessionsNewEnv(t)
	for _, other := range []struct {
		api               *API
		worktree, session string
	}{
		{env.api, "w-4a77b1c9", "s-00000002"},
		{env.api, "w-11111111", "s-00000001"},
		{otherAccount.api, "w-4a77b1c9", "s-00000001"},
	} {
		separate, err := other.api.prepareShellHome(other.worktree, other.session)
		if err != nil || separate == home {
			t.Fatalf("terminal homes are not isolated: %v", err)
		}
		if _, err := os.Stat(filepath.Join(separate, ".local", "bin", "my-tool")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("another terminal inherited installed tools or settings")
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(home)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("terminal home is not private")
		}
	}
}

func TestShellHomeRejectsInvalidIdentityAndProjectStorageBeforeCreating(t *testing.T) {
	for _, ids := range [][2]string{
		{"../escape", "s-00000001"}, {"w-4a77b1c9", "../escape"},
		{"w-4a77b1c9", "s-0000000A"}, {"w-4a77b1c9", "s-00000001/child"},
		{"", "s-00000001"}, {"w-4a77b1c9", ""},
	} {
		env := sessionsNewEnv(t)
		if _, err := env.api.prepareShellHome(ids[0], ids[1]); err == nil {
			t.Fatal("invalid identity accepted")
		}
		if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid identity created state")
		}
	}
	env := sessionsNewEnv(t)
	env.api.deps.ProjectsDir = env.dir
	env.api.deps.ConversationsDir = filepath.Join(env.dir, "must-not-create")
	if _, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001"); err == nil {
		t.Fatal("terminal state accepted inside a writable project")
	}
	if _, err := os.Stat(env.api.deps.ConversationsDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected terminal state created a project directory")
	}
}

func TestShellHomeRefusesLinkedRootAndForgetPreservesOutsideFiles(t *testing.T) {
	for _, swap := range []string{"home", "parent"} {
		t.Run(swap, func(t *testing.T) {
			env := sessionsNewEnv(t)
			home, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001")
			if err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			nativeWrite(t, outside, "keep", "outside file")
			link := home
			if swap == "parent" {
				link = filepath.Dir(home)
			}
			if err := os.Rename(link, link+"-retained"); err != nil {
				t.Fatal(err)
			}
			if !forgetLink(t, link, outside) {
				t.Skip("directory links are unavailable")
			}
			if _, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001"); err == nil {
				t.Fatal("terminal mounted a linked private state directory")
			}
			if err := env.api.forgetShellHome(registry.Session{ID: "s-00000001", WorktreeID: "w-4a77b1c9", Harness: "shell"}); err == nil {
				t.Fatal("forget accepted a linked private state directory")
			}
			data, err := os.ReadFile(filepath.Join(outside, "keep"))
			if err != nil || string(data) != "outside file" {
				t.Fatal("linked external state was changed")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 1 {
				t.Fatal("setup wrote through a rejected directory link")
			}
		})
	}
}

func TestShellForgetRemovesChildLinksWithoutFollowingThem(t *testing.T) {
	env := sessionsNewEnv(t)
	home, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001")
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	nativeWrite(t, outside, "keep", "outside file")
	if !forgetLink(t, filepath.Join(home, "outside"), outside) {
		t.Skip("directory links are unavailable")
	}
	if err := env.api.forgetShellHome(registry.Session{ID: "s-00000001", WorktreeID: "w-4a77b1c9", Harness: "shell"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Forget left the terminal home")
	}
	data, err := os.ReadFile(filepath.Join(outside, "keep"))
	if err != nil || string(data) != "outside file" {
		t.Fatal("Forget followed a child link outside the home")
	}
}

func TestShellSessionRestartKeepsHomeKillRetainsAndForgetRemovesOnlyItsHome(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body := `{"worktreeId":"w-4a77b1c9","harness":"shell"}`
	first := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", body), 201)
	sessionsAwaitWatch(t, done)
	spec := env.be.spec(t, 0)
	home := shellStateMount(t, spec)
	if spec.Env["HOME"] != shellHomeDir || spec.Env["npm_config_prefix"] != shellHomeDir+"/.local" {
		t.Fatal("terminal installs do not target its private home")
	}
	for _, name := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CODEX_API_KEY", "OPENCODE_CONFIG_CONTENT", "CODEX_HOME"} {
		if spec.Env[name] != "" {
			t.Fatalf("terminal received provider configuration: %s", name)
		}
	}
	nativeWrite(t, home, ".local/bin/installed-tool", "installed bytes")
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/"+first.ID+"/restart", "{}"), 200)
	sessionsAwaitWatch(t, done)
	if shellStateMount(t, env.be.spec(t, 1)) != home {
		t.Fatal("Restart mounted another home")
	}
	data, err := os.ReadFile(filepath.Join(home, ".local", "bin", "installed-tool"))
	if err != nil || string(data) != "installed bytes" {
		t.Fatal("Restart lost installed tool bytes")
	}
	second := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", body), 201)
	sessionsAwaitWatch(t, done)
	secondHome := shellStateMount(t, env.be.spec(t, 2))
	if first.ID == second.ID || home == secondHome {
		t.Fatal("separate terminals share a home")
	}
	if rec := env.do(t, http.MethodDelete, "/v1/sessions/"+first.ID, ""); rec.Code != 200 {
		t.Fatal("Kill failed")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal("Kill erased retained tools")
	}
	if rec := env.do(t, http.MethodDelete, "/v1/sessions/"+first.ID+"?purge=true", ""); rec.Code != 204 {
		t.Fatal("Forget failed")
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Forget retained its private terminal home")
	}
	if _, err := os.Stat(secondHome); err != nil {
		t.Fatal("Forget removed another terminal's home")
	}
}

func TestShellHomeFailureDoesNotDestroyTheRunningSession(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	first := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"shell"}`), 201)
	sessionsAwaitWatch(t, done)
	home := shellStateMount(t, env.be.spec(t, 0))
	if err := os.Rename(home, home+"-retained"); err != nil {
		t.Fatal(err)
	}
	if !forgetLink(t, home, t.TempDir()) {
		t.Skip("directory links are unavailable")
	}
	if rec := env.do(t, http.MethodPost, "/v1/sessions/"+first.ID+"/restart", "{}"); rec.Code != 500 {
		t.Fatal("Restart accepted an unsafe terminal home")
	}
	if env.be.destroyCount() != 0 {
		t.Fatal("Restart destroyed the running terminal before validating its home")
	}
}

func TestShellBootstrapHomeAndInitialToolPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Linux image bootstrap runs under Bash; Linux CI executes this test")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash is unavailable")
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "images", "sandbox", "entrypoint.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Execute the actual image helper, without entering its root UID bootstrap
	// or launching a server. Only its fixed HOME mapping is under test here.
	_, tail, found := strings.Cut(string(source), "luci_bootstrap_home() {")
	body, _, ended := strings.Cut(tail, "\n}")
	if !found || !ended {
		t.Fatal("image is missing its terminal HOME handoff")
	}
	for _, tc := range []struct{ command, home, guest, want string }{
		{"/bin/bash", shellHomeDir, "", shellHomeDir},
		{"/bin/bash", "/arbitrary/path", "", "/home/dev"},
		{"codex", shellHomeDir, "", "/home/dev"},
		{"claude", shellHomeDir, "", "/home/dev"},
		{"codex", "/arbitrary/path", "1", "/home/dev/.luci-agent-home"},
	} {
		cmd := exec.Command(bash, "-p", "-c", "luci_bootstrap_home() {"+body+"\n}\nluci_bootstrap_home")
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LUCI_COMMAND=" + tc.command, "HOME=" + tc.home, "LUCI_MICROVM_FIXED_HOME=" + tc.guest}
		out, err := cmd.Output()
		if err != nil || string(out) != tc.want {
			t.Fatalf("image HOME handoff for %s: %q, %v", tc.command, out, err)
		}
	}
	env := sessionsNewEnv(t)
	home, err := env.api.prepareShellHome("w-4a77b1c9", "s-00000001")
	if err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(home, ".local", "bin", "fixture-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf installed-tool-ran"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", `. "$HOME/.bash_profile"; fixture-tool`)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + home}
	if out, err := cmd.Output(); err != nil || string(out) != "installed-tool-ran" {
		t.Fatalf("terminal profile cannot run its installed tool: %q, %v", out, err)
	}
}
