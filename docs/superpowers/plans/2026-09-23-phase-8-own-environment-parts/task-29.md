### Task 29: `deploy/charts/broker` and `deploy/charts/hub` — the two servers, and the SQL that lets the Hub in; `deploy/charts/keycloak` beside them (OD-30)

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
- `pipelines/scripts/render-sql.sh`, `pipelines/scripts/render-sql_test.sh`, `pipelines/scripts/tests/fixtures/workloads-outputs.json` — fills the template's `{{…}}` placeholders from the `workloads` stack's `terraform output -json` file (the `stack-outputs` artefact), by output name, through the one mapping table below; refuses (exit 1, naming it) any placeholder left unfilled, any output missing, and any value that is not a plain identity name or a GUID (new; review I9, plan re-review b03 M-5)
- `deploy/charts/keycloak/{Chart.yaml,values.yaml,values.schema.json}`, `templates/{deployment,service,httproute,pdb,networkpolicy,secretproviderclass,serviceaccount,realm-job}.yaml`, `tests/*_test.yaml` (new; **OD-30**, ruling §9: our own Keycloak beside the two servers)
- `deploy/values/dev/{platform,cert-manager,edge,broker,hub,keycloak}.yaml`, `deploy/values/prod/…` — per-environment values holding **no id** (new)
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
oidc:                        # OD-30: a realm token, validated by oidcauth (task 17)
  issuer: ""                 # PANACEA_APP_OIDC_ISSUER: https://auth.<zone>/realms/panacea-app (foundation's edge_fqdns.auth)
  profile: keycloak          # PANACEA_APP_OIDC_PROFILE
  tenants: ""                # PANACEA_APP_BROKER_TENANTS: enrolled tids (owner input; a tid is an id, so it lives in the pipeline's variables, not values)
  audience: panacea-app-broker        # PANACEA_APP_BROKER_AUDIENCE
  allowedClients: panacea-app-desktop # PANACEA_APP_BROKER_ALLOWED_CLIENTS
  serviceClients: panacea-app-fleet   # PANACEA_APP_BROKER_SERVICE_CLIENTS
sandbox:
  image: ""                  # the sandbox digest
  machineImage: ""           # the machine digest
  runtimeClass: kata-vm-isolation   # one value feeds the broker AND platform.sandboxes.runtimeClass
  sessionCpuMillis: 2000            # one value feeds the broker's defaults, platform.warm.balloons AND platform.sandboxes.limitRange.default* (review I4)
  sessionMemoryMib: 4096
limits:
  maxCpuMillis: 6000         # DERIVED FROM THE SANDBOX SKU (task 17, b05 re-review 2): the largest session that fits beside a machine pod on an empty node; equals platform's sandboxes.limitRange.maxCpu (session_size_is_one_value); P07 rewrites both
  maxMemoryMib: 20480
  maxSandboxesPerUser: 8
  maxMachinesPerUser: 2
machines:
  diskGib: 64
  storageClass: panacea-machine-disk
  affinity: true
  idleSleep: 30m
timeouts:                    # task 17's table; the chart sets them so the cluster's values are read here, not inherited from code defaults
  ready: 120s                # PANACEA_APP_BROKER_READY_TIMEOUT
  unschedulableGrace: 5s     # PANACEA_APP_BROKER_UNSCHEDULABLE_GRACE (review I5); the schema cannot compare durations, the broker's config check refuses one not below `ready`
  shutdownGrace: 20s         # PANACEA_APP_BROKER_SHUTDOWN_GRACE
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
liveness `/healthz`, readiness `/readyz`; `terminationGracePeriodSeconds: 30`. Two variables come
from the downward API, not from values: `PANACEA_APP_BROKER_LEASE_NAMESPACE` from
`metadata.namespace` and `PANACEA_APP_BROKER_POD_NAME` from `metadata.name` (the sleeper's Lease,
task 19, review m11).

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

