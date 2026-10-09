# CLOUD-053: verify published Git bytes before announcing evidence

The original publication had checksum-bound files on disk but omitted ignored `.log` files from Git. A clean working file or successful filesystem checksum check cannot establish that another checkout can retrieve those bytes.

The new read-only `scripts/acceptance/verify-publication.py` reads each selected capsule's **SHA256SUMS from the selected Git index or resolved immutable commit**, snapshots its paths/modes/object IDs, and verifies the actual blob contents. It never reads a working-tree evidence file as a substitute. Inherited `GIT_*` overrides are cleared; repository root is explicit, text conversion/filters are not applied, and object replacement and lazy fetching are disabled. Missing promisor objects fail instead of being fetched. It does not stage, commit, fetch or modify an index.

```sh
python3 scripts/acceptance/verify-publication.py --repo . --index --require-complete docs/luci/evidence/CAPSULE
python3 scripts/acceptance/verify-publication.py --repo . --ref EXACT_COMMIT --require-complete docs/luci/evidence/CAPSULE
```

Multiple capsule roots can be supplied. `--ref` resolves once to a commit and reports that exact object ID. `--index` snapshots selected index entries and hashes those immutable blob IDs, even if the working tree differs. It reports a digest of the selected Git entry list. It does not lock publication against a later index edit; rerun after the final staging change and verify the resulting commit before handoff.

Missing blobs/manifests, wrong hashes, non-regular Git modes, unmerged entries, malformed/empty manifests, duplicate entries, self-listing manifests and unsafe paths fail. Paths must be portable, relative slash paths with no traversal, control characters, backslash or colon; SHA256SUMS uses standard `hash  path` or `hash *path` records. Escaped GNU filename records are not supported. Manifests are capped at 4 MiB; evidence blobs are hashed in 1 MiB chunks. `--require-complete` also rejects selected **Git entries** absent from the checksum list. It does not discover ignored/untracked working files absent from both Git and the manifest, nor establish the validity or completeness of the evidence itself.

## Controlled evidence

Eleven final real-Git tests pass in disposable temporary repositories. Each Git mutation checks an exact randomly marked owned root; inherited Git directory/index/object/config overrides are cleared for those mutations. Tests cover:

- Ignored nonempty and zero-byte `.log` omission fails; force-adding the original bytes passes.
- The original immutable commit continues to fail after local correction; the corrected commit passes.
- Wrong staged bytes fail even with correct working bytes; wrong unstaged bytes cannot invalidate correct staged bytes.
- Staged manifest bytes are authoritative; malformed/missing/duplicate/empty/self-listed manifests fail.
- Traversal/absolute/Windows/unsafe path cases, symlink mode and optional complete tracked coverage fail appropriately.
- Literal names containing spaces, unrelated unselected capsules, inherited Git redirection and fixture ownership refusal preserve their boundaries.
- A real local-file partial clone without the manifest blob reproduces the initial guard's lazy-fetch read-only gap. The identical regression fails before the one-line `GIT_NO_LAZY_FETCH=1` correction and passes afterward, preserving the partial clone's entire object store. No network remote was used.

`controlled-demonstration.json` retains five concrete original/after outcomes, including the original `Missing Git blob` and wrong-index `SHA256 mismatch`. Its Git revisions belong only to the synthetic fixture, which was removed after recording; they are not Luci revisions. These scenarios used the initial guard source and overlap the final eleven-test batch; they are not five additional independent regressions. The original ten-test batch remains separate and overlaps the final eleven.

`tests-result.json` binds identical source hashes before and after the test batch. Original zero-byte stdout and full stderr are retained. No Luci product test, build, app or real repository/index mutation was performed. Native Windows has not executed this Python/Git batch. The parent separately owns restoration and verification of the actual 12 omitted publication files; this controlled capsule does not certify that repair by itself.

`root-reproduction/` additionally preserves two expected failures against the actual immutable `5f318d5178210e95cd2b773f07686d88936d603e` publication. Root ran the initial captured guard; both receipts report missing Git blobs in the idle/counter capsules. This read-only reproduction does not modify that old commit. The later corrected guard is separately bound by the final eleven-test receipt. The root owns subsequent exact log restoration and index/commit verification.
