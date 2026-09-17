FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY sandboxd/go.mod sandboxd/go.sum ./sandboxd/
RUN cd sandboxd && go mod download
COPY sandboxd ./sandboxd
RUN cd sandboxd && CGO_ENABLED=0 go build -o /out/sandboxd ./cmd/sandboxd

FROM node:20-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps \
    && rm -rf /var/lib/apt/lists/*

# Agent harnesses. Pinned so a sandbox image is reproducible.
RUN npm install -g @anthropic-ai/claude-code@latest

RUN useradd -m -u 1001 -s /bin/bash dev
COPY --from=build /out/sandboxd /usr/local/bin/sandboxd
COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER dev
WORKDIR /home/dev
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
