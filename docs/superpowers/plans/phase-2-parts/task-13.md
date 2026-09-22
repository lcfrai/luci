### Task 13: Config, reaper, wiring and restart recovery

Nothing in Phase 2 runs until this task lands. `control-plane/cmd/api/main.go` today is six lines that print `panacea-app control plane` and exit, and `main_test.go` is a placeholder that asserts nothing. Tasks 4 through 12 produce ten packages that no process ever constructs. This task turns them into a program, and it owns the three places where a mistake is not a bug but an incident: the one file that reads the environment, the one loop that destroys sandboxes nobody is watching, and the one function that runs after the control plane has been restarted underneath a live agent.

**The bind refusal is the security rule of this task.** Contract §4 makes a non-loopback `PANACEA_APP_ADDR` an exit 2 at startup with an exact message, and that message is reproduced verbatim in `config.go` — do not reword it. The reasoning is the same one behind `dockerbe`'s non-loopback publish refusal (task 5): `/v1/sessions/{id}/attach` relays to a PTY running an agent with file-system access to a worktree, Phase 2 has no authentication in front of it (contract §1.3), and `docs/PROGRAM.md` puts identity in Phase 8. A control plane on `0.0.0.0` is an unauthenticated shell on the corporate LAN. The escape hatch exists (`PANACEA_APP_ALLOW_PUBLIC_BIND`) because someone will one day have a real reason; it is a decision a human has to type, not a default.

**The reaper's first rule is "change nothing".** Contract §5.3 step 1 is the whole reason this is a package with a test rather than twenty lines in `main`: when `backend.List` fails, the sweep returns without touching a single session. Docker Desktop restarts. If a failed `List` were read as "every sandbox is gone", one daemon restart would mark every session `exited`, and `exited` is terminal (§5.6) — the user's whole rail would die and no later sweep could bring it back. Every destructive rule in `Sweep` is downstream of a successful `List`.

**Restart re-reads the endpoint.** Contract §5.5 step 3 is a Docker fact that will bite in the first week of use: a container that is restarted, or a daemon that is restarted, republishes the sandbox port on a *different* ephemeral host port. The endpoint on disk is stale the moment Docker Desktop restarts, and attaching to a stale endpoint is connection-refused, which looks exactly like a dead agent. Recovery overwrites `Session.Endpoint` from the backend unconditionally — including overwriting it with `""`, which is what a `NonLoopback` sandbox now reports, because nothing may attach to one.

Three things here are **not** in the contract and are added deliberately, and one thing the contract does list is deliberately **not honoured**; all four are declared in the Interfaces block below rather than left to be discovered. **(1) `PANACEA_APP_READY_TIMEOUT`, default `60s`.** Task 5 records it as a cross-task dependency of this task: `dockerbe.Config.ReadyTimeout` exists, but until something parses an environment variable into it, production always gets the `backend.ReadyTimeout` constant and an operator whose laptop pulls a cold image has no way to raise it. Contract §4's table is closed and does not list it; this task opens it by exactly one row, and the default is `backend.ReadyTimeout` itself so the two cannot drift. **(2) The reaper destroys any sandbox whose `backend.Sandbox.NonLoopback` is set, and logs it distinctly.** `NonLoopback` is task 5's accepted amendment to contract §2.4: `List` keeps a LAN-published container in the slice with an empty `Endpoint` rather than failing the whole sweep, precisely so the reaper can see the one container it most needs to destroy. Nothing else in the phase destroys it — task 11 must *not* report it as `exited`, because a running LAN-exposed sandbox reported as exited is a fabricated status. The reaper is the only actor that resolves it, so "why did my sandbox vanish" has to have an answer in the log, at `warn`, naming the sandbox and the reason. **(3) Recovery maps `backend.StateStarting` to `registry.StatusStarting`.** Contract §5.5 step 3 names only `StateRunning` and `StateStopped`, and a `starting` sandbox is neither: the control plane went down between `Create` and the first `/healthz` answer. Leaving the stored status alone would report `running` for a sandbox that is not answering yet, which is the fabricated status this phase rules out; `exited` would be worse, because it is terminal. `starting` is the one of the four Phase 2 statuses that is true, so §5.5 step 3 gains exactly this one row and the reaper re-derives the real status on its first sweep.

**And the subtraction: `PANACEA_APP_KUBECONFIG` is parsed, and refused where it would be used.** Contract §4 lists the row and `loadConfig` still reads it into `Config.Kubeconfig` — symmetrically with `PANACEA_APP_DOCKER_HOST`, which the Kata path likewise ignores, and so that the refusal can name the variable the operator actually set instead of dropping the value on the floor. But `kubeClient` answers a non-empty value with `backend.ErrUnsupported` and a sentence saying what would have to change, because honouring it means reading a kubeconfig *file*, which needs `k8s.io/client-go/tools/clientcmd` and the two unpinned modules it drags in — and Group C ruling 8 keeps new modules out of `go.mod` in this task. Refusing loudly beats honouring silently-wrongly: an operator who points this at a file, gets a running process and never learns it authenticated with the in-cluster service account instead has been told something false about their own cluster credentials. The reach is small — the refusal is on the Kata arm of `newBackend`, and Kata is dormant in Phase 2 — and Phase 8 turns Kata on and honours the row then, with contract §8.5's pinning review.

Six decisions where the contract is silent, made here so nobody re-derives them. **(a) `loadConfig` takes a `getenv func(string) string` rather than calling `os.Getenv`.** Contract §4 requires that no package reads the environment at use time; injecting the lookup extends that to the test, which then needs no `t.Setenv` and is safe under `-race` and `t.Parallel()`. **(b) A malformed duration or size exits 2 rather than falling back to the default.** `PANACEA_APP_IDLE_AFTER=90` (no unit) is not a valid Go duration; silently treating it as 90s guesses, and silently treating it as the default ignores an operator who plainly meant something. The one exception is `PANACEA_APP_LOG_LEVEL`, which contract §4 explicitly makes a warn-and-continue. **(c) A sub-minimum `PANACEA_APP_REAP_INTERVAL` is clamped to `5s` with a warning, and so is `0`** — §4 says "minimum enforced value 5s" without saying how, and `time.NewTicker(0)` panics, so clamping is also the only non-crashing reading. **(d) When `LastOutputAt` is the zero time the reaper measures silence from `CreatedAt`.** A session that has never produced output otherwise reads as silent since year 1 and goes `idle` on the first sweep after creation. **(e) Recovery leaves an `exited` session exited even when its sandbox is still live**, logging one warning per occurrence instead of destroying it. §5.6 has no transition out of `exited`, and destroying a container at boot is the destructive option; `DELETE /v1/sessions/{id}?purge=1` (task 11) is the documented way out. This leaves one gap on purpose: such a container is neither reaped (rule 4 only destroys records-less sandboxes) nor attachable. It is visible in `docker ps`, and it is named here rather than papered over. **(f) Rule 2 does not fire on a session whose create is still in flight.** Task 11 writes the registry record with `SandboxID` set *before* it calls `backend.Create`, on purpose, so that a crash between the two leaves a record the reaper can collect. The cost is a window in which the sandbox id is real and the container does not exist yet, and rule 2 as the contract writes it would mark that session `exited` — terminal, for a session that is starting normally. The grace condition is stated once and is narrow: **rule 2 skips a session whose `Status` is `starting` and whose `CreatedAt` is within `Config.CreateGrace` of now**, defaulting to `backend.ReadyTimeout` and fed from `PANACEA_APP_READY_TIMEOUT`, because that is the same budget task 11's readiness watch spends before it flips the status itself. Outside that window a `starting` session whose sandbox is missing really is lost and rule 2 collects it, so nothing leaks. `running` and `idle` sessions are not skipped: their create finished.

**One reconciliation, and `.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-c-package-api-contract.md` has already made it.** `api.Deps` is task 10's type and the contract never declares its fields, so four tasks each invented a spelling; that document fixes them and it overrides this file. Step 11 passes exactly this set, in these spellings: `Store`, `Backend`, `Worktrees`, `Projects`, `Flags`, `Origin` (singular — not `Origins`), `Image` (task 11's field — not `SandboxImage`), `Resources`, `ReadyTimeout`, `BackendReady`, `Log`. `api.New` returns **two** values, `(*API, error)`, and step 11 checks the error rather than assigning one variable. There is **no `IdleAfter` field on `api.Deps`** and this task does not add one: ruling 4 puts the idle window on `reaper.Config`, because the reaper is what derives idle and the API never computes it. The clock is the reaper's for the same reason (`reaper.Config.Now`). If task 10 as landed still spells something differently, rename at that one call site; do not add a field to `api.Deps`, and do not construct a second one.

**Files:**
- `control-plane/cmd/api/config.go` — every `PANACEA_APP_*` variable parsed once into `Config`, the loopback bind refusal, the deferred-warning list (new)
- `control-plane/cmd/api/config_test.go` — `package main`: defaults, the refusal and its override, the clamp, the fallbacks, every rejection (new)
- `control-plane/internal/reaper/reaper.go` — `Reaper`, `Sweep`'s rules in order, `Run`, `BackendReady` (new)
- `control-plane/internal/reaper/reaper_test.go` — `package reaper_test`: a fake backend and a real `FileStore`, one test per rule, a fake clock, never a sleep (new)
- `control-plane/cmd/api/main.go` — **modified**: wiring, `recoverSessions`, the listener, the reaper goroutine, graceful shutdown (replaces the six-line placeholder)
- `control-plane/cmd/api/main_test.go` — **modified**: the §5.5 recovery rules and the exit-2 path (replaces the placeholder `TestMain`)

**Interfaces:**
- Produces:
  - `main.Config` and `main.loadConfig(getenv func(string) string) (Config, error)` — unexported, consumed only by `main.go` and its test
  - `main.recoverSessions(ctx, registry.Store, backend.Backend, *slog.Logger)` — contract §5.5, unexported and directly tested
  - `reaper.Config`, `reaper.New(Config) *Reaper`, `(*Reaper).Sweep(ctx) error`, `(*Reaper).Run(ctx)`, `(*Reaper).BackendReady() bool` — the surface contract §5.3 declares, plus two fields it does not: `Config.IdleAfter` (Group C ruling 4 moves the idle window here, off `api.Deps`) and `Config.CreateGrace` (decision (f)). `BackendReady` is what task 10's `GET /v1/healthz` reports
  - `PANACEA_APP_READY_TIMEOUT` — the one row this task adds to contract §4's table
