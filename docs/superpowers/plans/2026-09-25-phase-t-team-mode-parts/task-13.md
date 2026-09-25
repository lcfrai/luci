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
start already carries (`task-06.md:139`). Task 15 mounts those bytes unchanged and passes the
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

// Default is the embedded minimal default for class, or nil for an unknown class.
func Default(class model.Class) []byte {
	b, err := defaults.ReadFile("defaults/" + string(class) + ".json")
	if err != nil {
		return nil
	}
	return b
}

// GatewayFragment is the pinned copy of phase 6's fragment (S6.6).
func GatewayFragment() []byte {
	b, _ := defaults.ReadFile("defaults/gateway.json")
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
