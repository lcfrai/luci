Runtime grants omitted by installed wrappers
===========================================

The release producer refuses packages without retained attribution. These
exact-version files supplement original installed grants; they do not replace
or relicense any component. provenance.json records immutable source commits,
registry metadata and SHA512-verified published tarball identities.

@npmcli/agent 4.0.2 declares ISC and identifies GitHub Inc. as its author in
its original package.json, but the exact source and tarball omit a licence
file. Its original declaration is retained alongside the SPDX canonical ISC
text. The standard text's copyright placeholders remain verbatim; Luci has
not invented a copyright year or holder.

@sigstore/verify 3.1.1 uses the original Apache licence from its exact source
commit. spdx-exceptions 2.5.0 retains its original Linux Foundation attribution
and CC-BY-3.0 declaration. spdx-license-ids 3.0.23 retains its original CC0
declaration. The latter two also retain the canonical full SPDX licence texts.

OpenCode's exact 1.18.34 distribution declares MIT and lists its two installed
Linux x64 platform binaries as exact-version optional dependencies. Only these
matching distribution siblings share that original parent licence; arbitrary
dependencies never inherit their parent's licence.

Claude Code remains proprietary. Its original installed reservation of rights
is retained, with a separate explanation linking Anthropic's published
conditional preinstallation and user-authentication terms. No open-source
grant or successful provider authentication is claimed.
