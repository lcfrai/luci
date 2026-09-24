package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// env returns a getenv that reads from m and nothing else. Every test here uses
// it, so no test mutates the process environment, and every one of them is safe
// under t.Parallel() and under -race.
func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaultsAreAWorkingLaptop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_STATE_DIR": dir}))
	if err != nil {
		t.Fatalf("loadConfig with an empty environment: %v", err)
	}
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Addr", cfg.Addr, "127.0.0.1:8730"},
		{"Backend", cfg.Backend, "docker"},
		{"SandboxImage", cfg.SandboxImage, "panacea-app/sandbox:dev"},
		{"IdleAfter", cfg.IdleAfter, 90 * time.Second},
		{"IdleTimeout", cfg.IdleTimeout, time.Duration(0)},
		{"ReapInterval", cfg.ReapInterval, 30 * time.Second},
		{"ReadyTimeout", cfg.ReadyTimeout, backend.ReadyTimeout},
		{"CPUMillis", cfg.CPUMillis, 2000},
		{"MemoryMiB", cfg.MemoryMiB, 4096},
		{"Namespace", cfg.Namespace, "panacea-app"},
		{"LogLevel", cfg.LogLevel, slog.LevelInfo},
		{"SessionsPath", cfg.SessionsPath, filepath.Join(dir, "sessions", "sessions.json")},
		{"ConversationsDir", cfg.ConversationsDir, filepath.Join(dir, "conversations")},
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

// The registry tolerates a relative state directory and a bind mount does not:
// every session create would fail inside Docker with an error naming neither
// PANACEA_APP_STATE_DIR nor the conversation store.
func TestConversationsDirIsAbsoluteEvenForARelativeStateDir(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_STATE_DIR": "relative-state"}))
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

func TestLoadConfigRefusesANonLoopbackBind(t *testing.T) {
	t.Parallel()
	// ":8730" is in this table because it is the plausible typo and it binds
	// every interface. An empty host is not "the default"; it is 0.0.0.0.
	for _, addr := range []string{"0.0.0.0:8730", ":8730", "192.168.1.5:8730", "[::]:8730", "example.local:8730"} {
		_, err := loadConfig(env(map[string]string{"PANACEA_APP_ADDR": addr}))
		if err == nil {
			t.Errorf("PANACEA_APP_ADDR=%q was accepted; it puts an unauthenticated shell on the LAN", addr)
			continue
		}
		const want = "a sandbox attach socket on a non-loopback address is a shell on the LAN"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("PANACEA_APP_ADDR=%q: error %q does not carry the contract's message %q", addr, err, want)
		}
		if !strings.Contains(err.Error(), "PANACEA_APP_ALLOW_PUBLIC_BIND=1") {
			t.Errorf("PANACEA_APP_ADDR=%q: error %q does not name the override", addr, err)
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
		if _, err := loadConfig(env(map[string]string{"PANACEA_APP_ADDR": addr})); err != nil {
			t.Errorf("PANACEA_APP_ADDR=%q is loopback and was rejected: %v", addr, err)
		}
	}
}

func TestAllowPublicBindIsTheOnlyWayPast(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1", "true"} {
		cfg, err := loadConfig(env(map[string]string{
			"PANACEA_APP_ADDR":              "0.0.0.0:8730",
			"PANACEA_APP_ALLOW_PUBLIC_BIND": v,
		}))
		if err != nil {
			t.Errorf("PANACEA_APP_ALLOW_PUBLIC_BIND=%q did not override the refusal: %v", v, err)
			continue
		}
		if !cfg.AllowPublicBind {
			t.Errorf("PANACEA_APP_ALLOW_PUBLIC_BIND=%q parsed as false", v)
		}
	}
	// Contract §4: anything other than 1 or true is treated as unset, and
	// "TRUE" is in this list rather than the one above because the set is
	// exactly two spellings. Widening it case-insensitively is a change to a
	// locked document made on the one variable that disarms a security rule,
	// and the failure direction matters: an operator who types TRUE is refused
	// with a message naming PANACEA_APP_ALLOW_PUBLIC_BIND=1, which is the
	// spelling that works. Guessing the other way puts a shell on the LAN.
	for _, v := range []string{"yes", "0", "false", "on", "TRUE", "True", ""} {
		if _, err := loadConfig(env(map[string]string{
			"PANACEA_APP_ADDR":              "0.0.0.0:8730",
			"PANACEA_APP_ALLOW_PUBLIC_BIND": v,
		})); err == nil {
			t.Errorf("PANACEA_APP_ALLOW_PUBLIC_BIND=%q was read as an override", v)
		}
	}
}

