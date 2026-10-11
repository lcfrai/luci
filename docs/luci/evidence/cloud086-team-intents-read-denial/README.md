# CLOUD-086 cached-read denial follow-up

Independent inspection of the adjacent coordinator policy prompted a meaningful regression in the new team-intent component: with cached `canRead=true`, a later coordination GET401/403 left cached private intent and retained decision-draft presentation visible. The original production component is retained as `before.tsx.txt` (from commit `066a7bb5103e828ed4ca8fa6993c3171a181322a`). No installed occurrence is claimed.

The unchanged new cases in `CoordinatorTeamIntents.test.tsx` reproduce **two original failures**, with14 unrelated component cases filtered. The correction immediately removes cached intent/draft presentation on read denial while preserving the exact unresolved browser request. A later authorized exact readback restores the current view, settles only that recorded result and never resends the mutation. A failure reading fresh state after a confirmed mutation is also handled separately from the mutation acknowledgement.

Final affected command: **31 passes across the component/API files, 0 failures/skips**. These overlap the earlier29 passes. Four selected source/test bindings remained unchanged through that run. This is controlled React/API fixture evidence, not native/browser/real-model/team/deployment acceptance or complete compiler provenance. TypeScript was not repeated for this follow-up; the prior zero-exit result belongs to the earlier owned slice, and integrated typechecking remains with the parent validation.

Commands run from the repository root with task-local Node24.19:

```sh
node --max-old-space-size=512 desktop/node_modules/vitest/vitest.mjs run --root desktop src/components/orchestrator/CoordinatorTeamIntents.test.tsx -t 'hides cached private intents' --maxWorkers=1 --reporter=json --outputFile=../docs/luci/evidence/cloud086-team-intents-read-denial/before-tests.json
node --max-old-space-size=512 desktop/node_modules/vitest/vitest.mjs run --root desktop src/api/coordinatorIntents.test.ts src/components/orchestrator/CoordinatorTeamIntents.test.tsx --maxWorkers=1 --reporter=json --outputFile=../docs/luci/evidence/cloud086-team-intents-read-denial/final-tests.json
```

Setup and parent component props are unchanged. `SHA256SUMS` covers the capsule files except itself. CLOUD-086 and full lifecycle acceptance remain open.
