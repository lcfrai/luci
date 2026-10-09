# Combined CLOUD-046/047 frontend source validation

**PASSED at this source-validation scope.** The completed run records **2778 passed assertions / 158 configured result files**, 0 failed assertions, frontend exit 0 and TypeScript exit 0. [analysis.json](analysis.json) independently counts raw assertion statuses and configured result files. Vitest's suite totals also include nested suites; they are not the configured file count.

The base HEAD recorded at frontend completion is `e2abd0e39e934e68ead3fd3e906a79336a8bda19`. Product corrections were present as a combined candidate over that base, so this is **not a clean base-commit validation**. Exact before/after/after-typecheck input manifests bind the tested bytes; the publication commit can associate those hashes separately. All three captured repository lists (1185 entries) and dependency lists (19369 entries) compare equal: `{'repository': True, 'dependencies': True}`. The frozen wrapper documents scope and cache exclusions; this is not a claim of every possible compiler input.

[Original command](command.json), [result](result.json), [timeout](timeout.json), [typecheck command](typecheck-command.json), [full JSON reporter](full.json.gz), and all **four original stdout/stderr logs** are included. The frontend stderr retains the known jsdom warning: `Not implemented: HTMLCanvasElement's getContext() method: without installing the canvas npm package`. This warning is neither suppressed nor counted as a failed assertion. Empty logs remain exact zero-byte files. No failed or filtered historical run is relabelled as this combined run.

[original-bindings.json](original-bindings.json) binds exact original bytes to published bytes. Only the large full reporter is newly losslessly compressed; the six input gzip files are copied unchanged and their decoded hashes are included. Standard cloud task paths remain in original logs/wrapper/reporter. No private fixture files or credential stores were read to publish this capsule. [SHA256SUMS](SHA256SUMS) includes logs and every other member except itself.

## Reproduction and limits

Use an isolated checkout of the eventual source revision whose selected hashes match `repository-before.json.gz`, Node as recorded in command.json, and locked `npm ci` in desktop. Installed package bytes must match the dependency manifest before claiming an identical input set. Then run in desktop:

```sh
node node_modules/vitest/vitest.mjs run --maxWorkers=2 --reporter=json --outputFile=/fresh/private/evidence/full.json
npm run typecheck
```

The exact [capture wrapper](run-cloud046047-combined-validation.py) is retrievable. Its fixed `/workspace/luci` checkout and fresh output path are intentional original bytes: run it only in a disposable environment with that layout and an absent output directory; never overwrite the retained originals. It uses a 900-second frontend budget and a 300-second TypeScript budget. A newly reproduced result is separate evidence.

These are source/controlled-fixture checks, **not an installed app, native Windows, live-provider, four-project workload or latency pass**. This capsule does not close the local-product acceptance goal.
