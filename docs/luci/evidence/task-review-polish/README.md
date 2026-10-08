# Task review copy and integrated frontend verification

The task card now shows one process-exit explanation while preserving the server's
original warning. Reports distinguish agent claims from recorded checks, and an
answered question displays its original blocking assessment as history without
claiming that the underlying work is complete.

The UI-only changes were tested on the recorded source manifest over base
`c4400f11736e923617a93e8e86803f15d7187bef`. Backend and sandbox code did not change.

- The full frontend run passed **2,610 tests in 153 files**, with no failures or
  skips. Its 416 selected input hashes stayed unchanged throughout the run.
- The earlier focused run passed 63 tests in two affected files and TypeScript.
  Its broader 433-input manifest still matches after the full run.
- Ten controlled actual-source Chromium checks passed at 1440 and 390 pixels,
  including keyboard focus, no horizontal overflow and zero axe violations.
  These use fixture responses and do not establish installed/native behavior.

[Verification and hashes](verification.json), [full test report](full-frontend.json),
[focused checks](focused.json) and [browser observations](browser.json) are
retrievable here. Screenshots preserve their actual scroll positions. The original
LOCAL-008 Windows focus-timing question remains open; the Linux observations do
not explain or retroactively pass that result. Sustained performance and live-model
quality remain separate gates.
