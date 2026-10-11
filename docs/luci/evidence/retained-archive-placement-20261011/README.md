# Retained archive placement, 11 October 2026

The guarded operator command completed with exit 0 and `passed=true`: four exact generated archives (207,602,522 bytes) were copied to the retained `/tmp` filesystem, independently verified, and their original paths atomically replaced with links to the matching copies. Both pinned release manifests remained unchanged. The operation preserved the archive payloads; it did not extract or modify them, alter a fixture/ledger, change Docker, or run an application.

Observed free space changed from 6,722,994,176 to 6,930,587,648 bytes on the source filesystem and 853,315,584 to 645,705,728 bytes on the target. These observations are not a guarantee of later build or admission headroom.

The exact script SHA256 is `fc45246c7cc6f81641ac2ba72ac557dc98e055ecfc6c5e3090a698e2f568df65`. Invocation: `python3 placement.py --apply --expected-script-sha256 fc45246c7cc6f81641ac2ba72ac557dc98e055ecfc6c5e3090a698e2f568df65`. This is a completed one-off operation: the script requires a fresh absent destination and must not be rerun against the already placed archives. `PLAN.md` is the preserved preparation document, not an assertion that execution is still pending.

Original archive binaries remain publisher-held. Receipts bind their exact expected identities and the copy verification; publishing these receipts does not make those archives downloadable or prove restoration in a fresh environment.
