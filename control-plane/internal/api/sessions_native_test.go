package api

import (
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func nativeStateMount(t *testing.T, s backend.Spec) string {
	t.Helper()
	for _, m := range s.Mounts {
		if m.ContainerPath == "/home/dev/.luci-agent" {
			return m.HostPath
		}
	}
	t.Fatal("missing isolated provider home")
	return ""
}
func nativeWrite(t *testing.T, root, path, value string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCodexDeviceSignInRestartAndStoppedForkPreserveSeparateHomes(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	rec := env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","authOnly":true}`)
	first := sessionsDecodeOne(t, rec, 201)
	sessionsAwaitWatch(t, done)
	firstSpec := env.be.spec(t, 0)
	firstDir := nativeStateMount(t, firstSpec)
	if !slices.Contains(firstSpec.Args, "--device-auth") || firstSpec.Env["CODEX_HOME"] != "/home/dev/.luci-agent" {
		t.Fatal("sign-in is not delegated to official CLI/private home")
	}
	nativeWrite(t, firstDir, "auth.json", `{"tokens":{"access_token":"fixture-oauth-token"}}`)
	record, _ := env.st.Get(first.ID)
	if _, found, err := env.api.nativeResumable(record); err != nil || found {
		t.Fatal("auth-only state was offered as a conversation")
	}
	nativeWrite(t, firstDir, "sessions/2026/fixture.jsonl", "old conversation")
	nativeCodexTransientAliases(t, firstDir)
	rec = env.do(t, http.MethodPost, "/v1/sessions/"+first.ID+"/restart", `{}`)
	sessionsDecodeOne(t, rec, 200)
	sessionsAwaitWatch(t, done)
	secondSpec := env.be.spec(t, 1)
	secondDir := nativeStateMount(t, secondSpec)
	if firstDir == secondDir || slices.Contains(secondSpec.Args, "--device-auth") {
		t.Fatal("restart reused sign-in command or writable home")
	}
	auth, err := os.ReadFile(filepath.Join(secondDir, "auth.json"))
	if err != nil || !strings.Contains(string(auth), "fixture-oauth-token") {
		t.Fatal("restart lost credentials")
	}
	if _, err = os.Stat(filepath.Join(secondDir, "sessions")); !os.IsNotExist(err) {
		t.Fatal("clean restart retained old context")
	}
	body := `{"worktreeId":"w-4a77b1c9","harness":"codex","resume":"` + first.ID + `"}`
	sessionsProblemCode(t, env.do(t, http.MethodPost, "/v1/sessions", body), 409)
	nativeWrite(t, secondDir, "sessions/2026/new.jsonl", "new conversation")
	nativeCodexTransientAliases(t, secondDir)
	env.do(t, http.MethodDelete, "/v1/sessions/"+first.ID, "")
	fork := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", body), 201)
	sessionsAwaitWatch(t, done)
	third := env.be.spec(t, 2)
	thirdDir := nativeStateMount(t, third)
	if !slices.Contains(third.Args, "fork") || thirdDir == secondDir {
		t.Fatal("fork reused source state")
	}
	nativeWrite(t, thirdDir, "sessions/2026/new.jsonl", "fork edited independently")
	old, _ := os.ReadFile(filepath.Join(secondDir, "sessions/2026/new.jsonl"))
	if string(old) != "new conversation" {
		t.Fatal("fork modified source")
	}
	original, _ := env.st.Get(first.ID)
	if original.Status != registry.StatusExited {
		t.Fatal("source unexpectedly restarted")
	}
	if err := env.api.forgetNativeState(original); err != nil {
		t.Fatal(err)
	}
	child, _ := env.st.Get(fork.ID)
	if _, found, err := env.api.nativeResumable(child); err != nil || !found {
		t.Fatal("forgetting source removed fork")
	}
}

func TestNativeStateSnapshotsRejectLinksAndForgetCannotEscape(t *testing.T) {
	env := sessionsNewEnv(t)
	outside := t.TempDir()
	nativeWrite(t, outside, "secret.txt", "must not copy")
	source := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(source, "auth.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyNativeState(source, target, "codex", false); err == nil {
		t.Fatal("copied a symlink out of provider state")
	}
	if err := env.api.forgetNativeState(registry.Session{WorktreeID: "w-4a77b1c9", Harness: "codex", ConversationID: "../../outside"}); err == nil {
		t.Fatal("forget accepted path traversal")
	}
	data, _ := os.ReadFile(filepath.Join(outside, "secret.txt"))
	if string(data) != "must not copy" {
		t.Fatal("outside file changed")
	}
}

func TestNativeSecretsStayOutOfArgvAndDockerLocalEndpointIsExplicit(t *testing.T) {
	env := map[string]string{"OPENAI_API_KEY": "fixture-secret-not-for-argv"}
	args := nativeArgs("codex", []string{"--no-daemon"}, env, false)
	if strings.Contains(strings.Join(args, " "), env["OPENAI_API_KEY"]) || !strings.Contains(strings.Join(args, " "), `env_key="OPENAI_API_KEY"`) {
		t.Fatal("secret routing is not environment-only")
	}
	nativeArgs("codex", nil, env, true)
	if env["OPENAI_API_KEY"] != "" {
		t.Fatal("device sign-in inherited conflicting API credential")
	}
	env = map[string]string{"OPENCODE_CONFIG_CONTENT": `{"provider":{"luci-local":{"options":{"baseURL":"http://127.0.0.1:11434/v1"}}}}`}
	nativeDockerEndpoint(env)
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil || !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "http://host.docker.internal:11434/v1") {
		t.Fatal("container localhost was not translated to its model host")
	}
}

func TestOpenCodeRefusesAnImplicitPaidProvider(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsProblemCode(t, env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"opencode"}`), 409)
	if len(env.be.specs) != 0 {
		t.Fatal("unconfigured model started a sandbox")
	}
}

func TestWindowsNativeStorePreservesLegacyHomesAndPrefersMigratedCompactHomes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Docker physical path compatibility")
	}
	env := sessionsNewEnv(t)
	conversation := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	legacy := filepath.Join(env.api.deps.ConversationsDir, conversationpath.LegacyRelative(sessionConversationOwner, "setup-codex", "codex", conversation))
	compact := filepath.Join(env.api.deps.ConversationsDir, conversationpath.NativeRelative(sessionConversationOwner, "setup-codex", "codex", conversation))
	if got, err := env.api.nativeStore("setup-codex", "codex", conversation); err != nil || got != compact {
		t.Fatal("new Windows sessions do not use compact private homes")
	}
	nativeWrite(t, legacy, "auth.json", "legacy-owned-fixture")
	if got, err := env.api.nativeStore("setup-codex", "codex", conversation); err != nil || got != legacy {
		t.Fatal("standalone upgrade discarded existing provider home")
	}
	nativeWrite(t, compact, "auth.json", "migrated-and-rotated-fixture")
	if got, err := env.api.nativeStore("setup-codex", "codex", conversation); err != nil || got != compact {
		t.Fatal("managed worker reselected retired legacy home")
	}
	data, _ := os.ReadFile(filepath.Join(legacy, "auth.json"))
	if string(data) != "legacy-owned-fixture" {
		t.Fatal("native path selection modified legacy credentials")
	}
}
