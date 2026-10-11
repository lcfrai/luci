# CLOUD-089 — never-started Docker objects are not live memory/CPU demand

Ready for integrated and native Windows retest. This is a separate confirmed
defect from CLOUD-087's original `resource_observation_unavailable` refusal;
that failing snapshot was not retained and its cause remains unknown.

The observer previously put every `created` container into the live inventory.
A never-started unrelated container has no current stats and may have no hard
limits. The unchanged admission code then refused runtime installation with
`execution_limits_unknown`. The original controlled observer and actual
observer-to-admission failures are retained here.

The correction omits an ordinary created object from live CPU/RAM demand only
after inspecting its exact immutable identity and confirming created status,
no running/paused/restarting/dead state, no PID, no restart count and no prior
start time. Missing, changed or inconsistent observations stay conservative.
Launch-fence labels still require the existing full inert-marker shape check.
The 128-item inspection budget remains. Storage stays part of the measured
disk bounds; no zero-usage row is invented.

An omitted owned object remains absent from the live-observation identity set,
so pending reservations remain charged. The new API regression reserves and
binds a 6 GiB/1000 mCPU worker, reopens admission, then exercises worker, check,
preview and runtime-install admission. All four retain that exact ledger and
refuse another 3 GiB demand with `memory_pressure`. It also checks that unknown
running, paused and restarting foreign workloads still refuse admission.
No reservation is released by resource observation.

Final selected run: **11 top-level tests and 26 subtests passed, zero failed**.
One additional opt-in Docker storage test was skipped by its existing explicit
environment gate. There was no daemon workload, allocation, model call,
Windows execution, installer build or full-suite run. Counts overlap the
earlier regression runs and must not be added together.

The original observer fails the two created-container subcases. The first API
fixture omitted read-only `Ping` and panicked; this fixture error is retained
separately. The corrected fixture then reproduces the actual admission refusal
and the expected-memory-pressure assertion receives `execution_limits_unknown`;
the latter does **not** establish lost or released capacity. The identical
corrected fixture passes after the observer change.

`summary.json` records the commands, counts and selected final source hashes.
`before-source` preserves the original observer/new regression and the first
API fixture; `before-corrected-fixture` preserves the unchanged corrected API
regression. `after-source` contains the final three changed source files.
Raw JSONL and stderr files are retained, including failures. These are selected
source bindings, not a complete compiled-input or clean whole-checkout claim:
other workstreams had independent uncommitted changes in the shared workspace.

For native retest, run the same new tests plus the affected observer/admission
controls listed in `summary.json`, using the integrated published revision.
Then inspect real Docker-created objects and the cached resource snapshot
without deleting, starting or editing unrelated objects. Only subsequently
retry the bounded runtime installation under its unchanged capacity, storage
and authority policy. A new passing installation does not retrospectively
explain CLOUD-087's original unavailable observation.
