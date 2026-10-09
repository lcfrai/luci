# CLOUD-056 source review and staged reproduction

Pinned product: `edd0cb121ff6847f681ab6bf41509a497f043d41`.

**Prepared only; neither test has been executed.** Root owns the quiet installed acceptance window, invocation and repository mutations. No app, browser, Docker, provider, test runner or build was launched during this review. This directory is external staging, not a published capsule or a verification pass.

## Concrete effect and boundaries

A removed team member's next fresh `GET /v1/teams/{id}/workspace/v1/sessions` receives403 `team_access_denied`; their account authentication may remain valid. The client preserves the exact problem code but only401 expires authentication. Poller catch stores `lastError` and does not update sessions or invoke the catalogue retry hook. The provider does not subscribe to these denials: it refreshes membership independently on window focus/every60seconds. Until that successful refresh proves absence, `blocked` stays false and the admitted app remains rendered. Cached session titles, statuses and mounted terminal scrollback are not hidden by `DevelopLens`/`SessionPane` merely because `lastError` now records this denial.

The existing higher layer **does** hide admitted app children and disposes terminals after a fresh membership refresh returns the team absent. Initial nonmembers never mount children.401 triggers auth expiry, terminal disposal and cockpit reset. Workspace/account selection changes invalidate old responses. A project-specific grant removal normally leaves team membership valid and gives a **filtered200** session list, removing invisible rows; it is not the persistent membership403 case.

The backend closes already registered sockets/requests when grants or membership are durably changed and independently refuses fresh private reads/attach. This finding is cached UI visibility after a newly received authoritative refusal. It is **not** evidence of new private data delivery, secret access, a server authorization bypass, cross-project leakage, or a permanent cache leak. The prior document's generic403 observation alone did not establish this chain. The worst-case interval is governed by successful focus/60second membership refresh; failed refreshes can extend it, so do not promise a maximum60seconds without that condition.

## Staged controls

`authority-window.test.tsx` uses the actual TeamWorkspaceProvider, useProjectAccess, cockpit client, poller, SessionPane, Terminal and attach registry; auth profile, membership/project responses, fetch, WebSocket and xterm rendering are controlled fixtures. It does not mount the complete AppShell: its actual LensRouter/DevelopLens source contains no additional read-visibility barrier, recorded separately. The fake terminal's buffer is projected as DOM text to check retention/hidden ancestry, not xterm painting or browser accessibility.

Cases: fresh membership403 desired hide (expected original failure); separate fresh membership refresh hide/disposal; filtered200 removal; forbidden-origin and forbidden-host retention; transport failure/recovery; late success and late denial after workspace switch; the same after account-provider scope switch. The original desired-contract failure should be retained with successful controls, not interpreted as an entire test suite pass. Preparation fixes corrected unavailable matcher assumptions before any execution; no failed product/runtime evidence was overwritten.

`team_session_denial_contract_test.go` is a separate existing-production-registrar fixture control for exact fresh denial code, no session fields, valid account401 distinction and no worker mutation. Root may run it with a Go overlay, preserving repository bytes, then retain source bindings/logs. It does not need a real provider or Docker.

## Smallest correction design (not implemented)

1. Add a narrowly typed current-workspace authority invalidation signal carrying the selected team and request generation. Emit it only after both normal client scope fences pass for a confirmed **workspace membership** denial (`team_access_denied`) on the session catalogue. Do not treat every403, network error, forbidden-host/origin, project-specific denial or arbitrary problem message as lost membership. A team ID/fingerprint is scope evidence, not new authority.
2. In the matching provider scope, immediately hide admitted private children behind a recoverable workspace-access state and revalidate membership. Keep the selected workspace and existing drafts/context; do not switch silently to personal authority, delete durable work, erase user drafts, or reset terminal evidence indiscriminately. Pause stream input/reconnect while unverified, using a scoped reversible guard if needed; existing backend revocation remains the actual enforcement.
3. Only the latest scope/request successful membership result may unblock; absent membership stays blocked. A failed revalidation remains visibly blocked with Retry, rather than showing old private content again merely because a network error occurred. Existing provider request-generation checks must also invalidate an earlier pending successful membership refresh as soon as the denial event arrives.
4. Do not equate `team_access_changed` with permanent membership loss: that code can describe an in-flight request canceled by a project-grant revision while membership remains. If it prompts revalidation, preserve its scope distinction and await appropriate fresh authority, rather than destroying all workspace state. Verify restored membership and retained editors recover without stale socket actions or duplicate mutation.
5. Add deterministic controls for stale old-workspace/old-actor denial; out-of-order membership success; repeated denial coalescing; transient failure; recovery with retained drafts; fresh filtered200;401; and unchanged C054 allocation-generation/poller overwrite protections. Existing source guards are not a substitute for running these controls after the quiet window.

The policy above addresses visibly stale authority without making previously delivered bytes cryptographically erasable. Stored browser drafts and deliberate retained context have separate documented lifetimes; avoid unsupported whole-browser erasure promises.

## Deferred command

After root explicitly ends the installed measurement quiet window, an external `node_modules` symlink to the existing desktop dependencies permits test resolution (no install). Run from `desktop` with the supported task activation:

```
node node_modules/vitest/vitest.mjs run --config /workspace/.luci-tools/session-authority-review-edd0/vitest.config.ts --maxWorkers=1 --reporter=json --outputFile=/workspace/.luci-tools/session-authority-review-edd0/original-result.json
```

Capture stdout/stderr, exit status, exact sources before/after and tool versions. Current prepared source is unexecuted; resolve harness faults separately and retain them. Do not run this during an installed latency witness. All configuration is external and no app ports are opened.
