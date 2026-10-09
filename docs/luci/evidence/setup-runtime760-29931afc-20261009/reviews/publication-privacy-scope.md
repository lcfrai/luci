# Runtime760 setup publication: small-receipt review

This review reads only the selected small receipts and eight serialized archive metadata files. It does not read/hash an archive or Go tree, run an application/control/Docker operation, walk the 106-source tree, save environment configuration, or independently observe the earlier restoration. Environment values were not printed.

## Checked relationships

- `runtime-archive-760-9a42e556fa22/receipt.json` records published exact `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`, source reuse at `29931afc5b31ae9b9bd7b8973a7e3fdb840609a3`, archive SHA `637d1d02c701be5f4327bb76dcb58335e1665b27945e3f1cb08bb825c5fd96fd`, and 1,013,403,794 compressed bytes.
- All eight `metadata-*.raw` files match the SHA256 and byte counts in `diagnostics.json`. The literal image-config bytes hash to exact760; parsed config equals the diagnostic `rawConfig`. The Docker manifest contains one image and only `luci/sandbox:dev`; config rootfs equals the recorded expected rootfs.
- The diagnostic permits missing-to-null normalization only for Cmd, Labels, OnBuild and Volumes, with no differences recorded. Keep that explicit; do not say all raw JSON fields are byte-identical to Docker inspect.
- The three `new-*` setup/provenance files match `receipt.json/newPins`. The new provenance binds 106 runtime source entries, original runtime build at `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`, and reuse at exact299. The full 106 source bytes were not re-read for this review.
- Candidate `inputFiles` and archive-verifier `verifiedFiles` bind the current two runtime files, including sandboxd SHA `ee9f13ce282ad80d612fb7a4156b653183a18a17531fa5b787efb47d9163176b`. Base-layer copies have different historical hashes and must not be confused with the final overlay files.
- `reviewedRuntimeEvidence` equals the original candidate receipt path and SHA. It is source/export context, not a newly executed installed acceptance witness.

## Keep export, tag restoration and saved configuration separate

The archive publication/provenance still say `dockerLoadVerified=false`. Preserve those original bytes and flags. The later `runtime760-restore-check/result.json` separately records installer SHA `21cf1a024057fde3966f8e5582f6f8df9692ed00c114a202013b4d98221c356f`, exit 0, and exact760 tag restoration at 15:35:33–15:35:53Z. Its stated scope is an existing daemon retaining the same image/layers under an independent candidate tag. It does not verify a fresh daemon, fresh layer materialization, snapshot restoration, application behavior or Windows acceptance. Retain its stdout/stderr and untag output if source-bound execution provenance is included; no need to rewrite the older archive receipt.

The publication stage started with root/TMP 5,516,296,192 / 4,088,139,776 bytes. The recorded export floor is 3,905,250,841 on both filesystems, with an additional root compressed-output cap of 1,288,490,188 bytes; raw export is 2,316,254,720 against cap 2,563,073,561. Recorded post-export root/TMP is 4,500,803,584 / 4,088,139,776. These are historical export observations, not later placement outcomes, current capacity, future admission, or continuous free-space guarantees. Actual compressed size is below its hard cap; no cap weakening is implied.

`onboarding299-draft-source/representation-note.json` explicitly says exported complete-text local representations add one trailing newline to saved fields, with saved-read revision 39. Preserve that note beside exported scripts. Local-file hash equality does not establish original saved-field byte identity. A new literal configuration-save claim needs a separate saved-field revision/readback with exact character/byte representation rules; no such later save/readback was inspected here.

## Publication assertions and privacy boundaries

1. Assert every copied small original hash/length, and retain original-versus-transformed representation mappings. Bind the two helper/source revisions and the startup template separately from runtime image, product, package and saved-field revisions.
2. Assert raw metadata membership, config digest, exact manifest tag, ordered layer diffIDs, final runtime file hashes/modes/owners, and the four permitted missing/null fields. This verifies receipt/config metadata relationships; layer/archive rehash remains the original operation's assertion unless actually repeated.
3. Assert compressed/raw output caps and initial floor arithmetic without inferring allocated-space deltas or atomic admission operands. Keep source/consumer quiescence and daemon-drain limitations.
4. Environment names seen in the selected config records are only `PATH`, `NODE_VERSION`, and `YARN_VERSION`. Do not print values or copy host/container environments. If publication is names-only, raw config-bearing records need an explicit projection plus original SHA/length; a projected record cannot still claim byte identity or independently reproduce the image-config digest. If exact raw image-config bytes are published under a separately reviewed public-image policy, describe that policy rather than silently treating a projection as original.
5. Do not include Docker client configuration directories, auth stores, private runtime fixture/state/ledger files, archive payloads, or unrelated logs. Candidate tags, local staging paths, engine ID and process identities are metadata, not credentials; their inclusion still needs the declared path/redaction policy. This review is not a universal secret scan.

No new concrete source/receipt defect was identified in this narrow review. Final capsule inventory, checksums, transformations, companion links and saved-configuration proof remain to be checked when the capsule is supplied.
