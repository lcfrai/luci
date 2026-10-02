# Seams — where later phases plug into Phase 2

A **seam** is a place in Phase 2's shipped code where a later phase plugs in. Each row names
the place, what that phase does there, what it must not do, and the test that fails if the
seam moves. **This document is living**: a phase that consumes or changes a seam edits its row
in the same commit, and the manifest below refuses to pass until it has.

Check every seam with two commands, from the repository root:

    (cd desktop && npm test -- src/seams.test.ts)
    go test ./control-plane/internal/backend/ -run Seam -count=1

`desktop/src/seams.test.ts` holds an anchor per seam — a declaration or call, never a comment
— and fails unless this document's seam ids and its own are the same set. A row added here
without an anchor fails; an anchor with no row fails. Beside the anchors it holds two verbatim
copies to their sources (SX.2: `types.ts` and UI contract §2; S8.3: `katabe`'s package comment
and Go contract §2.6), and fails while a pending entry's task has not been told, in its own
plan, to clear it, and to check both files that clearing edits (the manifest and this document)
for other work before it starts and to name both in its commit.

**A row points at a file and a declaration, not at a line number**, wherever that file is one
a later phase or another chain still writes. The anchor in the manifest already pins the place
mechanically; a line number is the second, unchecked copy of the same fact, and it rots on any
edit *above* it. Measured on 2026-09-28 at `7a5a4874`: of the twenty-nine citations into
`sessions.go`, `client.ts`, `project.go` and `argv.go`, every one of the twenty-two resolved by
hand pointed at the wrong line, most by more than a hundred, because three other chains had grown
those files since the rows were written. `sessions.go:1069`, cited for `sessionResolveResume`, was
a bare `}`; the declaration had moved to `:1190`. Names survive that; numbers do not. Every other
citation in this document was resolved against its file the same day, and one more was wrong —
`main.go:306`, cited for `kubeClient`, which is at `:309` — so it lost its number too. Measured
again on 2026-09-29 at `c4ca81d8`: commits since `7a5a4874` had changed `api.go`, `project.go`,
`project_test.go` and `worktree.go`, and four of the six citations into them had rotted the same
way (`api.go:269,276` and `:391`, `project_test.go:429`, `worktree.go:32`, each now a comment or
another declaration), so all six name declarations now. The twenty-five that remain each point at
the line they claim, in files no commit has changed since `7a5a4874`, and they are the next
sweep: the rule to enforce is that a row may not cite a line in any file the manifest anchors,
because the anchor is the copy that fails when it is wrong.

Named by Phase 2 task 26 on 2026-09-26. The close-out ran task 26 **before** task 25
(`docs/superpowers/RESUME.md`, "Phase 2 close-out"), so the end-to-end proof on Docker that these
seams sit in — task 25's results document under `docs/verification/` — did not exist yet when they
were named. Task 25 has since run, and its results document is
`docs/verification/2026-09-27-phase-2-results.md`. Its run drove all nineteen checks, 10 PASS and
9 FAIL, and `o2.resume` is one of the failures, so nothing in it is cited here as proof of a seam.
SX.10 still rests on the Go tests its manifest entry names.

## Rules that bind every phase

Not seams — none has one location — but each is broken the same way by every phase that
forgets it, and each is already enforced where cited.

1. **`store/types.ts` is byte-identical to UI contract §2.** Adding a field amends §2 in the
   same change. Pinned by SX.2.
2. **A new persisted key bumps `PERSIST_VERSION` and writes a migration that translates and
   never drops** (`desktop/src/store/store.ts:50-53`). Two phases doing this concurrently
   produce two version-2 migrations; sequence them. Pinned by SX.3.
3. **A derived read is a selector in `store/selectors.ts` with a doc comment naming its
   surface**, never an inline `.filter()` inside `useCockpit(...)`, which re-renders on every
   2 s poll (group-d shared shape §4; `selectors.ts:3-5`).
