# Stable project selection and client keyboard labels

Selecting a project no longer toggles its tree expansion. Expansion has its own
labelled disclosure control and Left/Right keyboard behavior; an expanded row
with no loaded children does not focus an unrelated project. The Commands action
and palette now show shortcuts for the client keyboard; Settings no longer
displays a shortcut that actually opens Commands.

[Evidence](2026-10-08-local007-navigation.json) records 185 affected tests and
TypeScript passing with 418 unchanged frontend inputs. A controlled real-source
Chromium fixture reproduced late-branch displacement of the next project by
120 px at 1440 px width and 132 px at 390 px width. With the correction, all four
project hit rectangles stayed unchanged through selection-only coordinate
clicks. Disclosure, keyboard/focus and Ctrl+K actions passed; axe found zero
violations and neither viewport overflowed horizontally. Initial test-helper and
test-only type errors remain in the private failed receipts.

This does not prove the cause of the reported native WebDriver misselection.
Explicit or previously saved expansion can still add rows. No native WebView,
backend, worker or model ran in this fixture. The manifest identifies the tested
working-tree bytes on the recorded base commit; a later source commit packages
those changes. Native retesting remains open.
