# CLOUD-055: complete observation, unresolved responsiveness

The diagnostic coverage defect is corrected. This does **not** correct or pass
CLOUD-032 performance. Product remains the exact installed `d20ebd29` package;
the second diagnostic uses explicitly pinned helper source `edd0cb12`.

The original run retains 100 Home observations and 100 Define observations
censored by the 32-control cap. The first sample scanned the still-visible Home
before its click handler changed projects. The correction moves the existing
project-identity check ahead of editor scanning. Caps, actions, quotas, original
full-span timing, guards and thresholds are unchanged.

The second run records all 432 successful actions and 100 complete switch joins:
100 Home and 100 Define predicates, no missing/censored/overflow stages. Primary
switch p95 **522.380969 ms** and input p95 **110.087489 ms** both fail the unchanged
300/100 ms targets. The 100 resets and 32 trace follow-up actions remain separate.
No row is removed and no instrumentation cost is subtracted. Original result
SHA256 is `fcfa0cca877aac152ee449645caa068d26641f317e31979b63cdb000656d0ad3`.

The saved, enabled field was observed before the driver's later DOM marker in
all 100 switches. This is a structural observation, not first possible readiness,
paint, continuous actionability or pure transport overhead. Different slow rows
have delays before the Open click and inside the driver's verification awaits.
Narrower observation is needed before choosing a product optimization.

Preserved capsules:

- [Original frozen helper and controlled preparation](cloud055-readiness-original-20261009/README.md).
- [Original installed run, all rows and initial analysis correction](cloud055-readiness-original-analysis-20261009/README.md).
- [Exact two-line correction, original fixture-precondition failure and eight controlled checks](cloud055-readiness-correction-20261009/README.md).
- [Matching second installed diagnostic, all rows, joins and independent closure](cloud055-readiness-corrected-analysis-20261009/README.md).

Historical capsule prose describes the prospective state when it was frozen;
the second installed result is in the final capsule above. Earlier source tests
overlap and are not cumulatively counted. Six offline analyzer tests belong to
the first analysis; their unchanged algorithm is carried forward, not rerun.

Independent checks observe the exact recorded driver/controller identities
absent, the same Docker engine/image with empty containers and volumes, and
unchanged bounded artifacts. They do not establish a browser PID census or
whole-machine closure. The two PID-1 Chromium zombies from sustained attempt8
remain separate recorded facts. No historical charge or failed fixture changed.
The d20 archive was independently checked before its old release directory was
preserved for the edd0 build; later content at the original path is not evidence
for the old run. Prior Rust relocation moved bytes into memory-backed storage,
not a whole-host capacity improvement.