- Departs from contract §4 in one other place: **`PANACEA_APP_KUBECONFIG` is parsed but not honoured.** `loadConfig` reads it into `Config.Kubeconfig` and `kubeClient` refuses a non-empty value with `backend.ErrUnsupported`, naming the module that would have to be added. The field exists so the error can name the variable, not because anything reads the file. Reasons in the header; Phase 8 honours the row.
- Consumes:
  - task 4: `backend.Backend`, `backend.Sandbox` (including task 5's `NonLoopback`), `backend.State*`, `backend.Kind*`, `backend.Resources`, `backend.ReadyTimeout`, `backend.LabelManaged`, `backend.LabelSandboxID`
  - task 5: `dockerbe.Config`, `dockerbe.New` — `ReadyTimeout` and `Log` are set here for the first time
  - task 6: `katabe.Config`, `katabe.New`, `sandbox.NewManager` — reached only by `PANACEA_APP_BACKEND=kata`
  - task 7: `registry.Store`, `registry.Session`, `registry.Status*`, `registry.NewFileStore`, `registry.DefaultPath`, `registry.WorktreeFlagStore`, `statepath.EnvStateDir`
  - task 8: `project.NewScanner`, `project.NewExecRunner`, `worktree.Provider`, `worktree.Static`
  - task 9: `httpx.NewOriginPolicy`; task 10: `api.New`, `api.Deps`
- Depends on: 5, 6, 10, 11, 12 — this is the last task in the phase and the only one that imports every other package.

- [ ] **Step 1: Write the failing configuration tests**

These pin the two properties an operator's mistake reaches first — the bind refusal and the parse rejections — and the one property task 5 is waiting on, `ReadyTimeout` defaulting to `backend.ReadyTimeout`. Every test injects its environment rather than mutating the process's, so they all run in parallel and none of them can leak into the next.

The comparison helper is deliberate. Twelve `if got != want` blocks is the shape these tests want, but it is also twelve chances to print the wrong variable name in the failure message; a `[]struct{name string; got, want any}` keeps the name and the value adjacent. `any` compares correctly here because every value is a comparable dynamic type — `string`, `int`, `time.Duration`, `slog.Level`.

Create `control-plane/cmd/api/config_test.go`:

```go
package main

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
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
```

Append to the same file:

```go
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
	for _, addr := range []string{"127.0.0.1:0", "127.0.0.1:8730", "localhost:8730", "[::1]:8730"} {
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
```

Append the last group to the same file. `Warnings` is asserted by substring rather than by exact text: the point of the test is that the operator is told, not that the sentence never changes.

```go
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
```

- [ ] **Step 2: Watch the configuration tests fail to compile**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go test ./sandboxd/... ./control-plane/... -race -count=1
```

`loadConfig` and `Config` are package-level identifiers in `package main`, so the compiler reports `undefined:` **once per use site**, not once per symbol, and stops after ten with `too many errors` — which carries the tenth error's own position rather than a new one. There are more than ten call sites in step 1's file, so the transcript is ten lines and the truncation. The existing placeholder `main_test.go` still compiles and contributes nothing. Expect this block, among the `ok` lines from every other package:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api [github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api.test]
cmd\api\config_test.go:23:14: undefined: loadConfig
cmd\api\config_test.go:64:13: undefined: loadConfig
cmd\api\config_test.go:78:16: undefined: loadConfig
cmd\api\config_test.go:87:15: undefined: loadConfig
cmd\api\config_test.go:107:16: undefined: loadConfig
cmd\api\config_test.go:118:12: undefined: loadConfig
cmd\api\config_test.go:128:15: undefined: loadConfig
cmd\api\config_test.go:142:14: undefined: loadConfig
cmd\api\config_test.go:150:16: undefined: loadConfig
cmd\api\config_test.go:159:15: undefined: loadConfig
cmd\api\config_test.go:159:15: too many errors
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api [build failed]
FAIL
```

Those positions are this plan's file written out exactly as step 1 gives it, with one blank line between each appended group. If you reformatted as you pasted, your numbers move and only the shape matters.

Go prints `\` path separators on Windows and `/` elsewhere; that difference is cosmetic. What must be true before you continue is that the failure is `undefined: loadConfig` and nothing else — if a test also fails to compile for a reason of its own, fix that first, because a red step that is red for two reasons proves nothing.

- [ ] **Step 3: Write `control-plane/cmd/api/config.go`**

The identifier block is longer than the parsing. That is the point of contract §4: one file where a misspelled variable is visible, and constants rather than string literals at the use sites. `envStateDir` is `statepath.EnvStateDir` rather than a second copy of the same string, so renaming it breaks the build instead of quietly splitting the ladder in two.

Create `control-plane/cmd/api/config.go`:

```go
package main

import (
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// Every environment variable this process reads, in one block. Contract §4
// forbids any other package calling os.Getenv at use time, so a misspelling is
// visible here and a test constructs a Config instead of editing the
// environment it is running in.
const (
	envAddr                   = "PANACEA_APP_ADDR"
	envAllowPublicBind        = "PANACEA_APP_ALLOW_PUBLIC_BIND"
	envBackend                = "PANACEA_APP_BACKEND"
	envSandboxImage           = "PANACEA_APP_SANDBOX_IMAGE"
	envProjectsDir            = "PANACEA_APP_PROJECTS_DIR"
	envAllowedOrigins         = "PANACEA_APP_ALLOWED_ORIGINS"
	envIdleAfter              = "PANACEA_APP_IDLE_AFTER"
	envIdleTimeout            = "PANACEA_APP_IDLE_TIMEOUT"
	envReapInterval           = "PANACEA_APP_REAP_INTERVAL"
	envSessionCPUMillis       = "PANACEA_APP_SESSION_CPU_MILLIS"
	envSessionMemoryMiB       = "PANACEA_APP_SESSION_MEMORY_MIB"
	envDockerHost             = "PANACEA_APP_DOCKER_HOST"
	envKubeconfig             = "PANACEA_APP_KUBECONFIG"
	envNamespace              = "PANACEA_APP_NAMESPACE"
	envLogLevel               = "PANACEA_APP_LOG_LEVEL"
	envHarnessAllowUnverified = "PANACEA_APP_HARNESS_ALLOW_UNVERIFIED"

	// envReadyTimeout is NOT in contract §4's table. Task 5 records it as a
	// cross-task dependency of this task: dockerbe.Config.ReadyTimeout exists
	// but nothing sets it, so production always gets backend.ReadyTimeout and an
	// operator whose first pull of the sandbox image is cold has no way to wait
	// longer. This task adds exactly this one row to that table.
	envReadyTimeout = "PANACEA_APP_READY_TIMEOUT"

	// envStateDir is statepath's own constant, not a second spelling of it.
	// statepath.Dir reads this variable (contract §2.7) and this file reports
	// it; a rename in either place must break the build, not the ladder.
	envStateDir = statepath.EnvStateDir
)

const (
	defaultAddr         = "127.0.0.1:8730"
	defaultBackend      = "docker"
	defaultSandboxImage = "panacea-app/sandbox:dev"
	defaultNamespace    = "panacea-app"
	defaultIdleAfter    = 90 * time.Second
	defaultReapInterval = 30 * time.Second
	minReapInterval     = 5 * time.Second
)

// Config is the whole of this process's configuration, parsed once at startup.
type Config struct {
	Addr            string
	AllowPublicBind bool
	Backend         string // "docker" or "kata"; validated here, mapped in main.go
	SandboxImage    string
	StateDir        string // "" means the statepath ladder decides
	SessionsPath    string // the registry document, derived from StateDir
	ProjectsDir     string // "" means no projects, and therefore no worktrees
	AllowedOrigins  []string
	IdleAfter       time.Duration // 0 disables idle derivation
	IdleTimeout     time.Duration // 0 means never destroy an idle sandbox
	ReapInterval    time.Duration
	ReadyTimeout    time.Duration
	CPUMillis       int
	MemoryMiB       int
	DockerHost      string
	Kubeconfig      string // parsed so kubeClient can refuse it by name
	Namespace       string
	LogLevel        slog.Level

	HarnessAllowUnverified bool

	// Warnings are the complaints that must not stop the process. They are
	// collected rather than printed because the logger does not exist yet —
	// its level is one of the things being parsed — and main emits them as the
	// first lines it logs.
	Warnings []string
}

// Resources renders the two size variables for backend.Spec.
func (c Config) Resources() backend.Resources {
	return backend.Resources{CPUMillis: c.CPUMillis, MemoryMiB: c.MemoryMiB}
}
```

Append the parser to the same file. The address check is last because it is the one that can refuse to start, and refusing after everything else has parsed means the operator sees the real problem rather than the first problem.

```go
// loadConfig parses the environment through getenv. Taking the lookup as a
// parameter rather than calling os.Getenv is what makes every test in
// config_test.go parallel-safe: nothing here mutates process state.
func loadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	get := func(name, def string) string {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Addr:         get(envAddr, defaultAddr),
		Backend:      strings.ToLower(get(envBackend, defaultBackend)),
		SandboxImage: get(envSandboxImage, defaultSandboxImage),
		StateDir:     get(envStateDir, ""),
		ProjectsDir:  get(envProjectsDir, ""),
		DockerHost:   get(envDockerHost, ""),
		Kubeconfig:   get(envKubeconfig, ""),
		Namespace:    get(envNamespace, defaultNamespace),
		LogLevel:     slog.LevelInfo,
	}
	cfg.AllowPublicBind = parseFlag(get(envAllowPublicBind, ""))
	cfg.HarnessAllowUnverified = parseFlag(get(envHarnessAllowUnverified, ""))

	if cfg.Backend != string(backend.KindDocker) && cfg.Backend != string(backend.KindKata) {
		return Config{}, fmt.Errorf("%s=%q is not a backend: use %q or %q", envBackend, cfg.Backend, backend.KindDocker, backend.KindKata)
	}

	var err error
	// A malformed duration exits rather than falling back. PANACEA_APP_IDLE_AFTER=90
	// is not a Go duration; treating it as 90s guesses, and treating it as the
	// default ignores an operator who plainly meant something.
	if cfg.IdleAfter, err = parseDuration(envIdleAfter, get(envIdleAfter, ""), defaultIdleAfter); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = parseDuration(envIdleTimeout, get(envIdleTimeout, ""), 0); err != nil {
		return Config{}, err
	}
	if cfg.ReapInterval, err = parseDuration(envReapInterval, get(envReapInterval, ""), defaultReapInterval); err != nil {
		return Config{}, err
	}
	if cfg.ReadyTimeout, err = parseDuration(envReadyTimeout, get(envReadyTimeout, ""), backend.ReadyTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadyTimeout <= 0 {
		return Config{}, fmt.Errorf("%s=%q: a sandbox given no time to answer %s can never start", envReadyTimeout, get(envReadyTimeout, ""), backend.HealthPath)
	}
	if cfg.CPUMillis, err = parsePositiveInt(envSessionCPUMillis, get(envSessionCPUMillis, ""), backend.DefaultResources.CPUMillis); err != nil {
		return Config{}, err
	}
	if cfg.MemoryMiB, err = parsePositiveInt(envSessionMemoryMiB, get(envSessionMemoryMiB, ""), backend.DefaultResources.MemoryMiB); err != nil {
		return Config{}, err
	}

	// Contract §4 gives a 5s minimum without saying how it is enforced.
	// Clamping is also the only reading that does not crash: time.NewTicker
	// panics on a non-positive duration, so 0 cannot reach the reaper.
	if cfg.ReapInterval < minReapInterval {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("%s=%v is below the %v minimum; sweeping every %v instead", envReapInterval, cfg.ReapInterval, minReapInterval, minReapInterval))
		cfg.ReapInterval = minReapInterval
	}

	// Contract §4 makes an unknown log level a warning, not an exit: a typo
	// here must not stop the cockpit from starting.
	if raw := get(envLogLevel, ""); raw != "" {
		switch strings.ToLower(raw) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("%s=%q is not one of debug, info, warn, error; logging at info", envLogLevel, raw))
		}
	}

	// The allowlist REPLACES httpx.DefaultOrigins rather than extending it
	// (contract §4), so a value that parses to nothing is an error: silently
	// restoring the defaults it was set to replace is the opposite of what was
	// asked for.
	if raw := strings.TrimSpace(getenv(envAllowedOrigins)); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if o := strings.TrimSpace(part); o != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
			}
		}
		if len(cfg.AllowedOrigins) == 0 {
			return Config{}, fmt.Errorf("%s=%q contains no origin; unset it to use the defaults", envAllowedOrigins, raw)
		}
	}

	if cfg.HarnessAllowUnverified {
		cfg.Warnings = append(cfg.Warnings, envHarnessAllowUnverified+" is set, but harness.Build as landed in task 8 accepts no such option: unverified harnesses stay unavailable until one is added to images/sandbox/Dockerfile and its flags are checked against --help (contract §8.2)")
	}

	// filepath.Join(StateDir, "sessions") is byte-for-byte what statepath.Dir
	// returns when PANACEA_APP_STATE_DIR is set (contract §2.7), so the two
	// cannot disagree; going through registry.DefaultPath when it is unset
	// keeps the rest of the ladder in the one package that owns it.
	if cfg.StateDir != "" {
		cfg.SessionsPath = filepath.Join(cfg.StateDir, "sessions", "sessions.json")
	} else {
		cfg.SessionsPath = registry.DefaultPath()
	}

	if err := validateAddr(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

Append the helpers, still in `config.go`. The refusal message is contract §4's, word for word, including the full stop — an operator who searches the message finds the documentation, and a reworded one finds nothing.

```go
// validateAddr enforces contract §4's loopback rule. It runs last so that an
// operator with two problems is told about the parse error first and the bind
// refusal second, rather than fixing the bind and discovering the parse error
// on the next run.
func validateAddr(cfg *Config) error {
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return fmt.Errorf("%s=%q is not host:port: %w", envAddr, cfg.Addr, err)
	}
	if port == "" {
		return fmt.Errorf("%s=%q has no port", envAddr, cfg.Addr)
	}
	if cfg.AllowPublicBind || isLoopbackHost(host) {
		return nil
	}
	return fmt.Errorf("refusing to bind %s: a sandbox attach socket on a non-loopback address is a shell on the LAN. Set %s=1 to override.", cfg.Addr, envAllowPublicBind)
}

// isLoopbackHost is exactly the three spellings contract §4 lists. 127.0.0.2 is
// also loopback and is deliberately not here: widening this set is a change to
// a locked document, not a tidy-up. An empty host is not "the default" — it is
// every interface — so it falls through to the refusal.
func isLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || strings.EqualFold(host, "localhost")
}

// parseFlag implements contract §4's "any value other than 1 or true is also
// treated as unset", and it means ANY other value: "TRUE" is not "true".
// PANACEA_APP_ALLOW_PUBLIC_BIND is the one variable that disarms a security
// rule, so this comparison is exact rather than EqualFold — a case-insensitive
// widening is a change to a locked document, and the refusal message names the
// spelling that works.
func parseFlag(v string) bool {
	return v == "1" || v == "true"
}

func parseDuration(name, raw string, def time.Duration) (time.Duration, error) {
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a Go duration such as 90s or 5m: %w", name, raw, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s=%q is negative", name, raw)
	}
	return d, nil
}

func parsePositiveInt(name, raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a whole number: %w", name, raw, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero", name, raw)
	}
	return n, nil
}
```

- [ ] **Step 4: Watch the configuration tests pass**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go test ./control-plane/cmd/api/ -race -count=1 -v 2>&1 | tail -30
```

This is the one place a scoped command is worth running: the reaper does not exist yet, so the full workspace command would still be red for a reason that is not this step's. Step 1 writes **ten** tests, and the placeholder `main_test.go` has not been replaced yet, so the package reports **eleven** top-level `--- PASS` lines — the ten below plus `TestMain`, which asserts nothing and goes in step 9. Every one of them must pass and the package must end `ok`. The full command returns in step 12.

```
--- PASS: TestMain (0.00s)
    --- PASS: TestMain/main_imports (0.00s)
--- PASS: TestLoadConfigRefusesANonLoopbackBind (0.00s)
--- PASS: TestHarnessAllowUnverifiedWarnsThatNothingReadsItYet (0.00s)
--- PASS: TestLoadConfigRejectsAnUnknownBackendAndNamesBoth (0.00s)
--- PASS: TestLogLevelFallsBackLoudlyAndNeverExits (0.00s)
--- PASS: TestReapIntervalIsClampedRatherThanTrusted (0.00s)
--- PASS: TestIdleWindowsAndSizesRejectNonsenseRatherThanDefaulting (0.00s)
--- PASS: TestReadyTimeoutIsSettableAndDefaultsToTheBackendConstant (0.00s)
--- PASS: TestAllowPublicBindIsTheOnlyWayPast (0.00s)
--- PASS: TestAllowedOriginsReplaceTheDefaultsAndAnEmptyListIsAnError (0.00s)
--- PASS: TestLoadConfigDefaultsAreAWorkingLaptop (0.01s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api	6.909s
```

Every test calls `t.Parallel()`, so the order they finish in is not the order they are written in and will not be the order above; the `=== RUN`/`=== PAUSE`/`=== CONT` lines `-v` interleaves are noise. Count the `--- PASS` lines, not their sequence.

If `TestLoadConfigDefaultsAreAWorkingLaptop` fails on `SessionsPath` alone, the cause is almost always that `registry.DefaultPath()` was used for both branches: with `PANACEA_APP_STATE_DIR` injected through a fake `getenv`, `statepath.Dir` still reads the real process environment and returns a directory under `%LOCALAPPDATA%`. The `StateDir != ""` branch exists precisely so the injected value wins.

- [ ] **Step 5: Write the failing reaper tests — the fake backend and the safety rules**

`package reaper_test`, external, so the tests only reach the exported surface contract §5.3 declares. The store is a real `registry.FileStore` in `t.TempDir()` rather than a double: task 7 already proved the `FileStore` against `registrytest.RunStoreContract`, and a double here would only prove the reaper against a second implementation of `Update` that might not have the same locking. The clock is a closure over a fixed time — no test sleeps, and `Interval` is set to a minute in every test because none of them call `Run`.

Create `control-plane/internal/reaper/reaper_test.go`:

```go
package reaper_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

// fakeBackend answers List from a fixed slice and records every Destroy. Create
// and Attach are never called: the reaper only ever lists and destroys, and a
// panic here would be a real finding.
type fakeBackend struct {
	mu         sync.Mutex
	sandboxes  []backend.Sandbox
	listErr    error
	destroyErr map[string]error
	destroyed  []string
}

func (f *fakeBackend) Kind() backend.Kind { return backend.KindDocker }

func (f *fakeBackend) Create(context.Context, backend.Spec) (backend.Sandbox, error) {
	panic("reaper called Create")
}

func (f *fakeBackend) Attach(context.Context, string, string) (*websocket.Conn, error) {
	panic("reaper called Attach")
}

func (f *fakeBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sandboxes {
		if s.ID == id {
			return s, nil
		}
	}
	return backend.Sandbox{}, backend.ErrNotFound
}

func (f *fakeBackend) List(context.Context) ([]backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]backend.Sandbox, len(f.sandboxes))
	copy(out, f.sandboxes)
	return out, nil
}

