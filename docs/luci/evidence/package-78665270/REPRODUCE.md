# Verification and reproduction limits

Run `sha256sum -c SHA256SUMS` inside this capsule. `original-bindings.json` binds exact small input bytes; original receipts and build logs are copied unchanged. Large archives, extracted package payloads and runtime image archives are excluded, so public receipts alone cannot repeat verification of those private bytes.

The build command was `bash scripts/build-release.sh 0.5.0 linux-amd64`. The retained wrapper supplies clean/retrievable source checks, previous-release preservation, task-local tools, a 600-second process bound and package verification. A fresh build needs exact 786 source, the expected previous b94 release, enough space, and distinct unused destinations. Do not replay against retained release outputs. Historical absolute helper/tool paths are provenance, not guaranteed fresh-clone paths.

The wrapper manifest is its original preparation snapshot; new actual build receipts are separate. No wrapper controls, tests, package builds, Docker operations or native setup were rerun to assemble this publication. C075/C070 checks and runtime input parity retain their original execution scopes. Public data includes selected source, logs without captured credentials, small receipts and hash bindings; not private fixture state, bootstrap data or controller logs.

`assemble.py` requires original small files at the paths declared in it, an unused external output directory and the immutable Git objects. It refuses changed originals or mismatched source. It does not read large binary/image payloads, start services or alter the repository. The accompanying copy plan identifies only new evidence paths and the new Windows checkpoint; root owns publication after the runtime window closes.

The Windows checkpoint follows the existing isolated source/native procedure. This Linux helper must not be run against the shared Windows Docker host. Existing saved environment configuration remains separate and revision-specific; this evidence does not update or publish it.
