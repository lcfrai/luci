# Cloud032 critical-path source review

Date: 2026-10-08. Checkout: `/workspace/luci`, inspected for the parent task's d348f012 diagnostic. Source line numbers in the review sections below refer to the checkout inspected before implementation. The initial review made no application source changes and ran no tests, compiler, browser, benchmark, or runtime actions. A subsequent explicitly authorized implementation is recorded at the end of this note.

## Evidence boundary

The parent supplied these measurements from the latest diagnostic; this reviewer did not independently collect them:

- 198 of 200 input events matched the browser instrumentation.
- `x` input handler: p95 18 ms, maximum 35.9 ms.
- Backspace input handler: p95 5.9 ms, maximum 13.4 ms.
- The largest approximately 240 ms input observation included a 203 ms Node action, a 7.6 ms browser input handler, and 21.1 ms browser dispatch-to-expected-DOM interval.

These figures do not establish synchronous draft persistence, React rendering, or global draft registration as the cause of the measured tails. The intervals have different endpoints and cannot be treated as a decomposition of the total without additional correlated evidence. The two suggestions below are small reductions in demonstrably repeated source work, not a prediction that either will achieve the switch or input p95 targets. Browser/Node scheduling and automation overhead remain possible contributors. Do not describe the source findings as an attribution of the measured regression.

The already integrated absent-draft-clear and overview-pruning no-op fixes are not proposed again here.

## Exact measured input path

1. `desktop/src/components/definition/DefinitionWorkspace.tsx:233`: the `The idea` textarea calls `patch({ purpose: e.target.value })`.
2. `DefinitionWorkspace.tsx:139`: `patch` checks editing access, current saved-read readiness, mutation state, and pending recovery; it then replaces `spec` and sets `dirty=true`. Returning to the initial characters with Backspace does not reset `dirty` by content equality.
3. `DefinitionWorkspace.tsx:88`: `useProjectDraft` receives current `spec` and the saved baseline.
4. `desktop/src/hooks/useProjectDraft.ts:64`: a layout effect checks ownership of the current localStorage value, serializes the full document plus original baseline, scans other draft entries for expiry and quota accounting, and synchronously writes the current draft before paint. This occurs for each changed document value while dirty and eligible for retention. It is a real work path, but the supplied measurements do not implicate it in the long end-to-end observations.
5. `desktop/src/hooks/useUnsavedProjectDraft.ts:13`: draft registration depends on dirty/retained and stable project metadata, not document value. The first dirty edit can register retained=false, then clear/re-register after retained becomes true. Once those values stabilize, repeated x/Backspace on the same mounted editor should not repeatedly register global drafts.
6. `desktop/src/components/teams/TeamWorkspaceProvider.tsx:45`: real registry transitions rerender the provider. Its currently fresh `select` callback and provider object at lines 94 and 99 broadcast a new context to consumers even when membership and permissions have not changed.

## Preferred change 1: avoid constructing inactive definition children

Location: `desktop/src/components/definition/DefinitionWorkspace.tsx:204` through line 276. There are four entries in `views` at line 20. Every parent render currently constructs the current tab's complete children within each of the four map iterations.

`desktop/src/components/fluid/components/tabs-subtle.tsx:443` already renders only the selected panel's children at line 458. Thus the inactive children are constructed as JavaScript element trees but are not mounted. This is not four copies of the mounted form.

Suggested small implementation, local to DefinitionWorkspace:

```tsx
{views.map(([key], index) => (
  <TabsSubtlePanel key={key} idPrefix={tabsId}
    index={index} selectedIndex={selectedIndex}>
    {index === selectedIndex && (
      <div className="definition-layout" /* existing props */>
        {/* Existing panel children, unchanged. */}
      </div>
    )}
  </TabsSubtlePanel>
))}
```

This prevents three unused inner element-tree constructions per editor render. No change to the shared TabsSubtlePanel component is necessary.

Invariants:

- Keep all four outer panel nodes, their keys, IDs, tabIndex, hidden state, and aria-labelledby/aria-controls relationship.
- Keep manual keyboard activation: moving tab focus must not select a tab or request revision history.
- Preserve the existing parent-owned document, dirty state, draft recovery, tab state, and exact question-focus behavior.
- Retain existing selected-content mount/unmount behavior; inactive content was already unmounted by TabsSubtlePanel.
- No request scheduling, permission, save/approve revision, recovery, or provider-execution behavior changes.

Relevant existing tests to run once when implementation is scheduled (read here, not executed):