func (f *fakeBackend) Destroy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.destroyErr[id]; err != nil {
		return err
	}
	f.destroyed = append(f.destroyed, id)
	return nil
}

func (f *fakeBackend) Close() error { return nil }

func (f *fakeBackend) wasDestroyed(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.destroyed {
		if d == id {
			return true
		}
	}
	return false
}

func (f *fakeBackend) destroyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.destroyed)
}
```

Append the five fixtures, the tests for rules 1 and 2, and rule 2's grace test (decision (f)) to the same file:

```go
func newStore(t *testing.T) *registry.FileStore {
	t.Helper()
	store, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func seed(t *testing.T, store registry.Store, s registry.Session) {
	t.Helper()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = clock.Add(-time.Hour)
	}
	if err := store.Create(s); err != nil {
		t.Fatalf("seed %s: %v", s.ID, err)
	}
}

func get(t *testing.T, store registry.Store, id string) registry.Session {
	t.Helper()
	s, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return s
}

// managed is a sandbox this control plane created: both labels set, endpoint on
// loopback. Tests override the fields they care about.
func managed(id string, state backend.State) backend.Sandbox {
	return backend.Sandbox{
		ID:        id,
		Kind:      backend.KindDocker,
		Ref:       "container-" + id,
		Endpoint:  "127.0.0.1:49711",
		State:     state,
		CreatedAt: clock.Add(-time.Hour),
		Labels: map[string]string{
			backend.LabelManaged:   "true",
			backend.LabelSandboxID: id,
		},
	}
}

func newReaper(t *testing.T, store registry.Store, be backend.Backend, idleAfter, idleTimeout time.Duration) (*reaper.Reaper, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		IdleAfter:   idleAfter,
		IdleTimeout: idleTimeout,
		Interval:    time.Minute, // no test calls Run
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return r, logs
}

