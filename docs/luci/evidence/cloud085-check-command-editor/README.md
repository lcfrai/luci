# Shared check command editor extraction

The existing ProjectChecks command list and Advanced check commands form are now reusable by the coordinator. This is presentation extraction only: ProjectChecks still owns its authority/pending guard, JSON parsing, error state, trust consent and durable draft edits. Both disabled flags remain separate because the original flow permits editing fields while command actions are blocked by an active check.

The component emits raw strings and an add callback; callers must implement their own submission authorization, parsing and consent. It does not start checks or grant trust. Existing shared Button, labels, classes, disclosure and required input behavior are retained.

## Verification

- Focused single-worker Vitest: 66/66 passed, zero failures; 5 new editor tests and 61 unchanged ProjectChecks tests. Command and exit 0 are in test-exit.json; full output is tests.log.
- Eight selected source/test/runner inputs were identical before and after. These are bounded bindings, not an exhaustive compiler/dependency manifest.
- No separate TypeScript run in this extraction capsule; the parent will perform its shared integrated typecheck.
- No browser/native, Docker, provider, accessibility, responsiveness, coordinator delivery or full acceptance result is claimed.

Shared-checkout base while recording inputs: `e3b7d5212db4455315c98d46a4703a5cff6813a7`. The captured ProjectChecks changes and new editor/tests were uncommitted during this scoped run. Other agents' changes are not represented as tested by these two files.
