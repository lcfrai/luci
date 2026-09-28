### Task 27: The LiteLLM image we are allowed to ship

**The stock image carries a file whose licence forbids us to distribute it, and we distribute it into a customer's own tenancy.** LiteLLM's root `LICENSE` reads *"All content that resides under the `enterprise/` directory of this repository, if that directory exists, is licensed under the license defined in `enterprise/LICENSE`. Content outside of the above mentioned directories … is available under the MIT license"* (© 2023 Berri AI), and the carve-out file is *"The BerriAI Enterprise license … Copyright (c) 2024 - present Berrie AI Inc."* — read verbatim from `BerriAI/litellm@main/LICENSE` (1,419 B) and `…/enterprise/LICENSE.md` (2,223 B) on 2026-09-28, recorded at `docs/superpowers/planning/2026-09-28-oss-adoption/licence-platform.md:30`. The official Dockerfile copies `/app/enterprise` into the final image, and `pip install 'litellm[proxy]'` pins the separate `litellm-enterprise` distribution (`licence-platform.md:95`). OD-5 ships this stack into the customer's cloud, and even the we-host tier makes it a KPMG-published artifact. This task is OD-33's second half (`docs/superpowers/planning/2026-09-23-owner-decisions.md:203-222`, on the main tree's branch; this worktree's copy stops at OD-29).

**The obligation is on the build, not on a person.** The audit's own wording: *"do not rely on a developer remembering. If either is adopted, the build must fail when the proprietary path is present in the produced image, and the plan must name which artifact we take"* (`licence-platform.md:189`). So this task's real deliverable is not a Dockerfile; it is a test that fails on the stock image and passes on ours, run by the build and not by a reviewer.

**The version: `v1.102.1`, published 2026-09-23.** It is the latest release from `api.github.com/repos/BerriAI/litellm/releases/latest`, read 2026-09-28 (`docs/superpowers/planning/2026-09-28-oss-adoption/06-platform.md:1163`), and its release notes lead with cosign image signing and a recommendation to pin the **commit hash** rather than the tag, which this task does. One more reason, and it is the operational one: task 18's sixteen probes and task 26's canary are all written against a single pinned image, so the version this task fixes is the version those results mean. A different version there and here would make both records unreadable.

