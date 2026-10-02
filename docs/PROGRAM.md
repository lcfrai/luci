# Luci delivery programme

The programme is local-first, open source and owned by lcfr.ai. See [the current plan](luci/PLAN.md) for scope and [API.md](luci/API.md) for contracts.

1. Preserve and rebrand the working local runtime and terminal cockpit.
2. Make setup guide users through project location, Git identity, Docker runtime and optional provider sign-in.
3. Connect Define, Design, Develop and Deploy through real versioned artifacts.
4. Build local previews from committed source, verify HTTP health and keep release provenance.
5. Support an explicitly selected cloud workspace through verified SSH: projects, agent sessions and Docker previews run on that machine while the desktop remains local. Reuse Coolify for public application deployment, with account-owned AWS/Azure single-server provisioning, DNS guidance and cleanup.
6. Package the same application as portable downloads and a native desktop installer. Publish only artifacts that passed the acceptance checks.

Each step retires a concrete risk and produces an observable outcome. Run useful integration tests and complete journeys; avoid repeated reviews that do not add evidence. A unit test with a fake provider does not prove a real cloud rollout. Record those limits in the release evidence.

The implementation now includes the journey APIs, cloud provider adapters, provider login jobs, SSH connections, a cloud workspace installation/selection path, and review-artifact packaging. See [the release guide](luci/RELEASE.md) for build commands and remaining acceptance gates. These implemented paths must not be described as live-cloud or real-Docker validated on the basis of local fixtures. The development host's Docker engine is currently unavailable; Docker session/preview acceptance and both real provider journeys remain pending. Public source/history review, release signing and actual download publication also remain separate work.

Historical enterprise programme assumptions are preserved in `legacy/PROGRAM-panacea-2026-09.md`.
