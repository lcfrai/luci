# Preview cancellation management admission

The new POST release cancellation route requires current Read, Edit and Deploy authority. Execute and Secrets are not required. Cancellation receives the same suspended-service cleanup exemption as Stop; the exemption does not grant permissions.

`before/` ran the new regression tests against unchanged production sources: exit 1, 6 passing and 5 failing outcomes (two top-level tests and nine subtests). Authorized cancellation was unavailable. `fixed/` ran the exact same selection after the two production route entries were added: exit 0, all 11 outcomes passed. Both runs retained 86 stable selected input hashes; these are management package/module/activation inputs, not the whole transitive compilation closure.

The cases cover current Deploy without Execute, unsupported methods, missing Read/Edit/Deploy, active and suspended service, team and project Deploy revocation using the same login and expected release revision, restored authority, cross-project refusal, and continued refusal of new release creation under suspension. Product exact-operation/revision behavior and registrar source-guard availability are verified separately by the backend owner.

`original/` preserves all four starting files. `before-sources/` preserves the tested original production files plus the new regressions; `fixed-sources/` preserves the tested final files. `management.patch`, exact runner, invocation, stdout/stderr, outcome and before/after selected source bindings are retained. No full-suite or runtime result is claimed.
