### Task 29: `deploy/charts/broker` and `deploy/charts/hub` — the two servers, and the SQL that lets the Hub in

Two charts, one per server, each owning its Deployment, service account, HTTPRoute, disruption
budget and network policy; plus the Hub's migrate Job and the one SQL file the deploy pipeline runs
as a Postgres administrator. The Hub's side is **the Hub contract's** (sibling `feat/hub`,
`2026-09-23-hub-contract.md` §12.2 `:2140-2191` and §12.3 `:2193-2208`, which answer HA-4 and
HA-5 "field by field"); nothing here sets a Hub variable that table does not list (seam S8-H.1).

**Files:**
- `deploy/charts/broker/{Chart.yaml,values.yaml,values.schema.json}`, `templates/{deployment,serviceaccount,rbac,service,httproute,pdb,networkpolicy}.yaml`, `tests/*_test.yaml` (new)
- `deploy/charts/hub/{Chart.yaml,values.yaml,values.schema.json}`, `templates/{deployment,serviceaccount,migrate-job,service,httproute,pdb,networkpolicy}.yaml`, `tests/*_test.yaml` (new)
- `deploy/sql/hub-principals.sql.tpl` — the Entra principals and grants for the Hub's two identities (new)
- `deploy/values/dev/{platform,edge,broker,hub}.yaml`, `deploy/values/prod/…` — per-environment values holding **no id** (new)

#### Contract — `deploy/charts/broker/values.yaml` (complete)

```yaml
image: ""                    # <acr>/panacea-app/broker@sha256:… — from the images pipeline's digests (task 27)
replicas: 2
namespace: panacea-broker
sandboxNamespace: panacea-sandboxes
host: ""                     # broker.<zone> — from foundation's edge_fqdns
gateway:
  name: panacea
  namespace: panacea-edge
entra:
  tenants: ""                # PANACEA_APP_BROKER_ENTRA_TENANTS
  audience: ""               # identity stack: broker_api_client_id
  allowedClients: ""         # identity stack: desktop_client_id
sandbox:
  image: ""                  # the sandbox digest
  machineImage: ""           # the machine digest
  runtimeClass: kata-vm-isolation   # one value feeds the broker AND platform.sandboxes.runtimeClass
  sessionCpuMillis: 2000            # one value feeds the broker's defaults AND platform.warm.balloons
  sessionMemoryMib: 4096
limits:
  maxCpuMillis: 8000
  maxMemoryMib: 32768
  maxSandboxesPerUser: 8
  maxMachinesPerUser: 2
machines:
  diskGib: 64
  storageClass: panacea-machine-disk
  affinity: true
  idleSleep: 30m
resources:                   # the broker's own pod
  cpu: 500m
  memory: 512Mi
```

The Deployment sets every `PANACEA_APP_BROKER_*` variable of task 17's table from these values
and nothing else; `priorityClassName: panacea-platform-critical`; `nodeSelector` apps pool;
restricted `securityContext` (non-root 65532, read-only root, drop ALL, `RuntimeDefault`);
`automountServiceAccountToken: true` — **the broker is the one pod that needs its token** — on a
service account `broker` with **no** workload-identity label (it touches no Azure API, task 10);
liveness `/healthz`, readiness `/readyz`; `terminationGracePeriodSeconds: 30`.

**`rbac.yaml`** — a `Role` in `panacea-sandboxes` (never a `ClusterRole`), bound to
`panacea-broker/broker`:

| API group | Resources | Verbs |
|---|---|---|
| `""` | `pods` | `create`, `get`, `list`, `watch`, `delete` |
| `""` | `secrets` | `create`, `get`, `update`, `delete` |
| `""` | `persistentvolumeclaims` | `create`, `get`, `list`, `update`, `patch`, `delete` |
| `policy` | `poddisruptionbudgets` | `create`, `get`, `delete` |

