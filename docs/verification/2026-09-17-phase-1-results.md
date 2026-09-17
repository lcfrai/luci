# Phase 1 verification results — 2026-09-17

**Phase 1's defining risk is retired.** A real, unmodified Claude Code CLI was driven through the PTY-over-WebSocket pipe, in a Linux container, with full terminal fidelity.

Run from the development machine (Windows) against Docker Desktop 29.4.3 (linux/x86_64). Not yet run against AKS — see the remaining gaps at the end.

---

## What was tested

Three runs of the same harness, against progressively more real targets:

| Target | Result |
|---|---|
| Cross-compiled `sandboxd` in `debian:bookworm-slim` | 7/7 |
| Sandbox image (proof-of-concept vendored Dockerfile) | 9/9 |
| **Sandbox image built from the committed Dockerfile** | **9/9** |

## The checks

Against `panacea-app/sandbox:final`, built from `images/sandbox/Dockerfile` at commit `5390a4b`:

| Check | Result | Why it matters |
|---|---|---|
| Container starts, `/healthz` | ✅ 200 | the server runs in the image |
| `claude --version` as uid 1001 | ✅ `2.1.274 (Claude Code)` | the pinned agent CLI is present and runs non-root |
| WebSocket upgrade on `/attach` | ✅ | the transport works |
| Resize observed by the process | ✅ `stty size` → `40 120` | the full resize path works, not just the frame |
| ANSI colour passes through | ✅ | escape sequences survive framing |
| **Box drawing (UTF-8) intact** | ✅ `╭─────╮` | this is Claude Code's input frame |
| **Cursor control sequences intact** | ✅ `\033[2J\033[H` | TUI redraw works |
| Exit status propagates | ✅ `RC=1` | shell semantics preserved |
| 500-line burst not truncated | ✅ | backpressure does not drop output |
| **Claude Code runs through the PTY** | ✅ | the actual product goal |
| **Claude Code help renders** | ✅ | multi-line TUI output renders |
| Ctrl-C interrupts, shell survives | ✅ | control characters reach the process |

Box drawing and cursor control are the two that decide whether Claude Code's `╭─ > ─╮` frame renders correctly or turns to mush. Both pass.

## Blocker found and fixed: corporate TLS interception

The image build initially failed:

```
go: github.com/coder/websocket@v1.8.12: Get "https://proxy.golang.org/...":
    tls: failed to verify certificate: x509: certificate signed by unknown authority
```

Scoped precisely from inside a container: the **npm registry is reachable**, `proxy.golang.org` is **not** (`curl` → `000`). Only the Go module download was affected.

Fixed by **vendoring** (`sandboxd/vendor/`, 81 files, 289K, two modules) rather than by injecting a CA. Vendoring makes the image build hermetic and reproducible, which is the right answer for a locked-down enterprise build rather than a workaround. The workspace build is unaffected — Go ignores a member module's vendor directory when a `go.work` is present, so local and CI builds still use the module cache.

## Two false results caught during testing

Recorded because both would have produced a wrong conclusion:

1. **A false positive.** An early `healthz: 200` came from a leftover `sandboxd.exe` left running on the host from an earlier task's testing — the container had actually failed to start. Caught by checking which process held the port. Without that check, the PTY would have been "proven" against a process that was not running.
2. **Two false negatives, both harness bugs.** Debian's `printf` does not expand `\u` escapes (so the box-drawing check failed on a test defect, not a product defect), and `coder/websocket` tears down the whole connection when a read context is cancelled (so one timeout cascaded into three spurious failures). Both fixed before drawing conclusions.

## What is still unproven

Everything below needs an AKS cluster. The runbook at `docs/verification/phase-1-operator-runbook.md` covers it, and Stages 1 and 1b of that runbook are now **complete** — only the cluster stages remain.

1. **Kata microVM isolation.** The image and PTY are proven under a normal container runtime. They are not yet proven under `runtimeClassName: kata-mshv-vm-isolation`.
2. **The RuntimeClass name.** Nothing confirms that Terraform's `workload_runtime = "KataVmIsolation"` produces a RuntimeClass registered as `kata-mshv-vm-isolation`, which `control-plane/internal/sandbox/manager.go` hardcodes. If it differs, every sandbox pod stays Pending with "RuntimeClass not found".
3. **Kata cold-start duration.** Needed to size warm pools in Phase 2.
4. **The read-only-root volume fix in practice.** The pod spec mounts `emptyDir` at `/home/dev` and `/tmp` under a read-only root; unit-tested, not yet run in-cluster.
5. **The Tauri desktop window.** `npm run build`, `tsc --noEmit` and `cargo check` all pass, and the protocol mirror is byte-identical to the Go side, but no human or agent has seen the native window render a terminal.

## Reproducing this

```bash
docker build -f images/sandbox/Dockerfile -t panacea-app/sandbox:dev .
docker run --rm --entrypoint bash panacea-app/sandbox:dev -lc 'id -u; claude --version'
docker run -d --rm --name pa -p 8099:8080 panacea-app/sandbox:dev
curl -s -o /dev/null -w '%{http_code}\n' localhost:8099/healthz
# then connect any WebSocket client to ws://localhost:8099/attach
docker rm -f pa
```

Note: the image's `ENTRYPOINT` execs `sandboxd` and ignores `CMD`, so one-off commands need `--entrypoint bash`. Without it the container simply starts a server and appears to hang.
