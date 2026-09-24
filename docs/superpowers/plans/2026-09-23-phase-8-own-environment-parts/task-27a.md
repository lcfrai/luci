### Task 27a: The sandbox base moves off Node 20 before its first publish (review m15)

The sandbox image is `FROM node:20-bookworm-slim` (`images/sandbox/Dockerfile:10`), by tag.
Node 20 reached end of life on **2026-04-30** (from training knowledge, UNVERIFIED). Task 27
builds the image unchanged, and task 31 is where it is first published, into a regulated
environment. So the base moves **before that publish**, and it moves in its own task and commit
because **the sandbox image's digest is OD-8's adapter pin** (task 27, `:7-8`). A base change
moves the pin even when the Claude Code version does not change. It must therefore run the OD-8
canary on its own, and not ride inside another change.

**The choice (recommended; one `ARG` is the seam).** `node:24-trixie-slim`, pinned by digest.
Node 24 is the Active LTS line, with end of life on 2028-04-30. Debian 13 "trixie" is the current
stable. Bookworm's regular security support ended in 2026-06 and it is now in Debian LTS, which
covers fewer packages. All three facts are from training knowledge, UNVERIFIED, and step 1 checks
them. `node:22-bookworm-slim` is the smaller move, but Node 22 reaches end of life in 2027-04 and
bookworm stays on LTS coverage, so that move would have to be made again within a year.

**The ripple into the machine image.** Task 27 installs git from `bookworm-backports`, because
bookworm's git is 2.39 and `gitx.MinVersion` is 2.40.0 (task 27 `:25-31`). Trixie's own git is
the 2.47 line (from training knowledge, UNVERIFIED). If step 1 confirms it, the machine image
drops the backports line, since a `bookworm-backports` source on a trixie base fails at
`apt-get update`. If step 1 does not confirm it, the backports line becomes `trixie-backports`.
Either way, the existing git ≥ 2.40.0 assertion decides, not this paragraph.

**Files:**
- `images/sandbox/Dockerfile` — `ARG NODE_BASE=node:24-trixie-slim@sha256:<digest>`, then `FROM ${NODE_BASE}` for the runtime stage; nothing else changes (modify)
- `images/machine/Dockerfile` — the git source follows the base, as above (modify, task 27)
- `images/images_test.sh` — four rows (modify, task 27)

#### Tests to write first — rows added to `images/images_test.sh`

| Assertion | Mutant it kills |
|---|---|
| `docker run --rm --entrypoint node panacea-app/sandbox:<sha> --version` starts with `v24.` | a base left on an end-of-life Node line |
| the runtime `FROM` in `images/sandbox/Dockerfile` resolves to a reference carrying `@sha256:` (the `ARG` default is read, not the literal `FROM` line) | a tag-only base that drifts under the same pin |
| `/etc/os-release` `VERSION_CODENAME` of the sandbox and machine images is the same, and no file under `/etc/apt/sources.list.d/` in the machine image names another codename | a `bookworm-backports` source left on a trixie base, which breaks the machine build's `apt-get update` |
| `docker run --rm --entrypoint claude panacea-app/sandbox:<sha> --version` is `2.1.274` | a base move that quietly changed the CLI pin too; OD-8's adapter version must be the same, and only the digest moves |

Task 27's `git version ≥ 2.40.0` row and its "machine CLI equals sandbox CLI" row stay, and must
still pass.

#### Steps

- [ ] **Step 1:** Confirm the facts above from the source: the Node release schedule
  (github.com/nodejs/Release), Debian's security-support dates, and `git --version` in
  `node:24-trixie-slim`. Record the three answers and the base's digest in the task report. A
  different answer changes the recommended tag, and the report says which.
- [ ] **Step 2:** The four rows; `images_test.sh` fails on the first and second (the base is
  still `node:20-bookworm-slim`, by tag).
- [ ] **Step 3:** The two Dockerfile edits; `bash images/build.sh`; `images_test.sh` passes.
- [ ] **Step 4:** The OD-8 canary against the new sandbox image, locally, in whatever form phase 3
  task 17 defines. If that task has not landed, record `canary: phase 3 task 17 not present —
  skipped`, exactly as task 27's pipeline job does.
- [ ] **Step 5:** One live session on the **local** Docker backend with the new image, following
  `docker-daemon-lock.md` exactly (plan "Rules"): the session attaches and `claude --version`
  prints `2.1.274` inside it. The same Dockerfile serves phase 2's laptop backend, so this is the
  laptop's regression check.
- [ ] **Step 6:** Commit, naming the old and new digests in the message: the OD-8 pin moved.

#### Acceptance

```bash
bash images/build.sh                  # four images, tagged with the git sha
bash images/images_test.sh            # images_test: 11 of 11 passed
```

#### What this task does not claim

- That Claude Code `2.1.274` supports Node 24. It is expected, since the CLI needs Node 18 or
  later (from training knowledge, UNVERIFIED), and step 5 is the check.
- That the new base has no CRITICAL finding on the day `images` first scans it (task 27's rule
  still fails the run if it does).
- A policy for the next end of life. The plan's seam S8-I.1 (task 33 writes it into
  `docs/SEAMS.md`) says the base is re-chosen whenever its Node line enters maintenance, and that
  each move is its own task, because of OD-8.
