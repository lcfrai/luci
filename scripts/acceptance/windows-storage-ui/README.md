# Windows storage settings: source UI witness

This Linux Chromium helper renders Luci’s actual `DockerStorageConfirmation`,
shared controls and application CSS at 1440×1080 and 390×844. Authentication and
storage observations are explicitly controlled fixtures. It checks separate
host/guest values, an entered full Windows path, explicit review before the
fixture confirmation callback, keyboard focus/tab order, wrapped review content,
read-only polling after confirmation and automated axe findings.

**It does not run the management backend, Windows APIs, Docker, agent workers,
models or cloud operations.** It does not certify an installed package or native
Windows storage behavior. The actual management HTTP tests and frontend client
share `control-plane/internal/management/testdata/execution-storage-http.json`;
that separate contract evidence does not turn this callback fixture into an HTTP
test.

Use an unprivileged Linux account, Node, repository desktop dependencies installed
from the lockfile, and Chromium with its browser sandbox enabled. Run from the
repository root:

```sh
node scripts/acceptance/windows-storage-ui/source-browser.mjs \
  --report /path/to/fresh-evidence-directory/result.json \
  --chromium /usr/bin/chromium \
  --fixture-root /path/to/writable-private-temporary-parent
```

The output directory must be fresh. The helper starts its own loopback Vite
server on an available port, blocks browser requests outside that origin, creates
an isolated browser home, and closes its browser/server and removes that home.
It never connects to a configured Luci workspace. It records source hashes,
helper/fixture hashes, exact controlled calls, geometry, axe results and six
screenshots. It copies the executed helper and fixture into the evidence bundle.
Keep unsuccessful evidence directories; use a new directory after correcting a
fixture or product defect. `--help` requires no installed browser.

A screenshot is supplementary. Checked DOM outcomes establish the interaction;
actual native Windows probes, user authority, disk identity/settings invalidation
and admission checks remain separate acceptance gates. Do not use this helper’s
fixture numbers as a physical-capacity or runtime-performance measurement.
