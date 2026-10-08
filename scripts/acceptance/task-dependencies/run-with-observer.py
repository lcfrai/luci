#!/usr/bin/env python3
"""Run the installed same-project DAG workflow with the frozen independent resource observer.

Arguments after -- are passed to dag.mjs. This wrapper owns a
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
import time
import datetime

# Entire process budgets include preparation, workflow, and bounded cleanup.
PREPARATION_SECONDS = 120
WORKFLOW_SECONDS = 900
CLEANUP_SECONDS = 180
DRIVER_TOTAL_SECONDS = PREPARATION_SECONDS + WORKFLOW_SECONDS + CLEANUP_SECONDS
INTERRUPT_GRACE_SECONDS = 5



# The frozen sampler executes in this same interpreter. RUSAGE_SELF deliberately
# excludes its subprocesses and Docker daemon. Do not relabel it family cost.
SAMPLER_LAUNCHER = r"""
import datetime,json,os,resource,runpy,sys,time,signal
sampler,cost=sys.argv[1:3]
sys.argv=[sampler,*sys.argv[3:]]
started=time.monotonic()
status='completed'
exit_code=0
received_signal=None
def stop(signum,_frame):
    global received_signal
    received_signal=signum
    raise SystemExit(128+signum)
for signum in (signal.SIGTERM,signal.SIGINT):signal.signal(signum,stop)
try:
    runpy.run_path(sampler,run_name='__main__')
except SystemExit as error:
    exit_code=error.code if isinstance(error.code,int) else (0 if error.code is None else 1)
    status='completed' if exit_code==0 else 'failed'
    raise
except BaseException:
    status='failed'
    exit_code=1
    raise
