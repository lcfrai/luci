"""Fixed inherited-original binary reader. Unwired source, never an authority mint.

Only the future original native owner can bind this command to its actual child,
184 HANDLE_LIST and durable capture. No paths, acquisition, payload execution,
parent lifecycle, registry, purpose or adoption are implemented here.
"""
import ctypes as C
from ctypes import wintypes as W
import hashlib
import io
import json
import os
import re
import struct
import sys
import time
import types

ABI = 'luci-debian-binary-entry-v1'
READER = (22824, '2f52054fc43e926389180ba038ae40650318b940ef3efce5f767492f918f451b')
INSPECTOR = (39189, '06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba')
CHUNK, GLOBAL_BYTES, GLOBAL_CALLS = 65536, 335544320, 50000
IDENTITY_KEYS = ('fileType', 'fileAttributes', 'volumeSerial', 'fileIndex', 'bytes',
                 'creationFileTime', 'lastWriteFileTime', 'links')
FLAGS = ('--abi', '--operation-id', '--loan-id', '--source-binding-sha256',
         '--selection-sha256', '--selection-bytes', '--selection-identity',
         '--work-deadline-tick-ms', '--handles')
WIRE_MAGIC, WIRE_HEADER = b'LUCI-DBBE1\n', struct.Struct('<BBI32s')


class Refusal(ValueError):
    pass


def integer(value, minimum, maximum):
    if type(value) is not int or not minimum <= value <= maximum:
        raise Refusal('bounded integer')
    return value


def digest(value):
    if not isinstance(value, str) or not re.fullmatch('[0-9a-f]{64}', value):
        raise Refusal('sha256')
    return value


def closed(value, keys):
    if not isinstance(value, dict) or set(value) != set(keys):
        raise Refusal('closed metadata fields')
    return value


def text(value, maximum):
    if not isinstance(value, str) or not value or len(value.encode('utf-8')) > maximum or any(ord(c) < 32 or ord(c) == 127 for c in value):
        raise Refusal('bounded text')
    return value


def identity(value):
    closed(value, IDENTITY_KEYS)
    for key in IDENTITY_KEYS:
        integer(value[key], 0, (1 << (32 if key in ('fileType', 'fileAttributes', 'volumeSerial', 'links') else 64)) - 1)
    if value['fileType'] != 1 or value['links'] != 1 or value['fileAttributes'] & (0x10 | 0x400):
        raise Refusal('ordinary readonly-source subject')
    return dict(value)


def parse_argv(argv):
    if len(argv) != 18 or tuple(argv[::2]) != FLAGS or sum(len(v) + 1 for v in argv) > 8192:
        raise Refusal('fixed command ABI')
    values = dict(zip(FLAGS, argv[1::2]))
    if values['--abi'] != ABI:
        raise Refusal('ABI')
    for key in ('--operation-id', '--loan-id'):
        if not re.fullmatch('[0-9a-f]{32}', values[key]):
            raise Refusal('operation or loan binding')
    for key in ('--source-binding-sha256', '--selection-sha256'):
        digest(values[key])
    for key in ('--selection-bytes', '--work-deadline-tick-ms'):
        if not re.fullmatch('[0-9]{1,20}', values[key]):
            raise Refusal('command integer framing')
    values['selectionBytes'] = integer(int(values['--selection-bytes']), 1, 262144)
    values['deadlineTick'] = integer(int(values['--work-deadline-tick-ms']), 1, (1 << 64) - 1)
    fields = values['--selection-identity'].split(',')
    if len(fields) != 8 or any(not re.fullmatch('[0-9]{1,20}', v) for v in fields):
        raise Refusal('selection ordinary identity framing')
    values['selectionIdentity'] = identity(dict(zip(IDENTITY_KEYS, map(int, fields))))
    if values['selectionIdentity']['bytes'] != values['selectionBytes']:
        raise Refusal('selection size binding')
    fields = values['--handles'].split(',')
    if len(fields) != 181 or any(not re.fullmatch('[0-9a-f]{1,16}', v) for v in fields):
        raise Refusal('fixed inherited alias framing')
    handles = tuple(int(v, 16) for v in fields)
    if len(set(handles)) != 181 or any(v in (0, (1 << 64) - 1) for v in handles):
        raise Refusal('distinct valid inherited aliases')
    values['handles'] = handles
    return values


