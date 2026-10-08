#!/usr/bin/env python3
"""External LOCAL-016 launcher. No Docker mutations; fresh private outputs only."""
import argparse
import ast
import datetime
import hashlib
import json
import math
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import tempfile
import time

PREPARATION = 120
WORKFLOW = 600
CLEANUP = 180
TOTAL = PREPARATION + WORKFLOW + CLEANUP
TERM_GRACE = 5
REAP = 5
OBSERVER_SETTLE = 30
POSTFLIGHT = 15
OBSERVER_SHA = '1eb5f48c3b9b1e248e15ed2d058181b96e36485d914304397f35427c9cac725d'
SAMPLER_PATH = 'scripts/acceptance/four-project-queue/sample-linux-owned.py'
DRIVER_PATH = 'scripts/acceptance/worker-recovery/installed-worker-recovery.mjs'
BASIS = {
    'control-plane/internal/resourcebudget/probe_linux.go': 'ed0294860d6c721b1fd8f96da8c09a0f4fecb68b991c66c3b8f524cda985c405',
    'control-plane/internal/resourcebudget/cgroup_linux.go': '1843acdc15db9f9876545a1393bf6c21b93bc01873a64d8651be3adb26b5a4d1',
}


def sha(value):
    return hashlib.sha256(value).hexdigest()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def ordinary(path, limit=4 * 1024 * 1024):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and path.resolve() == path,
            'Input is not an ordinary unlinked file: ' + str(path))
    require(info.st_size <= limit, 'Input exceeds file bound: ' + str(path))
    return path.read_bytes()


def digest_file(path, deadline):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and path.resolve() == path,
            'Package input is linked or nonordinary')
    value = hashlib.sha256()
    with path.open('rb') as stream:
        while block := stream.read(1024 * 1024):
            require(time.monotonic() < deadline, 'Preparation deadline expired')
            value.update(block)
    return value.hexdigest()


def write_json(path, value):
    data = (json.dumps(value, indent=2) + '\n').encode()
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600), 'wb') as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())


def identity(pid):
    value = Path('/proc', str(pid), 'stat').read_text().rsplit(') ', 1)[1].split()
    return {'pid': pid, 'startTicks': int(value[19]), 'parentPid': int(value[1]),
            'group': int(value[2]), 'session': int(value[3])}


def owned_signal(child, selected, signum):
    if child.poll() is not None:
        return False
    try:
        require(identity(child.pid) == selected, 'Owned child identity changed; refusing signal')
        child.send_signal(signum)
        return True
    except (ProcessLookupError, FileNotFoundError):
        return False


def parse_args(argv):
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument('--window-granted', action='store_true', required=True)
    for key in ('repo', 'output', 'fixture-base', 'binary', 'web', 'provenance', 'image-provenance'):
        parser.add_argument('--' + key, type=Path, required=True)
    for key in ('expected-source', 'expected-helper-source', 'expected-image-id'):
        parser.add_argument('--' + key, required=True)
    # Reject duplicates and alternate equals spellings instead of silently taking the last pin.
    options = [value for value in argv if value.startswith('--')]
    if len(options) != len(set(options)) or any('=' in value for value in options):
        parser.error('Pass each named option once, with separate values')
    args = parser.parse_args(argv)
    if sys.platform != 'linux' or os.getuid() == 0:
        parser.error('A non-root Linux runtime is required')
    for value in (args.expected_source, args.expected_helper_source):
        if not re.fullmatch('[a-f0-9]{40}', value):
            parser.error('Exact lowercase 40-character source commits are required')
    if not re.fullmatch('sha256:[a-f0-9]{64}', args.expected_image_id):
        parser.error('An exact sha256 image ID is required')
    for key in ('repo', 'fixture_base', 'binary', 'web', 'provenance', 'image_provenance'):
        path = getattr(args, key).absolute()
        if path.resolve(strict=True) != path:
            parser.error('Inputs must use their real absolute paths')
        setattr(args, key, path)
    args.output = args.output.absolute()
    if args.output.exists() or args.output.is_symlink() or args.output.parent.resolve(strict=True) != args.output.parent:
        parser.error('Use a fresh output path under an existing ordinary parent')
    if not args.repo.is_dir() or not args.fixture_base.is_dir() or not args.web.is_dir():
        parser.error('Repository, fixture base and web must be directories')
    socket = args.fixture_base / '.ls-XXXXXXXX' / 'luci-worker-recovery-XXXXXX' / '.luci-docker-proxy-XXXXXX' / 'docker.sock'
    if len(os.fsencode(socket)) >= 108:
        parser.error('Fixture base is too long for the private Unix socket')
    return args


