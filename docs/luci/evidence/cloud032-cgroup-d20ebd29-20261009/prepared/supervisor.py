"""Optional lifecycle for the adjacent cgroup observer; never controls an app.

Only Linux pidfds can signal the direct observer child. The stdin gate prevents
observer execution before its pidfd is acquired. Settlement is bounded in the
supervising thread; filesystem operations are cooperative, not preemptible.
A process exit alone is NOT a complete observation or product-cleanup proof.
"""
from __future__ import annotations

import json
import hashlib
import math
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import threading
import time

MAX_WALL_SECONDS = 400.0
MAX_SETTLEMENT_SECONDS = 3.0
MAX_RECEIPT_BYTES = 32768
POLL_SECONDS = 0.05
GATE = "import os,sys; b=sys.stdin.buffer.read(1); sys.exit(76) if b!=b'G' else os.execv(sys.executable,[sys.executable,*sys.argv[1:]])"


class SystemOps:
    monotonic = staticmethod(time.monotonic)
    thread_cpu_ns = staticmethod(time.thread_time_ns)

    @staticmethod
    def probe_pidfd():
        if not hasattr(os, 'pidfd_open') or not hasattr(signal, 'pidfd_send_signal'):
            raise RuntimeError('Linux pidfd APIs are required; no PID signal fallback')
        fd = os.pidfd_open(os.getpid(), 0)
        os.close(fd)

    @staticmethod
    def spawn(command):
        return subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.DEVNULL,
                                stderr=subprocess.DEVNULL, close_fds=True)

    pidfd_open = staticmethod(lambda pid: os.pidfd_open(pid, 0))
    close = staticmethod(os.close)
    signal = staticmethod(lambda fd, sig: signal.pidfd_send_signal(fd, sig))


def limits(wall_seconds, term_grace, kill_grace):
    for value in (wall_seconds, term_grace, kill_grace):
        if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value <= 0:
            raise ValueError('Expected finite positive time limits')
    if wall_seconds > MAX_WALL_SECONDS or term_grace + kill_grace + POLL_SECONDS > MAX_SETTLEMENT_SECONDS:
        raise ValueError('Observer wall/settlement limit exceeded')


def watch_child(process, fd, ops, stop_requested, *, started_at, wall_seconds,
                term_grace=1.0, kill_grace=1.0):
    """Purely injectable own-child wait/signal state machine; no discovery."""
    limits(wall_seconds, term_grace, kill_grace)
    result = {'pid': process.pid, 'identityMechanism': 'direct-child-pidfd-before-exec',
              'startedAtMonotonic': started_at, 'wallSeconds': wall_seconds,
              'termGraceSeconds': term_grace, 'killGraceSeconds': kill_grace,
              'events': [], 'errors': [], 'forced': False, 'settled': False,
              'waitCallCount': 0, 'pidfdSignalAttempts': 0,
              'exitCode': None, 'observationComplete': None,
              'scope': 'Own observer process settlement only; observation data and application cleanup are not certified.'}
    deadline = started_at + wall_seconds
    previous = started_at

    def wait(timeout):
        result['waitCallCount'] += 1
        return process.wait(timeout=timeout)

    def clock():
        nonlocal previous
        now = ops.monotonic()
        if not math.isfinite(now) or now < previous:
            raise RuntimeError('Supervisor monotonic clock is invalid')
        previous = now
        return now

    def finish(code):
        result['exitCode'] = code
        result['settled'] = True
        result['finishedAtMonotonic'] = ops.monotonic()
        result['processOutcome'] = ('clean_exit' if code == 0 and not result['forced'] and not result['errors']
                                    and result.get('stopReason') in (None, 'owner_finished') else 'failed_or_partial')
        return result

    try:
        while True:
            now = clock()
            requested = stop_requested()
            if requested:
                result['stopReason'] = str(requested)[:80]
                break
            if now >= deadline:
                result['stopReason'] = 'wall_deadline'
                break
            try:
                return finish(wait(min(POLL_SECONDS, deadline - now)))
            except subprocess.TimeoutExpired:
                pass
    except Exception as error:
        result['errors'].append({'stage': 'watch', 'type': type(error).__name__})
        result['stopReason'] = 'watch_error'

    for sig, grace in ((signal.SIGTERM, term_grace), (signal.SIGKILL, kill_grace)):
        # pidfd remains pinned even when the numeric PID exits/reuses. Do not
        # consult a mutable process list or fall back to kill(pid).
        try:
            # A known exit immediately before signalling is settled without a signal.
            code = wait(0)
            return finish(code)
        except subprocess.TimeoutExpired:
            pass
        except Exception as error:
            result['errors'].append({'stage': 'pre_signal_wait', 'type': type(error).__name__})
        if sig == signal.SIGKILL:
            result['forced'] = True
        try:
            result['pidfdSignalAttempts'] += 1
            ops.signal(fd, sig)
            result['events'].append({'signal': sig.name, 'via': 'pidfd', 'accepted': True})
        except ProcessLookupError:
            result['events'].append({'signal': sig.name, 'via': 'pidfd', 'alreadyExited': True})
        except Exception as error:
            result['errors'].append({'stage': sig.name, 'type': type(error).__name__})
        try:
            return finish(wait(grace))
        except subprocess.TimeoutExpired:
            result['events'].append({'waitAfter': sig.name, 'timedOut': True})
        except Exception as error:
            result['errors'].append({'stage': 'wait_after_' + sig.name, 'type': type(error).__name__})
    result['processOutcome'] = 'settlement_unknown'
    result['errors'].append({'stage': 'settlement', 'type': 'OwnedChildStillUnconfirmed'})
    result['finishedAtMonotonic'] = ops.monotonic()
    return result


