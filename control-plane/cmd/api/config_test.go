package main

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// env returns a getenv that reads from m and nothing else. Every test here uses
// it, so no test mutates the process environment, and every one of them is safe
// under t.Parallel() and under -race.
func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDocumentRuntimeStartupRequiresIndependentPinsAndLocalController(t *testing.T){
 for _,kind:=range []string{"absent","both","profile-only","tools-only","uppercase","worker","kata"}{t.Run(kind,func(t *testing.T){
  values:=map[string]string{envStateDir:t.TempDir()}
  if kind!="absent"{values[envDocumentProfileSHA256]=strings.Repeat("a",64);values[envDocumentToolDomainSHA256]=strings.Repeat("b",64)}
  switch kind{case "profile-only":delete(values,envDocumentToolDomainSHA256);case "tools-only":delete(values,envDocumentProfileSHA256);case "uppercase":values[envDocumentProfileSHA256]=strings.Repeat("A",64);case "worker":values[envWorker]="1";case "kata":values[envBackend]="kata"}
  cfg,err:=loadConfig(env(values));if kind=="absent"||kind=="both"{if err!=nil||cfg.DocumentRuntime.Configured()!=(kind=="both"){t.Fatal("valid startup pins refused",err)}}else if err==nil{t.Fatal("partial/untrusted route admitted")}
 })}
}

func TestPrebuiltDocumentStartupDoesNotRequireWindowsProducerProfile(t *testing.T){
 for _,kind:=range []string{"image-only","image-and-proof","malformed-image","image-worker","image-kata"}{t.Run(kind,func(t *testing.T){
  values:=map[string]string{envStateDir:t.TempDir(),envDocumentImageManifestSHA256:strings.Repeat("c",64)}
  switch kind{case "image-and-proof":values[envDocumentProfileSHA256]=strings.Repeat("a",64);values[envDocumentToolDomainSHA256]=strings.Repeat("b",64);case "malformed-image":values[envDocumentImageManifestSHA256]="self-pinned";case "image-worker":values[envWorker]="1";case "image-kata":values[envBackend]="kata"}
  cfg,err:=loadConfig(env(values))
  if kind=="image-only"||kind=="image-and-proof"{if err!=nil||cfg.DocumentRuntime.ImageManifestSHA256!=values[envDocumentImageManifestSHA256]||cfg.DocumentRuntime.Configured()!=(kind=="image-and-proof"){t.Fatal("prebuilt configuration depended on Windows proof",err)}}else if err==nil{t.Fatal("unsupported image configuration admitted")}
 })}
}

func TestLoadConfigDefaultsAreAWorkingLaptop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := loadConfig(env(map[string]string{"LUCI_STATE_DIR": dir}))
	if err != nil {
		t.Fatalf("loadConfig with an empty environment: %v", err)
	}
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Addr", cfg.Addr, "127.0.0.1:8730"},
		{"Backend", cfg.Backend, "docker"},
		{"SandboxImage", cfg.SandboxImage, "luci/sandbox:dev"},
		{"IdleAfter", cfg.IdleAfter, 90 * time.Second},
		{"IdleTimeout", cfg.IdleTimeout, time.Duration(0)},
		{"ReapInterval", cfg.ReapInterval, 30 * time.Second},
		{"ReadyTimeout", cfg.ReadyTimeout, backend.ReadyTimeout},
		{"CPUMillis", cfg.CPUMillis, 1000},
		{"MemoryMiB", cfg.MemoryMiB, 1024},
		{"RuntimeBuildCPUMillis", cfg.RuntimeBuildCPUMillis, 1000},
		{"RuntimeBuildMemoryMiB", cfg.RuntimeBuildMemoryMiB, 1024},
		{"Namespace", cfg.Namespace, "luci"},
		{"LogLevel", cfg.LogLevel, slog.LevelInfo},
		{"SessionsPath", cfg.SessionsPath, filepath.Join(dir, "sessions", "sessions.json")},
		{"ConversationsDir", cfg.ConversationsDir, filepath.Join(dir, "conversations")},
		{"InstancePath", cfg.InstancePath, filepath.Join(dir, "instance", "instance-id")},
	} {
		if c.got != c.want {
			t.Errorf("Config.%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins = %v, want empty so httpx.DefaultOrigins applies", cfg.AllowedOrigins)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("a default configuration warned about %v; it must be silent", cfg.Warnings)
	}
	if cfg.AllowPublicBind || cfg.HarnessAllowUnverified {
		t.Error("an unset boolean must be false")
	}
}

