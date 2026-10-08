# Historical task-review copy witness

This directory publishes the exact previously executed source-component browser
witness. Publication adds no new browser pass. Its historical receipt remains
[`docs/luci/evidence/task-review-polish/browser.json`](../../../docs/luci/evidence/task-review-polish/browser.json).

| Preserved file | SHA-256 |
| --- | --- |
| `original/copy-browser.mjs` | `550fa1e3c9e376479f90e2f277285ad65de3ad10129feb675597617b4c484b93` |
| `original/fixture.tsx` | `72e784c08a67117472b7fea6803c279c040c8471ee9f1e06454d8e99ab71e97a` |

The fixture was recovered by JSON-decoding the script's `fixtureSource` literal,
without executing JavaScript. `original-manifest.json` records both byte counts
and hashes. The original script still contains its original `/workspace/luci`
checkout path and virtual `scripts/acceptance/task-review-ui/fixture.tsx` module
location. They have not been silently rewritten. The recovered file is a
readable historical copy; the driver continues using its embedded literal.

The witness renders real source components with controlled API callbacks,
permissions and queue behavior. It checks task-card explanations, reported versus
recorded checks, question/answer presentation, keyboard focus, axe results and
geometry at 1440 and 390 pixel widths. A 390 pixel viewport is a Linux browser
measurement, not a mobile device or Windows/WebView test. It does not execute
the whole Develop shell, a real backend, persistent mutations, Docker, agents,
models or deployment. Its button-close focus check waits for deferred restoration;
it does not resolve the original native Windows immediate-Escape observation.

## Prepare a reproduction at another checkout path

Use Linux, an ordinary non-root user, Node, Python3 and a sandbox-capable Chromium.
Install the checkout's locked desktop dependencies separately; the preparation
script never installs dependencies. It checks for Vite, Playwright and axe, then
creates a fresh directory containing an explicitly derived script and
`adaptation.json`. It starts no service or browser and modifies no original file.

```sh
python3 /ABS/luci/scripts/acceptance/task-review-polish/prepare-portable.py \
  --repo /ABS/luci \
  --output /ABS/private-evidence/task-review-copy-NEW
```

The derived script changes only the checkout path, the source fingerprint's
publication-directory coverage and the receipt's adaptation metadata. The virtual
fixture module retains its original depth so its relative source imports resolve
against the chosen checkout. Browser assertions, controlled fixture bytes and
historical cleanup behavior remain unchanged. The new receipt records the actual
derived helper hash; do not substitute the original hash for it.

After coordinating an exclusive browser acceptance window, execute explicitly:

```sh
LUCI_CHROMIUM_EXECUTABLE=/ABS/chromium \
LUCI_BROWSER_STATE_ROOT=/ABS/private-browser-state \
node /ABS/private-evidence/task-review-copy-NEW/copy-browser.portable.mjs \
  /ABS/private-evidence/task-review-copy-NEW
```

Chromium defaults to `/usr/bin/chromium` when its override is omitted. Browser
state defaults to a fresh child of `~/.cache/luci-ui-acceptance`; use the explicit
private override to keep acceptance separate. Vite binds loopback with an assigned
port, external browser requests are refused, and agent creation is a forbidden
fixture callback. These controls are part of the source-component witness, not a
claim about production backend permissions.

Retain the complete new output directory, `adaptation.json`, `result.json`,
screenshots and any failure output. A passing historical receipt is not a promise
that changed current components pass the old assertions. A failed receipt must
remain separate; choose another fresh directory only for an explicitly requested
rerun. No original script or fixture is overwritten by this preparation.

## Run the exact historical script

Only when the intended checkout is actually `/workspace/luci`, the unchanged
original can be executed directly in a separately authorized browser window:

```sh
LUCI_BROWSER_STATE_ROOT=/ABS/private-browser-state \
node /workspace/luci/scripts/acceptance/task-review-polish/original/copy-browser.mjs \
  /ABS/private-evidence/task-review-copy-original-NEW
```

This preserves the original fingerprint scope as well as its original behavior.
The portable adaptation records the publication directory too. Neither command
is run by publishing these files, and neither supplies native Windows evidence.
