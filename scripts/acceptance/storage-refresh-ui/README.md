# Storage refresh: controlled Settings browser witness

This Linux Chromium helper renders actual `SetupPanel`,
`DockerStorageConfirmation` and `RuntimePreparation` source and CSS. It supplies
controlled identity and HTTP-shaped callbacks; the unrelated dependency installer
panel is omitted. It makes no real management request, starts no native tool,
Docker worker or model, and does not measure physical storage.

Use an unprivileged account, Node, repository desktop lockfile dependencies and
sandboxed Chromium. From the repository root:

```sh
node scripts/acceptance/storage-refresh-ui/source-browser.mjs \
  --report /path/to/fresh-evidence-directory/result.json \
  --chromium /usr/bin/chromium \
  --fixture-root /path/to/writable-private-temporary-parent
```

The helper captures 1440×1080 and 390×844 Settings views. It holds a cached storage
read, checks that only its local refresh control is busy, and uses the actual
Appearance and Back controls while the request remains pending. It completes
the old reply after leaving storage and checks that it cannot update the new
view. It also confirms a synthetic location, observes the saved pending state,
and proves that a later storage measurement does not manufacture whole-workspace
readiness. Only clicking **Check readiness** invokes that separate fixture
endpoint. In onboarding, storage refresh must not repeat the initial preparation
check or disable **Continue to projects** / **Choose another setup option**.

Receipts distinguish all callback paths and record source/helper/fixture hashes,
geometry, axe results and screenshots. Source fingerprints cover frontend files
and dependencies but exclude the unrelated native Rust source. The helper blocks
browser requests outside its private loopback origin, closes its owned Vite and
browser processes, and removes its private browser home. Use a fresh output
directory each time and retain failures.

This establishes source UI behavior and busy-state isolation. It is not an
installed Windows witness, a native process-start trace, or proof that real
Git/Docker/AWS/Azure tools did not start on another machine. Native acceptance
should separately verify the packaged fix and preserve its process trace if
available.