func TestLocalResourceDefaultsPreserveExplicitAndRemoteSizing(t *testing.T) {
	for _, c := range []struct {
		name                               string
		values                             map[string]string
		memory, cpu, buildMemory, buildCPU int
	}{
		{"local", map[string]string{}, 1024, 1000, 1024, 1000},
		{"explicit", map[string]string{"LUCI_SESSION_MEMORY_MIB": "3072", "LUCI_SESSION_CPU_MILLIS": "1500", "LUCI_RUNTIME_BUILD_MEMORY_MIB": "2048", "LUCI_RUNTIME_BUILD_CPU_MILLIS": "1200"}, 3072, 1500, 2048, 1200},
		{"remote", map[string]string{"LUCI_BACKEND": "kata"}, 4096, 2000, 1024, 1000},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.values["LUCI_STATE_DIR"] = t.TempDir()
			got, err := loadConfig(env(c.values))
			if err != nil {
				t.Fatal(err)
			}
			if got.MemoryMiB != c.memory || got.CPUMillis != c.cpu || got.RuntimeBuildMemoryMiB != c.buildMemory || got.RuntimeBuildCPUMillis != c.buildCPU {
				t.Fatal(got.Resources(), got.RuntimeBuildResources())
			}
		})
	}
}

// The instance id is ownership of what the registry beside it records, so the
// two must sit in the same state directory on BOTH branches of the ladder: an
// id resolved from one directory and a registry from another would give a
// restart the right sessions and the wrong sandboxes. The empty environment
// takes the statepath ladder; nothing here writes to either path.
func TestTheInstanceIDLivesInTheSameStateDirectoryAsTheRegistry(t *testing.T) {
	t.Parallel()
	for name, m := range map[string]map[string]string{
		"LUCI_STATE_DIR set":   {"LUCI_STATE_DIR": t.TempDir()},
		"the statepath ladder": {},
	} {
		cfg, err := loadConfig(env(m))
		if err != nil {
			t.Fatalf("%s: loadConfig: %v", name, err)
		}
		if filepath.Base(cfg.InstancePath) != "instance-id" || filepath.Base(filepath.Dir(cfg.InstancePath)) != "instance" {
			t.Errorf("%s: InstancePath = %q, want instance/instance-id, the same on both branches", name, cfg.InstancePath)
		}
		root := filepath.Dir(filepath.Dir(cfg.SessionsPath))
		if got := filepath.Dir(filepath.Dir(cfg.InstancePath)); got != root {
			t.Errorf("%s: InstancePath %q is under %q, want the registry's state directory %q", name, cfg.InstancePath, got, root)
		}
	}
}

// The registry tolerates a relative state directory and a bind mount does not:
// every session create would fail inside Docker with an error naming neither
// LUCI_STATE_DIR nor the conversation store.
func TestConversationsDirIsAbsoluteEvenForARelativeStateDir(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"LUCI_STATE_DIR": "relative-state"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !filepath.IsAbs(cfg.ConversationsDir) {
		t.Fatalf("ConversationsDir = %q, want an absolute path", cfg.ConversationsDir)
	}
	if !strings.HasSuffix(cfg.ConversationsDir, filepath.Join("relative-state", "conversations")) {
		t.Errorf("ConversationsDir = %q, want it under the state directory", cfg.ConversationsDir)
	}
}