class FileInformation(C.Structure):
    _fields_ = [('attributes', W.DWORD), ('creation', W.FILETIME),
                ('access', W.FILETIME), ('write', W.FILETIME), ('volume', W.DWORD),
                ('sizeHigh', W.DWORD), ('sizeLow', W.DWORD), ('links', W.DWORD),
                ('indexHigh', W.DWORD), ('indexLow', W.DWORD)]


class WindowsEntryNative:
    """Literal fixed exports only; constructed solely inside future Windows main."""
    def __init__(self):
        if os.name != 'nt' or C.sizeof(C.c_void_p) != 8:
            raise Refusal('native 64-bit Windows required')
        self.kernel = C.WinDLL('kernel32.dll', use_last_error=True, winmode=0x800)
        for name, arguments, result in (
                ('GetTickCount64', [], C.c_ulonglong),
                ('GetFileType', [W.HANDLE], W.DWORD),
                ('GetHandleInformation', [W.HANDLE, C.POINTER(W.DWORD)], W.BOOL),
                ('GetFileInformationByHandle', [W.HANDLE, C.POINTER(FileInformation)], W.BOOL)):
            function = getattr(self.kernel, name)
            function.argtypes, function.restype = arguments, result

    def tick(self):
        return int(self.kernel.GetTickCount64())

    def file_type(self, handle):
        return int(self.kernel.GetFileType(handle))

    def flags(self, handle):
        value = W.DWORD()
        if not self.kernel.GetHandleInformation(handle, C.byref(value)):
            raise OSError(C.get_last_error(), 'original alias flags')
        return int(value.value)

    def info(self, handle):
        value = FileInformation()
        if not self.kernel.GetFileInformationByHandle(handle, C.byref(value)):
            raise OSError(C.get_last_error(), 'original alias identity')
        pair = lambda high, low: (int(high) << 32) | int(low)
        return {'fileAttributes': int(value.attributes), 'volumeSerial': int(value.volume),
                'fileIndex': pair(value.indexHigh, value.indexLow),
                'bytes': pair(value.sizeHigh, value.sizeLow),
                'creationFileTime': pair(value.creation.dwHighDateTime, value.creation.dwLowDateTime),
                'lastWriteFileTime': pair(value.write.dwHighDateTime, value.write.dwLowDateTime),
                'links': int(value.links)}  # LastAccessTime deliberately not bound.

    def convert_fd(self, handle):
        import msvcrt
        return msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)

    def stream(self, fd):
        return io.FileIO(fd, 'rb', closefd=True)

    def close_fd(self, fd):
        os.close(fd)


