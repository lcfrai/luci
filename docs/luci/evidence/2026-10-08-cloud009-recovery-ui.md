# CLOUD009: reviewed launch recovery UI

The frontend now offers **Stop uncertain launch and retain output** as a
separate action from existing absence-proof recovery. It reads support for the
selected attempt only when the dialog opens. Current project, build revision,
task and attempt remain visible, and a separate checkbox authorizes stopping
that original worker if necessary and retaining a permanent Docker recovery
marker. It never starts a replacement agent.

Recorded pending and proved recovery retain capacity until finalization. The UI
does not treat either state as a completed task or offer an automatic retry.
Legacy, backend and marker-limit refusals retain the server's explanation.
Changed revisions and revoked authority discard consent; late responses cannot
revive it. An uncertain mutation triggers one readback, without replaying the
mutation. Existing `recoverMissingChild` requests remain distinct.

**124 focused tests across seven files passed**, with zero failed, pending or
todo tests; TypeScript also passed. Source hashes were unchanged during those
checks. This is a focused new-feature check, not another complete frontend
baseline. The earlier 2,394-test baseline predates this feature.

Linux Chromium rendered the actual source components and CSS at 1440×1080 and
390×844 using explicitly controlled API callbacks. Both widths passed modal
focus trapping/restoration, consent, geometry and axe checks with zero findings.
The browser also exercised refusal, an uncertain callback response with one
readback and no mutation replay, and a controlled project switch during a delayed
availability read. Desktop and phone screenshots were visually inspected; the
phone dialog scrolls internally to its confirmation controls without horizontal
clipping.

- [Reproduction helper and exact accessible controls](../../../scripts/acceptance/launch-recovery-ui/README.md)
- [Summary JSON](2026-10-08-cloud009-recovery-ui.json)
- [Focused test and TypeScript receipt](cloud009-recovery-ui/final/result.json),
  [individual test outcomes](cloud009-recovery-ui/final/vitest.json),
  [test log](cloud009-recovery-ui/final/tests.log), and
  [TypeScript log](cloud009-recovery-ui/final/typecheck.log)
- [Browser receipt and full frontend hashes](cloud009-recovery-ui/browser/result.json)
- [Phone review](cloud009-recovery-ui/browser/recovery-review-390.png),
  [phone confirmation controls](cloud009-recovery-ui/browser/recovery-consent-390.png),
  [desktop review](cloud009-recovery-ui/browser/recovery-review-1440.png), and
  [legacy refusal](cloud009-recovery-ui/browser/recovery-legacy-refusal-390.png)
- [Original and curated artifact hashes](cloud009-recovery-ui/artifact-manifest.json)

The browser fixture omits the unrelated queue panel and uses synthetic project,
operation and recovery callbacks. Its `POST stop` record is a fixture callback,
not an HTTP request. **It stops no real worker and creates no Docker marker.**
Actual held-launch recovery, durable restart/finalization, current backend
authorization, installed-package behavior and native Windows remain separate
integration gates. These source checks neither replace nor pass those gates.
