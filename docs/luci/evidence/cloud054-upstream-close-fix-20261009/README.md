# CLOUD-054: duplicate WebSocket close frame, upstream correction

Product terminal source for discovery: edd0cb121ff6847f681ab6bf41509a497f043d41. No product source/module/vendor changes were made by this agent; root owns integration. Tests below use external overlays/workspace resolution. This capsule is ready for review; matching installed Chromium/Windows acceptance remains open.

## Actual diagnostic and reproduced mechanism

Installed v8 attempt7 retained two private26-byte protocol errors with SHA256c0d5bdb43e2d05cd4b10a62baba283c23fc955eafb8a5aff5dcacfaa589dcb1a, safely classified duplicate-close-received. Original private text/artifacts stay in the separately owned attempt7 receipt; no arbitrary raw transport error or credentials are copied here. Relay logging of a source1000 error did not prove downstream close completion.

coder/websocket1.8.12's handleControl unconditionally calls writeClose upon receiving peer Close. When this endpoint has already sent Close, acknowledging the peer's acknowledgement sends a second Close. Chromium rejects that sequence. Earlier Go-aware fixtures stopped at typed Exit/first close; the raw fixture also initially stopped after writing its acknowledgement. Those passing receipts remain true at their narrower scope and are not retroactively a full close-handshake pass.

The extended raw fixture now reads after peer acknowledgement. Before1.8.12 it receives first bytes88 02 03 e8 and then a second opcode8/payload03e8. The same unchanged final regression against1.8.15 receives one close then EOF. It exercises a real local PTY through production backend/API and authenticated owner gateway,205 independently masked one-byte stdin frames plus ready/body barriers. Direct API and owner-gateway branches both demonstrate failure/correction. This is controlled Go/raw TCP evidence; no Docker/Chromium or packaged binary was run by this agent.

## Upstream dependency correction

Use published github.com/coder/websocket v1.8.15 for both control-plane and sandboxd, updating module sums and sandbox vendor through normal tooling. No local fork or ad-hoc lock is needed. The release tracks sent/received close state, suppresses another Close and forbids further writes after Close. Its go.mod requires Go1.23, within Luci's existing supported runtime.

Verified fresh download used explicit HTTPS Go proxy and sum.golang.org with checksum/private exclusions empty. Module checksum h1:6B2JPeOGlpff2Uz6vOEH1Vzpi0iUz20A+lPVhPHtNUA=; go.mod h1:NX3SzP+inril6yawo5CQXx8+fk145lPDC6pumgx0mVg=. Origin tag commit9c8faadccd1b679e811a79ce506f8a10237251ad. All files in the actual test module cache matched that fresh verified download. Keep upstream licence attribution; key original/final source copies are included for the mechanism, not as a maintained fork. Earlier1.8.13 also contains sent/received tracking, but the evaluated correction is1.8.15.

## Evidence and counts

- Initial discovery regression:1top+2sub fail, recording unexpected0x88 after ACK. Preserved separately as before-test.jsonl plus first-regression-test.go.txt.
- Strengthened exact-byte regression: same final test bytes before/after;1top+2sub fail on1.8.12, then1top+2sub pass on1.8.15. Original failure now names full second opcode8/payload03e8. Do not add the initial and strengthened failures as distinct bugs.
- Affected production API/proxy batch:13top+6sub pass, covering normal frame drain, hidden upstream reason, overload1013, parent cancellation, dropped socket, authority-before-upgrade and dial failure.
- Selected upstream write/error/CloseNow/MidRead cases:1top+5sub pass. Concurrent close-ping and close during read/write:2top+4sub pass. These31 affected outcomes are not a full suite; no filtered tests are counted as skipped/passed.
- Portable tracked CI candidate: exact bytes2cf7e44be58a0b95ae3239059ded4c069a0c9dbca35f73f4707107c74d61e05f. Old1.8.12 fails all3subcases; new1.8.15 passes1top+3sub (Exit0, Exit7, overload). It uses framed upstream and production RelayHooked, with no production bridge or PTY dependency. It asserts the complete close/ACK/EOF boundary and reason privacy. Root may copy candidate/control-plane/internal/proxy/close_ack_test.go unchanged.

All runs use race detection, -p1/GOMAXPROCS2, count1 and bounded timeouts. Raw command/JSONL/stderr/Go exit receipts are retained. No compilation/method fixture failure occurred in these runs. Original earlier reader-schema investigation was a read-only extraction error, separately disclosed in the lifecycle capsule, not a product/test failure.

## Provenance boundaries

The strengthened real-PTY run captures3648 old/3649 fixed go-list -race -deps -test declared source/embed and explicit module/workspace/overlay inputs; all captured inputs stay unchanged within each run. These are source inputs, not every generated compiler artifact/toolchain binary. Complete manifests include external standard library/module paths so consumers must distinguish fetched sources from hash assertions. The affected batch carries the fixed manifest plus proxy package files, and adds full verified module-file parity. Its unchanged-input receipts are explicit.

The portable CI before/after test runs while root owns module integration, so it does not assume checkout version labels. Each private workspace explicitly selects1.8.12 or1.8.15; captured go-list module resolution proves the selected replacement. Original edd0 module/sum/proxy Git bytes are frozen separately. The CI candidate binds selected proxy/module/overlay inputs rather than claiming a whole compiled manifest. Selected inputs and exact candidate bytes are unchanged during both runs. These differ deliberately from the first real-PTY test's broader provenance scope.

Future integrated verification must bind final repository pins/vendor, package/controller bytes, rebuilt sandbox runtime image, helper revision and actual browser close outcome. No unchanged broad rerun is requested merely for publication. Preserve attempts1–7, their original failure/cleanup/zombie/charge distinctions, and all remaining release gates.