def private_receipt(path, value):
    encoded = (json.dumps(value, indent=2) + '\n').encode()
    if len(encoded) > MAX_RECEIPT_BYTES:
        raise ValueError('Supervisor receipt exceeds its bound')
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, 'O_NOFOLLOW', 0)
    fd = os.open(path, flags, 0o600)
    try:
        view = memoryview(encoded)
        while view:
            written = os.write(fd, view)
            if written <= 0:
                raise OSError('Supervisor receipt write did not progress')
            view = view[written:]
        os.fsync(fd)
    finally:
        os.close(fd)


def output_argument(arguments):
    values = {}
    index = 0
    while index < len(arguments):
        arg = arguments[index]
        if '=' in arg:
            key, value = arg.split('=', 1)
        else:
            key = arg
            index += 1
            if index >= len(arguments):
                raise ValueError('Missing observer argument value')
            value = arguments[index]
        if key not in ('--output', '--cgroup-dir', '--duration-seconds') or key in values:
            raise ValueError('Unknown or duplicate observer argument')
        values[key] = value
        index += 1
    path = Path(values.get('--output', ''))
    if not values.get('--output') or not path.is_absolute():
        raise ValueError('Observer requires one absolute new output path')
    return path


def validate_observation(path):
    """Bounded own-output serialization validation, never counter sampling."""
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | getattr(os, 'O_NOFOLLOW', 0))
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600:
            raise ValueError('Observation output is not an owned private regular file')
        if info.st_size > 2 * 1024 * 1024:
            raise ValueError('Observation output exceeds byte bound')
        data = bytearray()
        while True:
            part = os.read(fd, min(65536, 2 * 1024 * 1024 + 1 - len(data)))
            if not part:
                break
            data.extend(part)
            if len(data) > 2 * 1024 * 1024:
                raise ValueError('Observation output grew beyond bound')
    finally:
        os.close(fd)
    if not data or not data.endswith(b'\n'):
        raise ValueError('Observation lacks complete final line')
    lines = bytes(data).splitlines(keepends=True)
    if not 2 <= len(lines) <= 4000:
        raise ValueError('Observation row bound exceeded')
    def unique_object(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('Duplicate JSON key')
            result[key] = value
        return result
    rows = [json.loads(line, object_pairs_hook=unique_object, parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Invalid JSON constant'))) for line in lines]
    if not all(isinstance(row, dict) for row in rows) or rows[0].get('type') != 'header' or rows[-1].get('type') != 'footer':
        raise ValueError('Observation requires initial header and terminal footer')
    samples = rows[1:-1]
    if any(row.get('type') != 'sample' or type(row.get('index')) is not int or row['index'] != index for index, row in enumerate(samples)):
        raise ValueError('Observation samples are incomplete or repeated')
    footer = rows[-1]
    if footer.get('stopReason') not in ('duration_limit', 'owner_stop', 'report_bound'):
        raise ValueError('Unknown observation stop reason')
    if footer.get('sampleRows') != len(samples) or footer.get('rowsBeforeFooter') != len(lines)-1 or footer.get('bytesBeforeFooter') != sum(map(len, lines[:-1])):
        raise ValueError('Observation footer counts do not match')
    return {'serializationComplete': True, 'observerHeaderPID': rows[0].get('observerPID'),
            'rows': len(lines), 'bytes': len(data),
            'stopReason': footer['stopReason'], 'sampleRows': len(samples),
            'metadataStable': footer.get('metadataStable'),
            'unknownObservations': footer.get('unknownObservations'),
            'counterResetOrShapeChanges': footer.get('counterResetOrShapeChanges'),
            'scope': 'Bounded serialized capture only, not valid coverage, absence of throttling or product acceptance.'}


def source_record(path):
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            raise ValueError('Expected ordinary source file')
        with os.fdopen(os.dup(fd), 'rb') as stream:
            raw = stream.read(1024 * 1024 + 1)
        if len(raw) > 1024 * 1024:
            raise ValueError('Source input exceeds bound')
        return {'path': str(path), 'sha256': hashlib.sha256(raw).hexdigest(), 'bytes': len(raw)}
    finally:
        os.close(fd)


class ManagedObserver:
    """Root-owned optional context around an UNCHANGED workload launcher.

    observer_args are the adjacent observer.py's ordinary CLI arguments.
    receipt_dir MUST be a new private directory; existing paths are refused.
    .start() never starts Luci/browser/provider. .stop() signals only observer.
    .wait() reports process settlement and bounded JSONL serialization separately.
    Coverage/usability and application cleanup are never certified by exit 0.
    """
    def __init__(self, observer_args, receipt_dir, *, wall_seconds=400.0,
                 term_grace=1.0, kill_grace=1.0, ops=None):
        limits(wall_seconds, term_grace, kill_grace)
        if not isinstance(observer_args, list) or not all(isinstance(x, str) and '\0' not in x for x in observer_args):
            raise ValueError('Observer arguments must be strings')
        self.args = list(observer_args)
        self.output = output_argument(self.args)
        self.directory = Path(receipt_dir)
        self.wall_seconds, self.term_grace, self.kill_grace = wall_seconds, term_grace, kill_grace
        self.ops = ops or SystemOps()
        self._stop = threading.Event()
        self._done = threading.Event()
        self._reason = 'owner_finished'
        self._thread = None
        self._result = {'processOutcome': 'not_started', 'observationComplete': None}
        self._fd = None
        self._process = None
        self._source_paths = [Path(__file__), Path(__file__).with_name('observer.py')]
        self._sources_before = None

    def start(self):
        if self._thread is not None or self._process is not None:
            raise RuntimeError('Observer already started; no replay')
        self.ops.probe_pidfd()
        observer = Path(__file__).with_name('observer.py')
        if not stat.S_ISREG(observer.lstat().st_mode):
            raise ValueError('Expected ordinary adjacent observer.py')
        if os.path.lexists(self.output):
            raise FileExistsError('Refusing existing observer output')
        parent = self.directory.parent
        if not self.directory.is_absolute() or parent.resolve(strict=True) != parent:
            raise ValueError('Receipt parent must be absolute and nonlinked')
        parent_info = parent.stat()
        if not stat.S_ISDIR(parent_info.st_mode) or parent_info.st_uid != os.getuid() or stat.S_IMODE(parent_info.st_mode) & 0o077:
            raise ValueError('Receipt parent must be owned and private')
        self._sources_before = [source_record(path) for path in self._source_paths]
        self.directory.mkdir(mode=0o700, parents=False, exist_ok=False)
        # No child exists before all setup/preconditions and initial receipt.
        private_receipt(self.directory / 'invocation.json', {
            'observer': str(observer), 'argumentCount': len(self.args),
            'wallSeconds': self.wall_seconds, 'termGraceSeconds': self.term_grace,
            'killGraceSeconds': self.kill_grace, 'identityMechanism': 'pidfd',
            'scope': 'Observer only; no app mutation or observation completion claim.',
            'sourcesBefore': self._sources_before,
            'observerStdoutStderr': 'Discarded; core exception text is unavailable. Nonzero exit/missing receipt is classified without invented diagnosis.'})
        started = self.ops.monotonic()
        if not math.isfinite(started):
            raise ValueError('Invalid supervisor monotonic start')
        self._process = self.ops.spawn([sys.executable, '-I', '-c', GATE, str(observer), *self.args])
        try:
            self._fd = self.ops.pidfd_open(self._process.pid)
            self._process.stdin.write(b'G')
            self._process.stdin.flush()
            self._process.stdin.close()
        except Exception as error:
            # No observer exec if pidfd acquisition failed: release the EOF gate.
            try:
                self._process.stdin.close()
            except Exception:
                pass
            result = {'pid': self._process.pid, 'processOutcome': 'startup_failed',
                      'errorType': type(error).__name__, 'observationComplete': None,
                      'settled': False, 'scope': 'Observer startup refused; no PID-based fallback signal.'}
            try:
                if self._fd is not None:
                    result['settlement'] = watch_child(self._process, self._fd, self.ops, lambda: 'startup_error', started_at=started, wall_seconds=self.wall_seconds, term_grace=self.term_grace, kill_grace=self.kill_grace)
                    result['settled'] = result['settlement']['settled']
                else:
                    result['exitCode'] = self._process.wait(timeout=self.term_grace + self.kill_grace)
                    result['settled'] = True
            except Exception as wait_error:
                result['waitErrorType'] = type(wait_error).__name__
            if self._fd is not None:
                self.ops.close(self._fd)
                self._fd = None
            self._result = result
            private_receipt(self.directory / 'result.json', result)
            self._done.set()
            raise RuntimeError('Observer failed to acquire/activate exact owned identity') from error
        try:
            self._thread = threading.Thread(target=self._run, args=(started,), daemon=True,
                                            name='luci-cgroup-observer-watch')
            self._thread.start()
        except Exception as error:
            self._reason = 'watcher_start_failed'
            self._stop.set()
            self._run(started)
            raise RuntimeError('Observer watcher failed; exact owned observer settlement attempted') from error
        return self

    def _run(self, started):
        cpu_start = self.ops.thread_cpu_ns()
        try:
            self._result = watch_child(self._process, self._fd, self.ops,
                lambda: self._reason if self._stop.is_set() else None,
                started_at=started, wall_seconds=self.wall_seconds,
                term_grace=self.term_grace, kill_grace=self.kill_grace)
        except Exception as error:
            self._result = {'processOutcome': 'supervisor_failed', 'errorType': type(error).__name__,
                            'settled': False, 'observationComplete': None}
        finally:
            try:
                self._result['sourcesBefore'] = self._sources_before
                self._result['sourcesAfter'] = [source_record(path) for path in self._source_paths]
                self._result['sourceStable'] = self._result['sourcesBefore'] == self._result['sourcesAfter']
            except Exception as error:
                self._result['sourceStable'] = False
                self._result['sourceReadErrorType'] = type(error).__name__
            try:
                self._result['observation'] = validate_observation(self.output) if self._result.get('settled') else {'serializationComplete': False, 'reason': 'child_unsettled'}
                self._result['observerStartedAcknowledgedByHeader'] = self._result['observation'].get('observerHeaderPID') == self._process.pid
                self._result['observationComplete'] = bool(self._result['sourceStable'] and self._result['observerStartedAcknowledgedByHeader'] and self._result.get('exitCode') == 0 and self._result.get('processOutcome') == 'clean_exit' and self._result['observation'].get('serializationComplete'))
            except Exception as error:
                self._result['observationComplete'] = False
                self._result['observation'] = {'serializationComplete': False, 'errorType': type(error).__name__}
                self._result['observerStartedAcknowledgedByHeader'] = False
            if self._result.get('observationComplete') is not True:
                self._result['processOutcome'] = 'failed_or_partial' if self._result.get('settled') else 'settlement_unknown'
            if self._fd is not None:
                try:
                    self.ops.close(self._fd)
                except Exception as error:
                    self._result.setdefault('errors', []).append({'stage': 'pidfd_close', 'type': type(error).__name__})
                    self._result['processOutcome'] = 'failed_or_partial'
                self._fd = None
            self._result['watchdogThreadCPU'] = {'elapsedNS': self.ops.thread_cpu_ns() - cpu_start,
                'scope': 'Watch/wait/signal, source/output validation and pidfd close only. Excludes initial setup, final receipt serialization/fsync, owner thread, observer process and kernel/daemon cost; not total adjunct overhead.'}
            try:
                private_receipt(self.directory / 'result.json', self._result)
            except Exception as error:
                self._result['receiptErrorType'] = type(error).__name__
                self._result['processOutcome'] = 'failed_or_partial'
            self._done.set()

    def snapshot(self):
        return dict(self._result)

    def wait(self, timeout=None):
        if self._process is None:
            raise RuntimeError('Observer not started')
        if timeout is None:
            timeout = self.wall_seconds + MAX_SETTLEMENT_SECONDS
        if not isinstance(timeout, (int, float)) or not math.isfinite(timeout) or timeout < 0:
            raise ValueError('Expected finite nonnegative wait timeout')
        if not self._done.wait(timeout):
            return {'processOutcome': 'supervisor_wait_unconfirmed', 'settled': False,
                    'observationComplete': None, 'scope': 'Wait timed out; no cleanup success inferred.'}
        return self.snapshot()

    def stop(self, reason='owner_finished'):
        if not isinstance(reason, str) or len(reason) > 80:
            raise ValueError('Expected bounded stop reason')
        self._reason = reason
        self._stop.set()
        return self.wait(MAX_SETTLEMENT_SECONDS)

    def __enter__(self):
        return self.start()

    def __exit__(self, exc_type, exc_value, traceback):
        self.stop('owner_error' if exc_type else 'owner_finished')
        return False
