#!/usr/bin/env python3
"""Run the Linux installed workflow and its independent resource observer.

Arguments after -- are passed to four-project-queue.mjs. This wrapper owns a
fresh short fixture parent and publishes only its exact instance-file path to
the observer. It never reads provider credentials or changes Docker resources.
"""
import argparse
import contextlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--observation-dir', type=Path, required=True)
    parser.add_argument('--fixture-base', type=Path, required=True,
                        help='Existing roomy local directory, for example /workspace')
    parser.add_argument('driver_args', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    forwarded = args.driver_args
    if forwarded[:1] == ['--']:
        forwarded = forwarded[1:]
    if sys.platform != 'linux' or '--fixture-root' in forwarded:
        parser.error('Linux is required; the wrapper supplies its own --fixture-root')
    try:
        image = forwarded[forwarded.index('--expected-image-id') + 1]
    except (ValueError, IndexError):
        parser.error('Pass the driver --expected-image-id argument after --')
    output = args.observation_dir.resolve()
    pointer = output.with_name(output.name + '-instance-path.txt')
    if output.exists() or pointer.exists():
        parser.error('Use fresh observation and instance-path locations')
    base = args.fixture_base.resolve(strict=True)
    if not base.is_dir():
        parser.error('--fixture-base must be a directory')
    # The proxy adds a private Unix-socket directory below the driver fixture.
    template = base / '.lq-XXXXXXXX' / 'luci-swarm-execution-XXXXXX' / '.luci-docker-proxy-XXXXXX' / 'docker.sock'
    if len(os.fsencode(template)) >= 108:
        parser.error('--fixture-base is too long for the private Unix socket')
    output.parent.mkdir(parents=True, exist_ok=True)
    os.umask(0o077)
    fixture_parent = Path(tempfile.mkdtemp(prefix='.lq-', dir=base))
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    driver = observer = watcher = None
    failures = []
    stop = threading.Event()
    interrupted = None
    code = 1
    observed = 1

    def interrupt(signum, _frame):
        nonlocal interrupted
        interrupted = signum
        if 'Wrapper interrupted; driver cleanup requested' not in failures:
            failures.append('Wrapper interrupted; driver cleanup requested')
        if driver is not None and driver.poll() is None:
            try:
                driver.send_signal(signum)
            except ProcessLookupError:
                pass

    for signum in (signal.SIGTERM, signal.SIGINT):
        signal.signal(signum, interrupt)

    def publish_identity():
        try:
            while not stop.is_set() and driver.poll() is None:
                paths = list(fixture_parent.glob('luci-swarm-execution-*/state/control-plane/teams/team-*/runtime/state/instance/instance-id'))
                if len(paths) > 1:
                    raise RuntimeError('Multiple fixture instances; ownership cannot be inferred')
                if paths:
                    path = paths[0]
                    if path.is_symlink() or path.resolve() != path or not path.is_file():
                        raise RuntimeError('Fixture instance path is not an ordinary confined file')
                    with pointer.open('x') as stream:
                        stream.write(str(path) + '\n')
                    return
                stop.wait(0.1)
        except Exception as error:
            failures.append(str(error))
            if driver.poll() is None:
                try:
                    driver.send_signal(signal.SIGTERM)
                except ProcessLookupError:
                    pass

    try:
        if interrupted is not None:
            raise RuntimeError('Interrupted before driver startup')
        driver = subprocess.Popen(['node', str(here / 'four-project-queue.mjs'),
                                   *forwarded, '--fixture-root', str(fixture_parent)], cwd=repo)
        if interrupted is not None:
            raise RuntimeError('Interrupted during driver startup')
        start = Path('/proc', str(driver.pid), 'stat').read_text().rsplit(') ', 1)[1].split()[19]
        observer = subprocess.Popen([sys.executable, str(here / 'sample-linux-owned.py'),
                                     '--driver-pid', str(driver.pid), '--driver-start-ticks', start,
                                     '--instance-path-file', str(pointer), '--image-id', image,
                                     '--output', str(output), '--seconds', '1260'], cwd=repo)
        print(json.dumps({'driverPID': driver.pid, 'startTicks': start,
                          'samplerPID': observer.pid, 'observationDirectory': str(output)}), flush=True)
        watcher = threading.Thread(target=publish_identity, daemon=True)
        watcher.start()
        # Driver's own deadline is twenty minutes; leave room for its cleanup.
        code = driver.wait(timeout=1320)
    except BaseException as error:
        failures.append(type(error).__name__ + ': ' + str(error))
    finally:
        stop.set()
        if watcher is not None and watcher.ident is not None:
            try:
                watcher.join(timeout=2)
            except BaseException as error:
                failures.append('Identity watcher cleanup exception: ' + type(error).__name__)
        try:
            if driver is not None and driver.poll() is None:
                with contextlib.suppress(ProcessLookupError):
                    driver.send_signal(signal.SIGTERM)
                try:
                    driver.wait(timeout=180)
                except subprocess.TimeoutExpired:
                    # A forced driver exit cannot prove Docker cleanup. Preserve
                    # the fixture and report uncertainty for manual inspection.
                    failures.append('Driver cleanup exceeded 180 seconds; forced driver exit, Docker cleanup unknown')
                    with contextlib.suppress(ProcessLookupError):
                        driver.kill()
                    try:
                        driver.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        failures.append('Owned driver could not be reaped')
            elif driver is not None:
                driver.wait(timeout=5)
        except BaseException as error:
            failures.append('Driver cleanup exception; Docker cleanup unknown: ' + type(error).__name__)
        finally:
            # Observer cleanup must run even if driver signaling/reaping fails.
            try:
                if observer is not None:
                    try:
                        observed = observer.wait(timeout=30)
                    except subprocess.TimeoutExpired:
                        failures.append('Observer exceeded its settlement window; termination requested')
                        with contextlib.suppress(ProcessLookupError):
                            observer.terminate()
                        try:
                            observed = observer.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            with contextlib.suppress(ProcessLookupError):
                                observer.kill()
                            try:
                                observed = observer.wait(timeout=5)
                            except subprocess.TimeoutExpired:
                                failures.append('Owned observer could not be reaped')
            except BaseException as error:
                failures.append('Observer cleanup exception: ' + type(error).__name__)
        if not pointer.exists():
            failures.append('No owned team instance became available; no container coverage is claimed')
        if failures:
            print(json.dumps({'observerWrapperFailures': failures,
                              'privateFixtureParent': str(fixture_parent)}), file=sys.stderr)
    return code if code else (1 if observed or failures else 0)


if __name__ == '__main__':
    raise SystemExit(main())