// Rule 1. This is the rule that protects a night's work from a Docker Desktop
// restart, and it is the reason Sweep returns an error instead of swallowing one.
func TestSweepChangesNothingWhenTheBackendIsDown(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	be := &fakeBackend{listErr: errors.New("cannot connect to the Docker daemon")}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep reported success while the backend was down")
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q, want %q: a failed List must never be read as \"every sandbox is gone\"", got, registry.StatusRunning)
	}
	if be.destroyCount() != 0 {
		t.Errorf("destroyed %d sandboxes with no usable list", be.destroyCount())
	}
	if r.BackendReady() {
		t.Error("BackendReady stayed true after List failed")
	}
}

// Rule 2.
func TestSweepMarksADisappearedSandboxExited(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	seed(t, store, registry.Session{ID: "s-2", Status: registry.StatusStarting}) // no sandbox yet
	be := &fakeBackend{}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s1 := get(t, store, "s-1")
	if s1.Status != registry.StatusExited || s1.ExitReason != "sandbox disappeared" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"sandbox disappeared\")", s1.Status, s1.ExitReason)
	}
	if got := get(t, store, "s-2").Status; got != registry.StatusStarting {
		t.Errorf("s-2 = %q: a session with no SandboxID has no sandbox to have lost", got)
	}
	if !r.BackendReady() {
		t.Error("BackendReady is false after a successful sweep")
	}
}

// Rule 2's grace condition, decision (f). The fixture above is the easy half —
// a record with no SandboxID has no sandbox to have lost. The hard half is the
// one task 11 actually writes: SandboxID is set BEFORE backend.Create is
// called, so a sweep landing in that window sees an id no container answers to
// yet, and exited is terminal. Both directions are here, because a grace that
// never expires is a leak rather than a fix.
func TestSweepSparesASessionWhoseCreateIsStillInFlight(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-new", SandboxID: "sb-new", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-2 * time.Second), // Store.Create has run; ContainerCreate has not returned
	})
	seed(t, store, registry.Session{
		ID: "s-stale", SandboxID: "sb-stale", Status: registry.StatusStarting,
		CreatedAt: clock.Add(-10 * time.Minute), // nothing has been in flight for ten minutes
	})
	be := &fakeBackend{} // List succeeds and is empty: neither container exists
	r := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		Interval:    time.Minute,
		CreateGrace: 90 * time.Second,
		Now:         func() time.Time { return clock },
		Log:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-new").Status; got != registry.StatusStarting {
		t.Errorf("s-new = %q, want starting: a sweep inside the create window must not make a terminal decision", got)
	}
	s := get(t, store, "s-stale")
	if s.Status != registry.StatusExited || s.ExitReason != "sandbox disappeared" {
		t.Errorf("s-stale = (%q, %q), want (exited, \"sandbox disappeared\"): the grace is a window, not an exemption", s.Status, s.ExitReason)
	}
}
```

Append rules 3 and 4, and the two `NonLoopback` tests that come with rule 4. That pair is what gives "why did my sandbox vanish" an answer: one proves the destruction happens and is logged distinctly, the other proves that a *failed* destruction does not fabricate a status.

```go
// Rule 3.
func TestSweepMarksAStoppedSandboxExited(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateStopped)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "process exited" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"process exited\")", s.Status, s.ExitReason)
	}
	if be.wasDestroyed("sb-1") {
		t.Error("a stopped sandbox with a live record was destroyed; DELETE with purge owns that, not the reaper")
	}
}

// Rule 4. The crash-recovery path, and the one rule that can destroy a running
// agent if it is wrong, so it is tested from both sides in one test.
func TestSweepDestroysOrphansAndOnlyOrphans(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	unmanaged := managed("sb-3", backend.StateRunning)
	unmanaged.Labels = map[string]string{} // somebody else's container
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		managed("sb-1", backend.StateRunning), // has a record
		managed("sb-2", backend.StateRunning), // orphan
		unmanaged,
	}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-2") {
		t.Error("the orphan survived; a crash between Create and the record write leaks a container forever")
	}
	if be.wasDestroyed("sb-1") || be.wasDestroyed("sb-3") {
		t.Errorf("destroyed something that was not an orphan: %v", be.destroyed)
	}
	if !strings.Contains(logs.String(), "sb-2") {
		t.Errorf("the destruction was silent; logs were:\n%s", logs.String())
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("s-1 = %q, want running", got)
	}
}

// The task-5 amendment to contract §2.4: List keeps a LAN-published sandbox in
// the slice with NonLoopback set and an empty Endpoint, and the reaper is the
// only actor that resolves it.
func TestSweepDestroysANonLoopbackSandboxAndSaysWhy(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	exposed := managed("sb-1", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{exposed}}
	r, logs := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-1") {
		t.Fatal("a sandbox published off 127.0.0.1 survived the sweep")
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited {
		t.Errorf("status = %q, want exited once the sandbox is actually gone", s.Status)
	}
	if !strings.Contains(s.ExitReason, "127.0.0.1") {
		t.Errorf("ExitReason = %q; it must say why, because this is the one disappearance the user did not ask for", s.ExitReason)
	}
	line := logs.String()
	if !strings.Contains(line, "sb-1") || !strings.Contains(line, "127.0.0.1") {
		t.Errorf("the log does not identify the sandbox and the reason:\n%s", line)
	}
}

func TestSweepLeavesTheSessionAloneWhenTheNonLoopbackDestroyFails(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	exposed := managed("sb-1", backend.StateRunning)
	exposed.NonLoopback = true
	be := &fakeBackend{
		sandboxes:  []backend.Sandbox{exposed},
		destroyErr: map[string]error{"sb-1": errors.New("daemon says no")},
	}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q; the process is still running and still on the LAN, so reporting exited would be a fabricated status", got)
	}
}
```

Append the last three tests:

```go
// Rule 5, both directions, plus decision (d).
func TestSweepDerivesIdleFromSilenceAndBackAgain(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-2 * time.Minute),
	})
	// A session that has never produced a byte is measured from CreatedAt. The
	// zero time is not "silent for a very long while"; it is year 1, and it
	// would make every new session idle on the first sweep after creation.
	seed(t, store, registry.Session{
		ID: "s-2", SandboxID: "sb-2", Status: registry.StatusRunning,
		CreatedAt: clock.Add(-30 * time.Second),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		managed("sb-1", backend.StateRunning),
		managed("sb-2", backend.StateRunning),
	}}
	r, _ := newReaper(t, store, be, 90*time.Second, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("first Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusIdle {
		t.Errorf("s-1 = %q after two minutes of silence, want idle", got)
	}
	if got := get(t, store, "s-2").Status; got != registry.StatusRunning {
		t.Errorf("s-2 = %q thirty seconds after creation, want running", got)
	}
	if be.destroyCount() != 0 {
		t.Errorf("idle derivation destroyed something with IdleTimeout unset: %v", be.destroyed)
	}

	if _, err := store.Update("s-1", func(s *registry.Session) error {
		s.LastOutputAt = clock.Add(-5 * time.Second)
		return nil
	}); err != nil {
		t.Fatalf("recording output: %v", err)
	}
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("s-1 = %q after output resumed, want running", got)
	}
}

func TestIdleDerivationIsOffWhenIdleAfterIsZero(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-10 * time.Hour),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateRunning)}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := get(t, store, "s-1").Status; got != registry.StatusRunning {
		t.Errorf("status = %q with PANACEA_APP_IDLE_AFTER=0, want running", got)
	}
}

// Rule 6. Opt-in, and the only rule that destroys a sandbox the user still has
// a record for, so the reason has to survive into the rail.
func TestIdleTimeoutDestroysTheSandboxAndRecordsWhy(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	seed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		LastOutputAt: clock.Add(-30 * time.Minute),
	})
	be := &fakeBackend{sandboxes: []backend.Sandbox{managed("sb-1", backend.StateRunning)}}
	r, _ := newReaper(t, store, be, 90*time.Second, 10*time.Minute)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !be.wasDestroyed("sb-1") {
		t.Fatal("the idle timeout expired and the sandbox is still there")
	}
	s := get(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "idle timeout" {
		t.Errorf("s-1 = (%q, %q), want (exited, \"idle timeout\")", s.Status, s.ExitReason)
	}
}
```

- [ ] **Step 6: Watch the reaper tests fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go test ./control-plane/internal/reaper/ -race -count=1
```

`internal/reaper` now holds one file, and it is `package reaper_test`. There is no implementation to compile against, so this is not an `undefined:` failure — the go command cannot build the package at all, and it prints the directory as an absolute path:

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper: no non-test Go files in C:\Users\au-dev-dmackie1\Projects\PanaceaApp\control-plane\internal\reaper
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper [build failed]
FAIL
```

Once `reaper.go` exists but before it is complete, the failure becomes the familiar `undefined: reaper.New` form, one line per use site, capped at ten.

- [ ] **Step 7: Write `control-plane/internal/reaper/reaper.go`**

Three structural points before the code. **`Sweep` builds two maps up front and then applies the rules in contract §5.3's order over those maps**, rather than calling the backend inside each rule; the sweep therefore does at most one `List` and one `Destroy` per sandbox, and the rules read as the contract reads. **The `gone` set is what keeps the rules from fighting each other**: a sandbox destroyed by the non-loopback rule is still in the `live` snapshot, and without the set the stopped-sandbox rule would overwrite its `ExitReason` with `"process exited"` and lose the only explanation the user gets. **Rule 2 is the one rule whose verdict comes from the snapshot and is then overruled by the clock.** Rules 5 and 6 read `r.now()` too — `deriveIdle` and `enforceIdleTTL` could not be written without it, because idle *is* elapsed time and the snapshot says nothing about it. Rule 2 is the other shape: "this record's sandbox is not in the list any more" is a pure snapshot fact, and decision (f)'s grace window reads the clock only to **suppress** that verdict. A session whose `SandboxID` is set but whose container has not been created yet is indistinguishable, from the snapshot alone, from a session whose container has vanished — the difference is entirely how old the record is.

Create `control-plane/internal/reaper/reaper.go`:

```go
// Package reaper is the periodic sweep that keeps the registry and the backend
// telling the same story. It is a package rather than a loop inside main for
// two reasons: a reaper inside main cannot be unit-tested, and its rules are
// the ones most likely to destroy a running agent by mistake (contract §1.2
// correction 7).
//
// The first rule is "change nothing". Every destructive rule here is downstream
// of a SUCCESSFUL backend.List; a failed List returns immediately, because
// reading "the daemon is restarting" as "every sandbox is gone" would mark
// every session exited, and exited is terminal (contract §5.6).
package reaper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// The exit reasons this package writes. They reach the user through
// registry.Session.ExitReason and the session rail, so they are sentences.
const (
	reasonDisappeared = "sandbox disappeared"
	reasonProcessExit = "process exited"
	reasonIdleTimeout = "idle timeout"

	// reasonNonLoopback answers "why did my sandbox vanish". A sandbox whose
	// port was published off 127.0.0.1 is an unauthenticated shell on the LAN
	// (contract §2.4, backend.ErrNonLoopbackPublish); task 5's List keeps it in
	// the slice with NonLoopback set precisely so this sweep can destroy it,
	// and nothing else in the phase will.
	reasonNonLoopback = "sandbox destroyed: its port was published outside 127.0.0.1"
)