func TestWorkerConversationStoreOverrideIsAbsoluteAndIndependentOfRegistry(t *testing.T) {
	t.Parallel()
	state, compact := t.TempDir(), t.TempDir()
	cfg, err := loadConfig(env(map[string]string{"LUCI_STATE_DIR": state, "LUCI_CONVERSATIONS_DIR": compact}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConversationsDir != compact || cfg.SessionsPath != filepath.Join(state, "sessions", "sessions.json") {
		t.Fatal("conversation override changed session ownership")
	}
	if _, err = loadConfig(env(map[string]string{"LUCI_CONVERSATIONS_DIR": "relative-private-store"})); err == nil {
		t.Fatal("relative provider bind root was accepted")
	}
}

func TestLoadConfigRefusesANonLoopbackBind(t *testing.T) {
	t.Parallel()
	// ":8730" is in this table because it is the plausible typo and it binds
	// every interface. An empty host is not "the default"; it is 0.0.0.0.
	for _, addr := range []string{"0.0.0.0:8730", ":8730", "192.168.1.5:8730", "[::]:8730", "example.local:8730"} {
		_, err := loadConfig(env(map[string]string{"LUCI_ADDR": addr}))
		if err == nil {
			t.Errorf("LUCI_ADDR=%q was accepted; it puts an unauthenticated shell on the LAN", addr)
			continue
		}
		const want = "a sandbox attach socket on a non-loopback address is a shell on the LAN"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("LUCI_ADDR=%q: error %q does not carry the contract's message %q", addr, err, want)
		}
		if !strings.Contains(err.Error(), "LUCI_ALLOW_PUBLIC_BIND=1") {
			t.Errorf("LUCI_ADDR=%q: error %q does not name the override", addr, err)
		}
	}
	// 127.0.0.2:8730 is the accepted departure from contract §4's text, which
	// named three spellings, and it is in this table so the departure is a
	// property rather than a comment. validateAddr uses httpx.LoopbackHost —
	// the same predicate task 9's middleware applies to every request's Host
	// header — so the whole of 127.0.0.0/8 binds. Every address that admits is
	// unroutable from another host, and one predicate is what stops the bind
	// refusal and the runtime guard from disagreeing about the same address.
	// Revert to a literal three-spelling check and this line fails.
	for _, addr := range []string{"127.0.0.1:0", "127.0.0.1:8730", "127.0.0.2:8730", "localhost:8730", "[::1]:8730"} {
		if _, err := loadConfig(env(map[string]string{"LUCI_ADDR": addr})); err != nil {
			t.Errorf("LUCI_ADDR=%q is loopback and was rejected: %v", addr, err)
		}
	}
}

func TestAllowPublicBindIsTheOnlyWayPast(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1", "true"} {
		cfg, err := loadConfig(env(map[string]string{
			"LUCI_ADDR":              "0.0.0.0:8730",
			"LUCI_ALLOW_PUBLIC_BIND": v,
		}))
		if err != nil {
			t.Errorf("LUCI_ALLOW_PUBLIC_BIND=%q did not override the refusal: %v", v, err)
			continue
		}
		if !cfg.AllowPublicBind {
			t.Errorf("LUCI_ALLOW_PUBLIC_BIND=%q parsed as false", v)
		}
	}
	// Contract §4: anything other than 1 or true is treated as unset, and
	// "TRUE" is in this list rather than the one above because the set is
	// exactly two spellings. Widening it case-insensitively is a change to a
	// locked document made on the one variable that disarms a security rule,
	// and the failure direction matters: an operator who types TRUE is refused
	// with a message naming LUCI_ALLOW_PUBLIC_BIND=1, which is the
	// spelling that works. Guessing the other way puts a shell on the LAN.
	for _, v := range []string{"yes", "0", "false", "on", "TRUE", "True", ""} {
		if _, err := loadConfig(env(map[string]string{
			"LUCI_ADDR":              "0.0.0.0:8730",
			"LUCI_ALLOW_PUBLIC_BIND": v,
		})); err == nil {
			t.Errorf("LUCI_ALLOW_PUBLIC_BIND=%q was read as an override", v)
		}
	}
}

