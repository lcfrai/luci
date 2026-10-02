# Luci application design

This is the implementation authority for the shipped Luci application. The older
design-system document records the original direction; this file resolves that
direction against the current product and UAT findings.

Luci brings a project brief, visual design, implementation tasks, agent terminals,
files, Git, checks and releases into one workspace. People should understand where
they are, what has been saved, and what they can do next without reading developer
documentation. Local operation has the lowest hosting cost. Cloud setup must show
the account, resources, domain and cost approval before creating resources.

## Visual language

Use the warm graphite surface ramp and teal interactive accent in
`desktop/src/styles/tokens.css`. Status uses text and shape as well as colour.
Helper text uses the readable ink floor; reserve faint ink for non-text structure.
Keep body copy at a readable size. Screens do not need decorative grain, gradients,
glass effects or extra marketing labels. Elevation is reserved for overlays.

Use the installed system sans-serif stack and system monospace stack so the app
works offline without buying fonts. Excalidraw's open-licensed design fonts are
bundled locally. Navigation icons use the shared SVG component and stroke weight.
Do not add an icon or component library for a control the existing system provides.

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

First-run onboarding offers three real paths: on this computer, guided setup in
the user's AWS/Azure account, or connection to an existing Luci workspace over
verified SSH followed by that workspace's account sign-in. Reveal only the fields
for the selected path and let people change their choice. Do not imply a hosted
lcfr.ai subscription or URL-only account connection that the product does not offer.

## Verification

Use the connected browser UAT, real Docker browser journey, real SSH workspace
journey and installed Windows lifecycle checks. Screenshots establish appearance;
API, filesystem, process and container outcomes establish behaviour. Automated axe
checks supplement keyboard and visual checks; they do not certify accessibility.
Record external account prerequisites rather than claiming paid inference, cloud
creation or DNS/TLS work passed without exercising those accounts.
