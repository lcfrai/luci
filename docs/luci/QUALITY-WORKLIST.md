# Continuing product-quality work

The active objective remains [GOAL.md](GOAL.md), judged against
[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md). This is an implementation worklist,
not a replacement completion standard or an automatic background scheduler.
The current cloud session exposes no persistent-goal scheduling API.

## Current findings and required outcomes

| Finding | User consequence | Required correction and evidence | Status |
| --- | --- | --- | --- |
| Restoring a stale structured draft replaces a teammate's independently changed fields. | Recovering one person's constraints can silently undo another person's agreed outcome. | Preserve the actual original baseline; combine independent definition/design/task edits; require explicit choices for overlapping edits or unknown legacy baselines. Verify two people through the actual UI, retaining old run scope while reviewing the new scope. | Verified in the matching `a10d941e` two-person browser journey, including readable combined content before explicit restoration. Overlap, legacy and reference-repair regressions also pass. |
| Prototype review hides the screen's expected behavior and can reuse a consumed team browser invitation after navigation. | The person cannot compare the working result with the agreed experience or reliably return to it. | Keep routes, states and outcomes alongside the actual prototype; record source-bound human observations as unverified draft context; obtain a fresh explicit browser connection. | Verified end to end in `7ff66d3b` attempt 5: actual behavior, planned width, fresh invitation, source-bound observation and exact-screen amendment saved as an unapproved draft. |
| Deployment operations can say completed when a successful HTTP response carries a failed operation status. | The person receives a false success and no useful recovery path. | Display the actual operation outcome and remedy. Keep returning users connected to their retained release, source and health, with explicit supported cleanup/replacement controls. | Operation, permission and ownership checks pass. Matching `a10d941e` browser acceptance verifies retained release discovery, explicit owned stop, reviewed replacement and preview continuity through access refresh. |
| An unsent discussion disappears on remount; an uncertain send can lose its request identity. | Context is lost or the same intent can become a second paid request after refresh. | Scoped private draft recovery and durable exact-request identity; no automatic resend. Resolve acceptance or authoritative non-dispatch without racing a delayed original request. | Verified in the combined `7ff66d3b` attempt 5: unsent draft reload and pre-admission network-fault recovery retain exact text/request with zero model calls. Backend concurrency/receipt checks cover accepted and delayed requests separately. |
| Collaboration evidence stops before a complete amendment during active work. | Mechanism checks alone do not show people can understand and continue the combined workflow. | Two people preserve concurrent contributions, see old active scope versus new agreed context, renew review, build the amended behavior, retain a meaningful regression check, and inspect/accept the matching preview. | Matching `a10d941e` browser journey verifies active-scope understanding, both contributors’ edits, renewed review, actual CLI/checks, matching preview and acceptance. Provider remains a deterministic fixture, not live model reasoning. |
| Repository analysis can return a consequential question without a proposal, but the UI hides the question. | An imported application's discovery stops without explaining the decision needed to continue. | Show the question and editable answer, then explicitly continue analysis using its authoritative prior result and the selected connection. Preserve source/review checks and recover the reply draft. | Implemented with durable exact-request recovery, canonical answer ancestry, provider-revision consent and actual workspace-actor ownership. Complete source checks and six installed `e04fa35c` groups pass; the separate visual refinements below continue. Live model reasoning remains separate. |
| Stripe returns to the app without reopening subscription status. | A customer cannot see whether payment was verified or how to continue after checkout or cancellation. | Return to the authorized team's Service and data view, with explicit verification guidance and no entitlement inferred from the URL. Require an explicit choice before changing workspace. | Implemented with sign-in continuity, membership checks, explicit workspace choice and draft protection. Persisted return-URL version preserves existing Stripe idempotency parameters. Eight installed `e04fa35c` return groups pass using labelled billing fixtures; merchant acceptance remains separate. |
| Data controls invite operations outside the person's project permissions. | People encounter avoidable permission errors without knowing which access is required. | Explain read/edit/deploy/secret requirements, preserve drafts during access refresh, and fence operations after actual revocation. | Implemented across data, database, functions, email and domain controls. Delayed responses and downloads are fenced after permission changes; successful database recovery links directly to application deployments. Seven installed `e04fa35c` grant/refresh/revocation groups pass with labelled database display fixtures. |
| Completed analysis questions and restored answers sit below long settings/evidence, with no demonstrated transition from an option to editing. | The person can miss the consequential decision or fail to find the next action although the underlying request succeeds. | Prioritize the question, answer and explicit next action; focus the answer on an explicit user action without stealing focus on asynchronous completion. | Verified in the matching `7f39ea9e` installed journey: the reloaded question precedes the answer form, explicit answer actions focus the fully visible editor at 1440 and 390 px, and technical evidence stays behind disclosure. Asynchronous completion preserves the current scroll/focus. |
| The default expanded project overview consumes nearly half the phone viewport. | Context crowds out the actual work and the answer/send flow requires unnecessary nested scrolling. | A compact default at narrow widths, with an obvious context disclosure that respects explicit choices and retains drafts. | Verified at 390 × 844 in `7f39ea9e`: compact overview is 61 px and usable analysis area is 633 px, up from about 289 px. Explicit open/close retains the same draft DOM and text without sending work. Responsive/focus regressions also pass. |
| Platform permission refresh presents unconfirmed access as missing grants. | An authorized person is briefly told to ask an owner for permissions they already have. | Distinguish loading, transport failure and a confirmed missing grant without changing authorization or discarding work. | Verified in seven installed `7f39ea9e` permission groups: pending access says it is being checked; failed delivery gives retry guidance; actual secret revocation does not falsely claim deployment editing was revoked. |
| A programmatically selected team-settings tab is clipped on a phone. | Billing returns to the right panel but hides part of its selected label and focus ring. | Reveal the selected/focused tab within its own horizontal strip, including on resize, without moving the dialog or document. | Verified in eight installed `7f39ea9e` billing-return groups. At 390 px the selected tab and full focus ring fit within the strip; document/dialog scroll is unchanged. Four scoped scans have zero violations. One phone contrast item remains automated incomplete; a retained-image manual assessment finds unobscured text at 12.47:1, matching two saved passing observations. This is a sampled assessment, not full accessibility conformance. |
| Deployment and hosting fields use bare native controls against inline labels. | Core operation forms look inconsistent and provide poor spacing for reading and touch interaction. | Use locally imported platform styles, readable label spacing and responsive Luci fields while preserving validation and actions. | Verified in five installed `7f39ea9e` maintenance groups. Repair fields measure 350 × 44 px on a phone, with separated labels and complete keyboard focus outlines. The actual failed deployment row and its controls are visible; seven scoped accessibility scans have zero violations or incomplete items. |
| Data table, read-only and file fields still use bare native controls. | The data workflow has inconsistent readability, sizing and disabled states despite correct permission handling. | Apply scoped Luci field tokens and spacing, preserve native checkboxes and responsive columns, and verify real keyboard focus and draft retention. | Verified in clean `4c8012c1`: seven permission groups and four desktop/phone field observations pass. Both controls are 44 px high, labels and keyboard focus rings are fully visible, drafts survive, and 18 px checkboxes and zero horizontal overflow are retained. |
| Opening a new portable package silently reuses an older running package. | An update appears successful while the person is still using old code and UI. | Compare the running service with the selected portable package through its existing local ownership boundary; explain how to close the previous launcher before upgrading, preserving state and processes. | Five installed `1b0d8043` upgrade checks pass: legacy-service refusal preserves the process and 26 private/project files; explicit replacement preserves account/session/context; same-package reuse and cleanup pass. Separate verification binds the retained old extraction to all 939 archive members. Native Windows acceptance remains separate. |
| Removing a requirement leaves an invisible invalid link in its design screen. | Saving the design is blocked with no visible link to remove; deleting the whole screen would lose unrelated work. | Show unavailable linked requirements with explicit removal; retain all other screen content and existing approval semantics. | Source correction passes the 47-test Design batch and complete 1,887-test frontend suite. Five installed `1b0d8043` groups pass: actual definition removal, explicit unlink, retained screen fields/history, save/reload and separate approval. |
| Typing a comma in responsive widths immediately removes it. | Normal typing joins two widths into an invalid number although filling the entire value at once works. | Preserve raw edits and give recoverable validation before saving parsed widths; keep dirty-state and draft recovery intact. | Per-keystroke, exact draft reload and teammate width-conflict regressions pass. The same five installed groups verify ordinary keystrokes, the unfinished comma, blur/tab retention, reload/Restore, numeric persistence and renewed approval. |
| A generic untouched starter can become an approved design handoff. | A role-based application's design can advance to Develop without showing how its users actually complete the proposed tasks. | Keep starter material distinguishable from an authored, reviewable design; guide meaningful screen/role decisions before approval without inventing an architecture or accepting a rubber-stamp checkbox. | Explicit authoring, saved people/outcome guidance, unsent help and separate draft/approval guidance pass the full 1,890-test frontend suite and four installed `bfcd5cb1` groups. Four desktop/phone accessibility scans have no violations or incomplete findings; no live-model design claim. |
| The compiled service ignores help/version/unknown arguments and starts normally. | An operator asking for help can initialize default state and attempt to start a service. | Handle informational and rejected arguments before process protection, configuration or service setup; preserve normal no-argument startup and exit codes. | Early argument handling passes 21 focused outcomes and eight isolated source-binary checks. All eight exact packaged `bfcd5cb1` checks also pass: invalid config is bypassed for informational/rejected arguments, no fresh state is created, secret-shaped values are not echoed, and ordinary no-argument refusal is retained. |
| Manual questions route to chat and disappear from the overview once answered. | People lose the direct path to an existing answer field and cannot see recorded answers across stages. | Target the question's existing Define editor and retain saved Q&A with draft/reviewed status in the shared overview. | Implemented; complete 1,944-test frontend suite and type checking pass. Four installed bb720e51 continuity groups pass; explicit UI answers/reviews and recovery use the archive-bound app. |
| Custom check commands and unfinished arguments disappear on navigation or reload. | Consulting the agreed design loses the verification work being prepared. | Recover scoped setup drafts without restoring execution consent or applying old criteria to changed context. | Implemented; complete 1,944-test frontend suite and type checking pass. Four installed bb720e51 continuity groups pass; explicit UI answers/reviews and recovery use the archive-bound app. |
| A prepared run still invites coding after its reviewed context changes. | The visible next step contradicts the amendment; clicking it reaches an avoidable backend rejection. | Project current context freshness separately from historical run state, preserve active work and offer a useful review action. | Read-only projection and guidance pass full source suites, including 481 product-package race outcomes; stale history and active sessions remain intact. Four installed bb720e51 run-guidance groups also pass with real context amendment and preserved history. |
| A new material question blocks execution while its build still invites coding. | The project overview and build disagree about the decision needed before proceeding. | Project the private decision blocker separately and open its existing scoped question without replacing drafts or sending anything. | Read-only projection, exact-question navigation and both overlapping-read orderings pass HTTP/UI coverage in the current full source suites. No implicit send or composer replacement; four installed bb720e51 groups verify projection and exact focus over explicitly seeded private question history, preserving unsent and recovered replies with zero execution. |
| A local verification draft blocks asking Luci for help. | The command being prepared has no shared Save operation, yet the composer demands saving or discarding it. | Distinguish local command setup from unsaved shared context; preserve navigation/source guards and explain that commands must be explicitly included in the message. | The `6e7797f3` correction passes the complete 1,979-test / 126-file frontend suite. Four installed groups pass there and again on `f388ad44`, verifying the explicit local-draft boundary and editable unsent help; the missing provider/model independently disables Send. |
| Empty coding panes squeeze verification recovery into about 185 px on a phone. | Reading and comparing exact commands requires repeated scrolling through fragments, although the actions work. | Give no-session planning and checks one continuous scroll area; retain active session layouts. | Installed `6e7797f3` recovery uses one scroller and 525 px on a phone versus 185 px before; `f388ad44` retains that result. Visible heading/action scans have no violations or incomplete findings. |
| Develop says there is no saved design after a structured design handoff is approved. | The next stage contradicts the review the person just completed. | Describe the actual structured handoff and its review dependencies; distinguish the optional sketch. | Independently reproduced on clean `bb720e51`; the `6e7797f3` correction passes full source checks and installed proof. Canonical handoff facts and exact plan focus pass again in `f388ad44`. |
| Review implementation tasks leaves the plan collapsed. | The primary next action reaches the stage but does not expose the work it names. | Open and focus the scoped task destination after guarded navigation, preserving drafts and read-only review. | Independently reproduced on clean `bb720e51`; the `6e7797f3` correction passes full source checks and installed proof. Canonical handoff facts and exact plan focus pass again in `f388ad44`. |
| A phone overview stays expanded after opening the exact task plan or answer. | The named destination receives focus but has too little room for the actual work. | Fold the overview to its stage bar only after an explicit exact destination succeeds; preserve cancellation and background behavior. | Verified in clean `f388ad44` first-attempt 4/4: phone plan and exact Answer destinations retain focus while the overview collapses; the plan uses 525 px with no driver scroll/focus. Desktop stays open. The narrow panel correction passes 86 tests and type checking, separate from the preceding full suite. |
| A viewer can type a new implementation task that cannot be saved. | Read-only review creates an unsaveable browser draft and unnecessary navigation guard. | Gate the new-task field and submission consistently while preserving an existing editor draft after downgrade. | Source correction passes 54 focused plan/navigation/recovery tests and type checking; matching installed proof is being prepared. |
| A delayed project backup can download after access changes or its view closes. | Source and Git history can arrive after the interface has lost the permission or scope that admitted the request. | Fence delayed downloads and notices by current account/workspace/project and required permission; a later regrant must not revive an old result. | Implemented; 59 focused backup/history/delivery tests and the complete current source batch pass. Ordinary retained-grant refresh is preserved; unconfirmed completion offers an explicit retry. Matching installed proof is next. |
| Git can commit files changed after the displayed review. | Another contributor's unseen changes become part of the commit offered for publication. | Bind commits to the reviewed branch, source and index; reject stale review before staging, preserve the message and retain edits made after the captured snapshot. | Implemented; exact captured source, branch/index conflicts, protected files, normalization metadata, split indexes, native recovery logs and read-only behavior have source regressions. The complete source batch passes; matching two-person installed proof is next. |