func TestLoadConfigRejectsAnUnknownBackendAndNamesBoth(t *testing.T) {
	t.Parallel()
	_, err := loadConfig(env(map[string]string{"LUCI_BACKEND": "podman"}))
	if err == nil {
		t.Fatal("LUCI_BACKEND=podman was accepted")
	}
	for _, want := range []string{"podman", "docker", "kata"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	for _, v := range []string{"docker", "kata"} {
		cfg, err := loadConfig(env(map[string]string{"LUCI_BACKEND": v}))
		if err != nil {
			t.Fatalf("LUCI_BACKEND=%q: %v", v, err)
		}
		if cfg.Backend != v {
			t.Errorf("Backend = %q, want %q", cfg.Backend, v)
		}
	}
}

func TestReadyTimeoutIsSettableAndDefaultsToTheBackendConstant(t *testing.T) {
	t.Parallel()
	// This variable is not in contract §4; task 5 records it as a dependency of
	// this task, and until it exists dockerbe.Config.ReadyTimeout is test-only.
	cfg, err := loadConfig(env(map[string]string{"LUCI_READY_TIMEOUT": "4m"}))
	if err != nil {
		t.Fatalf("LUCI_READY_TIMEOUT=4m: %v", err)
	}
	if cfg.ReadyTimeout != 4*time.Minute {
		t.Errorf("ReadyTimeout = %v, want 4m", cfg.ReadyTimeout)
	}
	for _, v := range []string{"0", "-30s", "60"} {
		if _, err := loadConfig(env(map[string]string{"LUCI_READY_TIMEOUT": v})); err == nil {
			t.Errorf("LUCI_READY_TIMEOUT=%q was accepted; a create that never waits fails every time", v)
		}
	}
}

func TestReapIntervalIsClampedRatherThanTrusted(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1s", "0", "250ms"} {
		cfg, err := loadConfig(env(map[string]string{"LUCI_REAP_INTERVAL": v}))
		if err != nil {
			t.Fatalf("LUCI_REAP_INTERVAL=%q: %v", v, err)
		}
		if cfg.ReapInterval != 5*time.Second {
			t.Errorf("LUCI_REAP_INTERVAL=%q gave %v, want the 5s minimum", v, cfg.ReapInterval)
		}
		if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "LUCI_REAP_INTERVAL") {
			t.Errorf("LUCI_REAP_INTERVAL=%q clamped silently; warnings were %v", v, cfg.Warnings)
		}
	}
	cfg, err := loadConfig(env(map[string]string{"LUCI_REAP_INTERVAL": "45s"}))
	if err != nil {
		t.Fatalf("LUCI_REAP_INTERVAL=45s: %v", err)
	}
	if cfg.ReapInterval != 45*time.Second || len(cfg.Warnings) != 0 {
		t.Errorf("45s gave %v with warnings %v", cfg.ReapInterval, cfg.Warnings)
	}
}

// The other end of the same clamp, and it is task 25's F3 fix that made it
// necessary. The sandbox now stops answering /attach the instant its process
// ends, so the cockpit's attach ladder is no longer reset by a socket that
// opens and closes: it climbs once and gives up after RETRY_BUDGET_MS. If the
// record does not say `exited` by then, the pane lands terminal `failed` — no
// exit reason, and no Restart offered, because Restart is the answer to
// `exited` — on a session that merely finished normally.
//
// The sweep is what writes `exited`. LUCI_REAP_INTERVAL=2m was accepted
// before this, and it makes that failure the outcome of EVERY session that
// ends.
func TestReapIntervalCannotOutrunTheCockpitsAttachBudget(t *testing.T) {
	t.Parallel()
	// The relationship first, because the clamp below only pins the literal.
	// maxReapInterval = 59s would keep every assertion in this test green while
	// putting the sweep 1s inside a 60s budget, which is the configuration the
	// clamp exists to remove. What the maximum has to hold is that one whole
	// sweep, plus the cockpit's session poll and the render after it, fits
	// inside the budget with room — and config.go's own comment names the room:
	// "a quarter of the budget is far more than either needs".
	//
	// This is the only place either number is checkable: RETRY_BUDGET_MS lives
	// in the other tree (desktop/src/api/attachRegistry.ts, the F5 chain's
	// file), so cockpitAttachRetryBudget is this tree's transcription of it.
	if margin := cockpitAttachRetryBudget - maxReapInterval; margin < cockpitAttachRetryBudget/4 {
		t.Errorf("the %v maximum leaves %v of the cockpit's %v attach budget, less than the quarter "+
			"config.go claims: a sweep that lands this close to the ladder's give-up turns a normal "+
			"exit into a terminal failed pane",
			maxReapInterval, margin, cockpitAttachRetryBudget)
	}
	for _, v := range []string{"60s", "2m", "1h"} {
		cfg, err := loadConfig(env(map[string]string{"LUCI_REAP_INTERVAL": v}))
		if err != nil {
			t.Fatalf("LUCI_REAP_INTERVAL=%q: %v", v, err)
		}
		if cfg.ReapInterval != maxReapInterval {
			t.Errorf("LUCI_REAP_INTERVAL=%q gave %v, want the %v maximum: a sweep slower than "+
				"the cockpit's %v attach budget turns every normal exit into a failed attach",
				v, cfg.ReapInterval, maxReapInterval, cockpitAttachRetryBudget)
		}
		if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "LUCI_REAP_INTERVAL") {
			t.Errorf("LUCI_REAP_INTERVAL=%q clamped silently; warnings were %v", v, cfg.Warnings)
		}
	}
	// A bound, not a rewrite: what the operator asked for is kept whenever it
	// is inside the budget. The 45s case above already pins the boundary
	// itself, so this one pins the default an operator who set nothing gets.
	cfg, err := loadConfig(env(nil))
	if err != nil {
		t.Fatalf("loadConfig with nothing set: %v", err)
	}
	if cfg.ReapInterval != defaultReapInterval {
		t.Errorf("ReapInterval = %v with nothing set, want the %v default", cfg.ReapInterval, defaultReapInterval)
	}
	// The maximum has to leave the default alone, or the clamp is the
	// configuration rather than a guard on it.
	if defaultReapInterval > maxReapInterval {
		t.Errorf("the default (%v) is past the maximum (%v)", defaultReapInterval, maxReapInterval)
	}
}