func TestLoadConfigRejectsAnUnknownBackendAndNamesBoth(t *testing.T) {
	t.Parallel()
	_, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": "podman"}))
	if err == nil {
		t.Fatal("PANACEA_APP_BACKEND=podman was accepted")
	}
	for _, want := range []string{"podman", "docker", "kata"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	for _, v := range []string{"docker", "kata"} {
		cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": v}))
		if err != nil {
			t.Fatalf("PANACEA_APP_BACKEND=%q: %v", v, err)
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
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_READY_TIMEOUT": "4m"}))
	if err != nil {
		t.Fatalf("PANACEA_APP_READY_TIMEOUT=4m: %v", err)
	}
	if cfg.ReadyTimeout != 4*time.Minute {
		t.Errorf("ReadyTimeout = %v, want 4m", cfg.ReadyTimeout)
	}
	for _, v := range []string{"0", "-30s", "60"} {
		if _, err := loadConfig(env(map[string]string{"PANACEA_APP_READY_TIMEOUT": v})); err == nil {
			t.Errorf("PANACEA_APP_READY_TIMEOUT=%q was accepted; a create that never waits fails every time", v)
		}
	}
}

func TestReapIntervalIsClampedRatherThanTrusted(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1s", "0", "250ms"} {
		cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_REAP_INTERVAL": v}))
		if err != nil {
			t.Fatalf("PANACEA_APP_REAP_INTERVAL=%q: %v", v, err)
		}
		if cfg.ReapInterval != 5*time.Second {
			t.Errorf("PANACEA_APP_REAP_INTERVAL=%q gave %v, want the 5s minimum", v, cfg.ReapInterval)
		}
		if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "PANACEA_APP_REAP_INTERVAL") {
			t.Errorf("PANACEA_APP_REAP_INTERVAL=%q clamped silently; warnings were %v", v, cfg.Warnings)
		}
	}
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_REAP_INTERVAL": "45s"}))
	if err != nil {
		t.Fatalf("PANACEA_APP_REAP_INTERVAL=45s: %v", err)
	}
	if cfg.ReapInterval != 45*time.Second || len(cfg.Warnings) != 0 {
		t.Errorf("45s gave %v with warnings %v", cfg.ReapInterval, cfg.Warnings)
	}
}

func TestIdleWindowsAndSizesRejectNonsenseRatherThanDefaulting(t *testing.T) {
	t.Parallel()
	// 0 is meaningful for both idle variables: IDLE_AFTER=0 disables idle
	// derivation, IDLE_TIMEOUT=0 means never destroy. Both are contract §4.
	cfg, err := loadConfig(env(map[string]string{
		"PANACEA_APP_IDLE_AFTER":   "0",
		"PANACEA_APP_IDLE_TIMEOUT": "0",
	}))
	if err != nil {
		t.Fatalf("zero idle windows: %v", err)
	}
	if cfg.IdleAfter != 0 || cfg.IdleTimeout != 0 {
		t.Errorf("IdleAfter=%v IdleTimeout=%v, want both zero", cfg.IdleAfter, cfg.IdleTimeout)
	}
	for _, c := range []struct{ name, value string }{
		{"PANACEA_APP_IDLE_AFTER", "90"},
		{"PANACEA_APP_IDLE_AFTER", "-1s"},
		{"PANACEA_APP_IDLE_TIMEOUT", "soon"},
		{"PANACEA_APP_SESSION_CPU_MILLIS", "0"},
		{"PANACEA_APP_SESSION_CPU_MILLIS", "two"},
		{"PANACEA_APP_SESSION_MEMORY_MIB", "-4096"},
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
		cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_LOG_LEVEL": in}))
		if err != nil || cfg.LogLevel != want {
			t.Errorf("PANACEA_APP_LOG_LEVEL=%q gave (%v, %v), want %v", in, cfg.LogLevel, err, want)
		}
	}
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_LOG_LEVEL": "chatty"}))
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
		"PANACEA_APP_ALLOWED_ORIGINS": "tauri://localhost, http://localhost:1420 ",
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
	if _, err := loadConfig(env(map[string]string{"PANACEA_APP_ALLOWED_ORIGINS": " , "})); err == nil {
		t.Error("a set-but-empty allowlist was accepted; it silently restores the defaults it was meant to replace")
	}
}

func TestHarnessAllowUnverifiedWarnsThatNothingReadsItYet(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_HARNESS_ALLOW_UNVERIFIED": "1"}))
	if err != nil {
		t.Fatalf("PANACEA_APP_HARNESS_ALLOW_UNVERIFIED=1: %v", err)
	}
	if !cfg.HarnessAllowUnverified {
		t.Error("HarnessAllowUnverified = false")
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "harness.Build") {
		t.Errorf("setting it must say it has no effect yet; warnings were %v", cfg.Warnings)
	}
}

// Ruling 1: no PANACEA_APP_* variable is ever silently ignored. Contract §4
// carries three backend-specific variables while PANACEA_APP_BACKEND selects
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
		{"PANACEA_APP_DOCKER_HOST", "tcp://127.0.0.1:2375", "docker", false},
		{"PANACEA_APP_DOCKER_HOST", "tcp://127.0.0.1:2375", "kata", true},
		{"PANACEA_APP_KUBECONFIG", "/home/dev/.kube/config", "docker", true},
		// Set under kata this variable takes kubeClient's REFUSAL, which is a
		// different and louder thing than "this has no effect here"; warning
		// about it as well would tell the operator it is ignored when it is
		// about to stop the process.
		{"PANACEA_APP_KUBECONFIG", "/home/dev/.kube/config", "kata", false},
		{"PANACEA_APP_NAMESPACE", "panacea-sandboxes", "docker", true},
		{"PANACEA_APP_NAMESPACE", "panacea-sandboxes", "kata", false},
	} {
		cfg, err := loadConfig(env(map[string]string{
			c.name:                c.value,
			"PANACEA_APP_BACKEND": c.backend,
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
		cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": kind}))
		if err != nil {
			t.Fatalf("backend %q: %v", kind, err)
		}
		if len(cfg.Warnings) != 0 {
			t.Errorf("backend %q with nothing else set warned about %v", kind, cfg.Warnings)
		}
	}
}

// Seam S8.4 / P8-D15: which RuntimeClass AKS installs is unverified, so the
// name is configuration and a wrong guess is an environment change.
func TestRuntimeClassIsReadUnderKata(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": "kata"}))
	if err != nil {
		t.Fatalf("kata with no runtime class: %v", err)
	}
	if cfg.RuntimeClass != sandbox.DefaultRuntimeClass {
		t.Errorf("RuntimeClass unset = %q, want %q", cfg.RuntimeClass, sandbox.DefaultRuntimeClass)
	}
	cfg, err = loadConfig(env(map[string]string{
		"PANACEA_APP_BACKEND":       "kata",
		"PANACEA_APP_RUNTIME_CLASS": "kata-mshv-vm-isolation",
	}))
	if err != nil {
		t.Fatalf("kata with a runtime class: %v", err)
	}
	if cfg.RuntimeClass != "kata-mshv-vm-isolation" {
		t.Errorf("RuntimeClass = %q, want kata-mshv-vm-isolation", cfg.RuntimeClass)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("an applicable PANACEA_APP_RUNTIME_CLASS warned: %v", cfg.Warnings)
	}
}

// The backendScoped rule: no PANACEA_APP_* variable is silently ignored.
func TestRuntimeClassUnderDockerIsAWarning(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{
		"PANACEA_APP_BACKEND":       "docker",
		"PANACEA_APP_RUNTIME_CLASS": "kata-vm-isolation",
	}))
	if err != nil {
		t.Fatalf("docker with a runtime class set: %v; an inapplicable variable is no reason to refuse to run", err)
	}
	var logs bytes.Buffer
	logWarnings(slog.New(slog.NewTextHandler(&logs, nil)), cfg.Warnings)
	if !strings.Contains(logs.String(), "PANACEA_APP_RUNTIME_CLASS") || !strings.Contains(logs.String(), "kata") {
		t.Errorf("PANACEA_APP_RUNTIME_CLASS under docker must warn naming it and kata; the log was:\n%s", logs.String())
	}
	// Under docker the value is never used, so a malformed one warns too
	// rather than stopping a laptop.
	if _, err := loadConfig(env(map[string]string{"PANACEA_APP_RUNTIME_CLASS": "Not_A_Name"})); err != nil {
		t.Errorf("docker with a malformed, unused runtime class exited: %v", err)
	}
}

