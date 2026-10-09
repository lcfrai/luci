# Current installed Playwright dependency binding

This capture binds the current installed Playwright packages for prospective use by the settled-DOM control. It does not establish which dependency bytes ran in an earlier diagnostic. Product identity remains edd0; this package capture makes no product-build claim.

`manifest.json` inventories 176 ordinary files, 18,547,881 bytes and 39 directories across `playwright` and `playwright-core`, both version 1.63.0. The complete files are preserved under `snapshot/`, with executable modes and all package licences/notices retained. Resolved package roots, package JSON paths and ESM/CJS entrypoints are in the manifest. Node's built-in resolver inspected package metadata without importing package code.

- Manifest SHA-256: `377518398482124b8f74c7619b52ca2d21c023fc851b377ab9bb2cb3eecec785`
- Canonical content/mode inventory SHA-256: `c49a27bd78518d76caa1ba4d49414cf3034050e074e7be6301b6c341f3caf863`
- Adjacent `dependency-binding.py` SHA-256: `0966ad866420f28747e8a8799a4434bc63dc733b84011780022b4d159ddebf76`

Capture refused inner symlinks, non-ordinary file types and changes during descriptor-relative reads. Two complete installed-source content/stat passes matched. The preserved snapshot was then independently rehashed and matched the inventory. `capture-verification.json` records a subsequent successful read-only verification.

Use a fresh output filename for every check:

```sh
python3 /workspace/.luci-tools/cloud032-settled-evaluate-control-v1/dependency-binding.py verify /workspace/.luci-tools/cloud032-settled-evaluate-control-v1/dependencies/manifest.json FRESH_OUTPUT
```

The checker reads only the two manifest-bound installed package trees, hashes two stable passes, and writes one comparison receipt. Missing, extra, changed-content/mode or changed-type members fail; so do symlinks, special files, unstable reads and root-resolution changes. Both configured and resolved roots are restricted to the declared packages. It makes no package copies or dependency changes during verification, and rejects output paths inside either installed package, including paths reached through a symlinked parent. Exit status is zero only for a matching inventory. Output reuse is refused.

Limits are 10,000 ordinary files, 128 MiB of inventoried file bytes, 10,000 directories and a 16 MiB manifest. The snapshot excludes browser binaries, browser profiles and unrelated dependencies. The checker compares package bytes and modes, not process memory or the bytes a process previously loaded. Its own executed source and the expected manifest digest must be bound by the parent wrapper. No browser, Docker, application, timing control or test was launched for this capture.
