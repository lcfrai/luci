# Exact-source C054 package and runtime preparation

Source/helper preparation only, except the separately recorded root-run package attempt. The package helper is now frozen: its successful execution and previous-release preservation belong to the candidate-5f4a966f package receipts, not these inert controls. No runtime build/export, image retirement, cache operation or tool relocation was performed by this preparation task.

The new thin image must use clean retrievable source `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`, exact engine `ff40514f-d7cc-4ee7-8a1a-1cfafaeaf259`, and an explicit exclusive root-authorized window. Both Go module/vendor pins are1.8.15. The helper computes and verifies the complete current tracked runtime input set against this commit; the historical104-source count is not reused.

Root-only invocation after space/exclusivity checks (not executed by the preparer):

```sh
python3 -B /workspace/.luci-tools/cloud054-runtime-helper/prepare-runtime.py --expected-source 5f4a966f6d70264c92ee2176dde912e1b43fc3c7 --expected-engine ff40514f-d7cc-4ee7-8a1a-1cfafaeaf259 --window-granted build
```

The additive build requires the old333eff image and its exact tags/archive to remain present. Only after successful new candidate identity/config/source evidence may the separately reviewed operation retire the exact old tags. The trusted9e0 base remains protected. Export is separately budgeted using the actual new candidate size; run the same helper/pins with `verify-export /ABS/ACTUAL/receipt.json`. It streams and checks all image layer diff IDs and the exact binary/entrypoint payload; no successful export or fresh archive restore is claimed here. All working reserves and the installed6GiB floor remain distinct unchanged gates. Docker CLI calls use the pinned local Unix endpoint and fresh private HOME/config, not inherited contexts/TLS.

`build-package.py` requires the prior generated release to identify exactedd0, journals rename intent, preserves its artifact bytes and source provenance, then invokes the repository release build and existing verifier. The first root attempt refused an unfetched remote-tracking source before mutation; `package-attempt1-preflight-refusal.json` retains it. A later fetch does not change that original failure.

Nine tiny Python controls pass using fake subprocesses and disposable temp directories. The original draft had one module-syntax error (valid require/indirect form), preserved under `draft-before-module-syntax/`; the corrected nine and final nine overlap, not18 independent tests. These controls validate guards, not a Docker build, archive export or runtime behavior. Final source read-only review is separate.
