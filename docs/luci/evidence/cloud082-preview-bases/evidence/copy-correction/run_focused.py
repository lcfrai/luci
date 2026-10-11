import hashlib, json, os, pathlib, subprocess, sys, time
root = pathlib.Path("/workspace/luci/control-plane")
out = pathlib.Path("/workspace/.luci-tools/cloud082-preview-bases/copy-flags") / sys.argv[1]
out.mkdir(parents=True, exist_ok=True)
if (out / "invocation.json").exists():
    raise SystemExit("Refusing to overwrite an existing invocation")
paths = sorted((root / "internal/backend/dockerbe").glob("*.go")) + [root / "internal/backend/previews.go", root / "go.mod", root / "go.sum"]
def bind():
    rows = []
    for path in paths:
        data = path.read_bytes()
        rows.append({"path": str(path), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()})
    return rows
def save(name, value):
    (out / name).write_text(json.dumps(value, indent=2) + "\n")
before = bind()
save("before-source-bindings.json", before)
settings = {"GOMAXPROCS":"2", "GOTOOLCHAIN":"local", "GOPROXY":"off", "GOTMPDIR":"/workspace/.t", "GOCACHE":"/workspace/.luci-tools/cloud082-preview-bases/copy-flags/rebuild-cache", "GOMODCACHE":"/tmp/luci-retained-tools/go-mod", "GOWORK":"/workspace/.luci-tools/go.work", "GOFLAGS":"-mod=readonly", "PATH":"/workspace/.luci-tools/go1.26.3/bin:" + os.environ.get("PATH", "")}
command = ["/workspace/.luci-tools/go1.26.3/bin/go", "test", "-json", "-mod=readonly", "-count=1", "-p=1", "-timeout=120s", "-run", "^TestPreviewCopyQuoted(FlagsRefusedBeforeInspection|StageFlagsRemainSupported)$", "./internal/backend/dockerbe"]
save("invocation.json", {"cwd":str(root), "argv":command, "environment_overrides":settings, "started_utc":time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())})
env = os.environ.copy()
env.update(settings)
started = time.monotonic()
exit_code = None
try:
    with (out / "stdout.jsonl").open("wb") as stdout, (out / "stderr.txt").open("wb") as stderr:
        result = subprocess.run(command, cwd=root, env=env, stdout=stdout, stderr=stderr, check=False)
        exit_code = result.returncode
finally:
    after = bind()
    save("after-source-bindings.json", after)
    evidence = []
    for artifact in sorted(out.iterdir()):
        if artifact.is_file():
            data = artifact.read_bytes()
            evidence.append({"path":str(artifact), "bytes":len(data), "sha256":hashlib.sha256(data).hexdigest()})
    summary = {"exit_code":exit_code, "elapsed_seconds":round(time.monotonic()-started,3), "sources_unchanged":before==after, "source_file_count":len(before), "finished_utc":time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "artifacts":evidence}
    save("result.json",summary)
    print(json.dumps(summary))
sys.exit(exit_code if exit_code is not None else 125)