**Two removals, not one.** The directory is the visible half. `pip install 'litellm[proxy]'` also pins the `litellm-enterprise` **distribution**, so an image built from the MIT tree with `enterprise/` deleted can still install the proprietary code from PyPI (`licence-platform.md:95`, and the audit's section heading: *"the MIT/Enterprise split is not a directory you can simply avoid"*). The test checks both, by path and by dist-info name.

**Decisions this part makes:**
- **(a) Build from the MIT source tree at a pinned commit, never patch the published image.** A `RUN rm -rf /app/enterprise` on top of `berriai/litellm` deletes nothing: the bytes stay in the layer beneath, `docker save` still ships them, and any customer with the image can read them out. Distribution is what the licence forbids, and a whiteout file is still distribution.
- **(b) The test reads the **layers**, not a running container.** `docker save` the image, walk every layer tar's entry names, and fail on any `enterprise/` path or any `litellm[-_]enterprise*` dist-info. A `docker run … ls /app` check passes on an image whose lower layer still carries the files, which is exactly the failure (a) describes.
- **(c) It has to fail on the stock image before it means anything.** Step 4 runs the same script against `berriai/litellm:v1.102.1` and requires exit **1**. A build check that has never seen the thing it forbids is a check that has never run.
- **(d) Pin by digest; record the source commit beside it.** `images/litellm/Dockerfile` takes `ARG LITELLM_REF=<40-hex commit of v1.102.1>`; the built image's digest goes in one file, `images/litellm/DIGEST`, which task 17's compose and task 19's chart values read. A tag is not a pin.
- **(e) How the proxy is installed without the enterprise distribution is **UNVERIFIED** until the first build**, and the part says so rather than guessing. Two candidates, and the test decides between them, not a preference: **(i)** install from the pinned source tree with `enterprise/` removed and the enterprise pin dropped from the requirements file; **(ii)** install `'litellm[proxy]'` and then remove the `litellm-enterprise` distribution, asserting its absence. Whether the proxy **starts** without it is the open question; step 5 measures it.
- **(f) If the proxy cannot start without the enterprise distribution, that is a finding for the owner, not a workaround.** It would mean the shippable artifact is the MIT SDK and not the proxy, which reopens the gateway choice the audit already priced (Portkey MIT, Envoy AI Gateway Apache-2.0, `licence-platform.md:97`). OD-33 keeps LiteLLM and OD-21 stands, so nothing here reopens it on its own; the task records the measurement and stops.
- **(g) The notices ship with the image.** `THIRD-PARTY-NOTICES.md` carries LiteLLM's MIT text and its copyright line, in the image at `/licenses/` and in the repository. MIT's one obligation is the notice, and an image that satisfies the hard rule while dropping the easy one fails the same audit.

**Files:**
- `images/litellm/Dockerfile` — the MIT-only build, `ARG LITELLM_REF` pinned to the `v1.102.1` commit (new)
- `images/litellm/build.sh` — build, capture the digest into `DIGEST`, run the layer test, refuse to write `DIGEST` if it fails (new)
- `images/litellm/layers_test.sh` — the check, over `docker save`, exit `0`/`1`/`2` (new)
- `images/litellm/DIGEST` — one line, `sha256:…`, the only place a consumer reads (new)
- `images/litellm/THIRD-PARTY-NOTICES.md` — LiteLLM's MIT notice (new)
- `docs/verification/2026-09-28-phase-6-litellm-image.md` — one row per build: date, source commit, digest, layer-test verdict, stock-image control verdict, who built it (new)
- Amended, not written here: `task-17.md`, `task-18.md`, `task-19.md` (the amendments table below)

**Interfaces:**
- Produces:
  - the image, tagged `panacea-app/litellm:v1.102.1-mit`, and its digest in `images/litellm/DIGEST`
  - `images/litellm/layers_test.sh <image ref>`, exit `0` clean, `1` a forbidden path found (with the layer and the entry printed), `2` the image could not be read
  - `build.sh`, which writes `DIGEST` only on a `0`
- Consumes: nothing in this repository. Upstream `BerriAI/litellm` at the pinned commit, and a Python base image pinned by digest.
- Consumed by: task 17 (compose's `litellm` service), task 19 (chart values), task 18 (every probe result names this digest), task 26 (the canary's evidence line carries it).

**Amendments this task carries.**

| Part | The amendment | Why |
|---|---|---|
| `task-17.md` | Compose's `litellm` image is our ref read from `images/litellm/DIGEST`, never `berriai/litellm`. `compose_test.sh` gains one row: the `litellm` image is the digest in that file, and the string `berriai/litellm` appears nowhere in `compose.yaml` | Task 17's pin is "by digest" today, with no statement of **which** image. That is how the stock image gets in |
| `task-19.md` | `values.yaml`'s LiteLLM image is the same digest; `render_test.sh` gains the same two assertions over `helm template` | The chart is what a customer's tenancy actually runs |
| `task-18.md` | The probe record's header names the source commit as well as the digest, and a probe run against any other image is void | "What the pinned LiteLLM does" is only evidence if the pin is ours |

**Tests, and the mutant each kills.** `layers_test.sh` is the whole suite; each row is one assertion in it, and the stock-image control (step 4) is what proves the suite can fail at all.

| # | Assertion | Mutant killed |
|---|---|---|
| 1 | No layer holds an entry whose path contains `enterprise/` | The defect itself: the stock `COPY`, or a re-introduced one after an upstream bump |
| 2 | No layer holds a `litellm[-_]enterprise*` dist-info or `.pth` | Installing `'litellm[proxy]'` and deleting the directory: the PyPI half of the split (`licence-platform.md:95`) |
| 3 | The check walks **every** layer, and fails if the image has fewer layers than it could read (a `docker save` that half-worked is exit 2, never 0) | A script that reads the top layer only, or one whose `tar -t` failed and whose empty output read as "clean" |
| 4 | `/licenses/THIRD-PARTY-NOTICES.md` is present in the image and contains `MIT` and `Berri` | Decision (g): the notice quietly dropped while the hard rule passes |
| 5 | The image's own label `org.opencontainers.image.revision` equals the `LITELLM_REF` the build was given | A build from `main`, or from a tag that moved, recorded as if it were the pin |
| 6 | Run against `berriai/litellm:v1.102.1`, the script exits **1** and names `enterprise/` | A check that cannot fail — the one failure mode a licence gate must not have |

- [ ] **Step 1: Preflight, and record what upstream actually looks like**

```bash
docker --version > /tmp/p6t27a.out 2>&1; echo "exit=$?"
mkdir -p /tmp/p6t27src && cd /tmp/p6t27src
git clone --filter=blob:none --no-checkout https://github.com/BerriAI/litellm.git . > /tmp/p6t27b.out 2>&1; echo "exit=$?"
git rev-list -n1 v1.102.1 > /tmp/p6t27c.out 2>&1; echo "exit=$?"; cat /tmp/p6t27c.out   # the 40-hex REF
git checkout -q v1.102.1 && grep -n 'enterprise' Dockerfile
grep -rn 'litellm-enterprise\|litellm_enterprise' pyproject.toml requirements.txt 2>/dev/null
```

The `grep -n 'enterprise' Dockerfile` line is the one to record: OD-33 names **lines 75 and 150**, read from the audit and **not re-read against `v1.102.1` by this plan**. Write the real line numbers at the pinned tag into the verification document's first row. If they differ, that is drift between the audit's `main` and the pinned tag, not a contradiction — the count of `COPY` lines touching `enterprise/` is what matters, and any count above zero is the defect.

- [ ] **Step 2: Write the Dockerfile**

Multi-stage. Stage one takes the pinned ref, removes `enterprise/` **before anything installs**, and drops the enterprise pin from the requirements; stage two is a slim Python base pinned by digest, non-root, with `/licenses/THIRD-PARTY-NOTICES.md` and the OCI labels decision (d) and assertion 5 need:

```dockerfile
ARG LITELLM_REF=<40-hex, from step 1>
FROM python:3.13-slim@sha256:<pinned> AS src
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone --no-checkout https://github.com/BerriAI/litellm.git . \
 && git checkout -q ${LITELLM_REF} \
 && rm -rf enterprise .git \
 && test ! -e enterprise          # the build fails here, in the stage that has it
# … install into /opt/venv, per decision (e)'s candidate (i) or (ii) …
FROM python:3.13-slim@sha256:<pinned>
COPY --from=src /opt/venv /opt/venv
COPY images/litellm/THIRD-PARTY-NOTICES.md /licenses/THIRD-PARTY-NOTICES.md
LABEL org.opencontainers.image.revision="${LITELLM_REF}" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/BerriAI/litellm"
USER 65532:65532
```

The `rm -rf enterprise` sits in the **source** stage and nothing from that stage is copied except the virtualenv, so no layer of the final image ever held the directory — which is decision (a)'s whole point and what assertion 1 measures.

- [ ] **Step 3: Write `layers_test.sh`**

`bash`, `set -euo pipefail`, one argument (the image ref), no Docker network needed:

1. `docker save "$1" -o "$tmp/img.tar"`, then extract it. A non-zero exit, or an archive with no `manifest.json`, is exit **2** with the reason — never a pass.
2. Read the layer list from `manifest.json`, count it, and require the count to be at least 1. For each layer, `tar -tf` it into one stream of entry names.
3. Fail with exit **1**, printing the layer id and the first three offending entries, on any entry matching `(^|/)enterprise/`, or `litellm[-_]enterprise` (dist-info, `.pth`, wheel or source directory).
4. Require `licenses/THIRD-PARTY-NOTICES.md` in some layer, and `docker inspect` to show `org.opencontainers.image.revision` equal to `$LITELLM_REF` when that variable is set. A missing notice or a mismatched revision is exit 1 with its own message.
5. Print one evidence line, whatever the verdict: `IMAGE <CLEAN|FORBIDDEN|UNREADABLE> ref=<image> digest=<sha256:…> revision=<40-hex> layers=<n> hits=<n> at=<RFC 3339>`.

Assertion 3 is the one that is easy to get wrong: a `tar -tf` that fails prints nothing, and a `grep` over nothing finds nothing, which reads as clean. Every stage checks its own exit status into a variable before the next one runs, and the entry count per layer is part of the evidence line for the same reason.

- [ ] **Step 4: Prove the test fails on the stock image**

```bash
docker pull berriai/litellm:v1.102.1 > /tmp/p6t27d.out 2>&1; echo "exit=$?"
bash images/litellm/layers_test.sh berriai/litellm:v1.102.1 > /tmp/p6t27e.out 2>&1; echo "exit=$?"   # expect 1
grep -c 'enterprise' /tmp/p6t27e.out    # expect at least 1
```

Exit **1** here is the acceptance of this step. Anything else — a `0`, or a `2` — means the script cannot see the thing it exists to stop, and the task is not done, whatever our own image reports. Paste the evidence line into the verification document as the control row. If the registry is unreachable from this network, record it as **not run** and mark the whole task's verdict **UNPROVEN** rather than passing on our image alone (the corporate network blocks some hosts; `CLAUDE.md`, working agreements).

- [ ] **Step 5: Build, measure whether the proxy starts, record**

Run `build.sh`. It builds, runs `layers_test.sh` on the result, and writes `DIGEST` only on a `0`. Then the question decision (e) leaves open: start the container with task 02's generated `litellm.config.yaml` and a throwaway master key, and read `GET /health/readiness` (the path LiteLLM documents; **UNVERIFIED** against this image until task 18's first run, so treat a 404 there as "the endpoint is named differently", not as a failure to start). Three outcomes, all recorded in the verification document:

