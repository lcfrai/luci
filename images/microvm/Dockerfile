# Candidate root filesystem for the pinned E2B Embed template builder. The
# provider still has to inject and verify envd, KVM/cgroup mounts and private
# management networking; building this OCI image alone is not a microVM.
FROM golang:1.26-bookworm AS guest-build
WORKDIR /src
COPY control-plane/go.mod control-plane/go.sum ./control-plane/
RUN cd control-plane && go mod download
COPY control-plane ./control-plane
COPY sandboxd ./sandboxd
RUN cd control-plane && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/luci-microvm-bootstrap ./cmd/microvm-bootstrap \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/luci-microvm-collect ./cmd/microvm-collect \
    && cd /src/sandboxd && CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -trimpath -o /out/sandboxd ./cmd/sandboxd

FROM node:24-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps util-linux python3 python3-venv \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code@2.1.274 @openai/codex@0.160.0 opencode-ai@1.18.34
RUN useradd -m -u 1001 -s /bin/bash dev
COPY --from=guest-build /out/luci-microvm-bootstrap /usr/local/bin/luci-microvm-bootstrap
COPY --from=guest-build /out/luci-microvm-collect /usr/local/bin/luci-microvm-collect
COPY --from=guest-build /out/sandboxd /usr/local/bin/sandboxd
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN sed -i 's/\r$//' /usr/local/bin/entrypoint.sh \
    && chmod 0755 /usr/local/bin/entrypoint.sh /usr/local/bin/luci-microvm-bootstrap /usr/local/bin/luci-microvm-collect /usr/local/bin/sandboxd \
    && install -d -m 0700 /luci-transfer \
    && install -d -m 0755 /luci-work /home/dev/.luci-reviewed-context \
    && install -m 0600 /dev/null /luci-transfer/input.tar \
    && install -d -o dev -g dev -m 0700 /home/dev/.claude /home/dev/.claude/projects
USER root
WORKDIR /luci-work
# The authenticated envd Start RPC invokes the fixed bootstrap. Booting the
# image must not run a coding task or read a project archive on its own.
CMD ["/bin/sleep", "infinity"]
