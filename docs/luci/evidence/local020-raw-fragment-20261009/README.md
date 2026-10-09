# LOCAL-020: reject raw fragments in guarded check requests

Base revision: `2da1a821b8456c20a858fb86ef87649ea917d9de`. This fixes an acceptance-proxy input guard. It is not a Docker SDK defect or evidence of a previously observed backend failure. The actual SDK emits no fragment in the generated stats request. Earlier installed receipts and matching runtime/source bindings remain unchanged; this capsule does not retrospectively validate them against new helper bytes.

A raw known-ID request such as `/v1.51/containers/<ID>/stats?one-shot=1&stream=0#extra` was accepted because `new URL(...).searchParams` discards the fragment, while the upstream forwarding path preserves the original `req.url`. The local tester reported this with controlled VM inputs. This capsule independently reproduces the behavior using the actual proxy and a controlled Unix HTTP upstream: the original guard forwards it and returns 200 instead of the expected refusal. No Docker daemon or application is involved.

The existing `queryShape` guard now rejects any literal `#` before URL parsing. Testing the raw string also catches a trailing `#`, whose parsed `URL.hash` may be empty. Valid requests keep their original raw URL unchanged; no normalization is introduced. Percent-encoded fragment characters remain subject to existing decoded-value validation. This guard applies only where the existing check request policy calls it. Generic GET/HEAD observation paths retain their prior behavior.

## Results

- Same final regression against the exact original proxy: **one expected failure, one generic-read control pass**. The malformed known check stats request returned 200. The final corrected comparison passes.
- **50 affected proxy/transport tests passed**, zero failures/skips/cancellations, plus two syntax checks. This is the affected proxy suite, not a full helper/product suite.
- Six stats negatives cover both versioned/unversioned routes and literal nonempty, literal empty and encoded fragment forms. Four additional known-check JSON/logs/start/delete negatives demonstrate shared guard behavior with no forwarding, no Start and no rejected-query metadata disclosure.
- Twelve generic worker/fence/unknown GET/HEAD controls across both route forms preserve the exact fragment-containing upstream URI, with no new check-query metadata. Existing valid SDK queries, body/identity/stream restrictions, response/deadline/concurrency bounds and recovery controls also pass.
- Four selected current repository inputs match before/after validation, including the unchanged recovery scenario/final failure assertion; four explicitly pinned Git proxy/support inputs are frozen. This is selected controlled-test provenance, not full compiled product or installed evidence.

`commands.json` retains exact commands/times/exits and raw-log hashes. `tested-sources/` freezes final selected inputs; `retained-inputs/` preserves the original proxy/test plus initial before/after logs. `original-bound-regression` uses the final regression file with only the proxy replaced by its original 2da bytes. Earlier initial before/after runs overlap the final evidence; do not add their counts. No failed evidence was rewritten.

Reproduce from the exact candidate checkout with supported Node 24:

```sh
node --test --test-timeout=30000 --test-reporter=spec scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs
```

To reproduce the original failure, copy the frozen `tested-sources/scripts/` tree into a fresh disposable directory, restore `retained-inputs/original-fenced-docker-proxy.mjs` to `scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs`, and run the recorded `original-bound-regression` command there. Expected exit is 1. Do not overwrite a user's checkout. The existing proxy tests read four historical Git blobs from `/workspace/luci`; any test-only locator adjustment for another machine must be recorded. The exact capture program is retained and requires a fresh output directory.

No browser, application, provider, Docker operation or preserved execution fixture was started or changed. A subsequent runtime must use the newly committed exact helper binding. Source/test correction does not establish Windows, live-model or sustained performance acceptance.