The current corrective batch passes 2,051 frontend tests across 128 files,
2,120 Go outcomes (22 skips), 513 product-package race outcomes (one skip),
and type checking. Exact source inputs remain unchanged; original fixture/audit
and temporary-disk failures are retained beside the final passing logs in
`artifacts/quality-collaboration-boundaries/`. Its matching installed checks are
next. The earlier compact-handoff receipt is
`artifacts/package-eval/0.5.0-f388ad44/context-continuity-attempt-1/result.json`,
with screenshots and `rendered-review.json` beside it. Six scoped accessibility
scans have zero violations; one clipped, nonfocused Define Answer label remains
an automated color-contrast incomplete. Initial account/context are labelled
API scaffolding; claimed navigation, saves/reviews and recovery are UI actions.
No model, command or Docker execution occurs. The complete 1,979-test / 126-file
suite belongs to `6e7797f3`; `f388ad44` adds the 86-test panel/type-check result
and matching installed proof. These results do not close real-model quality,
representative authentication/roles/durable data, managed-service or native and
public-release gates in [PRODUCT-QUALITY.md](PRODUCT-QUALITY.md).

Two further concrete findings have source corrections, with installed acceptance continuing:

- A failed repository import reserved its final project name. Correcting Git access and retrying the same name returned `project_exists`, leaving the entry flow stuck. Owned staging, private receipts and no-overwrite publication now pass four installed `7ff66d3b` browser groups: actual failed HTTPS clone, corrected same-name import with both original commits/files, lost completion response, and explicit read-only recovery without another clone. No private-host authentication or live repository-analysis quality is inferred.
- Structured draft comparison previously exposed only JSON. Definition, design and task recovery now show readable content and explicit choices, demonstrated in the `a10d941e` browser journey; exact technical records remain available through deliberate disclosure. Sketch recovery remains a separate opaque-document path.

