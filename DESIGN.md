# Luci application design

**Latest direction, 15 October 2026:** [FIVE D AMENDMENT 2026 10 15](docs/luci/FIVE-D-AMENDMENT-2026-10-15.md) controls the five-D workspace extension, project home/pins/activity, document-led Discover, editable Design canvas, Cube-style Develop and Distribute with team XLSX controls. LC-02 is explicitly amended; LC-17–22 are added. Existing LC-01 and LC-03–16 obligations and live/resource/security gates remain. Root records exact scope review and worker ACKs before new-scope implementation; existing local/auth fixes continue. Prior stage-count wording below is historical where it conflicts. Usable local end-to-end development is the delivery priority.

This is the UI design guidance subordinate to [the controlling product contract](docs/luci/PRODUCT-CONTRACT.md). The owner scope audit passed for b3ecb011 on12October2026; aligned implementation is authorized again. One persistent conversational Luci per person is the primary interface; the five stages are revisitable views/tools of its context, upstream execution and actual policy-permitted delivery loop. Team coordination and elastic per-task cloud microVM swarms belong in that same accessible journey, with clear authority/cost/resource controls. The older design-system document retains historical direction; no static screen or current fixture establishes shipped product acceptance.

Luci brings a project brief, visual design, implementation tasks, agent terminals,
files, Git, checks and releases into one workspace. People should understand where
they are, what has been saved, and what they can do next without reading developer
documentation. Local operation has the lowest hosting cost. Cloud setup must show
the account, resources, domain and cost approval before creating resources.

## Visual language

The owner's 7 October identity selection is **Fourfold**, with four equal parts
forming one continuous loop. Canonical source and motion direction are in
[docs/luci/IDENTITY.md](docs/luci/IDENTITY.md). The activities are now **Discover,
Design, Develop, Deploy**, concurrent and iterative. This supersedes earlier
Define/Describe vocabulary and prior logo directions. The asset handoff does
not itself change the running app or supply an animation.

Use the warm graphite surface ramp and teal interactive accent in
`desktop/src/styles/tokens.css`. Status uses text and shape as well as colour.
Helper text uses the readable ink floor; reserve faint ink for non-text structure.
Keep body copy at a readable size. Screens do not need decorative grain, gradients,
glass effects or extra marketing labels. Elevation is reserved for overlays.

The owner's **8 October 2026 shadcn/ui requirement supersedes the earlier
Fluid Functionalism component preference**. Use reviewed, pinned MIT-licensed
shadcn/ui components and consistent accessible interaction patterns for the
interactive local product, including cross-project actions and attention queues.
The shared button, dialog, input, textarea and badge implementations now live in
`desktop/src/components/ui`, with immutable upstream originals and verified MIT
provenance in `third-party/shadcn-ui`. The old Fluid button/dialog paths are
compatibility adapters to these controls. Retained Fluid tabs, size context and
icons remain separately attributed. Preserve working behavior, keyboard/focus
handling, loading/error states and upstream grants as shared controls migrate.
Keep one coherent token/density system; do not append
an unrelated theme or replace working actions with static demonstrations.
Compile utilities without replacing the existing reset and map the chosen
shadcn components to Luci's appearance tokens and density.
Inter's OFL-licensed variable font is bundled locally so the app works offline.
Terminal text remains monospace. Navigation retains the shared Luci SVG icons.

Everyday use should be calm, clear and easy. Show one main action and the result
that matters; place runtime details, revisions and recovery tools behind deliberate
disclosures. Develop is the exception: show the user's actual CLI work and output.
Do not hide failures, invent progress or replace working actions with visual demos.

## Layout and interaction

At wide widths, projects and the project journey have persistent navigation.
Below 1100 CSS pixels the journey becomes a horizontal navigation strip. Below
760 pixels, projects open in a drawer with an explicit toggle, focus containment,
Escape dismissal and focus return. The workspace remains usable at narrow widths
and 200% equivalent reflow. Touch controls have a 44-pixel target.

Put the main action before optional configuration. Forms share field, help,
validation and button styles. Pending operations freeze submitted inputs; status
and errors are announced. Keep an unsuccessful draft visible and offer a specific
recovery action. Private runtime values clear after submission and are never read
back into the UI. Switching workspaces must preserve separate local and remote
identities.

Account entry must explain which workspace the account belongs to. An empty
installation opens account creation; a configured installation shows sign-in,
Create account and Forgot sign-in. Never assume the person opening a saved
installation remembers its credentials. The owned desktop service can create a
separate local account or recover the original owner's account while preserving
saved work. Shared workspaces accept administrator-issued invitations and explain
how to request a password reset. Keep first-run proof automatic when the launcher
provides it, with manual setup help only when needed. Signing out after setup and
opening with an expired session must return to the correct entry state.

First-run onboarding presents Local, Bring your own cloud (Azure first, with
guided commands), and Luci Cloud (clearly unavailable until developed). Existing
workspace connections remain a supported secondary route. Reveal only the fields
for the selected path and let people change their choice. Keep cloud readiness
and required authority truthful; no hosted subscription is currently available.

## Verification

Use the connected browser UAT, real Docker browser journey, real SSH workspace
journey and installed Windows lifecycle checks. Screenshots establish appearance;
API, filesystem, process and container outcomes establish behaviour. Automated axe
checks supplement keyboard and visual checks; they do not certify accessibility.
Record external account prerequisites rather than claiming paid inference, cloud
creation or DNS/TLS work passed without exercising those accounts.
