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
	"runtime"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const nativeAgentDir = "/home/dev/.luci-agent"

func (a *API) resolveSessionAgent(provider string, selection *string, source registry.Session, requestedModel string) (product.AgentConnectionReceipt, map[string]string, string, error) {
	receipt := product.AgentConnectionReceipt{Kind: "legacy"}
	if source.ID != "" && selection == nil {
		// An existing conversation keeps its own route, even when the user's
		// default changes. Old sessions explicitly retain legacy behaviour.
		id := source.AgentConnectionID
		selection = &id
	}
	if a.deps.Product == nil {
		if selection != nil && *selection != "" {
			return receipt, nil, "", errors.New("Saved agent connections are unavailable in this workspace.")
		}
		return receipt, nil, requestedModel, nil
	}
	receipt, env, err := a.deps.Product.ResolveAgentConnection(provider, selection)
	if err != nil {
		return receipt, nil, "", err
	}
	if source.ID != "" && (source.AgentConnectionID != receipt.ID || source.AgentConnectionID != "" && (source.AgentConnectionRevision != receipt.Revision || source.AgentConnectionKind != receipt.Kind || source.AgentConnectionProtocol != receipt.Protocol)) {
		return receipt, nil, "", errors.New("This session's agent connection changed or is unavailable. Start a new session with the connection you intend to use.")
	}
	model := requestedModel
	if receipt.Model != "" {
		prefixed := receipt.Model
		if provider == "opencode" && receipt.Kind != "legacy" {
			prefixed = "luci-gateway/" + receipt.Model
		}
		if (selection != nil || source.ID != "") && model != "" && model != "default" && model != receipt.Model && model != prefixed {
			return receipt, nil, "", errors.New("The requested model differs from this saved agent connection. Edit the connection or select its configured model.")
		}
		model = prefixed
	}
	return receipt, env, model, nil
}

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
	legacy := filepath.Join(a.deps.ConversationsDir, conversationpath.LegacyRelative(sessionConversationOwner, worktreeID, provider, conversation))
	if runtime.GOOS != "windows" {
		return legacy, nil
	}
	compact := filepath.Join(a.deps.ConversationsDir, conversationpath.NativeRelative(sessionConversationOwner, worktreeID, provider, conversation))
	if _, err := os.Lstat(compact); err == nil {
		return compact, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	// Existing standalone Windows stores keep their original homes. Managed
	// workers migrate them before launch; new conversations use compact homes.
	if _, err := os.Lstat(legacy); err == nil {
		return legacy, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return compact, nil
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
		delete(env, "CODEX_API_KEY")
		delete(env, "LUCI_CODEX_BASE_URL")
		delete(env, "LUCI_CODEX_MODEL")
		return []string{"-c", `cli_auth_credentials_store="file"`, "login", "--device-auth"}
	}
	if env["OPENAI_API_KEY"] == "" {
		return args
	}
	base := env["LUCI_CODEX_BASE_URL"]
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	providerConfig := `model_providers.luci-openai={name="Luci selected API",base_url=` + strconv.Quote(base) + `,env_key="OPENAI_API_KEY",wire_api="responses",supports_websockets=false}`
	return append([]string{"-c", `model_provider="luci-openai"`, "-c", providerConfig}, args...)
}

// Container localhost is the agent itself, not the Docker host. Docker's
// host-gateway alias reaches a deliberately configured host model service.
func nativeDockerEndpoint(env map[string]string) {
	for _, key := range []string{"LUCI_CODEX_BASE_URL", "ANTHROPIC_BASE_URL"} {
		if env[key] != "" {
			env[key] = nativeHostEndpoint(env[key])
		}
	}
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil {
		return
	}
	providers, _ := config["provider"].(map[string]any)
	for _, id := range []string{"luci-local", "luci-gateway"} {
		provider, _ := providers[id].(map[string]any)
		options, _ := provider["options"].(map[string]any)
		base, _ := options["baseURL"].(string)
		if options != nil {
			options["baseURL"] = nativeHostEndpoint(base)
		}
	}
	data, err := json.Marshal(config)
	if err == nil {
		env["OPENCODE_CONFIG_CONTENT"] = string(data)
	}
}

// Private resolver hints are consumed before a backend spec is built. The
// backend reserves LUCI_* for its command/attach-token contract; no exception
// is made for user-selected routes. The remaining names are provider inputs.
func nativeSandboxEnvironment(env map[string]string) {
	if base := env["LUCI_CODEX_BASE_URL"]; base != "" {
		env["OPENAI_BASE_URL"] = base
	}
	if key, ok := env["LUCI_GATEWAY_API_KEY"]; ok {
		env["OPENCODE_LOCAL_API_KEY"] = key
		env["OPENCODE_CONFIG_CONTENT"] = strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "{env:LUCI_GATEWAY_API_KEY}", "{env:OPENCODE_LOCAL_API_KEY}")
	}
	for _, key := range []string{"LUCI_CODEX_BASE_URL", "LUCI_CODEX_MODEL", "LUCI_GATEWAY_API_KEY", "LUCI_OPENCODE_MODEL"} {
		delete(env, key)
	}
}

func nativeHostEndpoint(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return base
	}
	port := u.Port()
	u.Host = "host.docker.internal"
	if port != "" {
		u.Host = net.JoinHostPort(u.Host, port)
	}
	return u.String()
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