The `2a60c1c0` source batch passes 1,701 frontend tests across 117 files, type
checking and production build, 1,997 Go test/subtest outcomes (22 skips), and
1,170 race outcomes across the three changed integration packages (five skips).
These checks do not replace the pending rendered journeys or live model proof.

Current implementation owners coordinate shared files before editing. Each
correction needs its scoped regression and rendered behavior where applicable;
failures stay recorded. Earlier `340f1590` results retain their original identities.

The clarification, billing-return and data-permission source batch passes all
1,874 frontend tests across 125 files, 2,045 Go outcomes (22 skips), and 1,230
changed-package race outcomes (five skips). Windows AMD64 and Darwin ARM64 API
test executables compile. The initial frontend run's obsolete recovery-copy
assertion and its focused navigation correction remain recorded alongside the
clean complete rerun in `artifacts/quality-clarification`. Installed acceptance
of the new batch remains separate from these source results.

## Gates that still need real external access

Live model reasoning and implementation, merchant commissioning, separately
billed model/infrastructure policy, owned managed/cloud hosts and domains, SMTP
delivery, signed distribution, and installed Windows diagnosis remain open.
GitHub currently rejects Actions runner admission for account payment/spending
reasons. No permission, credential, live result or production readiness is
inferred from elapsed time or from deterministic local fixtures.