| Outcome | What it means | What the task does |
|---|---|---|
| Starts, candidate (i) | The MIT tree alone serves the proxy | Ship it; note in the record that no enterprise distribution is installed |
| Starts, candidate (ii) only | `[proxy]` is needed, and the enterprise distribution can be removed after | Ship it, and assertion 2 is doing real work — say so |
| Does not start either way | The proxy is not separable from the enterprise distribution | **Stop.** Record it, raise it to the owner as decision (f) describes, and leave `DIGEST` unwritten. Tasks 17, 19 and 18 then have no image, which is the correct blocked state, not a reason to use the stock one |

- [ ] **Step 6: Make the three amendments**

Edit `task-17.md`, `task-19.md` and `task-18.md` as the amendments table says, each with a one-line note naming task 27 and OD-33.

- [ ] **Step 7: Verify and commit**

```bash
bash -n images/litellm/layers_test.sh > /tmp/p6t27f.out 2>&1; echo "exit=$?"
bash images/litellm/layers_test.sh "$(cat images/litellm/DIGEST)" > /tmp/p6t27g.out 2>&1; echo "exit=$?"   # expect 0
grep -c 'IMAGE CLEAN' /tmp/p6t27g.out            # expect 1
grep -rc 'berriai/litellm' deploy/local/gateway/compose.yaml deploy/charts/gateway/values.yaml   # expect 0 0
grep -c 'v1.102.1' docs/verification/2026-09-28-phase-6-litellm-image.md   # expect at least 1
git add images/litellm docs/verification/2026-09-28-phase-6-litellm-image.md
git commit -F <message file> -- images/litellm docs/verification/2026-09-28-phase-6-litellm-image.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-17.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-18.md docs/superpowers/plans/2026-09-25-phase-6-gateway-parts/task-19.md
```