def preflight(args, output, deadline):
    def git(*values):
        remaining = deadline - time.monotonic()
        require(remaining > 0, 'Preparation deadline expired')
        return subprocess.run(['git', *values], cwd=args.repo, check=True, stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL, timeout=min(15, remaining)).stdout
    require(git('rev-parse', '--show-toplevel').decode().strip() == str(args.repo), 'Repository root differs')
    require(git('rev-parse', 'HEAD').decode().strip() == args.expected_helper_source, 'Helper HEAD differs')
    require(git('status', '--porcelain', '--untracked-files=all') == b'', 'Helper checkout is dirty')
    require(git('rev-parse', args.expected_source + '^{commit}').decode().strip() == args.expected_source, 'Product commit is absent')
    sampler = ordinary(args.repo / SAMPLER_PATH)
    driver = ordinary(args.repo / DRIVER_PATH)
    require(sha(sampler) == OBSERVER_SHA, 'Sampler differs from successful DAG11 observer')
    for relative, data in ((SAMPLER_PATH, sampler), (DRIVER_PATH, driver)):
        require(git('show', args.expected_helper_source + ':' + relative) == data, 'Executed source differs from helper commit')
    for relative, digest in BASIS.items():
        require(sha(git('show', args.expected_source + ':' + relative)) == digest, 'Product memory formula differs from observer basis')
    package_raw = ordinary(args.provenance)
    image_raw = ordinary(args.image_provenance)
    package, image = json.loads(package_raw), json.loads(image_raw)
    require(package.get('sourceCommit') == args.expected_source and package.get('sourceDirty') is False, 'Package source pin is not clean and exact')
    require(digest_file(args.binary, deadline) == package.get('binarySHA256'), 'Package binary hash differs')
    require(digest_file(args.web / 'index.html', deadline) == package.get('webIndexSHA256'), 'Package web hash differs')
    require(args.web == args.binary.parent / 'web', 'Binary/web installation differs')
    require(image.get('candidateImage') == args.expected_image_id, 'Image receipt identity differs')
    exported = image.get('exportVerification', {})
    require(exported.get('status') == 'passed' and exported.get('candidateImage') == args.expected_image_id, 'Image export verification missing')
    sources = image.get('sourcesBefore', {})
    require(isinstance(sources, dict) and len(sources) > 10 and sources == image.get('sourcesAfter'), 'Image source fence differs')
    for relative, digest in sources.items():
        require(re.fullmatch(r'(sandboxd/|images/sandbox/)[A-Za-z0-9_./-]+', relative) and '..' not in Path(relative).parts,
                'Image source path is invalid')
        require(sha(git('show', args.expected_source + ':' + relative)) == digest, 'Image source does not match selected product')
    launcher_root = Path(__file__).resolve().parent
    manifest_raw = ordinary(launcher_root / 'launch-manifest.json')
    manifest = json.loads(manifest_raw)
    launcher_raw = ordinary(Path(__file__).resolve())
    cost_raw = ordinary(launcher_root / 'observer-self-cost.py')
    require(manifest['wrapperSHA256'] == sha(launcher_raw) and manifest['observerCostLauncherSHA256'] == sha(cost_raw), 'External launcher source fence differs')
    require(manifest['samplerSHA256'] == OBSERVER_SHA, 'External sampler pin differs')
    # Verify that the imported cost launcher is only a fixed Python program, then execute its exact frozen file later.
    ast.parse(cost_raw)
    for name, raw in (('accepted-wrapper.py', launcher_raw), ('accepted-observer-self-cost.py', cost_raw),
                      ('accepted-sampler.py', sampler), ('accepted-launch-manifest.json', manifest_raw)):
        (output / name).write_bytes(raw)
        os.chmod(output / name, 0o600)
    return {'productCommit': args.expected_source, 'helperCommit': args.expected_helper_source,
            'imageId': args.expected_image_id, 'driverSHA256': sha(driver), 'samplerSHA256': sha(sampler),
            'wrapperSHA256': sha(launcher_raw), 'observerCostLauncherSHA256': sha(cost_raw),
            'packageProvenanceSHA256': sha(package_raw), 'imageProvenanceSHA256': sha(image_raw),
            'binarySHA256': package['binarySHA256'], 'webIndexSHA256': package['webIndexSHA256'],
            'archiveSHA256': package['archiveSHA256'], 'imageSourceFiles': len(sources),
            'archiveMembers': 'Driver independently validates the frozen archive and every installed member before Docker allocation',
            'observerProductBasis': BASIS}


