# Integrated desktop typecheck — 11 October 2026

The full desktop TypeScript check passed once: activated Node **v24.19.0**, `node desktop/node_modules/typescript/bin/tsc --noEmit`, working directory `/workspace/luci/desktop`, exit **0**, elapsed **18.02 seconds**, 180-second execution bound. Stdout and stderr were empty. No installs or Vitest rerun were performed.

The actual integrated dirty working tree included CLOUD-076, CLOUD-078, root-applied CLOUD-079 hydration synchronization and SX2 seam fixture corrections. This is not a clean-HEAD claim. `run/source-before.json` and `run/source-after.json` bind all 1,229 selected effective inputs: every desktop src/e2e/scripts and root source/config/lock file, actual external seams Go/docs/task-plan/Dockerfile globs, and literal raw imports. They were identical. The Node executable, installed lock and selected runner/compiler/package entries were also identical before and after. The dependency binding is not an independent census of all transitive installed files.

`binding-and-execution.py` uses exact function source extracted from the established integrated frontend runner without executing its top-level full-suite code. `helper-provenance.json` records that source and hash. `run-typecheck.py` records and invokes only the full typecheck. The run preserves the real underlying exit, command, timestamps and bounded process cleanup. Source binding refuses unexpected environment files; recorded environment information includes variable names and explicit CI/NO_COLOR values only.

Invoke the original runner with `source /workspace/.luci-tools/activate.sh && PYTHONDONTWRITEBYTECODE=1 python3 /workspace/.luci-tools/desktop-typecheck-c076-c078-c079-sx2-20261011/run-typecheck.py`. The existing run directory is exclusive, so the original receipt cannot be overwritten accidentally.

This validation is separate from the compact CLOUD-079 capsule and all Vitest results. The original interrupted full frontend run remains 2938 passed / 2 failed; corrected targeted runs do not constitute a new full-suite result. Product and browser readiness gates remain separate. Root owns the source checkpoint following this pass.

`ARTIFACTS.txt` declares the complete included artifact list. `manifest.json` binds payload hashes; `SHA256SUMS` binds every declared file except itself. No temporary caches, private application data, environment values, credential files, or undeclared output copies are included.
