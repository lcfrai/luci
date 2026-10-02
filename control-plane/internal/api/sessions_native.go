package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const nativeAgentDir = "/home/dev/.luci-agent"

func nativeHarness(id string) bool {
	d, ok := harness.Lookup(harness.ID(id))
	return ok && d.NativeState()
}

func (a *API) nativeStore(worktreeID, provider, conversation string) (string, error) {
	if !nativeHarness(provider) || !harness.IsConversationID(conversation) {
		return "", errors.New("invalid native provider state identity")
	}
	if !sessionStoreSegment.MatchString(worktreeID) {
		return "", errors.New("invalid worktree state identity")
	}
	// Outside the entire Claude transcript mount: a Claude session must never
	// see another provider's auth or database through a nested bind directory.
	return filepath.Join(a.deps.ConversationsDir, "native", sessionConversationOwner, worktreeID, provider, conversation), nil
}

// Snapshot only stopped CLI homes. Their SQLite/WAL and provider transcripts
// remain upstream-owned formats, copied together rather than interpreted.
func copyNativeState(source, target string, credentialsOnly bool) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("provider state root is not a regular directory")
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	files, entries := 0, 0
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > 8192 {
			return errors.New("provider state exceeds the 8192-entry snapshot limit")
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("provider state contains a link or special file")
		}
		if name == "." {
			return nil
		}
		if info.IsDir() {
			if !credentialsOnly {
				return os.MkdirAll(filepath.Join(target, name), 0700)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return errors.New("provider state contains a special file")
		}
		if credentialsOnly && name != "auth.json" && filepath.ToSlash(name) != "data/opencode/auth.json" {
			return nil
		}
		files++
		total += info.Size()
		if files > 4096 || total > 256<<20 {
			return errors.New("provider state exceeds the 256 MiB/4096-file snapshot limit")
		}
		if err := hostpath.OneName(filepath.Join(source, name)); err != nil {
			return err
		}
		in, err := root.Open(name)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(filepath.Join(target, name)), 0700); err != nil {
			in.Close()
			return err
		}
		out, err := os.OpenFile(filepath.Join(target, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, info.Size()+1))
		in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != info.Size() {
			return errors.New("provider state changed during snapshot; stop its session first")
		}
		return nil
	})
}

func (a *API) prepareNativeState(worktreeID, provider, conversation string, source sessionResumeSource, credentialsOnly bool) (string, error) {
	dir, err := a.nativeStore(worktreeID, provider, conversation)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(a.deps.ConversationsDir, 0700); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(a.deps.ConversationsDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	rel, err := filepath.Rel(a.deps.ConversationsDir, dir)
	if err != nil {
		return "", err
	}
	if err = root.MkdirAll(filepath.Dir(rel), 0700); err != nil {
		return "", err
	}
	if err = root.Mkdir(rel, 0700); err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			_ = root.RemoveAll(rel)
		}
	}()
	if source.conversation != "" {
		from, err := a.nativeStore(source.worktreeID, provider, source.conversation)
		if err != nil {
			return "", err
		}
		if _, err = hostpath.Confine(a.deps.ConversationsDir, from); err != nil {
			return "", err
		}
		if err = copyNativeState(from, dir, credentialsOnly); err != nil {
			return "", err
		}
	}
	complete = true
	return dir, nil
}

func nativeEnvironment(provider string, env map[string]string, mode harness.PermissionMode) {
	if provider == string(harness.Codex) {
		env["CODEX_HOME"] = nativeAgentDir
		return
	}
	env["XDG_DATA_HOME"] = nativeAgentDir + "/data"
	env["XDG_CONFIG_HOME"] = nativeAgentDir + "/config"
	env["XDG_STATE_HOME"] = nativeAgentDir + "/state"
	env["XDG_CACHE_HOME"] = nativeAgentDir + "/cache"
	// Project configuration may supply untrusted instructions, but must not
	// redirect the configured local provider or install executable plugins.
	env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
	env["OPENCODE_DISABLE_MODELS_FETCH"] = "true"
	// Verified through the pinned CLI's debug-config command in the provider
	// smoke. Project configs must not override the selected permission mode.
	env["OPENCODE_DISABLE_PROJECT_CONFIG"] = "true"
	env["OPENCODE_PERMISSION"] = `{"*":"ask"}`
	if mode == harness.PermissionBypass {
		env["OPENCODE_PERMISSION"] = `{"*":"allow"}`
	}
	if mode == harness.PermissionPlan {
		env["OPENCODE_PERMISSION"] = `{"*":"ask","edit":"deny","bash":"deny"}`
	}
}