func TestRuntimeClassMustBeADNSSubdomain(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"kata-vm-isolation", "a", "kata.v2", "0kata9", strings.Repeat("a", 63) + "." + strings.Repeat("b", 189)} {
		if _, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": "kata", "PANACEA_APP_RUNTIME_CLASS": ok})); err != nil {
			t.Errorf("PANACEA_APP_RUNTIME_CLASS=%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"Kata", "kata_vm", "kata vm", "-kata", "kata-", "kata.", ".kata", "kata..v2", strings.Repeat("a", 254)} {
		_, err := loadConfig(env(map[string]string{"PANACEA_APP_BACKEND": "kata", "PANACEA_APP_RUNTIME_CLASS": bad}))
		if err == nil || !strings.Contains(err.Error(), "PANACEA_APP_RUNTIME_CLASS") {
			t.Errorf("PANACEA_APP_RUNTIME_CLASS=%q: err = %v, want a refusal naming the variable", bad, err)
		}
	}
	var out bytes.Buffer
	if code := run(env(map[string]string{"PANACEA_APP_BACKEND": "kata", "PANACEA_APP_RUNTIME_CLASS": "Kata_VM"}), &out); code != 2 {
		t.Errorf("run with a malformed runtime class = %d, want 2", code)
	}
	if !strings.Contains(out.String(), "PANACEA_APP_RUNTIME_CLASS") {
		t.Errorf("the exit-2 message does not name the variable: %q", out.String())
	}
}

