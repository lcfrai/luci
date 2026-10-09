# Reconstruct the captured dependency bytes

Run these commands from this publication directory only in a chosen preparation window. They are instructions, not a claim that reconstruction was executed for this publication. Python3 and curl are required. No npm install, lifecycle script or browser download is needed.

Choose two new sibling directories. Preserve any failed download or verification output instead of reusing it as proof. The following example refuses existing directory names:

```sh
mkdir npm-archives-new
curl --fail --location --proto '=https' --proto-redir '=https' --output npm-archives-new/playwright-1.63.0.tgz https://registry.npmjs.org/playwright/-/playwright-1.63.0.tgz
curl --fail --location --proto '=https' --proto-redir '=https' --output npm-archives-new/playwright-core-1.63.0.tgz https://registry.npmjs.org/playwright-core/-/playwright-core-1.63.0.tgz
python3 reproduce-dependencies.py --archives npm-archives-new --output reconstructed-dependencies-new
```

The verifier requires both SHA-512 pins in `dependency-pins.json`, then requires exact archive file-list and byte/hash equality against all176 captured ordinary files in `preparation/dependencies/manifest.json`. Unknown paths, duplicates, links, missing files, special members and mismatched bytes fail. Compressed archives are capped at32MiB each; total ordinary content is capped at128MiB. No archive member is directly extracted. The output directory is created only after both archives and all members validate; existing output is refused. Captured600/700 file modes are applied explicitly, rather than assuming npm archive modes match the original installation. Directory metadata, inodes and historical timestamps are not reconstructed.

A successful receipt establishes reconstruction of the captured library file bytes from those npm archives. It does not reconstruct the Chromium binary, all unrelated node_modules, local process state, historical environment load or measured timings. The separate installed dependency checker in the accepted helper has original absolute-path and directory bindings; do not weaken or rewrite its frozen historical receipt to claim it ran against this new directory.

The analysis exports are reproducible from the retained original result using `analysis/analyze.py` in its declared environment. That script's result SHA guard refuses different input bytes. `analysis/closure.py` records a one-time live observation; a later execution would be a new observation and must not overwrite the published receipt. Historical preparation commands are retained for review, with their expected failures and corrected outcomes distinct.
