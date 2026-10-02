# Luci project context

Luci is lcfr.ai's open-source application for Define → Design → Develop → Deploy. `docs/luci/PLAN.md` is the active scope and `docs/luci/API.md` the implementation contract.

- **Project**: an ordinary Git repository under the user's chosen projects folder. All four stages share its stable path-derived ID.
- **Brief**: a versioned purpose, audience, constraints and acceptance criteria. `LUCI.md` carries the current brief into coding sessions as untrusted project context.
- **Design**: an embedded Excalidraw scene, saved with the brief revision it was based on.
- **Worktree**: a checkout resolved through Git, with one or more coding or shell sessions.
- **Session**: an isolated Docker agent/shell process, live terminal and durable transcript. Local containers share the host kernel.
- **Release**: a record pinned to a brief revision and committed source. A deployment is only ready after a health check succeeds.
- **Deployment target**: a verified connection to a user's Coolify server, including its stable server identity. A cloud machine can be provisioned in the user's AWS or Azure account.
- **Cloud plan**: a reviewable resource template and quote assumptions. It is not evidence of a deployment.

The local default needs no organisation login, cloud database or Kubernetes cluster. Users configure their own Git identity, agent credentials and optional cloud account. Secrets are excluded from API read responses and project source.

Luci's own code is MIT licensed; dependencies retain their original licences. Local use has no Luci licence or platform fee. Keep cloud spending optional, choose the smallest viable machine for the intended workload, and review current provider quotes and separately billed model usage. A configured budget is a planning constraint, not an enforced provider spending limit.

Historical terminology and corporate planning are archived in `docs/legacy/` and `docs/superpowers/`; they do not govern the Luci build.
