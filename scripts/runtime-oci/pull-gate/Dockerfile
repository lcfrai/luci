FROM scratch
COPY --chown=65532:65532 luci-registry-gate /luci-registry-gate
COPY --chown=65532:65532 registry /usr/local/bin/registry
USER 65532:65532
WORKDIR /
EXPOSE 5000/tcp
ENTRYPOINT ["/luci-registry-gate"]
