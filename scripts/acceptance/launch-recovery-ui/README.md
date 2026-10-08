# Launch recovery: controlled source UI witness

Run this helper with an unprivileged Linux account, repository desktop
lockfile dependencies and a sandboxed Chromium installation:

```sh
node scripts/acceptance/launch-recovery-ui/source-browser.mjs \
  --report /path/to/fresh-evidence-directory/result.json \
  --chromium /usr/bin/chromium \
  --fixture-root /path/to/writable-private-temporary-parent
```

It renders actual `ProjectSwarm`, the launch-recovery dialog and shared controls
at 1440×1080 and 390×844. The fixture supplies controlled recovery availability,
mutation results and a project switch; it omits the unrelated queue panel. It
uses no real Luci backend, Docker daemon, task worker, provider or cloud service.
Do not interpret its recorded `POST stop` callback as a real HTTP request or a
stopped worker.

The checks cover modal focus trapping and restoration, explicit unchecked
consent, the exact selected project/run/revision/operation/child in the fixture
mutation, one mutation after consent, a legacy refusal, one readback without
mutation replay after an uncertain response, and a project switch while an old
availability response remains pending. Geometry and axe checks supplement the
captured desktop/phone review screenshots. Full frontend source hashes and
executed helper/fixture hashes are recorded; the output directory must be fresh.
The helper closes its own Vite/browser processes and removes its private browser
home. Browser requests outside its loopback fixture origin are rejected.

The actual application fetches support only after opening the selected attempt:
`GET /v1/product/projects/{project}/runs/{run}/swarm/operations/{operation}/recovery`.
The explicit mutation uses the existing abandon route with
`stopUncertainLaunch: true`, distinct from `recoverMissingChild: true`.

Stable accessible controls for a separate real integration witness:

- **Stop uncertain launch and retain output** opens the initial review dialog.
  The confirmation button has the same name; scope it to the dialog.
- The checkbox starts with **I authorize recovery of this exact attempt** and
  names stopping the original worker and retaining a permanent Docker marker.
- Pending/proved recovery uses **Continue launch recovery** and the dialog
  confirmation **Continue reviewed recovery**.
- **Refresh recovery availability** is read-only; **Keep inspecting** closes.

Actual Docker fencing, held request/process death, durable restart recovery,
current authorization and installed-package/native Windows behavior require
separate integration evidence. This fixture does not satisfy those gates.
