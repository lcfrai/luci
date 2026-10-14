### Task 181: Signed land commits for agent sessions (L-176)

Stream **WL**, control plane with a small Hub delta, about 2.5 hours (an estimate, not a
measurement). Depends on task 174 and on phase 3 task **P3-09** (land) sync-merged, with P3-02
(`gitx`) and P3-04 (`gitdeps.go`) under it. If P3-09's files are not on the sync base, this task
**waits** and writes no second copy (rule H2).

**The row's UNVERIFIED is answered by phase 3's plan: phase 3 does not sign, by design.** Its
§HGH H1 prefix puts `-c commit.gpgSign=false -c tag.gpgSign=false` on every host git call, so a
repository `gpg.program` is never run by P3-09's or P3-10's `commit-tree` (P3 `task-02.md:526` at
`94f954d`; the scope cited `:477`, which has since moved). The land commits as the user, with the
identity `api.Deps.GitIdentity` resolves for the project (P3 `task-09.md:93-95`), in
`GIT_AUTHOR_*` and `GIT_COMMITTER_*` (step 11, `task-09.md:121`). This task adds **one signed
path that keeps H1 whole**: every call is still unsigned unless it asks, and a call that asks
signs only with the machine's own key and program, never the repository's.

**What is signed, and what is not.** The **land's merge commit** is signed: it is the person's
act over the whole range, and its message carries the trailers (SW.9, task 21). **Agent commits
in the lane stay unsigned.** No signing key enters a sandbox: an agent holding one could sign
anything, and a signature would then say nothing about a person. The lane's commits are never
rewritten to sign them either; their ids are what the Files lens, the back-links and task 180's
grouping point at.

**Where the signing settings come from.** Only the machine's `git config --system`, then
`--global`, read once at startup by `MachineHardening` (P3 `task-02.md:118-122`), the same way it
reads H2's credential helpers and proxy. The program is resolved to an **absolute path once**
(the configured one, else `ssh-keygen` or `gpg` from `PATH` through `exec.LookPath`). Repository
config cannot choose any of it: every signing key a repository could set is overridden on the
command line, and the program keys the call does not use point at a path in H1's own empty
hooks directory, which never holds a file.

**Two rules, and the refusal between them.**
1. **The machine rule:** the user's own `commit.gpgSign=true` in system or global config means
   *always sign my lands*.
2. **The team rule:** the project's owning team's `TeamPolicy.requireSignedLand` (the Hub delta
   below), set by a team `admin`.
A land signs when either holds. When one holds and there is no usable format, key or program, the
land is refused **before any write** with `409 signing_unavailable`, and the branch tip is
unchanged. A land never falls back to unsigned when a rule asked for a signature.

**The Hub unreachable.** The land reads the team rule through the Hub client (`Deps.Work`,
SW.2). If the Hub cannot be reached it uses the last value this process read for that project;
with none, the result says `policyRead: false` and only the machine rule applies. The desktop
shows that sentence on the land's toast: *The team's signing rule could not be read, so only
your machine's rule applied.* That is a fact about this land, not a warning about the next.

**Files** (every phase 3 path is **UNVERIFIED until merge**; the phase 3 part that plans it is
cited):
- `control-plane/internal/gitx/harden.go` ⚑ (P3 `task-02.md:19`, `:110-122`) — `SigningConfig`,
  `Hardening.Signing`, `MachineHardening` reads it (modified)
- `control-plane/internal/gitx/runner.go` ⚑ (P3 `task-02.md:18`, `:70-97`) — `Call.Sign`, Exec's
  signing arguments (modified)
- `control-plane/internal/gitx/sign_test.go` — the gitx rows (new)
- `control-plane/internal/converge/land.go` ⚑ (P3 `task-09.md:36-132`) — step 8a, step 11's
  `Sign`, `LandResult.Signed`, `.SigningFormat`, `.PolicyRead`; `Lander.SignPolicy` (modified)
- `control-plane/internal/converge/land_sign_test.go` — the land rows (new)
- `control-plane/internal/api/gitdeps.go` ⚑ (P3 `task-04.md:85-107`) — `ReasonSigningUnavailable`
  (modified)
- `control-plane/cmd/api/main.go` ⚑ (P3-04's wiring) — builds `Lander.SignPolicy` from `Deps.Work`
  (modified)
- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03, `task-03.md:348-355`, `:504-508`) —
  `TeamPolicy.RequireSignedLand`, `PutTeamPolicyRequest.RequireSignedLand *bool` (modified)