After this worklist is verified, continue with the next unproven complete journey
and any independently actionable defect it exposes. Do not mark the overall goal
complete because these rows or a test suite pass.

## Current installed continuation evidence

The clean `2a60c1c0` and corrected `a10d941e` Linux packages each passed six
portable acceptance groups and seven launcher outcomes. Both have verified
archive/source integrity and licence evidence for all 346 components.
`a10d941e` was built from a clean isolated checkout while import work continued
separately. Its overall team-journey attempt is **not passed**: seven groups
succeeded, then the prototype handoff tab did not activate. Preserve this failed
receipt at `artifacts/package-eval/0.5.0-a10d941e/team-amendment-attempt-3/result.json`.

Earlier attempts remain recorded: attempt 1 stopped on an ambiguous driver
selector; attempt 2’s expected text was corrupted by missing UTF-8 metadata in
the fixture application. Both harness defects were corrected without weakening
outcome assertions. Separately, the deployment preview closed during routine
permission refresh; that product defect is fixed, with confirmed denial and
late-grant refusal preserved, and its actual browser regression now passes.

The rendered review also exposed an accepted-run summary that still repeated
its old acceptance instruction. The projection and guidance are corrected, retaining current invalidity and
health warnings; focused UI and actual HTTP run/health regressions pass. The handoff-tab interaction
is being reproduced separately before attributing or changing product behavior.

