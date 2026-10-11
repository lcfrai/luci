These controlled Node tests are prepared, not executed. Do not run them until root grants the C077 test window after C076.

`monitor-regression.test.mjs` is the same test body for both exact helper copies. It imports their exported observer and private-error functions. Its fake page runs the actual serialized native observer in a fresh VM context; fake CDP and Playwright sockets deliver controlled observations to the actual exported listeners. No browser, product application, Docker process, provider, compiler, or real network is used. The stimuli are controlled helper evidence, not an installed runtime replay or timing result.

After authorization, from this directory:

```sh
C077_VARIANT=before node --test --test-concurrency=1 monitor-regression.test.mjs
C077_VARIANT=fixed node --test --test-concurrency=1 monitor-regression.test.mjs fixed-guards.test.mjs
```

Preserve both commands' real exit codes and full outputs. The original copy is expected to fail the counterexamples. The fixed copy must pass them and the fixed-only guards; no results are claimed before execution. Private-error tests create and remove only their own private temporary directory.

The main shared counterexample contains one successful stream with typed Exit 0, one explicit Restart transition, ten refused handshakes, and a replacement successful stream with typed Exit 7. Independent shared cases exercise native, CDP, ordinal-nine, and more-than-32-error retention paths. Fixed guards cover 34 attempts per ladder, 68 total, excess successful streams, the closed interval after typed Exit, invalid Restart transitions, unopened-socket payload rejection, eight errors per channel/socket, UTF-8 text limits, and retained encoded bytes.

The 1088-row test verifies the aggregate byte bound and individual encoded row lengths. The file-byte limit is an additional bound behind the row-count and encoded-row bounds; compliant input cannot exceed their product, so these tests do not claim an artificial file-overflow branch was reached. The original 8-second held-read limit and product 10-second read timeout are not changed or exercised here.
