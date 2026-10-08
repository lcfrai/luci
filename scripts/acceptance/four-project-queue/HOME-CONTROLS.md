# Installed four-project queue controls

`home-controls.mjs` exercises the installed Linux web app against its real local
backend. It creates its own private state, accounts and four projects. It never
uses an existing workspace or Docker daemon.

The fixture explicitly seeds approved context, run and queue records through the
API. Browser controls then pause and reprioritize four queue entries, cancel one,
and attempt a two-project resume after execution access is revoked for one
selected project. Assertions compare actual queue IDs and revisions and require
one successful resume, one visible rejection, and no mutation of the rejected
entry. A separate browser check selects an older run, returns to the overview,
and verifies that the next action requests the exact latest run.

A separate, explicitly controlled UI witness first reads an actual project
overview, then intercepts **only that project's overview GET** to return
HTTP409 `branch_busy`. It requires the previous goal and stage to remain visibly
labelled as recorded, hides the stale next action, and changes a real queue
entry's priority through the browser while the overview response remains busy.
Subsequent injected HTTP403 and HTTP500 responses must clear the private summary;
a fresh successful backend read must restore it. The report separates these
interceptions under `controlledOverviewResponses` and records the real queue
revision change. **This does not establish actual server source-lock behavior
or a real access revocation for the overview.** Native slow-clone/source-lock
acceptance and backend lock-availability evidence remain separate gates.

Dispatch stays globally paused throughout. Docker points to an absent private
Unix socket, all inherited provider/cloud settings are omitted, browser requests
are restricted to the private app origin, and a loopback provider trap must
receive zero requests. Final checks require zero reserved operations and child
IDs. This is UI and authorization evidence, **not agent execution, model quality,
parallel performance, Windows, installer or cloud acceptance**.

## Run

Prerequisites: a non-root Linux account, Node with the repository's desktop
dependencies installed, Python 3.9+, Chromium, and an extracted release package
with its clean-build provenance and original archive. No Docker installation is
needed. The provenance JSON must identify the archive's local path and exact
SHA-256, binary SHA-256, web-index SHA-256, clean source status and source commit.
`verify-package.py` compares every regular archive member with the installed
package before any service starts.

From the repository root:

```sh
node scripts/acceptance/four-project-queue/home-controls.mjs \
  --package /path/to/extracted-package \
  --provenance /path/to/package-provenance.json \
  --expected-source FULL_40_CHARACTER_SOURCE_COMMIT \
  --report /path/to/fresh-evidence-directory/result.json \
  --fixture-root /workspace/.s \
  --chromium /usr/bin/chromium
```

Choose a short, writable fixture parent: Linux Unix socket paths are bounded.
The driver creates an unpredictable private subdirectory, dynamically chooses
loopback ports, and removes the fixture after its owned service and browser
processes stop. It writes a sanitized report, exact driver/verifier copies and
desktop/phone screenshots into a fresh evidence directory. Credentials stay in
memory or the disposable private fixture and are excluded from reports. Preserve
failed reports when correcting fixture assumptions; do not reuse an evidence
directory. `--help` only prints usage.

## Recorded witness

[The sanitized e81b5860 receipt](evidence/home-controls-e81b5860.json) passed on
2026-10-08 using source `e81b586077f8af97e5e59472e34eafd161f16b8a`, Chromium
151.0.7922.173 and all 947 verified installed package members. It records all real
queue mutations, the explicit fixture setup, the partial permission result, exact
run IDs and successful cleanup. The matching screenshots and full package receipt
are in `artifacts/quality-swarm/home-controls-e81-attempt4/` in the acceptance
workspace; screenshots are supplementary to the checked API/DOM outcomes.

Earlier attempts preserved in sibling artifact directories found fixture setup
errors: a too-short request key; a team viewer grant that correctly prevented
project-level edit escalation; and an expectation that queue-focused navigation
would provide a latest-run ID. The final fixture corrects those assumptions.

This receipt predates the `branch_busy` overview fallback and autonomous task
worker lifecycle. Those changes have separate component/typecheck evidence and
need a new packaged witness before claiming installed-release coverage. The
accepted e81 driver copy remains in its artifact directory; it saved Ask for its
non-executing fixture. The current driver explicitly saves Bypass to satisfy the
new task capability contract, still with dispatch paused and Docker absent. It
does not change any existing user route. It also adds the controlled overview
response witness described above; the e81 receipt contains no such claim. Rerun
this driver against the replacement package; the e81 receipt does not certify
these later revisions.