The clean `7ff66d3b` package passes six portable acceptance groups, seven launcher
outcomes and the four import/recovery groups above. Its archive and every installed
web file are independently bound to the recorded package provenance. Preserve
import attempts 1–3: Git identity setup, a wrong entry-button selector and a
child-exit callback race were driver defects. Attempt 4 passes strict cleanup
with exactly three import requests, three real clone starts and two recovery
reads. Four bounded accessibility scans have no violations; their contrast items
remain automated incomplete results. A separate entry-state manual diagnostic
supports the sampled header/help text contrast at both viewport sizes, with
original findings and scope limits retained. The full `7ff66d3b` team attempt 4
again passes seven groups and stops at prototype-to-handoff activation. Its trace
records no handoff pointer event. A bounded complete-layout reproduction identified
Chromium 151 sending the click into the preview iframe using its old pre-scroll
position. A fresh comparison with explicit scroll and settled tab/iframe geometry
delivered one ordinary click correctly. The corrected driver then passed all ten
combined groups in attempt 5; product handlers and behavior assertions were unchanged.
Both failed attempts and the direct comparison remain retained. This completes
the scoped fixture journey, not live-model or full production acceptance.

The `7ff66d3b` installed standalone PostgreSQL journey passes five groups,
including actual backup bytes, trusted reimport, restoration into a candidate,
rollback preserving both generations, and service restart. SQL fixture witnesses
verify the records independently of success text; they are labelled setup and
observation, not an application-authentication test. All fixture-owned runtime
resources were verified removed. Receipt:
`artifacts/package-eval/0.5.0-7ff66d3b/database-recovery-attempt-3/result.json`.
This baseline predates the new data-permission and recovery-navigation UI.
Full Supabase authentication, row isolation and stored-file recovery need an
adequate host; their prepared capacity/scenario record is
`artifacts/plans/data-operation-host-prerequisites.json`.