// newBackend's kata arm needs an in-cluster client, so no test reaches it; the
// wiring it uses is pinned here instead (task 14's predicted survivor P1: a
// configured runtime class dropped on the way to the pod manager).
func TestTheKataManagerGetsTheConfiguredRuntimeClass(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{
		"PANACEA_APP_BACKEND":       "kata",
		"PANACEA_APP_NAMESPACE":     "panacea-sessions",
		"PANACEA_APP_RUNTIME_CLASS": "rc-x",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := sandbox.Config{Namespace: "panacea-sessions", RuntimeClass: "rc-x"}
	if got := managerConfigFor(cfg); got != want {
		t.Fatalf("managerConfigFor = %+v, want %+v", got, want)
	}
}

// The whole kata construction after kubeClient, driven with a fake clientset:
// the pod it creates carries the configured runtime class and namespace (task
// 14's residual mutant P1c: newBackend bypassing managerConfigFor).
func TestTheKataBackendCreatesPodsWithTheConfiguredRuntimeClass(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset()
	be, err := kataBackend(client, Config{Namespace: "panacea-sessions", RuntimeClass: "rc-x", ReadyTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("kataBackend: %v", err)
	}
	_, err = be.Create(context.Background(), backend.Spec{
		ID: "s-1", Image: "x:y", Command: "claude", AttachToken: "tok", Resources: backend.DefaultResources,
	})
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create on a fake cluster = %v, want ErrNotRunning (the pod never schedules)", err)
	}
	pod, err := client.CoreV1().Pods("panacea-sessions").Get(context.Background(), sandbox.PodName("", "s-1"), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("no pod in the configured namespace: %v", err)
	}
	if rc := pod.Spec.RuntimeClassName; rc == nil || *rc != "rc-x" {
		t.Fatalf("runtimeClassName = %v, want rc-x: the configured class never reached the pod", rc)
	}
}
