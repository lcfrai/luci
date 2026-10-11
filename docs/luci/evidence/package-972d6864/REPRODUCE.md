# Reproduction scope

The retained files document an already completed build at exact source `972d6864a30b742b12545c8e12f4ca3ae2faa884`. Historical absolute paths identify that run. Original output paths are consumed and must not be overwritten; archives/extracted payloads are not included here.

For an independently admitted Linux rebuild, use a fresh isolated checkout at the exact source, the repository's Node 24.19.0 and Go 1.26.3 setup, frozen desktop lockfile and existing release instructions. Do not use a new Git worktree unless explicitly requested. Preserve user data and any existing generated release before a build. The product command recorded in the original receipt is:

```sh
GOMAXPROCS=2 GOFLAGS='-mod=readonly -p=1' bash scripts/build-release.sh 0.5.0 linux-amd64
```

Use fresh output/evidence paths and the original 600-second build budget with bounded owned-process cleanup. The copied wrapper is tied to `/workspace/luci`, checks exact clean/retrievable source, and preserves a distinct expected prior release; do not run it unchanged against consumed source/output state. Capture a fresh underlying exit and source identity.

The original verifier hashed the two archives against their manifest, extracted the portable archive into an exclusive fresh directory, compared every regular member, checked licence evidence and recorded API/web identities. Its output records 965 member matches. A replay requires the actual generated payloads and must retain its own results. This publication did not rerun that verifier or hash its large subjects.

For Windows, follow the [972d checkpoint](../../TEST-CHECKPOINT-972D6864.md) and [isolated native procedure](../../LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app). A Linux package or Windows installer generation does not demonstrate native execution. Keep 786 results separate and adopt 972d only at a closed test-batch boundary.
