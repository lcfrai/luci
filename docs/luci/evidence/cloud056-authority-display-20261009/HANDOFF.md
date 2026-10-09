# CLOUD-056 correction handoff

Root-owned integration is pending. Three production files are staged under `after/`; `cloud056.patch` applies against their exact edd0 bytes. Repository-ready test adaptation is under `repo-ready/desktop/src/components/teams/TeamWorkspaceAuthority.test.tsx`. No repository file was modified.

## Executed evidence

All runs used existing task-local Node24.19.0/npm11.9.0, one Vitest worker and a120second deadline. No timeout occurred. Forty-two selected repository inputs matched edd0 before and after; actual working HEAD was docs publication8c7bf16aa035b27dbea185b565052495eb60777f. This is edd0-source-identical evidence, not a clean edd0 whole-checkout claim or full compiled-input closure.

* `original-attempt1`: initial ten-case diagnostic, nine controls pass and one desired visibility assertion fails;6.476seconds. Original Vite CommonJS/ESM config warning retained. This was a product counterexample, not a test-loader failure.
* `regression-original-attempt1`: stable fourteen-case regression,9pass/5expected failures;4.298seconds. Fresh-denial hide, restored membership without old project grants, transient-failure hide, coalesced revalidation and failed catalogue recovery fail as expected.
* `regression-fixed-attempt1`: **same fourteen-case test/config** with exactly three external source overlays,14pass/0fail;3.766seconds. Raw JSON/stdout/stderr, actual commands, source/tool snapshots and frozen inputs retained separately. Counts overlap with the original diagnostic.

No actual app server, browser, Docker, provider or worker ran. Controlled React uses real provider/client/poller/pane/Terminal/registry with fake transport and xterm buffer projection. It establishes DOM ancestry/hidden-state retention, not real browser paint, first-frame privacy timing, full-shell accessibility, backend mutation behavior or native Windows behavior. The staged Go server refusal control has **not** run.

## Behavior

After existing request-generation and body-completion guards, only the exact unfiltered team-session catalogue403 `team_access_denied` invalidates the current workspace display.401 and general403/network behavior remain distinct. The provider hides private children immediately, preserves the selected workspace and mounted drafts, pauses writes/execution, and coalesces fresh membership verification. Failed or superseded verification never reopens private UI. New membership alone does not restore former project access: fresh project and session catalogues must prove current visibility before unblocking. Readable session scope preserves newer allocation rows when appropriate; hidden/missing rows are pruned through current catalogue/store operations.

Mounted editors deliberately retain their previous read projection while the parent is hidden. **Hidden mounted readers may still make server-guarded reads.** This correction does not promise blanket request suspension, complete browser-memory erasure or cancellation of server work. Backend authority remains independent. The comment was clarified after the passing fixed run; it is the only production-file difference from that run's frozen copies.

Transient revalidation failure retains drafts and terminal historical bytes, keeps views hidden and offers Retry. Confirmed absent membership follows the existing provider's terminal-disposal behavior; it does not delete durable project/session history. Scope/account changes fence late success and denial. Current provider epochs and C054 opaque execution-generation guards are preserved.

## Remaining integration validation

After the root's installed quiet window, integrate the three-file patch and relative-import regression, then run affected existing TeamWorkspaceProvider, client/auth/team-scope, poller and terminal/attach tests plus TypeScript. Confirm the existing canonical contract mirror only if required by actual modified shared contracts; there is no public API schema change here. Add any newly identified focused negative cases rather than rerunning unrelated full suites. Run the separate Go registrar control if useful for joining the real403 contract to the controlled frontend stimulus. Windows/full installed authority behavior remains a separate acceptance gate.

The repository-ready regression rewrites absolute staging imports to repository-relative imports only; that adapted file is not independently executed yet. Do not call the source patch integrated or released until root commits it and validates the actual integrated candidate.

`correction-manifest.json` SHA256: `79b4e16688e4680c506059aebad2349d7489c3811be58c5028ee9c6947fb962b`.
