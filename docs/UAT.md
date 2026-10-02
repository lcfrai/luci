# Luci user acceptance testing

Use one matching release candidate: installer or portable download, source ZIP,
release manifest and checksums must identify the same source commit. The Windows
installer is unsigned. The local app runs without a cloud account; isolated agents,
project checks and app previews need a working Docker engine.

## Start and choose your workspace

1. Install Luci, or extract the portable archive and run its launcher. Open the
   address printed by the portable launcher. The installed Windows app opens its
   own desktop window and private local service.
2. Create your first local account. The first account administers this installation.
   The installed and portable launchers supply the first-run token automatically.
   If an account already exists, use **Create account** for a separate local
   account or **Forgot sign-in details** to recover the original owner and saved
   work. Reopen the local launcher if these computer-owned controls are unavailable.
3. Choose **On this computer**, **Set up a cloud workspace**, or **Connect to an
   existing workspace**. You can change your choice before saving.
4. For local use, choose a projects folder and check readiness. Open **Git identity,
   agent connection and runtime**, save the name/email for your commits, build the
   sandbox image, and connect an agent provider. Provider usage is billed by the
   selected provider; Luci does not include model credits.
5. Create a project. Save its brief in Define, draw and save its screens in Design,
   then open Develop to manage tasks, checks and agent sessions.

Guided cloud setup uses **your AWS or Azure account**. Select your provider, sign
in through its CLI, choose a region and domain, and preview the cloud plan. Read
the resource and cost confirmation before creating anything. A monthly planning
budget is not a provider billing cap. The wizard then verifies SSH access, installs
the private Luci workspace and guides its account and deployment connection.

An existing workspace needs the server address, SSH user/key and independently
verified host fingerprint supplied by its administrator. After connecting, sign
in with **that workspace's Luci account**, or use **Create account** with the
administrator's invitation code, then select **Use this cloud workspace**.
Your local account remains separate. This is an existing Luci-server connection;
there is no hosted lcfr.ai subscription or URL-only invitation flow in this release.
An invited member starts ready in a private projects folder assigned on that
server. Create a project without choosing another server or folder. Settings shows
the active server account; its permissions govern workspace and runtime controls.

## Acceptance scenarios

| Scenario | Outcome to check |
| --- | --- |
| First-run choices | Each path explains its requirements before showing fields; Back returns to the choices. |
| Existing local account | Create a separate account or recover the original owner without deleting saved work; the old password stops working after recovery. |
| Shared account invitation | Accept the administrator's code on the intended workspace; used, expired and revoked codes cannot create another account. |
| Define and Design | Brief and drawing save, reload and reopen after an app restart. |
| Develop | Tasks retain their state. The selected branch/worktree and agent settings are clear. |
| Actual agent work | A connected provider edits the selected project, produces usable output and retains its conversation after restart/fork. Use your account to judge model quality. |
| Project checks | Run the application's meaningful checks in Docker; inspect output and failures. Task completion is independent from a passed check. |
| Files and Git | Files show saved output. Review and commit changes, then connect/push the reviewed commit to your repository. |
| Runtime configuration | Save a private variable. Its value clears and is never displayed again; variable names remain. Changes affect the next build/deployment. |
| Local preview | Build committed code, wait for health to pass, open the embedded preview, use the app, then Stop it. |
| Backup and restore | Download the archive and restore under a new name. Brief, design, plan and Git history return. Runtime variables, application databases and volumes need separate recovery. |
| Existing workspace | Incorrect remote credentials leave the local login intact. Correct credentials activate the remote workspace; Return to this machine restores local projects. |

See [the complete entry matrix](luci/ENTRY-UAT.md) for interrupted setup, stale
sessions, rejected passwords, local recovery and shared invitation acceptance.
| Accounts | A member cannot see another user's projects or administrative controls. Admin actions follow the displayed role and policy. |
| Installer lifecycle | Restart, reinstall and upgrade preserve user projects and identity. Uninstall removes app resources and preserves user data. |
| Keyboard and reflow | Tab into project navigation, use arrows, open/close the mobile drawer, and use forms at narrow widths and 200% zoom. |
| Your cloud/domain | Create approved resources in your account, set the instructed DNS records, verify HTTPS and deploy/stop/rollback the reviewed application. |

## Reproducible engineering acceptance

The release workflow runs the complete browser journey, real Linux Docker/terminal
and iframe journey, real OpenSSH workspace/account journey, multi-user control
checks, packaged startup on Windows/Linux/macOS, and installed Windows WebView2
and NSIS lifecycle tests. Outcomes, screenshots and process/container evidence are
uploaded by that exact run. Provider toolchain tests use isolated deterministic
API fixtures and prove integration, not paid model quality.

To repeat the connected browser journey with extracted release assets and Node
dependencies installed in the checkout:

```text
node scripts/run-ui-uat.mjs <compiled-luci-service> <shipped-web-folder> <shipped-runtime-folder>
```

The harness refuses to reuse a profile and writes its evidence into a fresh
`artifacts/uat/<id>/evidence` folder. The installed Windows harness is restricted
to disposable Windows GitHub runners. Docker and SSH scenarios run as a separate
ordinary Linux user. Live model accounts, provider purchases and public DNS/TLS
require the selected operator's credentials and are recorded separately from
these engineering results.