- `control-plane/internal/hub/migrate/sql/NNNN_work_wl_signing.sql` — one column on task 06's
  `team_work_policies` (`task-06.md:276`); the next free number at sync time (new)
- `control-plane/internal/hub/store/work_policies.go` ⚑ (task 11) — reads and writes the column
  (modified)
- `control-plane/internal/hub/hubclient/fake_work.go` ⚑ (task 05) — the same rule (modified)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — the `member` PUT row (modified)
- `desktop/src/api/work.ts` ⚑ (task 37) — `TeamPolicy.requireSignedLand` (modified)
- `desktop/src/components/admin/SignedLand.tsx` (+ test) — the one switch row on task 173's admin
  surface; task 173's part is not written at this writing, so its mount point is **UNVERIFIED**
  (new)

#### Interfaces — complete

```go
// gitx/harden.go (P3-02) — appended to the file; one field added to Hardening.

// SigningConfig is the machine's signing setup, read once at startup from `git config
// --system` then `--global` (a global value wins, as git's own order does). Never from a
// repository. The zero value signs nothing.
type SigningConfig struct {
	Format  string // "ssh" or "openpgp"; "" = no signing configured. x509 is not supported.
	Key     string // user.signingKey as the machine sets it: a key path, or "key::<public key>" for ssh
	Program string // absolute path: gpg.ssh.program / gpg.program if set, else LookPath("ssh-keygen" | "gpg")
	Always  bool   // commit.gpgSign=true on the machine: the user's own "sign my lands"
}

// Usable reports whether a signed call can run: a Format, a Key and an absolute Program that
// exists. The land refuses signing_unavailable when a rule asks and this is false.
func (s SigningConfig) Usable() bool

// Hardening gains, after LaneRoot:
//	Signing SigningConfig // MachineHardening fills it; zero when the machine configures none

// MachineHardening reads, beside H2's keys, gpg.format, user.signingKey, gpg.ssh.program,
// gpg.openpgp.program, gpg.program and commit.gpgSign from --system then --global (Dir root,
// Scope None, as H2's reads are). gpg.format "x509" or any other value → Format "" and the reason
// logged once. A relative Program is resolved with exec.LookPath once, here, and never again.
```

```go
// gitx/runner.go (P3-02) — one field added to Call, after NewLane.

	// Sign asks Exec to sign this call's commit with Hardening.Signing. Only `commit-tree` may
	// set it (Args[0] == "commit-tree"); any other Args[0], or a Signing with Format "", is
	// ErrCallShape and git never starts. So no call signs with a default, and no call signs by
	// accident.
	Sign bool

// Exec, on a Sign call only, appends after §HGH's prefix and before Call.Args, in this order:
//   -c gpg.format=<Format>
//   -c gpg.ssh.program=<Program | NoSigner>        (Program when Format is "ssh")
//   -c gpg.openpgp.program=<Program | NoSigner>    (Program when Format is "openpgp")
//   -c gpg.program=<Program | NoSigner>            (the legacy alias of gpg.openpgp.program)
//   -c gpg.x509.program=<NoSigner>
//   -c gpg.ssh.defaultKeyCommand=                  (empty: a repository key command never runs)
//   -c user.signingKey=<Key>
// and inserts -S directly after "commit-tree". H1's commit.gpgSign=false stays in the prefix:
// -S is explicit, so the prefix still stops every call that did not ask.
// NoSigner is filepath.Join(Hardening.HooksDir, "no-signer"): a path in H1's empty directory,
// which never holds a file, so a program key the call does not use cannot run anything.
```

```go
// converge/land.go (P3-09) — additions only.

// SignPolicy answers the team rule for a project: required, and whether the Hub was read. cmd/api
// builds it from Deps.Work (SW.2): the project's owning team, then TeamPolicy(ctx, teamID), with
// the last value per project kept in memory for when the Hub cannot be reached. nil means "no Hub
// in this build": read false, required false.
type SignPolicy func(ctx context.Context, projectID string) (required bool, read bool)

// Lander gains, after Identity:
//	Signing    gitx.SigningConfig // GitDeps' Hardening.Signing, the same value Exec holds
//	SignPolicy SignPolicy