func TestIdleWindowsAndSizesRejectNonsenseRatherThanDefaulting(t *testing.T) {
	t.Parallel()
	// 0 is meaningful for both idle variables: IDLE_AFTER=0 disables idle
	// derivation, IDLE_TIMEOUT=0 means never destroy. Both are contract §4.
	cfg, err := loadConfig(env(map[string]string{
		"LUCI_IDLE_AFTER":   "0",
		"LUCI_IDLE_TIMEOUT": "0",
	}))
	if err != nil {
		t.Fatalf("zero idle windows: %v", err)
	}
	if cfg.IdleAfter != 0 || cfg.IdleTimeout != 0 {
		t.Errorf("IdleAfter=%v IdleTimeout=%v, want both zero", cfg.IdleAfter, cfg.IdleTimeout)
	}
	for _, c := range []struct{ name, value string }{
		{"LUCI_IDLE_AFTER", "90"},
		{"LUCI_IDLE_AFTER", "-1s"},
		{"LUCI_IDLE_TIMEOUT", "soon"},
		{"LUCI_SESSION_CPU_MILLIS", "0"},
		{"LUCI_SESSION_CPU_MILLIS", "two"},
		{"LUCI_SESSION_MEMORY_MIB", "-4096"},
	} {
		_, err := loadConfig(env(map[string]string{c.name: c.value}))
		if err == nil {
			t.Errorf("%s=%q was accepted", c.name, c.value)
			continue
		}
		if !strings.Contains(err.Error(), c.name) || !strings.Contains(err.Error(), c.value) {
			t.Errorf("%s=%q: error %q names neither the variable nor the value", c.name, c.value, err)
		}
	}
}

func TestLogLevelFallsBackLoudlyAndNeverExits(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]slog.Level{
		"debug": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"Warn":  slog.LevelWarn,
		"error": slog.LevelError,
	} {
		cfg, err := loadConfig(env(map[string]string{"LUCI_LOG_LEVEL": in}))
		if err != nil || cfg.LogLevel != want {
			t.Errorf("LUCI_LOG_LEVEL=%q gave (%v, %v), want %v", in, cfg.LogLevel, err, want)
		}
	}
	cfg, err := loadConfig(env(map[string]string{"LUCI_LOG_LEVEL": "chatty"}))
	if err != nil {
		t.Fatalf("an unknown log level must warn, not fail: %v", err)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want the info fallback", cfg.LogLevel)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "chatty") {
		t.Errorf("the fallback was silent; warnings were %v", cfg.Warnings)
	}
}

