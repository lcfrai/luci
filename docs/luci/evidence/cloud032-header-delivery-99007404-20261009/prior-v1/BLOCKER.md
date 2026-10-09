# Installed experiment blocked at transport validity gate

The initial tiny transport proof passed 18 checkpoints with the exact original Playwright routing guard plus a separate CDP response-stage session. Its successful-body and post-body-cancellation coverage remains valid and unchanged.

Root then requested one affected pre-body cancellation case. That case failed: after headers were paused and `Fetch.getResponseBody` was pending, natural AbortSignal cancellation produced browser `AbortError` and the matching `Network.loadingFailed(canceled=true, net::ERR_ABORTED)`. The server observed exactly one original request, closed after zero body bytes and never finished the body. The capture promise did not settle within two seconds. It settled only when the exact owned browser was closed.

The failed proof did not call `Fetch.disable`, `continueResponse` or `failRequest` while body capture was pending. It did not replay a request. Cleanup joined the capture after browser closure and retained that browser teardown as the settlement basis; it does not claim a successful response drain. Original 18-check evidence remains byte-for-byte unchanged. The new evidence is one failed affected case, not a repeated or cumulative passing suite.

This is a concrete blocker for the proposed per-round successful-drain intervention. A dangling capture cannot be silently forgotten, and closing the browser between rounds would change the measurement protocol. No installed diagnostic, new runtime fixture, product edit or latency optimization occurred. The response-delivery hypothesis remains unestablished.

`summary-delivery.mjs` is an external, unrun implementation draft started before the additional proof completed. It is not imported by a generated driver, has not been syntax-checked or unit-tested, and is not an accepted experiment implementation. Its timeout path can record an invalid run and require owned browser closure; that does not make the failed validity gate pass. `status.json` binds it explicitly.

The immutable planning/reference manifest and fixed schedule describe the proposed experiment, not authorization to bypass this failure. The next step is root review of the failed gate before selecting any different response-delivery mechanism. No replacement guard, extra fetch, altered body, ignored cancellation or speculative product change is justified by this evidence.