- `desktop/src/components/definition/DefinitionWorkspace.test.tsx:80`: `activates linked definition panels manually without discarding edited answers` verifies edited text survives tab changes, keyboard focus alone does not fetch history, and selected tab/panel IDs remain linked.
- `DefinitionWorkspace.test.tsx:22`: `opens the exact question answer after a delayed load and focuses only once without saving or invoking an agent` protects delayed question navigation and focus.
- `DefinitionWorkspace.test.tsx:40`: `waits for explicit recovery before focusing a reordered question and preserves the recovered answers` protects interaction with recovery.
- `DefinitionWorkspace.test.tsx:69`: `keeps requirement identities while changing acceptance words` exercises controlled editing and saved identity.
- `DefinitionWorkspace.test.tsx:143`: `allows viewers to read shared definition while disabling edits, approval and provider execution` protects authority gates.

The existing complete DefinitionWorkspace test file is a sensible bounded verification target for this guard. Timing assertions or tests that merely duplicate the JSX condition would not establish a user benefit.

## Preferred change 2: keep team context stable across registry-only changes

Locations: `desktop/src/components/teams/TeamWorkspaceProvider.tsx:94` and line 99. The provider still needs its draft subscription for the switch dialog; eliminating that subscription would change behavior. Instead stabilize the outward context independently of draft-only provider rerenders.

Suggested implementation shape:

```tsx
// Add useMemo to the existing React import.
const select = useCallback((id: string) => {
  if (managedTeamId && id !== managedTeamId) return;
  if (id === selectedTeamId()) return;
  // Read live registry state at action time; do not capture a stale draft list.
  if (Object.keys(useCockpit.getState().unsavedProjectDrafts).length) {
    setPending(id);
  } else {
    apply(id);
  }
}, [managedTeamId, apply]);

const value = useMemo(() => ({
  teams,
  selected: teams.find(row => row.team.id === selected) ?? null,
  teamId: selected,
  managedTeamId,
  revision,
  refresh,
  select,
}), [teams, selected, managedTeamId, revision, refresh, select]);
```

`apply` and `refresh` are already callbacks at lines 46 and 55. The dialog's `retained` and `localSetup` derivation at lines 100 and 101 remains driven by the current registry. The proposal does not claim to eliminate all descendant renders; it prevents avoidable context broadcasts when the context's meaning is unchanged.

Invariants:

- New membership objects, selected team, managed binding, or revision must still publish changed context. Include every listed dependency; do not compare away revision updates.
- `useTeamProjectPermissions` at line 121 must continue revalidating by team, project, and revision. During refresh retain only permitted prior read projection and disable writes/execution as it does now.
- `select` must inspect current registry and current selected-team state when clicked. It must keep the managed-team restriction, draft dialog, explicit keep/discard choices, and `apply` behavior.
- Preserve initial membership admission, account/installation key isolation, hiding of revoked private views, in-flight result fences, and reset/remount on actual workspace switches.
- Do not cache permissions or authoritative project documents as part of this change.

Relevant existing tests to run once when implementation is scheduled (read here, not executed):

- `desktop/src/components/teams/TeamWorkspaceProvider.test.tsx:289`: `keeps an unsaved editor mounted while refreshing access and disables every stale write grant` verifies the same editor node/draft survives while stale grants are disabled and fresh grants restore.
- `TeamWorkspaceProvider.test.tsx:321`: `retains only prior read access after a temporary project lookup failure, then recovers on refresh` verifies transient failure, recovery, and confirmed denial.
- `TeamWorkspaceProvider.test.tsx:341`: `hides a revoked team without switching to personal authority or discarding its mounted drafts` protects revocation behavior.
- `TeamWorkspaceProvider.test.tsx:363`: `clears confirmed denied project permissions and rejects late results from a different project` protects scope/result fences.
- `TeamWorkspaceProvider.test.tsx:393`: `asks before switching a workspace with unsaved drafts (retained=%s)` covers both retained values and current switch guards.
- `TeamWorkspaceProvider.test.tsx:413`: `keeps local check drafts protected on workspace change without suggesting a nonexistent shared save` protects the verification-setup branch.
- `TeamWorkspaceProvider.test.tsx:62`: `resolves the remembered team before mounting project content or issuing project requests` and line 76, `opens the managed team regardless of a personal preference and refuses workspace switching`, protect admission and managed binding.

The existing complete TeamWorkspaceProvider test file is the bounded semantic verification target. If a separate regression check is warranted, one focused consumer observation could verify that draft-only registration leaves context identity unchanged while a real refresh publishes a new revision. Do not replace the authority/refresh behavior checks with render-count assertions.

For a batch implementing both preferred changes, run these two existing component test files once using the repository's Vitest command from `/workspace/luci/desktop`; source `/workspace/.luci-tools/activate.sh` first. Testing is deferred until the parent schedules it. No test command was executed in this review, and passing semantic tests would not establish latency targets.

