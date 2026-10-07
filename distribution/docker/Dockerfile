# syntax=docker/dockerfile:1
# Release users build from the bounded bundle prepared by prepare-docker-release.py.
FROM docker:28.5.1-cli@sha256:9190b0613792e658a7783cf14b2d5ace5941bb68ede7276922ea36ee457d76ad AS docker_tools
FROM docker:28.5.1-dind@sha256:ea9d20492ca1caaaba78e68453433895d256173c79281756e88b745647fcbcfd AS engine
RUN addgroup -S -g 1001 luci
COPY workspace-init.sh engine-entrypoint.sh /usr/local/bin/
RUN chmod 0755 /usr/local/bin/workspace-init.sh /usr/local/bin/engine-entrypoint.sh
ENTRYPOINT ["/usr/local/bin/engine-entrypoint.sh"]
CMD ["dockerd", "--host=unix:///run/luci-engine/docker.sock", "--group=luci", "--storage-driver=overlay2"]

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS runtime_tools

ARG TARGETARCH
RUN test "$TARGETARCH" = amd64
ENV DEBIAN_FRONTEND=noninteractive \
    AWS_EC2_METADATA_DISABLED=true \
    AZURE_CORE_COLLECT_TELEMETRY=0
RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
       ca-certificates curl git openssh-client unzip less groff jq tini \
    && rm -rf /var/lib/apt/lists/*
COPY --from=docker_tools /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker_tools /usr/local/libexec/docker/cli-plugins/ /usr/local/libexec/docker/cli-plugins/

# Fixed official assets and hashes, verified when this distribution source was prepared.
RUN curl --fail --location --retry 3 --output /tmp/gh.tar.gz \
      https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_amd64.tar.gz \
    && echo 'bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386  /tmp/gh.tar.gz' | sha256sum --check --strict \
    && mkdir /tmp/gh \
    && tar -xzf /tmp/gh.tar.gz --strip-components=1 -C /tmp/gh \
    && install -m 0755 /tmp/gh/bin/gh /usr/local/bin/gh \
    && mkdir -p /usr/local/share/licenses/github-cli \
    && cp /tmp/gh/LICENSE /usr/local/share/licenses/github-cli/LICENSE \
    && rm -rf /tmp/gh /tmp/gh.tar.gz
RUN curl --fail --location --retry 3 --output /tmp/aws.zip \
      https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.37.9.zip \
    && echo '6b3a6a3d7bb3997928f0bdf7b866914224abf242c2e54e1dcebbe84bee64f356  /tmp/aws.zip' | sha256sum --check --strict \
    && unzip -q /tmp/aws.zip -d /tmp \
    && /tmp/aws/install --install-dir /usr/local/aws-cli --bin-dir /usr/local/bin \
    && rm -rf /tmp/aws /tmp/aws.zip
RUN curl --fail --location --retry 3 --output /tmp/azure-cli.deb \
      https://packages.microsoft.com/repos/azure-cli/pool/main/a/azure-cli/azure-cli_2.88.0-1~bookworm_amd64.deb \
    && echo 'e589a155989afbde1ab88bd5b8669a1de2cd5582260e9eb446584506214d4b2e  /tmp/azure-cli.deb' | sha256sum --check --strict \
    && apt-get update \
    && apt-get install --yes --no-install-recommends /tmp/azure-cli.deb \
    && rm -rf /var/lib/apt/lists/* /tmp/azure-cli.deb
RUN docker --version && docker compose version && git --version \
    && gh --version && aws --version && az version

FROM runtime_tools AS controller
COPY bundle/ /opt/luci/
COPY entrypoint.sh healthcheck.sh /usr/local/bin/
RUN chmod 0755 /opt/luci/luci /usr/local/bin/entrypoint.sh /usr/local/bin/healthcheck.sh \
    && groupadd --gid 1001 luci \
    && useradd --uid 1001 --gid 1001 --no-create-home --home-dir /luci-workspace/home --shell /bin/sh luci \
    && test -x /opt/luci/luci \
    && test -s /opt/luci/web/index.html \
    && test -s /opt/luci/runtime/images/sandbox/Dockerfile \
    && test -s /opt/luci/LICENSE
ENV HOME=/luci-workspace/home \
    TMPDIR=/luci-workspace/tmp \
    LUCI_STATE_DIR=/luci-workspace/state \
    LUCI_PROJECTS_DIR=/luci-workspace/projects \
    LUCI_WEB_DIR=/opt/luci/web \
    LUCI_RUNTIME_DIR=/opt/luci/runtime \
    LUCI_CLOUD_BUNDLE=/opt/luci/cloud/luci-cloud-linux-amd64.tar.gz \
    LUCI_BACKEND=docker \
    DOCKER_HOST=unix:///run/luci-engine/docker.sock \
    LUCI_DOCKER_HOST=unix:///run/luci-engine/docker.sock
WORKDIR /luci-workspace
USER 1001:1001
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
