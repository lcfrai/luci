### Task 32: Phase 8 delta — Fleet's chart, the private gateway path, the Orchestrator's egress

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 12 (`fleetd`'s variables), P8-11 (the
platform chart), P8-29 (the broker and Hub charts, the pattern this chart copies), P6-19 (the
gateway chart and its NetworkPolicies). **Seams:** P8-D12 (sandbox egress), P6-D1 (the gateway is
never exposed).

**Built after the sync-merge of `feat/env` and `feat/phase-6-gateway`.** None of the files this
task extends exists yet: `deploy/` is absent on `feat/env` (read 2026-09-25), and phase 6's chart is
a plan row. Every sibling path below cites the sibling **plan** and is **UNVERIFIED until merge**:
the platform chart is `.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-11.md`
(values at `:36-83`, the sandbox policies at `:147-164`), the broker and Hub charts are that
directory's `task-29.md` (the Hub's values at `:121-150`, its environment and workload identity at
`:152-164`, its migrate hooks at `:166-183`, its policy at `:185-191`), the identity module and the
workloads stack are its `task-10.md` (`:91-105`, `:107-158`), and the gateway chart is
`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:262`.

Fleet is the only new server team mode adds to the cluster, and it runs the way the Hub runs: two
replicas, a PDB, a user-assigned identity by workload identity, Entra-only Postgres, no secret in
an environment variable, default-deny with a named allow per direction, Australia East only
(§2.5's invariant). The chart also carries the two network facts team mode changes in phase 8:
a sandbox may now reach one service, the gateway's front, and nothing else in the cluster
(P8-3, `2026-09-25-phase-6-gateway.md:308`); and the `orchestrator` class reaches **only** that
service, with no internet at all.

Patterns only from Panacea (OD-13); no Panacea file is copied. This task applies nothing to a
cluster: task 33 installs it by the owner's runbook, through phase 8's `deploy` pipeline (P8-D3).

**Decisions made here.**

**(a) The Orchestrator needs no egress to Fleet.** The skeleton's scope says the `orchestrator`
class egresses "to the gateway and Fleet's control route only". The control stream is **inbound**:
Fleet dials the broker, which relays to `sandboxd`'s `/control` on port 8080 (task 17's `control`
relay kind, `task-17.md:25-27`; task 19's dial to `{BROKER_URL}/v1/sandboxes/{id}/control`,
`task-19.md:494`), and P8-D12 already admits the broker, and only the broker, on 8080. So the
Orchestrator's egress is kube-dns and the gatekeeper's port, and nothing else. **Raised as gap
G32-1** against the scope's wording; the property it wanted (Fleet's control path works, and
nothing else does) holds.

**(b) The Orchestrator has no internet.** Its tools are `Bash(panacea *)` and `Read` only
(`05-design.md:428`), its inference goes to the gateway, and its control comes in from the broker.
A pod with no route out cannot post a teammate's words anywhere, whatever it is persuaded of. The
platform chart's `egress-internet` selects every pod (`task-11.md:155-159`), and NetworkPolicy allows
are a union, so an extra policy cannot subtract from it: `egress-internet`'s `podSelector` becomes
`panacea-app/class NotIn [orchestrator]`. Pods with no class label (workers today, machines,
balloons) still match. Task 58 adds `mirror` to the same list. **Requires of task 15** that every
sandbox pod carry `panacea-app/class: <spec.Class>` (task 15's part does not yet name a label;
gap G32-2).

**(c) The gateway allow names pods, not an address.** `sandbox-egress-gateway` allows egress from
every pod in `panacea-sandboxes` to pods `app.kubernetes.io/name: gatekeeper` in namespace
`panacea-gateway` on TCP 4000 (phase 6's port, `2026-09-25-phase-6-gateway.md:262`). Cilium
matches in-cluster pods by identity (`task-11.md:161-164`, UNVERIFIED there too), so the rule
admits the gatekeeper and nothing else on the service CIDR; keysmith (4100) and LiteLLM (4001) stay
unreachable from a sandbox. The label is phase 6's to confirm at merge (**UNVERIFIED**).

**(d) Fleet has its own namespace.** `panacea-fleet`, created by the platform chart with the same
Pod Security labels and `default-deny` as the other four (`task-11.md:92-99`, `:137-145`), and
`panacea-app/edge-routes: "true"`, because the desktop reaches Fleet through the Gateway exactly as
it reaches the Hub. A namespace of its own gives Fleet's identity a subject nobody else's pod can
claim (`system:serviceaccount:panacea-fleet:fleet`) and gives its policies a selector no Hub pod
matches.

**(e) Its own database, on the Hub's server, and two identities, as the Hub does.** Fleet's
schema `fleet` lives in a database `fleet` on the Hub's Flexible Server (P8-D10: Entra-only, private
endpoint), so no Hub role can read it and no Fleet role can read the Hub's. A `fleet-migrate`
identity (DDL, owner of the schema) runs a pre-install Job; the serving `fleet` identity gets DML
only, and `INSERT, SELECT` alone on `policy_versions` (task 10's append-only grant,
`task-10.md:375-378`). The roles are created once per environment by a human admin, as the Hub's
are (P8-D10, `task-29.md:193`). **Requires of task 12** a `fleetd migrate` subcommand, and that the
serving process only verifies the schema is at head when `PANACEA_APP_FLEET_MIGRATE=job`: task 12's
`main.go` calls `pgstore.Migrate` on start (`task-12.md:819`), which a DML-only role cannot do
(gap G32-3).

