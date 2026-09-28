### Task 13: OD-23 mechanism, part 1 — layers, merge, invariants, digest, minimal defaults

**Slice:** TM1-α. **Focused time:** 3 hours. **dependsOn:** none (task 09's `model.Class` is a
type name only; P6-14's fragment is read as input once `feat/phase-6-gateway` is merged).
**Seams:** defines **ST.4** (the layers, `policy.Render`, the digest, `PANACEA_APP_POLICY_SHA256`),
consumed by tasks 14, 15 and 41; consumes **S6.6** (phase 6's managed-settings fragment).

**The mechanism, not the policy.** OD-23, in the owner's words: *"For the time being, don't draft
too much policy. Just give the control plane a way to put our policies in"*
(`docs/superpowers/planning/2026-09-23-owner-decisions.md:151`). So this task builds the way in —
layers, a merge, a digest — and ships defaults that hold **only** what the product itself needs to
work: the `panacea` channel, the gateway key helper, managed-only hooks and rules, and the
worker's first-edit check. Every rule an organisation actually wants (its 144 allow rules, its 49
ask rules, `06` C3's inventory of the tenant file) arrives as the org's layer through task 14's
route, and none is written here. OD-23 supersedes the design's product file *"derived from the
tenant's live file and submitted to KPMG InfoSec"* (`05-design.md` §2.5 lines 241-248, Q12); the
tenant file is an input a human may upload as the org layer, not a build step.

**What reaches Claude Code is a file it already reads, and nothing else** (global constraint 10,
OD-8). The rendered bytes are Claude Code managed settings, written by task 15 to the Linux managed
path `/etc/claude-code/managed-settings.json` before the harness starts. Phase 6's part read the
docs for that path and for its merge rules on 2026-09-25 (`phase-6 …/task-14.md:11`: the file, plus
an optional `managed-settings.d/` whose `*.json` files merge after it in alphabetical order;
*"single values are replaced by the later file, lists combine, and nested blocks such as `env`
merge key by key"*). No component wraps, patches or parses the CLI.

**Layers merge in a fixed order, and the class invariants are applied after all of them.**

1. the embedded **default** for the class (`defaults/<class>.json`);
2. phase 6's **gateway fragment** (S6.6);
3. the **org** layer, the org's current version (task 14);
4. the **project** layer, the project's current version (task 14);
5. the **invariants** of the class, which no layer can remove.

The merge is Claude Code's own rule, so the rendered file means what a reader of the layers
expects: objects merge key by key, arrays combine (order kept, duplicates dropped), a scalar in a
later layer replaces the earlier one. A `null` anywhere in a layer is refused: it is how JSON would
spell "delete", and deletion is exactly what a layer may not do. Because arrays only grow, no layer
can drop `Bash(panacea *)` from `permissions.allow` by leaving it out; the ways a layer *could*
defeat an invariant are to override a scalar, to add a `deny` or `ask` rule that blocks the
channel, or to add a forbidden key, and each of those is an `*InvariantError` naming the invariant,
the class and the layer that broke it.

**A layer is per class, because the classes differ.** An org that adds `Bash(npm test)` for its
workers must not thereby break the Orchestrator, whose allowed tools are exactly `Bash(panacea *)`
and `Read`. So a layer document is an envelope: `{"v":1,"all":{…},"worker":{…},"orchestrator":{…},
"mirror":{…}}`, each section a Claude Code settings object, `all` merged before the class's own
section. This is the "per-class policy files" the brief asks of phase 8: one rendered file per
class, from one layer document per scope.

**The two layers are not equally trusted, because their authors are not.** The org layer is written
by an **org admin** (task 14, `VerbOrgPolicy`), the role already trusted with every sandbox in the
org, so it stays a full Claude Code settings object. The project layer is written by a **team
admin** (`task-12.md:315,339`), a role a project owner hands out — and a full settings object there
is code execution in every *other* member's pods: `hooks` and `statusLine.command` are command
lines, `allowManagedHooksOnly` (I3) makes each hook in this file a managed hook and therefore one
that runs, `apiKeyHelper`, `otelHeadersHelper`, `awsAuthRefresh` and `awsCredentialExport` are
command lines too, and `env` keys such as `BASH_ENV`, `NODE_OPTIONS` and `NODE_EXTRA_CA_CERTS`
reach the harness process. The Orchestrator's pod is the worst of them: its control socket stamps
its owner's session (task 16), so a command that runs there calls `panacea peer`, `direct` or
`human` **as that member** — exactly what OD-16 ("no user steers another's agent") and §3.3 rule 1
forbid, with none of task 12's role checks touched. So the project layer is a **tighten-only
allowlist** (I14), and no layer of either scope may hand the orchestrator class a command-bearing
key (I15). An org admin who wants a hook for the workers still has one: their own layer, audited as
theirs. The refusal is an `*InvariantError`, not a bare `ErrLayer`, so task 14's route answers 422
with `details.invariant` and the key an admin must remove (`task-14.md:964`).

**The gateway fragment is copied and pinned, not imported.** It lives in
`images/sandbox/managed-settings.gateway.json` (phase 6 task 14, **planned**, `phase-6-gateway.md:248`,
**UNVERIFIED until merge**), outside the `control-plane` module, so `go:embed` cannot reach it. The
control plane keeps a byte-identical copy at `internal/policy/defaults/gateway.json`, and a test
reads the image's file by relative path (as phase 6's own test does, `phase-6 …/task-14.md:200`)
and fails if the two differ. Phase 6 **also** installs the fragment as the drop-in
`/etc/claude-code/managed-settings.d/50-panacea-gateway.json` (`phase-6 …/task-14.md:16`); rendering
it into the managed file as well is harmless, because the drop-in's values equal the rendered ones
and Claude Code merges them key by key. Task 15 must not shadow that drop-in (its mount is the file,
not the directory).

**The digest is of the exact bytes Claude Code reads.** `Render` produces canonical JSON (keys
sorted at every depth, no insignificant whitespace, numbers kept verbatim, one trailing newline),
and the digest is `sha256:` + the lower-case hex SHA-256 of those bytes, the form task 06's agent
start already carries (`task-06.md:166`). Task 15 mounts those bytes unchanged and passes the
digest as `PANACEA_APP_POLICY_SHA256`; `sandboxd` recomputes it over the mounted file. Rendering is
deterministic: the same layers give the same bytes on every replica and every run.

**The invariants, per class.** `W` worker, `O` orchestrator, `M` mirror. The mirror pod runs no
agent (task 58's `panacea-mirror` is a plain program), so its file only locks the door.

| Id | Invariant | W | O | M | Source |
|---|---|---|---|---|---|
| I1 | `permissions.allow` contains `Bash(panacea *)` | ✓ | ✓ | — | skeleton 13; §5.2 item 1 |
| I2 | `apiKeyHelper` is `/usr/local/bin/panacea token`, phase 6's value (`phase-6 …/task-14.md:223`) | ✓ | ✓ | absent | global constraint 9; S6.6 |
| I3 | `allowManagedHooksOnly` is `true` | ✓ | ✓ | ✓ | §7.3 line 837 |
| I4 | `allowManagedPermissionRulesOnly` is `true` | ✓ | ✓ | ✓ | §7.3 line 837 |
| I5 | `hooks.PreToolUse` holds matcher `Edit\|Write\|MultiEdit` running `/usr/local/bin/panacea check --hook` | ✓ | — | — | §4.3 line 455; task 18's spelling (`task-18.md:359-362`) |
| I6 | `env.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` is `"1"` | ✓ | ✓ | ✓ | §2.5 line 248 |
| I7 | `permissions.defaultMode` is pinned: W `default`, O `dontAsk`, M `default` | ✓ | ✓ | ✓ | §2.5 line 245: *"so a headless Orchestrator never needs a prompt"* |
| I8 | the Orchestrator's `permissions.allow` is a subset of `{Bash(panacea *), Read}` and its `permissions.ask` is empty | — | ✓ | — | skeleton 13 |
| I9 | `forceLoginMethod` and `forceLoginOrgUUID` are absent | ✓ | ✓ | ✓ | either one blocks `apiKeyHelper` at startup (`phase-6 …/task-14.md:10`) |
| I10 | no `permissions.deny` or `permissions.ask` rule would match a `panacea` command | ✓ | ✓ | — | I1 would be dead otherwise: a deny outranks an allow |
| I11 | `disableAllHooks` is absent or `false` | ✓ | ✓ | ✓ | I5 would be dead otherwise |
| I12 | no **org or project** layer sets `env.ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `CLAUDE_CODE_USE_BEDROCK` or `CLAUDE_CODE_API_KEY_HELPER_TTL_MS` | ✓ | ✓ | ✓ | the first two outrank `apiKeyHelper` (`phase-6 …/task-14.md:9`); the route and the TTL are phase 6's (launch environment, and the fragment) |
| I14 | a **project** layer sets only allowlisted keys — `permissions.deny`, `permissions.ask`, and the closed list `cleanupPeriodDays`, `disableBypassPermissionsMode`, `includeCoAuthoredBy` — and any other key is refused, naming it | ✓ | ✓ | ✓ | its author is a **team** admin (`task-12.md:315,339`), not every member's root: OD-16 |
| I15 | no layer of either scope gives the **orchestrator** class a command-bearing key: `hooks`, `statusLine`, `mcpServers`, `enabledMcpjsonServers`, `aws*`, any `*Helper`, or an `env` key outside `orchestratorEnv` | — | ✓ | — | its socket stamps its owner's session (task 16), so a command there speaks as that member: OD-16, §3.3 rule 1 |

**I13 is task 41's** (`claudeMd`, `task-41.md:18,289`); this task leaves the id free and numbers its
two new invariants I14 and I15.

**I7's `dontAsk` is UNVERIFIED.** It is the mode that refuses, rather than prompts for, any tool
not pre-approved, which is what a headless turn needs; whether the pinned CLI version accepts the
value is not checked here, and task 33's `/status` check settles it. If it does not, the
Orchestrator's pin becomes `default` and I8 is what keeps a turn from stalling: an Orchestrator
whose only allowed tools are pre-approved never meets a prompt. **Which `env` keys a managed file
can lock is also UNVERIFIED** (§7.3; skeleton 13): I6 and I12 hold in the rendered file, and whether
a user-level or project-level `env` can still override them inside the sandbox is task 33's check.

**Files:**
- `control-plane/internal/policy/policy.go` — `Layer`, `Layers`, `Render`, `Default`, `ParseLayer`, the invariants, `InvariantError` (new)
- `control-plane/internal/policy/merge.go` — `merge`, the no-`null` rule (new)
- `control-plane/internal/policy/digest.go` — `canonical`, `Digest`, `EnvDigest`, `ManagedPath` (new)
- `control-plane/internal/policy/defaults/{worker,orchestrator,mirror}.json` — the minimal defaults, embedded (new)
- `control-plane/internal/policy/defaults/gateway.json` — the pinned copy of phase 6's fragment (new)
- `control-plane/internal/policy/policy_test.go`, `merge_test.go`, `digest_test.go` — one test per invariant with its mutant; determinism; the pin (new)

**Interfaces:**
- Produces (ST.4):
  - `policy.Render(layers Layers, class model.Class) (file []byte, digest string, err error)` — the skeleton's signature, unchanged
  - `policy.Layers{Gateway []byte; Org, Project Layer}`, `policy.Layer{Scope Scope; Version string; Doc []byte}`, `policy.Scope` (`default`, `gateway`, `org`, `project`), `policy.ParseLayer(doc []byte) (map[string]map[string]any, error)`
  - `policy.Default(class) []byte`, `policy.GatewayFragment() []byte`
  - `policy.Digest(file []byte) string` (`sha256:<hex>`), `policy.EnvDigest = "PANACEA_APP_POLICY_SHA256"`, `policy.ManagedPath = "/etc/claude-code/managed-settings.json"`, `policy.MaxLayerBytes` (64 KiB)
  - `*policy.InvariantError{Invariant, Class, Scope, Detail}`, `policy.ErrLayer`
- Consumes:
  - task 09 — `model.Class` and `model.ClassWorker`, `model.ClassOrchestrator`, `model.ClassMirror` (**assumed** spellings; task 09 names them)
  - phase 6 task 14 (planned) — the fragment's bytes, `images/sandbox/managed-settings.gateway.json`
  - task 18 — the hook command `panacea check --hook` (`task-18.md:62-67`)
- Consumed by: task 14 (`Render` on every `POST`, for every class), task 15 (the file and the digest into the pod; `sandboxd`'s check reimplements `Digest` in its own module), task 41 (the Orchestrator's instructions assume I8).

**`control-plane/internal/policy/digest.go`, complete:**

```go
package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	// ManagedPath is where Claude Code reads managed settings on Linux
	// (phase-6 …/task-14.md:11). Task 15 mounts the rendered file here.
	ManagedPath = "/etc/claude-code/managed-settings.json"
	// EnvDigest carries Digest(file) into the pod; sandboxd checks it (task 15).
	EnvDigest = "PANACEA_APP_POLICY_SHA256"
)

// Digest is "sha256:" + lower-case hex of SHA-256(file), task 06's form.
func Digest(file []byte) string {
	sum := sha256.Sum256(file)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// canonical encodes v with keys sorted at every depth (encoding/json sorts
// map keys), numbers verbatim (v was decoded with UseNumber), no HTML
// escaping, no indentation, and exactly one trailing newline (Encode's).
func canonical(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decode reads one JSON value with numbers kept as json.Number, refusing
// trailing data, so canonical(decode(x)) is stable for every valid x.
func decode(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errTrailing
	}
	return v, nil
}
```

**`control-plane/internal/policy/merge.go`, complete:**

```go
package policy

import (
	"errors"
	"fmt"
)

var errTrailing = errors.New("policy: trailing data after the JSON value")

// merge is Claude Code's own drop-in rule (phase-6 …/task-14.md:11): objects
// merge key by key, arrays combine keeping order and dropping duplicates, and
// anything else in src replaces dst. It never mutates src; it may reuse dst.
func merge(dst, src any) any {
	switch s := src.(type) {
	case map[string]any:
		d, ok := dst.(map[string]any)
		if !ok {
			d = map[string]any{}
		}
		for k, sv := range s {
			d[k] = merge(d[k], sv)
		}
		return d
	case []any:
		d, _ := dst.([]any)
		out := append([]any(nil), d...)
		seen := map[string]bool{}
		for _, x := range out {
			seen[key(x)] = true
		}
		for _, x := range s {
			if k := key(x); !seen[k] {
				seen[k] = true
				out = append(out, merge(nil, x))
			}
		}
		return out
	default:
		return src // a scalar: string, bool or json.Number
	}
}

// key is an array element's identity for de-duplication: its canonical bytes.
func key(x any) string {
	b, err := canonical(x)
	if err != nil {
		return fmt.Sprintf("%#v", x)
	}
	return string(b)
}

// noNull refuses a null anywhere in v, naming the path: null is how JSON
// would spell "delete", and a layer may not delete.
func noNull(v any, path string) error {
	switch t := v.(type) {
	case nil:
		return fmt.Errorf("%w: %s is null; a layer adds or tightens, it cannot delete", ErrLayer, path)
	case map[string]any:
		for k, x := range t {
			if err := noNull(x, path+"."+k); err != nil {
				return err
			}
		}
	case []any:
		for i, x := range t {
			if err := noNull(x, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
```

`merge(nil, x)` copies an array element, so a later merge into the result can never reach back into
a layer's parsed document; `TestMergeNeverMutatesALayer` pins it.

**`control-plane/internal/policy/policy.go`, complete (three blocks, one file):**

```go
// Package policy is OD-23's mechanism: per-class Claude Code managed
// settings, rendered from embedded minimal defaults, phase 6's gateway
// fragment, and the org's and project's versioned layers, with class
// invariants no layer can remove. It drafts no policy.
package policy

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

//go:embed defaults/*.json
var defaults embed.FS

// MaxLayerBytes bounds one layer document. KPMG's tenant file, the largest
// real input known (193 rules, 05-design.md §2.5), is well under it.
const MaxLayerBytes = 64 << 10

var ErrLayer = errors.New("policy: layer refused")

type Scope string

const (
	ScopeDefault Scope = "default"
	ScopeGateway Scope = "gateway"
	ScopeOrg     Scope = "org"
	ScopeProject Scope = "project"
)

// Layer is one scope's current version. An empty Doc is "no layer yet".
type Layer struct {
	Scope   Scope
	Version string // pv-…, for the error text only
	Doc     []byte // the envelope {"v":1,"all":{…},"worker":{…},…}
}

// Layers is Render's input. A nil Gateway means GatewayFragment().
type Layers struct {
	Gateway []byte
	Org     Layer
	Project Layer
}

// InvariantError names what a layer tried to remove.
type InvariantError struct {
	Invariant string // "I1".."I12"
	Class     model.Class
	Scope     Scope
	Detail    string
}

func (e *InvariantError) Error() string {
	return fmt.Sprintf("policy: %s layer breaks %s for the %s class: %s", e.Scope, e.Invariant, e.Class, e.Detail)
}

var sections = []string{"all", string(model.ClassWorker), string(model.ClassOrchestrator), string(model.ClassMirror)}

// ParseLayer refuses a layer document that is not the v1 envelope: an
// object, at most MaxLayerBytes, "v":1, only the four section keys, each an
// object, and no null anywhere. It does not render; Render calls it.
func ParseLayer(doc []byte) (map[string]map[string]any, error) {
	if len(doc) > MaxLayerBytes {
		return nil, fmt.Errorf("%w: %d bytes, the limit is %d", ErrLayer, len(doc), MaxLayerBytes)
	}
	v, err := decode(doc)
	if err != nil {
		return nil, fmt.Errorf("%w: not JSON: %v", ErrLayer, err)
	}
	env, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: the document is not an object", ErrLayer)
	}
	if err := noNull(env, "$"); err != nil {
		return nil, err
	}
	if n, _ := env["v"].(json.Number); n.String() != "1" {
		return nil, fmt.Errorf("%w: \"v\" must be 1", ErrLayer)
	}
	out := map[string]map[string]any{}
	for k, x := range env {
		if k == "v" {
			continue
		}
		if !slices.Contains(sections, k) {
			return nil, fmt.Errorf("%w: unknown section %q; sections are %v", ErrLayer, k, sections)
		}
		obj, ok := x.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: section %q is not an object", ErrLayer, k)
		}
		out[k] = obj
	}
	return out, nil
}
```

```go
const (
	PanaceaRule  = "Bash(panacea *)"
	KeyHelper    = "/usr/local/bin/panacea token"       // phase-6 …/task-14.md:223
	CheckHook    = "/usr/local/bin/panacea check --hook" // task-18.md:62-67
	CheckMatcher = "Edit|Write|MultiEdit"
)

// pinnedMode is I7. dontAsk is UNVERIFIED at the pinned CLI (task 33).
var pinnedMode = map[model.Class]string{
	model.ClassWorker: "default", model.ClassOrchestrator: "dontAsk", model.ClassMirror: "default",
}

// defaultsFS is the package's one test seam (TestI5 swaps it).
var defaultsFS fs.FS = defaults

// Default is the embedded minimal default for class, or nil for an unknown class.
func Default(class model.Class) []byte {
	b, err := fs.ReadFile(defaultsFS, "defaults/"+string(class)+".json")
	if err != nil {
		return nil
	}
	return b
}

// GatewayFragment is the pinned copy of phase 6's fragment (S6.6).
func GatewayFragment() []byte {
	b, _ := fs.ReadFile(defaultsFS, "defaults/gateway.json")
	return b
}

// Render is ST.4. The invariants are checked after the defaults and after
// each layer, so the error names the layer that broke one.
func Render(layers Layers, class model.Class) (file []byte, digest string, err error) {
	base := Default(class)
	if base == nil {
		return nil, "", fmt.Errorf("%w: unknown class %q", ErrLayer, class)
	}
	doc, err := decode(base)
	if err != nil {
		return nil, "", fmt.Errorf("policy: the embedded %s default: %w", class, err)
	}
	if class != model.ClassMirror { // the mirror holds no gateway token (task 58)
		gw := layers.Gateway
		if gw == nil {
			gw = GatewayFragment()
		}
		g, err := decode(gw)
		if err != nil {
			return nil, "", fmt.Errorf("policy: the gateway fragment: %w", err)
		}
		doc = merge(doc, g)
	}
	if err := check(class, doc, ScopeGateway); err != nil {
		return nil, "", err // a defect in the defaults or the fragment, not in anyone's layer
	}
	for _, l := range []Layer{layers.Org, layers.Project} {
		if len(l.Doc) == 0 {
			continue
		}
		secs, err := ParseLayer(l.Doc)
		if err != nil {
			return nil, "", fmt.Errorf("%s layer %s: %w", l.Scope, l.Version, err)
		}
		for _, name := range []string{"all", string(class)} {
			s, ok := secs[name]
			if !ok {
				continue
			}
			if err := forbiddenEnv(class, s, l.Scope); err != nil {
				return nil, "", err
			}
			if err := layerKeys(class, s, l.Scope); err != nil {
				return nil, "", err
			}
			doc = merge(doc, s)
		}
		if err := check(class, doc, l.Scope); err != nil {
			return nil, "", err
		}
	}
	if file, err = canonical(doc); err != nil {
		return nil, "", err
	}
	return file, Digest(file), nil
}

// forbiddenEnv is I12: these belong to phase 6 (the fragment, the launch
// environment), and the first two outrank apiKeyHelper.
func forbiddenEnv(class model.Class, section map[string]any, scope Scope) error {
	env, _ := section["env"].(map[string]any)
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
		"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS"} {
		if _, has := env[k]; has {
			return &InvariantError{Invariant: "I12", Class: class, Scope: scope, Detail: "env." + k + " is set by the platform, never by a layer"}
		}
	}
	return nil
}
```

```go
// projectKeys is I14's closed list: every key a project layer may set, in
// any section, for any class. Sorted, because it is printed in the refusal.
// Widening it is an owner decision, not a patch: each addition hands a team
// admin a setting in every other member's pods.
var projectKeys = []string{"cleanupPeriodDays", "disableBypassPermissionsMode", "includeCoAuthoredBy", "permissions"}

// projectPermissionKeys is the tighten-only half: a project layer adds
// rules that refuse or prompt, never one that allows, and never the mode.
var projectPermissionKeys = []string{"ask", "deny"}

// orchestratorEnv is the only env key any layer may name for the
// orchestrator class; every other one reaches the harness process.
var orchestratorEnv = []string{"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"}

// layerKeys is I14 and I15, applied to one section of one layer before that
// section is merged, so the refusal names the key its author must remove. It
// runs after forbiddenEnv, so a platform env key keeps its own error (I12).
func layerKeys(class model.Class, section map[string]any, scope Scope) error {
	for k, v := range section {
		if scope == ScopeProject {
			if !slices.Contains(projectKeys, k) {
				return &InvariantError{Invariant: "I14", Class: class, Scope: scope,
					Detail: k + " is not a key a project layer may set; a team admin may set only " + strings.Join(projectKeys, ", ")}
			}
			if k == "permissions" {
				sub, _ := v.(map[string]any)
				for pk := range sub {
					if !slices.Contains(projectPermissionKeys, pk) {
						return &InvariantError{Invariant: "I14", Class: class, Scope: scope,
							Detail: "permissions." + pk + " is not a key a project layer may set; it may tighten with " + strings.Join(projectPermissionKeys, " and ")}
					}
				}
			}
		}
		if class == model.ClassOrchestrator && commandBearing(k) {
			return &InvariantError{Invariant: "I15", Class: class, Scope: scope,
				Detail: k + " carries a command, and no layer may run one in another member's Orchestrator"}
		}
	}
	if class == model.ClassOrchestrator {
		env, _ := section["env"].(map[string]any)
		for k := range env {
			if !slices.Contains(orchestratorEnv, k) {
				return &InvariantError{Invariant: "I15", Class: class, Scope: scope,
					Detail: "env." + k + " reaches the Orchestrator's process; a layer may name only " + strings.Join(orchestratorEnv, ", ")}
			}
		}
	}
	return nil
}

// commandBearing is I15's key set: each of these is, or holds, a command
// line the harness runs (§7.3 line 837; phase-6 …/task-14.md:9-11). The
// suffix and prefix rules are deliberate: a key the pinned CLI version adds
// later, named like these, is refused before anyone reads its docs.
func commandBearing(k string) bool {
	switch k {
	case "hooks", "statusLine", "mcpServers", "enabledMcpjsonServers":
		return true
	}
	return strings.HasSuffix(k, "Helper") || strings.HasPrefix(k, "aws")
}
```

```go
// check applies I1-I11 to the merged document. The order is the table's.
func check(class model.Class, doc any, scope Scope) error {
	fail := func(inv, detail string) error {
		return &InvariantError{Invariant: inv, Class: class, Scope: scope, Detail: detail}
	}
	root, _ := doc.(map[string]any)
	perms, _ := root["permissions"].(map[string]any)
	allow, deny, ask := strs(perms["allow"]), strs(perms["deny"]), strs(perms["ask"])
	env, _ := root["env"].(map[string]any)
	if class != model.ClassMirror {
		if !slices.Contains(allow, PanaceaRule) {
			return fail("I1", `permissions.allow must contain "`+PanaceaRule+`"`)
		}
		if root["apiKeyHelper"] != KeyHelper {
			return fail("I2", `apiKeyHelper must be "`+KeyHelper+`"`)
		}
	} else if _, has := root["apiKeyHelper"]; has {
		return fail("I2", "the mirror holds no gateway token, so apiKeyHelper must be absent")
	}
	if root["allowManagedHooksOnly"] != true {
		return fail("I3", "allowManagedHooksOnly must be true")
	}
	if root["allowManagedPermissionRulesOnly"] != true {
		return fail("I4", "allowManagedPermissionRulesOnly must be true")
	}
	if class == model.ClassWorker && !hasCheckHook(root) {
		return fail("I5", "hooks.PreToolUse must run "+CheckHook+" on "+CheckMatcher)
	}
	if env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" {
		return fail("I6", `env.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC must be "1"`)
	}
	if perms["defaultMode"] != pinnedMode[class] {
		return fail("I7", fmt.Sprintf("permissions.defaultMode is pinned to %q for this class", pinnedMode[class]))
	}
	if class == model.ClassOrchestrator {
		for _, r := range allow {
			if r != PanaceaRule && r != "Read" {
				return fail("I8", fmt.Sprintf("the Orchestrator may be allowed only %s and Read, not %q", PanaceaRule, r))
			}
		}
		if len(ask) > 0 {
			return fail("I8", "the Orchestrator runs headless: permissions.ask must be empty")
		}
	}
	for _, k := range []string{"forceLoginMethod", "forceLoginOrgUUID"} {
		if _, has := root[k]; has {
			return fail("I9", k+" blocks apiKeyHelper at startup and must be absent")
		}
	}
	if class != model.ClassMirror {
		for _, r := range append(slices.Clone(deny), ask...) {
			if blocksPanacea(r) {
				return fail("I10", fmt.Sprintf("rule %q would block the panacea channel", r))
			}
		}
	}
	if v, has := root["disableAllHooks"]; has && v != false {
		return fail("I11", "disableAllHooks would disable the managed check hook")
	}
	return nil
}

func strs(v any) []string {
	xs, _ := v.([]any)
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func hasCheckHook(root map[string]any) bool {
	hooks, _ := root["hooks"].(map[string]any)
	entries, _ := hooks["PreToolUse"].([]any)
	for _, e := range entries {
		m, _ := e.(map[string]any)
		if m["matcher"] != CheckMatcher {
			continue
		}
		hs, _ := m["hooks"].([]any)
		for _, h := range hs {
			hm, _ := h.(map[string]any)
			if hm["type"] == "command" && hm["command"] == CheckHook {
				return true
			}
		}
	}
	return false
}

// probes are the command lines the product itself runs through Bash.
var probes = []string{"panacea team", "panacea check src/a.ts", "panacea check --hook", "panacea token"}

// blocksPanacea is I10, conservative: a Bash rule with no pattern, a
// "prefix:*" rule whose prefix any probe starts with, or a "*" wildcard
// pattern matching any probe, blocks the channel. It errs towards refusing
// a layer: a false positive costs an admin a reworded rule; a false negative
// costs every agent its channel.
func blocksPanacea(rule string) bool {
	tool, pat, hasPat := strings.Cut(strings.TrimSpace(rule), "(")
	if tool = strings.TrimSpace(tool); tool != "Bash" {
		return false
	}
	if !hasPat {
		return true
	}
	pat = strings.TrimSuffix(pat, ")")
	for _, p := range probes {
		if prefix, ok := strings.CutSuffix(pat, ":*"); ok {
			if strings.HasPrefix(p, prefix) {
				return true
			}
		} else if wild(pat, p) {
			return true
		}
	}
	return false
}

// wild matches s against pat where '*' is any run of characters, empty included.
func wild(pat, s string) bool {
	parts := strings.Split(pat, "*")
	if len(parts) == 1 {
		return pat == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(s, mid)
		if i < 0 {
			return false
		}
		s = s[i+len(mid):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}
```

**The defaults, complete.** Each is its class's invariants and nothing else. `permissions.deny`
and `ask` are absent, not empty, so an org layer's lists combine with nothing.

`control-plane/internal/policy/defaults/worker.json`:

```json
{
  "allowManagedHooksOnly": true,
  "allowManagedPermissionRulesOnly": true,
  "env": { "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1" },
  "hooks": {
    "PreToolUse": [
      { "matcher": "Edit|Write|MultiEdit",
        "hooks": [ { "type": "command", "command": "/usr/local/bin/panacea check --hook" } ] }
    ]
  },
  "permissions": { "allow": [ "Bash(panacea *)" ], "defaultMode": "default" }
}
```

`control-plane/internal/policy/defaults/orchestrator.json`:

```json
{
  "allowManagedHooksOnly": true,
  "allowManagedPermissionRulesOnly": true,
  "env": { "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1" },
  "permissions": { "allow": [ "Bash(panacea *)", "Read" ], "defaultMode": "dontAsk" }
}
```

`control-plane/internal/policy/defaults/mirror.json`:

```json
{
  "allowManagedHooksOnly": true,
  "allowManagedPermissionRulesOnly": true,
  "env": { "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1" },
  "permissions": { "defaultMode": "default" }
}
```

`control-plane/internal/policy/defaults/gateway.json` is phase 6's fragment **byte for byte**
(`phase-6 …/task-14.md:395-402`, as that part prints it on 2026-09-25):

```json
{
  "apiKeyHelper": "/usr/local/bin/panacea token",
  "env": {
    "CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "120000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
```

Phase 6's plan row says `"300000"` for the TTL (`phase-6-gateway.md:248`) and its part says
`"120000"`; the part is the later text and is what this copy follows. `TestGatewayCopyIsPinned`
settles it against the image's real file once phase 6 merges: whichever value is there, the copy
must equal it.

- [ ] **Step 1: Write the failing tests**

`control-plane/internal/policy/policy_test.go`, package `policy`. Every invariant test builds one
layer that tries to break it and asserts `errors.As(err, &ie)` with `ie.Invariant` equal to the
id, `ie.Scope` equal to the layer's scope and `ie.Class` equal to the class rendered — the error
must **name** what broke, or an admin cannot fix their layer. A helper `layer(t, scope, json)`
returns a `Layer`; `render(t, class, org, project)` calls `Render` with the pinned fragment.

| Test | The layer (or input) | Expected | The mutant it kills |
|---|---|---|---|
| `TestI1ADenyCannotRemoveThePanaceaRule` | project `{"v":1,"all":{"permissions":{"deny":["Bash(panacea *)"]}}}` | I10, project, worker | the skeleton's own example: a project layer that removes `Bash(panacea *)`; with arrays combining, this deny is the only way to try |
| `TestI1IsPresentInEveryAgentClass` | none | the worker and orchestrator files contain `Bash(panacea *)` in `permissions.allow` | a default file edited to drop it (I1 would then fail at `ScopeGateway`) |
| `TestI2AKeyHelperCannotBeReplaced` | org `{"v":1,"all":{"apiKeyHelper":"/bin/echo sk-ant"}}`, rendered for the worker (for the orchestrator the same layer is I15, and for a project layer it is I14) | I2, org | scalar replacement letting a static key in |
| `TestI2TheMirrorHasNoKeyHelper` | org `{"v":1,"mirror":{"apiKeyHelper":"/usr/local/bin/panacea token"}}` | I2, org, mirror | the gateway fragment merged into the mirror |
| `TestI3AndI4CannotBeTurnedOff` | org `{"v":1,"all":{"allowManagedHooksOnly":false}}`; then `…PermissionRulesOnly":false` | I3; I4 | a boolean invariant checked for presence, not value |
| `TestI5TheWorkerKeepsItsCheckHook` | `defaultsFS` swapped for an `fstest.MapFS` whose `worker.json` runs `panacea check` without `--hook` | I5, gateway | the hook dropped from `worker.json`, or task 18's `--hook` spelling lost |
| `TestI6NonessentialTrafficStaysOff` | org `{"v":1,"all":{"env":{"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC":"0"}}}`, rendered for the worker (the org scope, because I14 refuses any `env` from a project layer before I6 is reached) | I6, org | `env` merged by replacement of the whole block, or the value not checked |
| `TestI7TheModeIsPinned` | org `{"v":1,"worker":{"permissions":{"defaultMode":"bypassPermissions"}}}` | I7, org, worker | a later layer's scalar winning over the pin |
| `TestI8TheOrchestratorGetsNoOtherTool` | org `{"v":1,"all":{"permissions":{"allow":["Bash(npm test)"]}}}` | the **worker** renders; the **orchestrator** fails I8 | one invariant list for every class |
| `TestI8TheOrchestratorGetsNoAskRule` | org `{"v":1,"orchestrator":{"permissions":{"ask":["Read"]}}}` | I8 | a headless turn that can stall on a prompt |
| `TestI9ForceLoginIsRefused` | org `{"v":1,"all":{"forceLoginMethod":"console"}}`; then `forceLoginOrgUUID` | I9 twice | the key that blocks `apiKeyHelper` at startup (`phase-6 …/task-14.md:10`) |
| `TestI10RuleFormsThatBlockTheChannel` | deny `Bash`, `Bash(*)`, `Bash(pan:*)`, `Bash(panacea check*)`, `Bash(*token)` each → I10; deny `Bash(rm:*)`, `Bash(curl *)`, `Read(./.env)` each → renders | as listed | a matcher that knows only the exact string |
| `TestI11HooksCannotBeDisabled` | org `{"v":1,"all":{"disableAllHooks":true}}` | I11 | the check hook silently switched off |
| `TestI12PlatformEnvIsRefused` | one render per key of I12 in a project `env` | I12, project, naming the key (I12 is checked before I14, so the platform key keeps its own error) | a layer overriding the gateway route or TTL, or planting a key that outranks the helper |
| `TestAProjectLayerCannotAddAHook` | project `{"v":1,"worker":{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/bin/sh -c id"}]}]}}}` | I14, project, worker, naming `hooks` | b05's hole: a team admin's hook running in every other member's worker, made managed by I3 |
| `TestNoLayerMayAddAHookToTheOrchestrator` | org `{"v":1,"all":{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/bin/sh -c id"}]}]}}}`, rendered for each class | the worker renders; the orchestrator fails I15, org, naming `hooks` | an org hook reaching the pod whose socket speaks as its owner (task 16) |
| `TestAProjectLayerCannotSetACommandHelper` | project `{"v":1,"all":{…}}` once per key: `otelHeadersHelper`, `apiKeyHelper`, `statusLine`, `awsCredentialExport`, `awsAuthRefresh`, `mcpServers`, `enabledMcpjsonServers`, `env` | I14, project, naming the key | an allowlist written as a deny list, which the next CLI release outgrows |
| `TestTheOrchestratorEnvIsAllowlisted` | org `{"v":1,"orchestrator":{"env":{"NODE_OPTIONS":"--require /tmp/x.js"}}}`; then `BASH_ENV`, `NODE_EXTRA_CA_CERTS` | I15, org, orchestrator, naming `env.<key>` | a code-execution env key checked only against I12's five |
| `TestAProjectLayerMayStillTighten` | project `{"v":1,"all":{"permissions":{"deny":["Bash(curl *)"]}},"worker":{"permissions":{"ask":["Bash(git push:*)"]}}}` | renders for all three classes; the worker file holds both rules | an allowlist so tight the project layer has no use left |
| `TestALayerMayAddAndTighten` | org: `permissions.deny ["Bash(curl *)"]`, `ask ["Bash(git push:*)"]`, `env {"FOO":"1"}` for `worker` | renders; the file has both rules, `FOO`, and every invariant | an over-eager check refusing a legitimate tightening |
| `TestTheProjectLayerComesAfterTheOrg` | org `permissions.deny ["Bash(curl *)"]`, project `permissions.deny ["Bash(wget *)"]`, worker (an `env` pair cannot show the order any more: I14 refuses a project `env`) | both rules are present and the project's is the later element of the array | the layer order reversed |
| `TestNullIsRefusedWithItsPath` | org `{"v":1,"all":{"permissions":{"deny":null}}}` | `ErrLayer`, message contains `$.all.permissions.deny` | null treated as "delete" |
| `TestTheEnvelopeIsStrict` | `{"all":{}}` (no `v`), `{"v":2}`, `{"v":1,"workers":{}}`, `[]`, 64 KiB + 1 | each `ErrLayer` | an org typo silently ignored |
| `TestRenderingIsDeterministic` | the same layers rendered 100 times, with the layer's keys written in two different orders | one digest | map iteration order leaking into the bytes |
| `TestTheFileIsCanonical` | render, then `decode` and `canonical` the output | byte-equal; ends with exactly one `\n`; no `<` for a rule containing `<` | indentation, HTML escaping, or a second newline |
| `TestTheDigestIsOfTheFile` | `Digest(file)` equals the returned digest and matches `^sha256:[0-9a-f]{64}$` | as stated | a digest of the layers instead of the bytes Claude Code reads |
| `TestTheMinimalDefaultsAreOnlyTheInvariants` | each default parsed; its keys | exactly the keys the table requires for the class, nothing more | a drafted rule creeping into a default (OD-23) |
| `TestGatewayCopyIsPinned` | reads `../../../images/sandbox/managed-settings.gateway.json` | byte-equal to `defaults/gateway.json`; if the file does not exist yet, `t.Skip("P6-14 not merged: …")` | the copy drifting from the image |
| `TestMergeNeverMutatesALayer` | merge a layer twice into two bases | the layer's parsed value is unchanged, deep-equal before and after | aliasing an array between layers |

**`TestI5`'s seam.** No layer can remove the hook (arrays combine), so I5 can only fail through a
bad default. The test swaps the embedded FS for one with an edited `worker.json` through an
unexported `var defaultsFS fs.FS = defaults` that `Default` reads; it is the only test seam in the
package.

Two tests, complete, because they carry the task's two claims — a layer cannot remove the channel,
and a per-class rule does not leak across classes:

```go
func TestI1ADenyCannotRemoveThePanaceaRule(t *testing.T) {
	proj := Layer{Scope: ScopeProject, Version: "pv-0000000000000001",
		Doc: []byte(`{"v":1,"all":{"permissions":{"deny":["Bash(panacea *)"]}}}`)}
	for _, class := range []model.Class{model.ClassWorker, model.ClassOrchestrator} {
		_, _, err := Render(Layers{Project: proj}, class)
		var ie *InvariantError
		if !errors.As(err, &ie) {
			t.Fatalf("%s: Render = %v; want an *InvariantError", class, err)
		}
		if ie.Invariant != "I10" || ie.Scope != ScopeProject || ie.Class != class {
			t.Fatalf("%s: got %s/%s/%s; want I10/project/%s", class, ie.Invariant, ie.Scope, ie.Class, class)
		}
		if !strings.Contains(ie.Error(), "Bash(panacea *)") {
			t.Fatalf("the error %q does not name the rule an admin must remove", ie.Error())
		}
	}
}

func TestI8TheOrchestratorGetsNoOtherTool(t *testing.T) {
	org := Layer{Scope: ScopeOrg, Version: "pv-0000000000000002",
		Doc: []byte(`{"v":1,"all":{"permissions":{"allow":["Bash(npm test)"]}}}`)}
	file, _, err := Render(Layers{Org: org}, model.ClassWorker)
	if err != nil {
		t.Fatalf("worker: %v; an org may add an allow rule for its workers", err)
	}
	if !bytes.Contains(file, []byte(`"Bash(npm test)"`)) || !bytes.Contains(file, []byte(`"Bash(panacea *)"`)) {
		t.Fatalf("worker file lacks a rule: %s", file)
	}
	_, _, err = Render(Layers{Org: org}, model.ClassOrchestrator)
	var ie *InvariantError
	if !errors.As(err, &ie) || ie.Invariant != "I8" || ie.Class != model.ClassOrchestrator {
		t.Fatalf("orchestrator: Render = %v; want I8 for the orchestrator class", err)
	}
	// The per-class way to say the same thing renders for both classes.
	scoped := Layer{Scope: ScopeOrg, Doc: []byte(`{"v":1,"worker":{"permissions":{"allow":["Bash(npm test)"]}}}`)}
	if _, _, err := Render(Layers{Org: scoped}, model.ClassOrchestrator); err != nil {
		t.Fatalf("a worker-only section reached the orchestrator: %v", err)
	}
}
```

Run: `cd control-plane && go test ./internal/policy/ -count=1`. Expected before step 2: `undefined:
Render` and the other names.

- [ ] **Step 2: Write `digest.go`, `merge.go`, `policy.go` and the four `defaults/*.json`** as
  above, then run step 1's command. Expected: `ok`, with `TestGatewayCopyIsPinned` skipped until
  phase 6 merges (`-v` shows the skip and its message naming P6-14). Commit:
  `feat(policy): OD-23's layers, merge, class invariants and digest; minimal defaults only`.

- [ ] **Step 3: Acceptance**

```
cd control-plane
go vet ./internal/policy/
go test ./internal/policy/ -count=1 -v 2>&1 | grep -E "^(--- FAIL|--- SKIP|ok|FAIL)"
go test ./internal/policy/ -run 'TestI[0-9]+' -count=1 -v 2>&1 | grep -c "^--- PASS"
go test ./internal/policy/ -run 'TestAProjectLayer|TestNoLayerMay|TestTheOrchestratorEnv' -count=1 -v 2>&1 | grep -c "^--- PASS"
```

Expected: `go vet` silent; `ok` and at most one `--- SKIP` (the pin, before phase 6 merges; none
after, and task 33 requires none); the invariant tests count **at least 12**, one or more per id;
the I14/I15 tests, whose names say what they defend rather than an id, count **at least 4**.
After phase 6 merges, re-run: the pin must pass, and if the TTL differs, change `gateway.json`,
never the test.

**What this task does not claim.**
- That Claude Code honours any of it. Where the file is read, whether `dontAsk` is accepted, whether
  `env` in a managed file can be overridden, and whether `allowManagedHooksOnly` is enforced at the
  pinned CLI version are all **UNVERIFIED** here and are task 33's `/status` checks on the real
  cluster.
- That the defaults are a policy. They are OD-23's minimal default: what the product needs to
  function, and nothing an InfoSec reviewer would call a control set. `disableBypassPermissionsMode`,
  the tenant's 144 allow and 49 ask rules, and any deny list are for the org layer, by a human,
  through task 14's route.
- That the **org** layer is constrained the way the project layer is. An org admin may still set
  `hooks`, a `*Helper` and any `env` key (outside I12's five) for the worker and mirror classes:
  they are the one role already trusted with every sandbox in the org, and task 14 stores the
  version under their name and audits it as theirs. I14 bounds the **team** admin; I15 bounds
  **both** scopes, and only for the orchestrator class. Narrowing the org layer too is an owner
  decision, and the seam for it is `layerKeys`: one more branch on `scope`.
- That I10's matcher is Claude Code's. It is a conservative approximation of the documented rule
  forms (exact, `prefix:*`, `*`); a rule form it does not know is not refused, and one it refuses
  may be one Claude Code would not match.
- That the gateway copy is current before phase 6 merges; the pin test proves it after.
- Storage, versions, routes and audit are task 14's; delivery into a pod and the digest check are
  task 15's.