const defaultInterval = 30 * time.Second

// errUnchanged aborts a store.Update without writing. FileStore.Update runs the
// mutate function under its write lock and commits nothing when it returns an
// error (task 7), so this is how a rule that turns out to be a no-op avoids
// rewriting the whole JSON document.
var errUnchanged = errors.New("reaper: nothing to change")

// Config is contract §5.3's, field for field, plus two the contract does not
// name. IdleAfter is here rather than on api.Deps because the reaper is what
// derives idle and the API never computes it (Group C ruling 4). CreateGrace is
// decision (f).
type Config struct {
	Store       registry.Store
	Backend     backend.Backend
	IdleAfter   time.Duration
	IdleTimeout time.Duration
	Interval    time.Duration

	// CreateGrace is how long rule 2 leaves a starting session alone. Task 11
	// writes the registry record with SandboxID set BEFORE it calls
	// backend.Create, so for a moment the id is real and the container is not;
	// marking that session exited is terminal and it is still being created.
	// Zero means backend.ReadyTimeout, which is the same budget task 11's
	// readiness watch spends before it sets the status itself.
	CreateGrace time.Duration

	Now func() time.Time
	Log *slog.Logger
}

// Reaper sweeps. Sweep is exported so tests drive it directly with a fake clock
// and never sleep.
type Reaper struct {
	store       registry.Store
	be          backend.Backend
	idleAfter   time.Duration
	idleTimeout time.Duration
	interval    time.Duration
	createGrace time.Duration
	now         func() time.Time
	log         *slog.Logger

	mu       sync.RWMutex
	ready    bool
	reported bool
}

func New(cfg Config) *Reaper {
	r := &Reaper{
		store:       cfg.Store,
		be:          cfg.Backend,
		idleAfter:   cfg.IdleAfter,
		idleTimeout: cfg.IdleTimeout,
		interval:    cfg.Interval,
		createGrace: cfg.CreateGrace,
		now:         cfg.Now,
		log:         cfg.Log,
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.log == nil {
		r.log = slog.Default()
	}
	r.log = r.log.With("component", "reaper")
	// cmd/api clamps the interval to 5s, but New must not panic on a zero it
	// was handed by a test or a future caller: time.NewTicker(0) does.
	if r.interval <= 0 {
		r.interval = defaultInterval
	}
	// A zero grace would make rule 2 terminal the instant task 11 writes the
	// record, which is the defect this field exists to close, so zero means the
	// default rather than "no grace".
	if r.createGrace <= 0 {
		r.createGrace = backend.ReadyTimeout
	}
	return r
}

// BackendReady is what task 10's GET /v1/healthz reports. It is the result of
// the last sweep, not a fresh probe: the health endpoint must not be able to
// hang on a wedged Docker daemon.
func (r *Reaper) BackendReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ready
}
```

Append `Run` and `Sweep`. Note that `Run` logs a sweep error at **debug**: the loud warning belongs to the ready-state transition, which fires once when the daemon goes away and once when it comes back. A `warn` on every failed sweep would put two lines a minute in the log for as long as Docker Desktop takes to restart, and burying the transition is how an operator stops reading the log.

```go
// Run sweeps immediately and then every Interval until ctx is done. The
// immediate sweep is contract §5.5 step 5: restart recovery deliberately leaves
// orphans alone, because this is the pass that collects them.
func (r *Reaper) Run(ctx context.Context) {
	r.sweepQuietly(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.sweepQuietly(ctx)
		}
	}
}

func (r *Reaper) sweepQuietly(ctx context.Context) {
	if err := r.Sweep(ctx); err != nil {
		r.log.Debug("sweep returned an error", "err", err)
	}
}

// Sweep applies contract §5.3's rules, in order, against one snapshot.
func (r *Reaper) Sweep(ctx context.Context) error {
	// Rule 1: on a failed List, change nothing at all.
	live, err := r.be.List(ctx)
	if err != nil {
		r.markBackendDown(err)
		return fmt.Errorf("reaper: list sandboxes: %w", err)
	}
	sessions, err := r.store.List()
	if err != nil {
		return fmt.Errorf("reaper: list sessions: %w", err)
	}

	bySandbox := make(map[string]backend.Sandbox, len(live))
	for _, sb := range live {
		bySandbox[sb.ID] = sb
	}
	tracked := make(map[string]registry.Session, len(sessions))
	for _, s := range sessions {
		if s.SandboxID != "" {
			tracked[s.SandboxID] = s
		}
	}
	// gone holds what this sweep destroyed. The live snapshot still contains
	// those sandboxes, and without this set a later rule would overwrite the
	// ExitReason that explains the destruction.
	gone := make(map[string]bool)

	// Rule 1a, task 5's amendment to contract §2.4: a sandbox reported with
	// NonLoopback is published where the LAN can reach it. It is destroyed
	// first, before any rule can decide it is merely running.
	for _, sb := range live {
		if !sb.NonLoopback {
			continue
		}
		sess, known := tracked[sb.ID]
		if err := r.be.Destroy(ctx, sb.ID); err != nil {
			r.log.Error("could not destroy a sandbox published outside 127.0.0.1; it is still reachable from the LAN",
				"sandbox", sb.ID, "session", sess.ID, "ref", sb.Ref, "err", err)
			continue
		}
		gone[sb.ID] = true
		r.log.Warn("destroyed a sandbox whose port was published outside 127.0.0.1",
			"sandbox", sb.ID, "session", sess.ID, "ref", sb.Ref)
		if known {
			r.markExited(sess.ID, reasonNonLoopback)
		}
	}

	// Rule 2: a record whose sandbox is not in the list any more.
	now := r.now()
	for _, s := range sessions {
		if s.SandboxID == "" || s.Status == registry.StatusExited {
			continue
		}
		// Decision (f). Task 11 writes SandboxID before it calls
		// backend.Create, so inside the create window this id is real and the
		// container does not exist yet. exited is terminal (contract §5.6), so
		// the one rule that can see a half-built session must be the one that
		// waits. Only starting sessions are spared, and only for CreateGrace:
		// after that a starting session whose sandbox is missing really is
		// lost, and rule 2 collects it on the next sweep.
		if s.Status == registry.StatusStarting && now.Sub(s.CreatedAt) < r.createGrace {
			continue
		}
		if _, alive := bySandbox[s.SandboxID]; !alive {
			r.markExited(s.ID, reasonDisappeared)
		}
	}

	// Rule 3: the sandbox is there and its process is not.
	for _, s := range sessions {
		if s.SandboxID == "" || s.Status == registry.StatusExited || gone[s.SandboxID] {
			continue
		}
		sb, alive := bySandbox[s.SandboxID]
		if !alive {
			continue
		}
		if sb.State == backend.StateStopped || sb.State == backend.StateGone {
			r.markExited(s.ID, reasonProcessExit)
		}
	}

	// Rule 4: ours, but no record. This is the crash-recovery path, and it is
	// why task 11 writes the registry record BEFORE it creates the container.
	for _, sb := range live {
		if gone[sb.ID] || sb.Labels[backend.LabelManaged] != "true" {
			continue
		}
		id := sb.Labels[backend.LabelSandboxID]
		if id == "" {
			// Ours and unmappable. Docker labels are immutable, so this cannot
			// be a sandbox that lost its label; it is one created by a build
			// that set LabelManaged and nothing else.
			gone[sb.ID] = r.destroyOrphan(ctx, sb, "managed sandbox carries no "+backend.LabelSandboxID+" label")
			continue
		}
		if _, known := tracked[id]; !known {
			gone[sb.ID] = r.destroyOrphan(ctx, sb, "no session record")
		}
	}

	// Rule 5: idle derivation, opt-out via PANACEA_APP_IDLE_AFTER=0.
	if r.idleAfter > 0 {
		for _, s := range sessions {
			if gone[s.SandboxID] {
				continue
			}
			if s.Status == registry.StatusRunning || s.Status == registry.StatusIdle {
				r.deriveIdle(s.ID)
			}
		}
	}

	// Rule 6: idle TTL, opt-IN via PANACEA_APP_IDLE_TIMEOUT. Zero means never,
	// and that is the default on purpose: an agent left thinking overnight must
	// still be there in the morning.
	if r.idleTimeout > 0 {
		for _, s := range sessions {
			if s.SandboxID != "" && !gone[s.SandboxID] {
				r.enforceIdleTTL(ctx, s.ID, gone)
			}
		}
	}

	r.markBackendUp()
	return nil
}
```

Append the helpers. One consequence of running rule 5 before rule 6, which is the order contract §5.3 gives: if an operator sets `PANACEA_APP_IDLE_TIMEOUT` lower than `PANACEA_APP_IDLE_AFTER`, a session can go `running` → `idle` → destroyed inside a single sweep. That is the contract's order and it is not silently repaired here; it is worth a line in the Phase 2 runbook.

```go
// destroyOrphan reports whether the sandbox is now gone. A failed destroy is a
// warning and a retry next sweep, never an error that stops the loop: the
// reaper and an explicit DELETE race by design, and a reaper that gives up on
// "already gone" stops sweeping everything else too.
func (r *Reaper) destroyOrphan(ctx context.Context, sb backend.Sandbox, why string) bool {
	if err := r.be.Destroy(ctx, sb.ID); err != nil {
		r.log.Warn("could not destroy an orphan sandbox; retrying next sweep",
			"sandbox", sb.ID, "ref", sb.Ref, "why", why, "err", err)
		return false
	}
	r.log.Info("destroyed an orphan sandbox", "sandbox", sb.ID, "ref", sb.Ref, "why", why)
	return true
}

