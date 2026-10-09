# CLOUD-056: current workspace denial and bounded recovery

A fresh, current-generation session-list `403 team_access_denied` now hides the admitted workspace immediately. Reopening requires current membership plus fresh project/session visibility; restoring membership alone cannot show a previously cached project whose grant was removed. Stale scope results remain fenced. Failed recovery preserves drafts and retained buffers. Repeated denials in one blocked episode do not create repeated membership requests, while explicit Retry, focus and periodic refresh remain available. Hidden mounted readers can still perform independent server-guarded reads.

This is a cached-display correction. The backend already refuses fresh private reads after revocation. Neither a backend authorization bypass nor installed/browser/Windows acceptance is asserted. See `STATUS.json` for exact run scopes, failures, overlaps and remaining limits.

`final-source/` is the exact final three-production-file and eighteen-case regression state captured by the final two-file validation. Initial broader validation passed 383 assertions but TypeScript failed on the new fixture's mock type; those original bytes remain intact. The typed fixture and denial-episode follow-up then passed 46 affected assertions and TypeScript. The small unchanged Go403 registrar control passed in the earlier batch and was not rerun.

All original failures are retained. The first episode fixture's fourth original assertion was confounded by its later successful membership fake; the revised control passes on both original and corrected sources. Three meaningful call-count failures persist before the change and pass afterward. Historical manifests describe their captured moment and are not rewritten to claim later execution.

Reproduce with `FINAL-VALIDATION.md` after applying the initial and final patches, using the repository's pinned activation and existing frozen dependency installation. Do not run tests during a separate installed performance run. Full command/config/source captures accompany every retained batch. Selected original42-file scope and later432-input frontend snapshots are different scopes; neither is a full dependency export.

Publication requires explicitly adding all selected logs despite ignore rules, then verifying the publication manifest/checksum set against Git bytes. The capsule preserves controlled synthetic fixture strings and command paths, and contains no exported real application credentials or private runtime fixtures.
