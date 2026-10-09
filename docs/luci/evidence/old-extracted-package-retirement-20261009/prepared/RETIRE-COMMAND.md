# Prepared exact retirement command — not executed

The source is frozen at SHA256 `9e2e7f71f45bca645a5a50aee600db8e7093ae23b860aca4bdbb0def4a1243e6`. It pins `plan.json` SHA256 `c7e298a50984471954939b0d0f3f01e7f96a2e881cfb0252fcfaec5cf61287cd` and the exact bounded usage scanner. Syntax checks passed; no retirement or restoration control/execution has been performed. Root must first review this source and wait for the current runtime's closure.

After root approves the six exact paths and confirms the closed task window, use a fresh operation directory:

```sh
timeout --signal=TERM --kill-after=10s 660s \
  python3 -B /workspace/.luci-tools/old-extracted-packages-retirement-plan/retire.py \
  --operation /workspace/.luci-tools/old-extracted-package-retirement-operation-root1 \
  --closed-task-window \
  --accept-known-unreadable-identities 203:1521,254:1534,260700:2331192
```

Retain stdout/stderr and the outer exit result. The script has a600-second cooperative bound; the outer660+10-second bound is for synchronous stalls. It signals only its own invocation. Pending, all-target preflight, each target's pending/completed result and final/failure receipts are separate0600 files with flush/fsync. TERM/INT become a recorded failure while Python remains able to handle them. An outer forced kill or missing final receipt remains uncertainty, never a successful retirement claim.

Before any unlink, it checks all six archive SHA256 values, exact original alias identities, all current file hashes/metadata, exact roots, absent unexpected entries and bounded current process references. It repeats target verification and usage checks immediately before each target. Unreadability is accepted only for the three explicit PID/start pairs and the inspector's own disappearing fd observation; other unknowns or any target reference refuse mutation. It does not read process environments.

Deletion walks only the pinned member allowlist through opened, identity-checked directories. Regular files must still be ordinary singly-linked files with exact content/metadata at unlink; directories must retain their exact identities and be empty. Current packages b94ad37e/29931afc/5f4a966f are outside the allowlist and their root metadata is checked again. Archives, aliases, original receipts, fixtures, ledgers and Docker are never removed or rewritten.

A partial failure retains the completed/removed-member report and original archives. No automatic rollback or retry is claimed. `restore.py` is the separately prepared exact-archive restoration route; its current status and limitations remain in the plan. Retired old extracted paths would be unavailable to historical commands until restored, even though every historical receipt and compressed archive remains byte-identical.
