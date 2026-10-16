# LOCAL-106 and Codex auth source check receipt

Source ancestry: reviewed Distribute/LOCAL-096/099 join `864eae7484c5b54232018b5d9652a85079b16b41`; published reviewed Codex auth tree `3210e4048f9f49123ae8f285578d9629f0e3bfac`, joined as `099dcb5ad`. This candidate adds the bounded second-repair review correction. All checks below are offline source fixtures; no provider, Docker, model, account or cloud action was run.

Working directory for Go: `control-plane`. Common environment: `GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOTMPDIR=/dev/shm TMPDIR=/dev/shm`; tool: `/dev/shm/luci-toolchain/go/bin/go`, `-p 1`.

| Exact command suffix | Result |
| --- | --- |
| `go test -p 1 ./internal/product -run '^TestCoordinatorWaitsForCandidateThenBindsReservedReviewerBeforeDispatch$' -count=1` | PASS, two subcases, 13.242s. Final second-repair pass enters the real reviewed integration handler; final third finding refuses automatic and HTTP checkpoint repair without another queue item or run revision. |
| `go test -p 1 ./internal/api -run '^TestCodexAccount(ErrorsDistinguishGrantStorageAndUnconfirmedSave\|AdoptionDistinguishesMissingIncompleteAndUnreadableState)$' -count=1` | PASS, 0.028s. |

Working directory for UI: `desktop`, `TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768`.

| Exact command | Result |
| --- | --- |
| `npm test -- --maxWorkers=1 src/api/codexConnectionErrors.test.ts src/components/develop/CodexSignInPanel.test.tsx src/components/setup/AgentConnections.test.tsx` | PASS, 3 files, 18 tests, 5.27s. |
| `npm run typecheck` | **UNVERIFIED**: process killed with exit 137. The shared cgroup had 33,704,751,104 bytes charged of 34,359,738,368 bytes after this attempt. No TypeScript error was reported before termination; this is not a compile pass. |

The earlier coherent Distribute/LOCAL-096/099 focused checks are recorded in `docs/luci/evidence/local096-distribute-join-20261016/README.md`. They were not repeated here. Real Windows and installed execution remain open.
