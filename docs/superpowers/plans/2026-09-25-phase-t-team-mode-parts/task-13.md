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
  - `policy.Layers{Gateway []byte; Org, Project Layer}`, `policy.Layer{Scope Scope; Version string; Doc []byte}`, `policy.Scope` (`org`, `project`), `policy.ParseLayer(doc []byte) error`
  - `policy.Default(class) []byte`, `policy.GatewayFragment() []byte`
  - `policy.Digest(file []byte) string` (`sha256:<hex>`), `policy.EnvDigest = "PANACEA_APP_POLICY_SHA256"`, `policy.ManagedPath = "/etc/claude-code/managed-settings.json"`, `policy.MaxLayerBytes` (64 KiB)
  - `*policy.InvariantError{Invariant, Class, Scope, Detail}`, `policy.ErrLayer`
- Consumes:
  - task 09 — `model.Class` and `model.ClassWorker`, `model.ClassOrchestrator`, `model.ClassMirror` (**assumed** spellings; task 09 names them)
  - phase 6 task 14 (planned) — the fragment's bytes, `images/sandbox/managed-settings.gateway.json`
  - task 18 — the hook command `panacea check --hook` (`task-18.md:62-67`)
- Consumed by: task 14 (`Render` on every `POST`, for every class), task 15 (the file and the digest into the pod; `sandboxd`'s check reimplements `Digest` in its own module), task 41 (the Orchestrator's instructions assume I8).
