### Task 47: Phase 8 delta: AU egress allowlist, the Australia East guard, task 32's five probes

**Slice TM1. Depends on: 32, P8-32. About 3 hours. Seam: P8-D12.**

Three changes to phase 8, each the smallest that makes a `05-design.md` §2.5 promise checkable. It
is a delta on `feat/env` (worktree `.claude/worktrees/env`, read at `9ef70c3`), built after that
branch's sync-merge into `feat/team-mode`.

1. **AU sandboxes reach package registries and the git hosts, and nothing else.** An AU project
   keeps inference onshore through the gateway, but an agent with open internet can still `curl`
   the project's text anywhere (§2.5, `05-design.md:223-225`). Phase 8's platform chart (task 11)
   today gives **every** sandbox pod `ipBlock 0.0.0.0/0` minus IMDS and the private ranges
   (`task-11.md:155-159`, planned). This task splits that allow by a pod label the broker stamps,
   and gives AU session pods an FQDN allowlist instead.
2. **Nothing team mode runs sits outside Australia East** (§2.5 invariant, `05-design.md:228-231`).
   A new rule **G11** in phase 8's `guards.sh` (built, `infra/scripts/guards.sh`, ten rules today)
   fails any Terraform or chart value that places a region elsewhere.
3. **Five facts the design rests on get a measuring script** (`05-design.md:1154`, `:1356-1373`),
   as rows of phase 8 task 32's checklist, plus one this task needs for item 1.

**The one thing this task must not get wrong is the direction of the default.** A pod the broker
did not label must get **less** egress, never more. So the internet allow selects pods labelled
`panacea-app/route: global` explicitly; an AU pod is labelled `au`; a pod with no route label (a
bug, a hand-made pod, a future component) gets DNS and nothing else. The broker stamps `global`
when a create names no route, which keeps every phase 8 solo session exactly as it is.

**The second thing is that an FQDN allowlist on AKS is itself unverified.** Standard
`NetworkPolicy` matches addresses, not names, and registries sit behind CDNs. A name allowlist needs
Cilium's `toFQDNs`, which on AKS's managed Cilium needs Advanced Container Networking's security
feature (`network_profile.advanced_networking.security_enabled`, azurerm 4.81.0, named by phase 8's
Q1 third option, `phase-8-own-environment.md:306`; that it enforces `toFQDNs` is from training
knowledge, **UNVERIFIED**). Phase 8 does not build it. So this task turns it on for the cluster, and
adds **P28**, which proves an AU pod reaches the allowlist and nothing else. If P28 fails, AU
projects fail closed: the policy still denies the internet, so agents lose registries, not
isolation.

**Where the files are.** Phase 8 task 32's scripts live in `infra/smoke/proving/` and its record in
`docs/verification/phase-8-kata-proving.md` (`task-32.md:17-19`), **not** `infra/smoke/kata/*` as
the skeleton says (gap 1). Its rows are P01–P16 and P20–P22; this task adds P23–P28.

**Files:**
- `deploy/charts/platform/templates/networkpolicy-sandboxes.yaml` (phase 8 task 11, planned):
  `egress-internet` narrowed to `route: global`, `egress-internet-infra` for machines and balloons,
  and `CiliumNetworkPolicy/egress-au-allowlist` (modify, **UNVERIFIED until merge**)
- `deploy/charts/platform/values.yaml`, `values.schema.json` (task 11): `sandboxes.auEgress`
  (modify)
- `deploy/charts/platform/tests/netpol_au_test.yaml`: helm-unittest suite (new)
- `cp/cmd/broker/*` (phase 8 tasks 17–18, planned), `cp/internal/brokerapi` (task 17): 
  `SandboxSpec.Route`; the broker stamps `panacea-app/route` on every session pod (modify,
  **UNVERIFIED until merge**)
- `cp/internal/fleet/sessions/sessions.go` (task 20): sets `Route` on the spec it already builds
  (`task-20.md:270-284`) (modify: one field)
