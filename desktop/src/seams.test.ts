// Task 26. The named seams: where Phases 3, 4, 5 and 8 plug into what Phase 2 shipped.
// docs/SEAMS.md is the human half — what each seam is, what the phase does there, and
// what it must not do. This file is the mechanical half: an anchor per seam, read from
// disk, so a seam that moves fails HERE, with the document's path in the message, instead
// of being rediscovered by the phase that needed it.
//
// A seam that changes is not a failure to work around. Change the anchor here and the row
// in docs/SEAMS.md in the same commit; the last test below fails until both agree.

import { beforeAll, describe, expect, it } from "vitest";

// The files are collected with Vite's `import.meta.glob`, lazy and `?raw`, not read with node:fs,
// for the measured reason in styles/designsystem.test.ts's header: `@types/node` is not a
// dependency, so a node:fs import is TS2591 under `npm run typecheck`. The glob is expanded from
// disk on every run, so its key set IS "what exists": a file that moves drops out of it, and a
// file a later task creates (task 25's harness) joins it. Every pattern is relative to this file,
// desktop/src/, and reaches the rest of the repository through ../../.
const LOADERS = import.meta.glob<string>(
  [
    "../../control-plane/**/*.go",
    "../../desktop/src/**/*.{ts,tsx,css}",
    "../../desktop/e2e/**/*.mjs",
    "../../docs/SEAMS.md",
    "../../docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md",
    "../../docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md",
    "../../docs/superpowers/plans/phase-2-parts/task-*.md",
    "../../images/sandbox/Dockerfile",
  ],
  { query: "?raw", import: "default" },
);

/** A glob key, relative to desktop/src/ (or root-absolute), as a repo-relative path. */
function repoPath(key: string): string {
  const out = key.startsWith("/") ? ["desktop"] : ["desktop", "src"];
  for (const part of key.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") out.pop();
    else out.push(part);
  }
  return out.join("/");
}
const FILES = new Map(Object.entries(LOADERS).map(([key, load]) => [repoPath(key), load]));
const exists = (rel: string): boolean => FILES.has(rel);

/** Filled by beforeAll with every file a seam names, so each test below stays synchronous. */
const LOADED = new Map<string, string>();
/** Normalised to \n: this checkout mixes CRLF and LF (survey-phase-2-substrate.md:69-70). */
function text(rel: string): string {
  const source = LOADED.get(rel);
  if (source === undefined) throw new Error(`seams.test.ts did not load ${rel}`);
  return source;
}

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

/** Non-test sources under a directory — TypeScript, or Go for a control-plane rule —
 *  repo-relative and forward-slashed. */
