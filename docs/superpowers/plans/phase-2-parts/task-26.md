### Task 26: Name the seams Phases 3, 4, 5 and 8 plug into

The master plan gives this task half a line: *"named seams for Phases 3–5 and 8"* (`2026-09-18-phase-2-backend-and-cockpit.md:38`). The phase order it serves is settled — **3 Git, worktrees and convergence; 4 Discover; 5 Design canvas; 8 Kata backend, Terraform and warm pools** (`docs/PROGRAM.md:49-59`, renumbered in `6bcc9af`). A **named seam** is a precise place in Phase 2's shipped code where a later phase plugs in, named so that phase does not have to rediscover it: the file, the interface or the reserved point, what the phase does there, **what it must not do**, and a guard that proves the seam is still where this task said it was.

**Rediscovery is not hypothetical here; it has already happened three times.** The Phase 4 survey spent its §2 re-deriving five seams its own plan had declared, and found all five honoured (`docs/superpowers/planning/2026-09-23-phases-4-5-7/survey-phase-4-discover.md:44-54`) — and then found the sixth, the one that mattered, was never built: the Go contract reserved a slot for writing `CLAUDE.md` before an agent starts *"in prose"*, but *"no hook was built"* (`:97-104`). The substrate survey found `AppShell.tsx` rendering the wrong phase number because `PROGRAM.md` was renumbered eleven minutes after the code last changed and nothing swept the code (`survey-phase-2-substrate.md:48-50`, `:179-185`). And the Kata backend's own header says *"turning it on in Phase 8 is a configuration change and not a rewrite"* (`control-plane/internal/backend/katabe/kata.go:1-4`) — which is false, for a reason nobody had written down: the session-create path always sends a host mount and a working directory (`control-plane/internal/api/sessions.go:330-331`), and `katabe.Create` refuses both (`kata.go:84-89`). **Under `PANACEA_APP_BACKEND=kata`, every session create fails today.** The refusal is correct — a host path on an AKS node is meaningless, and refusing beats *"an empty worktree with no error anywhere"* (`kata.go:75-79`). The defect is the header's claim, and step 5 corrects it to say what Phase 8 actually has to build first: a way to get the worktree into the pod. Each of those three was knowable from the code, and each was found late because nothing named it.

#### What this task produces, and why that combination

Three candidate forms, and the choice is argued rather than assumed.

- **A document alone rots.** It is prose, nothing fails when it goes stale, and the `phase={5}` drift above is exactly a document and a file disagreeing with nothing to notice. `PROGRAM.md` was the document there.
- **Code comments alone cannot fail either.** A comment is prose that happens to live in a `.go` file. Shared shape §3A exists because this phase learnt, repeatedly, that *a comment cannot stand in for code* — five guards in tasks 14 to 24 were red against a correct tree because a comment satisfied or defeated them (`task-24.md:35-42`). A `// SEAM:` marker scattered across twenty files owned by twelve closed tasks is twenty declared departures for no mechanical value.
- **Tests alone say *what* and never *why* or *what not to do*.** A test pins that `worktree.Provider` has one method; it cannot tell Phase 3 that worktree writes belong in a new package and that widening `project`'s read-only whitelist is the mistake.

**So task 26 produces a document and a manifest that keep each other honest**, plus the one code change that turns a certain future conflict into a seam:

1. **`docs/SEAMS.md`** — the human half. One table row per seam: where it is, what the phase does, what it must not do, and the guard. Then notes on the four seams that are gaps rather than extension points. It sits beside `PROGRAM.md` rather than in a dated planning folder because it is **living**: each later phase that consumes or changes a seam edits its row in the same commit.
2. **`desktop/src/seams.test.ts`** — the mechanical half, in the shape `task-24.md:912-1146` proved on the ten defects: **an anchor in the implementation per seam**, read from disk, never a test title and never a comment. It reads Go files as well as TypeScript, because a seam's anchor is a declaration and a declaration is text; and it parses `docs/SEAMS.md` and fails unless the document's seam ids and the manifest's are **the same set**. A seam that moves fails with the document's path in the message. A seam added to the document without an anchor fails. An anchor with no document row fails.
3. **`control-plane/internal/backend/seam_test.go`** — the one seam whose contract is a *type*, not a line: the `Backend` interface Phase 8 plugs into. `dockerbe` and `katabe` already assert at compile time that they satisfy it (`docker.go:43`, `kata.go:25`); that catches an implementation falling behind the interface, and nothing catches the interface itself changing shape with both implementations changed to match. `reflect` does.
4. **The lens split** — gate-zero item **0b** of the planning pass (`dependency-graph-and-order.md:232`), whose owner column reads *"Phase 2 closeout"*, which is this task. `AppShell.tsx`'s `LensRouter` is *"the one guaranteed collision"* (`survey-phase-2-substrate.md:15-17`): Phases 3, 4, 5 and 7 each replace a different arm of one `switch` and each add an `import` to one contiguous block, which *"is four conflicts"* (`dependency-graph-and-order.md:143-145`). Five one-component files, each rendering exactly the placeholder it renders today, close `AppShell.tsx` for the programme and give every later phase a file nobody else opens. **The `phase={5}` on the deploy arm is not planned here**: task 23 owns that fix, and step 1 checks it landed as `phase={7}`.

And one ruling that the phase deferred to its own end: **open carry 12, `proxy.Relay` has no production caller** — *"either wire it or delete it, but do not ship a relay nothing calls"* (`.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-d-open-carries.md:101-102`). It is a seam question: the attach relay is the path Phase 8's pods are reached through, and a seam with two public names is a seam the next phase has to choose between. `api/attach.go:216` calls `proxy.RelayHooked`; `proxy.Relay` (`proxy.go:26-30`) is a three-line wrapper with one test caller. **Step 5 re-proves that, with an anchored grep, at the moment of deletion** — not from this plan's reading — and only then deletes it, making `RelayHooked` the one relay.

**This task changes no behaviour.** The lens stubs render byte-for-byte what `AppShell.tsx` renders today, so task 18's and task 20's `AppShell.test.tsx` passes unchanged — that is step 3's check. The relay deletion removes a function nothing calls. `katabe`'s header correction is a comment. Everything else is a document and two test files.

**A note on the expected-output blocks, as in task 24.** No command in this plan has been run. Where a transcript appears it is the shape the tool prints; match on the named assertion.

**Files:**
- `docs/SEAMS.md` — the seams, one row each, with the guard that pins it (new)
- `desktop/src/seams.test.ts` — the seam manifest: anchors across `desktop/` and `control-plane/`, the contract §2 byte gate as a test, and the document ↔ manifest sync (new)
- `control-plane/internal/backend/seam_test.go` — the `Backend` method set and the two `Kind` strings, by reflection (new)
- `desktop/src/components/lens/DiscoverLens.tsx`, `DesignLens.tsx`, `FilesLens.tsx`, `MergeLens.tsx`, `DeployLens.tsx` — one stub per placeholder arm, each rendering today's `LensPlaceholder` unchanged (new, five files)
- `desktop/src/components/AppShell.tsx` — five arms return their lens component; the `LensPlaceholder` import leaves. **A declared departure**: the file is task 18's and task 23's, both closed; the edit is gate-zero 0b's, whose owner is Phase 2 closeout (modify)
- `control-plane/internal/proxy/proxy.go` — `Relay` deleted; its doc sentence moves onto `RelayHooked` (modify)
- `control-plane/internal/proxy/proxy_test.go` — its one `proxy.Relay(…)` call becomes `proxy.RelayHooked(…, proxy.Hooks{})` (modify)
- `control-plane/internal/backend/backend.go` — the package comment's `proxy.Relay` becomes `proxy.RelayHooked` (modify)
- `control-plane/internal/backend/katabe/kata.go` — the package comment's *"a configuration change and not a rewrite"* replaced with what Phase 8 must build first. Comment only (modify)

**Interfaces:**
- Produces:
  - `docs/SEAMS.md` — thirty-two seams with ids `SX.1`–`SX.10` (every later phase, or no phase yet), `S3.1`–`S3.5`, `S4.1`–`S4.4`, `S5.1`–`S5.2` and `S8.1`–`S8.11`. It was twenty-eight: the Resume fix and the sandbox git + CA fix added four more mounts that `katabe` refuses, `S8.8`–`S8.11`, and each is recorded in Go contract §2.6 and in `backend.Mount`'s doc comment. The id is the key a later plan cites: *"this task consumes S4.1 and changes S4.2"*
  - `npm test -- src/seams.test.ts` and `go test ./control-plane/internal/backend/ -run Seam -count=1` — the two commands a later phase runs to check every seam is where `docs/SEAMS.md` says
  - `DiscoverLens`, `DesignLens`, `FilesLens`, `MergeLens`, `DeployLens` — each `(): JSX.Element`, each in `desktop/src/components/lens/`
