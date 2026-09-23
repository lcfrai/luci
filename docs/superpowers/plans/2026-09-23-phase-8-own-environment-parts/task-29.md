### Task 29: `deploy/charts/broker` and `deploy/charts/hub` — the two servers, and the SQL that lets the Hub in

Two charts, one per server, each owning its Deployment, service account, HTTPRoute, disruption
budget and network policy; plus the Hub's migrate Job and the one SQL file that a **human**
Postgres administrator runs once per environment (task 31 §7; no pipeline identity is a database
administrator, review I8). The Hub's side is **the Hub contract's** (sibling `feat/hub`,
`2026-09-23-hub-contract.md` §12.2 `:2140-2191` and §12.3 `:2193-2208`, which answer HA-4 and
HA-5 "field by field"); nothing here sets a Hub variable that table does not list (seam S8-H.1).

**Files:**
- `deploy/charts/broker/{Chart.yaml,values.yaml,values.schema.json}`, `templates/{deployment,serviceaccount,rbac,service,httproute,pdb,networkpolicy}.yaml`, `tests/*_test.yaml` (new)
- `deploy/charts/hub/{Chart.yaml,values.yaml,values.schema.json}`, `templates/{deployment,serviceaccount,migrate-job,service,httproute,pdb,networkpolicy}.yaml`, `tests/*_test.yaml` (new)
- `deploy/sql/hub-principals.sql.tpl` — the Entra principals and grants for the Hub's two identities (new)
- `pipelines/scripts/render-sql.sh`, `render-sql_test.sh`, `tests/fixtures/ids.env` — fills the template's `{{…}}` placeholders from a `KEY=value` file and refuses (exit 1, naming it) any placeholder left unfilled (new; review I9)
- `deploy/values/dev/{platform,cert-manager,edge,broker,hub}.yaml`, `deploy/values/prod/…` — per-environment values holding **no id** (new)
- `deploy/tests/netpolcover/netpolcover_test.go` — add `broker` and `hub` to the rendered charts; delete `testdata/backends.yaml`, which stood in for them (modify, task 13)

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
apiServer:                   # the same block, same keys and schema rule as task 13's cert-manager and edge charts (review I3)
  mode: entity               # entity: CiliumNetworkPolicy toEntities kube-apiserver; cidr: one ipBlock per cidrs entry on 443
  cidrs: []                  # cidr mode only; schema minItems 1 then; passed by pipelines/scripts/api-server-cidrs.sh (task 30)
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
Traefik pods in `panacea-edge` on 8080 (the peer is namespace `panacea-edge` **and** Traefik's pod
labels, the same labels task 13's `traefik-*` policies select; `netpolcover` fails if they
drift); egress to kube-dns, to `panacea-sandboxes` pods labelled
`panacea-app/component` in (`sandbox`, `machine`) on 8080/8081, to the API server (policy
`broker-api-server`, from the `apiServer` block: `mode: entity` renders a `CiliumNetworkPolicy`
with `toEntities: [kube-apiserver]` — **AKS-managed Cilium's support for that entity is
UNVERIFIED** — and `mode: cidr` renders one `ipBlock` per `apiServer.cidrs` entry on 443, with the
schema's `minItems: 1` and the template's `fail` guard of task 13, so an empty list never renders
a peerless rule), and 443 to `0.0.0.0/0` except the RFC 1918 ranges (Entra's JWKS). Switching
the broker after P20 is a one-value change in `deploy/values/<env>/broker.yaml`, as for the other
two charts; no chart change.

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

**`migrate-job.yaml`** — three Helm hook resources, all `helm.sh/hook: pre-install,pre-upgrade`
and `helm.sh/hook-delete-policy: before-hook-creation`, created in `helm.sh/hook-weight` order.
Helm runs `pre-install` hooks **before any of the release's regular resources exist**, and
platform's `default-deny` (task 11) is already in `panacea-hub`. So everything the Job's pod needs
to start and to reach the database is a hook of the same phase with a lower weight, never a regular
template:

| Weight | Resource | Why it must be a hook |
|---|---|---|
| `-10` | `ServiceAccount/hub-migrate`, annotation `azure.workload.identity/client-id: <migrate.serviceAccount.clientId>` | a regular SA does not exist yet on the first install; the Job's pod cannot be created and Helm waits out the hook timeout |
| `-5` | `NetworkPolicy/hub-migrate-egress`, selecting `app.kubernetes.io/component: migrate` only, `policyTypes: [Egress]`: kube-dns (UDP and TCP 53), `network.privateEndpointsCidr` on 5432, 443 to `0.0.0.0/0` except RFC 1918 and `169.254.0.0/16` (Entra) | a regular policy does not exist yet on the first install; the Job cannot resolve or reach Postgres, fails once (`backoffLimit: 0`) and `--atomic` rolls the release back. Upgrades would hide this, because the previous release's policy is still there |
| `0` | `Job/hub-migrate`, pod label `app.kubernetes.io/component: migrate` and `azure.workload.identity/use: "true"`, service account `hub-migrate`, command `["/hub", "migrate"]`, env `PANACEA_APP_HUB_MIGRATE_DATABASE_URL=postgres://<migrateUser>@<host>:5432/hub?sslmode=verify-full` and `PANACEA_APP_HUB_DB_AUTH=entra`, `backoffLimit: 0` | a failed migration stops the upgrade; `--atomic` rolls back |

Hook resources are not owned by the release: `helm uninstall hub` leaves the SA and the policy
behind. That is harmless: `before-hook-creation` replaces both on the next install, and no task in
this phase uninstalls a release (anyone who does deletes the two by name). **Helm's hook order is
from training knowledge and was not rendered, UNVERIFIED**; the `hub / migrate hooks exist before
the Job` test pins the weights, and task 31's first install observes it.

**`networkpolicy.yaml`** (in `panacea-hub`, a regular template; it selects the **serving** pods
only, `app.kubernetes.io/component: server`): ingress from Traefik on 8080 (namespace
`panacea-edge` **and** Traefik's pod labels, as the broker's); egress to kube-dns, to
the private-endpoint subnet on 5432 (`network.privateEndpointsCidr`, the same value task 11
takes), and 443 to `0.0.0.0/0` except RFC 1918 (Entra). The migrate Job's pod is covered by the
hook policy above, not by this one — `netpolcover`'s `hook_pods_are_allowed_before_they_run`
(task 13) fails if the Job's only allow is a regular template. **No CORS anywhere** (HA-6).

#### Contract — `deploy/sql/hub-principals.sql.tpl` (complete; rendered from the `workloads` outputs and run by a human, task 31 §7)

```sql
-- Run by a HUMAN member of the Postgres admin group (task 08), with their own Entra token,
-- connected to the `postgres` database. No pipeline identity is in that group (review I8).
-- Idempotent, and a re-run checks what exists rather than trusting it (review I9).
-- Placeholders are filled from the workloads stack's outputs; nothing here is a secret.
--
-- 1. The Hub's own roles.sql, once per environment (Hub contract §10.1): the same person runs
--    control-plane/internal/hub/migrate/roles.sql first, in database `hub`. The contract's file
--    is a bare `CREATE ROLE`, which errors on a second run; ask HC-1 (task 33) makes it
--    idempotent. Until the Hub answers, task 31 §7 runs it only when neither role exists.
-- 2. The two Entra principals, bound by OBJECT ID, never looked up by name: a user-assigned
--    identity's name is not unique in a tenant (review I9). The role name is still the
--    identity's name, because it is the user in the Hub's connection string (appUser,
--    migrateUser). A role that already exists must be bound to the expected object id, or the
--    block stops. A DO block, because a function in a FROM clause runs before any WHERE could
--    skip it.
DO $$
DECLARE
  bound text;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{{HUB_APP_IDENTITY}}') THEN
    PERFORM pgaadauth_create_principal_with_oid('{{HUB_APP_IDENTITY}}', '{{HUB_APP_OBJECT_ID}}', 'service', false, false);
  ELSE
    SELECT objectid INTO bound FROM pgaadauth_list_principals(false) WHERE rolname = '{{HUB_APP_IDENTITY}}';
    IF bound IS DISTINCT FROM '{{HUB_APP_OBJECT_ID}}' THEN
      RAISE EXCEPTION 'role % exists bound to object id %, expected %', '{{HUB_APP_IDENTITY}}', bound, '{{HUB_APP_OBJECT_ID}}';
    END IF;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{{HUB_MIGRATE_IDENTITY}}') THEN
    PERFORM pgaadauth_create_principal_with_oid('{{HUB_MIGRATE_IDENTITY}}', '{{HUB_MIGRATE_OBJECT_ID}}', 'service', false, false);
  ELSE
    SELECT objectid INTO bound FROM pgaadauth_list_principals(false) WHERE rolname = '{{HUB_MIGRATE_IDENTITY}}';
    IF bound IS DISTINCT FROM '{{HUB_MIGRATE_OBJECT_ID}}' THEN
      RAISE EXCEPTION 'role % exists bound to object id %, expected %', '{{HUB_MIGRATE_IDENTITY}}', bound, '{{HUB_MIGRATE_OBJECT_ID}}';
    END IF;
  END IF;
END
$$;
-- 3. Membership: the DDL identity owns, the DML identity only runs (Hub contract §10.1's
--    commented grants, made real for phase 8).
GRANT hub_owner TO "{{HUB_MIGRATE_IDENTITY}}";
GRANT hub_app   TO "{{HUB_APP_IDENTITY}}";
```

`pgaadauth_create_principal_with_oid` is Azure Database for PostgreSQL's function for an Entra
principal bound to an object id, and `pgaadauth_list_principals` lists the principals with their
object ids. **Both signatures — `(roleName, objectId, objectType, isAdmin, isMfa)` with
`objectType` `'service'` for a managed identity, and a result with `rolname` and `objectid`
columns — are from training knowledge, UNVERIFIED**; task 31 §7 runs `pgaadauth_list_principals(false)`
by hand first and records its real columns. The object ids are the `workloads` outputs
`hub_principal_id` and `hub_migrate_principal_id`: a user-assigned identity's `principal_id` is its
service principal's object id, the `oid` its tokens carry (task 10).

#### Tests to write first (helm-unittest; and a template test for the SQL)

| Suite / test | Asserts | Mutant it kills |
|---|---|---|
| `broker / rbac is a namespaced Role` | kind `Role`, namespace `panacea-sandboxes`, the four rules exactly | a `ClusterRole` — the broker able to create pods in `kube-system` |
| `broker / no exec, no attach, no portforward` | no rule names `pods/exec`, `pods/attach`, `pods/portforward` | P8-D7 undone by a convenience grant |
| `broker / secrets are get-by-name only` | the secrets rule has no `list` or `watch` | a broker that can enumerate every machine token |
| `broker / images are digests` | `image: x:latest` → render fails (schema) | a mutable broker |
| `broker / one value feeds the runtime class` | `PANACEA_APP_RUNTIME_CLASS` equals `sandbox.runtimeClass` (and the deploy pipeline passes the same value to `platform`) | broker and balloons on different RuntimeClasses |
| `broker / the route has no timeout` | `timeouts.request: 0s` | attach cut at the gateway's default |
| `broker / the API server is reachable in both modes` | `broker-api-server` selects the broker; default renders `toEntities: [kube-apiserver]`; `apiServer.mode: cidr` with two `cidrs` renders exactly those two `ipBlock`s on 443; `mode: cidr` with `cidrs: []` → render fails with the schema message | a broker that cannot create pods (sealed), or `cidr` mode with an empty list rendered as 443 to anywhere (review I3) |
| `broker / no workload identity` | the SA has no `azure.workload.identity/client-id` and pods no `use` label | an Azure identity on the one pod that can create pods |
| `hub / env is the contract's deployed column` | every `PANACEA_APP_HUB_*` key rendered is in the Hub contract's §12.2 table; `DB_AUTH=entra`; `sslmode=verify-full`; `TRUST_PROXY=true`; `ENV` is `dev` or `prod` | a Hub started with `ENV=local` (which allows an `http` authority) |
| `hub / migrate runs before serve, as the DDL identity` | the Job is a `pre-install,pre-upgrade` hook, SA `hub-migrate` with the migrate client id, command `hub migrate`, `backoffLimit: 0` | migrations run by the serving (DML) identity |
| `hub / migrate hooks exist before the Job` | `ServiceAccount/hub-migrate` and `NetworkPolicy/hub-migrate-egress` carry `helm.sh/hook: pre-install,pre-upgrade`, `before-hook-creation`, and a `hook-weight` lower than the Job's; the policy selects `component: migrate` and allows DNS, the PE CIDR on 5432 and 443 to world minus RFC 1918 | the SA or the policy as a regular template — the first install sealed by default-deny and rolled back, while upgrades pass |
| `hub / two replicas and a 30 s grace` | `replicas: 2`; `terminationGracePeriodSeconds >= 30`; a PDB `minAvailable: 1` | a rollout that drops every in-flight request |
| `hub / egress reaches Postgres and Entra only` | egress rules: DNS, the PE CIDR on 5432, 443 to world minus RFC 1918 | a Hub that can reach sandboxes |
| `sql / principals and grants are rendered for both identities` | the template renders two `pgaadauth_create_principal_with_oid` calls and the two grants, DDL to `hub_owner`, DML to `hub_app` | the grants swapped — the serving identity able to alter the schema (ADR-H1 G4) |
| `sql / principals are bound by object id` | each `pgaadauth_create_principal_with_oid` call carries its own identity's object id from the fixture (the app's id with the app's name, the migrate id with the migrate name); the text contains no name-only `pgaadauth_create_principal(`; each existing-role branch compares against the same id and `RAISE`s on a mismatch | a principal resolved by display name, which binds whichever identity in the tenant shares it; the two ids swapped; a re-run that accepts a role bound to someone else (review I9) |
| `sql / an unfilled placeholder fails the render` | a fixture missing `HUB_MIGRATE_OBJECT_ID` → `render-sql.sh` exits 1 naming the placeholder, and prints no SQL | a literal `{{…}}` sent to `psql`, which creates a role named after the placeholder |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §12 (it may have moved after this part was written); every
  Hub variable the chart sets must be in its table.
- [ ] **Step 2:** Both charts' values, schemas and suites; the suites fail.
- [ ] **Step 3:** Templates; the SQL template and its render test (`pipelines/scripts/render-sql.sh`
  with a fixture, asserting the output text); `helm lint` for both.
- [ ] **Step 4:** `deploy/values/{dev,prod}/*.yaml` with every non-id value; ids are left empty
  and filled by `deploy` (task 30) from Terraform outputs.
- [ ] **Step 5:** Add `broker` and `hub` to `netpolcover`'s chart list and delete its
  `testdata/backends.yaml`; it must pass with the real charts, and `traefik_reaches_every_routed_backend`
  now checks both `HTTPRoute`s. Mutation checks, each verified applied and red:
  - change the broker policy's Traefik peer label → `traefik_reaches_every_routed_backend`;
  - move `hub-migrate-egress` from a hook into `networkpolicy.yaml` as a regular template →
    `hook_pods_are_allowed_before_they_run` (task 13) and `hub / migrate hooks exist before the Job`;
  - drop the `helm.sh/hook` annotations from `ServiceAccount/hub-migrate` →
    `hook_pods_are_allowed_before_they_run`;
  - delete `broker-api-server` → `broker / the API server is reachable in both modes`
    (`netpolcover`'s `every_denied_workload_has_an_allow` stays green, because the broker keeps
    other egress allows; record that it does — the per-chart test is the guard, review I2).

  Commit.

#### Acceptance

```bash
helm lint deploy/charts/broker -f deploy/charts/broker/tests/values-test.yaml   # 0 failed
helm lint deploy/charts/hub -f deploy/charts/hub/tests/values-test.yaml         # 0 failed
helm unittest deploy/charts/broker deploy/charts/hub                            # all suites pass
go test ./deploy/tests/netpolcover/... -count=1                                  # ok, five charts rendered
bash pipelines/scripts/render-sql.sh deploy/sql/hub-principals.sql.tpl tests/fixtures/ids.env   # prints the DO block and the two grants, no {{ left
bash pipelines/scripts/render-sql_test.sh                                        # render-sql_test: 3 of 3 passed
```

#### What this task does not claim

- That the Hub's standard-library workload-identity token exchange works against Entra-only
  Postgres — the Hub contract itself lists it as "described, not prototyped" (`:2287`).
- That the Gateway's route timeout of `0s` is honoured by the chosen Traefik release (task 13).