// LandResult gains, after BroughtBack:
//	Signed        bool   `json:"signed"`        // the merge commit carries a signature
//	SigningFormat string `json:"signingFormat"` // "ssh" | "openpgp" | "" when not signed
//	PolicyRead    bool   `json:"policyRead"`    // false: the team rule could not be read; the machine rule alone applied

// Land's step list gains 8a, between Identity (8) and merge-tree (9), and step 11 gains Sign:
//  8a. required, read := SignPolicy(ctx, ProjectID) (nil → false, false)
//      sign := Signing.Always || required
//      sign && !Signing.Usable() → *Refusal{Reason: ReasonSigningUnavailable,
//        Details: {"why": "no_format" | "no_key" | "no_program", "rule": "machine" | "team"}}
//      — before any write, so the refs are untouched
//  11. merge = commit-tree <tree> -p old -p laneTip -F -   with Call.Sign = sign
//      a signing failure from git (a locked key, an absent agent) → *Refusal{Reason:
//      ReasonSigningUnavailable, Details: {"why": "signer_failed", "gitSays": <git's sentence>}};
//      the unreferenced commit object is left for git's own gc, and no ref moves (step 12 not run)
```

```go
// api/gitdeps.go (P3-04) — one constant in the closed list, after ReasonNoGitIdentity.
	ReasonSigningUnavailable = "signing_unavailable" // lands; details.why, details.rule | details.gitSays; 409, nothing written
```

**The Hub delta** (additive, SW.1's added-field rule; the same sync-merge rule as team mode's
Hub deltas: applied on top of `feat/hub` at sync, never a second copy).

```go
// hubapi/work.go (task 03) — TeamPolicy gains one field, after StopAtLandedWithoutServes.
	// RequireSignedLand: every land into this team's projects must be signed by the person
	// landing (task 181). Only the land reads it; no issue state depends on it.
	RequireSignedLand bool `json:"requireSignedLand"`

// PutTeamPolicyRequest gains a POINTER, not a bool. Task 11's PUT is a full replace ("every
// field present", task-11.md:123), and a client built before this task sends no such field; a
// plain bool would read that absence as false and silently switch a team's rule off. Absent
// (nil) keeps the stored value; present sets it.
	RequireSignedLand *bool `json:"requireSignedLand,omitempty"`
```

```sql
-- migrate/sql/NNNN_work_wl_signing.sql — the next free number at sync time (rule H1).
ALTER TABLE team_work_policies
  ADD COLUMN require_signed_land boolean NOT NULL DEFAULT false;
