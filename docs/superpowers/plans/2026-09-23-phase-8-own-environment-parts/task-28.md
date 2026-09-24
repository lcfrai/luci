### Task 28: CI for the monorepo — tool templates, and one pipeline per concern

Today CI is one file: `azure-pipelines.yml:1-17` builds, tests and vets `./sandboxd/...` and
`./control-plane/...` on `main` and `dev` pushes only — nothing for `desktop/` (which has
`typecheck`, `test` and `build` scripts, `desktop/package.json:7-13`), `infra/`, `images/`,
licences or secrets, and no PR trigger (survey §3.4, `:379-383`). OD-5 keeps a monorepo so a
cross-cutting change lands atomically (owner decisions `:16`); so **every tree a change touches is
checked in the same PR**, and nothing merges that a reviewer did not see green (survey §5,
`:439-449`).

**Azure DevOps is the CI of record** (P8-D13; `PROGRAM.md:11` "own ADO pipeline");
`github.com/lcfrai/panaceaapp` is a backup remote (OD-7), not a second CI. The root
`azure-pipelines.yml` **stays where it is** as the Go pipeline, so the definition already
registered against that path keeps working; the new pipelines live in `pipelines/` and each must
be registered in ADO and added to the branch policy as PR build validation — an owner action,
listed in task 31's runbook.

**Files:**
- `azure-pipelines.yml` — the Go pipeline: PR trigger, path filters, the tools template, `govulncheck` (modify)
- `pipelines/templates/tools.yml` — every tool, pinned by version and SHA-256 (new)
- `pipelines/{desktop,licences,secrets,infra,charts}.yml` (new)
- `pipelines/scripts/lint.sh` — the rules every pipeline file must follow, as a script (new)
- `licences/allow.yaml`, `licences/exceptions.yaml` — the OD-11 allowlist and the (empty) exception list (new)
- `licences/charts.yaml` — one row per Helm chart dependency, starting empty; task 13 adds the first rows (new)
- `pipelines/scripts/chart-licences.sh` — the Helm half of the OD-11 gate, with its cases (new)
- `.gitleaks.toml` — the secret scanner's allowlist of known test fixtures (new)

#### Contract — `pipelines/templates/tools.yml` parameters (complete)

```yaml
# pipelines/templates/tools.yml — steps template. Every tool is downloaded from its release URL,
# verified against the SHA-256 written here, and put on PATH. Nothing is `curl | sh`, nothing is
# "latest": Panacea's run-time installs drifted (survey §5, :474-476).
parameters:
  - name: tools
    type: object
    default: []        # any of: terraform, helm, helm-unittest, kubeconform, kubelogin,
                       # gitleaks, govulncheck, shellcheck, tflint, trivy, syft,
                       # go-licenses, cargo-deny, yq
steps:
  - ${{ each t in parameters.tools }}:
    - script: bash pipelines/scripts/install-tool.sh ${{ t }}
      displayName: install ${{ t }}
```

`pipelines/scripts/install-tool.sh <name>` holds one row per tool — **version, URL, SHA-256** —
and fails if the downloaded file's hash differs. The versions are chosen at implementation and
recorded in the script; Terraform is **1.15.8** (P8-D14). The same script is what a developer
runs locally, so CI and laptop cannot disagree.

#### The pipelines (triggers are `paths`-filtered on both `trigger:` for `main`/`dev` and `pr:` for every branch)

