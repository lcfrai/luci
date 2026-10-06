# Luci user acceptance testing

Use one matching release candidate: installer or portable download, source ZIP,
release manifest and checksums must identify the same source commit. The Windows
installer is unsigned. The local app runs without a cloud account; isolated agents,
project checks and app previews need a working Docker engine.

The completion standard is [the product-quality contract](luci/PRODUCT-QUALITY.md).
The scenarios below support it; a clean launch or passing fixture cannot substitute
for a representative, understandable idea-to-live journey with the real selected
model. Preserve source identities and record assistance, confusion and recovery,
as well as technical success.

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
| Codex subscription | Complete official device sign-in and Save Codex sign-in in Settings. Start a new implementation session; its recorded auth mode is ChatGPT. A saved API key must not replace the subscription. Stop before starting another session and check retained sign-in after restart. |
| Project checks | Run the application's meaningful checks in Docker; inspect output and failures. Task completion is independent from a passed check. |
| Files and Git | Files show saved output. Review and commit changes, then connect/push the reviewed commit to your repository. |
| Runtime configuration | Save a private variable. Its value clears and is never displayed again; variable names remain. Changes affect the next build/deployment. |
| Local preview | Build committed code, wait for health to pass, open the embedded preview, use the app, then Stop it. |
| Public static deployment | Publish the reviewed commit to a public GitHub repository. In Deploy, choose GitHub Pages and acknowledge public publication. Wait for the exact commit and HTTPS content check, then use the public site. Later default-branch pushes republish the committed /docs folder. |
| Backup and restore | Download the archive and restore under a new name. Brief, design, plan and Git history return. Runtime variables, application databases and volumes need separate recovery. |
| Existing workspace | Incorrect remote credentials leave the local login intact. Correct credentials activate the remote workspace; Return to this machine restores local projects. |

| Accounts | A member cannot see another user's projects or administrative controls. Admin actions follow the displayed role and policy. |
| Installer lifecycle | Restart, reinstall and upgrade preserve user projects and identity. Uninstall removes app resources and preserves user data. |
| Keyboard and reflow | Tab into project navigation, use arrows, open/close the mobile drawer, and use forms at narrow widths and 200% zoom. |
| Your cloud/domain | Create approved resources in your account, set the instructed DNS records, verify HTTPS and deploy/stop/rollback the reviewed application. |

See [the complete entry matrix](luci/ENTRY-UAT.md) for interrupted setup, stale
sessions, rejected passwords, local recovery and shared invitation acceptance.

The real subscription acceptance harness is `scripts/subscription-journey-eval.mjs`.
Run its prepare, auth, develop, verify and deploy stages against an intended Luci
test workspace; `--help` lists the environment requirements. Its `results.json`
cannot pass without real subscription activity, Docker command checks, matching
committed previews and a functioning public HTTPS site. A prerequisite pause stays
pending, and provider fixtures cannot satisfy this gate.

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

## Connected build, verification and preview regression

`scripts/project-journey-browser-uat.mjs` exercises a fresh local account and
project entirely through visible controls: provider settings, definition/design,
reviewed task plan and build location, actual Codex execution, a meaningful failed
check, a second coding attempt that repairs application behavior without changing
the acceptance test, matching check evidence, preview and explicit acceptance of
the original run. API/file reads observe identities and invariants; they do not
create the project or substitute for its user actions.

Run against a matching built service and web directory with an existing trusted
sandbox image and existing Node preview image:

```sh
node scripts/project-journey-browser-uat.mjs \
  --binary artifacts/browser-uat/luci --web desktop/dist \
  --image luci/sandbox:dev --release-image node:22-alpine
```

The driver refuses missing images rather than pulling them. Use an exclusive
Docker window and enough free storage; a vfs host needs room for one full sandbox.
Completed coding sandboxes are stopped through the UI before the next allocation.
Fixture cleanup checks process/container ownership. Supply `--chromium` when
using an explicitly installed browser and record its version in the result.

This deterministic local provider supplies a deliberately small implementation
with an injected behavioral defect. It proves UI continuity and real execution,
not model reasoning, authentication/data durability, public deployment or full
product readiness. Those remain separate complete journeys in the quality
contract. Preserve failed receipts and their source/build identities.
