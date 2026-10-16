# LOCAL120 original ZIP name correction

Parent `99755d66b9633cacb7c33b156ed923180c63173b` checked decoded `ZipInfo.filename` but Python sanitizes that property at the first NUL. The Go module ZIP hash contract concerns the original ZIP member name. `module_zip_h1` now requires `orig_filename == filename` before its existing control-character and h1 checks.

The new `test_module_h1_rejects_raw_local_and_central_nul_name` fixture writes one normal ZIP, changes the same name byte to NUL in both its local and central headers, confirms Python exposes different original and sanitized names, and requires refusal before h1 comparison. Exact command:

```
LUCI_PDF_PILOT_TMPDIR=/dev/shm timeout 15s python3 -B -m unittest -v test_source.PilotSourceTests.test_module_h1_rejects_raw_local_and_central_nul_name
```

Result: **1 passed**, 0.002 s; `git diff --check` passed. Earlier six source passes remain historical evidence for their earlier source revision. No Go compile, pilot acquisition or runtime test was run. The five joined Debian API tests and Go PDF boundary test remain unrun under the existing capacity gate.
