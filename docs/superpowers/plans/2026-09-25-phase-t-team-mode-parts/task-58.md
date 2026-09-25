### Task 58: Phase 8 delta — the lane mirror's per-project Kata pod

**Slice:** TM2. **Focused time:** 3 h. **dependsOn:** 15 (`Spec.Class`, `validateClass`, the
policy Secret and the control token, `task-15.md:95-315`), 48 (the read-only fetch credential,
which the mirror deliberately does **not** hold, decision (b)); through 15, P8-14 (**built**:
`control-plane/internal/sandbox/manager.go` on `feat/env`, `9ef70c3`), P8-11 (**planned**: the
platform chart, `phase-8 …/task-11.md:137-164`) and task 17 (the broker's Fleet caller). Built
here after `feat/env`'s sync-merge into `feat/team-mode`. **Seams:** P8-D12 (sandbox network
policy); produces the mirror's pod contract, which task 59's runner serves.

The lane mirror is one bare repository per team project, holding every member's lanes under
`refs/panacea/lanes/<owner-hash>/<wt-id>`, where tier M runs `git merge-tree` and the team land
queue builds its integration head (`05-design.md` §8.2, §8.3). **A worker controls every object on
its lane**, so git's parsers must never run over those objects in a process that holds anyone's
delegation or credential (`07` I11, §15.2's row). This task gives the mirror its own Kata pod,
per project, with nothing in it worth stealing and no way out.

**Decisions where the scope is silent or the sources disagree, made here.**

**(a) The mirror lives in its own namespace, `panacea-mirrors`, with no egress at all.** Phase 8's
`egress-internet` policy in `panacea-sandboxes` selects **every** pod there, sessions, machines
and balloons alike (`phase-8 …/task-11.md:155-159`). A mirror placed in that namespace would
inherit internet egress from a selector nobody reads as "the mirror". A fifth namespace, with the
platform chart's `default-deny` (`:137-145`) and exactly one allow, **ingress TCP 8080 from the
broker**, cannot inherit anything. It gets no DNS allow either: the mirror resolves nothing.

**(b) The mirror holds no credential, not even the read-only fetch credential.** The scope allows
it the fetch credential (task 48) and §8.2 says it *"reads the customer's remote with the
project's read-only fetch credential only"*. This part is stricter, and says why: fetching needs
egress to the git host, and an FQDN allow to `github.com` or `dev.azure.com` admits a push to
**any** repository on that host. A mirror compromised through a git parser bug on a hostile lane
could then send every member's lanes to a repository the attacker owns, with a credential or
without one. With no egress, the only channel out is the typed result Fleet reads (task 59). The
mirror gets its base another way: the lane owner's `machined`, which already fetches with the
fetch credential (task 48), sends the default branch as a bundle when the mirror reports a
missing prerequisite (task 59's `need-base`, task 60's sender). §8.2's *"no credential and no
egress"* sentence is the one this part keeps; its *"reads the customer's remote"* sentence is
met indirectly, through `machined`. Listed as gap 1 for the owner to confirm.

**(c) PTY-less: no `sandboxd`, no attach token.** The container's command is task 59's
`/usr/local/bin/panacea-mirror`, which serves one route, `/control`, on 8080, with task 15's
control token as its bearer. The attach Secret stays the id's claim (`manager.go:352-363`) but
holds **only** the `control` key; `PANACEA_APP_TOKEN` is not rendered. Phase 6's gateway token
directory and `apiKeyHelper` do not exist in this pod (task 13's invariant I2 already forbids the
helper in the mirror's policy, `task-13.md:465-473`). The policy Secret is still mounted: task 15
makes it mandatory for every class, and `mirror.json` *"only locks the door"* (`task-13.md:69-70`).

**(d) One project disk, which only the mirror can mount.** A `PersistentVolumeClaim`
`mirror-<project-hash>` in `panacea-mirrors`, storage class `panacea-machine-disk` (P8-11's
`Premium_ZRS`, `ReadWriteOnce`), 32 GiB, mounted read-write at `/mirror`. A PVC is namespaced
and a pod can mount only a claim in its own namespace, so no sandbox in `panacea-sandboxes` can
mount it: the scope's *"no sandbox mounting its disk"* is a Kubernetes guarantee here, not a review
rule, and the broker is the only writer of pods in `panacea-mirrors` (its Role, gap 2).

**(e) One mirror per project, named from the project.** Every mirror is created through one
owner view, `WithOwner(MirrorOwner)` with `MirrorOwner = "mirror"`, which matches `WithOwner`'s
`^[a-z0-9]{1,63}$` (`manager.go:66`) and can never equal an `OwnerLabelValue` (those are 32 hex
characters, `:141-144`). Its id is `MirrorID(projectID)`, the first 20 lower-case hex characters of
`sha256("mirror/" + projectID)`, so `PodName` gives `sandbox-mirror-<20 hex>` (35 characters).
The id carries the uniqueness, not the owner: `PodName` keeps only an owner's first 8 characters
(`:151-159`), and a per-project owner cut to 8 would collide at a few thousand projects. A second
`EnsureMirror` for the same project meets the first one's attach Secret, the claim, and returns
the running pod.

**(f) Retention.** `DeleteMirror(project)` deletes the pod, both Secrets and the PVC, whose
reclaim policy `Delete` (P8-11) then removes the disk: *the mirror goes when the project does*
(§8.2). **A member's lanes are deleted when they leave the project** (§8.2, and `05-design.md`'s
Q7): that is a `prune-owner` request to the running mirror (task 59), sent by task 44's leaver
path. It is **Q7's recommendation, not an owner decision**; this part builds it so the owner can
switch it off by one constant (`PruneOnLeave`) if Q7 is answered otherwise.

**(g) The image is pinned by digest, and git with it.** `EnsureMirror` refuses an `Image` that is
not `<repo>@sha256:<64 hex>`. The mirror image is built from `images/mirror/Dockerfile`, whose base
is pinned by digest and whose git is a pinned package version; the image digest moves only with
the CLI bump job's gates (task 65, §10.4).

**Files:**
- `control-plane/internal/sandbox/manager.go` (`feat/env`, task 15's shape) — `ClassMirror`,
  `MirrorCommand`, the mirror branch in `validateClass`, `containerEnv` without the attach token for
  a mirror, `attachSecret` without the `token` key for a mirror, the `mirror-disk` volume in `pod`,
  `componentMirror` in `podLabels` (modify)
- `control-plane/internal/sandbox/mirror.go` — `MirrorOwner`, `MirrorID`, `MirrorSpec`,
  `(*Manager).EnsureMirror`, `(*Manager).DeleteMirror`, `mirrorClaim` (new)
- `control-plane/internal/sandbox/mirror_test.go` — the pod's shape on the fake clientset (new)
- `control-plane/internal/backend/backend.go` — `KubeLabelComponent`'s comment gains `"mirror"`;
  `ValidateSpec` admits `Class: "mirror"` (modify)
- `control-plane/cmd/broker/mirrors.go`, `mirrors_test.go` — Fleet-only `PUT` and `DELETE
  /v1/mirrors/{projectId}`, and the `control` relay to a mirror (new, on task 17's broker)
- `deploy/charts/platform/templates/namespaces.yaml` — the fifth namespace (modify, P8-11)
- `deploy/charts/platform/templates/networkpolicy-mirrors.yaml` — one allow (new)
- `deploy/charts/platform/tests/networkpolicy_mirrors_test.yaml` — helm-unittest (new)
- `deploy/charts/broker/templates/rbac.yaml` — a `Role` in `panacea-mirrors` for pods, Secrets
  and PVCs, bound to the broker's service account (modify, P8 task 29, **UNVERIFIED until merge**)
- `images/mirror/Dockerfile` — base and git pinned; `panacea-mirror` as `ENTRYPOINT` (new)

**Interfaces:**
- Produces: `sandbox.ClassMirror`, `sandbox.MirrorCommand` (`/usr/local/bin/panacea-mirror`),
  `sandbox.MirrorOwner`, `sandbox.MirrorID(projectID)`, `sandbox.MirrorSpec`,
  `EnsureMirror`, `DeleteMirror`, `sandbox.MirrorMountPath` (`/mirror`), `sandbox.MirrorPort`
  (8080); the broker's two mirror routes and `GET /v1/mirrors/{projectId}/control`; the namespace
  `panacea-mirrors` and its policy `mirror-ingress-from-broker`.
- Consumes: task 15's `Class`, `validateClass`, `policySecret`, `ControlToken`, `ControlSecretKey`,
  `Create` order and `ownByPod` (`task-15.md:95-315`); P8-14's `PodName`, `WithOwner`, `pod`,
  `attachSecret` (`manager.go:126-168`, `:256-348`); task 13's `policy.Render(…, model.ClassMirror)`
  (`task-13.md:629-638`); task 17's `principalOf`, app-only Fleet claims and control relay
  (`task-17.md:36-73`, `:423`); P8-11's `default-deny` and `panacea-machine-disk`.
- Consumed by: task 59 (serves `/control` in this pod, reads `/mirror`), task 60 (sends bundles
  through the relay), task 61 (the land queue), task 44 (`prune-owner` on leave), task 69.

#### The additions to `manager.go` (complete; beside task 15's declarations)

```go
const (
	ClassMirror Class = "mirror" // task 58: one per team project, PTY-less

	// MirrorCommand is the mirror's one process (task 59). It is not run by
	// sandboxd: the mirror pod has no PTY and no /attach.
	MirrorCommand   = "/usr/local/bin/panacea-mirror"
	MirrorMountPath = "/mirror"
	MirrorPort      = 8080 // = backend.ContainerPort, so the broker's relay dials one port
	componentMirror = "mirror"

	mirrorCPU    = "500m"
	mirrorMemory = "1Gi" // merge-tree over a large repository; measured in task 69, not here
)
```

The mirror branch of `validateClass` (task 15's `switch spec.Class`, a new `case`):

```go
	case ClassMirror:
		if spec.Machine != "" { // P8-15's field: a mirror never takes a member's disk
			return spec, fmt.Errorf("sandbox %q: a mirror mounts only its project disk, got machine %q", spec.ID, spec.Machine)
		}
		if spec.AttachToken != "" {
			return spec, fmt.Errorf("sandbox %q: a mirror has no PTY and takes no attach token", spec.ID)
		}
		if spec.ControlToken == "" {
			return spec, fmt.Errorf("sandbox %q: a mirror is reached only through /control, which needs its token", spec.ID)
		}
		if len(spec.Env) != 0 {
			return spec, fmt.Errorf("sandbox %q: a mirror takes no caller environment", spec.ID)
		}
		if !digestRef.MatchString(spec.Image) {
			return spec, fmt.Errorf("sandbox %q: a mirror image must be pinned by digest, got %q", spec.ID, spec.Image)
		}
		if spec.Command != "" && spec.Command != MirrorCommand {
			return spec, fmt.Errorf("sandbox %q: a mirror runs %s, not %q", spec.ID, MirrorCommand, spec.Command)
		}
		spec.Command = MirrorCommand
		if spec.CPU == "" {
			spec.CPU = mirrorCPU
		}
		if spec.Memory == "" {
			spec.Memory = mirrorMemory
		}
```

with, beside `ownerPattern`:

```go
// digestRef is an image pinned by content: repo@sha256:<64 lower-case hex>.
var digestRef = regexp.MustCompile(`^[a-z0-9][a-z0-9._/:-]*@sha256:[0-9a-f]{64}$`)
```

`task-15.md:158-160` already refuses a `ControlToken` without a `SessionID`, because `sandboxd`
stamps it; the mirror has no `sandboxd`, so that rule moves inside the `worker` and
`orchestrator` cases and the mirror case above takes its place. `TestAControlTokenNeedsASessionID`
keeps passing for both of those classes.

**Three one-line branches in the builders.** `podLabels` sets `KubeLabelComponent` to
`componentMirror` for the mirror (so phase 8's `session-ingress-from-broker`, which selects
`component: sandbox`, can never match it even if a mirror were placed in the wrong namespace).
`containerEnv` skips `PANACEA_APP_TOKEN` when `spec.Class == ClassMirror`. `attachSecret` omits
`AttachSecretKey` from `Data` for a mirror, so the Secret's keys are exactly `{control}`. In `pod`,
for the mirror only, a sixth volume and mount join task 15's five:

```go
	if spec.Class == ClassMirror {
		volumes = append(volumes, corev1.Volume{Name: "mirror-disk", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: mirrorClaim(spec.ID)}}})
		mounts = append(mounts, corev1.VolumeMount{Name: "mirror-disk", MountPath: MirrorMountPath})
		ports = []corev1.ContainerPort{{Name: "control", ContainerPort: MirrorPort}}
	}
```

`ports` is the container's `Ports`, hoisted into a local as task 15 hoisted `volumes` and `mounts`
(`task-15.md:226-252`); the port number is phase 8's 8080 either way, only its name differs.
`run-panacea` and `snapshot` stay (task 15 gives every class both, and `panacea-mirror` writes
nothing to either); the scope's *"no token Secret"* holds because no volume projects the attach
Secret, whose one key is read only through `PANACEA_APP_CONTROL_TOKEN`'s `secretKeyRef`.

#### `mirror.go` (complete)

```go
package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MirrorOwner is the one owner view every mirror is created through
// (decision (e)). It can never equal an OwnerLabelValue (32 hex characters).
const MirrorOwner = "mirror"

// MirrorDiskSize is a mirror's project disk. A constant: resizing is a
// PVC expansion (allowVolumeExpansion, P8-11), not a spec field.
const MirrorDiskSize = "32Gi"

// MirrorStorageClass is P8-11's machine-disk class.
const MirrorStorageClass = "panacea-machine-disk"

// PruneOnLeave is decision (f): Q7's recommendation, not an owner decision.
const PruneOnLeave = true

// MirrorID is a project's mirror id: 20 hex characters of sha256("mirror/"+p).
func MirrorID(projectID string) string {
	sum := sha256.Sum256([]byte("mirror/" + projectID))
	return hex.EncodeToString(sum[:])[:20]
}

func mirrorClaim(id string) string { return "mirror-" + id }

// MirrorSpec is everything a mirror varies by. The caller (the broker, for
// Fleet) supplies the image, the rendered mirror policy and a fresh control
// token; the manager supplies the rest.
type MirrorSpec struct {
	ProjectID    string
	Image        string // repo@sha256:…
	PolicyFile   []byte // policy.Render(layers, model.ClassMirror)
	ControlToken string
}

// EnsureMirror creates the project's disk if absent, then its pod if absent,
// and returns the pod name. It must be called on WithOwner(MirrorOwner).
func (m *Manager) EnsureMirror(ctx context.Context, s MirrorSpec) (string, error) {
	if m.owner != MirrorOwner {
		return "", errors.New("sandbox: EnsureMirror needs the WithOwner(MirrorOwner) view")
	}
	if s.ProjectID == "" {
		return "", errors.New("sandbox: EnsureMirror: empty project")
	}
	id := MirrorID(s.ProjectID)
	v := m.WithLabels(map[string]string{"panacea-app/project": s.ProjectID}) // task 17's method
	if err := v.ensureClaim(ctx, id); err != nil {
		return "", err
	}
	name, err := v.Create(ctx, Spec{ID: id, Image: s.Image, Class: ClassMirror,
		PolicyFile: s.PolicyFile, ControlToken: s.ControlToken})
	if apierrors.IsAlreadyExists(err) {
		return PodName(m.owner, id), nil // the claim is held: the running mirror is the answer
	}
	return name, err
}

// ensureClaim creates the project disk once. An existing claim is kept: it
// holds every member's lanes, and a retry must never replace it.
func (m *Manager) ensureClaim(ctx context.Context, id string) error {
	size := resource.MustParse(MirrorDiskSize)
	sc := MirrorStorageClass
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: mirrorClaim(id), Namespace: m.cfg.Namespace,
			Labels: m.podLabels(Spec{ID: id, Class: ClassMirror})},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: size}},
		},
	}
	_, err := m.client.CoreV1().PersistentVolumeClaims(m.cfg.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("sandbox: mirror disk %s: %w", pvc.Name, err)
	}
	return nil
}

// DeleteMirror removes the pod, both Secrets and the disk. Absence of any is
// success. The disk goes last, so a failure part-way leaves the lanes, never
// a running pod over a deleted disk.
func (m *Manager) DeleteMirror(ctx context.Context, projectID string) error {
	id := MirrorID(projectID)
	if err := m.Delete(ctx, PodName(m.owner, id)); err != nil {
		return err
	}
	err := m.client.CoreV1().PersistentVolumeClaims(m.cfg.Namespace).Delete(ctx, mirrorClaim(id), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("sandbox: delete mirror disk: %w", err)
	}
	return nil
}
```

`corev1.VolumeResourceRequirements` is the PVC resources type from `k8s.io/api` v0.29 onward;
`feat/env` pins `k8s.io/api v0.31.1` (`control-plane/go.mod:7`, read 2026-09-25). `m.Delete(ctx,
podName)` is phase 8's (`manager.go:426`), which task 15 extends to the policy Secret.

#### The broker's mirror routes (`cmd/broker/mirrors.go`, wire shapes complete)

Task 17's decision 2 makes every Fleet request name a delegation. A mirror acts for **no** owner,
so these three routes are the one exception, and the only routes that accept Fleet's app-only
token *without* `X-Panacea-Delegation`; a request that carries one is `400 bad_request`, so no
delegation is ever bound to a mirror. A desktop token is `403 forbidden` on all three.

| Route | Request | Answer |
|---|---|---|
| `PUT /v1/mirrors/{projectId}` | `{"image": "<repo>@sha256:…"}` | `200 {"projectId", "pod", "state": "running"\|"starting"}`; `409 not_a_team_project`; `422 image_not_pinned`; `503 hub_unverifiable` |
| `DELETE /v1/mirrors/{projectId}` | — | `204`, also when nothing existed |
| `GET /v1/mirrors/{projectId}/control` | WebSocket upgrade, no body | `101`, then frames copied both ways, never decoded (task 17's relay, `task-17.md:423`); `404 mirror_not_found`; `409 mirror_not_running` |

```go
type mirrorPut struct {
	Image string `json:"image"`
}

type mirrorState struct {
	ProjectID string `json:"projectId"`
	Pod       string `json:"pod"`
	State     string `json:"state"` // running | starting
}
```

On `PUT` the broker, in order: checks the app-only Fleet claims (task 17 decision 1); asks the Hub
whether the project is shared with a team, as itself, cached at most 60 s like a delegation
(task 17 decision 3), and refuses `409 not_a_team_project` otherwise, so Fleet cannot create a pod
for an arbitrary id; renders the mirror policy with task 14's store (`policy.Render(layers,
model.ClassMirror)`); mints a fresh 32-byte control token, which it keeps in memory for the relay
and never returns; and calls `EnsureMirror` on its `panacea-mirrors` manager. The body carries
**no** project, owner, class or token: the path names the project and the broker supplies the rest
(global constraint 3). A pod that exists already keeps its token; the broker reads it back from the
attach Secret's `control` key when it restarts, which its Role in `panacea-mirrors` permits (gap 2).

`DELETE` calls `DeleteMirror`. Fleet calls it when the Hub's feed reports the project deleted
(task 11's feed), and never on a member leaving: that is `prune-owner` through `/control`.

#### The platform chart (complete)

`namespaces.yaml` renders a fifth `Namespace`, `panacea-mirrors`, with the same labels as the
other four (`part-of`, and `restricted` Pod Security for `enforce`, `audit` and `warn`,
`phase-8 …/task-11.md:92-99`) and **no** `panacea-app/edge-routes`. `networkpolicy-defaults.yaml`
already loops over the namespaces, so `default-deny` covers it with no edit; if it lists them by
hand, the fifth joins the list. `networkpolicy-mirrors.yaml`:

```yaml
{{- /* Task 58: the lane mirror. One allow; no egress rule of any kind, so the
       namespace's default-deny leaves it none, not even DNS (decision (a)). */}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mirror-ingress-from-broker
  namespace: panacea-mirrors
  labels:
    app.kubernetes.io/part-of: panacea-app
spec:
  podSelector:
    matchLabels:
      panacea-app/component: mirror
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: panacea-broker
          podSelector:
            matchLabels:
              app.kubernetes.io/name: broker
      ports:
        - protocol: TCP
          port: 8080
```

The `namespaceSelector` and `podSelector` sit in **one** `from` element, so both must match; two
elements would admit every pod in `panacea-broker` *or* any pod labelled `broker` anywhere, the
classic NetworkPolicy slip. `kubernetes.io/metadata.name` is set by the API server on every
namespace (Kubernetes 1.21+), so it cannot be forged by a label edit. How phase 8's
`session-ingress-from-broker` spells its selector is read at merge and matched.

`deploy/tests/netpolcover` (P8 task 13) gains one case, `mirror_has_no_egress`: it renders the
charts and fails if any `NetworkPolicy` whose `podSelector` could match `component: mirror` in
`panacea-mirrors` has `Egress` in `policyTypes` with a rule. A later chart that "just adds DNS"
fails the build.

#### `images/mirror/Dockerfile` (complete but for the two pins step 5 writes)

```dockerfile
# Task 58 (g): everything pinned. The base by digest; git by package version.
FROM mcr.microsoft.com/azurelinux/base/core:3.0@sha256:BASE_DIGEST_WRITTEN_IN_STEP_5
RUN tdnf install -y git-GIT_VERSION_WRITTEN_IN_STEP_5 && tdnf clean all \
 && git --version > /etc/panacea-mirror-git-version
COPY panacea-mirror /usr/local/bin/panacea-mirror
USER 1001:1001
ENTRYPOINT ["/usr/local/bin/panacea-mirror"]
```

The two tokens are replaced by literal values in step 5 and committed; a build with either token
still present fails at `FROM` or at `tdnf`, so an unpinned image cannot be built by accident.
That Azure Linux 3.0 is the right base (phase 8's sandbox image base, **UNVERIFIED** here) and that
`tdnf` accepts a `name-version` pin in this form are checked when step 5 runs. The binary is task
59's, built with `CGO_ENABLED=0` from the `sandboxd` module.

- [ ] **Step 1: Write the failing tests**

`control-plane/internal/sandbox/mirror_test.go`, on the fake clientset phase 8's tests use, with a
`mirrorSpec()` helper: project `pj-shop`, an image `acr.example/mirror@sha256:` + 64 `a`s, task
15's shared fixture as `PolicyFile`, a 43-character control token.

| Test | What it proves | The mutant it kills |
|---|---|---|
| `TestTheMirrorMountsNoMachineDisk` | the pod's volumes are exactly task 15's five plus `mirror-disk`, whose claim is `mirror-<MirrorID("pj-shop")>`; no volume names a machine claim; `Affinity` is nil | a mirror given a member's disk, or co-scheduled with one |
| `TestTheMirrorHasNoGatewayToken` | no env var matches `ANTHROPIC_*` or `PANACEA_APP_GATEWAY_*`; no volume or mount under `/run/panacea/gateway`; the rendered policy has no `apiKeyHelper` | a mirror able to call inference |
| `TestTheMirrorHasNoTokenSecret` | the attach Secret's keys are exactly `{control}`; no env `PANACEA_APP_TOKEN`; no volume projects any Secret but `<pod>-policy` | the attach token or a git token handed to the mirror |
| `TestAMirrorRefusesATaggedImage` | `…/mirror:1.2` → error, and the fake saw no create of any kind | git or the runner drifting under a moving tag |
| `TestAMirrorRefusesAMachineEnvAndAttachToken` | each of `Machine`, `Env{"A":"b"}`, `AttachToken` → error before any create | a worker's shape accepted as a mirror |
| `TestAMirrorRunsOnlyTheRunner` | `Command ""` → the container's command is `MirrorCommand`; `"sh"` → error | a shell as the mirror's process |
| `TestEnsureMirrorIsIdempotent` | two calls → one pod, one PVC, one attach Secret; the second returns the first's name | a second mirror, or a replaced disk that loses every lane |
| `TestAnExistingClaimIsKept` | a PVC `mirror-<id>` exists with a marker label → after `EnsureMirror`, the marker is still there | a claim deleted and recreated on retry |
| `TestEnsureMirrorNeedsTheMirrorView` | an owner view `WithOwner(OwnerLabelValue("t","o"))` → error | a mirror created under a person's owner label, visible to their desktop |
| `TestTwoProjectsGetTwoMirrors` | `pj-a` and `pj-b` → two pod names, two claims | a name derived from the truncated owner (decision (e)) |
| `TestDeleteMirrorRemovesTheDiskLast` | a fake reactor fails the pod delete → the PVC still exists; then success → pod, both Secrets and PVC are gone; a second call → nil | the disk deleted first, or absence reported as error |
| `TestTheMirrorIsLabelledMirror` | `panacea-app/component: mirror` on pod, Secrets and PVC; never `sandbox` | phase 8's session ingress rule matching a mirror |

`control-plane/cmd/broker/mirrors_test.go`: `TestMirrorRoutesAreFleetOnly` (a desktop token →
403 on all three); `TestMirrorRoutesRefuseADelegation` (Fleet with `X-Panacea-Delegation` → 400);
`TestMirrorPutNeedsATeamProject` (the Hub stand-in says solo → 409, no pod); `TestMirrorPutBodyIsImageOnly`
(an extra `projectId` field → 400 by `DisallowUnknownFields`); `TestMirrorTokenIsNeverReturned`
(the 200 body and every log line hold no byte of the control token).

`deploy/charts/platform/tests/networkpolicy_mirrors_test.yaml` (helm-unittest):
`mirror namespace is restricted` (the labels); `only the broker may reach a mirror` (one ingress
rule, one `from` element holding both selectors, port 8080 — kills the two-element slip); `the
mirror has no egress rule` (`policyTypes` is `[Ingress]` and `egress` absent).

- [ ] **Step 2: The manager.** Constants, the `validateClass` case, the three builder branches,
  `mirror.go`. Run `cd control-plane && go test ./internal/sandbox/ ./internal/backend/... -count=1`.
  Expected: `ok` twice, and task 15's tests still pass.

- [ ] **Step 3: The broker routes.** Run `go test ./cmd/broker/ -run Mirror -count=1 -v | grep -c
  "^--- PASS"`. Expected: `5`.

- [ ] **Step 4: The chart.** `helm unittest deploy/charts/platform` and `go test ./deploy/tests/netpolcover/...`
  from the repository root. Expected: every suite passes, `mirror_has_no_egress` among them.

- [ ] **Step 5: The image pins.** `docker buildx imagetools inspect
  mcr.microsoft.com/azurelinux/base/core:3.0 --format '{{json .Manifest.Digest}}'` gives the base
  digest; `tdnf list git` inside that base gives the version. Write both into the Dockerfile.
  `docker build images/mirror` must succeed and `docker run --rm <image> --version` print the pinned
  git version (task 59's runner prints it). Commit the Dockerfile with the pins in it.

- [ ] **Step 6: Acceptance**

```bash
cd control-plane && go vet ./internal/sandbox/ ./cmd/broker/ && go test ./internal/sandbox/ ./internal/backend/... ./cmd/broker/ -count=1 -race
cd .. && helm unittest deploy/charts/platform
grep -c "BASE_DIGEST_WRITTEN_IN_STEP_5\|GIT_VERSION_WRITTEN_IN_STEP_5" images/mirror/Dockerfile   # 0
```

**Deferred to task 69** (it needs the real cluster): that a mirror pod on a Kata node reaches no
address at all (a probe from inside the pod to the gateway, `github.com`, the Hub, IMDS and DNS,
each refused), that the broker reaches it on 8080, and that the PVC re-attaches after the node is
drained.

- [ ] **Step 7: Commit** by pathspec: `manager.go`, `mirror.go`, the tests, `backend.go`, the broker
  files, the chart files, the RBAC, the Dockerfile.

**What this task does NOT claim:**
- That Cilium enforces a policy with no egress rule as "deny all egress" on AKS's managed dataplane
  (training knowledge; task 69's probe).
- That 32 GiB, 500m and 1 GiB fit a real team project. They are starting values; task 69 measures.
- That the mirror is useful yet. It serves nothing until task 59's runner is in its image.
- That deleting a leaver's lanes is decided. It is Q7's recommendation (decision (f)).
- That the mirror's disk is backed up. Like machine disks, it is not in v1 (`phase-8 …/task-11.md:173`).

**Gaps found in the skeleton and in sibling parts:**
1. **Owner confirmation:** decision (b) is stricter than the scope and §15.2 (no fetch credential
   in the mirror). It moves the base fetch to `machined` (tasks 59, 60).
2. The broker needs a `Role` in `panacea-mirrors` (pods, Secrets, PVCs: create, get, list, delete)
   and a `panacea-mirrors` manager; phase 8 task 29's broker chart names only `panacea-sandboxes`.
3. Task 17's decision 2 (every Fleet request names a delegation) needs the mirror routes as its
   one written exception.
4. The skeleton's file map has no row for `mirror.go`, the broker's `mirrors.go` or
   `images/mirror/`.