class EntryAttempt:
    def __init__(self, command, native):
        self.command, self.native = command, native
        self.failed, self.budget, self.seeded = False, None, False
        self.compressed = self.calls = self.seeks = self.clocks = self.metadata = 0
        self.classes = {'index': [0, 0, 19044408, 768], 'small': [0, 0, 393216, 16]}
        self.last_tick, self.slots = None, []
        self.closed = False
        tick = self.check()
        if command['deadlineTick'] - tick > 300000:
            self.fail('original remaining work bound')

    def fail(self, reason):
        self.failed = True
        if self.budget is not None:
            self.budget.failed = True
        raise Refusal(reason)

    def check(self):
        if self.failed or self.budget is not None and self.budget.failed:
            self.fail('sticky failure')
        if self.clocks >= 1000000:
            self.fail('fixed clock call bound')
        self.clocks += 1
        try:
            now = self.native.tick()
        except BaseException:
            self.failed = True
            raise
        if type(now) is not int or now < 0 or self.last_tick is not None and now < self.last_tick or now >= self.command['deadlineTick']:
            self.fail('original owner deadline or clock')
        self.last_tick = now
        return now

    def metadata_call(self, function, handle):
        self.check()
        if self.metadata >= 1086:
            self.fail('fixed ordinary metadata call bound')
        self.metadata += 1
        try:
            value = function(handle)
            self.check()
            return value
        except BaseException:
            self.failed = True
            raise

    def snapshot(self, slot):
        kind = self.metadata_call(self.native.file_type, slot.handle)
        flags = self.metadata_call(self.native.flags, slot.handle)
        info = self.metadata_call(self.native.info, slot.handle)
        # These are CHILD alias-local flags, never parent original fingerprint.
        if type(flags) is not int or flags & 1 != 1 or flags & 2:
            self.fail('child alias flags')
        return identity(dict(info, fileType=kind))

    def class_reserve(self, slot, size):
        self.check()
        key = slot.category
        if key not in self.classes:
            return
        value = self.classes[key]
        if value[1] >= value[3] or size > value[2] - value[0]:
            self.fail('same owner class byte/call admission')
        value[0] += size
        value[1] += 1

    def bootstrap_reserve(self, slot, size):
        self.check()
        if type(size) is not int or not 1 <= size <= CHUNK or self.calls >= GLOBAL_CALLS or size > GLOBAL_BYTES - self.compressed:
            self.fail('global prelude read/seek admission')
        self.class_reserve(slot, size)
        self.compressed += size
        self.calls += 1

    def settle_class(self, slot, reserved, actual):
        if slot.category in self.classes:
            self.classes[slot.category][0] -= reserved - max(1, actual)

    def prepare(self):
        if self.slots:
            self.fail('one inherited stream preparation')
        for ordinal, handle in enumerate(self.command['handles']):
            slot = OriginalStream(self, ordinal, handle)
            self.slots.append(slot)  # partial conversion remains registered
            slot.initial = self.snapshot(slot)
            self.check()
            try:
                slot.fd = self.native.convert_fd(handle)
                if type(slot.fd) is not int or slot.fd < 0:
                    self.fail('original descriptor conversion')
                self.check()
                slot.raw = self.native.stream(slot.fd)
                self.check()
            except BaseException:
                self.failed = True
                raise
        if len(self.slots) != 181:
            self.fail('fixed stream count')
        if sum(v.initial['bytes'] for v in self.slots[:174]) != 144257468 or any(not 0 < v.initial['bytes'] <= 64 << 20 for v in self.slots[:174]):
            self.fail('package metadata census')
        if sum(v.initial['bytes'] for v in self.slots[174:178]) != 9489432 or any(not 0 < v.initial['bytes'] <= 16 << 20 for v in self.slots[174:178]):
            self.fail('index metadata census')
        if self.slots[178].initial != self.command['selectionIdentity'] or self.slots[179].initial['bytes'] != READER[0] or self.slots[180].initial['bytes'] != INSPECTOR[0]:
            self.fail('bootstrap source identity')
        # ALL initial seeks complete before the first read on ANY input.
        for slot in self.slots:
            slot.initial_seek()

    def seed(self, reader):
        self.check()
        if self.seeded or self.budget is not None or len(self.slots) != 181 or any(not v.positioned for v in self.slots):
            self.fail('single measured bootstrap seed')
        before = time.monotonic()
        remaining = (self.command['deadlineTick'] - self.check()) / 1000
        attempt = self

        class OwnerBudget(reader.Budget):
            def check(self):
                super().check()
                attempt.check()

        value = OwnerBudget(before + remaining)
        value.compressed, value.calls = self.compressed, self.calls
        self.seeded, self.budget = True, value
        return value

    def final_identities(self):
        for slot in self.slots:
            if self.snapshot(slot) != slot.initial:
                self.fail('original subject changed')

    def close(self):
        # Cleanup never calls the expired/sticky work clock or releases parent
        # ownership. Exact child fd/stream closure only; parent must retire Job.
        positive = len(self.slots) == 181
        for slot in self.slots:
            if slot.closed:
                continue
            try:
                if slot.raw is not None:
                    slot.raw.close()
                    slot.closed = slot.raw.closed is True
                elif slot.fd is not None:
                    self.native.close_fd(slot.fd)
                    slot.closed = True
                else:
                    slot.closed = False  # unconverted child HANDLE still owned
                positive = positive and slot.closed
            except BaseException:
                positive = False  # preserve exact backing; no path fallback
        self.closed = positive
        if not positive:
            self.failed = True
        return positive