Exit codes are read from a file, never through a pipe. Message: `build(gateway): LiteLLM v1.102.1 rebuilt from the MIT tree, enterprise/ gone, and a layer test that fails on the stock image (phase 6 task 27, OD-33)`, a body carrying the source commit, the digest, both verdicts from step 4 and step 5, and the attribution line.

**Acceptance:** step 4 exits 1 on `berriai/litellm:v1.102.1` and names `enterprise/`; step 7's run exits 0 with `IMAGE CLEAN`; `DIGEST` holds one `sha256:` line; the verification document holds the control row and the build row, both naming the source commit; `berriai/litellm` appears in no compose file or chart value.

**What this task does not claim:**

- **That the image is licence-clean in full.** It removes the one carve-out the audit verified and ships the MIT notice. LiteLLM's own transitive dependencies are not audited here; an SBOM and a full notice file are a separate piece of work, and nothing in this task should be read as one.
- **That upstream will keep the split where it is.** The check is by path and by distribution name at `v1.102.1`. An upstream that renames the directory, or moves proprietary code into the MIT tree, passes this test — which is why seam S6.13 says the check is re-read at every version bump and not just re-run.
- **That our rebuild is functionally identical to the stock image.** It is built from the same commit and installs the proxy, and task 18's sixteen probes are the measurement of what it does. Until they run, "it starts" is the only claim.
- **That the digest is reproducible.** Two builds of the same source produce different digests (timestamps, base-image metadata). The record names the source commit, which is the reproducible thing; the digest is the identity of one artifact.
- **That any of it has run.** Nothing in this task has been executed: no image was pulled, built or saved, and the line numbers OD-33 gives for the stock Dockerfile are the audit's reading, re-checked by step 1 and not by this plan.

**Gaps against the skeleton, for the orchestrator:**

1. **The skeleton's tech-stack line says "LiteLLM proxy (image pinned by digest in task 17)"** (`2026-09-25-phase-6-gateway.md:38`). It is now pinned in task 27 and consumed by 17; the skeleton's line is amended with the task list.
2. **No registry is named.** Local mode builds and uses the image by digest with no push. The cloud needs a registry a customer's AKS can pull from, which is phase 8's (ask **P8-9**, proposed here: the gateway image is built by our pipeline, signed, and pushed to the environment's registry; a customer-hosted tier pulls from theirs).
3. **Cosign signing is not built.** Upstream's release notes recommend it and `06-platform.md:1163` records that; signing our rebuild is the obvious next step and belongs with the pipeline that pushes it, not with the Dockerfile.
4. **The same obligation exists for Langfuse (`ee/`)** if it is ever adopted (`licence-platform.md:188`). Nothing in phase 6 uses it; the shape of `layers_test.sh` is the reusable part.

