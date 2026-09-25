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