class OriginalStream:
    def __init__(self, attempt, ordinal, handle):
        self.attempt, self.ordinal, self.handle = attempt, ordinal, handle
        self.category = 'package' if ordinal < 174 else 'index' if ordinal < 178 else 'small'
        self.initial, self.fd, self.raw, self.expected = None, None, None, None
        self.positioned = self.initial_attempted = self.interpass = self.closed = False
        self.offset = 0

    def _seek(self):
        a = self.attempt
        a.check()
        if self.raw is None or a.seeks >= 359:
            a.fail('seek owner/count')
        if a.budget is None:
            a.bootstrap_reserve(self, 1)
            ticket = None
        else:
            a.class_reserve(self, 1)
            ticket = a.budget.reserve_read(1)
        a.seeks += 1
        try:
            result = self.raw.seek(0, io.SEEK_SET)
            if type(result) is not int or result != 0:
                a.fail('checked original seek offset')
            a.check()
            if ticket is not None:
                a.budget.settle_read(ticket, 1)
            self.offset = 0
            return 0
        except BaseException:
            if ticket is not None:
                a.budget.fail_read(ticket)
            a.failed = True
            raise

    def initial_seek(self):
        if self.initial_attempted:
            self.attempt.fail('initial seek once')
        self.initial_attempted = True
        self._seek()
        self.positioned = True

    def seek(self, offset, whence=io.SEEK_SET):
        a = self.attempt
        a.check()
        key = self.expected
        if type(offset) is not int or offset != 0 or whence != io.SEEK_SET or not self.positioned or self.interpass or self.category == 'small' or a.budget is None or key is None or (key, 1) not in a.budget.finished or a.budget.passes.get(key) != 1:
            a.fail('single verified inter-pass rewind')
        self.interpass = True
        return self._seek()

    def read(self, size):
        a = self.attempt
        a.check()
        if not self.positioned or any(not v.positioned for v in a.slots) or self.closed or type(size) is not int or not 0 < size <= CHUNK:
            a.fail('fixed original read admission')
        exact = self.initial['bytes']
        amount = min(size, exact - self.offset) if self.offset < exact else 1
        if a.budget is None:
            if self.category != 'small':
                a.fail('no payload before verified bootstrap')
            a.bootstrap_reserve(self, amount)
        else:
            # Unchanged179Metered owns the already reserved GLOBAL ticket.
            pending = a.budget.pending_read
            if pending is None or pending[1] != size:
                a.fail('same179global read reservation required')
            a.class_reserve(self, amount)
        try:
            block = self.raw.read(amount)
            if not isinstance(block, bytes) or len(block) > amount:
                a.fail('original read ACK')
            a.check()
            if a.budget is None:
                a.compressed -= amount - max(1, len(block))
            a.settle_class(self, amount, len(block))
            self.offset += len(block)
            return block
        except BaseException:
            a.failed = True  # pending full request retained;179 latches too
            raise


def json_preflight(raw, maximum):
    if not isinstance(raw, bytes) or not 0 < len(raw) <= maximum:
        raise Refusal('metadata byte bound')
    depth = tokens = 0
    quoted = escape = False
    for value in raw:
        if quoted:
            if escape:
                escape = False
            elif value == 92:
                escape = True
            elif value == 34:
                quoted = False
        elif value == 34:
            quoted = True
        elif value in (91, 123):
            depth += 1
            tokens += 1
            if depth > 16:
                raise Refusal('metadata nesting bound')
        elif value in (93, 125):
            depth -= 1
            if depth < 0:
                raise Refusal('metadata framing')
        elif value in (44, 58):
            tokens += 1
        if tokens > 8192:
            raise Refusal('metadata structural allocation bound')
    if depth or quoted:
        raise Refusal('metadata incomplete')


