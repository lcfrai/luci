# Carried plan issues — task 19 (Helm chart, NetworkPolicies, Key Vault CSI)

Residual findings the OD-31 convergence ruling carried into the build. Fix each in your
task; say in your task report what you did with it.

---

## b05 Minor 2 — `keysmith-ingress` is now load-bearing and nothing tests its peer list

Source: `.superpowers/sdd/p6/plan-rereview-b05-2.md:59-66`, "Minor 2".

> **2. `keysmith-ingress` is now load-bearing and nothing tests its peer list.** `task-19.md:100-160` is the
> whole of `render_test.sh`: it pins `gatekeeper-ingress` to sandbox pods on 4000 (`:117-121`) and
> `litellm-ingress` to exactly `gatekeeper` and `keysmith` with no `namespaceSelector` (`:122-125`), and it
> has **no** assertion on `keysmith-ingress`. Round 2's whole argument — *"the Hub's cloud spend read stays
> refused at keysmith however much phase 8 opens"* (`task-19.md:3`) — rests on a property no test kills. A
> render that admitted `- {}` on 4100 would pass. Defence in depth only (keysmith still authorises by
> `TokenReview` and the `kube` caller table), which is why this is Minor. Fix, about 5 minutes: the
> `litellm-ingress` block copied for `keysmith-ingress`, wanting the broker peer and `gatekeeper`.

The policy itself is at `task-19.md:711-734` (name at `:714`, `policyTypes: [Ingress]`, peers
= the `.Values.broker` namespace + pods and `app.kubernetes.io/name: gatekeeper` on 4100),
verified in `.superpowers/sdd/p6/plan-rereview-b05-2.md:13`. Assert exactly that peer set —
an assertion that merely finds the policy present would not kill the `- {}` mutant.