finally:
    own=resource.getrusage(resource.RUSAGE_SELF)
    duration=time.monotonic()-started
    with open('/proc/self/stat') as status_file:
        start_ticks=int(status_file.read().rsplit(') ',1)[1].split()[19])
    clock_ticks=os.sysconf('SC_CLK_TCK')
    process_duration=time.clock_gettime(time.CLOCK_BOOTTIME)-start_ticks/clock_ticks
    receipt={'version':1,'status':status,'exitCode':exit_code,'pid':os.getpid(),'signalNumber':received_signal,
      'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),
      'scope':'Observer interpreter RUSAGE_SELF, including interpreter imports and launcher; excludes spawned Docker CLI/helpers and all Docker daemon work.',
      'wallSeconds':process_duration,'launcherWallSeconds':duration,'processStartClockTicks':start_ticks,'clockTicksPerSecond':clock_ticks,'userCPUSeconds':own.ru_utime,'systemCPUSeconds':own.ru_stime,
      'cpuSecondsPerWallSecond':(own.ru_utime+own.ru_stime)/process_duration if process_duration>0 else None,
      'maxRSSBytes':own.ru_maxrss*1024,'inputBlockOperations':own.ru_inblock,'outputBlockOperations':own.ru_oublock,
      'voluntaryContextSwitches':own.ru_nvcsw,'involuntaryContextSwitches':own.ru_nivcsw,
      'units':'Linux ru_maxrss KiB converted to bytes; block counts are I/O operations, not bytes. Lifetime wall uses CLOCK_BOOTTIME minus /proc creation ticks, with tick-resolution uncertainty.'}
    descriptor=os.open(cost,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(descriptor,'w') as output:
        json.dump(receipt,output,indent=2);output.write('\n')
"""


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
    self_cost = output.with_name(output.name + '-observer-self-cost.json')
    lifecycle_path = output.with_name(output.name + '-lifecycle.jsonl')
    if output.exists() or pointer.exists() or self_cost.exists() or lifecycle_path.exists():
        parser.error('Use fresh observation and instance-path locations')
    base = args.fixture_base.resolve(strict=True)
    if not base.is_dir():
        parser.error('--fixture-base must be a directory')
    # The proxy adds a private Unix-socket directory below the driver fixture.
    template = base / '.ls-XXXXXXXX' / 'luci-dag-XXXXXX' / '.luci-docker-proxy-XXXXXX' / 'docker.sock'
    if len(os.fsencode(template)) >= 108:
        parser.error('--fixture-base is too long for the private Unix socket')
    output.parent.mkdir(parents=True, exist_ok=True)
    os.umask(0o077)
    lifecycle = os.open(lifecycle_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_APPEND, 0o600)
    parent_fd = os.open(output.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:os.fsync(parent_fd)
    finally:os.close(parent_fd)
    journal_bytes = 0
    journal_failed = False
    def record(event, **fields):
        nonlocal journal_bytes, journal_failed
        if journal_failed:return False
        # Fixed event names and scalar numeric ownership/budget/exit fields only.
        allowed = {'pid','parentPid','group','session','startTicks','signalNumber','returnCode','driverPID','observerPID','workloadSeconds','cleanupSeconds','totalSeconds'}
        safe = {key:value for key,value in fields.items() if key in allowed and type(value) is int}
        value = {'version':1,'event':event,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),**safe}
        encoded = (json.dumps(value,separators=(',',':'))+'\n').encode()
        try:
            if len(encoded)>4096 or journal_bytes+len(encoded)>65536:raise RuntimeError('Wrapper lifecycle journal budget exceeded')
            if os.write(lifecycle,encoded)!=len(encoded):raise RuntimeError('Wrapper lifecycle journal short write')
            journal_bytes += len(encoded)
            os.fsync(lifecycle)
            return True
        except OSError:
            journal_failed = True
            return False
        except RuntimeError:
            journal_failed = True
            return False
    own_start = int(Path('/proc/self/stat').read_text().rsplit(') ',1)[1].split()[19])
    if not record('wrapper-start',pid=os.getpid(),parentPid=os.getppid(),group=os.getpgrp(),session=os.getsid(0),startTicks=own_start,workloadSeconds=WORKFLOW_SECONDS,cleanupSeconds=CLEANUP_SECONDS,totalSeconds=DRIVER_TOTAL_SECONDS):raise RuntimeError('Wrapper lifecycle journal unavailable before launch')
    fixture_parent = Path(tempfile.mkdtemp(prefix='.ls-', dir=base))
    here = Path(__file__).resolve().parent
    repo = Path(os.environ.get('LUCI_ACCEPTANCE_REPO', str(here.parents[2]))).resolve(strict=True)
    sampler = repo / 'scripts' / 'acceptance' / 'four-project-queue' / 'sample-linux-owned.py'
    if not sampler.is_file():
        parser.error('Set LUCI_ACCEPTANCE_REPO to the real checkout containing the frozen sampler')
    driver = observer = watcher = None
    failures = []
    stop = threading.Event()
    interrupted = None
    code = 1
    observed = 1
    driver_deadline = time.monotonic() + DRIVER_TOTAL_SECONDS
    total_budget_expired = False

    def interrupt(signum, _frame):
        nonlocal interrupted
        interrupted = signum
        record('wrapper-signal',signalNumber=signum)
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
                paths = list(fixture_parent.glob('luci-dag-*/state/instance/instance-id'))
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
        driver = subprocess.Popen(['node', str(here / 'dag.mjs'),
                                   *forwarded, '--fixture-root', str(fixture_parent)], cwd=repo)
        if interrupted is not None:
            raise RuntimeError('Interrupted during driver startup')
        start = Path('/proc', str(driver.pid), 'stat').read_text().rsplit(') ', 1)[1].split()[19]
        observer = subprocess.Popen([sys.executable, '-c', SAMPLER_LAUNCHER, str(sampler), str(self_cost),
                                     '--driver-pid', str(driver.pid), '--driver-start-ticks', start,
                                     '--instance-path-file', str(pointer), '--image-id', image,
                                     '--output', str(output), '--seconds', str(DRIVER_TOTAL_SECONDS + 15), '--include-checks'], cwd=repo)
        record('children-started',driverPID=driver.pid,observerPID=observer.pid)
        print(json.dumps({'driverPID': driver.pid, 'startTicks': start,
                          'samplerPID': observer.pid, 'observationDirectory': str(output), 'observerSelfCost': str(self_cost)}), flush=True)
        watcher = threading.Thread(target=publish_identity, daemon=True)
        watcher.start()
        # One explicit total budget, followed only by a five-second interrupt
        # grace. Do not add a second three-minute cleanup window after expiry.
        code = driver.wait(timeout=max(0.001,driver_deadline-time.monotonic()))
        record('driver-returned',returnCode=code)
    except BaseException as error:
        total_budget_expired = isinstance(error,subprocess.TimeoutExpired)
        record('wrapper-wait-timeout' if isinstance(error,subprocess.TimeoutExpired) else 'wrapper-failure')
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
                    record('driver-termination-request',driverPID=driver.pid,signalNumber=signal.SIGTERM.value)
                    driver.send_signal(signal.SIGTERM)
                try:
                    driver.wait(timeout=INTERRUPT_GRACE_SECONDS if total_budget_expired else max(INTERRUPT_GRACE_SECONDS,min(CLEANUP_SECONDS,driver_deadline-time.monotonic())))
                except subprocess.TimeoutExpired:
                    # A forced driver exit cannot prove Docker cleanup. Preserve
                    # the fixture and report uncertainty for manual inspection.
                    failures.append('Driver total budget/interrupt grace exceeded; forced driver exit, Docker cleanup unknown')
                    with contextlib.suppress(ProcessLookupError):
                        record('driver-forced-exit-request',driverPID=driver.pid,signalNumber=signal.SIGKILL.value)
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
                            record('observer-termination-request',observerPID=observer.pid,signalNumber=signal.SIGTERM.value)
                            observer.terminate()
                        try:
                            observed = observer.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            with contextlib.suppress(ProcessLookupError):
                                record('observer-forced-exit-request',observerPID=observer.pid,signalNumber=signal.SIGKILL.value)
                                observer.kill()
                            try:
                                observed = observer.wait(timeout=5)
                            except subprocess.TimeoutExpired:
                                failures.append('Owned observer could not be reaped')
            except BaseException as error:
                failures.append('Observer cleanup exception: ' + type(error).__name__)
        record('wrapper-settled',returnCode=code,driverPID=driver.pid if driver else 0,observerPID=observer.pid if observer else 0)
        os.close(lifecycle)
        if journal_failed:failures.append('Wrapper lifecycle journal failed; shutdown cause evidence incomplete')
        if not self_cost.exists():
            failures.append('Observer self-cost receipt missing; its own process cost is unverified')
        if not pointer.exists():
            failures.append('No owned personal instance became available; no container coverage is claimed')
        if failures:
            print(json.dumps({'observerWrapperFailures': failures,
                              'privateFixtureParent': str(fixture_parent)}), file=sys.stderr)
    return code if code else (1 if observed or failures else 0)


if __name__ == '__main__':
    raise SystemExit(main())
