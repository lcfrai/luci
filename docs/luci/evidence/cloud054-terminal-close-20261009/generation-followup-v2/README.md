# CLOUD-054 allocation projection and authorized visibility follow-up

The main frontend stage owns `sessions.go`, `session_execution_generation.go` and `session_execution_generation_test.go` under `frontend/backend/control-plane/internal/api`. Review confirmed the domain-separated HMAC uses the existing canonical 256-bit attach-token bytes, encodes session/allocation identities unambiguously, and exposes only an optional hexadecimal fingerprint. Missing/legacy/cleared tokens omit it. No credential, raw token, private allocation/ref/endpoint, authorization capability or persisted migration is added. A fingerprint is equality evidence, not a monotonically increasing clock.

Main staged projection before/fixed receipts are preserved in `frontend/evidence/before-generation-projection` and `fixed-generation-projection`. Original projection lacks the new fingerprint; original omission/secret-key controls remain. The fixed projection passes with race detection and exact selected source hashes before/after. Ordinary status/output/attach-count/endpoint updates preserve a known generation; session, token and allocation changes do not alias. Terminal cleanup's omission is explicit and must not itself clear the frontend's ended latch.

This follow-up adds two integration files:

- Unchanged `../generation-followup/session_execution_generation_http_test.go` → `control-plane/internal/api/session_execution_generation_http_test.go`.
- Corrected `team_session_execution_generation_test.go` → `control-plane/internal/management/team_session_execution_generation_test.go`.

The actual production HTTP restart test seeds a canonical token, reads the current fingerprint, restarts the same session ID, and verifies the changed fingerprint remains stable through readiness completion and a subsequent GET. No private runtime fields or token are present in that GET. Existing JSON/private-field and restart lifecycle tests also passed under the exact staged API overlay.

The first supplemental invocation passed all API cases and the existing team filter case, but the authored new team marker case failed. Its fixture incorrectly reused the hidden-project marker for an **authorized provider session** in the visible project. The old fixture, raw failure and command/source hashes remain unchanged in `../generation-followup/`. This version gives unrelated fixture sessions a separate marker and reserves the hidden marker for `s-hidden`; no policy/production change occurred. Only the corrected unfinished management case was rerun. It passes under race detection: authorized list/detail retain the visible fingerprint, hidden-project detail is403 without the marker, revoking project read makes detail403 and removes the marker from the next filtered list.

This management fixture injects synthetic fingerprints into a controlled worker response and tests the real gateway's filtering; it is not an end-to-end HMAC generation test. That computation is covered separately by the API projection and HTTP restart tests. These scoped results do not establish native Windows, installed Docker, live-model or full-suite acceptance, and overlapping projection tests are not additive unique coverage.