| File | Paths | Agent | Steps | Gate |
|---|---|---|---|---|
| `azure-pipelines.yml` (Go) | `go.work`, `go.work.sum`, `control-plane/**`, `sandboxd/**` | `ubuntu-latest` | Go 1.23 (`GoTool@0`, as `:9-11`); build, `go test ./sandboxd/... ./control-plane/... -race -count=1`, vet (today's three, `:12-17`); **`govulncheck`** on both modules; **`git diff --exit-code go.work */go.mod`** after the build (the `go 1.23` directive rule) | PR |
| `pipelines/desktop.yml` | `desktop/**` | `ubuntu-latest` **and** `windows-latest` | `npm ci`, `npm run typecheck`, `npm test`; on Windows also `cargo clippy --manifest-path desktop/src-tauri/Cargo.toml -- -D warnings`; `npm run tauri build` on `main` only (Windows minutes, survey §6.2 `:515`) | PR; signing is phase 9 (`PROGRAM.md` open risk 3) |
| `pipelines/licences.yml` | any `go.mod`/`go.sum`, `desktop/package*.json`, `desktop/src-tauri/Cargo.*` | `ubuntu-latest` | Go: `go-licenses report` for each module; npm: the **production** dependency graph's licences; Cargo: `cargo deny check licenses`; each compared with `licences/allow.yaml` | PR — **OD-11 rule 3, applied repo-wide** |
| `pipelines/secrets.yml` | all | `ubuntu-latest` | `gitleaks` over the PR's commits (`--log-opts=<base>..HEAD`), and over full history on `main` | PR |
| `pipelines/infra.yml` | `infra/**` | `ubuntu-latest` | `guards.sh`, then `validate.sh` **per directory as a job matrix** (both task 01's; task 10, which writes `role-allowlist.sh`, adds its step between the two in the same commit, so this task never calls a script that does not exist yet) (each `terraform test` takes minutes); `tflint` with the azurerm ruleset; `trivy config` (IaC misconfiguration) failing on HIGH; `shellcheck` over `infra/**/*.sh`, `images/*.sh`, `pipelines/scripts/*.sh` — **no Azure call, no credentials, safe on every PR** | PR |
| `pipelines/charts.yml` | `deploy/**`, `licences/**` | `ubuntu-latest` | first, once: **`bash pipelines/scripts/chart-licences.sh`** (every `deploy/charts/*/Chart.lock` dependency has a row in `licences/charts.yaml` whose licence is in `licences/allow.yaml`; review m6); then per chart: `helm dependency build`, `helm lint`, `helm unittest`, `helm template … \| kubeconform -strict -summary`. Task 13 later adds one cross-chart job: Go 1.23 and `go test ./deploy/tests/netpolcover/... -count=1` (every workload in a default-denied namespace has an allow in each direction it needs, and every `pre-install` hook's allow exists before the hook runs; the test runs `helm dependency build` itself) | PR |

`images.yml` is task 27's; `infra-plan.yml` and `deploy.yml` are task 30's and are **not
registered** until the environment exists.

#### Contract — `licences/allow.yaml` (complete)

```yaml
# OD-11 rule 3: dependencies are MIT, Apache-2.0, BSD or ISC only — applied to every linked
# dependency graph in this repository (Go modules, the desktop's production npm graph, Cargo)
# and to every Helm chart dependency (each Chart.lock entry, recorded in licences/charts.yaml).
# Tools that run in CI or at apply but are not shipped (terraform, helm, scanners) are not
# dependencies. Terraform providers are tools too: MPL-2.0, run by Terraform as separate
# processes, shipped in nothing, pinned by hash in each stack's .terraform.lock.hcl.
allowed:
  - MIT
  - Apache-2.0
  - BSD-2-Clause
  - BSD-3-Clause
  - ISC
# A dependency outside this list fails the gate unless licences/exceptions.yaml names it, its
# licence, why it is needed, and the owner's approval (a commit reference). The file starts empty.
# Excluded by name: tldraw (commercial licence), and anything AGPL or GPL (owner decisions :96-98).
denied_packages:
  - tldraw
  - "@tldraw/*"
```

**The first run will find things.** npm graphs routinely carry `0BSD`, `BlueOak-1.0.0`,
`CC-BY-4.0` (e.g. browser-support data) or `Python-2.0` packages. Step 3 runs the three reports
locally **before** the gate is enforced and records every finding in the commit message; a
production dependency outside the list becomes either a replacement task or an exception the
owner approves. The gate is not loosened to make the first run pass.

#### Contract — `licences/charts.yaml` and `chart-licences.sh` (the Helm half, review m6)

A chart's `Chart.yaml` has no standard licence field, so the licence of each chart dependency is
**recorded by a person** and the script makes that record mandatory and machine-checked. A new
dependency, or a version bump, cannot merge without a row a reviewer sees in the diff.

```yaml
# licences/charts.yaml — one row per (name, repository, version) in any deploy/charts/*/Chart.lock.
# licence is an SPDX id and must be in licences/allow.yaml (or named in exceptions.yaml).
# source is where the licence was read: the chart's LICENSE file or its repository's licence.
dependencies: []
#  - name: traefik                      # example shape only; task 13 writes the real rows
#    repository: https://traefik.github.io/charts
#    version: "<the pinned version>"
#    licence: <SPDX id read from source>
#    source: <URL of the LICENSE file read>
```

`chart-licences.sh` parses every `deploy/charts/*/Chart.lock`'s `dependencies:` list (with
`yq`, pinned in `install-tool.sh` like every tool) and fails, naming the chart and the entry, when:

| Case (a fixture under `pipelines/scripts/testdata/chart-licences/`) | Expected |
|---|---|
| a `Chart.lock` entry with no row in `licences/charts.yaml` | fail, names the entry |
| a row whose `version` differs from the lock's (a bump with no re-read licence) | fail |
| a row whose `licence` is not in `allowed:` and not in `exceptions.yaml` | fail |
| a row whose `name` is in `denied_packages:` | fail |
| a row with no `source` | fail |
| every entry recorded, every licence allowed | pass |
| no `Chart.lock` in the tree (true until task 13) | pass, printing `0 chart dependencies` |

Rows that match no `Chart.lock` entry are reported as stale and fail too, so the file cannot
drift into a list of licences nobody uses. The check reads only the repository: **no network
call**. It does not prove the recorded licence is right; that is the reviewer's read of the
`source` URL, and is stated as such in the task report.

#### Tests to write first — `pipelines/scripts/lint.sh` (runs in `infra.yml`, and locally)

| Rule | Mutant it kills |
|---|---|
| every pipeline file has `trigger:` and `pr:` blocks, each with `paths.include` | a pipeline that runs on every change, or on none |
| every pipeline that installs a tool uses `templates/tools.yml` | a `curl` install with no checksum |
| no `latest` / `*` version and no `\| sh` / `\| bash` anywhere under `pipelines/` | drift by the next release |
| every `azureSubscription:` value is a `$(variable)`, never a literal name | a service connection hard-coded to one estate |
| `infra-plan.yml` and `deploy.yml` start with the estate guard (task 30) | a deploy that can target the wrong environment |
| `install-tool.sh` has a SHA-256 for every tool it names | a tool added without its hash |
| every `bash <path>` a pipeline file runs names a file that exists in the tree | a pipeline calling a script a later task has not written yet (review m2: `role-allowlist.sh` is task 10's, so task 10 adds its step) |
| each file names only its own connections: `infra-plan.yml` names only `PANACEA_APP_<ENV>_PLAN_SERVICE_CONNECTION`; `images.yml` names only `…_PUSH_…`; `deploy.yml` names `…_INFRA_…` in `apply`, `…_CLUSTER_…` in `cluster` and `smoke`, and `…_PLAN_…` in the outputs-only job; every other file names none (task 30's service-connection table) | a write connection named in a PR-built file. Like every rule here this is hygiene: the ADO branch control on the connection is the boundary (review C3) |

#### Steps

- [ ] **Step 1:** `lint.sh` and its cases; `install-tool.sh` with the pinned rows;
  `chart-licences.sh` and its seven fixture cases, each shown failing before the script exists.
- [ ] **Step 2:** The template; the Go pipeline edit; the five new pipelines; `lint.sh` passes.
- [ ] **Step 3:** Run the three licence reports and `gitleaks detect` locally; record the findings;
  write `.gitleaks.toml` for the test fixtures (the all-ones GUIDs, the fake tokens in tests) —
  **by path and exact value, never by a broad regex**.
- [ ] **Step 4:** Commit.

#### Acceptance

```bash
bash pipelines/scripts/lint.sh                   # lint: 7 of 7 rules pass over 6 pipeline files (images.yml, task 27, comes after)
bash pipelines/scripts/install-tool.sh gitleaks && gitleaks detect --no-banner   # no leaks found
bash pipelines/scripts/chart-licences.sh         # 0 chart dependencies (task 13 adds the first)
```

The pipelines themselves run only in ADO; **none has run**. Their first green PR run is task 31's
checklist item, after the owner registers them.

#### What this task does not claim

- That KPMG's ADO organisation allows the Microsoft-hosted Windows agent or has the parallel jobs
  (owner input, survey §7.1 item 4).
- That the npm and Cargo graphs pass the gate today (step 3 finds out).
