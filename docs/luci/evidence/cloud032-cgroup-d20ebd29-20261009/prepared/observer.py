"""Bounded external cgroup-v2 counter observation; never controls an application.

No process discovery, per-process sampling, browser connection or profiler.
Synchronous kernel-file reads are cooperative; use the separately reviewed own-
observer supervisor when an enforced process lifetime is required.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import signal
import stat
import time
from dataclasses import dataclass

MAX_SECONDS = 400
MAX_ROWS = 4000
MAX_BYTES = 2 * 1024 * 1024
INTERVAL_NS = 100_000_000
FOOTER_RESERVE = 32 * 1024
SAMPLE_RESERVE = 16 * 1024
STAT_LIMIT = 4096
REQUIRED = ('usage_usec', 'user_usec', 'system_usec', 'nr_periods', 'nr_throttled', 'throttled_usec')
KNOWN = set(REQUIRED) | {'nice_usec', 'nr_bursts', 'burst_usec', 'core_sched.force_idle_usec'}
KEY = re.compile(r'[a-z][a-z0-9_.]*\Z')


class FormatError(ValueError):
    pass


def parse_stat(raw: bytes) -> dict:
    if not raw or len(raw) > STAT_LIMIT:
        raise FormatError('empty_or_oversize')
    try:
        lines = raw.decode('ascii').splitlines()
    except UnicodeDecodeError as exc:
        raise FormatError('non_ascii') from exc
    values = {}
    for line in lines:
        fields = line.split()
        if len(fields) != 2 or not KEY.fullmatch(fields[0]) or len(fields[0]) > 64:
            raise FormatError('invalid_record')
        key, number = fields
        if key in values:
            raise FormatError('duplicate_key')
        if not number.isascii() or not number.isdecimal() or len(number) > 20:
            raise FormatError('invalid_counter')
        value = int(number)
        if value > 2**64 - 1:
            raise FormatError('counter_overflow')
        values[key] = value
        if len(values) > 32:
            raise FormatError('too_many_fields')
    return {'values': values, 'missing': [key for key in REQUIRED if key not in values],
            'unknownKeys': sorted(set(values) - KNOWN)}


def counter_delta(previous: dict | None, current: dict) -> dict:
    if previous is None:
        return {'status': 'baseline', 'values': None, 'resetKeys': [], 'changedFields': []}
    changed = sorted(set(previous) ^ set(current))
    resets = sorted(key for key in previous.keys() & current.keys() if current[key] < previous[key])
    if changed or resets:
        return {'status': 'counter_reset_or_shape_change', 'values': None,
                'resetKeys': resets, 'changedFields': changed}
    return {'status': 'observed', 'values': {key: current[key] - previous[key] for key in current},
            'resetKeys': [], 'changedFields': []}


def next_deadline(previous: int, now: int, interval: int = INTERVAL_NS) -> tuple[int, int]:
    next_value = previous + interval
    skipped = 0
    if next_value <= now:
        skipped = (now - next_value) // interval + 1
        next_value += skipped * interval
    return next_value, skipped


class SystemClock:
    monotonic_ns = staticmethod(time.monotonic_ns)
    wall_ns = staticmethod(time.time_ns)
    cpu_ns = staticmethod(time.process_time_ns)
    sleep = staticmethod(time.sleep)


@dataclass(frozen=True)
class Limits:
    duration_ns: int = MAX_SECONDS * 1_000_000_000
    rows: int = MAX_ROWS
    bytes: int = MAX_BYTES
    footer_reserve: int = FOOTER_RESERVE
    interval_ns: int = INTERVAL_NS

    def validate(self):
        if not (0 < self.duration_ns <= MAX_SECONDS * 1_000_000_000
                and 3 <= self.rows <= MAX_ROWS and 128 <= self.footer_reserve < self.bytes <= MAX_BYTES
                and self.interval_ns == INTERVAL_NS):
            raise ValueError('Unsupported bounds')


class BoundedWriter:
    """Fresh private regular file, append-only writes, final-row reserve."""
    def __init__(self, path: Path, limits: Limits):
        limits.validate()
        path = Path(path)
        parent = path.parent
        if not path.is_absolute() or parent.resolve() != parent:
            raise ValueError('Output must have an absolute nonlinked parent')
        info = parent.stat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) & 0o077:
            raise ValueError('Output parent must be owned and private')
        self.fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_APPEND | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
        try:
            os.fchmod(self.fd, 0o600)
        except BaseException:
            os.close(self.fd)
            raise
        self.path, self.limits = path, limits
        self.rows, self.bytes, self.failed = 0, 0, False

    def can_sample(self) -> bool:
        # The parser permits at most 32 bounded uint64/key pairs. This reserve
        # exceeds the resulting sample/delta/reset JSON size. Stop before a read
        # that cannot be retained; do not silently discard an observed counter.
        return (self.rows + 1 <= self.limits.rows - 1
                and self.bytes + SAMPLE_RESERVE <= self.limits.bytes - self.limits.footer_reserve)

    def append(self, value: dict, *, final: bool = False) -> bool:
        data = (json.dumps(value, separators=(',', ':'), allow_nan=False) + '\n').encode()
        row_limit = self.limits.rows if final else self.limits.rows - 1
        byte_limit = self.limits.bytes if final else self.limits.bytes - self.limits.footer_reserve
        if self.rows + 1 > row_limit or self.bytes + len(data) > byte_limit:
            return False
        try:
            offset = 0
            while offset < len(data):
                count = os.write(self.fd, data[offset:])
                if count <= 0:
                    raise OSError('nonprogressing_write')
                offset += count
                self.bytes += count
            self.rows += 1
            return True
        except BaseException:
            self.failed = True
            raise

    def close(self):
        if self.fd is not None:
            try:
                os.fsync(self.fd)
            finally:
                os.close(self.fd)
                self.fd = None


class KernelReader:
    """Only fixed files at the chosen cgroup and this observer's own metadata."""
    def __init__(self, cgroup: Path):
        self.path = Path(cgroup)
        if not self.path.is_absolute() or self.path.resolve() != self.path:
            raise ValueError('Cgroup directory must be absolute and nonlinked')
        self.fd = os.open(self.path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        self.read_count = 0

    def _read(self, path: str, limit: int, *, relative=False) -> dict:
        self.read_count += 1
        fd = None
        try:
            fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC,
                         dir_fd=self.fd if relative else None)
            if not stat.S_ISREG(os.fstat(fd).st_mode):
                return {'status': 'unavailable', 'reason': 'not_regular'}
            raw = os.read(fd, limit + 1)
            if len(raw) > limit:
                return {'status': 'unavailable', 'reason': 'oversize'}
            return {'status': 'observed', 'raw': raw}
        except OSError as exc:
            return {'status': 'unavailable', 'errno': exc.errno}
        finally:
            if fd is not None:
                os.close(fd)

    def stat(self) -> dict:
        result = self._read('cpu.stat', STAT_LIMIT, relative=True)
        if result['status'] != 'observed':
            return result
        raw = result['raw']
        try:
            return {'status': 'observed', 'rawBytes': len(raw), 'rawSHA256': hashlib.sha256(raw).hexdigest(), **parse_stat(raw)}
        except FormatError as exc:
            return {'status': 'unavailable', 'reason': str(exc), 'rawBytes': len(raw),
                    'rawSHA256': hashlib.sha256(raw).hexdigest()}

    def _text(self, path: str, limit: int, *, relative=False) -> dict:
        result = self._read(path, limit, relative=relative)
        if result['status'] != 'observed':
            return result
        try:
            text = result['raw'].decode('ascii').strip()
        except UnicodeDecodeError:
            return {'status': 'unavailable', 'reason': 'non_ascii'}
        return {'status': 'observed', 'value': text}

    def metadata(self) -> dict:
        def identity(info):return {'device': info.st_dev, 'inode': info.st_ino}
        held = identity(os.fstat(self.fd))
        try:
            current = identity(self.path.stat(follow_symlinks=False))
        except OSError:
            current = None
        namespaces = {}
        for name in ('cgroup', 'time'):
            self.read_count += 1
            try:namespaces[name] = {'status': 'observed', 'value': os.readlink('/proc/self/ns/' + name)}
            except OSError as exc:namespaces[name] = {'status': 'unavailable', 'errno': exc.errno}
        return {'cgroupDirectory': str(self.path), 'heldIdentity': held, 'currentPathIdentity': current,
                'pathStillSame': held == current,
                'cpuMax': self._text('cpu.max', 128, relative=True),
                'effectiveCpuset': self._text('cpuset.cpus.effective', 1024, relative=True),
                'observerCgroupMembership': self._text('/proc/self/cgroup', 4096),
                'namespaces': namespaces,
                'clocksource': self._text('/sys/devices/system/clocksource/clocksource0/current_clocksource', 128),
                'clockAPIs': {name: {'implementation': time.get_clock_info(name).implementation,
                                     'monotonic': time.get_clock_info(name).monotonic,
                                     'adjustable': time.get_clock_info(name).adjustable,
                                     'resolution': time.get_clock_info(name).resolution}
                              for name in ('monotonic', 'time', 'process_time')},
                'scope': 'Visible selected cgroup only; other tasks may share it and hidden ancestors are not observed.'}

    def close(self):
        os.close(self.fd)


