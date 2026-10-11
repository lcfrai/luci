# CLOUD-084/088: protected launch and native outcome checkpoint

The controller now records the protected root launch identity before accepting the existing authenticated sandboxd typed Exit frame. It preserves the exact task/VM/process/source/context identity through restart, keeps the reservation charged, and does not infer output capture, review, integration or provider billing from process exit. The shared capture types bind a later producer to this previously observed kernel identity; structural guest validation cannot replace the controller's acknowledged-record validation.

This source checkpoint has controlled TLS/Connect/WebSocket and private filesystem evidence. It is **not** an actual E2B, KVM, template, upstream model, Windows, performance or whole-cloud workflow pass. Controller collection invocation and validated return ingestion are not wired yet. The guest collector's separately developed freeze/snapshot code ran with controlled kernel fixtures; its actual root-permission test skipped on this unprivileged host.

## Executed scope

`fixed-result.json` records a single joined Linux race run: **65 top-level +127 subtests passed for e2bbe; 15 top-level +44 subtests passed for microvmguest, with one root-permission test skipped**. Both bootstrap and collector commands compiled and have no command tests. There were zero test failures. These counts include existing tests and overlap earlier capsules; they are not additional unique acceptance checks.

All **61 selected repository input hashes** stayed unchanged across this run. `source-before.json` and `source-after.json` cover the selected adapter/backend/statelease/guest/command/module paths, not every compiled standard-library or external dependency input. The nine newly changed adapter files are copied under `source/`. The root guest sources are bound by the selected manifests and their separate guest capsule. The root's subsequent new-path exclusion correction is outside this run's claim. `run.py` is the exact wrapper. The `fixed` filenames label the resulting joined source, not an executed before/after counterexample for these new capabilities.

New meaningful oracles cover protected JSON/PID/operation refusal, guest structural checks versus controller acknowledgement, exact native-frame identity and stdout-text rejection, disconnected/unauthorized observations, no replay or released charge after restart, preserved journal bytes on unchanged observation, durable checkpoint failure/re-acknowledgement, and reviewed cancellation waking a held observation before exact VM removal. Cancellation remains explicit authority; native completion itself never deletes a VM. Controller Close retains its owned transport drain semantics from the earlier lifecycle capsule.

## Remaining integration boundaries

- Root-private `/luci-work/attempt/launch-kernel.json` is read over the configured authenticated envd file route, strictly bounded and matched to the acknowledged envd PID. The pinned direct command/bootstrap/entrypoint Exec chain must still be validated in the deployed template.
- Output metadata uses `output/source-manifest.json`, `output/changes.json` and `output/source/<path>`; the source manifest hash binds exact JSON bytes. These types do not themselves validate a full returned snapshot against its retained input baseline or establish that a cgroup is frozen.
- Actual template cgroup ownership/escape prevention, root permissions, KVM/provider private traffic, configured model egress and real-model output remain unverified. The backend is not selectable in the product API/scheduler yet.
- No cloud resource, model request, container build or credential import was performed by this run. Guard values and declared floors are retained in `guard.json`; the run is not whole-computer responsiveness evidence.