function sourcesUnder(dir: string): string[] {
  const isSource = (f: string): boolean =>
    (/\.tsx?$/.test(f) && !/\.test\.tsx?$/.test(f)) || (/\.go$/.test(f) && !/_test\.go$/.test(f));
  return [...FILES.keys()]
    .filter((f) => f.startsWith(`${dir}/`) && isSource(f))
    .filter((f) => !f.startsWith("desktop/src/test/"))
    .sort();
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

interface SeamTest {
  file: string;
  decl: RegExp;
  /** The task that creates `file`, when it has not run yet. While the file is absent the entry
   *  waits; the day it appears the entry FAILS until `pending` is deleted here and docs/SEAMS.md
   *  cites what that task proved. Task 26 ran before task 25 (docs/superpowers/RESUME.md's
   *  close-out order), so the first pending entry was task 25's harness. The waiting task's plan
   *  must say to clear it; the "hands every pending entry" test below fails until it does. */
  pending?: string;
}

interface Seam {
  /** The row id in docs/SEAMS.md. */
  id: string;
  name: string;
  anchors: Anchor[];
  trees?: TreeRule[];
  /** Behaviour tests the seam's contract rests on, by declaration, not by title. */
  tests?: SeamTest[];
}

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
      // Task 25's headed harness. Task 26 ran first, so this waits for it (see SeamTest.pending).
      { file: "desktop/e2e/phase2.e2e.mjs", decl: /^check\("o2\.resume",/m, pending: "task 25" },
      { file: "control-plane/internal/api/sessions_resume_test.go", decl: /^func TestResumePassesTheSourcesConversationNotTheCockpitID\(/m },
      { file: "control-plane/internal/api/sessions_resume_test.go", decl: /^func TestCreateMountsOnlyThisWorktreesConversationStore\(/m },
      { file: "control-plane/internal/harness/argv_test.go", decl: /^func TestBuildRefusesAForkThatReusesTheSourceID\(/m },
    ],
  },

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
    name: "the Materialise window in handleSessionCreate — ruled (R1), reserved in prose, not built",
    anchors: [
      { file: "control-plane/internal/api/sessions.go", present: /^func \(a \*API\) handleSessionCreate\(w http\.ResponseWriter, r \*http\.Request\) \{$/m },
      // Decision 0c is ruled (orchestrator-rulings.md R1) and not built. The day api.Deps grows
      // the materialiser R1 names, this goes red and docs/SEAMS.md's S4.2 row says where it went.
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
    // One rule, by where the name may appear rather than where it may not. Three rounds of rules
    // that each listed a way to reach the constructor (`new`, a global's member, bound, passed,
    // destructured, a string) each had a survivor they did not list: a value (mutation A1), a
    // destructuring (TS-M1, TS-M5), an alias of the global (review 2 m1, TS-R1). So either name,
    // anywhere in code (a string included), trips it unless it follows a type position's
    // token (`:`, a single `|` or `&`, `<`, `as`, `typeof`) or reads a constant (`WebSocket.OPEN`).
    // attachRegistry.ts's `ws: WebSocket | null` stays clear. A value after a colon (an object
    // literal's `{ open: WebSocket }`, a ternary's else branch) reads like an annotation, and a
    // name built at run time (`globalThis["Web" + "Socket"]`, fix round 2's predicted survivor
    // TS-X1, which survived) is not the word, so neither is seen. Other type positions
    // (`Map<string, WebSocket>`) and a string that names either word trip it, loudly: reword the
    // string, or add the position here and to the row.
    trees: [
      {
        dir: "desktop/src",
        absent: /(?<!(?::|(?<![|&])[|&]|<|\bas|\btypeof)\s*)\b(?:EventSource|WebSocket)\b(?!\s*\.\s*[A-Z][A-Z_]*\b)/,
        except: ["desktop/src/api/client.ts"],
      },
    ],
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
      // Nor a wrapper under another name: a three-line relay beside RelayHooked is what Relay was.
      { file: "control-plane/internal/proxy/proxy.go", absent: /^func (?!RelayHooked\()\w*Relay\w*\(/m },
      { file: "control-plane/internal/api/attach.go", present: /\bproxy\.RelayHooked\(r\.Context\(\), client, upstream, / },
      // And that call is the only one: a wrapper under any name has to call RelayHooked.
      { file: "control-plane/internal/api/attach.go", absent: /\bRelayHooked\b[\s\S]*\bRelayHooked\b/ },
    ],
    // And not in another file: `func Relay(` re-added as proxy/relay.go, or a method, passed the
    // proxy.go anchors (mutation A2 of task 26's report). Any exported-style *Relay* function or
    // method in the control plane's Go, other than RelayHooked; attach.go's relayAttached is
    // the handler that calls it, and its lower-case `relay` does not match.
    // Nor as a function value: `var Relay = func(…) error { return RelayHooked(…) }` is the deleted
    // wrapper in another spelling (task 26 review m3, mutant G-M1). So a package-level var or const
    // named *Relay*, alone or inside a `var (` / `const (` group, trips it too.
    trees: [
      { dir: "control-plane", absent: /^func\s+(?:\([^)]*\)\s*)?(?!RelayHooked\()\w*Relay\w*\s*\(/m, except: [] },
      {
        dir: "control-plane",
        absent: /^(?:var|const)\s+(?!RelayHooked\b)\w*Relay\w*\b|^(?:var|const)\s*\((?:\n(?!\)).*)*?\n\s+(?!RelayHooked\b)\w*Relay\w*\b/m,
        except: [],
      },
      // And by what a wrapper does rather than what it is called: `func Pipe(…) { return
      // RelayHooked(…) }` passed every name rule (mutant G-M4). RelayHooked is named in code only by
      // its declaration and attach.go's one call (the anchor above holds attach.go to one).
      { dir: "control-plane", absent: /(?<!^func )\bRelayHooked\b/m, except: ["control-plane/internal/api/attach.go"] },
      // And by what a relay holds: two connections. `func Pipe(ctx, client, upstream
      // *websocket.Conn) error` with its own copy loop passed every rule above (task 26 review 2
      // m2, mutant G-R1). So a connection type (`*websocket.Conn`, or any package's `Conn`, such as
      // the `net.Conn` that websocket.NetConn returns) in code, as a parameter, a func literal's, a
      // field, a var or an element type, trips it everywhere but RelayHooked's parameters, its
      // copyMessages loop and relayAttached, the handler that calls it. A result, `(*websocket.Conn,
      // error)` from Dial and both Attach, does not. A relay whose connections are never named by
      // type (two `:=` locals, inferred) is invisible to it, which the S8.7 row says.
      {
        dir: "control-plane",
        absent:
          /(?<!\bfunc RelayHooked\([^)]*|\bcopyMessages := func\([^)]*|\bfunc \(a \*API\) relayAttached\([^)]*|\(\*?)\b\w+\.Conn\b/,
        except: [],
      },
      // And by what a relay moves: messages. One whose connections are typed by a local interface
      // (fix round 2's predicted survivor G-X1, which survived the rule above) names the message
      // type in that interface; one that wraps them as streams names websocket.NetConn and a
      // message kind. No control-plane code names any of the four today: RelayHooked's loop infers
      // its `typ`. Phase 8 writing its own frame with a named kind trips it, loudly.
      { dir: "control-plane", absent: /\b\w+\.(?:MessageType|MessageBinary|MessageText|NetConn)\b/, except: [] },
    ],
  },
  /* The six mounts after the worktree, in four seams, each refused by katabe's `len(spec.Mounts) > 0`
   * (S8.3's anchor); Go contract §2.6 and backend.Mount's doc comment record every one. One seam per
   * thing Phase 8 must provide another way, so the day it provides one, exactly one row moves: S8.8
   * holds the store and a fork's carried transcript, S8.11 the two git files. */
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
      { file: "control-plane/internal/api/sessions.go", present: /^\t\tenv\["NODE_EXTRA_CA_CERTS"\] = sessionCABundlePath$/m },
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

const UI_CONTRACT = "docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md";
const GO_CONTRACT = "docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md";
const KATA = "control-plane/internal/backend/katabe/kata.go";
/** The plan of the task a pending entry waits for: "task 25" is phase-2-parts/task-25.md. */
function planOf(task: string): string {
  const n = /^task (\d+)$/.exec(task);
  return n === null ? `(no plan for "${task}")` : `docs/superpowers/plans/phase-2-parts/task-${n[1].padStart(2, "0")}.md`;
}
/** Any phase's seam id: SX.n, or S<phase>.n for EVERY phase number, not only the four this task
 *  names. A narrower pattern let a document row for phase 6, 7 or 9 go uncounted, so a seam
 *  added to docs/SEAMS.md without an anchor passed (mutation M6 of task 26's report). */
const SEAM_ID = /^S(?:X|\d+)\.\d+$/;

describe("the named seams — docs/SEAMS.md", () => {
  // Every file a seam names, loaded once. A named file that does not exist is not loaded, and
  // the test that names it reports it; the generous timeout is for a saturated machine, where
  // Vite's first transform of a hundred raw modules can outlast the default hook timeout.
  beforeAll(async () => {
    const named = new Set<string>([DOC, UI_CONTRACT, "desktop/src/store/types.ts", GO_CONTRACT, KATA]);
    for (const seam of SEAMS) {
      for (const a of seam.anchors) named.add(a.file);
      for (const t of seam.tests ?? []) {
        named.add(t.file);
        if (t.pending !== undefined) named.add(planOf(t.pending));
      }
      for (const rule of seam.trees ?? []) for (const f of sourcesUnder(rule.dir)) named.add(f);
    }
    await Promise.all(
      [...named].map(async (rel) => {
        const load = FILES.get(rel);
        if (load !== undefined) LOADED.set(rel, (await load()).replace(/\r\n/g, "\n"));
      }),
    );
  }, 120_000);

  it("names every seam once, with an id of the form the document uses", () => {
    const ids = SEAMS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.filter((id) => !SEAM_ID.test(id))).toEqual([]);
  });

  it("still has every seam's anchor where the document says it is", () => {
    const moved: string[] = [];
    for (const seam of SEAMS) {
      for (const a of seam.anchors) {
        if (!exists(a.file)) {
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
        if (!exists(t.file)) {
          // A pending entry waits for the task that creates its file; any other is missing.
          if (t.pending === undefined) missing.push(`${seam.id}: ${t.file} does not exist`);
        } else if (t.pending !== undefined) {
          missing.push(
            `${seam.id}: ${t.file} now exists, so ${t.pending} has landed — delete this entry's ` +
              `\`pending\` and make ${DOC}'s ${seam.id} row and note cite what ${t.pending} proved`,
          );
        } else if (!t.decl.test(text(t.file))) {
          missing.push(`${seam.id}: ${t.file} lost ${t.decl}`);
        }
      }
    }
    expect(missing).toEqual([]);
  });

  it("hands every pending entry to the plan of the task that clears it", () => {
    // A pending entry goes red the day its task lands, so that task's plan must say to clear it,
    // or its own final gate fails for a reason it was never given (task 26 review I2; shared
    // shape §1 rules 1-2: the task that makes the insertion point names its consumer). One
    // paragraph of the plan names this file and the seam, and says to delete the `pending` field.
    // A paragraph that only mentions the entry does not count: task 25's Interfaces block does
    // that, and it passed with the instruction deleted (mutant I2-M).
    const deletes = /\bdelete\b[^.]*`pending`/i;
    const unhanded: string[] = [];
    for (const seam of SEAMS) {
      for (const t of seam.tests ?? []) {
        if (t.pending === undefined) continue;
        const plan = planOf(t.pending);
        if (!exists(plan)) {
          unhanded.push(`${seam.id}: ${t.pending}'s plan ${plan} does not exist`);
          continue;
        }
        const told = text(plan)
          .split(/\n[ \t]*\n/)
          .some((p) => p.includes("desktop/src/seams.test.ts") && p.includes(seam.id) && deletes.test(p));
        if (!told) {
          unhanded.push(`${seam.id}: ${plan} never tells ${t.pending} to delete this entry's \`pending\``);
        }
        // The handoff makes that task edit this file and DOC, so its commits name both, and so
        // does its pre-flight dirty-path check: `git commit -- <path>` takes the path's whole
        // working tree, so another agent's uncommitted edit in either would ride in that task's
        // commit, and nothing after the commit can tell (task 26 review 2, I1).
        const commands = text(plan).replace(/\\\n/g, " ").split("\n");
        const paths = (c: string): string[] =>
          (c.replace(/#.*$/, "").split(/\s--\s/)[1] ?? "").split(/\s+/).filter(Boolean);
        const committed = commands.filter((c) => /^git commit\s/.test(c)).flatMap(paths);
        const dirtyAt = commands.findIndex((c) => /^#.*\bis dirty\b/.test(c));
        const dirty = paths(commands.find((c, i) => i > dirtyAt && /^git status --porcelain\s/.test(c)) ?? "");
        for (const handed of ["desktop/src/seams.test.ts", DOC]) {
          if (!committed.includes(handed)) {
            unhanded.push(`${seam.id}: no \`git commit … --\` in ${plan} names ${handed}`);
          }
          if (dirtyAt < 0 || !dirty.some((d) => handed === d || handed.startsWith(`${d}/`))) {
            unhanded.push(`${seam.id}: ${plan}'s "… is dirty" check does not cover ${handed}`);
          }
        }
      }
    }
    expect(unhanded).toEqual([]);
  });

  it("S8.3: katabe's package comment is byte-identical to Go contract §2.6's copy", () => {
    // The contract's copy kept "a configuration change and not a rewrite" after kata.go dropped
    // it as false (task 26 review I1). A restatement that is a verbatim copy is gated like
    // types.ts (SX.2), so the next edit to either one has to move both.
    const header = (source: string): string => {
      const end = source.indexOf("\npackage katabe\n");
      expect(end).toBeGreaterThan(0);
      return source.slice(0, end);
    };
    const lines = text(GO_CONTRACT).split("\n");
    const heading = lines.findIndex((l) => l.startsWith("### 2.6 `internal/backend/katabe`"));
    const open = lines.findIndex((l, i) => i > heading && l === `${FENCE}go`);
    const close = lines.findIndex((l, i) => i > open && l === FENCE);
    expect([heading, open, close].every((i) => i >= 0)).toBe(true);
    expect(header(lines.slice(open + 1, close).join("\n") + "\n")).toBe(header(text(KATA)));
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
    expect(exists(DOC)).toBe(true);
    const rows = [...text(DOC).matchAll(/^\|\s*(S(?:X|\d+)\.\d+)\s*\|/gm)].map((m) => m[1]);
    expect(rows.length).toBe(new Set(rows).size); // one table row per seam, never two
    expect([...rows].sort()).toEqual(SEAMS.map((s) => s.id).sort());
  });
});
