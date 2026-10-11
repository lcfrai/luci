# CLOUD-084: bounded microVM reuse review

Observed 11 October 2026 UTC. This is an official-source review, not an executed E2B/Fly deployment, benchmark, price quote, provider connection or Luci backend implementation. No packages were installed; no credentials were inspected; no provider resources were created. Glasses discovery was relayed by the Windows tester. This cloud agent independently used read-only official GitHub API/raw sources, not a Glasses call.

The reviewable decision and implementation contract is [CLOUD-MICROVM-EXECUTION.md](../../CLOUD-MICROVM-EXECUTION.md): E2B Cloud is the proposed first concrete microVM provider; local Docker remains the working primary path. E2B Embed is a later BYOC candidate requiring actual KVM/provider/cost verification. Fly Machines is the compared alternative, not a supported Luci option. Kata remains dormant and rejects today's mount/working-directory contract.

`source-receipt.json` binds 23 inspected official source objects, six selected Luci source files and nine published copies/excerpts by SHA-256 and byte length. Exact repository revisions and URLs are included. The E2B `infra` repository now identifies itself as E2B Runtime; both the legacy official URL and canonical `runtime` URL were successfully read at the same pinned commit. Full external source retrieval records are retained outside the checkout. The copied process/filesystem contracts are examples of existing reuse surfaces, not new Luci dependencies.

Contiguous excerpts preserve original bytes and recorded inclusive line boundaries. They are not standalone compilable/tested programs. Full source hashes and immutable URLs permit reconstruction. Apache-2.0 licence copies accompany the selected upstream material. These source licences do not establish hosted-service terms or upstream coding-engine redistribution rights.

Official website requests to e2b.dev, docs.e2b.dev and fly.io returned proxy 403; official GitHub reads succeeded. Consequently current E2B managed pricing/quotas, account permissions, service version parity and regional availability are explicitly unverified. No pricing figure, boot-time claim or whole-product acceptance is derived from this research.

Validation: exact copied bytes, excerpt boundaries, selected-source hashes and capsule checksums were verified locally; this is publication integrity only. No product tests or runtime acceptance were needed for this documentation-only review. CLOUD-084 remains open.
