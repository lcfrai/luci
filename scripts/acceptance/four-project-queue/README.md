# Installed four-project acceptance helpers

These controlled fixtures exercise real local software. They do not measure live
model reasoning, production application quality, or Windows responsiveness.

- [Runtime workflow](RUNTIME-WORKFLOW.md): installed browser, four real Docker/Codex
  tasks, frozen context, natural task exits, shared capacity, recovery and human
  review/integration. Provider replies are deliberately scripted.
- [Home controls](HOME-CONTROLS.md): native browser queue controls in an isolated
  paused workspace with an unavailable Docker endpoint.
- [Resource measurement](RESOURCE-MEASUREMENT.md): independent owned-process and
  container observations, with explicit scope and missing metrics.

Each attempt must use a fresh report directory. Preserve failures and exact
package/source/image identities; do not select favorable timing results. Never
copy raw private fixture state or credentials into Git or a public report.
