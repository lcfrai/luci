# Luci delivery programme

The programme is local-first, open source and owned by lcfr.ai. See [the current plan](luci/PLAN.md) for scope and [API.md](luci/API.md) for contracts.

1. Preserve and rebrand the working local runtime and terminal cockpit.
2. Make setup guide users through project location, Git identity, Docker runtime and optional provider sign-in.
3. Connect Define, Design, Develop and Deploy through real versioned artifacts.
4. Build local previews from committed source, verify HTTP health and keep release provenance.
5. Support an explicitly selected cloud workspace through verified SSH: projects, agent sessions and Docker previews run on that machine while the desktop remains local. Reuse Coolify for public application deployment, with account-owned AWS/Azure single-server provisioning, DNS guidance and cleanup.
6. Package the same application as portable downloads and a native desktop installer. Publish only artifacts that passed the acceptance checks.

Each step retires a concrete risk and produces an observable outcome. Run useful integration tests and complete journeys; avoid repeated reviews that do not add evidence. A unit test with a fake provider does not prove a real cloud rollout. Record those limits in the release evidence.

The implementation now includes the journey APIs, cloud provider adapters, provider login jobs, SSH connections, a cloud workspace installation/selection path, and review-artifact packaging. [CI run 36953123907](https://github.com/lcfrai/luci/actions/runs/36953123907) at commit `4c117f6a088189f640ad946459f36c9072d344c1` passed the real Linux Docker shell/Git/transcript/preview journey, Linux and macOS packaged-service checks, and the Windows native installer build. The Docker probes used no model inference. The development host's Docker engine remains unavailable; a model-backed agent journey, clean-host installer evaluation and both real cloud-provider journeys remain pending. See [the release guide](luci/RELEASE.md) for the exact evidence and remaining gates. Source/history review, release signing and actual download publication also remain separate work.

Historical enterprise programme assumptions are preserved in `legacy/PROGRAM-panacea-2026-09.md`.
