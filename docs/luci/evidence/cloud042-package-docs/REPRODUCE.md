# Reproduce the focused checks

Prerequisites: Python 3 and Git. Run from the source checkout containing this capsule. The fixture runner creates disposable temporary Git repositories, does not fetch, and does not start Luci, a browser or Docker. It does not validate native packaging or live URL availability.

```sh
python3 -m unittest discover -s docs/luci/evidence/cloud042-package-docs/source/after/scripts -p test_stage_release_docs.py -v
bash -n docs/luci/evidence/cloud042-package-docs/source/after/scripts/build-release.sh
```

The final recorded fixture run has 22 passing cases and zero skips. Native symlink privilege can produce a scoped skip on a Windows host; preserve that result rather than reporting equivalent native coverage. The fixtures' official remote URLs and tracking refs are synthetic local setup, not actual fetch evidence.

The separate integrated source-export check was run from the repository `scripts` directory:

```sh
python3 -m unittest test_export_public_source -v
```

Its recorded 20 passes apply to the 30 captured integrated inputs under `integrated-source-export/`; running it on another revision is a new result.

The root also ran `Parser::ParseFile` under retained PowerShell 7.5.5 on Linux. `integrated-source-export/parse-release.ps1` retains the parser-only wrapper with its checkout path replaced by `<source-checkout>`; substitute the actual checkout path to reproduce it. The bound `powershell-parse.json` reports zero errors and unchanged script bytes. This does not execute the release script or establish Windows behavior. The earlier PATH-only discovery limitation is superseded by this scoped parse.

After the candidate is integrated, create a clean commit and publish/fetch through the normal authenticated repository workflow. Do not synthesize an official tracking ref to satisfy the guard. Invoke the documentation staging helper with the exact full commit and a **new outside-checkout output directory**:

```sh
python3 scripts/stage-release-docs.py /absolute/source/checkout /absolute/new/docs-stage --source-commit=<full-clean-commit> --source-dirty=false
```

The angle-bracket commit is a placeholder to replace, not literal shell syntax. Existing output directories are refused. Missing official cached ancestry preserves full docs; unavailable provenance must not be bypassed by forcing a purported association. Do not run this example against a dirty checkout while claiming clean source. Source exports and dirty development builds use the helper's conservative full-documentation path through the release scripts.

The recorded intermediate actual staging used exact 765fcaad and the frozen intermediate helper. It is preserved for audit rather than offered as final acceptance. The final candidate must be staged and built again after integration; the package's source identity, licence inventory, actual archive size and native behavior require their own receipts.