4. **CSS obeys shared shape §3B's six rules** — token import first, no colour function outside
   `tokens.css`, no gradient, no `transition: all`, `var(--numeric)` on live numbers, status
   separable without motion or colour. `desktop/src/styles/designsystem.test.ts` greps them.
5. **`jest-dom` is not installed** (shared shape §3C). Assert on DOM properties.
6. **No feature reads PTY bytes to derive meaning** (`docs/PROGRAM.md:129`). Every seam below
   respects this, and a phase that needs "blocked on you" needs a new server signal, not a
   client derivation (`survey-phase-2-substrate.md:90-94`).

## The seams

| ID | Phase | Where | The phase does | The phase must not | Guard |
|---|---|---|---|---|---|
| SX.1 | 3, 4, 5, 7 | `desktop/src/components/lens/{Discover,Design,Files,Merge,Deploy}Lens.tsx`; `AppShell.tsx` `LensRouter` | Replace the body of its own lens file | Edit `AppShell.tsx`, which is closed; add a `Lens` union member — all six exist (`types.ts`) | seams.test.ts `SX.1` |
| SX.2 | all | `desktop/src/store/types.ts` ≡ UI contract §2's fenced block | Amend both in one commit when the model grows | Add a field to one side | seams.test.ts `SX.2` gate |
| SX.3 | all | `desktop/src/store/store.ts:16-17` (`PERSIST_KEY`, `PERSIST_VERSION = 1`), `partialize` at `:34-48` | Add a root key, bump the version, write the migration, update this row | Nest a slice — `persist` shallow-merges at the root (`store.ts:3-6`) | seams.test.ts `SX.3` |
| SX.4 | 3, 4, 5 | `control-plane/internal/api/api.go` — every route registers through the recorder (`routes.Handle`, or `routes.HandleFunc`, which calls it) | Add its routes to the table and to both audit maps | Register on the raw mux; add a side-effecting GET | `TestNoRegisteredGetCanChangeState`, `TestNoFileRegistersOnTheRawMuxBehindTheRecorder` |
| SX.5 | 3, 4, 5 | `control-plane/internal/statepath/statepath.go:30` `Dir(sub)` | Take one subdirectory — Phase 4's is `Dir("discover")` | Re-derive the state ladder | seams.test.ts `SX.5` |
| SX.6 | 3, 4, 5 | `desktop/src/components/palette/commands.ts` — `Command`, `buildCommands(state)` | Append its commands; ranking is fixed here, filtering only removes | Add a destructive verb one keystroke from a filter (open carry 4, defect 10's shape) | seams.test.ts `SX.6` |
| SX.7 | all | `desktop/src/api/problems.ts:13,37` — `ProblemKind`, `classify` | Switch over `ProblemKind` at its call site with a declared return type and its own sentences | Add a `default:` to that switch, or a shared `sentenceFor` | seams.test.ts `SX.7` |
| SX.8 | all | `desktop/src/hooks/heartbeat.ts` — `BEAT_MS`, `beatDelayStyle()`, `useHeartbeat()` | Use them for anything that breathes or ticks | Build a third clock; subscribe a row per list item to the 1 Hz tick | seams.test.ts `SX.8` |
| SX.9 | 3 (`--drift`); unowned (`--need`) | `desktop/src/styles/tokens.css:75,77` | Use the token the day the status can be derived, and update task 24's audit assertion in the same change | Invent a second amber or purple | designsystem.test.ts *defines --drift and references it nowhere* |
| SX.10 | 8; every phase that resumes | Claude Code started with `--session-id <uuid>`, and a resume forked from that UUID with `--resume <uuid> --fork-session` (`control-plane/internal/harness/argv.go`, the two `argv = append` calls); the cockpit's id resolved to Claude's by `sessionResolveResume` (`api/sessions.go`); the conversation store persisted by `sessionMounts`'s mount of `conversationStore` at `sessionConversationDir` (`api/sessions.go`), whose mount point the image makes as `dev` (`RUN mkdir -p /home/dev/.claude/projects`, `images/sandbox/Dockerfile:32`) | Carry the store's persistence into Kata pods, whose home is an `emptyDir` (see note, and S8.8) | Hand the cockpit's session id to `claude --resume`; resume without `--fork-session`; parse agent output to learn Claude's | seams.test.ts `SX.10`; task 25 `o2.resume` — **FAILED** in `docs/verification/2026-09-27-phase-2-results.md` (*"timed out after 15000 ms waiting for the resumed session"*, caused by **F2** — a host-git read 500'd the resume's own create, so the fork was never reached), so not proof |
| S3.1 | 3 | `control-plane/internal/project` — read-only by design (`project.go`'s package comment) | Put worktree lifecycle and every git write in a new package | Widen `project`'s whitelist of `for-each-ref`, `rev-parse`, `worktree list` | `TestEveryGitCommandIsReadOnly` (`project_test.go`) |
| S3.2 | 3, 4 | `control-plane/internal/worktree/worktree.go`'s `Provider.Path(ctx, worktreeID)` | Make every worktree it creates resolve through it; session create does (`sessions.go`, in `handleSessionCreate`), and so does restart (in `handleSessionRestart`) | Resolve paths a second way | seams.test.ts `S3.2` |
| S3.3 | 3 | `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}` (`catalog.go:33`); `Worktree.ready?` (`types.ts:85`); `setWorktreeReady` (`client.ts`) | Gate merge on `ready` | Derive readiness any other way (known defect 9) | seams.test.ts `S3.3` |
| S3.4 | 3 | `desktop/src/lib/immutable.ts` — `moveWithinVisible`, `idAtVisiblePosition` | Reorder the merge queue through them | Pass an index across a filter boundary (known defect 10) | defects.test.tsx row 10 |
| S3.5 | 3 | `FilesLens.tsx`, `MergeLens.tsx` — both render phase 3 | Fill both | Treat Files' ownership as settled: `docs/PROGRAM.md` names no owner for it (open carry 8) | seams.test.ts `S3.5` |
| S4.1 | 4 (write), all (read) | `Session.contextRef`: `registry/session.go:102-104`; `api/sessions.go`'s four mentions (the response type and its build, the create request type, and the record create writes); `types.ts:163`; `defaults.ts:110,127`; carried by resume and fork (`client.ts`, both `from.contextRef` copies) | Write it on create; add a resolve endpoint so surfaces can show a brief's title | Make Phase 2 code interpret it; render it from a component before updating this row — the tree tripwire fires | seams.test.ts `S4.1` |
| S4.2 | 4 | `handleSessionCreate` and `handleSessionRestart` (`api/sessions.go`): in each, between its `Worktrees.Path` and its `Backend.Create`. One window, two occupants: the resume fix's Claude session id is minted there, and the `Materialiser` ruled by R1 lands there | Materialise `CLAUDE.md` there through R1's `Materialiser` (declared in `api`, held in `api.Deps`), **materialising first**, and stamp `ContextRef` from the key it returns, so the stamp and the minted session id both describe the process that starts | Rely on the client to call it first — silent inheritance failure; derive `ContextRef` from anything but the materialiser's return | seams.test.ts `S4.2` (`api.go` has no materialiser) |
| S4.3 | 4 | `desktop/src/lib/immutable.ts` — `replaceAt`, `replaceById` | Implement `editAnswer` through them | Splice later answers (known defect 7) | defects.test.tsx row 7 |
| S4.4 | 4 | `project.Scanner.ProjectName` / `BranchName` (`project.go`); `/api/v1/` prefix strip (`api.go`'s `http.StripPrefix("/api", guarded)`) | Use them as its `Naming` and its route prefix | Delete the prefix strip — its comment says why | seams.test.ts `S4.4` |
| S5.1 | 5 | `desktop/src/api/poller.ts`'s `POLL_INTERVAL_MS` and `client.ts`'s `makeSocket` — the only transports | Bring its canvas room as a named second transport, and update this row | Open a socket or `EventSource` anywhere without updating this row — the tree tripwire fires on either name anywhere in code — in every module extension Vite bundles under `desktop/src` (`.ts`, `.tsx`, `.mts`, `.cts`, `.js`, `.jsx`, `.mjs`, `.cjs`), strings included, since only what TypeScript's own parser calls a comment is stripped, never a string, template, regex or JSX text (so `new`, a member of any object, bound, passed, returned, destructured, imported or named as a string), unless it follows a type position's token (`:`, a single `\|` or `&`, `<`, `as`, `typeof`) or reads a constant (`WebSocket.OPEN`). It cannot tell a value after a colon from an annotation (`{ open: WebSocket }`, a ternary's else branch), nor see a name built at run time (`globalThis["Web" + "Socket"]`). It cannot see a dependency that opens its own socket, so adding such a dependency also means updating this row | seams.test.ts `S5.1` |
| S5.2 | 5 | `.shell-lens` (`shell.css:30-33`); `DesignLens.tsx` | Render into the fixed box and own its scroll | Resize the shell or scroll the page | seams.test.ts `S5.2` |
| S8.1 | 8 | `backend.Backend` (`backend.go:337-345`), seven methods; `katabe` implements it (`kata.go:32`) | Turn katabe on behind it | Change the interface without this row — a method, a signature, the package a signature's type comes from (`*websocket.Conn` is `coder/websocket`), or the set of `Kind` constants, which is exactly two in the whole control plane. Reflection fails on the first three, a type-check of every control-plane package's source on the last (by the language's rules, so a bare name in a `const (…)` group, `= KindKata`, an alias of `Kind` (another package's included), a constant declared in `katabe` or any other package, and a third value with no name such as `backend.Kind("x")` or an untyped `"x"` compared with a `Kind` all count) | `TestSeamBackendIsTheSevenMethodsBothImplementationsServe`, `TestSeamBackendKindsAreTheTwoPersistedStrings` |
| S8.2 | 8 | `PANACEA_APP_BACKEND=kata` (`config.go:31`) → `newBackend` → `kubeClient` (`main.go`), in-cluster only | Add `clientcmd` under contract §8.5's pinning review if a kubeconfig is needed | Honour `PANACEA_APP_KUBECONFIG` silently | seams.test.ts `S8.2` |
| S8.3 | 8 — **its first problem** | Create and restart always send `WorkDir` and host `Mounts` (`api/sessions.go`, in both `backend.Spec` literals), built by `sessionMounts` with the worktree first; `katabe.Create` refuses both (`kata.go:101-106`), so every create under Kata fails today | Before anything else: design how a worktree gets into a pod, then change both sides and this row | Drop the refusal to make creates pass — that ships an empty worktree with no error; treat Kata as a configuration switch | `TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv`, `TestCreateRejectsMountsAndCreatesNoPod`, `TestCreateRejectsAWorkDir`; seams.test.ts `S8.3` (katabe's package comment and Go contract §2.6's copy, byte-identical) |
| S8.4 | 8 | `const runtimeClass = "kata-mshv-vm-isolation"` (`sandbox/manager.go:20`) | Confirm it against a real cluster (Phase 1 unproven item 2) | Ship it unobserved | seams.test.ts `S8.4` |
| S8.5 | 8 | `isolationFor` (`api/catalog.go:174`); the rail's badge (`MachineChip.tsx:45`) | Report `microvm` for Kata machines | Hide the badge client-side | seams.test.ts `S8.5` |
| S8.6 | 8 | `desktop/src/api/client.ts:6-8` — no `Authorization` header | Add auth with its server half, together | Add a header "for safety" in an earlier phase | seams.test.ts `S8.6` (see note) |
| S8.7 | 8 | `proxy.RelayHooked` (`proxy/proxy.go`), called once at `api/attach.go:216` | Reach pods through it | Add a second relay, in any file — the tree tripwire fires on any other `func …Relay…(`, or package-level `var` or `const` named `…Relay…` (a relay as a function value), in the control plane, on any code that names `RelayHooked` except its declaration and `attach.go`'s one call (a wrapper under any name), and on any connection type (`*websocket.Conn`, `net.Conn`, any package's `Conn`) in code (a parameter, a field, a var) other than a result — recognised by the parameter list's closing `)` in front of it, so a type assertion (`a.(*websocket.Conn)`) is not one — and, **in `proxy/proxy.go` and `api/attach.go` only**, the parameters of `RelayHooked`, its `copyMessages` loop and `relayAttached`: those three exemptions are the files' own, and the same spellings in any other file trip it (a relay under any name, by what it holds), and on any code that names a message kind (`websocket.MessageType`, `MessageBinary`, `MessageText`) or `websocket.NetConn`, or relays over the standard library instead (`httputil.`, `.Hijack(`, as in `httputil.NewSingleHostReverseProxy(pod).ServeHTTP(w, r)`, which has forwarded a WebSocket upgrade since Go 1.12) (by what it moves). It cannot see a relay whose connections and messages are never named by type (inferred `:=` locals only) and that does not reach the pod through either of those two standard-library forms; `proxy.Relay` was deleted by task 26 (open carry 12) | seams.test.ts `S8.7` |
| S8.8 | 8 | The conversation store: `<state>/conversations/local/<worktree>/` mounted read-write at `sessionConversationDir` (`api/sessions.go`), and a fork's staged read-only copy of its source's transcript (`carried`, appended by `sessionMounts`); refused by S8.3's `len(spec.Mounts) > 0` | Provide a volume per user and worktree that outlives the pod and holds `projects/` only (Go contract §2.6), so SX.10's resume survives a pod | Put the store in the pod's `emptyDir` home; share one store across users or worktrees | `TestCreateMountsOnlyThisWorktreesConversationStore`, `TestAForkIsShownAStagedReadOnlyCopyOfTheSourcesTranscript`; seams.test.ts `S8.8` |
| S8.9 | 8 | The CA bundle: `PANACEA_APP_CA_BUNDLE` mounted read-only at `sessionCABundlePath` = `/etc/panacea-app/ca-bundle.pem` and named by `NODE_EXTRA_CA_CERTS` (`api/sessions.go`); refused by S8.3's check | Deliver the operator's CA to the pod another way, at the same path and named by the same variable | Drop `NODE_EXTRA_CA_CERTS`, without which Claude Code cannot start on a TLS-inspecting network; set `SSL_CERT_FILE` or `NODE_TLS_REJECT_UNAUTHORIZED`, which tests assert are absent | `TestACABundleIsMountedReadOnlyAndNamedByNodeExtraCACerts`; seams.test.ts `S8.9` |
| S8.10 | 8 | A primary worktree's `.git/worktrees`, mounted read-only over itself at `sessionWorktreeAdminDir` (`api/sessions.go`); refused by S8.3's check | Decide it with S8.3's worktree delivery; under OD-2's one local clone per worktree it may vanish, and this row with it | Make it writable: the sandbox's git calls every linked worktree prunable, and a `git worktree prune` there deletes the host's metadata (`sessionWorktreeAdmin`'s comment) | `TestAPrimaryWorktreesLinkedWorktreeMetadataIsReadOnlyInTheSandbox`; seams.test.ts `S8.10` |
| S8.11 | 8 | The host git's global excludes and attributes files, read-only at `sessionGitExcludesPath` and `sessionGitAttributesPath` (`api/sessions.go`), mounted only when the host's git reads them; refused by S8.3's check | Carry the user's own git config into the pod, or state that a cloud sandbox has none | Copy a file the host's git does not read; make either writable | `TestTheHostsGlobalGitFilesReachTheSandboxReadOnlyAndOnlyWhenTheHostsGitReadsThem`; seams.test.ts `S8.11` |

## Notes on the gaps

### S4.2 — ruled, reserved in prose, not built

The Go contract §5.1 reserved the space — *"Anything Phase 4 later needs on disk before the
agent starts — `CLAUDE.md` — is written between steps 1 and 5"* — but `api.Deps`
(`api.go`) has no materialiser field (`survey-phase-4-discover.md:97-104`). Who calls
`Materialise` was **decision 0c** (`dependency-graph-and-order.md:233`), and the orchestrator has
ruled it: **R1** in `docs/superpowers/planning/2026-09-23-phases-4-5-7/orchestrator-rulings.md`
— a `Materialiser` interface declared in `api`, satisfied by Phase 4's `contextwrite`, held in
`api.Deps`, called between worktree resolution and `Backend.Create` in both `handleSessionCreate`
and `handleSessionRestart`, returning the key it wrote so the handler stamps `ContextRef` from it.
Phase 4 owns `sessions.go` for that change.

**One window, two occupants, in this order.** The resume fix already mints Claude's session id in
the same window (`api/sessions.go`, in create and in restart). Phase 4
materialises **first**, so the stamped `ContextRef` and the minted session id both describe the
process that actually starts. The guard pins the state that R1 is ruled and not built: the day a
materialiser lands in `api.go`, the manifest fails and this note is rewritten to say where it went.

### S8.3 — the first thing Phase 8 must solve: getting the worktree into the pod

Every create and restart sends host-path mounts, the worktree first, and a working directory
(`api/sessions.go`, in both `backend.Spec` literals), and `katabe.Create` refuses both, correctly — *"a host
path on an AKS node is meaningless"*, and silently dropping the mount would hand the agent *"an
empty worktree with no error anywhere"* (`kata.go:92-106`). The six mounts after the worktree
are seams of their own, four of them, S8.8 to S8.11: each is refused by the same check, and each
needs its own answer once the worktree has one. So under `PANACEA_APP_BACKEND=kata` every
session create fails today. `katabe`'s header used to say turning it on was *"a configuration
change and not a rewrite"*; task 26 corrected it to say this instead. **Before Phase 8 does
anything else, it designs how a worktree reaches a pod** — and the refusal stays until that
design replaces the mount, never the other way round. The three Go tests in the Guard column pin
the two halves; the one that flips first names the side that changed. The fourth holds Go
contract §2.6's copy of `katabe`'s header byte-identical to `kata.go`'s, because the copy kept
the old claim after the file was corrected (task 26 review I1).

### S3.5 — Files has a phase number and no owner

`AppShell.tsx` rendered Files as phase 3 from the start, and `FilesLens.tsx` still does;
`docs/PROGRAM.md`'s phase 3 — *"diff review with transcript back-links"* — is its plausible home.
But `PROGRAM.md` names no owner for Files, and open carry 8 raises it with the product owner.
Task 23's sweep found a third placeholder of the same class — *Authority*, asserted as Phase 3 on
the strength of `docs/AGENTIC-UX.md` §7 (`group-d-open-carries.md:90-94`) — and one product
decision settles all three. Until then, the phase number in `FilesLens.tsx` is a claim the code
makes, not a decision.

### SX.10 — resume by Claude's own id, from a store that outlives its sandbox

Phase 2 planning found resume could not restore context: the client sent the cockpit's own
session id to `claude --resume`, where the Go contract then typed `Resume` as a prior agent
session id (Go contract §2.9, `harness.Request`, since amended), and a resumed session is a new
sandbox, so the old conversation store was gone. A separate fix, landed before task 25, starts
Claude Code with `--session-id <uuid>`, resumes from that UUID as a fork (`--resume <uuid>
--fork-session`, the only pairing the CLI accepts), and persists the conversation store (S8.8).
The Go tests in the manifest's SX.10 entry prove the argv, the resolution and the mount. Task
25's `o2.resume` is the end-to-end proof of identity and the store, and the owner's step-18
observation in its results document the proof that the agent remembers. Task 25's results
document, `docs/verification/2026-09-27-phase-2-results.md`, records `o2.resume` as **FAILED** —
*"timed out after 15000 ms waiting for the resumed session"*, caused by finding F2: a host-git
read answered that check's own resume create (`POST /v1/sessions`) with a 500, so the control
plane never made the forked session. That the read hit F2's 10 s bound is *inferred* from its
bare `exit status 1: `, not observed — the results document says so, and F2's own definition is
that in this form the cause is never named. A `git config` call that failed without saying why,
not resume, lost this seam its only end-to-end proof. It records the step-18
observation as PENDING OWNER (OD-12), so neither proves anything here yet. Three runs have now
failed it, all recorded there and none claimed, in three different places: run 1 downstream of a
create cancelled half-made (finding F1), not on resume itself; the diagnostic run in the resume
UI, with no `Resume` menu item within 30 s (open observation OO-1); and the document's own run one
step further on, where the `⋯` menu opened, the `Resume` item was clicked, and the create it made
was refused (F2), so no resumed session reached the control plane within 15 s. That last run did
measure the steps before the fork — a UUID `--session-id` on the source, its transcript written
into the persisted store, and the source destroyed — but the fork itself was never reached, and
OO-1's own symptom did not recur. So nothing here is evidence that resume works. Three runs, three
different places. Two have a cause — F1 and F2, neither of them resume itself — and the diagnostic
run's has none: its symptom did not recur, which is not the same as being explained, so that red
stays open against the resume UI. The cheapest way to close the seam is to fix F2's 10 s bound and
re-run `o2.resume`.
**This is a Phase 8 seam as well as a resume one**: a
Kata pod's home is an `emptyDir` (`docs/verification/2026-09-17-phase-1-results.md:67`), which
dies with the pod exactly as a container's home did, so Phase 8 must carry the fix's persistence
into pods — or resume breaks again on the first cloud machine.

### S8.6 — why the guard ignores comments

`client.ts:6-8` names the header in a comment explaining its absence, so the anchor runs over
comment-stripped code. A comment there is only what the language's own lexer calls one —
TypeScript's parser, for `client.ts` — so a `//` inside a string, a template, a regex literal or
JSX text is kept, and a header written after `DEFAULT_BASE_URL`'s URL on the same line fails the
anchor. Before task 26's fix round 3 the stripper blanked the rest of any line after a `//`, a URL
string's included (review 3 I1, mutant M-AUTH), and until its round-4 check it still did after a
`//` or `/*` inside a regex literal, a nested template or JSX text. This note used to accept that
as a narrow false green; `seams.test.ts`'s `codeOnly` test now pins `DEFAULT_BASE_URL`'s line
whole, and each of those forms.

## Carried, not seams

Recorded so they are not mistaken for seams, each with its owner:

- **`pinned` is unbounded across branches** — whichever task next opens `prefsSlice`; the
  cheapest prune is where the poller replaces sessions (open carry 9).
- **An unpin does not survive a layout flip or restart** — the next `PERSIST_VERSION` bump can
  tell *never pinned* from *unpinned* (open carry 10).
- **`observe` drops `dirty` before the write** — Phase 3 (open carry 11; `api/attach.go:276-299`).
- **Contract §4.7's refresh half** — declared undelivered by Phase 2 task 24; its loader asks
  "is this list absent", so an `invalidate(key)` action is all a refresh needs.
- **Where Phase 5's canvas server runs** — the product owner and Phase 8
  (`survey-phase-5-design.md:433-443`).
