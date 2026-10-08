# Reported native Windows own-sidecar lifecycle at 2dcf245d

The local tester reports two actual release-app cycles at exact
`2dcf245daca98d93e9cf7a40399b8fc78f4608a2`. `runtime_info` selected the explicit
isolated E: state with no startup error. Native controls created the owner and
four projects with distinct revision-1 definitions; all four context switches
worked. Controlled reopen passed returning-owner sign-in, persistence of all
four definitions and switching after the navigation tree stabilized. Exact-window
Alt+F4 exited the application and its owned sidecar on both cycles.

All 1,808 tracked-file hashes and HEAD remained unchanged. The candidate and
original checkouts stayed clean; final owned processes, ports and containers
were zero. The four original user Docker containers remained unchanged. No
provider inference or agent worker ran. This supplements the separate
[native package report](2026-10-08-native-package-2dcf245d-reported.json).

**LOCAL-004's full ordinary-state preservation gate remains open.** Contents,
paths, lengths and attributes were unchanged on both cycles, and the legacy
root remained absent. The first cycle observed a 527.7-microsecond last-write
difference for one ordinary-state instance directory; both timestamps were on
2 October, not the current test date. The cause is unconfirmed, and no product
overwrite is established. The second cycle's metadata comparison was exact.
The local tester will refresh handle-based metadata before assigning a cause;
the second pass does not erase the first observation.

**LOCAL-007 is an unconfirmed P2 navigation observation.** A rapid click during
tree expansion selected delta rather than the intended gamma. Later stable-tree
switching passed. No cross-project write or leak was observed. Reproduction and
source investigation remain pending. Windows shortcut glyphs are a separate
minor UI correction assigned to the UI owner.

This is a sanitized relay of the local thread's reports. The cloud session has
not inspected the native raw files or their hashes; they remain under the local
`2dcf245d/native-lifecycle/` evidence directory. The earlier readiness-race,
GET-body and decorative-label harness failures are retained. The
[structured receipt](2026-10-08-native-lifecycle-2dcf245d-reported.json) separates
reported facts and unresolved checks.

The unsigned installer was not executed. Native LOCAL-005 refresh verification,
CLOUD-009 delayed-request/lost-ack recovery, sustained multi-project resources
and real-provider result quality remain separate gates. This is not a completed
local-product or cloud-release claim.
