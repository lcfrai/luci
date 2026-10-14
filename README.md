# Luci

**One conversation, from an idea to an application.**

Luci is an open-source workspace for planning, designing, building and delivering software. Your Luci keeps the purpose of a project, its decisions, design, implementation and reviews together, so you can keep improving the same idea instead of rebuilding context for every tool.

## What you can do

- **Discover the right thing to build.** Discuss the problem, explore requirements and record the decisions that matter.
- **Design how it should work.** Develop the experience, screens and acceptance criteria alongside the conversation.
- **Develop with your chosen coding agent.** Give Codex, Claude Code or a supported OpenCode connection the project's context and an isolated working environment. Review the resulting changes and run checks against the recorded source.
- **Deploy reviewed work.** Preview an application, configure its runtime and publish through a supported deployment target under your approval and spending policy.
- **Change direction without starting over.** Bring a new request back into the conversation. Luci tracks the updated context, affected work and decisions still needing review.

Discover, Design, Develop and Deploy are connected activities you can revisit. You can switch between projects, prioritize work, inspect outputs and recover interrupted tasks. Projects remain ordinary files and Git repositories. Team workspaces add shared, versioned context and distinct permissions for editing, execution, secrets and review.

Luci coordinates existing coding harnesses; model access comes from the connection you choose. Model subscriptions, API usage and any cloud resources have their own costs.

## Choose where to run

First setup offers three options:

| Option | How it works | Availability |
| --- | --- | --- |
| **Local** | Luci runs on your computer. Coding work runs in isolated Docker environments with configurable CPU, memory and scheduling limits. | The local application is the current testing candidate. |
| **Bring your own cloud** | Use your own infrastructure and account. Guided setup provides commands to prepare the environment and connect it to Luci. | Azure is the first target. Its setup and runtime integration are being prepared; live Azure acceptance is still pending. AWS is deferred. |
| **Luci Cloud** | A future hosted service operated by Luci. | Not available yet. We will develop and validate the service before launch. |

Running an application you built and running Luci's development workers are separate choices. A deployment target does not, by itself, supply a cloud coding environment. The app should only show a connection as ready after the relevant checks succeed.

## Get started

1. Get the Windows installer or portable package from the [releases page](https://github.com/lcfrai/luci/releases), using the candidate and setup notes supplied with that build. For a portable package, extract it before running its launcher.
2. Create your account and choose **Local** or **Bring your own cloud**. Setup explains the required tools and connection steps. Local coding work needs a running Docker engine; a local model is optional.
3. Connect a supported coding harness and model. Codex subscription sign-in and API-based access are separate connection choices.
4. Create a project, clone an existing repository, or open your work. Tell Luci what you want to build.

The interface and saved projects remain available when a worker must wait for capacity. Resource settings control concurrent work and retain space for the rest of your computer. Local model requirements depend on the model and available CPU/GPU memory; using a connected model provider is a separate choice from where coding work runs.

For setup details, source builds and package prerequisites, see the [installation and release guide](docs/luci/RELEASE.md). The [Docker distribution](distribution/docker/README.md) can also package the application and its private execution engine without host Go, Node.js or Python.

## Your work and your control

Luci keeps project context, task history and reviewed outputs together. Editing, running code, using secrets and publishing are distinct permissions. A completed coding task is not automatically a passed check or an approved release. Uncertain external outcomes remain visible so they can be reconciled without silently repeating an action.

Local use has no Luci licence or platform subscription fee. Review the costs of your selected model, cloud account and deployment services before enabling them. A configured budget is an application control, not a promise that a cloud provider cannot charge more.

## Project status and contributing

Local automated acceptance and the next-machine testing handoff are in progress. Azure source and fixture checks are not proof of a live cloud deployment, per-task microVM execution, retirement or final billing. The future Luci Cloud hosted service has not launched.

Contributors can start with the [product contract](docs/luci/PRODUCT-CONTRACT.md), [implementation map](docs/luci/LC-IMPLEMENTATION-MAP.md), [API documentation](docs/luci/API.md), [reuse decisions](docs/luci/REUSE.md) and [contributor instructions](AGENTS.md). Technical evidence and historical plans live under `docs/`; they are separate from the application's user experience.

Luci's own code uses the [MIT licence](LICENSE). Third-party components retain their original licences and notices.