```

- **The route** is task 11's `PUT /v1/teams/{teamId}/work-policy`, team `admin` only, unchanged
  in shape; `store.PutTeamPolicy`'s `changed` list names `requireSignedLand` when it moves, so
  task 11's `team.work_policy_set` event carries it (`task-11.md:288`).
- **The fake** (`hubclient/fake_work.go`) applies the same nil-keeps rule; a conformance row runs
  it against both.
- **The desktop mirror:** `TeamPolicy` in `desktop/src/api/work.ts` (task 37) gains
  `requireSignedLand: boolean`; its put request gains `requireSignedLand?: boolean`.
- **The switch** (`SignedLand.tsx`): one row, *Require signed lands*, with the sentence under it:
  *Each land into this team's projects must be signed by the person landing. Agent commits stay
  unsigned. A land without a usable key is refused, not left unsigned.* It is a checkbox for an
  `admin`; a `member` sees the value as text. No count of signed lands, no share, no badge.

#### Tests first — each with the mutant it kills

The gitx and land rows run on P3-02's `gitxtest.Repo` rig with a **throwaway ed25519 key**
generated per test by `ssh-keygen -t ed25519 -N "" -f <tmp>/k` into `t.TempDir()`, and a test
machine config written to a temp `GIT_CONFIG_GLOBAL` with `GIT_CONFIG_NOSYSTEM=1`, so the
developer's own keys and config are never read or used. A test skips with a stated reason when
`ssh-keygen` is absent (`exec.LookPath`), and says so in its output, never passes silently.

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestSignedLandVerifies` (`land_sign_test.go`) | machine config `gpg.format=ssh`, `user.signingKey=<tmp>/k.pub`, `commit.gpgSign=true`; a land; then a test-only `git -c gpg.ssh.allowedSignersFile=<tmp>/allowed log -1 --format=%G? <merge>` prints `G`; `LandResult.Signed` true, `SigningFormat` `ssh` | a dropped `-S` |
| `TestRepositorySignerNeverRuns` (`land_sign_test.go`) | the project's `.git/config` sets `gpg.format=openpgp`, `gpg.program`, `gpg.ssh.program` and `gpg.ssh.defaultKeyCommand` to a canary script that writes `<tmp>/ran`, and `user.signingKey` to a bogus key; a signed land succeeds with the machine's key and `<tmp>/ran` does not exist | a signed call that lets repository program or key settings through |
| `TestRequiredWithoutKeyRefuses` (`land_sign_test.go`) | `SignPolicy` returns required; machine config has no signing key → `*Refusal` `signing_unavailable`, `details.why` `no_key`, `details.rule` `team`; `rev-parse refs/heads/<b>` equals the tip before | a silent unsigned fallback |
| `TestUnsignedByDefault` (`land_sign_test.go`) | no machine rule, `SignPolicy` not required → `%G?` is `N`; `Signed` false, `SigningFormat` `""` | "always sign" |
| `TestLaneCommitsNotResigned` (`land_sign_test.go`) | a lane of three commits; after a signed land, `rev-list <old>..<merge>^2` is the same three ids, and each `%G?` is `N` | a land that rewrites the range to sign it |
| `TestPolicyUnreadUsesMachineRule` (`land_sign_test.go`) | `SignPolicy` returns `(false, false)`, machine `Always` → signed, `PolicyRead` false | the Hub's absence treated as "required" or as "machine rule off" |
| `TestSignCallWithoutFormatRefused` (`sign_test.go`) | `Call{Args: ["commit-tree", …], Sign: true}` with `Signing{}` → `ErrCallShape`, git never starts (P3-02's spy) | a signing default |
| `TestSignOnlyOnCommitTree` (`sign_test.go`) | `Sign: true` with `Args[0] == "tag"` → `ErrCallShape` | a signed call shape wider than the land needs |
| `TestSignArgsOrder` (`sign_test.go`) | the spy's argv for a signed call holds H1's `commit.gpgSign=false`, then the seven signing `-c` pairs in the stated order, then `commit-tree -S`; unused program keys equal `NoSigner` | a missing override, or `-c` after the subcommand |
| `TestMachineHardeningReadsSigning` (`sign_test.go`) | a temp global config with `gpg.format=ssh` and a bare `gpg.ssh.program=ssh-keygen` → `Program` is absolute; a repository config naming another program changes nothing | a program resolved per call, or read from the repository |
| Hub: `member PUT is 403` (`matrix_integration_test.go`) | a `member` PUT with `requireSignedLand: true` → 403 and the stored value unchanged | a missing matrix row |
| Hub: `absent keeps` (conformance, `-tags integration` and the fake) | set true by an `admin`; a PUT without the field → still true | an old client silently switching the rule off |
| `SignedLand` (`SignedLand.test.tsx`) | an `admin` sees a checkbox; a `member` sees *Required* or *Not required* as text; `expectNoPrediction(container)` passes | a member-editable control; a count or share |

#### Steps

- [ ] **Step 1: Preflight.** Rule H2 for `gitx/harden.go`, `gitx/runner.go`,
  `converge/land.go`, `api/gitdeps.go` and `cmd/api/main.go`; tasks 03, 05, 06 and 11 on the
  Hub sync base. Re-read `Hardening`, `Call`, `LandResult` and the reason list; the phase 3 names
  win. Confirm H1's prefix still holds `commit.gpgSign=false` (P3 `task-02.md:526`).
- [ ] **Step 2: gitx, tests first.** Write `sign_test.go`; run
  `go test ./control-plane/internal/gitx/... -run 'Sign|MachineHardeningReadsSigning' -count=1`.
  Expected: FAIL, `unknown field Sign in struct literal of type gitx.Call`. Implement
  `SigningConfig`, `Usable`, `MachineHardening`'s reads, `Call.Sign` and Exec's arguments. Re-run.
  Expected: `ok`.
- [ ] **Step 3: The land, tests first.** Write `land_sign_test.go`; expected FAIL,
  `LandResult has no field or method Signed`. Implement step 8a, step 11's `Sign`, the three
  result fields and the constant. Re-run. Expected: `ok`. Then P3-09's own land tests, unchanged:
  `go test ./control-plane/internal/converge/... -race -count=1`. Expected: `ok`.
- [ ] **Step 4: The Hub delta, tests first.** Write the matrix row and the conformance row;
  expected FAIL on the missing field. Add the field, the pointer, the migration, the store and the
  fake. Re-run with `-tags integration`. Expected: `ok`.
- [ ] **Step 5: The wiring.** In `cmd/api/main.go`, build `SignPolicy` from `Deps.Work` and set
  `Lander.Signing` from the `Hardening` Exec already holds. `go build ./control-plane/...`.
  Expected: exit 0.
- [ ] **Step 6: The switch, tests first.** `SignedLand.test.tsx`, then the component and the
  `work.ts` field; `cd desktop && npx vitest run src/components/admin`. Expected: `ok`.
- [ ] **Step 7: Acceptance, then commit** (`git add` the new files, then
  `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
go test ./control-plane/internal/gitx/... ./control-plane/internal/converge/... ./control-plane/internal/api/... -race -count=1
# expected: "ok" per package; a skipped signing row prints "ssh-keygen not found" and is named in the commit body
go test -tags integration ./control-plane/internal/hub/server/... ./control-plane/internal/hub/conformance/... -count=1
# expected: "ok" per package, including the member-PUT and absent-keeps rows
cd desktop && npx vitest run src/components/admin && npm run typecheck
# expected: every file passes; tsc exits 0
```

#### What this task does not claim

- **Not that GitHub or Azure DevOps shows the land as verified.** The user must register the key
  with the host; how each host treats an SSH-signed merge commit is **UNVERIFIED** on both.
- **Not that a host rule requiring every commit signed accepts the land.** The lane's parents are
  agent commits, unsigned by design, so such a rule refuses the push; how each host counts a
  merge's parents is **UNVERIFIED**.
- **Not signing in the cloud.** A managed session holds no user key (phases 6 and 8,
  **UNVERIFIED**); this is the desktop's local land only.
- **Not that P3-10's revert is signed.** The same `Call.Sign` applies once P3-10 is merged; this
  row names only P3-09.
- **Not x509 (gpgsm) signing.** Format `x509` on the machine reads as "no signing configured",
  and a rule that asks for a signature is then refused, stated with `details.why` `no_format`.
- **Not that the Hub's project-to-team read exists under that name.** The land needs the
  project's owning team; the Hub holds it (`projects.team_id`, `task-06.md` / `task-07.md:175`),
  and the `hubclient.Work` method that returns it to the control plane is **UNVERIFIED** here
  (the desktop's mirror is task 57's `ProjectScope.teamId`). The part names it as a skeleton gap.
- **Not that phase 3's files look as cited.** Every phase 3 path and line is from the plan at
  `94f954d`, **UNVERIFIED until merge**.
- Design: 05 §1 row L-176, §2.3 (the land on the thread), §3.6 (trailers carry ids). Parity:
  closes **L-176**.

**Interfaces:**
- Produces:
  - `gitx.SigningConfig{Format, Key, Program, Always}`, `SigningConfig.Usable()`,
    `Hardening.Signing`, `Call.Sign`
  - `converge.SignPolicy`, `Lander.Signing`, `Lander.SignPolicy`, `LandResult.Signed`,
    `.SigningFormat`, `.PolicyRead`
  - `api.ReasonSigningUnavailable = "signing_unavailable"` (409; `details.why` one of `no_format`,
    `no_key`, `no_program`, `signer_failed`)
  - Hub: `TeamPolicy.RequireSignedLand`, `PutTeamPolicyRequest.RequireSignedLand *bool`, the
    `require_signed_land` column
  - desktop: `TeamPolicy.requireSignedLand`, `SignedLand`
- Consumes:
  - P3-02 — `Hardening`, `MachineHardening`, `Call`, `Exec`, `ErrCallShape`, H1's prefix, the spy
  - P3-09 — `Lander`, `Land`'s steps 8-12, `Refusal`, `LandResult`
  - P3-04 — the closed reason list and its 409 mapping
  - task 03 — `TeamPolicy`, `PutTeamPolicyRequest`; task 11 — the policy route and store; task
    05 — the fake and conformance; task 06 — `team_work_policies`; task 20 — `expectNoPrediction`
  - task 173 — the admin surface the switch sits on (**UNVERIFIED**: part not written)
- Amends: task 11's PUT gains an optional field whose absence keeps the stored value.
