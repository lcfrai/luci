# Held tasks — phase 6 gateway

A task is HELD when a dependency outside this branch has not landed. Do not dispatch a held
task: its implementer would either stub the missing dependency or invent it, and both cost
more than the wait. Move a task out of this file only when the admission test below passes
in this worktree.

---

## 08a — `cmd/keysmith` caller authentication — **HELD**

**The plan's own dependency line** (`docs/superpowers/plans/2026-09-25-phase-6-gateway.md:173`):
task 08a depends on `03; **env 17 step 0 merged** (`internal/oidcauth`, cross-tree; OD-30
review-1 M5)`. `task-08.md:11` repeats it, and `task-08.md:14` says 08a "runs in wave 3
beside task 05 **once env 17 step 0 is merged**". The wave list puts it in wave 3 with the
same parenthesis (skeleton `:204`).

**What 08a needs from that stream** (`task-08.md:49`, verbatim): env task 17 step 0 is the
rename of the built `control-plane/internal/entra`, giving
`oidcauth.New(Config) (*JWKSValidator, error)`, `oidcauth.Validator`,
`Config{Profile, Issuer, TenantIDs, Audience, RequiredScope, AllowedClients, ServiceClients}`,
`Claims{TenantID, ObjectID, ClientID, Service}`, `(Claims).Key()`, `ProfileKeycloak`,
`ErrKeySetUnavailable`, `ErrSignature`. Task 08's step 0 gate is
`grep -n 'ProfileKeycloak\|ServiceClients\|Service  *bool' internal/oidcauth/*.go`, expecting
all three (`task-08.md:87`). Its test file imports the package directly
(`task-08.md:748`, `:774`, `:779`).

**Verified in this worktree, 2026-09-28:**

| Check | Command | Result |
|---|---|---|
| the package | `ls -d control-plane/internal/oidcauth` | **does not exist** |
| the package it is renamed from | `ls -d control-plane/internal/entra` | **does not exist either** |
| the ruling it is built to | `ls docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` | **not in this worktree** (it is in the main tree) |
| what has been merged onto `feat/phase-6-gateway` | `git log --merges --oneline` | `0adbd763` `feat/phase-2-cockpit`, `f898f5e8` `fix/sandbox-git-ca` — and nothing else |

So neither the package nor its predecessor nor its ruling is reachable from this branch, and
no merge on this branch could have brought them.

**Admission test — all four must pass in this worktree before 08a is dispatched:**

1. `ls -d control-plane/internal/oidcauth` exits 0.
2. `grep -n 'ProfileKeycloak\|ServiceClients\|Service  *bool' control-plane/internal/oidcauth/*.go`
   returns all three names — task 08's own step 0 gate, `task-08.md:87`.
3. `go build ./control-plane/...` exits 0 with the package present.
4. `ls docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` exits 0, so 08a's
   implementer can read §5 and §6 rather than infer them from `task-08.md`.

**Who lands it, and how.** The env stream owns task 17 step 0. This branch takes it by a
merge from `feat/env` once that step is merged there — not by copying the package in, which
would fork the rename the ruling calls for. That merge is not this run's to make: rule 1
forbids it here, and the main tree is running a live defect-fix wave.

**Cost of the hold, from the plan's own schedule** (skeleton `:198`): 08a has **7 hours of
slack before it delays task 17**, and 08b — which depends on 08a — has 2.5 hours of slack
before 17. Nothing else waits on it. So the hold is free until the env merge is more than
about 7 focused hours away from wave 3.

**Not held:** every other task on the 27-row table. 08b is blocked only by 08a and by tasks
02, 07, 09 and 13, all of which are in this tree.
