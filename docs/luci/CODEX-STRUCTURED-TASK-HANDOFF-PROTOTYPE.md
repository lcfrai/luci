# Codex structured terminal handoff prototype

This source connects fresh local Docker Codex task attempts to Luci's existing
owned handoff collector. It does not establish model quality, passing checks,
independent review, integration, delivery acceptance, image compatibility or
runtime acceptance. Other harnesses and historical sessions keep their existing
report protocol. No task is resumed or replayed by this connector.

## Data and lifecycle

1. Existing authority, execution-contract, source, capacity and image admission
   still select/reserve the original task session. After captured context is
   written, the controller creates a new exclusive private terminal-contract
   directory and mounts it read-only. The stored wrapper arguments bind the raw
   contract SHA256. Contract fields bind original project/run/operation/session,
   attempt/worktree/task, saved coding or reviewer role and review target, task
   and review-assignment digests, full implementation/source and reviewed-context
   digests, actual helper profile/image, and length-framed command digest.
2. The adapter invokes the already-selected upstream command once. It adds only
   fixed `--output-schema` and `--output-last-message` paths before the opaque
   prompt. Controller developer instructions replace the legacy instruction to
   write `handoff.json` with a final structured response. Authentication remains
   in the existing provider argument/environment preparation; no credentials are
   copied into the terminal contract or report.
3. The adapter forwards opaque stdout/stderr with bounded pipe backpressure.
   It does not parse transcripts, partial JSON events or prose. Only original
   child close with code zero, no signal/failure, and both streams' end and close
   permit a bounded single-link identity-checked final-file read. Strict UTF-8
   JSON rejects duplicate keys, unknown fields, wrong types and explicit bounds.
   The unchanged product decoder remains the final report/path validator.
4. A fresh `.codex-terminal` directory is a retained no-replay marker. Existing
   markers/reports/receipts, nonzero/signal/unknown completion, missing/unsafe/
   partial/invalid JSON, or interrupted publication never retry or overwrite.
   The receipt is created exclusively before `handoff.json`; the report is the
   final commit marker. A partial receipt/report remains failed/unconfirmed.

The candidate and receipt are **agent-writable, same-UID untrusted data**. They
correlate bytes with a controller-owned contract; they do not attest to EOF,
checks, authorization, or genuine authorship. An agent can write files in its
output mount. This is not an isolation boundary against a malicious task.

## Controller-only eligibility fact

`TaskTerminalExitZero` is a private optional registry fact. Its sole positive
writer is `markTaskTerminalExitedWithEOF`, through the existing guarded session
terminal update. It requires the passive observer's known typed sandboxd Exit
and normal upstream EOF. The update rechecks original allocation/reference,
creation, token or exact generic-terminal upgrade, run/operation/worktree,
command/contract, implementation and reviewed context, one-shot observer intent,
and the original observation budget inside the registry write lock. Desktop
attach diagnostics for these adapter sessions stay generic until that EOF
observation, preventing an earlier Exit-frame-only diagnostic from racing it.
Legacy session diagnostic behavior remains unchanged. Readiness/restart clear
both new private fields; fresh/forked records start false/omitted. Nothing parses
`ExitReason` to infer a zero exit.

The passive observer uses sandboxd's existing independent broadcast subscription
(`Hub.SubscribeWithOverflow` / `server.attach`). It never writes stdin/Resize,
steals another subscriber's frames, increments user attachment counts, starts a
process, reconnects, retries, renews lifetime or signals a worker. It discards
bounded opaque frames and decodes only the typed Exit. One persisted attempt
marker prevents a second controller reader after failure or restart.

The **new observer-only 30-minute limit** is anchored to immutable registry
`CreatedAt`. It is not a pre-existing execution/task deadline. Invalid, zero,
future or expired timestamps refuse observation. One monotonic context covers
dial/drain/normal socket close, with the absolute original deadline checked
before effect and inside the final registry update. Expiry, lost/overflowed
streams, missing normal EOF, controller restart, readiness missed by a very
short task, or a task completing after this bound leave the live connector's
eligibility unknown. Socket disposal does not stop the worker. No lifetime or
execution-resource policy is changed. The durable original-container path below
can recover that eligibility independently; no attachment is retried.

