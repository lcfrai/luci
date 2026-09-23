### Task 27: The images — broker, machine, Hub — and the `images` pipeline

Four images run in the environment. One exists (`images/sandbox/Dockerfile:1-34`) and has never
been published (survey §3.3, `:374-375`); three are new. P8-D8 fixes how all of them are built:
**Go binaries are compiled outside Docker** (`CGO_ENABLED=0 GOOS=linux go build`) and copied in,
because the corporate proxy breaks `go mod download` inside a build (`images/sandbox/Dockerfile:3-8`)
and the control-plane module is not vendored; and every image is **referenced by digest**, so the
sandbox image's digest is also OD-8's adapter pin.

**Files:**
- `images/broker/Dockerfile`, `images/hub/Dockerfile`, `images/machine/Dockerfile` (new)
- `images/build.sh` — builds the binaries and the four images locally, the same way CI does (new)
- `images/images_test.sh` — the image assertions below, run locally and in CI (new)
- `pipelines/images.yml` (new); `pipelines/templates/tools.yml` — appends the image tools (modify, after task 28)

#### Contract — the four images

| Image | Base | Contents | Runs as | Entrypoint |
|---|---|---|---|---|
| `panacea-app/sandbox` | `node:20-bookworm-slim` (unchanged, `Dockerfile:10`) | unchanged: sandboxd from vendor, git, ripgrep, Claude Code `2.1.274` | `dev` 1001 | `entrypoint.sh` |
| `panacea-app/machine` | **`FROM` the sandbox image, by the digest this pipeline run just built** | + `/usr/local/bin/machined`; + git from `bookworm-backports` (below) | `dev` 1001 | `/usr/local/bin/machined` |
| `panacea-app/broker` | `gcr.io/distroless/static-debian12:nonroot`, **by digest** | `/broker` | 65532 | `/broker` |
| `panacea-app/hub` | the same distroless base, by digest | `/hub` (`control-plane/cmd/hub`, ADR-H1 `:286`) | 65532 | `["/hub", "serve"]`; the chart's migrate Job runs `["/hub", "migrate"]` (ADR-H1 `:500-501`) |

**Why the machine image adds git.** Phase 3's `gitx.MinVersion` is `2.40.0` (`merge-tree
--merge-base`, phase 3 task 02 `:128-130`), and `machined` runs `gitx.CheckVersion` at start
(task 21). Debian bookworm's own git is the 2.39 line — **from training knowledge, UNVERIFIED** —
so the machine image installs git from `bookworm-backports` (`USER root`, `apt-get install -t
bookworm-backports git`, `USER dev`), and `images_test.sh` asserts the version rather than trusting
this sentence. The **sandbox** image is left alone: an agent's git is the agent's, and phase 3 runs
nothing of its own there.

`images/build.sh` (contract):

```bash
#!/usr/bin/env bash
# images/build.sh [--push <registry>]   — the ONE way images are built, locally and in CI.
#   1. go build (CGO_ENABLED=0 GOOS=linux GOARCH=amd64, -trimpath,
#      -ldflags "-s -w -X main.version=<git sha>") of cmd/broker, cmd/machined, cmd/hub
#      into images/.out/ (gitignored)
#   2. docker build images/sandbox (context: repo root, as today)
#   3. docker build images/machine  --build-arg SANDBOX_IMAGE=<the sandbox image id from step 2>
#   4. docker build images/broker, images/hub
#   5. with --push: docker push each, then write images/.out/digests.json:
#        {"sandbox":"<registry>/panacea-app/sandbox@sha256:…", "machine":…, "broker":…, "hub":…}
# Tags are the git sha only; nothing deploys by tag.
```

#### Contract — `pipelines/images.yml`

- **Triggers**: PR and `main`/`dev` pushes on paths `images/**`, `sandboxd/**`, `control-plane/**`,
  `go.work`.
- **Jobs**: `build` (the script, no push) → `sbom` (an SPDX SBOM per image, published as the
  pipeline artefact `sbom`) → `scan` (a vulnerability scan per image; **fails on a CRITICAL with a
  fix available**; the report is an artefact) → `canary` (below) → `push` (**`main`/`dev` only**,
  `--push` to the environment's ACR through the OIDC service connection; publishes
  `images/.out/digests.json` as the artefact **`image-digests`**, which task 30's `deploy` consumes).
- **The OD-8 canary**: the job runs phase 3's transcript-adapter canary (its task 17,
  `internal/transcript/claudecode`) against **the sandbox image built in this run**, in whatever
  form that task defines. If phase 3 task 17 has not landed, the job prints
  `canary: phase 3 task 17 not present — skipped` and passes; it never silently disappears.
- **Tools** (SBOM generator, scanner) are pinned in `pipelines/templates/tools.yml` by version and
  checksum; this task appends them.

#### Tests to write first — `images/images_test.sh` (Docker Desktop locally; the CI agent in the pipeline)

| Assertion | Mutant it kills |
|---|---|
| `docker run --rm panacea-app/broker:<sha>` exits **2** and prints `PANACEA_APP_BROKER_ENTRA_TENANTS is required` | a broker image that starts with no configuration |
| the broker and Hub images have **no shell**: `docker run --rm --entrypoint /bin/sh …` fails to start | a debug base that ships a shell to anyone who gets exec |
| `docker inspect` user of broker and Hub is `65532` (nonroot), of machine `1001` | a root container on a `restricted` namespace (task 11 would reject it at admission, late) |
| `docker run --rm --entrypoint git panacea-app/machine:<sha> version` ≥ **2.40.0** | `machined` failing `CheckVersion` on first wake |
| the machine image's `/usr/local/bin/claude --version` equals the sandbox image's (`2.1.274`) | a machine image built `FROM` a different sandbox than the one sessions run — two CLI pins |
| `docker run --rm --entrypoint /usr/local/bin/machined panacea-app/machine:<sha>` exits 2 naming `PANACEA_APP_MACHINED_TOKEN` | a machine agent that serves unauthenticated |
| every `FROM` in `images/broker/Dockerfile` and `images/hub/Dockerfile` carries `@sha256:` | a floating base |

#### Steps

- [ ] **Step 1:** Check whether the Hub plan already ships an `images/hub/Dockerfile`; if it does,
  use it and drop that row here.
- [ ] **Step 2:** `images_test.sh`; `build.sh`; the three Dockerfiles; run both scripts on Docker
  Desktop (PowerShell for Docker, as phase 2's constraint says for path handling).
- [ ] **Step 3:** `pipelines/images.yml`; append the tools to the template; commit.

#### Acceptance

```bash
bash images/build.sh                  # four images, tagged with the git sha
bash images/images_test.sh            # images_test: 7 of 7 passed
```

The pipeline itself is exercised on its first PR run in ADO (an owner input: the pipeline must be
registered, task 30). Until then only the scripts are verified.

#### What this task does not claim

- That the distroless base's current digest has no CRITICAL finding on the day the scan first
  runs.
- Licence scanning **of image contents**: OS packages (git, node, glibc) are not linked code; the
  OD-11 licence gate (task 28) covers the Go, npm and Cargo dependency graphs. The SBOM makes the
  image contents reviewable; it does not gate on them.