// Only static provider settings enter argv. A selected API key stays in the
// environment; ChatGPT sign-in uses Codex's own private credential file.
func nativeArgs(provider string, args []string, env map[string]string, authOnly bool) []string {
	if provider != string(harness.Codex) {
		return args
	}
	if authOnly {
		delete(env, "OPENAI_API_KEY")
		return []string{"-c", `cli_auth_credentials_store="file"`, "login", "--device-auth"}
	}
	if env["OPENAI_API_KEY"] == "" {
		return args
	}
	return append([]string{"-c", `model_provider="luci-openai"`, "-c", `model_providers.luci-openai={name="OpenAI API",base_url="https://api.openai.com/v1",env_key="OPENAI_API_KEY",wire_api="responses"}`}, args...)
}

// Container localhost is the agent itself, not the Docker host. Docker's
// host-gateway alias reaches a deliberately configured host model service.
func nativeDockerEndpoint(env map[string]string) {
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil {
		return
	}
	providers, _ := config["provider"].(map[string]any)
	provider, _ := providers["luci-local"].(map[string]any)
	options, _ := provider["options"].(map[string]any)
	base, _ := options["baseURL"].(string)
	u, err := url.Parse(base)
	if err != nil {
		return
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return
	}
	port := u.Port()
	u.Host = "host.docker.internal"
	if port != "" {
		u.Host = net.JoinHostPort(u.Host, port)
	}
	options["baseURL"] = u.String()
	data, err := json.Marshal(config)
	if err == nil {
		env["OPENCODE_CONFIG_CONTENT"] = string(data)
	}
}

func nativeMount(mounts []backend.Mount, path string) []backend.Mount {
	return append(mounts, backend.Mount{HostPath: path, ContainerPath: nativeAgentDir})
}

func (a *API) nativeResumable(sess registry.Session) (string, bool, error) {
	candidates := []string{sess.ConversationID}
	for i := len(sess.EarlierConversationIDs) - 1; i >= 0; i-- {
		candidates = append(candidates, sess.EarlierConversationIDs[i])
	}
	for _, conversation := range candidates {
		if conversation == "" {
			continue
		}
		dir, err := a.nativeStore(sess.WorktreeID, sess.Harness, conversation)
		if err != nil {
			return "", false, err
		}
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", false, errors.New("provider state is not a regular directory")
		}
		if _, err := hostpath.Confine(a.deps.ConversationsDir, dir); err != nil {
			return "", false, err
		}
		// Presence only, never parse or rewrite a provider's transcript/DB.
		history := filepath.Join(dir, "sessions")
		if sess.Harness == string(harness.OpenCode) {
			history = filepath.Join(dir, "data", "opencode", "opencode.db")
		}
		info, err = os.Lstat(history)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", false, errors.New("provider history is a link or special file")
		}
		if info.IsDir() || info.Mode().IsRegular() && info.Size() > 0 {
			return conversation, true, nil
		}
	}
	return "", false, nil
}

func (a *API) forgetNativeState(sess registry.Session) error {
	for _, id := range append([]string{sess.ConversationID}, sess.EarlierConversationIDs...) {
		if id == "" {
			continue
		}
		dir, err := a.nativeStore(sess.WorktreeID, sess.Harness, id)
		if err != nil {
			return err
		}
		if _, err = os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		// Never traverse an agent-created root link/junction during Forget.
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("provider state root is not a regular directory")
		}
		if _, err = hostpath.Confine(a.deps.ConversationsDir, dir); err != nil {
			return err
		}
		if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(a.deps.ConversationsDir)+string(filepath.Separator)) {
			return fmt.Errorf("provider state is outside its owned directory")
		}
		if err = os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
