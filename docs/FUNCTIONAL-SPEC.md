# Luci functional specification

Luci takes one project through Define, Design, Develop and Deploy in a consistent application. It works locally by default and offers a guided deployment using the user's own AWS or Azure resources and domain.

## Setup

The app starts its own local service. First run explains local/cloud choices, saves the projects folder, checks dependencies, records the person's Git identity, generates an optional SSH key and installs the bundled agent runtime when Docker is ready. Settings offers the same controls without closing terminal sessions. Provider authentication uses the provider's supported CLI/browser flow.

## Define and Design

Create or clone an HTTPS Git repository without overwriting existing files. Define saves a revisioned brief and `LUCI.md`; stale edits produce a conflict instead of losing changes. Design embeds a themed Excalidraw editor and persists its scene alongside the brief revision. Exported content must escape project text and use a restrictive content policy.

## Develop

The project catalogue resolves branches/worktrees through Git. Coding and shell sessions stream real PTY output; transcripts survive service restarts. Files and Git tools show repository content, changes and history, and commit with the person's identity. Coding harnesses receive a prompt to read the canonical project brief. File reads reject traversal, symlinks and known credential locations.

## Deploy

A local preview builds a bounded committed snapshot, with no host mounts and a port exposed only on loopback. Release history records the exact commit and brief revision. HTTP health must succeed before readiness is shown. Stop acts only on containers Luci owns.

Remote deployment uses a verified Coolify target and a Git repository that server can access. Luci tracks provider IDs, avoids duplicate resources on retries and verifies public HTTPS after provider completion. A domain conflict is reported for resolution.

The cloud wizard creates complete AWS Lightsail or Azure VM templates. It shows cost assumptions, requires explicit acknowledgement, refuses existing resource names, persists operation progress, guides SSH host trust and DNS, and verifies ownership before deletion. Initial Coolify account setup is shown through a protected management connection, inside Luci's setup flow.

## Release gates

A downloadable build must pass typechecking, applicable Go/frontend tests, packaging, a real local UI journey and restart persistence checks. Real container-session and AWS/Azure rollout results are recorded separately. Code and package notices retain their dependencies' licences. No private credentials or corporate deployment endpoints belong in a public release.

Current implementation details live in `luci/API.md`; historical specification is archived in `legacy/FUNCTIONAL-SPEC-panacea-2026-09.md`.
