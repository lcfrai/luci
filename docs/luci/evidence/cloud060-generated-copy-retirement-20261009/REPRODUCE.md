# Review and bounded reproduction

Verify the publication checksums from this capsule directory with `sha256sum -c SHA256SUMS`. `preparation/manifest.json` lists its own earlier 16 prepared members; compare their hashes without rewriting that manifest. `original-bindings.json` maps exact copied originals. `analysis.json` separates original preparation, controlled fixtures and actual journals.

The recorded one-time retirement must **not** be replayed against the same paths: they now contain compatibility links, and its preconditions intentionally no longer describe their original inode state. The old preparation README contains a historical root invocation, not an instruction to run it again. Do not invoke `retire.py` or `verify.py` merely to review this publication.

To reproduce only the tiny controls, first copy `preparation` into a new disposable directory outside the capsule so generated control receipts cannot overwrite evidence. In that copy run `python3 -B controls.py`, then `python3 -B failure-controls.py retire.py fixed-rerun.json`. Both use tiny temporary files/fake observations, not the real candidate roots. For the original adversarial demonstration use `python3 -B failure-controls.py source-v1/retire.py original-rerun.json`; exit 1 and three failures are expected. Original receipts must remain unchanged, and new results must be labelled new controlled runs. No controls were rerun when this supplement was assembled.

Source and reference/archive identities remain in the original proof. Byte reconstruction after retirement is a separately reviewed action requiring those retained exact bytes, expected paths, modes and consent; it is not supplied or executed by this publication.