- Consumes: task 25's results document, cited in `docs/SEAMS.md`'s preamble as the evidence that the substrate these seams sit in was running when they were named; task 24's `defects.test.tsx` shape (anchors, `codeOnly`), copied rather than imported, because a test file importing another test file's internals is how two suites start failing together; task 15's contract §2 byte gate (`task-15.md:276-279`), re-expressed as a test
- Required by: every later phase plan. `dependency-graph-and-order.md:315` gates any edit to `AppShell.tsx` on *"after the lens-seam task (0b)"*; after this task that edit is unnecessary
- **Adds no selector, no fixture, no store key and no route.** A seam test that needed a new selector would be a seam test that changed the thing it pins
- **Does not decide anything the planning pass left to the orchestrator or the owner.** Three seams are gaps with a pending decision — who calls `Materialise` (decision 0c, `dependency-graph-and-order.md:233`), who owns the `Files` lens (open carry 8), and how Kata receives a worktree (Phase 8's first problem). `docs/SEAMS.md` names each, cites the decision, and pins the **current** state with a tripwire anchor, so the day the decision lands the manifest fails and the document is updated with it. That is the whole of this task's part in each. The four mount seams `S8.8`–`S8.11` are the same Phase 8 problem as `S8.3`, one mount each, and are pinned the same way
- **Runs after the resume fix** (`b4be570`, round 1 `2e3db79`) **and the sandbox git + CA fix** (`2d6d95b`…`31576b7`), both landed ahead of task 25. Seam SX.10 names the mechanism the Resume fix built — Claude's own session id, a resume that forks it, and a conversation store that outlives its sandbox — and pins it. Step 1 (g) checks it landed

**Every shell command below runs through `~/.claude/tools/claudezsh/claudezsh '<command>'`**, per the device-wide rule; the blocks show them bare.

- [ ] **Step 1: Check what this task assumes**

Every one of these belongs to another task. If one is false, **stop and report it to the orchestrator**, per `task-24.md:33`. Every guard is anchored (shared shape §3A).

```bash
# (a) Tasks 24 and 25 landed. This task names seams in a substrate that has RUN; a seam
#     named in code that has only ever passed vitest is a claim, not a seam.
ls docs/verification/*-phase-2-results.md                                   # one file
test -f desktop/src/defects.test.tsx && test -f desktop/src/styles/designsystem.test.ts && echo "task 24: present"
grep -c '2\.1\.267' control-plane/internal/harness/harness.go control-plane/internal/harness/argv.go   # 0 and 0 (task 25 step 5)

# (b) Task 23 fixed the deploy arm's phase, and nothing else in LensRouter moved.
cd desktop
grep -cE '^\s+phase=\{7\}$'                      src/components/AppShell.tsx   # 1 — deploy
grep -cE '^\s+phase=\{5\}$'                      src/components/AppShell.tsx   # 1 — design only
grep -cE '^\s+phase=\{3\}$'                      src/components/AppShell.tsx   # 2 — files, merge
grep -cE '^\s+phase=\{4\}$'                      src/components/AppShell.tsx   # 1 — discover
grep -cE '^\s+<LensPlaceholder$'                 src/components/AppShell.tsx   # 5
grep -cE '^\s+return <DevelopLens />;$'          src/components/AppShell.tsx   # 1
ls src/components/lens/                                                     # LensPlaceholder.tsx only
cd ..

# (c) Nothing this task creates exists yet.
test ! -e docs/SEAMS.md && test ! -e desktop/src/seams.test.ts \
  && test ! -e control-plane/internal/backend/seam_test.go && echo "task 26: nothing yet"

# (d) The Kata create-path gap is as the preamble states it. Both halves. Since the resume fix
#     and the git + CA fix, the create path builds its mounts in ONE function, sessionMounts:
#     the worktree first, then up to five more, each a Phase 8 seam (S8.8-S8.11).
grep -cE '^\tmounts := \[\]backend\.Mount\{\{HostPath: worktreePath, ContainerPath: sessionWorkDir\}\}$' \
  control-plane/internal/api/sessions.go                                    # 1
grep -cE '^\t\tMounts:\s+sandboxMounts,$'      control-plane/internal/api/sessions.go         # 2 — create and restart
grep -cE '^\t\tmounts = append\(mounts, ' control-plane/internal/api/sessions.go               # 6 — S8.8 (2), S8.9, S8.10, S8.11 (2)
grep -cE '^\tif len\(spec\.Mounts\) > 0 \{$'   control-plane/internal/backend/katabe/kata.go   # 1
grep -cE '^\tif spec\.WorkDir != "" \{$'       control-plane/internal/backend/katabe/kata.go   # 1
grep -c  'is a configuration change and not a rewrite' control-plane/internal/backend/katabe/kata.go   # 1 — step 5 corrects it

# (e) proxy.Relay has exactly one caller, and it is a test. Step 5 re-runs this at deletion.
grep -rnE '\bproxy\.Relay\(' control-plane --include=*.go                   # one line: proxy_test.go:57
grep -rnE '\bproxy\.RelayHooked\(' control-plane --include=*.go | grep -v _test.go   # one line: attach.go:216

# (f) Nothing this task edits is dirty.
git status --porcelain -- desktop/src/components/AppShell.tsx desktop/src/components/lens \
  control-plane/internal/proxy control-plane/internal/backend/backend.go \
  control-plane/internal/backend/katabe/kata.go docs/SEAMS.md               # expect no output

# (g) The resume fix landed (it precedes task 25). Seam SX.10 pins what it built. The desktop
#     still sends the COCKPIT id (`resume: from.id`, count 1) on purpose, and the control
#     plane resolves it (UI contract §4.4's resume note, Go contract §3.9), so resume is
#     checked server-side and never by that client line.
grep -cE '^[^/]*"--session-id"'   control-plane/internal/harness/argv.go    # >= 1
grep -cE '^[^/]*"--fork-session"' control-plane/internal/harness/argv.go    # >= 1
grep -cE '^func \(a \*API\) sessionResolveResume\(' control-plane/internal/api/sessions.go   # 1
grep -cE '^RUN mkdir -p /home/dev/\.claude/projects$' images/sandbox/Dockerfile               # 1
```

**The declaration that makes the conversation store outlive its sandbox is a mount**, and the Resume fix put it in `sessionMounts` (`control-plane/internal/api/sessions.go`): `mounts = append(mounts, backend.Mount{HostPath: conversationStore, ContainerPath: sessionConversationDir})`, with `const sessionConversationDir = "/home/dev/.claude/projects"`. The image makes the mount point with `RUN mkdir -p /home/dev/.claude/projects` (`images/sandbox/Dockerfile`), so `~/.claude` stays owned by `dev`. Step 2's `SX.10` entry anchors both, and step 6's `SX.10` row names both. Re-derived at `a38c268`; the entry carries no line number, because this file has moved under every fix since.

`grep -E` does not understand `\t` on every platform, and on this machine it does not; if (d) prints `0` for a line you can see in the file, re-run it with `LC_ALL=C.UTF-8 grep -cP` (plain `grep -P` refuses this machine's default locale: *"-P supports only unibyte and UTF-8 locales"*) before concluding anything. Every (d) count above was checked at `a38c268` with the same JavaScript regexes `seams.test.ts` uses. **If (b) finds `phase={5}` twice**, task 23's fix has not landed — do not make it here; the orchestrator was explicit that it is task 23's. **If (d) finds either Kata line genuinely gone**, somebody has started Phase 8's worktree delivery, and seam S8.3 below must be rewritten from the code as it now stands before anything else is written. **If (g) shows the resume fix absent** (it found it present at `a38c268`), stop: `SX.10` would pin the broken mechanism the fix replaces, and the manifest would go red the day the fix lands.

- [ ] **Step 2: Write the failing seam manifest**

The shape is `task-24.md`'s defect manifest, and the reasons carry over whole. **It keys on implementation anchors, not on test titles** — titles are prose that authors reword (`task-24.md:914-916`). **Every `present` anchor is a declaration or a call, never a comment**, and every `absent` anchor runs against `codeOnly(...)` so a file may *discuss* the thing it must not do (`task-24.md:935-953`). What is new is the last test: **the document and the manifest must name the same set of seams**, which is what stops `docs/SEAMS.md` becoming the next document nothing checks.

It reads Go as well as TypeScript. That is deliberate and it is not a second implementation of anything: a seam's anchor is a declaration, a declaration is a line of text, and the Go-side guards in shared shape §3A are `grep`s over the same lines. Behaviour that is not a line — the `Backend` method set — is pinned in Go, by step 4.

Two line-ending facts, both from this checkout. It mixes CRLF and LF with `core.autocrlf = true` and no `.gitattributes` (`survey-phase-2-substrate.md:69-70`), so every file is normalised to `\n` before a multi-line anchor is tried. And this file never contains three backticks in a row, because the plan quoting it would close its own code fence on them; the contract's fence is built with `repeat(3)`.

Create `desktop/src/seams.test.ts`:

```ts
// Task 26. The named seams: where Phases 3, 4, 5 and 8 plug into what Phase 2 shipped.
// docs/SEAMS.md is the human half — what each seam is, what the phase does there, and
// what it must not do. This file is the mechanical half: an anchor per seam, read from
// disk, so a seam that moves fails HERE, with the document's path in the message, instead
// of being rediscovered by the phase that needed it.
//
// A seam that changes is not a failure to work around. Change the anchor here and the row
// in docs/SEAMS.md in the same commit; the last test below fails until both agree.

import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/** The repository root. This file lives at desktop/src/seams.test.ts. */
const ROOT = fileURLToPath(new URL("../../", import.meta.url));
const at = (rel: string): string => join(ROOT, rel);
/** Normalised to \n: this checkout mixes CRLF and LF (survey-phase-2-substrate.md:69-70). */
const text = (rel: string): string => readFileSync(at(rel), "utf8").replace(/\r\n/g, "\n");

const DOC = "docs/SEAMS.md";
const FENCE = "`".repeat(3);

/** Comments blanked, newlines kept — for `absent` anchors only. The same rule and the same
 *  naivety as defects.test.tsx's codeOnly: it also blanks a // inside a string, which is why
 *  no `absent` anchor below is run over a file whose strings carry a URL. Go and TypeScript
 *  share both comment forms, so one function serves both. */
function codeOnly(source: string): string {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .replace(/\/\/[^\n]*/g, "");
}

/** Non-test TypeScript under a directory, repo-relative and forward-slashed. */
function sourcesUnder(dir: string): string[] {
  const out: string[] = [];
  const walk = (abs: string): void => {
    for (const entry of readdirSync(abs, { withFileTypes: true })) {
      const full = join(abs, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
        out.push(relative(ROOT, full).split(sep).join("/"));
      }
    }
  };
  walk(at(dir));
  return out.filter((f) => !f.startsWith("desktop/src/test/"));
}

interface Anchor {
  file: string;
  /** A declaration or call the seam cannot exist without. Matched against the raw source. */
  present?: RegExp;
  /** Code whose presence would mean the seam changed shape. Matched against codeOnly(). */
  absent?: RegExp;
}

interface TreeRule {
  /** Every non-test source under this directory must not contain `absent`, in code, */
  dir: string;
  absent: RegExp;
  /** except in these files. */
  except: string[];
}

interface Seam {
  /** The row id in docs/SEAMS.md. */
  id: string;
  name: string;
  anchors: Anchor[];
  trees?: TreeRule[];
  /** Behaviour tests the seam's contract rests on, by declaration, not by title. */
  tests?: { file: string; decl: RegExp }[];
}
```

The seam list. It is one array literal across the next four blocks; do not close it early. The five lens seams are generated, because five hand-written copies of one rule is how one of them ends up checking the wrong phase:

```ts
/** The five placeholder arms and the phase each renders today. Deploy is 7 after task 23. */
const LENSES: ReadonlyArray<[name: string, lens: string, phase: number]> = [
  ["Discover", "discover", 4],
  ["Design", "design", 5],
  ["Files", "files", 3],
  ["Merge", "merge", 3],
  ["Deploy", "deploy", 7],
];
const lensFile = (name: string): string => `desktop/src/components/lens/${name}Lens.tsx`;
const APPSHELL = "desktop/src/components/AppShell.tsx";

const SEAMS: Seam[] = [
  /* ── every later phase ──────────────────────────────────────────────────────────── */
  {
    id: "SX.1",
    name: "one lens, one file; AppShell.tsx is closed",
    anchors: [
      ...LENSES.flatMap(([name, lens, phase]): Anchor[] => [
        { file: lensFile(name), present: new RegExp(`^export function ${name}Lens\\(\\): JSX\\.Element \\{$`, "m") },
        { file: lensFile(name), present: new RegExp(`^\\s+lens="${lens}"$`, "m") },
        { file: lensFile(name), present: new RegExp(`^\\s+phase=\\{${phase}\\}$`, "m") },
        { file: APPSHELL, present: new RegExp(`^\\s+return <${name}Lens />;$`, "m") },
      ]),
      { file: APPSHELL, present: /^\s+return <DevelopLens \/>;$/m },
      // The shell renders no placeholder of its own any more: a lens that needs one owns it.
      { file: APPSHELL, absent: /\bLensPlaceholder\b/ },
    ],
    tests: [{ file: "desktop/src/components/AppShell.test.tsx", decl: /\bit\s*\(/ }],
  },
  {
    id: "SX.2",
    name: "the domain model is byte-gated against contract §2",
    anchors: [{ file: "desktop/src/store/types.ts", present: /^export type Lens =/m }],
    // The gate itself is the dedicated test at the bottom of this file.
  },
  {
    id: "SX.3",
    name: "the persisted store shape, at version 1",
    anchors: [
      { file: "desktop/src/store/store.ts", present: /^export const PERSIST_VERSION = 1;$/m },
      { file: "desktop/src/store/store.ts", present: /^export const PERSIST_KEY = "panacea-app\.cockpit";$/m },
      { file: "desktop/src/store/store.ts", present: /^export const partialize = \(s: CockpitState\): Persisted => \(\{$/m },
    ],
  },
  {
    id: "SX.4",
    name: "routes register through the recorder, and the audit sees every one",
    anchors: [
      { file: "control-plane/internal/api/api.go", present: /^func \(rs \*routes\) HandleFunc\(pattern string, h func\(http\.ResponseWriter, \*http\.Request\)\) \{$/m },
    ],
    tests: [
      { file: "control-plane/internal/api/routes_audit_test.go", decl: /^func TestNoRegisteredGetCanChangeState\(/m },
      { file: "control-plane/internal/api/routes_audit_test.go", decl: /^func TestNoFileRegistersOnTheRawMuxBehindTheRecorder\(/m },
    ],
  },
  {
    id: "SX.5",
    name: "each phase's state lives under statepath.Dir(sub)",
    anchors: [{ file: "control-plane/internal/statepath/statepath.go", present: /^func Dir\(sub string\) string \{$/m }],
  },
  {
    id: "SX.6",
    name: "the command palette is one registry",
    anchors: [
      { file: "desktop/src/components/palette/commands.ts", present: /^export interface Command \{$/m },
      { file: "desktop/src/components/palette/commands.ts", present: /^export function buildCommands\(state: CockpitState\): Command\[\] \{$/m },
    ],
  },
  {
    id: "SX.7",
    name: "every control-plane error is classified once; the prose stays at the call site",
    anchors: [
      { file: "desktop/src/api/problems.ts", present: /^export type ProblemKind =$/m },
      { file: "desktop/src/api/problems.ts", present: /^export function classify\(error: ApiError\): ProblemKind \{$/m },
    ],
  },
  {
    id: "SX.8",
    name: "one heartbeat for the whole window",
    anchors: [
      { file: "desktop/src/hooks/heartbeat.ts", present: /^export const BEAT_MS = 2400;$/m },
      { file: "desktop/src/hooks/heartbeat.ts", present: /^export function beatDelayStyle\(\)/m },
      { file: "desktop/src/hooks/heartbeat.ts", present: /^export function useHeartbeat\(\): number \{$/m },
    ],
  },
  {
    id: "SX.9",
    name: "--need and --drift are defined and unused, for the phase that can derive them",
    anchors: [
      { file: "desktop/src/styles/tokens.css", present: /^\s+--need:\s+oklch\(/m },
      { file: "desktop/src/styles/tokens.css", present: /^\s+--drift:\s+oklch\(/m },
    ],
    tests: [{ file: "desktop/src/styles/designsystem.test.ts", decl: /defines --drift and references it nowhere/ }],
  },
  {
    id: "SX.10",
    name: "resume by Claude's own session id, as a fork, from a conversation store that outlives its sandbox",
    anchors: [
      // The Resume fix (b4be570, 2e3db79). Claude Code is started with its own UUID, and a
      // resume forks it under a new one: the only pairing the CLI accepts.
      { file: "control-plane/internal/harness/argv.go", present: /^[^/\n]*"--session-id"/m },
      { file: "control-plane/internal/harness/argv.go", present: /^[^/\n]*"--fork-session"/m },
      // The cockpit's id still travels from the desktop ON PURPOSE (client.ts sends
      // `resume: from.id`); the control plane resolves it to the Claude UUID here.
      { file: "control-plane/internal/api/sessions.go", present: /^func \(a \*API\) sessionResolveResume\(/m },
      // The store that outlives the sandbox: one mount of <state>/conversations/local/<worktree>/
      // at the one persisted directory, which the image makes as the agent user.
      { file: "control-plane/internal/api/sessions.go", present: /^const sessionConversationDir = "\/home\/dev\/\.claude\/projects"$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: conversationStore, ContainerPath: sessionConversationDir\}\)$/m },
      { file: "images/sandbox/Dockerfile", present: /^RUN mkdir -p \/home\/dev\/\.claude\/projects$/m },
    ],
    tests: [
      { file: "desktop/e2e/phase2.e2e.mjs", decl: /^check\("o2\.resume",/m },
      { file: "control-plane/internal/api/sessions_resume_test.go", decl: /^func TestResumePassesTheSourcesConversationNotTheCockpitID\(/m },
      { file: "control-plane/internal/api/sessions_resume_test.go", decl: /^func TestCreateMountsOnlyThisWorktreesConversationStore\(/m },
      { file: "control-plane/internal/harness/argv_test.go", decl: /^func TestBuildRefusesAForkThatReusesTheSourceID\(/m },
    ],
  },
```

`SX.9`'s test entry is the one place a `decl` matches a test's *title* rather than a declaration, and it is deliberate: `designsystem.test.ts` is a list of `it(...)` calls with no named functions (`task-24.md:254-257`), and the title is what a Phase 3 implementer will search for the day that test goes red on their first `var(--drift)`. If task 24's title is reworded, this entry is reworded with it.

The Phase 3 and Phase 4 seams, continuing the same array:

```ts
  /* ── Phase 3: git, worktrees, coordination and convergence ──────────────────────── */
  {
    id: "S3.1",
    name: "git writes live outside package project, whose whitelist stays read-only",
    anchors: [
      { file: "control-plane/internal/project/project.go", present: /^func \(s \*Scanner\) Path\(ctx context\.Context, worktreeID string\) \(string, error\) \{$/m },
    ],
    tests: [{ file: "control-plane/internal/project/project_test.go", decl: /^func TestEveryGitCommandIsReadOnly\(/m }],
  },
  {
    id: "S3.2",
    name: "every worktree resolves through worktree.Provider",
    anchors: [
      { file: "control-plane/internal/worktree/worktree.go", present: /^type Provider interface \{\n\tPath\(ctx context\.Context, worktreeID string\) \(string, error\)\n\}$/m },
    ],
  },
  {
    id: "S3.3",
    name: "the worktree ready flag: the one thing merge may gate on",
    anchors: [
      { file: "control-plane/internal/api/catalog.go", present: /\{"PATCH \/v1\/projects\/\{projectId\}\/worktrees\/\{worktreeId\}", http\.HandlerFunc\(a\.handlePatchWorktree\)\},/ },
      { file: "desktop/src/store/types.ts", present: /^\s+ready\?: boolean;$/m },
      { file: "desktop/src/api/client.ts", present: /^\s+setWorktreeReady: async \(projectId, worktreeId, ready\) =>$/m },
    ],
  },
  {
    id: "S3.4",
    name: "reordering a filtered list goes through the visible-position helpers",
    anchors: [
      { file: "desktop/src/lib/immutable.ts", present: /^export function moveWithinVisible\b/m },
      { file: "desktop/src/lib/immutable.ts", present: /^export function idAtVisiblePosition\b/m },
    ],
    tests: [{ file: "desktop/src/defects.test.tsx", decl: /^\s+n: 10,$/m }],
  },
  {
    id: "S3.5",
    name: "the Files and Merge lenses — phase 3 by the code, unassigned by PROGRAM.md for Files",
    anchors: [
      { file: lensFile("Files"), present: /^\s+phase=\{3\}$/m },
      { file: lensFile("Merge"), present: /^\s+phase=\{3\}$/m },
    ],
  },

  /* ── Phase 4: Discover ──────────────────────────────────────────────────────────── */
  {
    id: "S4.1",
    name: "Session.contextRef: carried everywhere, written and read by no surface yet",
    anchors: [
      { file: "control-plane/internal/registry/session.go", present: /^\tContextRef string `json:"contextRef"`$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tContextRef:\s+req\.ContextRef,$/m },
      { file: "desktop/src/store/types.ts", present: /^\s+contextRef: string;$/m },
      { file: "desktop/src/store/defaults.ts", present: /^\s+contextRef\?: string;$/m },
      // Resume and fork both carry it — the case most likely to have been missed.
      { file: "desktop/src/api/client.ts", present: /contextRef: from\.contextRef,[\s\S]+contextRef: from\.contextRef,/ },
    ],
    // The tripwire. No component touches it today (survey-phase-2-substrate.md:355-358); the
    // day Phase 4 renders or writes it from a surface, this goes red and the row is updated.
    trees: [{ dir: "desktop/src/components", absent: /\bcontextRef\b/, except: [] }],
  },
  {
    id: "S4.2",
    name: "the Materialise window in handleSessionCreate — reserved in prose, not built",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^func \(a \*API\) handleSessionCreate\(w http\.ResponseWriter, r \*http\.Request\) \{$/m },
      // Decision 0c is open. The day api.Deps grows a materialiser, this goes red.
      { file: "control-plane/internal/api/api.go", absent: /Materiali[sz]/ },
    ],
  },
  {
    id: "S4.3",
    name: "append-or-replace edits go through replaceAt / replaceById",
    anchors: [
      { file: "desktop/src/lib/immutable.ts", present: /^export function replaceAt</m },
      { file: "desktop/src/lib/immutable.ts", present: /^export function replaceById</m },
    ],
    tests: [{ file: "desktop/src/defects.test.tsx", decl: /^\s+n: 7,$/m }],
  },
  {
    id: "S4.4",
    name: "display names, worktree paths and the /api/v1 prefix Discover's routes already use",
    anchors: [
      { file: "control-plane/internal/project/project.go", present: /^func \(s \*Scanner\) ProjectName\(projectID string\) string \{/m },
      { file: "control-plane/internal/project/project.go", present: /^func \(s \*Scanner\) BranchName\(branchID string\) string \{/m },
      { file: "control-plane/internal/api/api.go", present: /guarded\.Handle\("\/api\/v1\/", http\.StripPrefix\("\/api", guarded\)\)/ },
    ],
  },
```

`S4.2`'s `absent` is run over `codeOnly(api.go)`, and that matters: the planning pass's own documents discuss `Materialise` at length, and a future comment in `api.go` explaining why no materialiser exists yet must not trip it. `S3.4` and `S4.3` cite `defects.test.tsx`'s rows by their `n:` line rather than restating the anchors — task 24's manifest already pins those four exports, and one implementation of each check is the rule `task-24.md:92` gives for why its audit imports task 14's helpers instead of re-writing them.

The Phase 5 and Phase 8 seams, closing the array:

```ts
  /* ── Phase 5: the Design canvas ─────────────────────────────────────────────────── */
  {
    id: "S5.1",
    name: "one push-free transport: the 2 s poll and the attach sockets, nothing else",
    anchors: [
      { file: "desktop/src/api/poller.ts", present: /^export const POLL_INTERVAL_MS = 2_000;$/m },
      { file: "desktop/src/api/client.ts", present: /^\s+const makeSocket = opts\?\.webSocket \?\? \(\(url: string\) => new WebSocket\(url\)\);$/m },
    ],
    // The tripwire. Phase 5's canvas brings its own room (survey-phase-5-design.md:444-448);
    // the day a second transport opens, this goes red and the row says where it lives.
    trees: [{ dir: "desktop/src", absent: /\bnew (?:EventSource|WebSocket)\(/, except: ["desktop/src/api/client.ts"] }],
  },
  {
    id: "S5.2",
    name: "a lens is handed a fixed box and owns its own scroll",
    anchors: [
      { file: "desktop/src/components/shell.css", present: /^\.shell-lens \{\n  min-block-size: 0;\n  overflow: hidden;\n\}$/m },
      { file: lensFile("Design"), present: /^export function DesignLens\(\): JSX\.Element \{$/m },
    ],
  },

  /* ── Phase 8: Kata backend, Terraform and warm pools ────────────────────────────── */
  {
    id: "S8.1",
    name: "backend.Backend: the seven methods katabe already implements",
    anchors: [
      { file: "control-plane/internal/backend/katabe/kata.go", present: /^var _ backend\.Backend = \(\*Backend\)\(nil\)$/m },
      { file: "control-plane/internal/backend/dockerbe/docker.go", present: /^var _ backend\.Backend = \(\*Backend\)\(nil\)$/m },
    ],
    tests: [
      { file: "control-plane/internal/backend/seam_test.go", decl: /^func TestSeamBackendIsTheSevenMethodsBothImplementationsServe\(/m },
      { file: "control-plane/internal/backend/seam_test.go", decl: /^func TestSeamBackendKindsAreTheTwoPersistedStrings\(/m },
    ],
  },
  {
    id: "S8.2",
    name: "PANACEA_APP_BACKEND=kata selects katabe, with in-cluster credentials only",
    anchors: [
      { file: "control-plane/cmd/api/config.go", present: /^\tenvBackend\s+= "PANACEA_APP_BACKEND"$/m },
      { file: "control-plane/cmd/api/main.go", present: /^\tcase string\(backend\.KindKata\):$/m },
      { file: "control-plane/cmd/api/main.go", present: /^func kubeClient\(kubeconfig string\) \(\*kubernetes\.Clientset, error\) \{$/m },
    ],
  },
  {
    id: "S8.3",
    name: "the gap: the create path always mounts a host worktree, and katabe refuses one",
    anchors: [
      // sessionMounts builds every create's and restart's mounts, the worktree first.
      { file: "control-plane/internal/api/sessions.go", present: /^\tmounts := \[\]backend\.Mount\{\{HostPath: worktreePath, ContainerPath: sessionWorkDir\}\}$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tMounts:\s+sandboxMounts,$/m },
      { file: "control-plane/internal/backend/katabe/kata.go", present: /^\tif len\(spec\.Mounts\) > 0 \{$/m },
      { file: "control-plane/internal/backend/katabe/kata.go", present: /^\tif spec\.WorkDir != "" \{$/m },
    ],
    tests: [
      { file: "control-plane/internal/api/sessions_test.go", decl: /^func TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv\(/m },
      { file: "control-plane/internal/backend/katabe/kata_test.go", decl: /^func TestCreateRejectsMountsAndCreatesNoPod\(/m },
      { file: "control-plane/internal/backend/katabe/kata_test.go", decl: /^func TestCreateRejectsAWorkDir\(/m },
    ],
  },
  {
    id: "S8.4",
    name: "the RuntimeClass name, hard-coded and never observed on a cluster",
    anchors: [{ file: "control-plane/internal/sandbox/manager.go", present: /^const runtimeClass = "kata-mshv-vm-isolation"$/m }],
  },
  {
    id: "S8.5",
    name: "isolation is reported per machine, and the rail badges anything that is not a microVM",
    anchors: [
      { file: "control-plane/internal/api/catalog.go", present: /^func isolationFor\(k backend\.Kind\) string \{$/m },
      { file: "desktop/src/components/rail/MachineChip.tsx", present: /\{machine\.isolation === "microvm" \? null : \(/ },
    ],
  },
  {
    id: "S8.6",
    name: "no Authorization header on the desktop hop until Phase 8 adds its server half",
    anchors: [{ file: "desktop/src/api/client.ts", absent: /Authorization/ }],
  },
  {
    id: "S8.7",
    name: "one relay: proxy.RelayHooked",
    anchors: [
      { file: "control-plane/internal/proxy/proxy.go", present: /^func RelayHooked\(ctx context\.Context, client, upstream \*websocket\.Conn, h Hooks\) error \{$/m },
      { file: "control-plane/internal/proxy/proxy.go", absent: /^func Relay\(/m },
      { file: "control-plane/internal/api/attach.go", present: /\bproxy\.RelayHooked\(r\.Context\(\), client, upstream, / },
    ],
  },
  /* The four mounts after the worktree, each refused by katabe's `len(spec.Mounts) > 0`
   * (S8.3's anchor), each a thing Phase 8 must provide another way: Go contract §2.6 and
   * backend.Mount's doc comment record every one. One seam per mount, so the day Phase 8
   * delivers one, exactly one row moves. */
  {
    id: "S8.8",
    name: "the conversation store (and a fork's carried transcript): a volume per user and worktree that outlives the pod",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: conversationStore, ContainerPath: sessionConversationDir\}\)$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, carried\)$/m },
    ],
    tests: [
      { file: "control-plane/internal/api/sessions_resume_test.go", decl: /^func TestCreateMountsOnlyThisWorktreesConversationStore\(/m },
      { file: "control-plane/internal/api/sessions_fork_test.go", decl: /^func TestAForkIsShownAStagedReadOnlyCopyOfTheSourcesTranscript\(/m },
    ],
  },
  {
    id: "S8.9",
    name: "the CA bundle: read-only at /etc/panacea-app/ca-bundle.pem, named by NODE_EXTRA_CA_CERTS",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^const sessionCABundlePath = "\/etc\/panacea-app\/ca-bundle\.pem"$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: caBundle, ContainerPath: sessionCABundlePath, ReadOnly: true\}\)$/m },
    ],
    tests: [{ file: "control-plane/internal/api/sessions_sandboxenv_test.go", decl: /^func TestACABundleIsMountedReadOnlyAndNamedByNodeExtraCACerts\(/m }],
  },
  {
    id: "S8.10",
    name: "a primary worktree's .git/worktrees, read-only over itself (may vanish under OD-2's one clone per worktree)",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^const sessionWorktreeAdminDir = sessionWorkDir \+ "\/\.git\/worktrees"$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: worktreeAdmin, ContainerPath: sessionWorktreeAdminDir, ReadOnly: true\}\)$/m },
    ],
    tests: [{ file: "control-plane/internal/api/sessions_sandboxenv_test.go", decl: /^func TestAPrimaryWorktreesLinkedWorktreeMetadataIsReadOnlyInTheSandbox\(/m }],
  },
  {
    id: "S8.11",
    name: "the host git's global excludes and attributes files, read-only, from the user's own config",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: git\.excludes\.mount, ContainerPath: sessionGitExcludesPath, ReadOnly: true\}\)$/m },
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tmounts = append\(mounts, backend\.Mount\{HostPath: git\.attributes\.mount, ContainerPath: sessionGitAttributesPath, ReadOnly: true\}\)$/m },
    ],
    tests: [{ file: "control-plane/internal/api/sessions_gitfiles_test.go", decl: /^func TestTheHostsGlobalGitFilesReachTheSandboxReadOnlyAndOnlyWhenTheHostsGitReadsThem\(/m }],
  },
];
```

**`S8.6`'s `absent` is safe under `codeOnly` for a reason worth checking rather than trusting.** `client.ts` has URL strings in it, and `codeOnly` blanks everything after a `//` — including the `//` inside `"http://127.0.0.1:8730"`. That can only *hide* a match, never create one, so the failure mode is a false green: an `Authorization` header written on the same line as a URL, after it. `client.ts:6-8` names the header in a comment, which is why the raw form is not used. Accept the narrow false-green over a permanent false red; the S8.6 row in `docs/SEAMS.md` says so.

And the six tests, appended to the same file:

```ts
describe("the named seams — docs/SEAMS.md", () => {
  it("names every seam once, with an id of the form the document uses", () => {
    const ids = SEAMS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.filter((id) => !/^S[X3458]\.\d+$/.test(id))).toEqual([]);
  });

  it("still has every seam's anchor where the document says it is", () => {
    const moved: string[] = [];
    for (const seam of SEAMS) {
      for (const a of seam.anchors) {
        if (!existsSync(at(a.file))) {
          moved.push(`${seam.id} (${seam.name}): ${a.file} does not exist`);
          continue;
        }
        const source = text(a.file);
        if (a.present !== undefined && !a.present.test(source)) {
          moved.push(`${seam.id} (${seam.name}): ${a.file} lost ${a.present}`);
        }
        if (a.absent !== undefined && a.absent.test(codeOnly(source))) {
          moved.push(`${seam.id} (${seam.name}): ${a.file} gained ${a.absent}`);
        }
      }
    }
    // Each line names a seam. Fix the code — or, if the seam genuinely moved, change the
    // anchor here AND its row in docs/SEAMS.md in the same commit.
    expect(moved).toEqual([]);
  });

  it("still has every test a seam's contract rests on", () => {
    const missing: string[] = [];
    for (const seam of SEAMS) {
      for (const t of seam.tests ?? []) {
        if (!existsSync(at(t.file))) missing.push(`${seam.id}: ${t.file} does not exist`);
        else if (!t.decl.test(text(t.file))) missing.push(`${seam.id}: ${t.file} lost ${t.decl}`);
      }
    }
    expect(missing).toEqual([]);
  });

  it("keeps every tree-wide tripwire", () => {
    const tripped: string[] = [];
    for (const seam of SEAMS) {
      for (const rule of seam.trees ?? []) {
        for (const file of sourcesUnder(rule.dir)) {
          if (rule.except.includes(file)) continue;
          if (rule.absent.test(codeOnly(text(file)))) {
            tripped.push(`${seam.id} (${seam.name}): ${file} matches ${rule.absent}`);
          }
        }
      }
    }
    // A trip is not necessarily a defect: it is usually the later phase arriving. Update
    // the seam's row in docs/SEAMS.md to say where the thing now lives, then the rule.
    expect(tripped).toEqual([]);
  });

  it("SX.2: types.ts is byte-identical to contract §2, line endings aside", () => {
    // task-15.md:276-279's awk gate, as a test, so it runs on every `npm test` rather than
    // when someone remembers it. Same extraction: the first ts fence after the §2 heading.
    const lines = text("docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md").split("\n");
    const heading = lines.findIndex((l) => l.startsWith("## 2. The domain model"));
    const open = lines.findIndex((l, i) => i > heading && l === `${FENCE}ts`);
    const close = lines.findIndex((l, i) => i > open && l === FENCE);
    expect([heading, open, close].every((i) => i >= 0)).toBe(true);
    const contract = lines.slice(open + 1, close).join("\n");
    expect(text("desktop/src/store/types.ts").replace(/\n+$/, "")).toBe(contract.replace(/\n+$/, ""));
  });

  it("docs/SEAMS.md names exactly the seams this file pins", () => {
    expect(existsSync(at(DOC))).toBe(true);
    const rows = [...text(DOC).matchAll(/^\|\s*(S[X3458]\.\d+)\s*\|/gm)].map((m) => m[1]);
    expect(rows.length).toBe(new Set(rows).size);   // one table row per seam, never two
    expect([...rows].sort()).toEqual(SEAMS.map((s) => s.id).sort());
  });
});
```

```bash
cd desktop
npm test -- src/seams.test.ts
```

**What you must see: six tests, three failing**, and each failure for the reason below — no other.

| Test | Why it fails now | What the failure lists |
|---|---|---|
| *still has every seam's anchor…* | the five lens files do not exist; `AppShell.tsx` still imports `LensPlaceholder`; `proxy.Relay` still exists | `SX.1` — fifteen `does not exist` lines and five `lost …return <XLens />` lines, then `gained /\bLensPlaceholder\b/`; `S3.5` twice and `S5.2` once, `does not exist`; `S8.7 … gained /^func Relay\(/m` |
| *still has every test a seam's contract rests on* | step 4 has not written the Go seam test | `S8.1: control-plane/internal/backend/seam_test.go does not exist`, twice |
| *docs/SEAMS.md names exactly…* | step 6 has not written the document | `expected false to be true` |

**The three that pass are the evidence that matters most**, because they pass against today's tree without anything in this task having run: the id check; the **tree tripwires** — no component touches `contextRef` and nothing outside `client.ts` opens a socket, which is `survey-phase-2-substrate.md:355-358` and `:452` still true after tasks 22 to 25; and **the contract §2 gate**, which has only ever been run by hand (`task-15.md:276-279`) and now runs on every `npm test`. **If the §2 gate fails**, `types.ts` and contract §2 have drifted since task 15 — most likely the resume fix, if it touched the `Session` type without amending §2 — and that belongs to whoever made the edit, not to this task. **If any other anchor outside the table above fails**, a seam this document is about to name has already moved — that anchor's row must be written from the code as it is, and the discrepancy goes in the commit body.

The reporter chrome is approximate, for the reason at the top of this file; `Tests  3 failed | 3 passed (6)` is the line to check.

- [ ] **Step 3: Split the five placeholder arms into five lens files, and close `AppShell.tsx`**

This is gate-zero **0b** (`dependency-graph-and-order.md:147`, `:232`). **Each stub renders exactly what its arm renders today** — same `lens`, same `phase`, same `what`, copied from `AppShell.tsx:28-67` as step 1 (b) left it — so no test anywhere changes, and `AppShell.test.tsx`'s placeholder assertions (`survey-phase-2-substrate.md:215-216` names `:112`'s `/Arrives in phase 3\./`) pass untouched. That is the proof the split changed nothing a user can see. **Copy each `what` string from `AppShell.tsx` itself, not from this plan**: the five below were read on 2026-09-23 and task 23 may have reworded one since.

The header of each file is the seam's local pointer: it says who owns the file, and that the shell is closed. It is the only place this task writes a seam comment, because these five files are new and have no other header to carry it.

Create `desktop/src/components/lens/DiscoverLens.tsx`:

```tsx
// Seam SX.1 (docs/SEAMS.md). Phase 4 owns this file: Discover replaces the body below and
// nothing else in the tree has to change. AppShell.tsx routes the "discover" lens here and
// is closed for the programme. Until Phase 4 it renders what AppShell rendered inline
// before task 26: the placeholder, naming the phase that brings the lens.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DiscoverLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="discover"
      phase={4}
      what="Turn a conversation into a spec this branch can be measured against."
    />
  );
}
```

`desktop/src/components/lens/DesignLens.tsx`:

```tsx
// Seam SX.1 and S5.2 (docs/SEAMS.md). Phase 5 owns this file: the Design canvas replaces the
// body below. It is handed a fixed box — .shell-lens is min-block-size 0, overflow hidden —
// and owns its own scroll. AppShell.tsx routes the "design" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DesignLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="design"
      phase={5}
      what="A canvas of real HTML frames an agent can edit directly."
    />
  );
}
```

`desktop/src/components/lens/FilesLens.tsx`:

```tsx
// Seam SX.1 and S3.5 (docs/SEAMS.md). Phase 3 owns this file by the phase number it has always
// rendered; docs/PROGRAM.md does not yet name an owner for Files, and S3.5 records that as the
// product owner's decision. AppShell.tsx routes the "files" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function FilesLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="files"
      phase={3}
      what="The worktree's files, with what each session changed."
    />
  );
}
```

`desktop/src/components/lens/MergeLens.tsx`:

```tsx
// Seam SX.1 and S3.5 (docs/SEAMS.md). Phase 3 owns this file: docs/PROGRAM.md's phase 3 is
// "diff review … merge ordering, conflict resolution". Reordering goes through S3.4's
// visible-position helpers, never an index across a filter. AppShell.tsx routes here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function MergeLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="merge"
      phase={3}
      what="Review a worktree's diff and land it on the branch."
    />
  );
}
```

`desktop/src/components/lens/DeployLens.tsx`:

```tsx
// Seam SX.1 (docs/SEAMS.md). Phase 7 owns this file: the Deploy connector replaces the body
// below. Task 26 names seams for phases 3-5 and 8, but closing AppShell.tsx needs every arm
// to be a file, so this one exists too. AppShell.tsx routes the "deploy" lens here and is closed.
import type { JSX } from "react";
import { LensPlaceholder } from "./LensPlaceholder";

export function DeployLens(): JSX.Element {
  return (
    <LensPlaceholder
      lens="deploy"
      phase={7}
      what="Ship the branch, with the spec attached to the release."
    />
  );
}
```

**Then `AppShell.tsx`, and this is a declared departure.** The file is task 18's, edited by tasks 20 and 23, all closed; shared shape §1 would normally forbid this task touching it. The departure is justified by the owner column of gate-zero 0b — *"Phase 2 closeout"* — and bounded: **one import line out, five in, five arm bodies replaced, one comment added, nothing else.** Replace the import of `LensPlaceholder`:

```tsx
import { LensPlaceholder } from "./lens/LensPlaceholder";
```

with the five lens imports, in `LensRouter`'s arm order:

```tsx
import { DiscoverLens } from "./lens/DiscoverLens";
import { DesignLens } from "./lens/DesignLens";
import { FilesLens } from "./lens/FilesLens";
import { MergeLens } from "./lens/MergeLens";
import { DeployLens } from "./lens/DeployLens";
```

Add one comment immediately above `function LensRouter`:

```tsx
// CLOSED FOR THE PROGRAMME — task 26, seam SX.1 in docs/SEAMS.md. Every lens is one file
// under ./lens/, and a later phase fills its own file rather than this switch: five phases
// editing one import block is five merge conflicts (dependency-graph-and-order.md:143-145).
```

And replace each placeholder arm's `return ( <LensPlaceholder … /> );` with one line. The `develop` arm, task 20's comment block and the `default` arm are untouched:

```tsx
    case "discover":
      return <DiscoverLens />;
    case "design":
      return <DesignLens />;
    case "files":
      return <FilesLens />;
    case "merge":
      return <MergeLens />;
    case "deploy":
      return <DeployLens />;
```

```bash
cd desktop
npm test -- src/components/AppShell.test.tsx
npm test -- src/seams.test.ts
npm run typecheck
grep -cE '^import \{ [A-Za-z]+Lens \} from "\./lens/[A-Za-z]+Lens";$' src/components/AppShell.tsx   # 5
grep -cE '^import \{ DevelopLens \} from "\./develop/DevelopLens";$' src/components/AppShell.tsx  # 1
grep -c  'LensPlaceholder' src/components/AppShell.tsx                                             # 0
```

**What you must see.** `AppShell.test.tsx` passes with **the same test count it had before this step** — record the number before editing, and a change in it means a test was lost, not that the split worked. The seam manifest is now **four passing, two failing**: *every seam's anchor* fails on `S8.7` alone (`gained /^func Relay\(/m`), *every test* on `S8.1`, and *docs/SEAMS.md* on the missing document. `SX.1`, `S3.5` and `S5.2` have left the failure list. The typecheck is silent. `grep -c LensPlaceholder` should be `0` counting comments too — the comment this step adds deliberately does not name the component. If it prints more, read the lines: a comment task 23 wrote that names it is fine, because the manifest's `SX.1` anchor runs against `codeOnly` and is the gate; an `import` or a JSX use is not.

- [ ] **Step 4: Pin the `Backend` interface by its method set**

`dockerbe` and `katabe` each carry `var _ backend.Backend = (*Backend)(nil)` (`docker.go:43`, `kata.go:25`), and `kata.go:22-24` says what that buys: *"if the seam ever grows a method Kubernetes cannot serve, this file stops compiling on a laptop instead of in Phase 8 on a cluster."* That catches an implementation falling behind the interface. **It does not catch the interface and both implementations changing together** — which is exactly what a Phase 3 task adding, say, a `Snapshot` method to both would do, and it would move Phase 8's seam without Phase 8's document knowing. Reflection over the interface catches that, and only that.

**This test is green on its first run by construction**, because it describes the tree as it is. Step 7 is where it is made to fail.

Create `control-plane/internal/backend/seam_test.go`:

```go
package backend_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// TestSeamBackendIsTheSevenMethodsBothImplementationsServe pins seam S8.1 in
// docs/SEAMS.md: the interface Phase 8's Kata backend plugs into, exactly as
// Phase 2 shipped it. dockerbe and katabe assert at compile time that they
// satisfy it; this pins the other direction — that the interface itself did not
// grow, shrink or change a signature while both implementations changed to
// match. If this fails because the seam was meant to change, change the want
// below and the S8.1 row in docs/SEAMS.md in the same commit.
func TestSeamBackendIsTheSevenMethodsBothImplementationsServe(t *testing.T) {
	want := map[string]string{
		"Attach":  "func(context.Context, string, string) (*websocket.Conn, error)",
		"Close":   "func() error",
		"Create":  "func(context.Context, backend.Spec) (backend.Sandbox, error)",
		"Destroy": "func(context.Context, string) error",
		"Get":     "func(context.Context, string) (backend.Sandbox, error)",
		"Kind":    "func() backend.Kind",
		"List":    "func(context.Context) ([]backend.Sandbox, error)",
	}
	typ := reflect.TypeOf((*backend.Backend)(nil)).Elem()
	got := make(map[string]string, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		got[m.Name] = m.Type.String()
	}
	if !reflect.DeepEqual(got, want) {
		names := make([]string, 0, len(got))
		for name := range got {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Fatalf("backend.Backend moved: seam S8.1 (docs/SEAMS.md) no longer matches.\nmethods now: %v\n got: %v\nwant: %v", names, got, want)
	}
}

// TestSeamBackendKindsAreTheTwoPersistedStrings pins the other half of S8.1.
// backend.go:79-80 says these strings are persisted in the registry and
// returned over REST, so renaming one strands every session recorded under it.
func TestSeamBackendKindsAreTheTwoPersistedStrings(t *testing.T) {
	if backend.KindDocker != "docker" || backend.KindKata != "kata" {
		t.Fatalf("Kind strings are %q and %q, want \"docker\" and \"kata\" (seam S8.1, docs/SEAMS.md)", backend.KindDocker, backend.KindKata)
	}
}
```

```bash
go test ./control-plane/internal/backend/ -run Seam -count=1 -v
```

**What you must see:** both tests `--- PASS`. **If the first fails on a formatting difference** — `got` shows the same seven names with a signature spelt differently, say `websocket.Conn` qualified another way — the reflection string is what Go prints on this toolchain and the `want` is my transcription of it: correct `want` to what `got` prints **after reading `backend.go:283-291` to confirm the interface really is the seven methods it says**. A different *set of names* is never a formatting difference.

- [ ] **Step 5: Two corrections the seams require — katabe's header, and one relay**

**5a — correct `katabe`'s header, which says the opposite of what the code does.** `kata.go:1-4` ends *"It compiles and is unit-tested now so that turning it on in Phase 8 is a configuration change and not a rewrite."* Step 1 (d) confirmed why that is false: every create and restart sends host `Mounts`, the worktree first, and a `WorkDir` (`api/sessions.go:992-993`, `:1678-1679`), and `Create` refuses both (`kata.go:94-99` before this step, `:101-106` after it), so under `PANACEA_APP_BACKEND=kata` every session create fails. **The refusal is right and stays** — `kata.go:75-79` gives the reason, *"silently dropping it would hand the agent an empty worktree with no error anywhere"*. The defect is the sentence a Phase 8 planner would read first and believe. Replace the package comment, lines 1-4, with:

```go
// Package katabe runs sandboxes as Kata Containers pods on AKS. It is DORMANT:
// docs/PROGRAM.md phase 8 is the first real Azure spend, and nothing in Phase 2
// selects this backend by default. It compiles and is unit-tested now so that
// the Backend seam cannot drift away from what Kubernetes can serve.
//
// Turning it on is NOT only a configuration change. Every session create sends
// host-path Mounts, the worktree first, and a WorkDir (api/sessions.go), and
// Create refuses both below — correctly, because a host path on an AKS node is
// meaningless. So under PANACEA_APP_BACKEND=kata every create fails today. The
// first thing Phase 8 must build is a way to get the worktree into the pod:
// seam S8.3 in docs/SEAMS.md.
```

`package katabe` stays on the line after it. Nothing else in the file changes.

```bash
grep -c 'is a configuration change and not a rewrite' control-plane/internal/backend/katabe/kata.go   # 0
grep -cE '^// seam S8\.3 in docs/SEAMS\.md\.$'         control-plane/internal/backend/katabe/kata.go   # 1
go vet ./control-plane/internal/backend/katabe/
```

**5b — rule on open carry 12: one relay, `proxy.RelayHooked`.** `api/attach.go:216` is the only production caller of either relay, and it calls `RelayHooked`. `proxy.Relay` (`proxy.go:26-30`) is Phase 1's original, now a three-line wrapper that passes empty `Hooks`, and its one caller is a test (`proxy_test.go:57`). **The ruling is delete, not wire**: wiring would mean a second production relay path, and the attach relay is the path Phase 8's pods are reached through — it needs one name, the one that carries the size hooks the idle status is derived from (`proxy.go:12-21`).

**Confirm it has no production caller now, at the moment of deletion** — not from step 1's reading, and not from this plan's. Both greps are anchored so a comment cannot answer: a line counts only if its first non-space character is not `/`, which is what keeps `backend.go:9`'s package comment, which names `proxy.Relay`, out of the result.

```bash
# Callers from other packages, in code, outside tests. sandboxd is a separate module and is
# searched anyway, because "nothing calls it" has to be true of the whole repository.
grep -rnE '^[[:space:]]*[^/[:space:]].*\bproxy\.Relay\(' control-plane sandboxd --include=*.go \
  | grep -v '_test\.go:' | grep -v '/vendor/'                               # expect no output
# Callers inside package proxy, where the name is spelt bare.
grep -nE '^[[:space:]]*[^/[:space:]].*[^.[:alnum:]]Relay\(' control-plane/internal/proxy/proxy.go
                                                                            # exactly one line: func Relay(
```

**If the first grep prints anything, stop**: something started calling `Relay` after this plan was written, the ruling no longer holds as stated, and the orchestrator decides again. Only then delete.

In `control-plane/internal/proxy/proxy.go`, delete `Relay` and its two-line comment, and give its sentence to `RelayHooked`, whose comment currently begins by defining itself in terms of the function being deleted:

```go
// RelayHooked copies messages between client and upstream until either closes
// or ctx is cancelled, reporting each message's size to h. It returns the first
// error observed. It is the only relay: seam S8.7 in docs/SEAMS.md.
//
// A hook fires after a successful read and before the write, so a message the
// agent produced is counted even when the desktop has already gone away. That
// is deliberate: the timestamp records when the agent last did something, not
// when a socket last succeeded.
func RelayHooked(ctx context.Context, client, upstream *websocket.Conn, h Hooks) error {
```

In `control-plane/internal/proxy/proxy_test.go:57`:

```go
		_ = proxy.RelayHooked(r.Context(), client, up, proxy.Hooks{})
```

And in the package comment of `control-plane/internal/backend/backend.go:9`:

```go
// proxy.RelayHooked already takes *websocket.Conn.
```

```bash
gofmt -l control-plane/internal/proxy control-plane/internal/backend          # expect no output
go test ./sandboxd/... ./control-plane/... -race -count=1
grep -rnE '\bRelay\(' control-plane --include=*.go | grep -v RelayHooked      # expect no output
```

**What you must see:** every package `ok`, and no remaining `Relay(` that is not `RelayHooked(`. The Go contract's two mentions (`2026-09-18-phase-2-go-contract.md:169`, `:1945`) and Phase 1's plan are **historical design records** and are not edited; `docs/SEAMS.md`'s S8.7 row records the ruling so a reader of the contract finds where it went.

- [ ] **Step 6: Write `docs/SEAMS.md`**

**One table row per seam, and nothing else in the file may begin a line with `| S`** — the manifest's last test counts rows by that pattern, so a second table or a notes list in the same shape would count twice. The notes below the table use headings, not rows. Every cell cites the file and line a reader opens; the *Guard* column names the test that fails when the seam moves.

Fill `<date>` with the commit date and `<run-date>` with task 25's results document's date. Create `docs/SEAMS.md`:

```markdown
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
without an anchor fails; an anchor with no row fails.

Named by Phase 2 task 26 on <date>, against a substrate proven end to end on Docker in
`docs/verification/<run-date>-phase-2-results.md`.

## Rules that bind every phase

Not seams — none has one location — but each is broken the same way by every phase that
forgets it, and each is already enforced where cited.

1. **`store/types.ts` is byte-identical to UI contract §2.** Adding a field amends §2 in the
   same change. Pinned by SX.2.
2. **A new persisted key bumps `PERSIST_VERSION` and writes a migration that translates and
   never drops** (`desktop/src/store/store.ts:44-47`). Two phases doing this concurrently
   produce two version-2 migrations; sequence them. Pinned by SX.3.
3. **A derived read is a selector in `store/selectors.ts` with a doc comment naming its
   surface**, never an inline `.filter()` inside `useCockpit(...)`, which re-renders on every
   2 s poll (group-d shared shape §4; `selectors.ts:3-5`).
4. **CSS obeys shared shape §3B's six rules** — token import first, no colour function outside
   `tokens.css`, no gradient, no `transition: all`, `var(--numeric)` on live numbers, status
   separable without motion or colour. `desktop/src/styles/designsystem.test.ts` greps them.
5. **`jest-dom` is not installed** (shared shape §3C). Assert on DOM properties.
6. **No feature reads PTY bytes to derive meaning** (`docs/PROGRAM.md:116`). Every seam below
   respects this, and a phase that needs "blocked on you" needs a new server signal, not a
   client derivation (`survey-phase-2-substrate.md:90-94`).

## The seams

| ID | Phase | Where | The phase does | The phase must not | Guard |
|---|---|---|---|---|---|
| SX.1 | 3, 4, 5, 7 | `desktop/src/components/lens/{Discover,Design,Files,Merge,Deploy}Lens.tsx`; `AppShell.tsx` `LensRouter` | Replace the body of its own lens file | Edit `AppShell.tsx`, which is closed; add a `Lens` union member — all six exist (`types.ts`) | seams.test.ts `SX.1` |
| SX.2 | all | `desktop/src/store/types.ts` ≡ UI contract §2's fenced block | Amend both in one commit when the model grows | Add a field to one side | seams.test.ts `SX.2` gate |
| SX.3 | all | `desktop/src/store/store.ts:16-17` (`PERSIST_KEY`, `PERSIST_VERSION = 1`), `partialize` at `:34-48` | Add a root key, bump the version, write the migration, update this row | Nest a slice — `persist` shallow-merges at the root (`store.ts:3-6`) | seams.test.ts `SX.3` |
| SX.4 | 3, 4, 5 | `control-plane/internal/api/api.go:269` — every route registers through the recorder (`routes.Handle`) | Add its routes to the table and to both audit maps | Register on the raw mux; add a side-effecting GET | `TestNoRegisteredGetCanChangeState`, `TestNoFileRegistersOnTheRawMuxBehindTheRecorder` |
| SX.5 | 3, 4, 5 | `control-plane/internal/statepath/statepath.go:30` `Dir(sub)` | Take one subdirectory — Phase 4's is `Dir("discover")` | Re-derive the state ladder | seams.test.ts `SX.5` |
| SX.6 | 3, 4, 5 | `desktop/src/components/palette/commands.ts` — `Command`, `buildCommands(state)` | Append its commands; ranking is fixed here, filtering only removes | Add a destructive verb one keystroke from a filter (open carry 4, defect 10's shape) | seams.test.ts `SX.6` |
| SX.7 | all | `desktop/src/api/problems.ts:13,37` — `ProblemKind`, `classify` | Switch over `ProblemKind` at its call site with a declared return type and its own sentences | Add a `default:` to that switch, or a shared `sentenceFor` | seams.test.ts `SX.7` |
| SX.8 | all | `desktop/src/hooks/heartbeat.ts` — `BEAT_MS`, `beatDelayStyle()`, `useHeartbeat()` | Use them for anything that breathes or ticks | Build a third clock; subscribe a row per list item to the 1 Hz tick | seams.test.ts `SX.8` |
| SX.9 | 3 (`--drift`); unowned (`--need`) | `desktop/src/styles/tokens.css:75,77` | Use the token the day the status can be derived, and update task 24's audit assertion in the same change | Invent a second amber or purple | designsystem.test.ts *defines --drift and references it nowhere* |
| SX.10 | 8; every phase that resumes | Claude Code started with `--session-id <uuid>`, and a resume forked from that UUID with `--resume <uuid> --fork-session` (`control-plane/internal/harness/argv.go:115,118`); the cockpit's id resolved to Claude's by `sessionResolveResume` (`api/sessions.go:1069`); the conversation store persisted by `sessionMounts`'s mount of `conversationStore` at `sessionConversationDir` (`api/sessions.go:58,346`), whose mount point the image makes as `dev` (`RUN mkdir -p /home/dev/.claude/projects`, `images/sandbox/Dockerfile:32`) | Carry the store's persistence into Kata pods, whose home is an `emptyDir` (see note, and S8.8) | Hand the cockpit's session id to `claude --resume`; resume without `--fork-session`; parse agent output to learn Claude's | seams.test.ts `SX.10`; task 25 `o2.resume` |
| S3.1 | 3 | `control-plane/internal/project` — read-only by design (`project.go:1-6`) | Put worktree lifecycle and every git write in a new package | Widen `project`'s whitelist of `for-each-ref`, `rev-parse`, `worktree list` | `TestEveryGitCommandIsReadOnly` (`project_test.go:429`) |
| S3.2 | 3, 4 | `control-plane/internal/worktree/worktree.go:32` `Provider.Path(ctx, worktreeID)` | Make every worktree it creates resolve through it; session create does (`sessions.go:805`), and so does restart (`:1501`) | Resolve paths a second way | seams.test.ts `S3.2` |
| S3.3 | 3 | `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}` (`catalog.go:33`); `Worktree.ready?` (`types.ts:85`); `setWorktreeReady` (`client.ts:315`) | Gate merge on `ready` | Derive readiness any other way (known defect 9) | seams.test.ts `S3.3` |
| S3.4 | 3 | `desktop/src/lib/immutable.ts` — `moveWithinVisible`, `idAtVisiblePosition` | Reorder the merge queue through them | Pass an index across a filter boundary (known defect 10) | defects.test.tsx row 10 |
| S3.5 | 3 | `FilesLens.tsx`, `MergeLens.tsx` — both render phase 3 | Fill both | Treat Files' ownership as settled: `docs/PROGRAM.md` names no owner for it (open carry 8) | seams.test.ts `S3.5` |
| S4.1 | 4 (write), all (read) | `Session.contextRef`: `registry/session.go:102-104`; `api/sessions.go:651,682,738,953`; `types.ts:163`; `defaults.ts:110,127`; carried by resume and fork (`client.ts:362,391`) | Write it on create; add a resolve endpoint so surfaces can show a brief's title | Make Phase 2 code interpret it; render it from a component before updating this row — the tree tripwire fires | seams.test.ts `S4.1` |
| S4.2 | 4 | `handleSessionCreate` (`api/sessions.go:791`): between worktree resolution (`:805`) and `Backend.Create` (`:983`) | Materialise `CLAUDE.md` there, once decision 0c is made | Rely on the client to call it first — silent inheritance failure | seams.test.ts `S4.2` (`api.go` has no materialiser) |
| S4.3 | 4 | `desktop/src/lib/immutable.ts` — `replaceAt`, `replaceById` | Implement `editAnswer` through them | Splice later answers (known defect 7) | defects.test.tsx row 7 |
| S4.4 | 4 | `project.Scanner.ProjectName` / `BranchName` (`project.go:172,175`); `/api/v1/` prefix strip (`api.go:391`) | Use them as its `Naming` and its route prefix | Delete the prefix strip — its comment says why | seams.test.ts `S4.4` |
| S5.1 | 5 | `desktop/src/api/poller.ts:10` and `client.ts:297` — the only transports | Bring its canvas room as a named second transport, and update this row | Open a socket or `EventSource` anywhere without updating this row — the tree tripwire fires | seams.test.ts `S5.1` |
| S5.2 | 5 | `.shell-lens` (`shell.css:30-33`); `DesignLens.tsx` | Render into the fixed box and own its scroll | Resize the shell or scroll the page | seams.test.ts `S5.2` |
| S8.1 | 8 | `backend.Backend` (`backend.go:337-345`), seven methods; `katabe` implements it (`kata.go:32`) | Turn katabe on behind it | Change the interface without this row — reflection fails | `TestSeamBackendIsTheSevenMethodsBothImplementationsServe`, `TestSeamBackendKindsAreTheTwoPersistedStrings` |
| S8.2 | 8 | `PANACEA_APP_BACKEND=kata` (`config.go:31`) → `newBackend` (`main.go:258`) → `kubeClient` (`:306`), in-cluster only | Add `clientcmd` under contract §8.5's pinning review if a kubeconfig is needed | Honour `PANACEA_APP_KUBECONFIG` silently | seams.test.ts `S8.2` |
| S8.3 | 8 — **its first problem** | Create and restart always send `WorkDir` and host `Mounts` (`api/sessions.go:992-993`, `:1678-1679`), built by `sessionMounts` with the worktree first (`:340-341`); `katabe.Create` refuses both (`kata.go:101-106`), so every create under Kata fails today | Before anything else: design how a worktree gets into a pod, then change both sides and this row | Drop the refusal to make creates pass — that ships an empty worktree with no error; treat Kata as a configuration switch | `TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv`, `TestCreateRejectsMountsAndCreatesNoPod`, `TestCreateRejectsAWorkDir` |
| S8.4 | 8 | `const runtimeClass = "kata-mshv-vm-isolation"` (`sandbox/manager.go:20`) | Confirm it against a real cluster (Phase 1 unproven item 2) | Ship it unobserved | seams.test.ts `S8.4` |
| S8.5 | 8 | `isolationFor` (`api/catalog.go:174`); the rail's badge (`MachineChip.tsx:45`) | Report `microvm` for Kata machines | Hide the badge client-side | seams.test.ts `S8.5` |
| S8.6 | 8 | `desktop/src/api/client.ts:6-8` — no `Authorization` header | Add auth with its server half, together | Add a header "for safety" in an earlier phase | seams.test.ts `S8.6` (see note) |
| S8.7 | 8 | `proxy.RelayHooked` (`proxy/proxy.go`), called once at `api/attach.go:216` | Reach pods through it | Add a second relay; `proxy.Relay` was deleted by task 26 (open carry 12) | seams.test.ts `S8.7` |
| S8.8 | 8 | The conversation store: `<state>/conversations/local/<worktree>/` mounted read-write at `sessionConversationDir` (`api/sessions.go:58,346`), and a fork's staged read-only copy of its source's transcript (`carried`, `:349`); refused by S8.3's `len(spec.Mounts) > 0` | Provide a volume per user and worktree that outlives the pod and holds `projects/` only (Go contract §2.6), so SX.10's resume survives a pod | Put the store in the pod's `emptyDir` home; share one store across users or worktrees | `TestCreateMountsOnlyThisWorktreesConversationStore`, `TestAForkIsShownAStagedReadOnlyCopyOfTheSourcesTranscript`; seams.test.ts `S8.8` |
| S8.9 | 8 | The CA bundle: `PANACEA_APP_CA_BUNDLE` mounted read-only at `sessionCABundlePath` = `/etc/panacea-app/ca-bundle.pem` and named by `NODE_EXTRA_CA_CERTS` (`api/sessions.go:296,352`); refused by S8.3's check | Deliver the operator's CA to the pod another way, at the same path and named by the same variable | Drop `NODE_EXTRA_CA_CERTS`, without which Claude Code cannot start on a TLS-inspecting network; set `SSL_CERT_FILE` or `NODE_TLS_REJECT_UNAUTHORIZED`, which tests assert are absent | `TestACABundleIsMountedReadOnlyAndNamedByNodeExtraCACerts`; seams.test.ts `S8.9` |
| S8.10 | 8 | A primary worktree's `.git/worktrees`, mounted read-only over itself at `sessionWorktreeAdminDir` (`api/sessions.go:40,343`); refused by S8.3's check | Decide it with S8.3's worktree delivery; under OD-2's one local clone per worktree it may vanish, and this row with it | Make it writable: the sandbox's git calls every linked worktree prunable, and a `git worktree prune` there deletes the host's metadata (`sessionWorktreeAdmin`'s comment, `:562-570`) | `TestAPrimaryWorktreesLinkedWorktreeMetadataIsReadOnlyInTheSandbox`; seams.test.ts `S8.10` |
| S8.11 | 8 | The host git's global excludes and attributes files, read-only at `sessionGitExcludesPath` and `sessionGitAttributesPath` (`api/sessions.go:304-305,355,358`), mounted only when the host's git reads them; refused by S8.3's check | Carry the user's own git config into the pod, or state that a cloud sandbox has none | Copy a file the host's git does not read; make either writable | `TestTheHostsGlobalGitFilesReachTheSandboxReadOnlyAndOnlyWhenTheHostsGitReadsThem`; seams.test.ts `S8.11` |
```

Then, below the table in the same file, the notes — headings only, no table rows:

```markdown
## Notes on the gaps

### S4.2 — reserved in prose, never built

The Go contract §5.1 reserved the space — *"Anything Phase 4 later needs on disk before the
agent starts — `CLAUDE.md` — is written between steps 1 and 5"* — but `api.Deps`
(`api.go:68`) has no materialiser field (`survey-phase-4-discover.md:97-104`). Who calls
`Materialise` is **decision 0c** (`dependency-graph-and-order.md:233`), the orchestrator's.
The guard pins the current state: the day a materialiser lands in `api.go`, the manifest fails
and this note is rewritten to say where it went.

### S8.3 — the first thing Phase 8 must solve: getting the worktree into the pod

Every create and restart sends host-path mounts, the worktree first, and a working directory
(`api/sessions.go:992-993`, `:1678-1679`), and `katabe.Create` refuses both, correctly — *"a host
path on an AKS node is meaningless"*, and silently dropping the mount would hand the agent *"an
empty worktree with no error anywhere"* (`kata.go:92-106`). The four mounts after the worktree
are seams of their own, S8.8 to S8.11: each is refused by the same check, and each needs its own
answer once the worktree has one. So under `PANACEA_APP_BACKEND=kata` every
session create fails today. `katabe`'s header used to say turning it on was *"a configuration
change and not a rewrite"*; task 26 corrected it to say this instead. **Before Phase 8 does
anything else, it designs how a worktree reaches a pod** — and the refusal stays until that
design replaces the mount, never the other way round. The three tests in the Guard column pin
the two halves; the one that flips first names the side that changed.

### S3.5 — Files has a phase number and no owner

`AppShell.tsx` has always rendered Files as phase 3, and `docs/PROGRAM.md`'s phase 3 —
*"diff review with transcript back-links"* — is its plausible home. But `PROGRAM.md` names no
owner for Files, and open carry 8 raises it with the product owner. Task 23's sweep found a third
placeholder of the same class — *Authority*, asserted as Phase 3 on the strength of
`docs/AGENTIC-UX.md` §7 (`group-d-open-carries.md:90-94`) — and one product decision settles all
three. Until then, the phase number in `FilesLens.tsx` is a claim the code makes, not a decision.

### SX.10 — resume by Claude's own id, from a store that outlives its sandbox

Phase 2 planning found resume could not restore context: the client sent the cockpit's own
session id to `claude --resume`, where the Go contract then typed `Resume` as a prior agent
session id (Go contract §2.9, `harness.Request`, since amended), and a resumed session is a new
sandbox, so the old conversation store was gone. A separate fix, landed before task 25, starts
Claude Code with `--session-id <uuid>`, resumes from that UUID as a fork (`--resume <uuid>
--fork-session`, the only pairing the CLI accepts), and persists the conversation store (S8.8). Task 25's `o2.resume` proves identity
and the store; the owner's step-18 observation in the results document proves the agent
remembers. **This is a Phase 8 seam as well as a resume one**: a Kata pod's home is an `emptyDir`
(`docs/verification/2026-09-17-phase-1-results.md:67`), which dies with the pod exactly as a
container's home did, so Phase 8 must carry the fix's persistence into pods — or resume breaks
again on the first cloud machine.

### S8.6 — why the guard ignores comments

`client.ts:6-8` names the header in a comment explaining its absence, so the anchor runs over
comment-stripped code. The stripper also blanks the rest of any line after a `//` inside a URL
string, which can hide a match and never create one: a header written after a URL on the same
line would pass. That narrow false green is accepted over a permanent false red.

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
```

```bash
cd desktop
npm test -- src/seams.test.ts
```

**What you must see: six passed.** The document sync is the one that went green in this step. **If it fails with two sorted arrays that differ by one id**, a row was mistyped or a note was written as a table row — `grep -nE '^\|\s*S[X3458]\.[0-9]+\s*\|' ../docs/SEAMS.md` lists every line the test counted, and it must print exactly thirty-two.

Two things to check by reading, because no test can: that every file and line the table cites is the one that opens when a reader follows it — **re-read each against the tree as it stands after steps 3 and 5**, because step 3 moved `AppShell.tsx`'s lines and step 5 moved `proxy.go`'s and `kata.go`'s. The header grew by seven lines, and every `kata.go` citation in the S8.1 and S8.3 rows and notes is already the after-5a number (HEAD's plus seven: `:32`, `:101-106`, `:92-106`, derived at `aab769f`); confirm each rather than shifting it again. Every other line in the table was re-derived at `aab769f` too; a file changed since then (task 25 edits `harness.go` and `argv.go`) is re-read. And check that no row's *must not* column contradicts a rule another row states.

- [ ] **Step 7: Prove the manifest can fail**

**A seam guard that has never been red proves nothing**, and every guard in this task is green by construction once steps 3 to 6 are done — the same position `task-24.md:1218` was in. So falsify it. Make each change, run the one command, confirm the named test goes red **for the named reason**, then **undo the change before making the next**.

| Mutation | Command | Test that must go red | Failure it must print |
|---|---|---|---|
| In `DesignLens.tsx`, `phase={5}` → `phase={6}` | `npm test -- src/seams.test.ts` | *still has every seam's anchor* | `SX.1 (one lens, one file; AppShell.tsx is closed): desktop/src/components/lens/DesignLens.tsx lost /^\s+phase=\{5\}$/m` |
| Re-add `import { LensPlaceholder } from "./lens/LensPlaceholder";` to `AppShell.tsx` | same | *still has every seam's anchor* | `SX.1 … AppShell.tsx gained /\bLensPlaceholder\b/` |
| Add a field `Materialiser func()` to `api.Deps` in `api.go` | same | *still has every seam's anchor* | `S4.2 … api.go gained /Materiali[sz]/` |
| Add `void new EventSource("x");` to the body of `DesignLens` | same | *keeps every tree-wide tripwire* | `S5.1 … DesignLens.tsx matches /\bnew (?:EventSource\|WebSocket)\(/` |
| Add `{contextRef}` to any JSX in `SessionPaneHeader.tsx` | same | *keeps every tree-wide tripwire* | `S4.1 … SessionPaneHeader.tsx matches /\bcontextRef\b/` |
| Add a row `\| S9.1 \| 9 \| x \| x \| x \| x \|` to the table in `docs/SEAMS.md` | same | *docs/SEAMS.md names exactly the seams this file pins* | two sorted arrays differing by `"S9.1"` |
| Add `Snapshot(ctx context.Context, id string) error` to `backend.Backend`, and a `Snapshot` method returning `nil` to both `dockerbe.Backend` and `katabe.Backend` | `go test ./control-plane/internal/backend/... -run Seam -count=1` | `TestSeamBackendIsTheSevenMethodsBothImplementationsServe` | `backend.Backend moved: seam S8.1 (docs/SEAMS.md) no longer matches`, with `Snapshot` in *methods now* |

**The seventh row is the one this task exists for.** It is the change that compiles everywhere — both implementations grew the method, so both `var _ backend.Backend` assertions stay green — and only the reflection test notices that Phase 8's seam moved. If it does **not** go red, the test is decorative; fix the test, not the mutation.

Two mutations are deliberately **not** in the table. Deleting a lens file makes `tsc` fail before vitest reports anything useful, which proves the compiler works, not the manifest. And changing `Backend` without touching the implementations fails `go build`, for the same reason — that is the compile-time assertion's job, which already existed.

Then confirm you left nothing behind:

```bash
git status --porcelain | sort
git diff -- control-plane/internal/api/api.go desktop/src/components/develop/SessionPaneHeader.tsx   # expect no output
```

`git status` lists exactly the thirteen paths of the Files block and nothing else. A fourteenth is an un-reverted mutation, or another agent's file — the second is **not** staged.

- [ ] **Step 8: Guards, then commit**

Anchored, per shared shape §3A.

```bash
cd desktop && npm test && npm run typecheck && cd ..
go test ./sandboxd/... ./control-plane/... -race -count=1
git diff --stat -- go.work sandboxd/go.mod control-plane/go.mod           # expect no output

# The document and the manifest agree on the count, independently of the test that compares them.
grep -cE '^\|\s*S[X3458]\.[0-9]+\s*\|'       docs/SEAMS.md                 # 32
grep -cE '^\s+id: "S[X3458]\.[0-9]+",$'      desktop/src/seams.test.ts     # 32

# The lens split.
ls desktop/src/components/lens/                                            # six .tsx files
grep -cE '^export function [A-Za-z]+Lens\(\): JSX\.Element \{$' desktop/src/components/lens/*Lens.tsx
                                                                           # 1 in each of the five stubs
grep -cE '^\s+return <[A-Za-z]+Lens />;$'    desktop/src/components/AppShell.tsx   # 6

# The relay ruling.
grep -cE '^func Relay\('                      control-plane/internal/proxy/proxy.go # 0
grep -cE '^func RelayHooked\('                control-plane/internal/proxy/proxy.go # 1

# katabe's header now says what Phase 8 must build, and the refusal it describes is intact.
grep -c  'is a configuration change and not a rewrite' control-plane/internal/backend/katabe/kata.go   # 0
grep -cE '^\tif len\(spec\.Mounts\) > 0 \{$'   control-plane/internal/backend/katabe/kata.go         # 1

# Nothing this task wrote still carries a placeholder from the plan. SX.10's row names the
# resume fix's declarations itself now; the grep stays, so a row copied from an older plan fails.
grep -c '<date>\|<run-date>\|<the resume fix' docs/SEAMS.md                  # 0
grep -c 'THIRD ANCHOR' desktop/src/seams.test.ts                            # 0 — replaced by the real anchor

git status --porcelain | sort                                              # the thirteen paths, nothing else
```

The `grep -cE '^export function …Lens'` line prints one count per file matched by `*Lens.tsx` — the five stubs; `LensPlaceholder.tsx` does not end in `Lens.tsx` — so five lines, each `1`.

Then commit, staging by name — never `git add -A`, `-u` or `.`. **Write the message to a file with the file-writing tool and commit with `git commit -F`**: a multi-line heredoc through the audited wrapper can terminate early at its extra `bash -c` boundary. **The trailer is `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`**, as the master plan's Global Constraints now state.

`$SCRATCH/c26.txt`:

```text
docs(seams): name where phases 3, 4, 5 and 8 plug into phase 2, and pin each one

docs/SEAMS.md names thirty-two seams: the file, what the later phase does
there, what it must not do, and the test that fails if the seam moves. It is
living - a phase that changes a seam changes its row in the same commit - and
desktop/src/seams.test.ts refuses to pass until the document and the manifest
name the same set.

The manifest pins each seam by an anchor in the implementation, never a comment,
across desktop/ and control-plane/, and re-expresses task 15's contract section 2
byte gate as a test so it runs on every npm test. A reflection test pins
backend.Backend's method set: the compile-time assertions in dockerbe and katabe
catch an implementation falling behind the interface, and nothing caught the
interface moving with both implementations changed to match.

Five lens stubs close AppShell.tsx for the programme (gate-zero 0b): each
renders exactly the placeholder its arm rendered, so no test changed, and four
phases no longer edit one import block.

Open carry 12 is ruled: proxy.Relay had no production caller - re-proved by an
anchored grep at the moment of deletion - and is deleted; proxy.RelayHooked is
the one relay.

katabe's package comment said turning Kata on in Phase 8 was "a configuration
change and not a rewrite". It is not: every create sends a host mount and a
working directory, and katabe correctly refuses both, so every create under
Kata fails today. The comment now says so, and names getting the worktree into
the pod as the first thing Phase 8 must build (seam S8.3).

Three seams are gaps with a pending decision, pinned in their current state so
the decision's arrival fails the manifest: who calls Materialise (0c), how Kata
receives a worktree, and who owns the Files lens. SX.10 pins the resume fix's
mechanism - Claude's own session id and a persisted conversation store - which
Phase 8 must carry into pods.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

```bash
git add docs/SEAMS.md desktop/src/seams.test.ts control-plane/internal/backend/seam_test.go \
        desktop/src/components/lens/DiscoverLens.tsx desktop/src/components/lens/DesignLens.tsx \
        desktop/src/components/lens/FilesLens.tsx desktop/src/components/lens/MergeLens.tsx \
        desktop/src/components/lens/DeployLens.tsx desktop/src/components/AppShell.tsx \
        control-plane/internal/proxy/proxy.go control-plane/internal/proxy/proxy_test.go \
        control-plane/internal/backend/backend.go control-plane/internal/backend/katabe/kata.go
git commit -F "$SCRATCH/c26.txt"
```

**What is not claimed.** The seams are named where the code was on the day of the commit, and pinned so that a move is loud. They are **not** a design for any later phase: every "the phase does" cell is the minimum the seam already supports, and a later phase's own plan decides everything past it. The three gaps are named, not closed; each is the orchestrator's, the product owner's or Phase 8's to decide, and the manifest is built to go red on the day each one is.