**Not granted, and tested as absent:** `pods/exec`, `pods/attach`, `pods/portforward` (P8-D7:
nothing uses exec), anything on `nodes`, `namespaces`, `serviceaccounts`, `roles`, `rolebindings`,
and `list`/`watch` on `secrets` (the broker reads a machine's token Secret by name only).

**`networkpolicy.yaml`** (in `panacea-broker`, on top of task 11's default-deny): ingress from the
Traefik pods in `panacea-edge` on 8080; egress to kube-dns, to `panacea-sandboxes` pods labelled
`panacea-app/component` in (`sandbox`, `machine`) on 8080/8081, to the API server (a
`CiliumNetworkPolicy` with `toEntities: [kube-apiserver]` — **AKS-managed Cilium's support for that
entity is UNVERIFIED**; fallback is an `ipBlock` for the API server's address, looked up at deploy),
and 443 to `0.0.0.0/0` except the RFC 1918 ranges (Entra's JWKS).

**`httproute.yaml`** — `HTTPRoute/broker`, `parentRefs` the Gateway, `hostnames: [host]`, one rule
to `Service/broker:8080`, timeouts `request: 0s` (a WebSocket attach is one long request).

#### Contract — `deploy/charts/hub/values.yaml` (complete)

```yaml
image: ""                     # <acr>/panacea-app/hub@sha256:…
replicas: 2                   # Hub contract §12.3: safe
namespace: panacea-hub
host: ""                      # hub.<zone>
gateway:
  name: panacea
  namespace: panacea-edge
env: dev                      # PANACEA_APP_HUB_ENV: dev | prod (never local)
database:
  host: ""                    # data stack: postgres_fqdn
  name: hub
  appUser: ""                 # workloads: hub_identity_name          (the DML identity, member of hub_app)
  migrateUser: ""             # workloads: hub_migrate_identity_name  (the DDL identity, member of hub_owner)
  maxConns: 10                # per replica; 2 × 10 + 1 migrate = 21 (Hub contract §12.3)
entra:
  tenants: ""                 # "<tid>=<org display name>"
  audience: ""                # identity: hub_api_client_id
  allowedClients: ""          # identity: desktop_client_id
serviceAccount:
  clientId: ""                # workloads: hub_client_id
migrate:
  serviceAccount:
    clientId: ""              # workloads: hub_migrate_client_id
resources:
  cpu: 500m
  memory: 512Mi
```

The Deployment's environment is exactly the Hub contract's deployed column:
`PANACEA_APP_HUB_ENV`, `PANACEA_APP_HUB_ADDR=0.0.0.0:8080`,
`PANACEA_APP_HUB_DATABASE_URL=postgres://<appUser>@<host>:5432/hub?sslmode=verify-full`,
`PANACEA_APP_HUB_DB_AUTH=entra`, `PANACEA_APP_HUB_DB_MAX_CONNS`,
`PANACEA_APP_HUB_TENANTS`, `PANACEA_APP_HUB_AUDIENCE`, `PANACEA_APP_HUB_ALLOWED_CLIENTS`,
`PANACEA_APP_HUB_TRUST_PROXY=true` (it sits behind the Gateway). Pod label
`azure.workload.identity/use: "true"` and the service account's `azure.workload.identity/client-id`
annotation make the webhook inject `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`,
`AZURE_FEDERATED_TOKEN_FILE`, `AZURE_AUTHORITY_HOST` (contract `:2171-2174`).
`terminationGracePeriodSeconds: 30` (contract: "≥ 30"). Liveness `/healthz`, readiness `/readyz`.
`automountServiceAccountToken: false` on the pod — the projected WI token is a separate volume the
webhook adds — **whether the webhook's projection works with automount off is UNVERIFIED**;
the test pins the choice and task 31's smoke observes it.

**`migrate-job.yaml`** — a Helm `pre-install,pre-upgrade` hook Job, `hook-delete-policy:
before-hook-creation`, service account `hub-migrate` (its own WI client id), command
`["/hub", "migrate"]`, env `PANACEA_APP_HUB_MIGRATE_DATABASE_URL=postgres://<migrateUser>@<host>:5432/hub?sslmode=verify-full`
and `PANACEA_APP_HUB_DB_AUTH=entra`, `backoffLimit: 0` (a failed migration stops the upgrade;
`--atomic` rolls back).

**`networkpolicy.yaml`** (in `panacea-hub`): ingress from Traefik on 8080; egress to kube-dns, to
the private-endpoint subnet on 5432 (`network.privateEndpointsCidr`, the same value task 11
takes), and 443 to `0.0.0.0/0` except RFC 1918 (Entra). **No CORS anywhere** (HA-6).

#### Contract — `deploy/sql/hub-principals.sql.tpl` (complete; rendered and run by `deploy`, task 30)

```sql
-- Run by the deploy pipeline as a member of the Postgres admin group (task 08), connected to the
-- `postgres` database with an Entra token. Idempotent. Placeholders are filled from the workloads
-- stack's outputs; nothing here is a secret.
--
-- 1. The Hub's own roles.sql, once per environment (Hub contract §10.1): the pipeline runs
--    control-plane/internal/hub/migrate/roles.sql first, in database `hub`.
-- 2. The two Entra principals, named exactly as the managed identities:
--    A DO block, because a function in a FROM clause runs before any WHERE could skip it.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{{HUB_APP_IDENTITY}}') THEN
    PERFORM pgaadauth_create_principal('{{HUB_APP_IDENTITY}}', false, false);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{{HUB_MIGRATE_IDENTITY}}') THEN
    PERFORM pgaadauth_create_principal('{{HUB_MIGRATE_IDENTITY}}', false, false);
  END IF;
END
$$;
-- 3. Membership: the DDL identity owns, the DML identity only runs (Hub contract §10.1's
--    commented grants, made real for phase 8).
GRANT hub_owner TO "{{HUB_MIGRATE_IDENTITY}}";
GRANT hub_app   TO "{{HUB_APP_IDENTITY}}";
```

`pgaadauth_create_principal` is Azure Database for PostgreSQL's function for Entra principals —
**its exact signature (name, isAdmin, isMfa) is from training knowledge, UNVERIFIED**; task 31's
runbook runs it by hand first.

#### Tests to write first (helm-unittest; and a template test for the SQL)

| Suite / test | Asserts | Mutant it kills |
|---|---|---|
| `broker / rbac is a namespaced Role` | kind `Role`, namespace `panacea-sandboxes`, the four rules exactly | a `ClusterRole` — the broker able to create pods in `kube-system` |
| `broker / no exec, no attach, no portforward` | no rule names `pods/exec`, `pods/attach`, `pods/portforward` | P8-D7 undone by a convenience grant |
| `broker / secrets are get-by-name only` | the secrets rule has no `list` or `watch` | a broker that can enumerate every machine token |
| `broker / images are digests` | `image: x:latest` → render fails (schema) | a mutable broker |
| `broker / one value feeds the runtime class` | `PANACEA_APP_RUNTIME_CLASS` equals `sandbox.runtimeClass` (and the deploy pipeline passes the same value to `platform`) | broker and balloons on different RuntimeClasses |
| `broker / the route has no timeout` | `timeouts.request: 0s` | attach cut at the gateway's default |
| `broker / no workload identity` | the SA has no `azure.workload.identity/client-id` and pods no `use` label | an Azure identity on the one pod that can create pods |
| `hub / env is the contract's deployed column` | every `PANACEA_APP_HUB_*` key rendered is in the Hub contract's §12.2 table; `DB_AUTH=entra`; `sslmode=verify-full`; `TRUST_PROXY=true`; `ENV` is `dev` or `prod` | a Hub started with `ENV=local` (which allows an `http` authority) |
| `hub / migrate runs before serve, as the DDL identity` | the Job is a `pre-install,pre-upgrade` hook, SA `hub-migrate` with the migrate client id, command `hub migrate`, `backoffLimit: 0` | migrations run by the serving (DML) identity |
| `hub / two replicas and a 30 s grace` | `replicas: 2`; `terminationGracePeriodSeconds >= 30`; a PDB `minAvailable: 1` | a rollout that drops every in-flight request |
| `hub / egress reaches Postgres and Entra only` | egress rules: DNS, the PE CIDR on 5432, 443 to world minus RFC 1918 | a Hub that can reach sandboxes |
| `sql / principals and grants are rendered for both identities` | the template renders two `pgaadauth_create_principal` calls and the two grants, DDL to `hub_owner`, DML to `hub_app` | the grants swapped — the serving identity able to alter the schema (ADR-H1 G4) |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §12 (it may have moved after this part was written); every
  Hub variable the chart sets must be in its table.
- [ ] **Step 2:** Both charts' values, schemas and suites; the suites fail.
- [ ] **Step 3:** Templates; the SQL template and its render test (`pipelines/scripts/render-sql.sh`
  with a fixture, asserting the output text); `helm lint` for both.
- [ ] **Step 4:** `deploy/values/{dev,prod}/*.yaml` with every non-id value; ids are left empty
  and filled by `deploy` (task 30) from Terraform outputs. Commit.

#### Acceptance

```bash
helm lint deploy/charts/broker -f deploy/charts/broker/tests/values-test.yaml   # 0 failed
helm lint deploy/charts/hub -f deploy/charts/hub/tests/values-test.yaml         # 0 failed
helm unittest deploy/charts/broker deploy/charts/hub                            # all suites pass
bash pipelines/scripts/render-sql.sh deploy/sql/hub-principals.sql.tpl tests/fixtures/ids.env   # prints the four statements
```

#### What this task does not claim

- That the Hub's standard-library workload-identity token exchange works against Entra-only
  Postgres — the Hub contract itself lists it as "described, not prototyped" (`:2287`).
- That the Gateway's route timeout of `0s` is honoured by the chosen Traefik release (task 13).
