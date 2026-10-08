# Private capture format, version 1

`run.json` is strict JSON. Every field below is required; there are no ignored
extra fields. Keep actual requests, responses, source and private product exports
outside Git. Evaluation results contain artifact IDs, SHA-256 hashes and byte
counts, not the raw captures or paths. Inspect any intended public report before
publishing: experiment-selected names and IDs are caller supplied.

Top-level fields:

| Field | Value |
| --- | --- |
| `version` | `1` |
| `kind` | `live-provider-capture` or `controlled-parser-fixture`; never relabel fixture data as live |
| `caseId`, `variant` | Exact prepared case and variant |
| `conditionId`, `trialId` | Stable IDs fixed before capture |
| `execution` | `{mode: "serial" or "parallel", concurrencyLimit: 1..4, batchId: string}`; serial requires limit 1 |
| `harness` | `{name, revision, adapterRevision}`; live revision is full 40–64 hex source/content identity, adapter revision is a 64-hex hash |
| `model` | `{provider, name, snapshot}`; `snapshot` is the actually resolved model version or `null`, never an invented pin |
| `artifacts` | Map from safe artifact ID to `{path, sha256}`; all paths relative to this run directory |
| `inputArtifact`, `answerArtifact`, `settingsArtifact`, `sourceSnapshotArtifact`, `contextTraceArtifact` | IDs in `artifacts` |
| `requests` | Ordered array of 1–32 request records below, including relevant orchestrator and worker calls |

The settings artifact is an object containing `model` exactly equal to the
manifest model plus all actual generation settings, tool permissions, context
limits, retry policy and seed support. Unsupported seeds, unresolved snapshots
and non-deterministic provider behavior must remain explicit. Do not store API
keys, authorization headers, cookies or personal data in any fixture. Use
synthetic project content and capture bodies only in a private directory.

`sourceSnapshotArtifact` contains exactly:

```json
{
  "caseProjectId": "stock-room",
  "projectId": "ACTUAL_PRODUCT_PROJECT_ID",
  "sourceCommit": "ACTUAL_40_OR_64_HEX",
  "contextRevision": "ACTUAL_REVIEWED_REVISION_OR_BINDING",
  "sourceArtifactHashes": {
    "raw-brief-export": "ACTUAL_SHA256",
    "raw-native-bindings": "ACTUAL_SHA256"
  }
}
```

The symbolic corpus project and the application's generated project ID are
different identities and both remain explicit. Every source-artifact value
must match that declared artifact's actual bytes. Keep source tree manifests,
reviewed context manifests, run/task/operation bindings, independently recorded
checks and native API receipts here as applicable. This is an artifact binding,
not an assertion that a supplied file was obtained honestly.

`contextTraceArtifact` contains exactly:

```json
{
  "projectId": "ACTUAL_PRODUCT_PROJECT_ID",
  "sourceCommit": "ACTUAL_40_OR_64_HEX",
  "contextRevision": "ACTUAL_REVIEWED_REVISION_OR_BINDING",
  "harnessRevision": "ACTUAL_HARNESS_REVISION",
  "captureKind": "luci-native",
  "nativeBindingsArtifact": "raw-native-bindings",
  "runId": "ACTUAL_RUN_ID",
  "taskId": "ACTUAL_TASK_ID",
  "operationId": "ACTUAL_OPERATION_ID",
  "contextDigest": "PROJECTION_DIGEST_DEFINED_BELOW",
  "deliveredSources": {
    "stock-brief-r9": {
      "artifact": "selected-brief-text",
      "originArtifact": "raw-brief-export",
      "originPointer": "/purpose"
    }
  },
  "requestIds": ["request-1"]
}
```

Include **every** delivered visible source exactly once. `artifact` is the exact
UTF-8 text of that prepared source. The origin must be a captured source artifact,
with a JSON pointer to a string containing those exact bytes, or `null` to use
the complete raw UTF-8 origin. This forbids substituting an unrelated hand-written
summary for actual captured context. `contextDigest` hashes canonical sorted
JSON mapping source IDs to selected-text SHA-256 hashes. It is an evaluator
projection digest. Preserve native Luci digests separately: C011's typed
dependency-packet digest and the manifest's rendered-file byte hash have different
definitions and must not be conflated.

Live Luci captures require `captureKind: luci-native` and a declared native
bindings JSON artifact included in the source snapshot. The native artifact's
specific product schema must be checked by the adapter and independent reviewer;
this generic evaluator does not pretend to validate every version of Luci's
private schema. A direct baseline uses `adapter-baseline`; controlled tests use
`controlled-parser-fixture`. A controlled context cannot support a live label.

Each request record contains exactly:

```json
{
  "id": "request-1",
  "requestArtifact": "provider-request-1",
  "responseArtifact": "provider-response-1",
  "sourceIds": ["stock-brief-r9", "dependency-packet", "recorded-checks", "old-worker-note"],
  "startedAt": "2026-10-08T00:00:00.000000Z",
  "finishedAt": "2026-10-08T00:00:01.000000Z",
  "outputPointer": "/output/0/content/0/text",
  "usagePointers": {
    "inputTokens": "/usage/input_tokens",
    "outputTokens": "/usage/output_tokens",
    "cachedInputTokens": null
  },
  "billingArtifact": null
}
```

The request is the actual provider JSON body and its `model` must match the
declared model name. Each listed source must occur fully in an actual request
string; all selected sources must be covered across the captured requests.
The frozen case task and input digest must also appear in captured request text.
On-demand context is counted at the request in which the worker actually reads
it, not assumed read merely because a bundle file exists. Artifact byte hashes
and request-source projections preserve delivery lineage. They cannot prove
the model attended to a string or detect every hidden call; the capture adapter
must account for all calls and omissions independently.

The response is the captured JSON response. Exactly one `outputPointer` selects
the final model-produced string whose trimmed bytes equal the answer artifact;
intermediate requests have `outputPointer: null`. No fence stripping, schema
repair, guessed citations or model-answer rewriting is allowed. Streaming
transports require a separately reviewed lossless capture/assembly adapter and
retained raw stream lineage. This evaluator itself does **not** parse SSE,
authenticate provider request IDs or establish transport completeness.

Usage pointers select nonnegative integers from captured responses. Unsupported
or missing usage is `null`, including cached tokens; no byte-to-token estimate
is substituted. Billing is optional. If present, its artifact is exactly
`{requestId, amount, currency, kind:"provider-billed"}` and uses actual observed
charges, not a rate-card guess. Aggregating mixed currencies is refused.

Every artifact must be an ordinary single-link file, no symlink or escaping
parent. Each is at most 2 MiB; at most 96 artifacts and 32 MiB aggregate are read.
Nested JSON is bounded to 32 levels. Large actual traces need a deliberate,
reviewed format revision; do not silently truncate or omit requests to fit.
Each case has at most one hour of captured request span. These limits bound the
offline evaluator, not a provider's actual billing or runtime consumption.

All timestamp and SHA placeholders above are documentation, not valid evidence.