// enforceIdleTTL destroys first and records second. If the destroy fails the
// session stays idle: the process really is still running, and reporting exited
// for a sandbox that is still there is the fabricated status this phase rules
// out.
func (r *Reaper) enforceIdleTTL(ctx context.Context, id string, gone map[string]bool) {
	// Re-read. Rule 5 may have changed this session a moment ago, and the
	// snapshot at the top of the sweep is stale. This is the only Get in the
	// package and it is not a get-then-put: the write below still goes through
	// Update (contract §2.8's concurrency discipline).
	cur, err := r.store.Get(id)
	if err != nil {
		return
	}
	if cur.Status != registry.StatusIdle || cur.SandboxID == "" {
		return
	}
	if r.now().Sub(lastActivity(cur)) <= r.idleTimeout {
		return
	}
	if err := r.be.Destroy(ctx, cur.SandboxID); err != nil {
		r.log.Warn("could not destroy an idle-timed-out sandbox; leaving the session idle",
			"session", id, "sandbox", cur.SandboxID, "err", err)
		return
	}
	gone[cur.SandboxID] = true
	r.log.Info("destroyed a sandbox that exceeded the idle timeout",
		"session", id, "sandbox", cur.SandboxID, "idleTimeout", r.idleTimeout)
	r.markExited(id, reasonIdleTimeout)
}

func (r *Reaper) deriveIdle(id string) {
	now := r.now()
	_, err := r.store.Update(id, func(s *registry.Session) error {
		silent := now.Sub(lastActivity(*s))
		switch {
		case s.Status == registry.StatusRunning && silent > r.idleAfter:
			s.Status = registry.StatusIdle
		case s.Status == registry.StatusIdle && silent <= r.idleAfter:
			s.Status = registry.StatusRunning
		default:
			return errUnchanged
		}
		return nil
	})
	r.reportUpdate(id, err)
}

func (r *Reaper) markExited(id, reason string) {
	_, err := r.store.Update(id, func(s *registry.Session) error {
		if s.Status == registry.StatusExited {
			return errUnchanged // exited is terminal; the first reason is the true one
		}
		s.Status = registry.StatusExited
		s.ExitReason = reason
		s.AttachCount = 0 // nothing is connected to a process that has ended
		return nil
	})
	r.reportUpdate(id, err)
}

func (r *Reaper) reportUpdate(id string, err error) {
	if err == nil || errors.Is(err, errUnchanged) || errors.Is(err, registry.ErrNotFound) {
		return // ErrNotFound: deleted between the snapshot and now, which is fine
	}
	r.log.Warn("could not update a session", "session", id, "err", err)
}

// lastActivity is when this session was last observably doing something. A
// session that has never produced output has a zero LastOutputAt, and
// now.Sub(zero) is two thousand years: measuring from CreatedAt instead is what
// stops a session going idle on the first sweep after it is created.
func lastActivity(s registry.Session) time.Time {
	if !s.LastOutputAt.IsZero() {
		return s.LastOutputAt
	}
	return s.CreatedAt
}

func (r *Reaper) markBackendDown(err error) {
	r.mu.Lock()
	announce := !r.reported || r.ready
	r.ready, r.reported = false, true
	r.mu.Unlock()
	if announce {
		r.log.Warn("backend is not answering; session statuses are left exactly as they are until it does", "err", err)
	}
}

func (r *Reaper) markBackendUp() {
	r.mu.Lock()
	announce := !r.reported || !r.ready
	r.ready, r.reported = true, true
	r.mu.Unlock()
	if announce {
		r.log.Info("backend is answering")
	}
}
```

- [ ] **Step 8: Watch the reaper tests pass**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go test ./control-plane/internal/reaper/ -race -count=1 -v 2>&1 | tail -30
```

All ten tests report `--- PASS` and the package ends `ok`:

```
--- PASS: TestSweepDestroysOrphansAndOnlyOrphans (2.34s)
--- PASS: TestIdleDerivationIsOffWhenIdleAfterIsZero (2.39s)
--- PASS: TestSweepLeavesTheSessionAloneWhenTheNonLoopbackDestroyFails (2.40s)
--- PASS: TestSweepDestroysANonLoopbackSandboxAndSaysWhy (2.42s)
--- PASS: TestIdleTimeoutDestroysTheSandboxAndRecordsWhy (2.43s)
--- PASS: TestSweepSparesASessionWhoseCreateIsStillInFlight (2.48s)
--- PASS: TestSweepMarksADisappearedSandboxExited (2.48s)
--- PASS: TestSweepDerivesIdleFromSilenceAndBackAgain (2.56s)
--- PASS: TestSweepChangesNothingWhenTheBackendIsDown (0.31s)
--- PASS: TestSweepMarksAStoppedSandboxExited (0.27s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper	7.064s
```

The seconds are `-race` plus a `FileStore` per test writing to a real `t.TempDir()`; none of these tests sleeps. They are parallel, so the order is not the file's order.

Two failures worth recognising on sight. If `TestSweepDestroysANonLoopbackSandboxAndSaysWhy` passes but `TestSweepMarksAStoppedSandboxExited` now fails with `ExitReason` set to the non-loopback sentence, the `gone` set is not being consulted by rule 3. If `TestSweepSparesASessionWhoseCreateIsStillInFlight` fails on `s-new` while every other test passes, rule 2 is missing decision (f)'s grace and is marking a session that is still being created `exited`, which is terminal — that is the bug the test exists for, not a flake.

- [ ] **Step 9: Replace `main_test.go` with the restart-recovery tests**

The file that exists today is a placeholder whose only test asserts nothing; delete its contents rather than adding to them. (Its function is named `TestMain` but takes a `*testing.T`, so it is an ordinary test rather than the `func TestMain(m *testing.M)` entry point — nothing depends on it.)

These tests drive `recoverSessions` directly, with a real `FileStore` and a fake backend, so they need no Docker daemon and no listener. The last one drives `run` itself, which is the only cheap way to prove the exit-2 path end to end: `loadConfig` fails before anything is constructed, so nothing binds and nothing is dialled.

Replace `control-plane/cmd/api/main_test.go` entirely:

```go
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

var bootClock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

// recoveryBackend answers List and nothing else. recoverSessions must never
// create, attach to or destroy anything: it runs before the listener, and a
// boot that destroys a container is a boot that loses work.
type recoveryBackend struct {
	mu        sync.Mutex
	sandboxes []backend.Sandbox
	listErr   error
}

func (f *recoveryBackend) Kind() backend.Kind { return backend.KindDocker }

func (f *recoveryBackend) Create(context.Context, backend.Spec) (backend.Sandbox, error) {
	panic("recovery called Create")
}

func (f *recoveryBackend) Attach(context.Context, string, string) (*websocket.Conn, error) {
	panic("recovery called Attach")
}

func (f *recoveryBackend) Destroy(context.Context, string) error {
	panic("recovery called Destroy")
}

func (f *recoveryBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sandboxes {
		if s.ID == id {
			return s, nil
		}
	}
	return backend.Sandbox{}, backend.ErrNotFound
}

func (f *recoveryBackend) List(context.Context) ([]backend.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]backend.Sandbox, len(f.sandboxes))
	copy(out, f.sandboxes)
	return out, nil
}

func (f *recoveryBackend) Close() error { return nil }

func bootStore(t *testing.T) *registry.FileStore {
	t.Helper()
	store, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func bootSeed(t *testing.T, store registry.Store, s registry.Session) {
	t.Helper()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = bootClock.Add(-time.Hour)
	}
	if err := store.Create(s); err != nil {
		t.Fatalf("seed %s: %v", s.ID, err)
	}
}

func bootGet(t *testing.T, store registry.Store, id string) registry.Session {
	t.Helper()
	s, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return s
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
```

Append the tests to the same file:

```go
// Contract §5.5 step 3. The endpoint on disk is stale the moment Docker Desktop
// restarts, and attaching to a stale endpoint is connection-refused, which
// looks exactly like a dead agent.
func TestRecoverRepublishesTheEndpoint(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning,
		Endpoint: "127.0.0.1:49711", SandboxRef: "container-old", AttachCount: 3,
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-new",
		Endpoint: "127.0.0.1:52002", State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Endpoint != "127.0.0.1:52002" {
		t.Errorf("Endpoint = %q, want the port the daemon republished on", s.Endpoint)
	}
	if s.SandboxRef != "container-new" {
		t.Errorf("SandboxRef = %q, want container-new", s.SandboxRef)
	}
	if s.Status != registry.StatusRunning {
		t.Errorf("Status = %q, want running", s.Status)
	}
	if s.AttachCount != 0 {
		t.Errorf("AttachCount = %d, want 0: no client is attached to a process that just came back", s.AttachCount)
	}
}

// Contract §5.5 steps 3 and 4.
func TestRecoverSettlesMissingAndStoppedSandboxes(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning})
	bootSeed(t, store, registry.Session{ID: "s-2", SandboxID: "sb-2", Status: registry.StatusIdle})
	bootSeed(t, store, registry.Session{ID: "s-3", Status: registry.StatusStarting})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-2", Kind: backend.KindDocker, Ref: "container-sb-2", State: backend.StateStopped,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s1 := bootGet(t, store, "s-1")
	if s1.Status != registry.StatusExited || s1.ExitReason != "sandbox disappeared while the app was not running" {
		t.Errorf("s-1 = (%q, %q)", s1.Status, s1.ExitReason)
	}
	s2 := bootGet(t, store, "s-2")
	if s2.Status != registry.StatusExited || s2.ExitReason != "process exited" {
		t.Errorf("s-2 = (%q, %q)", s2.Status, s2.ExitReason)
	}
	if got := bootGet(t, store, "s-3").Status; got != registry.StatusStarting {
		t.Errorf("s-3 = %q: a session with no SandboxID has no sandbox to recover", got)
	}
}

// Contract §5.5 step 2, and the failure mode it names: marking everything
// exited because Docker Desktop had not finished starting loses a night's work.
func TestRecoverChangesNothingWhenTheBackendIsDown(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711",
	})
	be := &recoveryBackend{listErr: errors.New("docker daemon is still starting")}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Status != registry.StatusRunning || s.Endpoint != "127.0.0.1:49711" {
		t.Errorf("s-1 = (%q, %q), want it untouched so the reaper can converge later", s.Status, s.Endpoint)
	}
}

// Contract §5.6: there is no transition out of exited. Decision (e): recovery
// does not destroy the lingering sandbox either, it says so and moves on.
func TestRecoverNeverResurrectsAnExitedSession(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusExited, ExitReason: "process exited",
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-sb-1",
		Endpoint: "127.0.0.1:52002", State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Status != registry.StatusExited || s.ExitReason != "process exited" {
		t.Errorf("s-1 = (%q, %q), want the terminal state it already had", s.Status, s.ExitReason)
	}
}

// A LAN-published sandbox reports NonLoopback with an empty Endpoint (task 5).
// Recovery must overwrite the stored endpoint with that empty string rather
// than keeping the old one, and must NOT invent a status: the reaper destroys
// it on its first sweep, which happens immediately.
func TestRecoverClearsTheEndpointOfANonLoopbackSandbox(t *testing.T) {
	t.Parallel()
	store := bootStore(t)
	bootSeed(t, store, registry.Session{
		ID: "s-1", SandboxID: "sb-1", Status: registry.StatusRunning, Endpoint: "127.0.0.1:49711",
	})
	be := &recoveryBackend{sandboxes: []backend.Sandbox{{
		ID: "sb-1", Kind: backend.KindDocker, Ref: "container-sb-1",
		Endpoint: "", NonLoopback: true, State: backend.StateRunning,
	}}}

	recoverSessions(context.Background(), store, be, quietLogger())

	s := bootGet(t, store, "s-1")
	if s.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty: nothing may attach to a sandbox on the LAN", s.Endpoint)
	}
	if s.Status != registry.StatusRunning {
		t.Errorf("Status = %q; the process is still running and the reaper owns the destruction", s.Status)
	}
}

func TestRunRefusesANonLoopbackBindWithExitCode2(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	code := run(env(map[string]string{"PANACEA_APP_ADDR": "0.0.0.0:8730"}), &out)
	if code != 2 {
		t.Errorf("run returned %d, want 2", code)
	}
	if !strings.Contains(out.String(), "a sandbox attach socket on a non-loopback address is a shell on the LAN") {
		t.Errorf("run printed %q, which does not explain the refusal", out.String())
	}
}
```

