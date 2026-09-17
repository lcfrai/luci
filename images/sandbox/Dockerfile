FROM golang:1.23-bookworm AS build
WORKDIR /src
# Build hermetically from the committed vendor/ directory: the corporate
# proxy intercepts TLS to proxy.golang.org (module downloads fail cert
# verification), so `go mod download` cannot run in this build environment.
# Vendoring removes the network dependency entirely.
COPY sandboxd ./sandboxd
RUN cd sandboxd && CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -o /out/sandboxd ./cmd/sandboxd

FROM node:20-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps \
    && rm -rf /var/lib/apt/lists/*

# Agent harnesses. Pinned so a sandbox image is reproducible. Version
# observed via `npm view @anthropic-ai/claude-code version` on 2026-09-17.
RUN npm install -g @anthropic-ai/claude-code@2.1.274

RUN useradd -m -u 1001 -s /bin/bash dev
COPY --from=build /out/sandboxd /usr/local/bin/sandboxd
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER dev
WORKDIR /home/dev
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
