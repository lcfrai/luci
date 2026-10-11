from pathlib import Path
import datetime, json, os, shutil, signal, subprocess, time

base = Path(__file__).resolve().parent
source = base / 'source'
receipts = base / 'receipts'
overrides = {
    'PATH': '/workspace/.luci-tools/bin:/workspace/.luci-tools/go1.26.3/bin:' + os.environ['PATH'],
    'GOMAXPROCS': '2', 'GOFLAGS': '-mod=readonly -p=1', 'GOTOOLCHAIN': 'local',
    'GOPROXY': 'off', 'GOWORK': str(base / 'go.work'),
    'GOCACHE': str(base / 'go-cache'), 'GOTMPDIR': str(base / 'go-tmp'),
    'TMPDIR': str(base / 'tmp'), 'npm_config_cache': str(base / 'npm-cache'),
    'GOMODCACHE': '/tmp/luci-retained-tools/go-mod', 'GOPATH': '/tmp/luci-gopath',
    'PYTHONDONTWRITEBYTECODE': '1',
}
argv = ['bash', 'scripts/build-release.sh', '0.5.0', 'linux-amd64']
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
start = time.monotonic()
invocation = {'argv': argv, 'cwd': str(source), 'environment_overrides': overrides,
              'started_utc': now(), 'workspace_floor_bytes': 1536 << 20,
              'tmp_floor_bytes': 384 << 20, 'wall_timeout_seconds': 1800}
(receipts / 'build-invocation.json').write_text(json.dumps(invocation, indent=2) + '\n')
reason = None
samples = []
with (receipts / 'build.stdout').open('wb') as stdout, (receipts / 'build.stderr').open('wb') as stderr:
    process = subprocess.Popen(argv, cwd=source, env={**os.environ, **overrides},
                               stdout=stdout, stderr=stderr, start_new_session=True)
    (receipts / 'build-process.json').write_text(json.dumps({'pid': process.pid, 'process_group': process.pid}) + '\n')
    while process.poll() is None:
        sample = {'elapsed_seconds': round(time.monotonic() - start, 2),
                  'workspace_free_bytes': shutil.disk_usage('/workspace').free,
                  'tmp_free_bytes': shutil.disk_usage('/tmp').free}
        samples.append(sample)
        (receipts / 'build-progress.json').write_text(json.dumps(sample) + '\n')
        if sample['workspace_free_bytes'] < (1536 << 20) or sample['tmp_free_bytes'] < (384 << 20):
            reason = 'free-space floor reached'
        elif sample['elapsed_seconds'] > 1800:
            reason = 'wall timeout reached'
        if reason:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            break
        time.sleep(5)
    exit_code = process.wait()
result = {'exit_code': exit_code, 'stop_reason': reason, 'finished_utc': now(),
          'elapsed_seconds': round(time.monotonic() - start, 2),
          'minimum_workspace_free_bytes': min(x['workspace_free_bytes'] for x in samples),
          'minimum_tmp_free_bytes': min(x['tmp_free_bytes'] for x in samples),
          'source_commit': '80462898735a191cceaa4cb33e486b42b0a35f3c'}
(receipts / 'build-disk-samples.json').write_text(json.dumps(samples, indent=2) + '\n')
(receipts / 'build-result.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2), flush=True)
raise SystemExit(exit_code if exit_code >= 0 else 1)