**(f) Fleet calls the Hub and the broker at their https URLs.** `PANACEA_APP_FLEET_HUB_URL` and
`_BROKER_URL` must be `https` (`task-12.md:129`, `task-19.md:504`), and in the cluster TLS ends at
Traefik, so Fleet goes through the edge like the desktop, with the same token checks. Its policy
allows egress to Traefik's pods on the edge's HTTPS container port **and** to 443 outside the
private ranges; which one a pod actually takes depends on how `hub.<zone>` resolves inside the
cluster, **UNVERIFIED**, and task 33 records it. Keysmith is plain http inside its namespace
(`:4100`, no route), so that allow is direct.

**Files:**
- `deploy/charts/fleet/{Chart.yaml,values.yaml,values.schema.json}` (new)
- `deploy/charts/fleet/templates/{_helpers.tpl,serviceaccount.yaml,deployment.yaml,service.yaml,httproute.yaml,pdb.yaml,networkpolicy.yaml,migrate-job.yaml,secretproviderclass.yaml}` (new)
- `deploy/charts/fleet/tests/*_test.yaml`, `tests/values-test.yaml` (new, helm-unittest)
- `deploy/values/{dev,prod}/fleet.yaml` — per-environment values holding **no id** (new)
- `deploy/sql/fleet-principals.sql.tpl` — the database, the two Entra principals, the grants (new)
- `deploy/charts/platform/values.yaml`, `values.schema.json`, `templates/namespaces.yaml`,
  `templates/networkpolicy-defaults.yaml`, `templates/networkpolicy-sandboxes.yaml`,
  `tests/netpol_test.yaml` — `namespaces.fleet`; `egress-internet`'s selector; the two new
  sandbox policies (modified; P8-11's files)
- `deploy/charts/gateway/templates/networkpolicy.yaml` — keysmith admits Fleet's pods on 4100;
  `deploy/charts/gateway/values.yaml` — keysmith's Fleet caller entry on (modified; P6-19's files;
  task 20 left the entry off, `task-20.md:587-589`)
