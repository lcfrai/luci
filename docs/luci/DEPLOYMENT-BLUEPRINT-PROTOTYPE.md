# Deployment blueprint prototype

This is a planning/compiler prototype, not a live cloud connector. It gives
an application, inference workload, existing infrastructure and optional
telemetry receiver one inspectable topology. It does not replace the existing
Local, GitHub Pages, Coolify or AWS/Azure single-machine execution paths.
Those paths retain their current authority and acceptance gates.

## What it produces

`deploymentplan.Compile` is deterministic and performs no I/O. It returns:

- A canonical blueprint and SHA-256 digest binding the project, supplied source
  revision, configuration revision, resource references and service settings.
- Dependency-ordered review/compile steps and aggregate explicit resource
  requests/limits across replicas. These are requested capacity, not verified
  cluster availability, a price estimate or a provider billing cap.
- Unresolved decisions, without guessing images, ports, replica counts,
  resource budgets, model servers, model files or destination identities.
- For a fully specified Kubernetes topology, exact JSON manifest strings and
  their SHA-256 digests: one `apps/v1` Deployment and one internal ClusterIP
  Service per app or inference service. JSON strings preserve the exact bytes
  through an API response; hashes cover those strings including final newline.

Invalid topology (duplicate IDs, unknown dependencies, cycles), mutable image
tags, unsafe model paths, credential-bearing telemetry URLs and inconsistent
resource requests are rejected. Missing required inputs produce decisions;
no partial manifest set is emitted. The compiler bounds 32 services, 64
external resources and 128 KiB input. GPU requirements use explicit
`nvidia.com/gpu` counts; no device plugin is installed or verified.

Inference uses an explicit digest-pinned existing container image, its exact
command/arguments, and a read-only mount of an existing populated model PVC.
No model/image acquisition occurs. Telemetry optionally supplies standard
OpenTelemetry OTLP/HTTP environment variables for an existing HTTPS receiver.
This does not instrument an app, send data, validate a receiver, provision
PostHog or create a business analytics system.

The dependency graph is a planning order, not a Kubernetes startup/readiness
barrier. Manifests have no automatic service-dependency wait, health probes,
ingress, public load balancer, credentials, namespace, storage or cluster
creation. Image compatibility, model licence, GPU scheduling, application
health, network policy, secret binding, costs and recovery require further
review and adapter work. Default pods do not mount service-account tokens;
containers drop Linux capabilities and privilege escalation.

## Plan-only API

`POST /v1/product/projects/{id}/deployment-blueprint` accepts a strict
`Blueprint` JSON body, requires an authenticated actor and current project
read access, and rechecks access before returning. The path and body project
IDs must agree. It returns `Cache-Control: no-store` and persists nothing.
This POST performs computation and bounded project-identity reads only: no
Git commands, provider calls, subprocess, model, sandbox or release mutation.

Minimal input (all unresolved values deliberately remain decisions):

```json
{
  "version": 1,
  "projectId": "p-your-project",
  "source": {"commit": "", "configurationRevision": 0},
  "infrastructure": {
    "id": "cluster", "kind": "kubernetes", "reference": "", "namespace": ""
  },
  "services": [{
    "id": "app", "kind": "app", "image": "", "port": 0,
    "replicas": 0,
    "resources": {
      "requestMilliCpu": 0, "limitMilliCpu": 0,
      "requestMemoryMiB": 0, "limitMemoryMiB": 0
    }
  }]
}
```

For a compiled app, supply the exact 40-character lowercase source commit,
existing target reference and namespace, `registry/image@sha256:` plus its
64-character digest, port, replicas and nonzero resource requests/limits.
Inference additionally requires `kind: "inference"`, explicit `command` or
`args`, and `model: {name, claimName, mountPath, relativePath?}`.
An existing analytics resource is `{id, kind: "analytics", reference}`;
an optional service `telemetry: {resourceId, endpoint}` binds that resource.
Other external resource kinds are database, storage, secret and ingress;
they are references, not provisioning operations or injected credentials.

Supplied commit/configuration values are intent, not independently verified
project evidence. `sourceVerified` and `effectsApplied` remain false, even
when the output state is `compiled-prototype`.

## Connector contract and next commissioning gates

The package defines `Connector.Plan`, `Observe` and `Apply` separately, with
plan digest, operation identity, original resource references and prior
observation. No implementation of live `Apply` ships in this prototype.
An approval reference is opaque and is not self-authorizing. A future adapter
must resolve it against current actor/project authority, exact source and
target, expected state, acknowledged effects and current provider costs.

An ambiguous provider result must remain `unknown` with its original IDs;
it must not become absent or trigger an automatic create replay. A future
apply needs durable operation reservation, observation/reconciliation and
owned cleanup before real commissioning. JSON output is not a dry-run API
result, a successful rollout, a checked release or full product acceptance.

Source tests cover app/inference manifests using the existing Kubernetes Go
types, explicit model/GPU/OTLP bindings, canonicalization without mutation,
cross-project isolation, dependency order, missing decisions and refusal of
contradictory input. API tests cover access, revocation, exact project binding
and bounded strict JSON. Their execution is a separate reviewed test batch;
source authoring alone does not establish a pass.

Primary format references: [Kubernetes Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/),
[Kubernetes Services](https://kubernetes.io/docs/concepts/services-networking/service/),
and [OpenTelemetry OTLP exporter configuration](https://opentelemetry.io/docs/specs/otel/protocol/exporter/).
