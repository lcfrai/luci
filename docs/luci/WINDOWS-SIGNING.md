# Windows publisher signing and installation

Luci reuses Tauri's NSIS installer, existing icon, MIT licence page and safe
upgrade/uninstall hooks. The installer installs for the current user, groups its
shortcut under **Luci**, and uses the Luci icon for installation and uninstall.
The welcome page introduces Discover, Design, Develop and Deploy, identifies
the per-user install and explains Docker Desktop for local coding. The finish
page sends the user into Luci for setup. Its app description identifies the
integrated project workspace. First setup
still offers Local, Bring your own cloud and truthfully unavailable Luci Cloud;
the installer does not silently install Docker, fetch models or create cloud
resources. NSIS presentation changes are source-only until a matched installer
is built and its install/upgrade/uninstall flow is observed.

## Provisioning prerequisites

1. Choose the actual legal publisher identity and obtain a publicly trusted
   code-signing certificate through the organization's approved process. The
   existing builder supports the exact certificate in `Cert:\CurrentUser\My`
   with accessible private-key provider, code-signing EKU and current validity.
   Hardware-backed keys may require the provider/token and operator consent.
   Do not export keys, commit PFX files or pass passwords to this script. Root's
   current-user store inspection found no signing identity; none is provisioned
   by this source work. The requested display publisher is **lcfr ai**;
   `bundle.publisher` is product metadata, not proof of the
   certificate's legal subject.
2. Provision the Windows SDK SignTool and expose `signtool.exe` on the
   maintainer's PATH or supply `-SignToolPath` with its absolute ordinary path,
   alongside the existing pinned release build tools. Root independently found
   a valid Microsoft-signed SDK tool at
   `C:\Program Files (x86)\Windows Kits\10\bin\10.0.19041.0\x64\signtool.exe`,
   outside PATH; this is an existing prerequisite, not a new installation. The
   builder uses `/sha1` to select that public thumbprint, `/fd SHA256`,
   `/tr <HTTPS endpoint>` and `/td SHA256`. Each SignTool invocation has a
   120-second deadline, including Tauri's object-form signing-command wrapper,
   with bounded process/read cleanup; failure or unknown
   cleanup stops release preparation, without an automatic retry. Timestamp and
   certificate trust/revocation services must be reachable during authorized
   signing. No timestamp or signing service is contacted by the offline tests.
3. Supply the verified source-bound runtime payload and pinned E2B source
   required by [the release builder](RELEASE.md), with clean version-aligned
   source and complete notices. LOCAL-047 producer denial and LOCAL-049 security
   hold remain open. Neither signing nor Microsoft's receipt of a sample would
   clear those holds or prove a clean verdict. The latest permitted exact-sample
   restore found no quarantined item, recovered no file and submitted nothing;
   do not rebuild or repackage to evade the unresolved detection.
4. Existing user authority covers signing, release preparation and eventual
   GitHub delivery once the real signer and exact safety/acceptance gates are
   satisfied; no repeated generic user approval is needed. Held artifact/runtime/
   package checks and the reviewed publication conditions still apply. New paid
   enrollment or cloud provisioning needs concrete cost and identity authority.
   Test the exact signed installed bytes on a clean Windows machine, including
   upgrade, retained private state, uninstall, ordinary verified runtime setup
   and actual upstream task completion before claiming release acceptance. No
   build, signing, publication, certificate purchase or account query occurred
   in this patch.

Azure Artifact Signing Public Trust is a possible **organization** route, not a
configured fallback. Microsoft's current prerequisites include an Azure
subscription and Microsoft Entra tenant ID, registered Microsoft.CodeSigning
resource provider, an Artifact Signing account in a supported region, suitable
resource-creation RBAC plus Artifact Signing Identity Verifier and Certificate
Profile Signer roles, portal-only identity
validation and a certificate profile. Australian organizations are eligible;
individual developer eligibility is currently limited to the United States and
Canada. Its managed signing integration is not implemented by this certificate-
store interface; it needs a separately reviewed signing-client/identity plan,
access and cost acknowledgement before any Azure queries or provisioning.
[Microsoft prerequisite and identity instructions](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart).

## Verification and evidence

The builder explicitly signs the Go core before `package-identity` and archives
are generated. It checks that core, the NSIS-source and runtime sidecar copies,
Tauri application and installer with `signtool verify /pa /all /tw` plus
`Get-AuthenticodeSignature`: trusted status, exact expected signer and timestamp
signer with timestamping EKU are required. `nativeSigning.files` binds each role
to its final byte length and SHA-256. `timestampProtocol: RFC3161-requested`
describes the configured `/tr`/Tauri `tsp` operation; it does not claim an
independent ASN.1 timestamp-protocol audit. The generated uninstaller still
needs installed-byte verification during clean-host acceptance. No uninstaller
verification is claimed from checking only the outer installer.

Consumers should obtain the expected public signer identity/thumbprint and
SHA256SUMS from an independently trusted release channel, verify the download
hash, and inspect its Windows Digital Signatures details. For independent
maintainer verification of the exact installer:

```powershell
signtool.exe verify /pa /all /tw 'E:\reviewed\Luci_0.5.0_x64-setup.exe'
Get-AuthenticodeSignature -LiteralPath 'E:\reviewed\Luci_0.5.0_x64-setup.exe'
```

Do not disable Defender, create exclusions, install a test root or bypass a
security warning to make acceptance pass. Authenticode identifies the signer
and detects changes; it is separate from antivirus assessment and SmartScreen
reputation. A valid newly signed application may still lack reputation. No
warning-free installation or Microsoft clearance is promised.

Primary interfaces: [Microsoft SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool),
[Tauri Windows signing](https://v2.tauri.app/distribute/sign/windows/),
[Tauri NSIS installer](https://v2.tauri.app/distribute/windows-installer/).
The NSIS fields were also checked against the locally cached, Cargo-lock-pinned
`tauri-utils 2.10.1` configuration; welcome/finish hook ordering was checked
against the [pinned Tauri CLI 2.12.1 template](https://github.com/tauri-apps/tauri/blob/tauri-cli-v2.12.1/crates/tauri-bundler/src/bundle/windows/nsis/installer.nsi).
Offline fixture tests exercise refusals and
ordering; they do not establish real certificate, timestamp, SmartScreen or
installed-package acceptance.
