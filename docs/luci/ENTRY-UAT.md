# Entry and account acceptance

Entry acceptance distinguishes account setup from workspace setup. An existing
identity can remain even when the projects folder has never been configured.
That installation must offer a clear way forward without deleting saved data or
pretending it is a new workspace.

The cases below complement the Define, Design, Develop and Deploy journey. A
successful run records actual outcomes; merely listing a case here is not a claim
that a particular release passed it.

| State and user action | Required result | Executable evidence |
| --- | --- | --- |
| Fresh ordinary browser, no setup code | First-account guidance explains the launcher or administrator code; private content stays unavailable. | `scripts/entry-browser-eval.mjs` |
| Fresh browser with a stale saved session | The stale bearer is removed and first-account setup remains usable. | Browser entry receipt |
| Temporary control-plane network failure | Retry returns to the real entry form; it does not remain a sign-in-only screen with stale status. | Browser entry receipt; one deliberate transport fault |
| Manual first-account code | Confirmed password and password visibility work; create, sign out and sign in return to the same identity. | Browser entry receipt |
| Portable launcher fragment | The actual one-time code is prefilled, removed from the URL and used only when creating the account. | Browser entry receipt |
| OS-owned portable launcher | Its private per-launch owner proof is removed from the URL and enables local account creation/recovery only on canonical loopback. Recovery keeps the original project and revokes the old password and live sessions. | Browser entry receipt; foreign-origin rejection in frontend tests |
| Initialized browser with stale credentials | Create-account and recovery-help choices remain visible. Ordinary browser and bearer requests cannot use native account creation or owner recovery. | Browser entry receipt; actual HTTP capability rejection |
| Browser invitation admission | A real administrator-issued code creates one member. Invalid, used, revoked and expired codes cannot authenticate. | Browser entry receipt and management tests |
| Installed app, existing owner, rejected password | The OS-owned native control creates a distinct local account; the original account and its projects remain intact. | `scripts/native-installed-eval.mjs`, candidate 0.4.2 or later |
| Installed app, account created but setup interrupted | Sign out before choosing a projects folder, reject incorrect credentials, and retain create/recovery choices. Valid sign-in returns to unsaved setup choices. | Native installed receipt |
| Installed app owner recovery | Recovery returns to the original owner ID and preserves brief, drawing, Git and private application configuration. The old password fails and the replacement succeeds. | Native installed receipt |
| Existing SSH workspace sign-in | Wrong remote credentials preserve local sign-in; correct credentials activate the verified remote workspace and save durable remote work. | `scripts/workspace-browser-eval.mjs` |
| Selected remote Admin invitation | Invitation is issued on the selected remote control plane, redeemed through the actual SSH connection, and creates a distinct remote member. Returning local preserves both realms. | SSH browser receipt |
| Invited member's first cloud project | The assigned workspace is immediately ready. Cloud controls show the member's actual permissions; the member creates and saves a project without server setup, and owner/member project reads stay isolated in both directions. | SSH browser receipt and real supervised-worker management test |
| Native fresh install, restart, reinstall and upgrade | Known and recovered credentials continue to work after replacement. Uninstall removes owned application resources while preserving user data. | `scripts/test-installed-windows.ps1` |

`scripts/run-ui-uat.mjs` runs the independent browser entry fixture before the
existing application journey. The fixtures are fresh private directories under
`artifacts/uat`; they never reuse a normal user profile. Invitation expiry is
tested by stopping only that disposable service, setting one saved invitation's
expiry in the past, and restarting. The system clock and production endpoints are
unchanged. Passwords, raw invitation codes and session tokens are omitted from
receipts.

Installed tests use the actual release WebView and production native commands.
They run only on disposable Windows CI runners. The old 0.3/0.4.1 binaries may be
used to seed an upgrade; their lack of newer controls is explicit. New candidates
must provide the controls and cannot silently skip their acceptance.

The manual installed diagnostic also accepts `upgrade_from_release=v0.4.1-rc.4`
to reproduce an existing 0.4.1 owner's upgrade to the new entry controls. The
workflow independently pins that previous installer by SHA256 and size, checks
its release manifest, and asserts the version of the executable actually run.
The old UI creates its owner, project, brief, drawing and Git commit; the new UI
then exercises unknown credentials, separate local account creation and recovery
of that exact saved owner ID. The retained `upgrade_from_03` route remains a
separate legacy migration case; the two previous-release choices are exclusive.

Ordinary browser access does not imply control of the operating-system account.
Uninvited local creation and owner recovery require the installed app's private
native capability or the explicit portable launcher's private, per-launch proof.
The latter is enabled only for that launcher, loaded from its protected state file
and accepted by the UI only on canonical loopback. It is distinct from the native
capability, which is never written to a file. Remote account creation uses an
invitation, and local owner recovery must never reset a remote account. Live model
quality, cloud purchases and public DNS/TLS are outside these account tests.

The original owner's unsaved first setup and a managed account's assigned
workspace are different states. Acceptance reads the current account's real
setup status before dismissing a dialog; it must not assume every new account
opens the same onboarding screen or force a click through a backdrop.
