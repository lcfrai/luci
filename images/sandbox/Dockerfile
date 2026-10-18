FROM golang:1.26-bookworm AS build
WORKDIR /src
# Committed dependencies keep the Go build independent of module downloads.
COPY sandboxd ./sandboxd
RUN cd sandboxd && CGO_ENABLED=0 GOMAXPROCS=1 GOMEMLIMIT=512MiB GOFLAGS="-mod=vendor -p=1" go build -o /out/sandboxd ./cmd/sandboxd \
    && tar -C /usr/local -czf /out/go-toolchain.tar.gz go

# Claude Code 2.1.274 requires Node >=22 (verified with npm view). Node 24
# is a supported LTS line; the previous Node 20 image is end-of-life.
FROM node:24-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps util-linux python3 python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Agent harnesses. Pinned so a sandbox image is reproducible. Version
# observed via `npm view @anthropic-ai/claude-code version` on 2026-09-17.
RUN NODE_OPTIONS=--max-old-space-size=512 npm_config_jobs=1 npm install -g @anthropic-ai/claude-code@2.1.274 @openai/codex@0.160.0 opencode-ai@1.18.34
COPY images/sandbox/codex-native-task.cjs /usr/local/lib/luci/codex-native-task.cjs

RUN useradd -m -u 1001 -s /bin/bash dev
COPY --from=build /out/sandboxd /usr/local/bin/sandboxd
# A directory COPY makes the classic daemon materialize the entire Go tree
# outside the RUN memory/CPU limits. Transfer one compressed ordinary file,
# then expand it inside a resource-capped RUN instead. Keep the full toolchain.
COPY --from=build /out/go-toolchain.tar.gz /tmp/go-toolchain.tar.gz
RUN tar -C /usr/local -xzf /tmp/go-toolchain.tar.gz && rm /tmp/go-toolchain.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
# Source packages built on Windows may contain CRLF even before Git attributes
# take effect. Normalize it and make it readable by dev even when the source
# checkout uses a restrictive umask.
RUN sed -i 's/\r$//' /usr/local/bin/entrypoint.sh && chmod 0755 /usr/local/bin/entrypoint.sh

USER dev
WORKDIR /home/dev
# The control plane bind-mounts each worktree's conversation store over
# ~/.claude/projects, and over nothing else in ~/.claude: that directory is
# where claude keeps the transcripts --resume reads, and its siblings are where
# /login writes credentials. Docker creates a mount point's missing parents as
# root, which leaves ~/.claude unwritable by dev and breaks /login, so the image
# creates the path first, as dev.
RUN mkdir -p /home/dev/.claude/projects
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