def decode_selection(raw):
    json_preflight(raw, 262144)
    def pairs(entries):
        out = {}
        for key, value in entries:
            if key in out:
                raise Refusal('duplicate metadata key')
            out[key] = value
        return out
    def parse_number(value):
        if len(value) > 20:
            raise Refusal('metadata integer width')
        return int(value)
    def unsupported(_):
        raise Refusal('noninteger metadata number')
    return json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=pairs,
                      parse_int=parse_number, parse_float=unsupported, parse_constant=unsupported)


def object_ref(value, role, maximum):
    closed(value, ('role', 'sha256', 'bytes'))
    if value['role'] != role:
        raise Refusal('fixed byte role')
    digest(value['sha256'])
    integer(value['bytes'], 1, maximum)
    return value


def catalog_ref(value):
    closed(value, ('schema', 'kind', 'digest', 'bytes'))
    if type(value['schema']) is not int or value['schema'] != 1 or value['kind'] != 'signed-index-bindings':
        raise Refusal('signed index record ref')
    digest(value['digest'])
    integer(value['bytes'], 1, 65536)


def validate_selection(value, attempt):
    closed(value, ('schema', 'abi', 'operationId', 'loanId', 'sourceBindingSha256',
                   'signedIndexes', 'selection', 'indexes', 'fingerprints'))
    if type(value['schema']) is not int or value['schema'] != 1 or value['abi'] != ABI or any(value[key] != attempt.command[flag] for key, flag in (('operationId', '--operation-id'), ('loanId', '--loan-id'), ('sourceBindingSha256', '--source-binding-sha256'))):
        attempt.fail('actual parent selection binding')
    catalog_ref(value['signedIndexes'])
    selection = closed(value['selection'], ('schema', 'dependencyLockSha256', 'acquisitionManifestSha256', 'candidatePolicySha256', 'packages'))
    if type(selection['schema']) is not int or selection['schema'] != 1:
        attempt.fail('selection schema')
    for key in ('dependencyLockSha256', 'acquisitionManifestSha256', 'candidatePolicySha256'):
        digest(selection[key])
    packages = selection['packages']
    if not isinstance(packages, list) or len(packages) != 174:
        attempt.fail('fixed174selection')
    subjects, tuples, total = set(), set(), 0
    for ordinal, row in enumerate(packages):
        closed(row, ('ordinal', 'name', 'version', 'architecture', 'filename', 'bytes', 'sha256', 'allowedCandidateIds'))
        if type(row['ordinal']) is not int or row['ordinal'] != ordinal or row['architecture'] not in ('amd64', 'all'):
            attempt.fail('lock ordinal/architecture')
        for key, maximum in (('name', 128), ('version', 256), ('filename', 512)):
            text(row[key], maximum)
        filename = row['filename']
        if not filename.startswith('pool/') or any(p in ('', '.', '..') for p in filename.split('/')) or '\\' in filename or ':' in filename:
            attempt.fail('evidence filename data only')
        integer(row['bytes'], 1, 64 << 20)
        digest(row['sha256'])
        key = row['name'], row['version'], row['architecture']
        if key in tuples or row['sha256'] in subjects or not isinstance(row['allowedCandidateIds'], list) or not 1 <= len(row['allowedCandidateIds']) <= 6 or len(set(row['allowedCandidateIds'])) != len(row['allowedCandidateIds']):
            attempt.fail('unique finite package bindings')
        for candidate in row['allowedCandidateIds']:
            text(candidate, 128)
        tuples.add(key)
        subjects.add(row['sha256'])
        total += row['bytes']
        attempt.slots[ordinal].expected = row['bytes'], row['sha256']
    if total != 144257468:
        attempt.fail('package aggregate')
    indexes = value['indexes']
    if not isinstance(indexes, list) or len(indexes) != 4:
        attempt.fail('four Packages subjects')
    previous, total, ordinals = '', 0, set()
    for number, row in enumerate(indexes):
        closed(row, ('object', 'bindings'))
        ref = object_ref(row['object'], 'xz-index', 16 << 20)
        if ref['sha256'] <= previous or not isinstance(row['bindings'], list) or not 1 <= len(row['bindings']) <= 6:
            attempt.fail('SHA ordered index subjects')
        previous, total = ref['sha256'], total + ref['bytes']
        last = -1
        for binding in row['bindings']:
            closed(binding, ('ordinal', 'candidateId'))
            n = integer(binding['ordinal'], 0, 11)
            text(binding['candidateId'], 128)
            if n <= last or n in ordinals:
                attempt.fail('logical Packages binding ordinals')
            last = n
            ordinals.add(n)
        attempt.slots[174 + number].expected = ref['bytes'], ref['sha256']
    if total != 9489432 or len(ordinals) != 6:
        attempt.fail('six logical/four byte Packages bindings')
    fingerprints = value['fingerprints']
    slots = list(range(178)) + [179, 180]
    if not isinstance(fingerprints, list) or len(fingerprints) != 180:
        attempt.fail('180shared ordinary fingerprints')
    for expected_slot, row in zip(slots, fingerprints):
        closed(row, ('slot', 'identity'))
        if type(row['slot']) is not int or row['slot'] != expected_slot or identity(row['identity']) != attempt.slots[expected_slot].initial:
            attempt.fail('same original shared identity')
        slot = attempt.slots[expected_slot]
        if expected_slot < 178 and slot.expected[0] != slot.initial['bytes']:
            attempt.fail('selected original size')
    return value


