# Review: Phase 8 — Our own environment

**Plan under review:** `docs/superpowers/plans/2026-09-23-phase-8-own-environment.md` and its 33 parts
in `2026-09-23-phase-8-own-environment-parts/`, on `feat/env` at `b17f43a`.
**Reviewer:** adversarial plan reviewer, 2026-09-23. Read-only except this file.
**Status:** IN PROGRESS — sections below are filled in as the review proceeds.

---

## Coverage

(pending)

## Strengths

(pending)

## Issues

Paths are relative to `docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/` unless
they start with a repo directory. Line numbers are the part file's.

### Critical

(being written — see the working notes at the end of this file)

### Important

(being written)

### Minor

(being written)

---

## Working notes (review in progress; folded into the sections above before the verdict)

- N1 (verified by running it). `terraform test` in Terraform 1.15.8: a test file's `variables`
  block **overrides** `-var-file`. Probe: a module with `variable "environment"`, a test file
  setting `environment = "dev"`, `terraform test -var-file=prod.tfvars` holding `"prod"`: the run
  saw `"dev"` (`Failure! 0 passed, 1 failed`, diff `- "dev" + "prod"`). Consequence for task
  01:60-63 (per-tfvars test runs) and task 02:126-135 (file-level variables set every tfvars key).
- N2. task-11.md:60-62 LimitRange sets only `max`; task-12.md:62-67 pre-pull init containers have
  no resources. Kubernetes defaults a Container LimitRange's `default` to `max` when absent, and
  `defaultRequest` to `default` (from training knowledge of `SetDefaults_LimitRangeItem`,
  UNVERIFIED here) → init containers get 8 CPU / 32Gi requests → DaemonSet pod unschedulable on a
  D8s_v5.
- N3. task-03.md:150-168: agents subnet has `default_outbound_access_enabled = false` and no NAT
  association (NAT only on `local.node_subnets` = system/apps/sandbox; test pins exactly 3); task
  07's firewall routes only node subnets. Check task 30 before concluding.
- N4. task-11.md:103-104 default-deny in all four namespaces incl. `panacea-edge`; task 13 renders
  no NetworkPolicy; grep of every part for NetworkPolicy found only task 29's broker and hub
  policies. Check task 29 before concluding.
- N5. task-07.md:96 `*.azurecr.io` on the ACR rule undoes the data-endpoint rationale of
  task-07.md:412.
- N6. Plan :294 says one variable `allowed_ingress_cidrs` feeds both the edge and the dev API
  server; task-07.md:153-157, :346-347 use a separate `api_authorized_ip_ranges`.
- N7. task-04.md:123-125 says task 10's allowlist names Cost Management Contributor; task-10.md:149-159
  does not.
- N8. P8-D6 (plan :132) says `managed-csi-premium` (LRS); task-11.md:65-67 uses `Premium_ZRS`.

## Assessment

(pending)