**A second `Role`, `broker-lease`, in `panacea-broker`** (the broker's own namespace), bound to the
same service account, for the sleeper's Lease (task 19, review m11):

| API group | Resources | resourceNames | Verbs |
|---|---|---|---|
| `coordination.k8s.io` | `leases` | — | `create` |
| `coordination.k8s.io` | `leases` | `broker-sleeper` | `get`, `update` |

`create` cannot be narrowed by name (Kubernetes RBAC does not apply `resourceNames` to `create`),
so it is its own rule; `get` and `update` reach only the one Lease. No `delete`, `list`, `watch` or
`patch`. It sits in `panacea-broker`, not in the sandbox Role, because the Lease is the broker's
coordination and not a sandbox object: the sandbox Role keeps its four rules exactly, and nothing
the broker can write in `panacea-sandboxes` grows. It needs no new grant for the cluster
identity: that identity is RBAC Admin in `panacea-broker` (task 30's `RBAC_ADMIN_NAMESPACES`),
and the RBAC escalation check most likely passes because Admin's data actions include
`escalate` there: under the Azure RBAC webhook, the check sees webhook grants only through the
`escalate` verb, so "Admin covers leases" alone would not pass it (b09 re-review 7; training
knowledge of the built-in role's wildcard data actions, **UNVERIFIED** either way; the broker
release's install in task 31 §7a proves the outcome), and `sandboxes-deployer.json` is unchanged.

**`networkpolicy.yaml`** (in `panacea-broker`, on top of task 11's default-deny): ingress from the
Traefik pods in `panacea-edge` on 8080 (the peer is namespace `panacea-edge` **and** Traefik's pod
labels, the same labels task 13's `traefik-*` policies select; `netpolcover` fails if they
drift); egress to kube-dns, to `panacea-sandboxes` pods labelled
`panacea-app/component` in (`sandbox`, `machine`) on 8080/8081, to the API server (policy
`broker-api-server`, from the `apiServer` block: `mode: entity` renders a `CiliumNetworkPolicy`
with `toEntities: [kube-apiserver]` — **AKS-managed Cilium's support for that entity is
UNVERIFIED** — and `mode: cidr` renders one `ipBlock` per `apiServer.cidrs` entry on 443, with the
schema's `minItems: 1` and the template's `fail` guard of task 13, so an empty list never renders
a peerless rule), and 443 to `0.0.0.0/0` except the RFC 1918 ranges (the realm's JWKS at `auth.<zone>`, reached
through the edge's public IP, OD-30). Switching
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
oidc:                         # OD-30; variable names are the Hub contract's, as hub task 44a folds the ruling in
  issuer: ""                  # PANACEA_APP_OIDC_ISSUER, as the broker's
  profile: keycloak           # PANACEA_APP_OIDC_PROFILE
  tenants: ""                 # "<tid>=<org display name>" (unchanged: tid still arrives as a claim, mapper P2)
  audience: panacea-app-hub   # PANACEA_APP_HUB_AUDIENCE (was the Hub API's client id)
  allowedClients: panacea-app-desktop
  serviceClients: panacea-app-fleet,panacea-app-gateway   # ruling §6
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
`PANACEA_APP_OIDC_ISSUER`, `PANACEA_APP_OIDC_PROFILE` (OD-30),
`PANACEA_APP_HUB_TENANTS`, `PANACEA_APP_HUB_AUDIENCE`, `PANACEA_APP_HUB_ALLOWED_CLIENTS`,
`PANACEA_APP_HUB_SERVICE_CLIENTS` (OD-30, ruling §6; the contract gains it through hub task 44a),
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
takes), and 443 to `0.0.0.0/0` except RFC 1918 (Entra for its database token; the realm's JWKS at
`auth.<zone>`, OD-30). The migrate Job's pod is covered by the
hook policy above, not by this one — `netpolcover`'s `hook_pods_are_allowed_before_they_run`
(task 13) fails if the Job's only allow is a regular template. **No CORS anywhere** (HA-6).

#### Contract — `deploy/charts/keycloak/values.yaml` (complete; OD-30, ruling §9)

Our Keycloak, modelled on Panacea's as a pattern only (OD-13), **minus Panacea's outage class**
(ADR-H1 §6.1): two replicas with Keycloak's persistent user sessions in Postgres, never one replica on
`--cache=local`; the image pinned by digest, never a chart default. It serves realm `panacea-app`
(task 09's file) at `https://auth.<zone>/realms/panacea-app`.

```yaml
image: ""                     # quay.io/keycloak/keycloak@sha256:… — the current Keycloak 26 release, by digest
configCliImage: ""            # adorsys/keycloak-config-cli@sha256:… (Apache-2.0), the build for that Keycloak major
replicas: 2                   # schema minimum 2
namespace: panacea-identity   # task 11
host: ""                      # auth.<zone> — foundation's edge_fqdns.auth (task 07)
gateway:
  name: panacea
  namespace: panacea-edge
database:
  host: ""                    # data stack: postgres_fqdn
  name: keycloak              # task 08
  user: ""                    # Keycloak's identity name; the auth method is task 08's U-9
realm: ""                     # never in a values file: deploy passes --set-file realm=deploy/keycloak/realm/panacea-app.json
keyVault:
  name: ""                    # foundation: key_vault_name
  secrets:                    # stored by the owner (task 09); synced by the Secrets Store CSI driver
    entraIdp: keycloak-entra-idp-secret
    fleet: keycloak-client-panacea-app-fleet
    gateway: keycloak-client-panacea-app-gateway
    bootstrapAdmin: keycloak-bootstrap-admin
serviceAccount:
  clientId: ""                # Keycloak's user-assigned identity (Key Vault; Postgres per U-9)
resources:
  cpu: "1"
  memory: 1536Mi
```

The Entra tenant id and the existing registration's client id reach the realm Job as
`PANACEA_APP_ENTRA_TENANT_ID` and `PANACEA_APP_ENTRA_IDP_CLIENT_ID` from the deploy pipeline's variables
(task 30), so no values file holds an id. **Keycloak's user-assigned identity is a `workloads` output
that task 10 does not yet make** (ruling §9's table does not name task 10): flagged for task 10's owner.

**What the templates render:**
- `deployment.yaml` — `Deployment/keycloak`, `replicas`, `start` with `KC_DB=postgres`,
  `KC_DB_URL=jdbc:postgresql://<host>:5432/keycloak?sslmode=verify-full`, `KC_HOSTNAME=https://<host>`,
  `KC_HTTP_ENABLED=true` (TLS ends at the edge), `KC_PROXY_HEADERS=xforwarded`, `KC_HEALTH_ENABLED=true`,
  `KC_CACHE=ispn` with the Kubernetes stack over `Service/keycloak-discovery`; the bootstrap admin from
  the CSI-synced Secret; the restricted security context (non-root, no escalation, all capabilities
  dropped, `RuntimeDefault`); probes on the management port 9000.
- `service.yaml` — `Service/keycloak` on 8080, and headless `Service/keycloak-discovery` for clustering.
- `httproute.yaml` — `HTTPRoute/keycloak`, `hostnames: [host]`, **only** `PathPrefix` `/realms/` and
  `/resources/`: the admin console and admin API are never routed beyond the cluster.
- `pdb.yaml` — `minAvailable: 1`. `serviceaccount.yaml` — workload identity `clientId`.
- `secretproviderclass.yaml` — the four Key Vault secrets, synced to `Secret/keycloak-secrets`.
- `networkpolicy.yaml` — ingress from Traefik's pods on 8080, from the realm Job's pod on 8080, and
  between Keycloak's own pods on the clustering port; egress to kube-dns, the private-endpoint CIDR on
  5432, 443 to `0.0.0.0/0` except RFC 1918 (Entra, `login.microsoftonline.com`), and its own pods.
- `realm-job.yaml` — `Job/keycloak-realm`, a `post-install,post-upgrade` hook (`before-hook-creation`),
  so it runs on **every deploy**: keycloak-config-cli against `http://keycloak.panacea-identity.svc:8080`
  with `IMPORT_FILES_LOCATIONS` = the realm mounted from a ConfigMap of `realm`,
  `IMPORT_VARSUBSTITUTION_ENABLED=true`, `IMPORT_MANAGED_CLIENT_SCOPE=no-delete` (task 09 says why),
  the bootstrap admin, and the secrets and ids task 09's `$(env:…)` names. Never `--import-realm`,
  which skips a realm that exists ("exists" is not "correct").

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
--    idempotent. Until the Hub answers, task 31 §7 runs it (one transaction, ON_ERROR_STOP)
--    only when neither role exists, and re-runs its GRANT CREATE alone when both do.
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
  -- 3. Membership: the DDL identity owns, the DML identity only runs (Hub contract §10.1's
  --    commented grants, made real for phase 8). INSIDE the block, after both checks: a
  --    RAISE above aborts the block, so no grant reaches a role bound to another object id
  --    even if psql were run without ON_ERROR_STOP (plan re-review b03 I-1). GRANT of a role
  --    already held is a notice, not an error, so a re-run is safe.
  EXECUTE format('GRANT hub_owner TO %I', '{{HUB_MIGRATE_IDENTITY}}');
  EXECUTE format('GRANT hub_app TO %I', '{{HUB_APP_IDENTITY}}');
END
$$;
-- Nothing follows the block. task 31 §7 runs this file, and roles.sql, with
-- `psql -v ON_ERROR_STOP=1 --single-transaction -f <file>` and reads the exit code.
```

`pgaadauth_create_principal_with_oid` is Azure Database for PostgreSQL's function for an Entra
principal bound to an object id, and `pgaadauth_list_principals` lists the principals with their
object ids. **Both signatures — `(roleName, objectId, objectType, isAdmin, isMfa)` with
`objectType` `'service'` for a managed identity, and a result with `rolname` and `objectid`
columns — are from training knowledge, UNVERIFIED**; task 31 §7 runs `pgaadauth_list_principals(false)`
by hand first and records its real columns. The object ids are the `workloads` outputs
`hub_principal_id` and `hub_migrate_principal_id`: a user-assigned identity's `principal_id` is its
service principal's object id, the `oid` its tokens carry (task 10).

**The mapping, written once** (plan re-review b03 M-5). `render-sql.sh <template> <workloads.json>`
holds this table and nothing else decides it; a human never types the ids:

| Placeholder | `workloads` output (task 10) | Accepted value |
|---|---|---|
| `HUB_APP_IDENTITY` | `hub_identity_name` | `^[A-Za-z0-9_-]{3,128}$` |
| `HUB_APP_OBJECT_ID` | `hub_principal_id` | a lower-case GUID |
| `HUB_MIGRATE_IDENTITY` | `hub_migrate_identity_name` | `^[A-Za-z0-9_-]{3,128}$` |
| `HUB_MIGRATE_OBJECT_ID` | `hub_migrate_principal_id` | a lower-case GUID |

It reads each output's `value` with `python3 -c` and the standard `json` module (no `jq`, which the
operator's workstation may lack; `python3` is a §7 prerequisite in task 31). A value outside its
pattern exits 1 naming the key, so no quote or `$$` can reach the SQL text. The four output names
are task 10's (`parts/task-10.md:145-155`); if task 10 renames one, this table and the fixture move
with it.

#### Tests to write first (helm-unittest; and a template test for the SQL)

| Suite / test | Asserts | Mutant it kills |
|---|---|---|
| `broker / rbac is a namespaced Role` | kind `Role`, namespace `panacea-sandboxes`, the four rules exactly | a `ClusterRole` — the broker able to create pods in `kube-system` |
| `broker / the lease role is narrow and in the broker's namespace` | `Role/broker-lease` in `panacea-broker`; exactly the two rules above (`create` alone; `get`, `update` on `resourceNames: [broker-sleeper]`); no `delete`, `list`, `watch`, `patch`; no rule on `leases` in the `panacea-sandboxes` Role; the Deployment sets `PANACEA_APP_BROKER_LEASE_NAMESPACE` and `_POD_NAME` from `fieldRef` `metadata.namespace` / `metadata.name` (review m11) | a `leases` grant on every name, or one in `panacea-sandboxes`; a sleeper with no Lease identity, which runs on both replicas |
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
| `hub / egress reaches Postgres and Entra only` | egress rules: DNS, the PE CIDR on 5432, 443 to world minus RFC 1918 (Entra and the realm) | a Hub that can reach sandboxes |
| `broker / the realm settings are OD-30's` | `PANACEA_APP_OIDC_PROFILE=keycloak`, `PANACEA_APP_BROKER_AUDIENCE=panacea-app-broker`, `_ALLOWED_CLIENTS=panacea-app-desktop`, `_SERVICE_CLIENTS=panacea-app-fleet`; no `PANACEA_APP_BROKER_ENTRA_*` key | a broker still configured for Entra, refusing every realm token |
| `keycloak / two replicas and a PDB` | `replicas: 2`; `replicas: 1` fails the schema; PDB `minAvailable: 1` | Panacea's one-replica outage class (ADR-H1 §6.1) |
| `keycloak / the images are digests` | `image: x:26` or `configCliImage: x:latest` → render fails | a moving Keycloak under a pinned realm file |
| `keycloak / sessions live in Postgres` | `KC_DB=postgres`, the URL names database `keycloak` with `sslmode=verify-full`; no `KC_CACHE=local` | offline tokens lost on a pod move, so launches stop being silent |
| `keycloak / the admin console is not routed` | the HTTPRoute's matches are exactly `PathPrefix` `/realms/` and `/resources/`; none is `/` or `/admin` | the admin API on the internet |
| `keycloak / the realm converges on every deploy` | the Job is a `post-install,post-upgrade` hook with `before-hook-creation`; `IMPORT_VARSUBSTITUTION_ENABLED=true`, `IMPORT_MANAGED_CLIENT_SCOPE=no-delete`; the ConfigMap holds `realm`; no container arg is `--import-realm` | an import-once realm whose edits never land |
| `keycloak / no secret is a literal` | every secret-bearing env var is a `secretKeyRef` to `keycloak-secrets`; `realm` empty fails the render | a client secret in a values file |
| `keycloak / egress reaches Postgres and Entra only` | DNS, the PE CIDR on 5432, 443 to world minus RFC 1918, its own pods | Keycloak reaching sandboxes |
| `sql / principals and grants are rendered for both identities` | the template renders two `pgaadauth_create_principal_with_oid` calls and the two grants, DDL to `hub_owner`, DML to `hub_app` | the grants swapped — the serving identity able to alter the schema (ADR-H1 G4) |
| `sql / principals are bound by object id` | each `pgaadauth_create_principal_with_oid` call carries its own identity's object id from the fixture (the app's id with the app's name, the migrate id with the migrate name); the text contains no name-only `pgaadauth_create_principal(`; each existing-role branch compares against the same id and `RAISE`s on a mismatch | a principal resolved by display name, which binds whichever identity in the tenant shares it; the two ids swapped; a re-run that accepts a role bound to someone else (review I9) |
| `sql / no grant runs outside the checked block` | in the rendered text, both `GRANT` statements sit between the DO block's `BEGIN` and its `END`, after both `RAISE EXCEPTION` lines; nothing but comments follows `$$;` | a top-level `GRANT` after the block, which psql's default (report the error, carry on, exit 0) runs after the oid check fails — a foreign identity handed `hub_owner` (plan re-review b03 I-1) |
| `sql / an unfilled placeholder fails the render` | a fixture missing the output `hub_migrate_principal_id` → `render-sql.sh` exits 1 naming `HUB_MIGRATE_OBJECT_ID` and the output, and prints no SQL; a template with an unknown `{{HUB_OTHER}}` → exit 1 naming it | a literal `{{…}}` sent to `psql`, which creates a role named after the placeholder |
| `sql / each placeholder reads its own output` | a fixture whose four outputs hold four distinct values → each lands in exactly its mapped placeholder (the app name beside the app id in the first call, the migrate name beside the migrate id in the second) | the mapping table swapped for one pair, which binds the serving name to the DDL identity's oid (plan re-review b03 M-5) |
| `sql / a value that is not a name or a GUID is refused` | `hub_identity_name` = `id-hub"; DROP ROLE hub_owner; --` → exit 1 naming `HUB_APP_IDENTITY`; `hub_principal_id` = `not-a-guid` → exit 1 naming `HUB_APP_OBJECT_ID`; no SQL printed | an output value that breaks out of the SQL literal or the `%I` quoting |

#### Steps

- [ ] **Step 1:** Re-read Hub contract §12 (it may have moved after this part was written); every
  Hub variable the chart sets must be in its table.
- [ ] **Step 2:** The three charts' values, schemas and suites (`keycloak` added by OD-30); the suites fail.
- [ ] **Step 3:** Templates; the SQL template and its render test (`pipelines/scripts/render-sql.sh`
  with a fixture, asserting the output text); `helm lint` for both.
- [ ] **Step 4:** `deploy/values/{dev,prod}/*.yaml` with every non-id value; ids are left empty
  and filled by `deploy` (task 30) from Terraform outputs.
- [ ] **Step 5:** Add `broker`, `hub` and `keycloak` to `netpolcover`'s chart list and delete its
  `testdata/backends.yaml`; it must pass with the real charts, and `traefik_reaches_every_routed_backend`
  now checks both `HTTPRoute`s. Add the case `session_size_is_one_value` (review I4; b05
  re-review 2, 4, 9). It compares **rendered objects, not keys**: for each env, it renders
  `platform` and `broker` with `-f deploy/values/<env>/<chart>.yaml`, exactly as task 30
  deploys them, so a key the env file leaves to the chart's `values.yaml` is compared at its
  effective value. A compared object or field missing from the render fails the case; it never
  skips. For each of dev and prod:
  - **The session size** (the one size every cloud session gets, since remotebe sends none, task
    22): the broker Deployment's `PANACEA_APP_BROKER_SESSION_CPU_MILLIS` with an `m` suffix equals
    the balloon container's CPU request and limit and `LimitRange/sandboxes`'s `default.cpu` and
    `defaultRequest.cpu`; the same for memory with `Mi`.
  - **The ceiling** (derived from the sandbox SKU, task 17): `PANACEA_APP_BROKER_MAX_CPU_MILLIS`
    with `m` equals `LimitRange/sandboxes`'s `max.cpu`, and `_MAX_MEMORY_MIB` with `Mi` equals
    `max.memory`. Quantities are compared parsed (`6` and `6000m` are equal), not as strings.
  - `sandboxes.runtimeClass` as rendered on the balloon equals the broker's
    `PANACEA_APP_RUNTIME_CLASS` (task 17's name, shared with `cmd/api`).

  The two charts read separate files, so "one value" is this test, not a shared key. Mutation
  checks, each verified applied and red:
  - set dev's `platform.yaml` `sandboxes.limitRange.defaultCpu` to `1000m` →
    `session_size_is_one_value`;
  - set dev's `broker.yaml` `limits.maxCpuMillis` to `8000` → `session_size_is_one_value`;
  - delete `warm.balloons.cpu` from dev's `platform.yaml` and set the chart default to `1000m` →
    `session_size_is_one_value` (the effective value is compared, not the absent key);
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
helm lint deploy/charts/keycloak -f deploy/charts/keycloak/tests/values-test.yaml   # 0 failed
helm unittest deploy/charts/broker deploy/charts/hub deploy/charts/keycloak     # all suites pass
go test ./deploy/tests/netpolcover/... -count=1                                  # ok, six charts rendered
bash pipelines/scripts/render-sql.sh deploy/sql/hub-principals.sql.tpl pipelines/scripts/tests/fixtures/workloads-outputs.json   # prints the DO block with both grants inside it, no {{ left
bash pipelines/scripts/render-sql_test.sh                                        # render-sql_test: 6 of 6 passed (the six `sql /` rows)
```

#### What this task does not claim

- That the Hub's standard-library workload-identity token exchange works against Entra-only
  Postgres — the Hub contract itself lists it as "described, not prototyped" (`:2287`).
- That the Gateway's route timeout of `0s` is honoured by the chosen Traefik release (task 13).
- That Keycloak 26 clusters on AKS through the headless Service, that its image runs under
  `restricted`, and that keycloak-config-cli's settings named here exist in the pinned build: all
  **from training knowledge, UNVERIFIED** until task 31 installs it. How Keycloak authenticates to
  the Entra-only server is task 08's U-9, unsettled.