def read_bootstrap(slot, expected):
    a, result, hashed = slot.attempt, bytearray(), hashlib.sha256()
    if slot.initial['bytes'] != expected[0]:
        a.fail('bootstrap exact size')
    while len(result) < expected[0]:
        block = slot.read(min(CHUNK, expected[0] - len(result)))
        if not block:
            a.fail('bootstrap short EOF')
        hashed.update(block)
        result.extend(block)
    if slot.read(1) or hashed.hexdigest() != expected[1]:
        a.fail('bootstrap exactEOF/SHA')
    return bytes(result)


def json_bound(value, remaining, depth=0):
    if depth > 16 or remaining < 0:
        raise Refusal('output JSON allocation bound')
    if isinstance(value, str):
        size = 2
        for char in value:
            n = ord(char)
            size += 2 if char in ('"', '\\') else 6 if n < 32 or n >= 127 and n <= 65535 else 12 if n > 65535 else 1
            if size > remaining:
                raise Refusal('output JSON string bound')
        return size
    if value is None or isinstance(value, bool):
        return 5
    if type(value) is int:
        if not -(1 << 64) < value < (1 << 64):
            raise Refusal('output integer bound')
        return len(str(value))
    if isinstance(value, (list, tuple, dict)):
        total = 2
        entries = value.items() if isinstance(value, dict) else enumerate(value)
        for index, (key, row) in enumerate(entries):
            total += int(index > 0)
            if isinstance(value, dict):
                total += json_bound(key, remaining - total, depth + 1) + 1
            total += json_bound(row, remaining - total, depth + 1)
            if total > remaining:
                raise Refusal('output JSON aggregate bound')
        return total
    raise Refusal('closed output JSON')


def encode_json(value):
    json_bound(value, 65536)
    raw = json.dumps(value, ensure_ascii=True, separators=(',', ':'), allow_nan=False).encode('ascii')
    if len(raw) > 65536:
        raise Refusal('output JSON bound')
    return raw


