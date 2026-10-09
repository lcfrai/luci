# C032 pre-body cancellation: failed affected proof

One new controlled pre-body cancellation case ran, and failed. The earlier 18-check transport proof was not repeated, edited, or counted cumulatively. Its publication manifest and all eight payload hashes were verified unchanged before and after this attempt.

The same frozen original Playwright route guard remained active alongside a separate CDP Fetch Response-stage session. A tiny loopback server flushed successful response headers announcing an 82-byte body, then deliberately withheld the body. Fetch.getResponseBody started and remained pending. An explicit checkpoint recorded the browser fetch unsettled, its AbortSignal still clear, and zero server body bytes written.

The page then invoked its own AbortController.abort(). The browser returned AbortError, and Network.loadingFailed reported the exact paused network request ID with canceled=true and net::ERR_ABORTED. The server closed the original response with zero body bytes written and responseFinished=false. The target reached the server exactly once and the unchanged original route guard exactly once. No replay occurred.

Despite that correlated natural cancellation, Fetch.getResponseBody did not settle within the declared 2,000 ms join bound. The proof therefore failed. Cleanup closed only the owned Chromium context without issuing Fetch.disable, Fetch.continueResponse, or Fetch.failRequest while capture was pending. The capture promise rejected after browser closure with Target page, context or browser has been closed, and was then joined. This is browser-teardown resolution after a failed proof, not a successful response drain and not an accepted InvalidInterceptionId result.

The owned loopback server closed, the owned profile was removed, and no owned browser processes or active paused IDs remained after teardown. The original frozen source, current runner, and prior proof evidence hashes remained unchanged. No retry occurred. The main installed diagnostic remains unrun.

COMMANDS.json records the sole exact runtime invocation and observed exit 1. attempt-1.log contains original merged stdout and stderr, not reconstructed separate streams. The original result and raw event log are retained, with frozen route-guard and runner copies. observations.json indexes the source result and reports the affected server/guard/cancellation/cleanup facts without changing the failed result.

This tiny Chromium/loopback result disproves bounded spontaneous settlement of an in-flight getResponseBody command for this tested cancellation ordering. It does not establish server/backend work preservation or exercise byte/count overflow branches. A production diagnostic must treat this path as failure and must not disable or modify Fetch under an outstanding body-capture promise while claiming successful cleanup.