def main(argv=None):
    args = parse_args(sys.argv[1:] if argv is None else argv)
    os.umask(0o077)
    # Output must not itself dirty the selected checkout after its creation.
    if args.output.is_relative_to(args.repo):
        allowed = subprocess.run(['git', 'check-ignore', '-q', '--no-index', '--', str(args.output / 'wrapper-result.json')],
                                 cwd=args.repo, timeout=10, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        require(allowed.returncode == 0, 'Output inside repository must already be ignored')
    args.output.mkdir(mode=0o700)
    start = time.monotonic()
    deadline = start + TOTAL
    driver = observer = None
    driver_id = observer_id = None
    fixture_parent = binding = None
    interruption = None
    forwarded = False
    forced = False
    failures = []
    logs = []
    lifecycle = (args.output / 'wrapper-lifecycle.jsonl').open('x')
    journal_bytes = 0

    def record(event, **fields):
        nonlocal journal_bytes
        raw = json.dumps({'version': 1, 'event': event, 'at': now(), **fields}, separators=(',', ':')) + '\n'
        journal_bytes += len(raw.encode())
        require(journal_bytes <= 65536, 'Wrapper journal bound exceeded')
        lifecycle.write(raw)
        lifecycle.flush()
        os.fsync(lifecycle.fileno())

    def interrupt(signum, _frame):
        nonlocal interruption, forwarded
        if interruption is None:
            interruption = time.monotonic()
        record('wrapper-signal', signalNumber=signum)
        if driver is not None and driver_id is not None and not forwarded:
            forwarded = True
            owned_signal(driver, driver_id, signum)
            record('driver-first-interrupt', signalNumber=signum)

    for signum in (signal.SIGINT, signal.SIGTERM):
        signal.signal(signum, interrupt)
    record('wrapper-start', identity=identity(os.getpid()), budgetSeconds=TOTAL)
    pointer = args.output / 'instance-path.txt'
    cost = args.output / 'observer-self-cost.json'
    result_path = args.output / 'result' / 'result.json'
    try:
        binding = preflight(args, args.output, min(deadline, start + PREPARATION))
        write_json(args.output / 'source-binding-before.json', binding)
        require(interruption is None, 'Interrupted before runtime startup')
        fixture_parent = Path(tempfile.mkdtemp(prefix='.ls-', dir=args.fixture_base))
        write_json(args.output / 'ownership.json', {'version': 1, 'privateFixtureParent': str(fixture_parent),
                   'output': str(args.output), 'originalFixtureAccessed': False, 'preserveOnEveryOutcome': True})
        def logfile(name):
            stream = (args.output / name).open('xb')
            logs.append(stream)
            return stream
        command = ['node', str(args.repo / DRIVER_PATH), '--window-granted', '--repo', str(args.repo),
                   '--expected-source', args.expected_source, '--expected-helper-source', args.expected_helper_source,
                   '--expected-image-id', args.expected_image_id, '--binary', str(args.binary), '--web', str(args.web),
                   '--provenance', str(args.provenance), '--image-provenance', str(args.image_provenance),
                   '--report', str(result_path), '--fixture-root', str(fixture_parent), '--observer-instance-pointer', str(pointer)]
        driver = subprocess.Popen(command, cwd=args.repo, start_new_session=True,
                                  stdout=logfile('driver.stdout.log'), stderr=logfile('driver.stderr.log'))
        driver_id = identity(driver.pid)
        record('driver-started', identity=driver_id)
        require(interruption is None, 'Interrupted during driver startup')
        observer_seconds = math.ceil(max(1, deadline - time.monotonic())) + TERM_GRACE + REAP + 20
        observer = subprocess.Popen([sys.executable, str(args.output / 'accepted-observer-self-cost.py'),
                                     str(args.output / 'accepted-sampler.py'), str(cost),
                                     '--driver-pid', str(driver.pid), '--driver-start-ticks', str(driver_id['startTicks']),
                                     '--instance-path-file', str(pointer), '--image-id', args.expected_image_id,
                                     '--output', str(args.output / 'observer'), '--seconds', str(observer_seconds), '--include-checks'],
                                    cwd=args.repo, start_new_session=True,
                                    stdout=logfile('observer.stdout.log'), stderr=logfile('observer.stderr.log'))
        observer_id = identity(observer.pid)
        record('observer-started', identity=observer_id, maximumSeconds=observer_seconds)
        while driver.poll() is None:
            active_deadline = min(deadline, interruption + CLEANUP) if interruption is not None else deadline
            require(time.monotonic() < active_deadline, 'Driver total/interrupt-cleanup budget expired')
            try:
                driver.wait(timeout=min(1, max(.001, active_deadline - time.monotonic())))
            except subprocess.TimeoutExpired:
                pass
        record('driver-returned', exitCode=driver.returncode)
    except BaseException as error:
        failures.append(type(error).__name__ + ': ' + str(error)[:1000])
        record('wrapper-failure', errorType=type(error).__name__)
    finally:
        try:
            if driver is not None and driver.poll() is None:
                if driver_id is None:
                    driver_id = identity(driver.pid)
                if not forwarded:
                    forwarded = True
                    owned_signal(driver, driver_id, signal.SIGTERM)
                    record('driver-term-request', identity=driver_id)
                remaining = TERM_GRACE if time.monotonic() >= deadline else max(TERM_GRACE, min(CLEANUP, deadline - time.monotonic()))
                if interruption is not None:
                    remaining = min(remaining, max(TERM_GRACE, interruption + CLEANUP - time.monotonic()))
                try:
                    driver.wait(timeout=remaining)
                except subprocess.TimeoutExpired:
                    forced = True
                    failures.append('Forced driver exit; controller and Docker cleanup remain unknown')
                    owned_signal(driver, driver_id, signal.SIGKILL)
                    record('driver-kill-request', identity=driver_id)
                    driver.wait(timeout=REAP)
        except BaseException as error:
            forced = True
            failures.append('Driver settlement failed: ' + type(error).__name__)
        finally:
            try:
                if observer is not None:
                    if observer_id is None:
                        observer_id = identity(observer.pid)
                    try:
                        observer.wait(timeout=OBSERVER_SETTLE)
                    except subprocess.TimeoutExpired:
                        failures.append('Observer exceeded settlement budget')
                        owned_signal(observer, observer_id, signal.SIGTERM)
                        record('observer-term-request', identity=observer_id)
                        try:
                            observer.wait(timeout=TERM_GRACE)
                        except subprocess.TimeoutExpired:
                            owned_signal(observer, observer_id, signal.SIGKILL)
                            record('observer-kill-request', identity=observer_id)
                            observer.wait(timeout=REAP)
            except BaseException as error:
                failures.append('Observer settlement failed: ' + type(error).__name__)
        if interruption is not None:
            failures.append('Wrapper interrupted; attempt is unsuccessful')
        for stream in logs:
            stream.close()
        checks = {'driverReceiptPassed': False, 'sourceStable': False, 'observerComplete': False}
        coverage = {}
        try:
            require(binding is not None, 'Source preflight did not complete')
            require(sha(ordinary(args.repo / DRIVER_PATH)) == binding['driverSHA256'], 'Driver source changed')
            require(sha(ordinary(args.repo / SAMPLER_PATH)) == OBSERVER_SHA, 'Sampler source changed')
            require(sha(ordinary(Path(__file__).resolve())) == binding['wrapperSHA256'], 'Wrapper source changed')
            require(sha(ordinary(Path(__file__).resolve().parent / 'observer-self-cost.py')) == binding['observerCostLauncherSHA256'], 'Observer cost launcher changed')
            post_deadline = time.monotonic() + POSTFLIGHT
            for command, expected in ((['git', 'rev-parse', 'HEAD'], args.expected_helper_source.encode() + b'\n'),
                                      (['git', 'status', '--porcelain', '--untracked-files=all'], b'')):
                actual = subprocess.run(command, cwd=args.repo, check=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                        timeout=max(.001, post_deadline - time.monotonic())).stdout
                require(actual == expected, 'Helper checkout changed during run')
            checks['sourceStable'] = True
            write_json(args.output / 'source-binding-after.json', binding)
        except BaseException as error:
            failures.append('Final source check: ' + type(error).__name__ + ': ' + str(error)[:1000])
        try:
            result = json.loads(ordinary(result_path, 8 * 1024 * 1024))
            checks['driverReceiptPassed'] = result.get('passed') is True
            require(checks['driverReceiptPassed'], 'Installed scenario receipt did not pass')
        except BaseException as error:
            failures.append('Final driver check: ' + type(error).__name__ + ': ' + str(error)[:1000])
        try:
            instance_path = Path(ordinary(pointer, 4096).decode().strip())
            require(fixture_parent is not None and instance_path.is_relative_to(fixture_parent) and
                    instance_path.parts[-3:] == ('state', 'instance', 'instance-id'), 'Driver instance pointer is outside owned fixture')
            instance = ordinary(instance_path, 256).decode().strip()
            metadata = json.loads(ordinary(args.output / 'observer' / 'metadata.json'))
            summary = json.loads(ordinary(args.output / 'observer' / 'summary.json'))
            own_cost = json.loads(ordinary(cost))
            require(metadata['helperSHA256'] == OBSERVER_SHA and metadata['driver'] == {'pid': driver_id['pid'], 'startTicks': driver_id['startTicks']}, 'Observer identity/source mismatch')
            require(sha(ordinary(args.output / 'observer' / 'accepted-sampler.py')) == OBSERVER_SHA, 'Accepted observer source changed')
            require(summary.get('driverExitObserved') is True and summary.get('samples', 0) > 0, 'Observer did not complete post-exit coverage')
            require(summary.get('instanceIdentitySHA256') == sha(instance.encode()), 'Observer instance differs')
            require(own_cost.get('status') == 'completed' and own_cost.get('exitCode') == 0 and own_cost.get('pid') == observer_id['pid'], 'Observer self-cost receipt incomplete')
            coverage = {'samples': summary['samples'], 'containerCount': len(summary.get('containerIds', [])),
                        'driverExitObserved': True, 'containerIssueCounts': summary.get('containerIssueCounts', {}),
                        'controlPlaneCgroupMemory': metadata.get('controlPlaneCgroupMemory'),
                        'baseline': summary.get('baseline')}
            require(coverage['containerCount'] > 0, 'No exact owned container observation was retained')
            checks['observerComplete'] = True
        except BaseException as error:
            failures.append('Final observer check: ' + type(error).__name__ + ': ' + str(error)[:1000])
        passed = bool(driver is not None and driver.returncode == 0 and observer is not None and observer.returncode == 0
                      and all(checks.values()) and not failures)
        report = {'version': 1, 'passed': passed, 'exitCode': 0 if passed else 1, 'finishedAt': now(),
                  'elapsedSeconds': time.monotonic() - start, 'driverExitCode': driver.returncode if driver else None,
                  'observerExitCode': observer.returncode if observer else None, 'driverIdentity': driver_id,
                  'observerIdentity': observer_id, 'privateFixtureParent': str(fixture_parent) if fixture_parent else None,
                  'sourceBinding': binding, 'checks': checks, 'observerCoverage': coverage, 'failures': failures,
                  'budgetsSeconds': {'preparation': PREPARATION, 'workflow': WORKFLOW, 'cleanup': CLEANUP, 'driverTotal': TOTAL,
                                     'driverInterruptGrace': TERM_GRACE, 'driverReap': REAP, 'observerSettlement': OBSERVER_SETTLE,
                                     'observerTermGrace': TERM_GRACE, 'observerReap': REAP, 'postflight': POSTFLIGHT,
                                     'outerPlannedMaximum': TOTAL + TERM_GRACE + REAP + OBSERVER_SETTLE + TERM_GRACE + REAP + POSTFLIGHT},
                  'forcedExitCleanupUnknown': forced, 'originalFixtureAccessed': False,
                  'limitations': ['API-only scripted native-worker acceptance; no app-control/browser or model-quality claim',
                                  'Driver death is not evidence of controller or Docker cleanup; a forced exit preserves that uncertainty',
                                  'Observer starts after driver: no pre-run idle baseline; sampled counters do not reproduce a prior admission snapshot',
                                  'Private fixture and raw output are retained on every outcome; publish only reviewed allowlisted evidence']}
        record('wrapper-settled', passed=passed, driverExitCode=report['driverExitCode'], observerExitCode=report['observerExitCode'])
        write_json(args.output / 'wrapper-result.json', report)
        lifecycle.close()
        print(json.dumps({'passed': passed, 'report': str(args.output / 'wrapper-result.json'),
                          'driverExitCode': report['driverExitCode'], 'observerExitCode': report['observerExitCode']}), flush=True)
    return 0 if passed else 1


if __name__ == '__main__':
    raise SystemExit(main())