- `infra/terraform/modules/aks/*` (phase 8 task 06, planned): `advanced_networking` with
  `security_enabled = true` (modify, **UNVERIFIED until merge**)
- `infra/scripts/guards.sh`, `guards_test.sh` (phase 8 task 01, built): G11, and the summary's
  `of 10` becomes `of 11` (modify)
- `infra/smoke/proving/{p23-orchestrator-size,p24-guest-write-sweep,p25-source-addr,p26-team-launch,p27-pty-round-trip,p28-au-egress}.sh`
  and six cases in `proving_test.sh` (phase 8 task 32, planned) (new / modify)
- `docs/verification/phase-8-kata-proving.md` (task 32): six rows, each `NOT RUN` (modify)

**Interfaces:**
- Consumes: phase 8 task 11's `networkpolicy-sandboxes.yaml`, its labels
  `panacea-app/component: sandbox | machine | balloon` (`task-11.md:147-159`); task 17's broker
  labels and the `panacea-app/source-addr` annotation (`task-17.md:70-87`); task 20's `route`
  (`task-20.md:284`); task 32's (this plan) allow from sandbox pods to the gateway's service port
  and the Orchestrator class's egress, which stay as task 32 writes them; phase 8 task 32's
  `proving_test.sh` style (`task-32.md:48-54`).
- Produces: the pod label `panacea-app/route: global | au`; `SandboxSpec.Route`; G11; the values
  block `sandboxes.auEgress`; checklist rows P23–P28.