- [ ] **Step 10: Watch the recovery tests fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go test ./control-plane/cmd/api/ -race -count=1
```

There are exactly six use sites of two missing identifiers — `recoverSessions` in five tests and `run` in one — so this is six lines and no `too many errors` truncation:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api [github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api.test]
cmd\api\main_test.go:NN:2: undefined: recoverSessions
cmd\api\main_test.go:NN:2: undefined: recoverSessions
cmd\api\main_test.go:NN:2: undefined: recoverSessions
cmd\api\main_test.go:NN:2: undefined: recoverSessions
cmd\api\main_test.go:NN:2: undefined: recoverSessions
cmd\api\main_test.go:NN:10: undefined: run
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/cmd/api [build failed]
FAIL
```

If you see a seventh line naming `env`, `config_test.go` was not written or was written in a different package; both files are `package main` and share that helper.

- [ ] **Step 11: Rewrite `control-plane/cmd/api/main.go`**

Four things in here are decisions rather than plumbing, and each has a comment in the code saying so. **`run` takes `getenv` and an `io.Writer`** so the exit-2 path is testable without a process. **The `http.Server` sets `ReadHeaderTimeout` and nothing else** — a `ReadTimeout` or `WriteTimeout` would kill `/attach`, which is a WebSocket that sits idle for minutes and then streams for an hour; that is the single most common way a relay gets broken by a "hardening" change. **`srv.Shutdown` does not close attach sockets**, because `coder/websocket` hijacks the connection and `net/http` does not track hijacked connections: shutdown returns as soon as the plain HTTP handlers are done, and the relays die when the process does. That is acceptable precisely because Group A made the PTY survive detach — the sandbox keeps running and the desktop reattaches with scrollback replay. **The reaper is stopped and drained by a `defer` registered after the backend's `Close`**, so it unwinds before it on every exit path, including the one where `srv.Serve` fails and `serve` returns without reaching the shutdown block; `be.Close()` running underneath an in-flight sweep is a `Destroy` against a closed Docker client.

Replace `control-plane/cmd/api/main.go` entirely:

```go
// Command api is the panacea-app control plane: one HTTP surface on loopback,
// one JSON registry on disk, one backend, one reaper. It owns no logic of its
// own beyond wiring, configuration and contract §5.5's restart recovery.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	// k8s.io/client-go/kubernetes and .../rest and nothing else from that
	// module. k8s.io/client-go/tools/clientcmd — the package that reads a
	// kubeconfig FILE — is deliberately absent: it pulls github.com/imdario/mergo
	// and github.com/spf13/pflag, and mergo is in neither go.mod nor go.sum, so
	// importing it makes every GOPROXY=off command in step 12 fail with
	// "updates to go.mod needed". Group C ruling 8: no new module enters go.mod
	// in this task. See kubeClient below for what that costs.
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/reaper"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

func main() { os.Exit(run(os.Getenv, os.Stderr)) }

// run is main with its two impure edges as parameters, so that the exit-2 path
// is a unit test rather than a process. Exit codes: 2 is a configuration
// refusal the operator must fix, 1 is a runtime failure, 0 is a clean signal.
func run(getenv func(string) string, errOut io.Writer) int {
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintf(errOut, "panacea-app: %v\n", err)
		return 2
	}
	log := slog.New(slog.NewTextHandler(errOut, &slog.HandlerOptions{Level: cfg.LogLevel}))
	for _, w := range cfg.Warnings {
		log.Warn(w)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := serve(ctx, cfg, log); err != nil {
		log.Error("control plane stopped", "err", err)
		return 1
	}
	return 0
}
```

Append `serve` to the same file. **The `api.Deps` literal is the reconciliation point named in the Interfaces block**: task 10 owns those field names, and if they differ, rename here rather than adding fields there.

```go
func serve(ctx context.Context, cfg Config, log *slog.Logger) error {
	be, err := newBackend(cfg, log)
	if err != nil {
		return err
	}
	defer func() { _ = be.Close() }()

	// statepath.Dir creates nothing on purpose (contract §2.7) — the caller
	// knows whether a missing directory is an error. Here it is not.
	if err := os.MkdirAll(filepath.Dir(cfg.SessionsPath), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	store, err := registry.NewFileStore(cfg.SessionsPath)
	if err != nil {
		return fmt.Errorf("open session registry %s: %w", cfg.SessionsPath, err)
	}

	// Contract §5.5, once, before anything can serve a request from a stale
	// endpoint.
	recoverSessions(ctx, store, be, log)

	rp := reaper.New(reaper.Config{
		Store:       store,
		Backend:     be,
		IdleAfter:   cfg.IdleAfter,
		IdleTimeout: cfg.IdleTimeout,
		Interval:    cfg.ReapInterval,
		CreateGrace: cfg.ReadyTimeout,
		Log:         log,
	})
	// The reaper gets its own context rather than ctx, so that a SIGINT drains
	// the HTTP server while the reaper is still sweeping a live backend; ctx is
	// already cancelled by the time Shutdown is called, and a reaper on ctx
	// would have stopped at the signal.
	reapCtx, stopReaper := context.WithCancel(context.Background())
	reaperDone := make(chan struct{})
	go func() {
		defer close(reaperDone)
		rp.Run(reapCtx)
	}()
	// Registered AFTER the backend's Close and therefore run BEFORE it: defers
	// unwind last-in-first-out, and be.Close() under an in-flight sweep is a
	// Destroy against a closed Docker client. This one defer covers every exit
	// from here down, including the serve-error path that returns without
	// reaching the shutdown block.
	defer func() {
		stopReaper()
		<-reaperDone
	}()

	scanner := project.NewScanner(cfg.ProjectsDir, project.NewExecRunner())
	// Contract §4: with no projects directory the provider is worktree.Static{},
	// so every session create fails 404 worktree_not_found. That is correct and
	// visible — no projects directory means no worktrees to work in.
	var worktrees worktree.Provider = worktree.Static{}
	if cfg.ProjectsDir != "" {
		worktrees = scanner
	}

	// api.New returns (*API, error): it validates every dependency it cannot
	// default and names the nil one, because a wiring mistake should stop the
	// process here with a sentence rather than panic on the first request.
	// There is no IdleAfter field to pass — the idle window belongs to the
	// reaper above, which is the only thing that derives idle.
	handler, err := api.New(api.Deps{
		Store:        store,
		Backend:      be,
		Worktrees:    worktrees,
		Projects:     scanner,
		Flags:        store, // registry.WorktreeFlagStore, implemented by *FileStore
		Origin:       httpx.NewOriginPolicy(cfg.AllowedOrigins, log),
		Image:        cfg.SandboxImage,
		Resources:    cfg.Resources(),
		ReadyTimeout: cfg.ReadyTimeout,
		BackendReady: rp.BackendReady,
		Log:          log,
	})
	if err != nil {
		return fmt.Errorf("build the HTTP surface: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}

	srv := &http.Server{
		Handler: handler,
		// ReadHeaderTimeout and nothing else. A ReadTimeout or a WriteTimeout
		// would kill /attach: the relay is a WebSocket that is idle for minutes
		// and then streams for an hour, and both timeouts apply to it.
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("control plane listening",
		"addr", ln.Addr().String(),
		"backend", be.Kind(),
		"sessions", cfg.SessionsPath,
		"projects", cfg.ProjectsDir,
		"image", cfg.SandboxImage,
	)

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Shutdown does not close hijacked connections, and coder/websocket hijacks
	// every /attach. Those relays end when the process does; the sandbox and its
	// PTY survive (Group A task 3), so the desktop reattaches with scrollback.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("graceful shutdown did not finish in time; closing what is left", "err", err)
		_ = srv.Close()
	}
	// The reaper is stopped and drained by the deferred function above, after
	// the HTTP server has finished and before the backend is closed.
	return nil
}

func newBackend(cfg Config, log *slog.Logger) (backend.Backend, error) {
	switch cfg.Backend {
	case string(backend.KindDocker):
		// BindIP is deliberately not configurable: dockerbe defaults it to
		// 127.0.0.1 and refuses anything else, and there is no safe non-loopback
		// value to expose as an environment variable.
		return dockerbe.New(dockerbe.Config{
			Host:         cfg.DockerHost,
			ReadyTimeout: cfg.ReadyTimeout,
			Log:          log,
		})
	case string(backend.KindKata):
		client, err := kubeClient(cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kata backend: %w", err)
		}
		return katabe.New(katabe.Config{
			Manager:      sandbox.NewManager(client, cfg.Namespace),
			ReadyTimeout: cfg.ReadyTimeout,
		})
	default:
		// loadConfig already refused anything else; this arm exists so the
		// switch is total, not because a user can reach it.
		return nil, fmt.Errorf("unknown backend %q", cfg.Backend)
	}
}

// kubeClient builds a Kubernetes client for the Kata path from the in-cluster
// service account, and refuses PANACEA_APP_KUBECONFIG rather than honouring it.
//
// Reading a kubeconfig FILE needs k8s.io/client-go/tools/clientcmd, which drags
// github.com/imdario/mergo and github.com/spf13/pflag into go.mod — modules no
// other package here needs and that go.sum does not pin. Group C ruling 8 keeps
// them out: Kata is dormant in Phase 2, the Docker backend is what this task
// wires, and the real Kata deployment runs inside the cluster, where the
// service account is the credential anyway. So the variable is refused with a
// sentence naming what would have to change, which is honest, rather than
// ignored, which is not. loadConfig still parses it into Config.Kubeconfig for
// exactly that reason: the value has to reach here for the error to name it.
// Phase 8 turns Kata on and adds the module then, with contract 8.5's pinning
// review it deserves.
func kubeClient(kubeconfig string) (*kubernetes.Clientset, error) {
	if kubeconfig != "" {
		return nil, fmt.Errorf("%s is set, but Phase 2 reads Kubernetes credentials from the in-cluster service account only — a kubeconfig file needs k8s.io/client-go/tools/clientcmd, which this phase does not add to go.mod: %w", envKubeconfig, backend.ErrUnsupported)
	}
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		// Outside a cluster this is the expected state of every developer
		// laptop, so it is ErrUnavailable and not a panic.
		return nil, fmt.Errorf("%w: %v", backend.ErrUnavailable, err)
	}
	return kubernetes.NewForConfig(restCfg)
}
```

