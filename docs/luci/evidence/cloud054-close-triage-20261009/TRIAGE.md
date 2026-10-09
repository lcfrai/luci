# CLOUD-054 installed terminal close follow-up

Product: `edd0cb121ff6847f681ab6bf41509a497f043d41`. Selected terminal source remains byte-identical at the recorded later documentation HEAD. No repository source changes were made. No product correction is proposed without a demonstrated cause.

## Retained installed observation

Attempt5 delivers its final stdout marker, then the exact 13-byte typed Exit body `exit status 0` (SHA256 `7146fc872b9a9dba5098f80e211bc7a8af3d9b2fbf8a3e5d9447db130fcaa3b2`), then browser error and close1006/unclean. The UI correctly shows ended without an overlay before the held running-status response expires. This does not identify which hop caused the abnormal close. Supported Stop subsequently releases its exact charge. The original failed attempt is retained unchanged.

Original launcher sets LUCI_LOG_LEVEL=error and discards stdout/stderr. The existing API emits `attach relay ended` at Info. Its original relay error is therefore unavailable and must not be reconstructed. The passive browser monitor only reports an opaque Error event. Frontend negative audit SHA256 `b321562807ed89d34683344be90fb2843e2599aad0425cdd1cf94048d54e856c` finds no recorded generation/access/overload/disposal transition and no normal typed-exit close call, but native close-call tracing was absent.

## Controlled checks executed

All three new tests are external Go overlays, GOMAXPROCS2, -race, -p=1, count1, test deadline90s and outer100s. Raw Go JSONL, stderr and Go exit status are retained. Three top-level tests and six subtests pass, no failures/skips. Counts overlap and are not a full suite.

- `sandbox/`: actual local PTY /bin/sh, stdout, typed Exit0 and Exit7, then normal close1000.
- `gateway/`: framed fake upstream through real backend.Dial and API; authenticated owner-gateway branch uses production management socket-ticket and owned connection. Both branches close1000; no allocation destruction.
- `combined/`: actual sandboxd.Server with local /bin/bash PTY, real backend.Dial/API relay and authenticated owner gateway. Both branches preserve final stdout, Exit0 then close1000; no allocation destruction. An external module-root fixture bridge exposes the unchanged internal server solely to the controlled test.

All clients use coder/websocket. These tests do not exercise Chromium, Docker, the packaged controller, sandboxd main/its exit-linger lifecycle, actual per-keystroke browser input timing, or an installed correction. There is no reproduced Go source defect from this batch. Do not infer a Chrome bug from that negative result.

`selected-source-bindings.json` is an honest POST-RUN selection of 23 relevant source/dependency paths compared against immutable edd0. It is not a complete compiled-input or pre/post capture. `controlled-results.json` retains raw-result counts/timestamps and a reproducible invocation reconstructed from known invocation settings, not an original shell transcript. Root concurrently owns other documentation/publication changes.

## Transport observations from source

Both sandboxd/API Accept sites and backend Dial leave CompressionMode unset; coder/websocket v1.8.12 defaults to disabled. No installed negotiated-extension header was retained. Compression remains an observational check, not a supported explanation. API owner traffic uses InitialHandler directly rather than the non-owner reverse proxy; the controlled owner gateway covers its ownedSocket wrapper.

See NEXT-INSTALLED-DIAGNOSTIC.md for the smallest fresh-run diagnostic. No further unmodified source suite is requested. CLOUD-054 remains open; diagnostic/helper work stays under CLOUD-059.