def metadata_stable(before: dict, after: dict) -> bool:
    required = ['heldIdentity', 'currentPathIdentity', 'pathStillSame', 'cpuMax', 'effectiveCpuset',
                'observerCgroupMembership', 'namespaces']
    if not before.get('pathStillSame') or not after.get('pathStillSame'):
        return False
    if any(before.get(key) != after.get(key) for key in required):
        return False
    values = [before.get(key, {}) for key in ('cpuMax', 'effectiveCpuset', 'observerCgroupMembership')]
    values += list(before.get('namespaces', {}).values())
    return bool(values) and all(value.get('status') == 'observed' for value in values)


def observe(reader, writer: BoundedWriter, *, clock=None, stop=None, limits=Limits()) -> dict:
    limits.validate()
    clock = clock or SystemClock()
    stop = stop or (lambda: False)
    began = clock.monotonic_ns()
    cpu_began = clock.cpu_ns()
    end = began + limits.duration_ns
    before_start = {'monotonicNS': began, 'wallNS': clock.wall_ns()}
    before = reader.metadata()
    before_end = {'monotonicNS': clock.monotonic_ns(), 'wallNS': clock.wall_ns()}
    header = {'type': 'header', 'version': 1, 'observerPID': os.getpid(),
              'limits': {'durationNS': limits.duration_ns, 'intervalNS': limits.interval_ns,
                         'totalRows': limits.rows, 'totalBytes': limits.bytes, 'footerReserveBytes': limits.footer_reserve},
              'metadataBefore': before, 'readBracket': {'start': before_start, 'end': before_end},
              'scope': 'Shared visible cgroup counter deltas only. No process/browser/application attribution or timing subtraction.'}
    if not writer.append(header):
        raise RuntimeError('Header cannot fit configured report bound')
    planned = clock.monotonic_ns()
    previous, previous_end = None, None
    count = unknown = resets = skipped_total = overrun_reads = 0
    max_late = max_read = 0
    positive = False
    gaps_since_valid = 0
    reason = 'duration_limit'
    while True:
        now = clock.monotonic_ns()
        if stop():reason = 'owner_stop';break
        if now >= end:break
        if not writer.can_sample():reason = 'report_bound';break
        if now < planned:
            clock.sleep(min(planned - now, end - now) / 1e9)
            continue
        start_mono = clock.monotonic_ns()
        start_wall = clock.wall_ns()
        observation = reader.stat()
        end_wall = clock.wall_ns()
        end_mono = clock.monotonic_ns()
        late, duration = max(0, start_mono - planned), max(0, end_mono - start_mono)
        max_late, max_read = max(max_late, late), max(max_read, duration)
        if duration > limits.interval_ns:overrun_reads += 1
        if observation['status'] == 'observed':
            delta = counter_delta(previous, observation['values'])
            delta.update(previousValidReadEndMonotonicNS=previous_end, interveningUnknownSamples=gaps_since_valid)
            if delta['status'] == 'counter_reset_or_shape_change':resets += 1
            if delta['values'] is not None and (delta['values'].get('nr_throttled', 0) > 0 or delta['values'].get('throttled_usec', 0) > 0):positive = True
            previous, previous_end, gaps_since_valid = observation['values'], end_mono, 0
            if observation['missing']:unknown += 1
        else:
            unknown += 1;gaps_since_valid += 1
            delta = {'status': 'unavailable', 'values': None}
        following, skipped = next_deadline(planned, end_mono, limits.interval_ns)
        skipped_total += skipped
        row = {'type': 'sample', 'index': count, 'plannedMonotonicNS': planned,
               'readBracket': {'startMonotonicNS': start_mono, 'startWallNS': start_wall,
                               'endMonotonicNS': end_mono, 'endWallNS': end_wall},
               'latenessNS': late, 'readDurationNS': duration, 'missedDeadlinesBeforeNext': skipped,
               'observation': observation, 'delta': delta}
        if not writer.append(row):
            raise RuntimeError('Bounded sample exceeded reserved space; capture incomplete')
        count += 1
        planned = following
    after_start = {'monotonicNS': clock.monotonic_ns(), 'wallNS': clock.wall_ns()}
    after = reader.metadata()
    after_end = {'monotonicNS': clock.monotonic_ns(), 'wallNS': clock.wall_ns()}
    stable = metadata_stable(before, after)
    summary = {'type': 'footer', 'version': 1, 'stopReason': reason,
               'elapsedNS': clock.monotonic_ns() - began, 'observerSelfCPUNS': clock.cpu_ns() - cpu_began,
               'processCPUAtFooterNS': clock.cpu_ns(),
               'cooperativeDeadlineOverrunNS': max(0, clock.monotonic_ns() - end),
               'sampleRows': count, 'unknownObservations': unknown, 'counterResetOrShapeChanges': resets,
               'missedSamplingDeadlines': skipped_total, 'overrunReads': overrun_reads,
               'maxLatenessNS': max_late, 'maxReadDurationNS': max_read,
               'readCount': reader.read_count, 'metadataAfter': after,
               'readBracket': {'start': after_start, 'end': after_end}, 'metadataStable': stable,
               'anyPositiveObservedThrottleDelta': positive,
               'noPositiveThrottleWithinCompleteObservedPairs': bool(count >= 2 and not positive and not unknown and not resets and stable and reason != 'report_bound'),
               'rowsBeforeFooter': writer.rows, 'bytesBeforeFooter': writer.bytes,
               'limitations': ['Shared cgroup includes unrelated work; no per-action/process cause established.',
                              'Monotonic/wall brackets permit coarse phase association only; no driver/browser clock-origin patch.',
                              'Hidden ancestors, between-sample changes and synchronous-read stalls are not ruled out.',
                              'Skipped or unavailable intervals remain explicit; no catch-up samples or imputed zeros.',
                              'Capture interval coverage must be joined by owner to the unchanged launcher; this observer does not control it.',
                              'Self CPU covers observe() entry through footer construction; processCPUAtFooterNS also includes interpreter startup. Final serialization/fsync/close cost remains outside both fields; no cost subtraction.',
                              'Synchronous reads/fsync need the separate own-observer supervisor for a hard process bound.']}
    if not writer.append(summary, final=True):
        raise RuntimeError('Reserved final receipt did not fit')
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cgroup-dir', default='/sys/fs/cgroup')
    parser.add_argument('--output', required=True)
    parser.add_argument('--duration-seconds', type=float, default=MAX_SECONDS)
    args = parser.parse_args()
    if not math.isfinite(args.duration_seconds) or not 0 < args.duration_seconds <= MAX_SECONDS:
        parser.error('duration must be positive and at most 400 seconds')
    limits = Limits(duration_ns=int(args.duration_seconds * 1e9))
    stopping = False
    def request_stop(_signum, _frame):
        nonlocal stopping
        stopping = True
    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    reader = KernelReader(Path(args.cgroup_dir))
    writer = None
    try:
        writer = BoundedWriter(Path(args.output), limits)
        observe(reader, writer, stop=lambda: stopping, limits=limits)
    finally:
        if writer is not None:writer.close()
        reader.close()


if __name__ == '__main__':
    main()
