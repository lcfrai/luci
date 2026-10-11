# Luci workspace surface store

Adds a runtime-only Luci/tools presentation choice. Luci is the default, and showLuci changes presentation without changing the selected project, phase, draft registration or pending navigation. Selecting the same phase reveals its retained editor without a discard request. Different phase/branch destinations select tools only through existing navigation guards; plain project selection preserves the current surface. Changed account/workspace navigation scope returns to Luci. Focus-only session/worktree updates do not change the surface. No persisted field or version migration was added.

## Scoped verification

The four-file single-worker Vitest command in test-exit.json passed 85/85 cases, exit 0. This includes the new presentation/draft/scope tests plus existing source-navigation, project-navigation and store tests. All 14 selected source/test/runner bindings were unchanged across the run. This is a bounded input manifest, not an exhaustive compiler/dependency/source-tree binding. The raw log retains the existing jsdom HTMLCanvasElement getContext diagnostic; it did not fail an assertion. There was no earlier failed run in this slice.

The read-only resource guard allowed this source-test batch with about 8.99 GB visible cgroup memory headroom and 676.8 MB workspace disk space; it is neither atomic product admission nor a performance measurement. No separate TypeScript command, app/browser/native/Docker/provider work or latency test ran in this capsule. Parent integration will verify the actual primary-pane presentation and compile the combined UI. These tests do not establish a personal cross-project coordinator or full product acceptance.

Shared-checkout base at test capture: `5ebf08170cc79b32e0f04d3383f80ab528e47d31`. Only uiSlice.ts, luciSurface.test.ts and this capsule belong to this change; other agents' product work is outside this scoped result.
