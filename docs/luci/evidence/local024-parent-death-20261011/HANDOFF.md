# LOCAL-024: owned Windows sidecar ends with its native parent

Review artifact only; no repository edits were made by this agent. `LOCAL-024.patch` applies to `bb36ced437e031cc1f0a32b580280adeafe54de7`; its exact source binding and successful `git apply --check` are in `integration-binding.json`. The root may integrate this patch independently of C080.

## Change

Windows acceptance reported that terminating the native parent through driver DELETE-session left its owned API/conhost/port alive. The existing graceful `RunEvent::Exit` callback cannot run after abrupt parent termination.

The managed Windows launcher now creates a private, non-inheritable Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. `CreateProcessW` receives `PROC_THREAD_ATTRIBUTE_JOB_LIST`, so the process joins the job during creation: no running or suspended child exists before its job ownership is installed. Only the native parent holds the job handle; the exit waiter holds only a process handle. The explicit inherited-handle list contains only a NUL handle for bounded disposal of output already discarded by the prior event loop. Setup errors close the job; there is no unprotected fallback if the OS rejects job attributes or process creation. Nested-job compatibility is required, not inferred from API readiness.

Tauri still resolves the bundled executable, then passes its exact program, arguments, environment overrides and current directory to the owned launcher. No shell/PATH lookup is added. The external `LUCI_API` branch, explicit state selection, bootstrap/local-access tokens and Go state-lease implementation are unchanged. The complete normal-exit callback is byte-preserved, including authenticated shutdown and its existing fallback. Other platforms retain the existing Tauri child/event behavior. This change does not promise graceful flushing, Docker cleanup, or reservation release after abrupt death.

## Observed cloud checks

| Check | Actual result | Scope |
| --- | --- | --- |
| Windows module `cargo check --lib --target x86_64-pc-windows-msvc` | exit 0, 3.485 s | Exact launcher/quote modules; standalone pinned dependency harness; expected unused warnings |
| Windows tests `cargo check --tests --target x86_64-pc-windows-msvc` | exit 0, 0.418 s | Exact production module and all controlled Windows tests compile; no execution |
| Host quote test compilation | exit 0, 0.473 s | Pure UTF-16 helper only |
| Host quote tests | exit 0; 4 passed, 0 failed | Space/Unicode path, empty/quoted/trailing-slash args, invalid NUL/path, unpaired UTF-16 |
| Patch applicability and original-file parity | exit 0; 3/3 originals match | Against the recorded bb36ced4 source |

Every check has unchanged before/after input hashes, original stdout/stderr, invocation and actual exit in `validation/`. Rust 1.99.0 and Windows target std were checksum-verified. `dependencies/*downloads.json` records exact download identities; `validation/environment-supplement.json` supplies the runner's RUSTFLAGS omitted from its original structured environment fields. The original receipts are not rewritten. Independent read-only review found no concrete blocker.

These are focused cross-compilation and host-helper results, not a full Tauri native build or a Windows behavior pass.

## Windows tester: first isolated kernel check

From this artifact directory on Windows with Rust/MSVC available:

```powershell
cargo test --locked --manifest-path windows-check/Cargo.toml --lib windows_sidecar:: -- --test-threads=1
```

This supplied portable harness uses the exact patch module/test files and the same registry versions/checksums. It has not been executed in this cloud environment. Expected selection: 9 ordinary tests (5 Windows process tests plus 4 quote tests) and 1 intentionally ignored subprocess fixture. Do not run the ignored fixture directly. Record the actual source hashes, tool versions, command exit and raw output; do not infer success from the expected count.

The process tests use disposable test executables in a unique temporary directory; they need no API, WebView, Docker, credentials or user state. They cover: abrupt parent termination at the exact post-CreateProcess/pre-waiter boundary; sidecar and descendant termination with real nested-job self-assignment; survival of an unrelated sibling; cleanup after an injected post-create error; natural exit observation; missing executable refusal; and rejection of a NUL-bearing environment value before child execution. Cleanup retains handles for the specific fixtures. Fixtures self-expire after 60 s on failures; failed directories are retained.

After integration, run the same module tests through the real native crate and build the matching native package using the established Windows setup:

```powershell
cargo test --locked --manifest-path desktop/src-tauri/Cargo.toml --lib windows_sidecar:: -- --test-threads=1
```

Full native compilation, any supported minimum-Windows checks, and the real driver reproduction remain pending. On an environment lacking JOB_LIST support or compatible nesting, startup must report its Win32 error and create no unowned fallback child; no such OS capability fault was executed here.

## Native acceptance still required

1. Run from the matching clean integrated source/package in the already restored controlled Windows environment. Record native parent/API/conhost identities with creation times, selected state path and owned API port. Confirm normal state and bootstrap identity before the case.
2. Reproduce the actual driver DELETE-session parent termination, without substituting ordinary window close. Observe the original API/conhost/process-tree identities and port disappear without manual cleanup; retain an unrelated disposable process and externally managed API as survival sentinels.
3. Check state ownership: same-state contention while the owner is alive, automatic OS lease release after its death, then reopen the same state successfully with the same identity/bootstrap. The persistent `.luci-state.lock` file is expected; do not delete it to simulate lease release. Keep explicit state selection unchanged.
4. Separately exercise ordinary close and authenticated graceful shutdown, then `LUCI_API` override close/death: external API ownership must remain external. Record actual exits and state behavior; abrupt kill is not evidence of graceful flushing or Docker reservation cleanup.

No Windows runtime pass, native build pass, package identity, deployment result or final release acceptance is claimed by this artifact.