### Durable original-container continuity

The new task-only image capability `ai.lcfr.task-exit-normal-zero=v1` is required
alongside the structured-result schema before a fresh automated Codex task can
start. The fixed task command opts sandboxd into a strict PID1 mode: container
zero is possible only after the original successful `Wait`, completed PTY read
loop, and clean listener shutdown/join. Manual closure, signals, unsuccessful
Wait or listener failures cannot produce this positive fact. Generic sessions
and historical images retain their old behavior and never acquire eligibility
from container zero alone.

The backend observes only the stored immutable original CID, image ID, creation
and start timestamps, fixed contract/protocol labels and stopped daemon state;
it rejects restarts or changed/unknown identity. The controller rechecks the
private canonical task and settled launch receipt, then durably records a
private terminal fact before normal stopped-container removal. A new registry
generation may require the next normal reaper pass; existing restart CAS is not
weakened. Persisted facts survive the controller restart and container cleanup,
so an hours-long task can still reach the existing collector after the live
observer's transport window ends. No new worker, replay, task deadline, renewed
authority, or model call is introduced.

Only opted-in normal-zero permits report collection, not acceptance. A known
nonzero or OOM is retained as unsuccessful container termination (not an exact
child code or signal), permits ordinary cleanup, and closes existing task
observation as failed without requiring a successful report. Unknown/corrupt
identity remains held. Typed live Exit facts are retained separately; container
inspection never fabricates WebSocket EOF. The worker-authored report/receipt
remains untrusted and all source, review, integration and delivery gates remain.

This source requires a separately approved runtime image upgrade containing the
new sandboxd, entrypoint and exact labels. No current image was acquired, built,
launched or verified by this source change, and the retained 3bd internal app
candidate remains untouched. Compatibility/source tests are authored but unrun.

The existing collector still holds the exact stopped-original/output fence,
reads the original reviewed context, verifies the matching operation and source,
and decodes the canonical report. For new adapter sessions it additionally
requires the controller fact and exact receipt/contract correlation. Reported
checks remain claims. `ObserveTaskCompletion`, current-authority checks,
independent reviewer obligations and integration/release gates are retained.

## Source and verification scope

Codex's documented final-schema/file pattern is also already used by Luci's
`internal/assistance/repository.go`. Official references:

- https://learn.chatgpt.com/docs/non-interactive-mode
- https://learn.chatgpt.com/docs/cli/reference

The retained native-helper wrapper keeps its exact Codex 0.160.0 compatibility
probe, delegation settings and authentication shapes. Its only new permitted
arguments are the fixed schema/final-file pair; arbitrary output paths remain
rejected. The sandbox Dockerfile copies the adapter and schema and advertises the exact
capability/schema labels. The existing trusted build-context collector adds only
these two fixed inputs/COPY paths for that recipe; historical recipes retain the
legacy closure. No image was
built, installed or executed for this proposal. Existing immutable helper image
policy still requires a separately built/reviewed configured image; an old image without the exact advertised capability/schema refuses before
Backend.Create, with no fallback. ImageInspect reads only existing local image
metadata; it never pulls, builds or probes a CLI. Initial admission pins its
immutable image ID into the contract and Spec; final admission re-observes that
exact ID and contract before Create. Labels advertise capability, not proof of
wrapper bytes or successful execution. Activating this source requires a
separately reviewed new image build and actual compatibility/identity acceptance
after the runtime hold. The retained `3bd01c06` internal candidate excludes this
connector and retains its prior runtime; it cannot claim this feature active.
The connector was integrated into checkout source on 31 October. That source
change does not upgrade an installed runtime image. A fresh automated Docker
Codex task requires the new advertised image capability; an old image refuses
before allocation. Ordinary interactive sessions, existing historical task
records, other harnesses and non-Docker paths retain their prior behavior.

Authored tests cover strict/duplicate JSON, no replay/overwrite, report digest,
opaque argv and native flag boundaries, child close plus stream EOF, nonzero/
unknown/cancellation, controller-only exit facts, stale original observers,
receipt/source/attempt binding, legacy encoding, restart reset, typed broadcast
EOF and the nonrenewing observation budget. They have not been executed here.