Append the recovery, still in `main.go`:

```go
// recoverSessions is contract §5.5. It runs once, before the listener, and it
// is not part of the reaper on purpose: the reaper converges a system that is
// running, this reconciles one that was not. Orphans are left to the reaper's
// first sweep, which starts immediately.
func recoverSessions(ctx context.Context, store registry.Store, be backend.Backend, log *slog.Logger) {
	sessions, err := store.List()
	if err != nil {
		log.Warn("could not read the session registry at startup", "err", err)
		return
	}
	if len(sessions) == 0 {
		return
	}
	live, err := be.List(ctx)
	if err != nil {
		// THE failure mode this rule exists to prevent: marking everything
		// exited because Docker Desktop had not finished starting loses a
		// night's work, and exited is terminal (contract §5.6).
		log.Warn("backend did not answer at startup; every session is left exactly as it was and the reaper will converge",
			"sessions", len(sessions), "err", err)
		return
	}
	bySandbox := make(map[string]backend.Sandbox, len(live))
	for _, sb := range live {
		bySandbox[sb.ID] = sb
	}

	for _, s := range sessions {
		if s.SandboxID == "" {
			continue // nothing was ever created for this record
		}
		sb, alive := bySandbox[s.SandboxID]
		if !alive {
			if s.Status == registry.StatusExited {
				continue
			}
			applyUpdate(store, log, s.ID, func(rec *registry.Session) {
				rec.Status = registry.StatusExited
				rec.ExitReason = "sandbox disappeared while the app was not running"
				rec.AttachCount = 0
			})
			continue
		}
		if s.Status == registry.StatusExited {
			// §5.6 has no transition out of exited, so the record stays
			// terminal — and the container is not destroyed here either, because
			// destroying at boot is the irreversible choice. DELETE
			// /v1/sessions/{id}?purge=1 owns that. The consequence is a
			// container the reaper will not collect while the record exists;
			// this line is the only warning anyone gets about it.
			log.Warn("a finished session still has a live sandbox; delete the session with purge to remove it",
				"session", s.ID, "sandbox", s.SandboxID, "state", sb.State)
			continue
		}
		if s.Endpoint != sb.Endpoint {
			// Logged out here rather than inside the mutate function: the store
			// holds its write lock while that runs.
			log.Info("sandbox endpoint moved while the app was not running",
				"session", s.ID, "was", s.Endpoint, "now", sb.Endpoint)
		}
		applyUpdate(store, log, s.ID, func(rec *registry.Session) {
			// Overwritten unconditionally, including with "": a NonLoopback
			// sandbox reports an empty endpoint (task 5), and keeping the old
			// one would let a client attach to something on the LAN.
			rec.Endpoint = sb.Endpoint
			rec.SandboxRef = sb.Ref
			rec.AttachCount = 0
			switch sb.State {
			case backend.StateRunning:
				// The reaper re-derives idle on its first sweep.
				rec.Status = registry.StatusRunning
			case backend.StateStopped, backend.StateGone:
				rec.Status = registry.StatusExited
				rec.ExitReason = "process exited"
			case backend.StateStarting:
				// The one row this task adds to contract §5.5 step 3, which
				// names only StateRunning and StateStopped. A sandbox that is
				// still starting is neither: the control plane went down
				// between Create and the first /healthz answer. Reporting
				// running would fabricate a status, exited is terminal, and
				// starting is simply true. The reaper re-derives it within one
				// interval.
				rec.Status = registry.StatusStarting
			}
		})
	}
}

func applyUpdate(store registry.Store, log *slog.Logger, id string, apply func(*registry.Session)) {
	if _, err := store.Update(id, func(s *registry.Session) error {
		apply(s)
		return nil
	}); err != nil {
		log.Warn("could not update a session during recovery", "session", id, "err", err)
	}
}
```

- [ ] **Step 12: Run the whole workspace, vet it, and start the binary once**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off gofmt -l sandboxd control-plane && GOPROXY=off go vet ./sandboxd/... ./control-plane/... && GOPROXY=off go test ./sandboxd/... ./control-plane/... -race -count=1
```

`gofmt -l` prints the names of files that need formatting and nothing when they are all clean, so silence there is the pass. `go test ./...` is invalid at this workspace root — it fails with `directory prefix . does not contain modules listed in go.work` — which is why both package sets are named. Every package must end `ok` or `no test files`.

Then prove the three startup paths by hand, because no test starts a listener. Build outside the repository so nothing untracked lands in it:

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && GOPROXY=off go build -o /tmp/panacea-api.exe ./control-plane/cmd/api && PANACEA_APP_ADDR=0.0.0.0:8730 /tmp/panacea-api.exe; echo "exit=$?"
```

**Path one, the refusal.** Exactly one line on stderr, and `exit=2`:

```
panacea-app: refusing to bind 0.0.0.0:8730: a sandbox attach socket on a non-loopback address is a shell on the LAN. Set PANACEA_APP_ALLOW_PUBLIC_BIND=1 to override.
exit=2
```

**Path two, a working laptop.** Run it with no variables set at all. Two lines, the second from the reaper's immediate first sweep:

```
time=2026-09-22T11:52:27.510+10:00 level=INFO msg="control plane listening" addr=127.0.0.1:8730 backend=docker sessions=C:\Users\au-dev-dmackie1\AppData\Local\PanaceaApp\state\sessions\sessions.json projects="" image=panacea-app/sandbox:dev
time=2026-09-22T11:52:27.590+10:00 level=INFO msg="backend is answering" component=reaper
```

Stop it with Ctrl-C and confirm the `shutting down` line appears.

**Path three, no Docker daemon — and this one exits.** `dockerbe.New` pings the daemon inside `New` and returns `backend.ErrUnavailable` (`docker.go:127-134`), with a doc comment that says so deliberately: "A Docker Desktop that is installed but not running must fail here, not later with a nil dereference." `serve` calls `newBackend` first, so nothing binds and nothing recovers. That is landed, intentional task 5 behaviour, **not** a wiring defect and not something to change here. Reproduce it without stopping Docker Desktop by pointing the client at a port nothing answers on:

```bash
PANACEA_APP_DOCKER_HOST=tcp://127.0.0.1:1 /tmp/panacea-api.exe; echo "exit=$?"
```

```
time=2026-09-22T11:39:11.316+10:00 level=ERROR msg="control plane stopped" err="dockerbe: no Docker daemon answered — start Docker Desktop, or set PANACEA_APP_DOCKER_HOST: error during connect: Head \"http://127.0.0.1:1/_ping\": dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.: backend: backend is unavailable"
exit=1
```

The reaper's `backend is not answering; session statuses are left exactly as they are until it does` warning is the *other* case: a daemon that dies while this process is already running. That one is a warning and the process stays up, which is the whole point of rule 1.

- [ ] **Step 13: Commit**

Nothing outside these six files changed, and `control-plane/go.mod` and `go.sum` are byte-identical to what you started with. Verify that before you stage anything — `git diff --stat control-plane/go.mod control-plane/go.sum` must print nothing. Every import this task adds is either internal to the module or already a direct requirement: `k8s.io/client-go/kubernetes` and `.../rest` for the Kata path, and `github.com/coder/websocket` for the test fakes' `Attach` signature.

The one import that would have broken this is `k8s.io/client-go/tools/clientcmd`, and step 11 does not use it — see `kubeClient`. It pulls `github.com/imdario/mergo`, which is in neither file, so adding it makes every `GOPROXY=off` command in step 12 fail with `go: updates to go.mod needed; to update it: go mod tidy` before a single test runs. Group C ruling 8: no new module enters `go.mod` in this task. So if `go.mod` moved, something ran `go mod tidy` or built with `-mod=mod`; revert both files and find out which import did it, because contract §8.5 makes an unpinned resolution a real risk on this network.

Do not stage `control-plane/api.exe` or anything under `/tmp` from step 12.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && git add \
  control-plane/cmd/api/config.go \
  control-plane/cmd/api/config_test.go \
  control-plane/cmd/api/main.go \
  control-plane/cmd/api/main_test.go \
  control-plane/internal/reaper/reaper.go \
  control-plane/internal/reaper/reaper_test.go
```

Write the message to `/tmp/task13-msg.txt` **with your editor**, not with a heredoc: a multi-line heredoc can terminate early when the command crosses an extra shell boundary, and the symptom is a commit whose message stops at the first blank line while the rest of your text runs as shell commands. The file's contents are exactly this, and `git commit -F` reads it in the next block:

```
feat(control-plane): config, reaper, wiring and restart recovery

cmd/api/config.go parses every PANACEA_APP_* variable once, refuses a
non-loopback bind with exit 2, clamps the reap interval and collects the
warnings the logger cannot emit until its own level is known. It adds one
row to contract section 4's table: PANACEA_APP_READY_TIMEOUT, default 60s,
passed into dockerbe.Config.ReadyTimeout, which task 5 recorded as a
cross-task dependency and without which that field is test-only.

internal/reaper implements section 5.3's sweep, in order, over one snapshot,
plus the rule task 5's amendment requires: a sandbox reported with
NonLoopback is destroyed first and logged distinctly, so "why did my sandbox
vanish" has an answer. A failed backend.List changes nothing at all.

cmd/api/main.go wires the ten packages, runs section 5.5's recovery before
the listener - re-reading the endpoint, because a restarted daemon
republishes on a different host port - and shuts down gracefully. The
server sets ReadHeaderTimeout only: a ReadTimeout or WriteTimeout would kill
/attach.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

Confirm the file survived whole before committing — its last line must be the `Co-Authored-By:` trailer, and if it is not, the heredoc trap above is exactly what happened:

```bash
tail -1 /tmp/task13-msg.txt && cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp && git commit -F /tmp/task13-msg.txt
```