- `deploy/charts/edge/templates/networkpolicy*.yaml` — Traefik admits `panacea-fleet` pods on its
  HTTPS port (modified; P8-13's file, **UNVERIFIED** name until merge)
- `deploy/tests/netpolcover/netpolcover_test.go` — add `fleet` to the rendered charts; two cases
  (modified; P8-13's file, extended by P8-29, `task-29.md:16`)
- `infra/terraform/stacks/workloads/{main.tf,outputs.tf,variables.tf}`, `tests/workloads.tftest.hcl`
  — `module.fleet`, `module.fleet_migrate`, their outputs (modified; P8-10's files)
- `infra/terraform/stacks/identity/*.tf` — Fleet's API registration (`Fleet.Access`) and the
  broker's `Sandboxes.Fleet` app role assigned to Fleet's identity (modified; P8-09's files; task
  17 needs the role, `task-17.md:386-388`)
- `pipelines/scripts/render-sql.sh`'s mapping table — four `fleet_*` placeholders (modified; P8-29's
  file)

**Interfaces:**
- Produces:
  - the chart `fleet`, values below, and the environment `fleetd` reads (task 12's table,
    `task-12.md:125-147`, plus tasks 19 and 20's additions)
  - pod labels other charts select on: `app.kubernetes.io/name: fleet`,
    `app.kubernetes.io/component: server` (serving) and `migrate` (the Job)
  - the platform's `sandbox-egress-gateway` and `orchestrator-egress` policies, and the
    `panacea-fleet` namespace
  - `workloads` outputs `fleet_client_id`, `fleet_identity_name`, `fleet_principal_id`,
    `fleet_migrate_client_id`, `fleet_migrate_identity_name`, `fleet_migrate_principal_id`; the
    `identity` output `fleet_api_client_id`
- Consumes:
  - P8-10's `modules/workload-identity` (`task-10.md:91-105`), unchanged
  - P8-29's chart layout, its migrate-hook weights and its SQL renderer, as patterns
  - P6-19's gatekeeper and keysmith labels and ports (`:4000`, `:4100`)
  - task 12, 19, 20: every `PANACEA_APP_FLEET_*` variable
- Requires of **task 12**: `fleetd migrate` and `PANACEA_APP_FLEET_MIGRATE` (`startup`, the
  default for the offline stack, or `job`) (G32-3).
- Requires of **task 15**: the `panacea-app/class` pod label (G32-2).
- Requires of **task 19**: nothing new. `PANACEA_APP_FLEET_CONTROL_KEY_FILE` is a file this chart
  mounts from Key Vault through the Secrets Store CSI add-on (P8-D11), secret `fleet-control-key`.
- Requires of **tasks 10 and 12**: an Entra token for Postgres per connection (pgx `BeforeConnect`
  with the workload identity), switched by `PANACEA_APP_FLEET_PG_AUTH=entra`, as the Hub's
  `PANACEA_APP_HUB_DB_AUTH=entra` (`task-29.md:155`). Task 12 leaves the flow **UNVERIFIED** and
  names tasks 10 and 32 (`task-12.md:1294`); this chart sets the variable and task 12 must read it
  (G32-4).

#### Contract — `deploy/charts/fleet/values.yaml` (complete)

```yaml
# Every key is required by values.schema.json (additionalProperties: false).
image: ""                     # <acr>/panacea-app/fleetd@sha256:…  (a digest, never a tag)
replicas: 2                   # task 12: two stateless replicas, singletons by advisory lock
namespace: panacea-fleet
host: ""                      # fleet.<zone>, the desktop's route
gateway:                      # phase 8's edge Gateway, as the Hub's chart names it
  name: panacea
  namespace: panacea-edge
edge:
  httpsPort: 8443             # Traefik's HTTPS container port (P8-13, UNVERIFIED until merge)
installId: ""                 # PANACEA_APP_FLEET_INSTALL_ID, 32 lower-case hex, per environment
database:
  host: ""                    # data stack: postgres_fqdn (the Hub's server, P8-D10)
  name: fleet
  appUser: ""                 # workloads: fleet_identity_name
  migrateUser: ""             # workloads: fleet_migrate_identity_name
  maxConns: 8                 # per replica; 2 x 8 + 1 = 17 beside the Hub's 21 (task-29.md:137)
entra:
  authority: https://login.microsoftonline.com
  tenants: ""                 # comma-separated tenant GUIDs
  audience: ""                # identity: fleet_api_client_id's app id URI
  desktopClients: ""          # identity: desktop_client_id
  tokenUrl: ""                # https://login.microsoftonline.com/<tid>/oauth2/v2.0/token
hub:
  url: ""                     # https://hub.<zone>
  scope: ""                   # <hub app id uri>/.default
broker:
  url: ""                     # https://broker.<zone>
  scope: ""                   # api://<broker>/.default
gatewayService:
  baseUrl: http://gatekeeper.panacea-gateway.svc.cluster.local:4000
  keysmithUrl: http://keysmith.panacea-gateway.svc.cluster.local:4100
  keysmithAudience: keysmith  # the projected token's audience phase 6 task 08 reviews (UNVERIFIED)
keyVault:
  name: ""                    # foundation: key_vault_name
  tenantId: ""
  controlKeySecret: fleet-control-key   # >= 32 random bytes, created by the owner's runbook
serviceAccount:
  clientId: ""                # workloads: fleet_client_id
migrate:
  serviceAccount:
    clientId: ""              # workloads: fleet_migrate_client_id
tuning:
  membershipTtl: 60s          # at most 60s (task 12)
  outboxTick: 500ms
  logLevel: info
resources:
  cpu: 500m
  memory: 512Mi
network:
  privateEndpointsCidr: ""    # the value the platform chart takes (task-11.md:49)
```

`values.schema.json` is draft-07 from the block above: every key `required`, every object
`additionalProperties: false`, `replicas` `minimum: 2`, `image` `pattern: "@sha256:[0-9a-f]{64}$"`,
`installId` `pattern: "^[0-9a-f]{32}$"`, `hub.url` and `broker.url` `pattern: "^https://"`, and
`tuning.membershipTtl` one of `15s`, `30s`, `45s`, `60s`. **There is no `devInsecure` key**:
`PANACEA_APP_FLEET_DEV_INSECURE` is never rendered, so a cluster cannot run Fleet against an http
Hub or authority (task 12, `:110`).

#### What the Fleet templates render

`deployment.yaml` — `Deployment/fleet`, `replicas` from values, pod labels
`app.kubernetes.io/name: fleet`, `app.kubernetes.io/component: server`,
`azure.workload.identity/use: "true"`; priority class `panacea-platform-critical`
(`task-11.md:105`: Fleet is on the path of every team agent, so it must not be preempted by a
session burst); `topologySpreadConstraints` over `topology.kubernetes.io/zone`, `maxSkew: 1`,
`whenUnsatisfiable: ScheduleAnyway`; `terminationGracePeriodSeconds: 30`;
`automountServiceAccountToken: false`; non-root, read-only root, all capabilities dropped,
`RuntimeDefault` seccomp (`restricted` Pod Security, `task-11.md:93-99`); requests equal limits
from `resources`. Liveness `/healthz`, readiness `/readyz` on 8080 (`task-12.md:61-66`). Volumes:
the CSI secret (`/run/panacea/fleet/control-key`, read-only) and a projected service-account
token with `audience: <gatewayService.keysmithAudience>`, `expirationSeconds: 3600`
(`/run/panacea/fleet/keysmith/token`). The environment, every variable literal:

| Variable | Value |
|---|---|
| `PANACEA_APP_FLEET_ADDR` | `0.0.0.0:8080` |
| `PANACEA_APP_FLEET_PG_DSN` | `postgres://<database.appUser>@<database.host>:5432/<database.name>?sslmode=verify-full&pool_max_conns=<database.maxConns>` |
| `PANACEA_APP_FLEET_PG_AUTH` | `entra` (G32-4) |
| `PANACEA_APP_FLEET_MIGRATE` | `job` (G32-3) |
| `PANACEA_APP_FLEET_HUB_URL`, `_HUB_SCOPE` | `hub.url`, `hub.scope` |
| `PANACEA_APP_FLEET_TOKEN_URL`, `_CLIENT_ID` | `entra.tokenUrl`, `serviceAccount.clientId` |
| `PANACEA_APP_FLEET_ASSERTION_FILE` | `$(AZURE_FEDERATED_TOKEN_FILE)`, the webhook-injected path (`task-29.md:158-160`) |
| `PANACEA_APP_FLEET_ENTRA_AUTHORITY`, `_ENTRA_TENANTS`, `_AUDIENCE`, `_DESKTOP_CLIENTS` | from `entra` |
| `PANACEA_APP_FLEET_REQUIRED_SCOPE` | `Fleet.Access` |
| `PANACEA_APP_FLEET_MEMBERSHIP_TTL`, `_OUTBOX_TICK`, `_LOG_LEVEL` | from `tuning` |
| `PANACEA_APP_FLEET_CONTROL_KEY_FILE` | `/run/panacea/fleet/control-key/fleet-control-key` |
| `PANACEA_APP_FLEET_BROKER_URL`, `_BROKER_SCOPE` | `broker.url`, `broker.scope` |
| `PANACEA_APP_FLEET_INSTALL_ID` | `installId` |
| `PANACEA_APP_FLEET_GATEWAY_BASE_URL`, `_KEYSMITH_URL` | from `gatewayService` |
| `PANACEA_APP_FLEET_KEYSMITH_TOKEN_FILE` | `/run/panacea/fleet/keysmith/token` |

No value is a secret: the control key is a file, the database password does not exist, and the
Hub token is minted from the projected assertion. `$(AZURE_FEDERATED_TOKEN_FILE)` relies on
Kubernetes' dependent-variable expansion of a variable the webhook injects into the same
container; **that the webhook's variable is visible to that expansion is UNVERIFIED**, and the
fallback is the literal path the webhook documents, pinned by a test either way.

`serviceaccount.yaml` — `ServiceAccount/fleet`, annotation `azure.workload.identity/client-id:
<serviceAccount.clientId>`. `service.yaml` — `Service/fleet`, ClusterIP, 80 → 8080.
`httproute.yaml` — `HTTPRoute/fleet` on `host`, parent the edge Gateway, backend `fleet:80`, as the
Hub's route. `pdb.yaml` — `PodDisruptionBudget/fleet`, `minAvailable: 1`. `secretproviderclass.yaml`
— `SecretProviderClass/fleet-control-key`, provider `azure`, `clientID` the serving identity,
object `controlKeySecret`, **no** `secretObjects` (the key is never synced into a Kubernetes
Secret). `migrate-job.yaml` — the Hub's three hooks, renamed (`task-29.md:166-183`):
`ServiceAccount/fleet-migrate` at weight `-10`, `NetworkPolicy/fleet-migrate-egress` at `-5`
(kube-dns, the private-endpoint CIDR on 5432, 443 outside the private ranges for Entra), and
`Job/fleet-migrate` at `0` running `["/fleetd", "migrate"]` with the migrate DSN and
`backoffLimit: 0`.

#### Contract — `deploy/charts/fleet/templates/networkpolicy.yaml` (complete)

```yaml
{{- $private := list "10.0.0.0/8" "172.16.0.0/12" "192.168.0.0/16" "169.254.0.0/16" "168.63.129.16/32" }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: fleet-server
  namespace: {{ .Values.namespace }}
spec:
  podSelector:
    matchLabels: { app.kubernetes.io/name: fleet, app.kubernetes.io/component: server }
  policyTypes: [Ingress, Egress]
  ingress:
    - from:   # the desktop, through the edge only
        - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: {{ .Values.gateway.namespace }} } }
          podSelector: { matchLabels: { app.kubernetes.io/name: traefik } }
      ports: [ { port: 8080, protocol: TCP } ]
  egress:
    - to:     # DNS
        - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: kube-system } }
          podSelector: { matchLabels: { k8s-app: kube-dns } }
      ports: [ { port: 53, protocol: UDP }, { port: 53, protocol: TCP } ]
    - to:     # Postgres, the Hub's server's private endpoint
        - ipBlock: { cidr: {{ required "network.privateEndpointsCidr" .Values.network.privateEndpointsCidr }} }
      ports: [ { port: 5432, protocol: TCP } ]
    - to:     # the Hub and the broker at their https hosts, in-cluster path (decision f)
        - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: {{ .Values.gateway.namespace }} } }
          podSelector: { matchLabels: { app.kubernetes.io/name: traefik } }
      ports: [ { port: {{ .Values.edge.httpsPort }}, protocol: TCP } ]
    - to:     # keysmith: per-session gateway tokens (task 20)
        - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: panacea-gateway } }
          podSelector: { matchLabels: { app.kubernetes.io/name: keysmith } }
      ports: [ { port: 4100, protocol: TCP } ]
    - to:     # Entra, and the Hub or broker if their hosts resolve to the edge's public address
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
{{- range $private }}
              - {{ . }}
{{- end }}
      ports: [ { port: 443, protocol: TCP } ]
```

Fleet has **no** rule to `panacea-sandboxes`: every byte it exchanges with a sandbox goes through
the broker's relay (decision a), so a Fleet compromise does not add a network path to a sandbox.
It has no rule to the gatekeeper or LiteLLM: Fleet never calls a model. The Traefik label is
P8-13's (**UNVERIFIED** until merge; `task-29.md:107` says the broker's policy uses "the same
labels task 13's `traefik-*` policies select", and this chart reads them from the same helper).

#### Contract — the platform chart's additions (complete)

`values.yaml` gains `namespaces.fleet: panacea-fleet` (schema: required, a DNS-1123 label).
`namespaces.yaml` renders a fifth `Namespace` with the same labels as `panacea-hub`, including
`panacea-app/edge-routes: "true"`; `networkpolicy-defaults.yaml` ranges over all five.
`networkpolicy-sandboxes.yaml` changes one selector and gains two policies:

```yaml
# egress-internet: unchanged rules; the selector now leaves out the orchestrator class (decision b).
spec:
  podSelector:
    matchExpressions:
      - { key: panacea-app/class, operator: NotIn, values: [orchestrator] }
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: sandbox-egress-gateway, namespace: {{ .Values.namespaces.sandboxes }} }
spec:
  podSelector: {}           # every sandbox, machine and orchestrator pod (decision c)
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: panacea-gateway } }
          podSelector: { matchLabels: { app.kubernetes.io/name: gatekeeper } }
      ports: [ { port: 4000, protocol: TCP } ]
```

The `orchestrator` class needs no policy of its own: with `egress-internet` no longer selecting
it, its egress is exactly `allow-dns` plus `sandbox-egress-gateway`, and its ingress is the
existing `session-ingress-from-broker` on 8080 (`task-11.md:151-152`), provided task 15 keeps the
`panacea-app/component: sandbox` label on it. Machines gain the gateway allow too; that is harmless
(a machine pod holds no gateway token) and keeps one rule instead of two.

#### Contract — the Terraform additions (complete)

`stacks/workloads/main.tf` gains, beside `module.hub` and `module.hub_migrate` (`task-10.md:145-156`):

```hcl
module "fleet" {
  source          = "../../modules/workload-identity"
  name            = "id-panacea-fleet-${var.environment}"
  oidc_issuer_url = data.terraform_remote_state.foundation.outputs.aks_oidc_issuer_url
  namespace       = var.namespaces.fleet
  service_account = "fleet"
  role_assignments = [{
    scope                = data.terraform_remote_state.foundation.outputs.key_vault_id
    role_definition_name = "Key Vault Secrets User" # the control key, read through CSI
  }]
  tags = local.tags
}

module "fleet_migrate" {
  source           = "../../modules/workload-identity"
  name             = "id-panacea-fleet-migrate-${var.environment}"
  oidc_issuer_url  = data.terraform_remote_state.foundation.outputs.aks_oidc_issuer_url
  namespace        = var.namespaces.fleet
  service_account  = "fleet-migrate"
  role_assignments = [] # DDL rights are a Postgres grant, not an Azure role
  tags             = local.tags
}
```

`variables.tf`'s `namespaces` object gains `fleet = string`, default `"panacea-fleet"`.
`outputs.tf` gains the six `fleet_*` outputs named under Interfaces, with the Hub's descriptions
(`task-10.md:146-153`), and `helm_values` gains `fleet.serviceAccount.clientId` and
`fleet.migrate.serviceAccount.clientId`. The module's input names (`name`, `oidc_issuer_url`,
`location` if it has one) and the foundation output names are the sibling plan's; **UNVERIFIED
until merge**, and step 5 reconciles them. `Key Vault Secrets User` must already be in
`infra/terraform/role-allowlist.txt` (P8-10's `role-allowlist.sh`); if it is not, adding it is a
reviewed change to that file, not a bypass.

Scoped to the vault, that role reads **every** secret in it, including the Hub's if it ever
has any (`hub_key_vault_access`, `task-10.md:126-130`). A per-secret role assignment (scope
`<vault id>/secrets/fleet-control-key`) is the tighter grant; Key Vault RBAC supports a secret
scope (training knowledge, **UNVERIFIED** for `azurerm_role_assignment`), and step 5 uses it if
`terraform plan` accepts it.

`stacks/identity` gains a fourth registration, **Fleet API**: app id URI `api://panacea-fleet-<env>`,
one delegated scope `Fleet.Access` that the desktop public client is pre-authorised for, and an
app role `Sandboxes.Fleet` on the **broker** API's registration, assigned to `module.fleet`'s
principal (the role task 17 checks, `task-17.md:386-388`). Output `fleet_api_client_id`.

`deploy/sql/fleet-principals.sql.tpl`, rendered by P8-29's `render-sql.sh` and run by a human
Postgres admin (P8-D10):

```sql
-- As the Entra admin, connected to the "postgres" database.
CREATE DATABASE fleet;
SELECT * FROM pgaadauth_create_principal_with_oid('{{fleet_migrate_identity_name}}', '{{fleet_migrate_principal_id}}', 'service', false, false);
SELECT * FROM pgaadauth_create_principal_with_oid('{{fleet_identity_name}}', '{{fleet_principal_id}}', 'service', false, false);
-- Then connected to "fleet".
CREATE SCHEMA IF NOT EXISTS fleet AUTHORIZATION "{{fleet_migrate_identity_name}}";
GRANT USAGE ON SCHEMA fleet TO "{{fleet_identity_name}}";
ALTER DEFAULT PRIVILEGES FOR ROLE "{{fleet_migrate_identity_name}}" IN SCHEMA fleet
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO "{{fleet_identity_name}}";
-- task 10's append-only half (task-10.md:375-378) is its migration's REVOKE, which runs as the owner.
REVOKE ALL ON DATABASE fleet FROM PUBLIC;
GRANT CONNECT ON DATABASE fleet TO "{{fleet_identity_name}}", "{{fleet_migrate_identity_name}}";
```

`pgaadauth_create_principal_with_oid`'s argument list is the one P8-29's template uses; its exact
signature is **UNVERIFIED** there too (`task-29.md:250`), and this template follows whatever that
task records. The Hub's roles get no grant on `fleet`, and Fleet's none on `hub`.

- [ ] **Step 1: Write the tests first, each with the mutant it kills**

helm-unittest suites in `deploy/charts/fleet/tests/` and the platform chart's `tests/`; two cases in
`netpolcover`; one `terraform test` run. Run them now: every one fails (no chart, no policy).

| Suite / test | Asserts | Mutant it kills |
|---|---|---|
| `fleet / two replicas and a PDB` | `replicas: 2`; `PodDisruptionBudget` `minAvailable: 1`; `replicas: 1` fails schema | a single replica, so one pod restart stops every team agent's turns |
| `fleet / image by digest` | `image: x:latest` fails schema | a mutable tag |
| `fleet / no secret in the environment` | no `env` item has `valueFrom.secretKeyRef`; no env value matches `password=`; `SecretProviderClass` has no `secretObjects` | the control key synced into a Secret and an env var |
| `fleet / no dev-insecure` | no rendered env name is `PANACEA_APP_FLEET_DEV_INSECURE` | the offline stack's switch reaching a cluster |
| `fleet / every variable task 12 requires` | the env names equal the set in `tests/fleetd-env.txt`, which step 2 generates from `config.go`'s table | a variable renamed in `config.go`, so `fleetd` exits 2 on the cluster |
| `fleet / workload identity on server and migrate` | both SAs carry their client-id annotation; both pod templates carry the `use` label; the `migrate` SA's id is not the server's | DDL rights on the serving identity (task 10's grant made moot) |
| `fleet / migrate hooks exist before the Job` | weights `-10`, `-5`, `0`; hook policy selects `component: migrate` only | the Job's pod sealed on the first install (`task-29.md:176`) |
| `fleet / no path to a sandbox` | no egress rule of `fleet-server` names `panacea-sandboxes` | a direct Fleet-to-pod path around the broker |
| `fleet / no path to a model` | no egress rule names `gatekeeper` or `litellm` | Fleet holding a gateway path it does not need |
| `platform / orchestrators have no internet` | `egress-internet`'s selector has `NotIn [orchestrator]` on `panacea-app/class`; a pod labelled `orchestrator` matches no policy with a `0.0.0.0/0` rule | the selector left `{}` and an orchestrator-only allow added beside it (a union cannot subtract) |
| `platform / the gateway allow is one port on one app` | `sandbox-egress-gateway` has one `to` (gatekeeper in `panacea-gateway`) and one port, 4000/TCP | `namespaceSelector` alone, which opens keysmith and LiteLLM |
| `platform / five namespaces, five default-denies` | `panacea-fleet` exists with `enforce: restricted` and a `default-deny` | Fleet's namespace left open |
| `netpolcover / sandbox_reaches_one_service` | over the rendered platform, gateway and fleet charts: the set of (namespace, pod labels, port) a `panacea-sandboxes` pod may reach inside the cluster is exactly `{kube-dns:53, gatekeeper:4000}` | a later chart adding an in-cluster allow from sandboxes |
| `netpolcover / every_denied_workload_has_an_allow` | stays green with `fleet` added (the existing case, `task-11.md:141-145`) | Fleet's pods with no egress (the chart renders but `fleetd` cannot start) |
| `workloads.tftest.hcl / fleet identities` | `module.fleet.subject == "system:serviceaccount:panacea-fleet:fleet"`; `module.fleet_migrate` has no role assignment; the Fleet role is scoped to the vault or narrower | a subject typo; an Azure role on the DDL identity |

`sandbox_reaches_one_service` is the skeleton's "a sandbox pod cannot reach any other service
port", written as a render check. The live half (a pod that actually tries) is task 47's probe on
a real Kata pod, since Cilium's handling of `ipBlock` is **UNVERIFIED** (`task-11.md:161-164`).

- [ ] **Step 2: The chart.** Chart, values, schema, templates; `tests/fleetd-env.txt` generated by
  `grep -o 'PANACEA_APP_FLEET_[A-Z_]*' control-plane/cmd/fleetd/config.go | sort -u`, minus
  `_DEV_INSECURE` and `_TEST_PG_DSN`, and checked in so the suite fails when `config.go` moves.
- [ ] **Step 3: The platform chart's delta.** The fifth namespace, the selector, the gateway allow.
  P8-11's existing suites stay green.
- [ ] **Step 4: The sibling charts' deltas.** Keysmith's ingress from Fleet on 4100 and its Fleet
  caller entry on (P6-19); Traefik's ingress from `panacea-fleet` (P8-13); `netpolcover`'s chart list
  and two cases.
- [ ] **Step 5: Terraform and SQL.** `module.fleet`, `module.fleet_migrate`, the outputs, the
  identity registration, the SQL template and its four placeholders in `render-sql.sh`'s table.
  Reconcile every module input and output name against the merged files, and record any rename in
  the commit message.
- [ ] **Step 6: Acceptance.**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && helm lint deploy/charts/fleet -f deploy/charts/fleet/tests/values-test.yaml && helm unittest deploy/charts/fleet deploy/charts/platform
```
Expected: `1 chart(s) linted, 0 chart(s) failed`; helm-unittest `Charts: 2 passed, 2 total`, every
suite passed.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && go test ./deploy/tests/netpolcover/... -count=1
```
Expected: `ok`, with `fleet` among the rendered charts and both new cases run (`-v` lists them).

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/infra/terraform/stacks/workloads && terraform init -backend=false && terraform test
```
Expected: every run passes, `fleet identities` among them.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && helm template fleet deploy/charts/fleet -f deploy/charts/fleet/tests/values-test.yaml | grep -c "kind: HTTPRoute"
```
Expected: `1` — the desktop's route to Fleet. And the same command over
`deploy/charts/gateway` prints `0` (P6-D1: no route to the gateway, unchanged here).

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && bash pipelines/scripts/render-sql.sh deploy/sql/fleet-principals.sql.tpl pipelines/scripts/tests/fixtures/workloads-outputs.json | grep -c "{{"
```
Expected: `0` (after the fixture gains the six `fleet_*` outputs).

Helm is not on the build laptop (`task-11.md:14-17`); if it still is not, the helm steps run in the
CI job P8-28 pins, and this part's commit says which ran where.

- [ ] **Step 7: Commit** after `git add` of the new files: `git commit -F
  .superpowers/sdd/team/msg/t32.txt -- deploy infra/terraform pipelines/scripts`.

**What this task does not claim:**

- **An install.** Nothing here reaches a cluster; task 33 installs it through the `deploy` pipeline
  by the owner's runbook, after the owner creates the Key Vault secret and runs the SQL.
- **That Cilium enforces these policies in a Kata pod as rendered.** Task 47 probes every deny from
  inside a real pod (P8-32's checklist gains its five probes there).
- **That an Orchestrator runs with no internet.** The CLI may try non-essential traffic at start
  (telemetry, update checks); the class's managed file (task 13) should turn it off, and whether a
  blocked call slows or breaks a headless turn is **UNVERIFIED**; task 33 records it.
- **Which path Fleet's https calls take** (decision f), or that Azure's load balancer hairpins
  from a pod under CNI Overlay. Task 33 records it.
- **The AU allowlist, the Australia East guard and the mirror pod's zero egress**: tasks 47 and 58.

**Gaps against the skeleton and sibling parts, raised here:**

- **G32-1.** The scope's "the `orchestrator` class's egress to … Fleet's control route" has no
  counterpart: control is inbound through the broker (decision a).
- **G32-2.** Task 15 must label every sandbox pod `panacea-app/class: <Spec.Class>` and keep
  `panacea-app/component: sandbox` on the orchestrator class.
- **G32-3. CLOSED by fix batch b07.** It read: task 12 migrates on start with the serving pool, and
  a DML-only role cannot. Task 12 now reads `PANACEA_APP_FLEET_MIGRATE` (`start` default, or `job`);
  on `job` it runs no migration and refuses to serve a schema behind its code, naming the Job
  (`task-12.md:144-147`, `:166-177`, the start path and two config tests). This chart's value is
  therefore read by the binary it sets it on.
- **G32-4. CLOSED by fix batch b07.** It read: task 12 has no Entra switch for Postgres. It now
  reads `PANACEA_APP_FLEET_PG_AUTH` (`password` default, or `entra`), and on `entra` the pool's
  password is an Entra token refreshed from the same assertion source the Hub client uses. Both
  variables fail the start by name on any other value, so a typo in this chart is a named error and
  not a silent default.
- **Skeleton.** The file map's `deploy/charts/fleet/**` row does not list the platform, gateway,
  edge, `netpolcover`, `workloads`, `identity` and SQL files this task must touch; the deltas table
  lists only `deploy/charts/platform/templates/*` and the two Terraform modules for task 32.
