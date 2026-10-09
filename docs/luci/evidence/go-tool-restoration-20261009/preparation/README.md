# Required Go restoration — prepared only

Root-owned operation; no actual tool relocation performed by preparation or controls. Invoke only after the root confirms its exclusive tool window and no known compiler/tool users remain:

```sh
python3 -B /workspace/.luci-tools/go-tool-restore-v1/run.py --acknowledge-exclusive-window
```

The wrapper pins `operation_common.py`, `restore_go.py` and the byte-identical prior `tree_support.py`. It creates a fresh private operation directory, a source-bound live-owner coordination receipt (maximum600s), and private stdout/stderr plus durable wrapper metadata. Process waits are400s, then10s TERM, then10s KILL settlement; timeout/forced/unsettled states fail. Bounded settlement also runs when a checkpoint write fails. Leader exit is not a descendant census; post-signal group settlement stays explicitly unobserved. Signals target only the exact spawned child process group after checking its known PID/start. Python scheduling and synchronous OS I/O are not preemptively bounded. No app, compiler, Go version command, Docker request or unrelated signal is made. The inherited support module's older Rust/PowerShell main is not an entry point for this operation.

The exact original `/tmp/luci-retained-tools/go1.26.3` tree must match the historical manifest at `/workspace/.luci-tools/tool-relocation-20261009T1158/go1.26.3/tree-before.json` (SHA256`50b4a84d580c4b072378c2cea371854fa750c29ef512c634b0282c365a57f242`,16,688 entries,232,365,810 logical bytes). Its present workspace path must be the exact expected symlink. All regular-file bytes, directory/file modes, uid/gid and mtime are compared; symlinks, hardlinks and special entries inside the tree refuse. The historical manifest remains unchanged and is referenced rather than duplicated into every numbered journal.

The helper copies to a private workspace staging directory, verifies both original and copy, and atomically exchanges the workspace alias with the copy. Only then does it exchange the TMP original with an alias to the now-real workspace tree. After another process/owner observation and verification, it removes only the exact old duplicate. Thus the final real workspace tree does not depend on TMP retention; the old TMP path remains compatible. Inode/ctime/access-time and hardlink topology are not preserved or claimed. Real executable behavior is a separate root check after successful restoration.

Root pre-copy free space must cover232,365,810 logical bytes plus6,736,759,858 bytes and64MiB working guard; post-copy must retain floor+64MiB. Final observation retains the6,736,759,858-byte root floor. Logical byte estimates are checked against actual free space after copying. This is neither future Create admission nor proof that both filesystems reach the separate3,905,250,841-byte image-export floor.

Readable exe/cwd/argv/maps/fd references to the exact paths refuse. Exact stable unreadable PID/start identities203/1521,254/1534 and260700/2331192 are accepted only under root's stated limited coordination; new/missing/reused identities refuse. This is not universal inactivity evidence, and no process environments are inspected.

Failures leave numbered receipts and every observed partial path. There is no automatic rollback or retry. Before a later operator action, inspect the last completed stage and independently reverify the full retained original/copy. Before either exchange the TMP original remains a real tree; after the first exchange both real copies may remain; after the second, the old tree may remain at the named TMP scratch path. Never rerun against occupied stage/scratch paths or remove uncertain duplicates merely to make the guard pass.

Watchdog evidence: the same three fake-process/signal controls fail against the retained original draft and pass after correction (initial child checkpoint failure, timeout checkpoint failure, forced settlement). No real child or signal is used.

Controlled evidence:10 tiny same-code disposable-tree cases pass. They cover final bytes/mode/link topology, changed byte, symlink/hardlink surprises, active reference, unaccepted unknown, existing stage, insufficient free space, refusal between exchanges and active reference before retirement. Fixture roots, source counts/hash and process observations are explicitly substituted; they do not hash or move actual Go. No runtime-readiness claim follows from these controls.