## Switch path and auxiliary reads

- `desktop/src/components/rail/Rail.tsx:164`: All projects calls selectProject(null). `desktop/src/components/rail/ProjectRow.tsx:44` and the overview card call selectProject(project.id).
- `desktop/src/store/slices/uiSlice.ts:208`: validates selection and preserves pending-navigation draft guards. `navigationState` at line 46 restores per-project stage and persists scoped preferences at line 66. `desktop/src/preferences/projectNavigation.ts:50` synchronously serializes/bounds navigation records. Zustand preference persistence in `desktop/src/store/store.ts:72` also remains synchronous.
- `desktop/src/components/AppShell.tsx:148`: home/lens replacement; line 151 keys the lens by project/source revision; line 154 mounts OrchestratorPanel for each selected project.
- `desktop/src/components/definition/DefinitionWorkspace.tsx:113`: fresh specification request; editing remains gated until its saved-read state is ready. `desktop/src/api/definition.ts:53` routes GET to `/v1/product/projects/{id}/specification`.
- `desktop/src/components/orchestrator/OrchestratorPanel.tsx:70`: agent-connections hook; line 135 requests current orchestrator view; the effect beginning at line 157 triggers fresh reads on read permission, source revision, and lens changes. These reads occur even if the responsive overview starts collapsed.
- `desktop/src/hooks/useAgentConnections.ts:20` reloads the agent catalogue on each enabled hook mount and on the agent-connections-changed event; it is separate from the specification read.
- In team mode, separate useProjectAccess calls in definition and orchestrator create separate permission-hook requests (`TeamWorkspaceProvider.tsx:131`). Personal mode resolves permissions directly and does not issue those team project-access requests.
- `desktop/src/App.tsx:165`: catalogue subscription requests a selected project's branches only while its branch entry is absent. This is not an unconditional warm-switch branch refetch.
- `desktop/src/components/projects/ProjectOverviewBoard.tsx:52`: overview loads use AbortController; cleanup aborts requests when home unmounts or visible membership changes. Cross-project overview workers are not intentionally kept mounted inside a selected project.
- `desktop/src/components/journey/ContextStorage.tsx:24`: content is already gated by its open disclosure; no mount-time context-storage read when closed.

Secondary candidates, separate from the preferred small batch:

- Thread AbortSignal through read-only definition/orchestrator GET wrappers and abort obsolete reads on project unmount. `desktop/src/api/product.ts:481` already accepts a signal. Keep fresh destination GETs, permission checks, and existing generation/alive fences. Do not abort/retry mutations or infer their outcome from cancellation.
- `OrchestratorPanel.tsx:560` hides an already-rendered conversation, including turns, storage controls, and composer. A first-open rendering boundary could defer initially unseen children, then retain them once opened. Parent-owned recovery, durable request identity, pending-job observation, and authority gates must remain live. This needs its own focused interaction review and should not be mixed casually into the two small changes.
- Draft-storage metadata parsing could be reduced only if correlated profiling supports it. Preserve exact cross-tab raw-value ownership checks, quota accounting, expiry, and current durability. The supplied handler/DOM timings do not support attributing the large input observations to this code.

## Subsequently authorized team-context implementation

The parent then authorized implementation of preferred change 2, assigning this agent only `desktop/src/components/teams/TeamWorkspaceProvider.tsx` and `desktop/src/components/teams/TeamWorkspaceProvider.test.tsx`. Preferred change 1 belongs to the parent agent.

- Implemented `useCallback` selection with `[managedTeamId, apply]`, retaining the live `getState().unsavedProjectDrafts` check.
- Implemented context `useMemo` with `[teams, selected, managedTeamId, revision, refresh, select]`.
- Added a controlled memoized context consumer test proving draft add/update/remove does not broadcast context, a draft added after the consumer rendered still guards switching, and removing it allows switching.
- Added a controlled consumer test proving a refresh of the identical membership-list object still publishes a revision and revalidates project access; a subsequent member/project permission downgrade reaches consumers and disables editing.
- Executed only `npm test -- src/components/teams/TeamWorkspaceProvider.test.tsx` from `/workspace/luci/desktop`, after sourcing `/workspace/.luci-tools/activate.sh`: **1 test file passed, 28 tests passed, 4.76 seconds**, exit 0. Log: `/workspace/.luci-tools/cloud032-critical-path-d348/teams-component-test.log`.
- `git diff --check` on the two assigned files passed. This agent ran no compiler, browser, benchmark, or broader test suite. Coordinated typechecking and latency measurement remain with the parent. The semantic test result does not establish latency improvement.