func TestAllowedOriginsReplaceTheDefaultsAndAnEmptyListIsAnError(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{
		"LUCI_ALLOWED_ORIGINS": "tauri://localhost, http://localhost:1420 ",
	}))
	if err != nil {
		t.Fatalf("two origins: %v", err)
	}
	want := []string{"tauri://localhost", "http://localhost:1420"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i := range want {
		if cfg.AllowedOrigins[i] != want[i] {
			t.Errorf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], want[i])
		}
	}
	if _, err := loadConfig(env(map[string]string{"LUCI_ALLOWED_ORIGINS": " , "})); err == nil {
		t.Error("a set-but-empty allowlist was accepted; it silently restores the defaults it was meant to replace")
	}
}

func TestHarnessAllowUnverifiedWarnsThatNothingReadsItYet(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"LUCI_HARNESS_ALLOW_UNVERIFIED": "1"}))
	if err != nil {
		t.Fatalf("LUCI_HARNESS_ALLOW_UNVERIFIED=1: %v", err)
	}
	if !cfg.HarnessAllowUnverified {
		t.Error("HarnessAllowUnverified = false")
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "harness.Build") {
		t.Errorf("setting it must say it has no effect yet; warnings were %v", cfg.Warnings)
	}
}

// Ruling 1: no LUCI_* variable is ever silently ignored. Contract §4
// carries three backend-specific variables while LUCI_BACKEND selects
// one backend, so an operator can always set one the running backend has no use
// for. The table below is the whole rule, both directions — a warning for every
// inapplicable combination and silence for every applicable one — and it goes
// through a real slog handler rather than reading Config.Warnings directly,
// because what the operator gets is the log line, not the slice.
func TestInapplicableBackendVariablesWarnAndNameTheBackend(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, value, backend string
		wantWarning          bool
	}{
		{"LUCI_DOCKER_HOST", "tcp://127.0.0.1:2375", "docker", false},
		{"LUCI_DOCKER_HOST", "tcp://127.0.0.1:2375", "kata", true},
		{"LUCI_KUBECONFIG", "/home/dev/.kube/config", "docker", true},
		// Set under kata this variable takes kubeClient's REFUSAL, which is a
		// different and louder thing than "this has no effect here"; warning
		// about it as well would tell the operator it is ignored when it is
		// about to stop the process.
		{"LUCI_KUBECONFIG", "/home/dev/.kube/config", "kata", false},
		{"LUCI_NAMESPACE", "luci-sandboxes", "docker", true},
		{"LUCI_NAMESPACE", "luci-sandboxes", "kata", false},
	} {
		cfg, err := loadConfig(env(map[string]string{
			c.name:         c.value,
			"LUCI_BACKEND": c.backend,
		}))
		if err != nil {
			t.Fatalf("%s=%q with backend %q: %v", c.name, c.value, c.backend, err)
		}
		var logs bytes.Buffer
		logWarnings(slog.New(slog.NewTextHandler(&logs, nil)), cfg.Warnings)
		named := strings.Contains(logs.String(), c.name)
		switch {
		case c.wantWarning && !named:
			t.Errorf("%s set under backend %q was ignored in silence; the log was:\n%s", c.name, c.backend, logs.String())
		case c.wantWarning && !strings.Contains(logs.String(), c.backend):
			t.Errorf("%s under backend %q warned without naming the backend:\n%s", c.name, c.backend, logs.String())
		case !c.wantWarning && named:
			t.Errorf("%s applies to backend %q and must not warn; the log was:\n%s", c.name, c.backend, logs.String())
		}
	}

	// Silence when nothing inapplicable is set, on both backends: a warning
	// every operator sees on every start is one nobody reads.
	for _, kind := range []string{"docker", "kata"} {
		cfg, err := loadConfig(env(map[string]string{"LUCI_BACKEND": kind}))
		if err != nil {
			t.Fatalf("backend %q: %v", kind, err)
		}
		if len(cfg.Warnings) != 0 {
			t.Errorf("backend %q with nothing else set warned about %v", kind, cfg.Warnings)
		}
	}
}