- Required by: task 57 (TM1 proof reads P23–P28's results), task 33's runbook (runs P23 and P25
  during the scripted conflict), task 56 (the route badge's `AU · required` is only honest if P28
  passed; the badge task states so).

#### The chart — complete source

```yaml
# deploy/charts/platform/values.yaml (append under sandboxes:)
  auEgress:
    # 05-design.md §2.5: an AU project's session pods reach these names on 443 and nothing
    # else on the internet. Package registries and the two git hosts (OD-4) only. A name here
    # is exact (matchName); a pattern is one leading "*." label (matchPattern). No bare "*".
    fqdns:
      - registry.npmjs.org
      - registry.yarnpkg.com
      - pypi.org
      - files.pythonhosted.org
      - proxy.golang.org
      - sum.golang.org
      - crates.io
      - static.crates.io
      - index.crates.io
      - repo.maven.apache.org
      - api.nuget.org
      - github.com
      - codeload.github.com
      - objects.githubusercontent.com
      - dev.azure.com
    patterns:
      - "*.visualstudio.com"
```

`values.schema.json` gains `sandboxes.auEgress` as a required object, `additionalProperties:
false`; `fqdns` a non-empty array of `"pattern": "^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$"`;
`patterns` an array of `"pattern": "^\\*\\.[a-z0-9-]+(\\.[a-z0-9-]+)+$"`. So `*`, `*.com` and a
name with a path are refused at `helm lint`. **The list is a starting set from knowledge; which
registries AU projects need is the owner's call (gap 3).** It has no model provider: an AU
sandbox reaches inference only through the gateway's private path (task 32), never
`api.anthropic.com`, which is the Global route.

```yaml
# deploy/charts/platform/templates/networkpolicy-sandboxes.yaml (replace egress-internet)
{{- $n := .Values.namespaces.sandboxes }}
{{- define "platform.internetExcept" }}
          except:
            - 169.254.169.254/32
            - 168.63.129.16/32
            - 10.0.0.0/8
            - 172.16.0.0/12
            - 192.168.0.0/16
{{- end }}
---
# Machines and balloons: unchanged from phase 8 task 11. machined fetches with the project's
# read-only credential (task 48); it runs no agent code.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: egress-internet-infra, namespace: {{ $n }} }
spec:
  podSelector:
    matchExpressions:
      - { key: panacea-app/component, operator: In, values: [machine, balloon] }
  policyTypes: [Egress]
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
{{- include "platform.internetExcept" . }}
---
# Session pods of a Global project, and every solo session (the broker stamps "global" when a
# create names no route). A session pod with no route label matches nothing here.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: egress-internet, namespace: {{ $n }} }
spec:
  podSelector:
    matchLabels: { panacea-app/component: sandbox, panacea-app/route: global }
  policyTypes: [Egress]
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
{{- include "platform.internetExcept" . }}
```

Task 11's `each of network.* if it lies outside those` entries of the `except` list are kept in
`platform.internetExcept` exactly as task 11 renders them (the helper above shows the five fixed
entries only). `networkpolicy-defaults.yaml`, `allow-dns` and both ingress policies are untouched.

```yaml
# deploy/charts/platform/templates/networkpolicy-sandboxes.yaml (append)
---
# An AU project's session pods: DNS through Cilium's DNS proxy (so it learns name -> address),
# then 443 to the allowlist only. Task 32's gateway allow and the Orchestrator class's egress
# are separate policies and still apply; Kubernetes and Cilium policies are a union of allows.
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata: { name: egress-au-allowlist, namespace: {{ $n }} }
spec:
  endpointSelector:
    matchLabels: { panacea-app/component: sandbox, panacea-app/route: au }
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports: [ { port: "53", protocol: ANY } ]
          rules:
            dns: [ { matchPattern: "*" } ]
    - toFQDNs:
{{- range .Values.sandboxes.auEgress.fqdns }}
        - matchName: {{ . | quote }}
{{- end }}
{{- range .Values.sandboxes.auEgress.patterns }}
        - matchPattern: {{ . | quote }}
{{- end }}
      toPorts:
        - ports: [ { port: "443", protocol: TCP } ]
```

The DNS rule allows every lookup and resolves nothing it does not allow to connect to: a name
off the list resolves, and the connection to its address is dropped. That the DNS rule must be in
the same policy as `toFQDNs` for Cilium to populate the name cache is from Cilium's documentation
as remembered, **UNVERIFIED**; P28 checks it.

**The broker stamps the label** (phase 8 tasks 17–18, planned; built after the merge):

```go
// control-plane/internal/brokerapi (modify: one field on SandboxSpec)

	// Route is the project's inference route, "global" or "au" (05-design.md §2.5).
	// The broker stamps it as the pod label panacea-app/route, which selects the
	// pod's egress policy. "" is "global": a solo create from remotebe names no
	// route, and keeps phase 8's egress exactly. Any other value is 400
	// bad_request, details.field "route", and no pod is created.
	Route string `json:"route,omitempty"`
```

The label is set by the broker from the spec, never by a caller-supplied label map: this task
makes task 17's `WithLabels` (`task-17.md:325`) refuse the key `panacea-app/route`, so Fleet cannot
spell `global` for an AU project by a second path. Fleet sets `Route` from the value it already
reads through `gwire.RouteSource` before every create (`task-20.md:284`), and a route source error
still means no create. **A solo AU project's sessions still get `global` egress** until remotebe
sends `Route` (gap 2).

**The cluster feature** (phase 8 task 06's `modules/aks`, planned):

```hcl
# infra/terraform/modules/aks/main.tf (modify: inside network_profile)
    advanced_networking {
      observability_enabled = false
      security_enabled      = true   # Cilium FQDN policy for AU sandboxes (Phase T task 47)
    }
```

The block's shape is azurerm 4.81.0's, as phase 8's Q1 names it; the pinned provider version at
merge decides, **UNVERIFIED**. ACNS is billed per node (from knowledge, **UNVERIFIED** amount); the
cost goes in phase 8's cost model row as an owner-visible line (gap 4).

#### G11, the Australia East guard — complete source

`guards.sh` runs from the repository root after its two `cd`s (`guards.sh:29-31`), so `deploy/`
is a sibling of `$BASE`. G11 reads two things. **Terraform:** one new tag from the existing awk
pass, so no second process. **Chart values:** one `grep` over `deploy/values/**/*.yaml` and
`deploy/charts/*/values.yaml`, captured whole and its status tested, as every read is
(`guards.sh:22-25`). G11 is the first rule that reads outside `infra/`; the header comment says so.

```bash
# infra/scripts/guards.sh — the awk SCAN program gains (beside the TFVKEY rule, :179):
#   LOC <file> <attr> <value>   a location attribute set to a string literal            (G11)
(is_tf || is_vars) && /^[ \t]*([a-z_]*_)?location[ \t]*=[ \t]*"/ {
  a = $0; sub(/^[ \t]*/, "", a); sub(/[ \t]*=.*/, "", a)
  v = $0; sub(/^[^"]*"/, "", v); sub(/".*/, "", v); print "LOC\t" cur "\t" a "\t" v
}
(is_tf) && /^variable[ \t]+"([a-z_]*_)?location"/ { in_locvar = 1 }
(is_tf) && in_locvar && /^}/ { in_locvar = 0 }
(is_tf) && in_locvar && /^[ \t]*default[ \t]*=[ \t]*"/ {
  v = $0; sub(/^[^"]*"/, "", v); sub(/".*/, "", v); print "LOC\t" cur "\tdefault\t" v
}
```

```bash
# infra/scripts/guards.sh — the tag loop gains  LOC) LOCS+=("$rest") ;;  and, before the summary:

# --- G11: every region is Australia East (Phase T task 47; 05-design.md §2.5) ----------------------
# Fleet, its Postgres, the lane mirror, the gateway and its usage store, and every sandbox pool
# run in Australia East. The rule is wider than that list on purpose: one estate, one region, so
# a new component cannot be missed by name. "global" is the literal some Azure resources take
# for a control-plane object with no data location (from knowledge, UNVERIFIED per resource).
# The AU route's Bedrock region is AWS's name and is pinned too: ap-southeast-2 (Sydney).
REGION="australiaeast"
begin 11
for l in "${LOCS[@]}"; do
  f="${l%%"$TAB"*}" r="${l#*"$TAB"}" attr="${r%%"$TAB"*}" v="${r#*"$TAB"}"
  case "$v" in "$REGION"|global) ;; *) fail "$f" "$attr = \"$v\"; team mode runs in $REGION only" ;; esac
done
VALS="$(find deploy/values deploy/charts -name '*.yaml' \( -path 'deploy/values/*' -o -path 'deploy/charts/*/values.yaml' \) 2> /dev/null | sort)"
if [ -n "$VALS" ]; then
  HITS="$(grep -HnE '^[[:space:]]*(location|azureRegion|topology\.kubernetes\.io/region|bedrockRegion):' $VALS)"
  rc=$?; [ "$rc" -le 1 ] || read_failed "values scan" grep "$rc"
  while IFS= read -r h; do
    [ -n "$h" ] || continue
    file="${h%%:*}" rest="${h#*:}" rest="${rest#*:}"
    key="${rest%%:*}" key="${key//[[:space:]]/}" v="${rest#*:}" v="${v//[[:space:]\"\']/}"
    want="$REGION"; [ "$key" = bedrockRegion ] && want="ap-southeast-2"
    [ "$v" = "$want" ] || [ -z "$v" ] || fail "$file" "$key: $v; team mode runs in $want only"
  done <<< "$HITS"
fi
end

echo "guards: $PASSED of 11 passed"
[ "$PASSED" -eq 11 ]
```

An empty value (`location: ""`, a pipeline-filled slot) passes; the value the pipeline passes is
the Terraform output, which G11 has already pinned. `bedrockRegion` is phase 6's key as this part
names it; **phase 6's gateway chart is not written**, so the key is renamed mechanically to
phase 6's at merge (gap 5). The `$VALS` word-split is deliberate: chart paths hold no spaces
(phase 8's naming rule), and `find … | sort` feeds it once.

#### The six checklist rows

Appended to `docs/verification/phase-8-kata-proving.md` in phase 8 task 32's columns: row, the
fact, the decision it guards, the script, what changes if it is false. Every result cell is
`NOT RUN`. Each script splits into a **collector** (talks to the cluster, owner-gated like the rest
of task 32, `task-32.md:3-9`) and a **verdict** function that reads only the collector's saved
output, so `proving_test.sh` runs every verdict on canned text with no cluster.

| Row | Fact | Guards | Script and method | If false |
|---|---|---|---|---|
| P23 | An Orchestrator pod's real peak under TM1-α load stays inside the `orchestrator` class's limits with 25% headroom | task 15's class shape | `p23-orchestrator-size.sh`: during task 33's scripted conflict, `kubectl top pod -n panacea-sandboxes -l panacea-app/class=orchestrator --no-headers` every 10 s; verdict: per pod, max CPU `m` and memory `Mi` against the limits read from the pod spec | task 15's `orchestrator` request and limit are raised to the measured peak ÷ 0.75, and phase 8's quota row follows |
| P24 | A file a worker writes inside the guest shows in Fleet's activity within one sweep (≤ 10 s, task 34) | §3.3 rule 4; task 34's sweep outside the VM | `p24-guest-write-sweep.sh`: in a worker session, write `p24-<nonce>.txt`; poll `panacea files --json` from the owner's Orchestrator every 1 s for 30 s; verdict: seconds to first sight, over 5 runs, all ≤ 10 | the shared-disk view from the machine pod lags the guest (virtiofs caching, P03's disk); task 34's sweep reads through `sandboxd`'s own `stat` instead, and §3.3's "observed outside the VM" is re-argued |
| P25 | The source address the gateway sees for a session pod equals the pod IP the broker recorded | §2.5 token binding; task 17's `source-addr` | `p25-source-addr.sh`: from a session pod, one gateway call with its token; read the gateway's access-log line for that request id and the pod's `panacea-app/source-addr` annotation; verdict: equal, or the two addresses printed | the gateway binds tokens to a client certificate the broker issues per pod (mTLS), the design's stated fallback (`05-design.md:1357`); task 20 and phase 6's keysmith change |
| P26 | A team launch from the warm pool reaches the worker's first output byte in under 10 s | §10.2 | `p26-team-launch.sh`: 10 launches through task 21's team route, each timed from the request to the first attach frame with output; verdict: median and max, pass if max < 10 s | the number is reported as measured on the launch surface (no "instant" claim anywhere); balloons and the warm floor are resized by P06's rule |
| P27 | The PTY round trip desktop → local control plane → Fleet → broker → `sandboxd` is under 150 ms at p95 from Australia | §10.3 | `p27-pty-round-trip.sh`: from a laptop on the KPMG network in Australia, through `fleetbe`, 200 single-byte writes to `cat` with a 250 ms gap, each timed to its echo; verdict: p50, p95, max | the relay hop that costs most (read from Fleet's and the broker's per-frame timing logs) is removed or co-located; if none can be, §10.3's budget is restated with the measured number |
| P28 | An AU session pod reaches every allowlisted name on 443 and nothing else | this task's `egress-au-allowlist`; the ACNS flag | `p28-au-egress.sh`: in an AU-labelled throwaway session pod, 3 s probes to `registry.npmjs.org`, `github.com`, `dev.azure.com` (must connect) and `example.com`, `api.anthropic.com`, `1.1.1.1:443`, `169.254.169.254` (must not); verdict: fails on the first wrong answer and names it | FQDN policy is not active on the cluster: AU projects keep failing closed (no registries); the owner chooses between ACNS, the firewall mode of phase 8's Q1, or an in-cluster allowlisting proxy |

P23 to P27 are the five of `05-design.md:1154`; **P28 is this task's own**, because an allowlist
nobody has watched deny anything is the exact kind of assumed fact task 32 exists to test
(`task-32.md:11-15`). P23 and P25 run inside task 33's runbook, P24 after task 34 lands, P26 and
P27 after task 21, and P28 before any AU project is created.