class OutputFrames:
    def __init__(self, stream, attempt):
        self.stream, self.attempt = stream, attempt
        self.ordinal = self.bytes = self.calls = self.bundle_bytes = 0
        self.refs = []

    def write(self, raw):
        offset = 0
        while offset < len(raw):
            self.attempt.check()
            if self.calls >= 32768:
                self.attempt.fail('finite output write calls')
            self.calls += 1
            chunk = raw[offset:offset + CHUNK]
            try:
                count = self.stream.write(chunk)
                if type(count) is not int or not 0 < count <= len(chunk):
                    self.attempt.fail('partial output write ACK')
                self.attempt.check()
            except BaseException:
                self.attempt.failed = True
                raise
            offset += count

    def emit(self, kind, part, raw):
        self.attempt.check()
        expected = (1 + self.ordinal % 2, self.ordinal // 2) if self.ordinal < 22 else (3, 255)
        maximum = 4 << 20 if kind == 1 else 65536
        if self.ordinal >= 23 or (kind, part) != expected or not isinstance(raw, bytes) or not 0 < len(raw) <= maximum:
            self.attempt.fail('fixed23frame sequence')
        amount = WIRE_HEADER.size + len(raw) + (len(WIRE_MAGIC) if self.ordinal == 0 else 0)
        if amount > (20 << 20) - self.bytes or kind == 1 and len(raw) > (16 << 20) - self.bundle_bytes:
            self.attempt.fail('aggregate output admission')
        self.bytes += amount  # full pending frame stays charged on error
        if kind == 1:
            self.bundle_bytes += len(raw)
        hashed = hashlib.sha256(raw).digest()
        if self.ordinal == 0:
            self.write(WIRE_MAGIC)
        self.write(WIRE_HEADER.pack(kind, part, len(raw), hashed))
        self.write(raw)
        self.refs.append({'kind': kind, 'part': part, 'bytes': len(raw), 'sha256': hashed.hex()})
        self.ordinal += 1


def produce_parts(selection, attempt, reader, readers, output):
    budget = attempt.budget
    packages, observed = selection['selection']['packages'], []
    expected = {(v['name'], v['version'], v['architecture']): v for v in packages}
    for number, subject in enumerate(selection['indexes']):
        slot = attempt.slots[174 + number]
        attempt.check()
        observed.append(reader.scan_packages(slot, slot.expected, expected, budget, readers))
    for part in range(11):
        attempt.check()
        first, count = part * 16, 14 if part == 10 else 16
        segments, rows = [], []
        for package in packages[first:first + count]:
            slot = attempt.slots[package['ordinal']]
            control = reader.read_deb_control(slot, slot.expected, package, budget, readers)
            matches = []
            key = package['name'], package['version'], package['architecture']
            for index, observation in enumerate(observed):
                paragraph = observation['matches'].get(key)
                binding = selection['indexes'][index]
                ordinals = [v['ordinal'] for v in binding['bindings'] if v['candidateId'] in package['allowedCandidateIds']]
                if paragraph is not None and ordinals:
                    matches.append((index, paragraph, ordinals))
            if not matches:
                attempt.fail('no exact allowed signed-index paragraph')
            claims = reader.selected_claims([{'fields': row['fields'], 'bindingOrdinals': ordinals} for _, row, ordinals in matches], control['fields'])
            ordinal = package['ordinal']
            for match, (_, paragraph, _) in enumerate(matches):
                segments.append((ordinal, 1, match, paragraph['raw']))
            segments.extend(((ordinal, 2, 0, control['raw']), (ordinal, 3, 0, claims)))
            rows.append((package, matches))
        bundle, spans = reader.frame_bundle(segments, first, count, budget)
        bundle_ref = {'role': 'binary-binding-bundle', 'sha256': hashlib.sha256(bundle).hexdigest(), 'bytes': len(bundle)}
        span_map = {(s['ordinal'], s['kind'], s['slot']): dict(bundle=bundle_ref, offset=s['offset'], bytes=s['bytes'], sha256=s['sha256']) for s in spans}
        bindings = []
        for package, matches in rows:
            ordinal = package['ordinal']
            matched = []
            for match, (index, paragraph, ordinals) in enumerate(matches):
                observation = observed[index]
                matched.append({'indexBindingOrdinals': ordinals, 'compressedIndex': selection['indexes'][index]['object'],
                                'decodedIndexSha256': observation['decodedSHA256'], 'decodedIndexBytes': observation['decodedBytes'],
                                'paragraphOffset': paragraph['offset'], 'paragraph': span_map[ordinal, 1, match]})
            bindings.append({'lockOrdinal': ordinal, 'name': package['name'], 'version': package['version'],
                             'architecture': package['architecture'], 'filename': package['filename'],
                             'debObject': {'role': 'deb-package', 'sha256': package['sha256'], 'bytes': package['bytes']},
                             'matches': matched, 'control': span_map[ordinal, 2, 0], 'selectedFieldClaims': span_map[ordinal, 3, 0]})
        raw_part = encode_json({'schema': 1, 'recordType': 'luci-debian-binary-binding-part-v1',
                                'operationId': selection['operationId'], 'part': part, 'signedIndexes': selection['signedIndexes'],
                                'dependencyLockSha256': selection['selection']['dependencyLockSha256'],
                                'firstLockOrdinal': first, 'bindings': bindings})
        output.emit(1, part, bundle)
        output.emit(2, part, raw_part)
        # Do not retain the previous emitted bundle while constructing next.
        del bundle, raw_part, segments, rows, span_map, bindings
    summary = budget.observation_summary()
    if attempt.seeks != 359 or any(not s.positioned or s.category != 'small' and not s.interpass for s in attempt.slots):
        attempt.fail('complete359positioning witnesses required')
    attempt.final_identities()
    result = {'schema': 1, 'state': 'complete-observations-not-adopted', 'operationId': selection['operationId'],
              'loanId': selection['loanId'], 'sourceBindingSha256': selection['sourceBindingSha256'],
              'selectionSha256': attempt.command['--selection-sha256'], 'signedIndexes': selection['signedIndexes'],
              'rawOutputs': list(output.refs), 'decodedIndexes': [dict(compressed=subject['object'], decodedSha256=row['decodedSHA256'], decodedBytes=row['decodedBytes']) for subject, row in zip(selection['indexes'], observed)],
              'counters': summary, 'seekCalls': attempt.seeks, 'classDebits': {key: {'bytes': value[0], 'calls': value[1]} for key, value in attempt.classes.items()},
              'packageCount': 174, 'packageBytes': 144257468, 'sourceArchiveBinding': 'not-evaluated',
              'dependencyClosure': 'not-evaluated', 'expectedInstalledFiles': 'not-derived',
              'observedInstalledOutput': 'not-observed', 'serverEligibility': 'not-evaluated-server-owned'}
    encoded = encode_json(result)
    if not attempt.close():
        attempt.fail('child original descriptors closure unknown')
    output.emit(3, 255, encoded)


def run(command, native, output_stream):
    attempt = EntryAttempt(command, native)
    try:
        attempt.prepare()
        selected = read_bootstrap(attempt.slots[178], (command['selectionBytes'], command['--selection-sha256']))
        source = read_bootstrap(attempt.slots[179], READER)
        inspector = read_bootstrap(attempt.slots[180], INSPECTOR)
        value = validate_selection(decode_selection(selected), attempt)
        attempt.check()
        reader = types.ModuleType('luci_owned_binary_reader_179')
        reader.__file__ = '<same-original-pinned-reader-179>'
        exec(compile(source, reader.__file__, 'exec'), reader.__dict__)
        attempt.check()
        readers = reader.load_pinned_readers(inspector)
        attempt.check()
        attempt.seed(reader)
        produce_parts(value, attempt, reader, readers, OutputFrames(output_stream, attempt))
        return 0
    except BaseException:
        attempt.failed = True
        raise
    finally:
        if not attempt.closed:
            attempt.close()


def main():
    try:
        command = parse_argv(sys.argv[1:])
        native = WindowsEntryNative()
        stream = sys.stdout.buffer.raw
        if not isinstance(stream, io.FileIO):
            raise Refusal('unbuffered owned stdout required')
        return run(command, native, stream)
    except BaseException as error:
        # Fixed bounded diagnostic only; never serialize input text/credentials.
        message = ('incomplete-observations:' + type(error).__name__ + '\n').encode('ascii')
        try:
            os.write(2, message[:512])
        except BaseException:
            pass
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
