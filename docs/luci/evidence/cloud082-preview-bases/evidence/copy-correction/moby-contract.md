# COPY flag contract behind the regression

Source review, not a Docker daemon execution. Docker v28.3.3 vendor.mod:65 pins github.com/moby/buildkit v0.23.2; its classic builder imports that Dockerfile instruction/parser implementation (builder/dockerfile/dispatchers.go:25-27).

Docker COPY flags may contain quoted values. Splitting a quoted flag on whitespace loses the flag boundary: in `COPY --chown="0:0 " --from=alpine:3.22 /etc/os-release /out`, whitespace splitting produces `--chown="0:0`, then `"`, then `--from=alpine:3.22`. The previous Luci scan stopped on `"` and did not inspect the external --from.

The consequence does not depend on chown succeeding. The pinned classic builder resolves CopyCommand.From at dispatchers.go:115-119, before assigning Chown at :127 and before performCopy invokes parseChownFlag at internals.go:160-161. getImageMount uses localOnly only for a completed stage (dispatchers.go:140-149). For an external image, imagecontext.go:28-34 chooses PullOptionPreferLocal even when PullParent is false, so absence may trigger an implicit pull.

The focused regression calls PreparePreview with a warm FROM plus the problematic COPY and requires refusal before any image inspection or mutation. It proves the application-side parser gap and its correction; it does not run Moby's parser or claim a real image was pulled. The correction deliberately supports whole quoted flag values without whitespace while rejecting quoted-whitespace, embedded quoting, and escaped flag ambiguities before any other flag is skipped. Source operands, including JSON paths with spaces, remain untouched.
