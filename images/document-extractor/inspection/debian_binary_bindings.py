"""Data-only binary binding readers. No acquisition, paths, launcher or adoption.

The future owned worker supplies original readonly streams and the exact guarded
06b inspector source. Loading this module alone does not load that source or read
anything. Outputs are observations, never a current-authority/closure capability.
"""
import hashlib
import json
import re
import struct
import time
import types

INSPECTOR_SHA256 = '06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba'
INSPECTOR_BYTES = 39189
CHUNK = 65536
MIB = 1024 * 1024
# 2 * (144257468 package + 9489432 index bytes) = 307493800.
# The remaining 28050520 bytes cover counted EOF probes/failed requests. These
# are one attempt's limits, shared across all indexes/packages/eleven parts.
COMPRESSED_LIMIT = 320 * MIB
EXPANDED_LIMIT = 512 * MIB
BUNDLE_MAGIC = b'LUCI-DBB1\n'
BUNDLE_HEADER = struct.Struct('<HBBI32s')
FIELD_NAMES = ('package', 'version', 'architecture', 'depends', 'pre-depends',
               'provides', 'breaks', 'conflicts', 'replaces', 'source',
               'built-using', 'static-built-using', 'multi-arch')
RELATIONS = FIELD_NAMES[3:9]


class Refusal(ValueError):
    pass


def load_pinned_readers(guarded_source):
    """Project source only, not a payload import or path-based fallback.

    The process bridge must hold the source object's original guard before this
    call. Syntax/hash checks here do not establish that ownership or provenance.
    """
    if not isinstance(guarded_source, bytes) or len(guarded_source) != INSPECTOR_BYTES or hashlib.sha256(guarded_source).hexdigest() != INSPECTOR_SHA256:
        raise Refusal('exact frozen inspector source required')
    module = types.ModuleType('luci_pinned_inspector_06b')
    module.__file__ = '<owned-pinned-inspector-06b>'
    exec(compile(guarded_source, module.__file__, 'exec'), module.__dict__)
    return module


class Budget:
    def __init__(self, deadline):
        if not isinstance(deadline, (int, float)) or not time.monotonic() < deadline <= time.monotonic() + 300:
            raise Refusal('bounded work deadline')
        self.deadline = deadline
        self.compressed = self.expanded = self.calls = self.paragraphs = 0
        self.retained = self.index_expanded = 0
        self.passes = {}
        self.finished = set()
        self.objects = {'index': {}, 'package': {}}
        self.failed = False
        self.pending_read = None

    def check(self):
        if self.failed:
            raise Refusal('attempt already failed/exhausted')
        if time.monotonic() >= self.deadline:
            self.failed = True
            raise Refusal('work deadline')

    def reserve_read(self, size):
        """Admit the full bounded request before touching its original stream.

        Short/EOF results refund unused reservation afterwards. A failed or
        unsupported read keeps the entire reservation charged and is sticky.
        """
        self.check()
        if type(size) is not int or not 0 < size <= CHUNK or self.pending_read is not None or self.calls >= 50000 or size > COMPRESSED_LIMIT-self.compressed:
            self.failed = True
            raise Refusal('compressed request/call admission')
        self.calls += 1
        self.compressed += size
        self.pending_read = (self.calls, size)
        return self.pending_read

    def settle_read(self, reservation, actual):
        if self.pending_read != reservation or type(actual) is not int or not 0 <= actual <= reservation[1]:
            self.failed = True
            raise Refusal('read reservation settlement')
        self.pending_read = None
        self.compressed -= reservation[1] - max(1, actual)
        self.check()

    def fail_read(self, reservation):
        # Never refund unknown/failed I/O; all further effects are forbidden.
        if self.pending_read == reservation:
            self.pending_read = None
        self.failed = True

    def charge(self, size):
        self.check()
        if type(size) is not int or not 0 <= size <= CHUNK:
            self.failed = True
            raise Refusal('expansion charge range')
        self.expanded += size
        if self.expanded > EXPANDED_LIMIT:
            self.failed = True
            raise Refusal('aggregate actual expansion')

    def retain(self, size):
        self.check()
        if type(size) is not int or size < 0:
            self.failed = True
            raise Refusal('retained charge range')
        self.retained += size
        if self.retained > 16 * MIB:
            self.failed = True
            raise Refusal('aggregate retained evidence')

    def begin_object(self, kind, identity):
        if kind not in self.objects:
            self.failed = True
            raise Refusal('fixed input class')
        objects = self.objects[kind]
        maximum_count, maximum_bytes = (4, 9489432) if kind == 'index' else (174, 144257468)
        if identity in objects or len(objects) >= maximum_count or identity[0] > maximum_bytes - sum(objects.values()):
            self.failed = True
            raise Refusal('unique input class/count/aggregate')
        objects[identity] = identity[0]

    def observation_summary(self):
        self.check()
        if self.failed:
            raise Refusal('attempt has a failed original observation')
        for kind, count, size in (('index', 4, 9489432), ('package', 174, 144257468)):
            objects = self.objects[kind]
            if len(objects) != count or sum(objects.values()) != size or any((identity, 1) not in self.finished or (identity, 2) not in self.finished for identity in objects):
                raise Refusal('incomplete original observations')
        return {'state': 'complete-observations-not-adopted', 'compressedReadBytes': self.compressed,
                'readCalls': self.calls, 'expandedBytes': self.expanded, 'paragraphs': self.paragraphs}


class Metered:
    """Every actual compressed read, skip and failed read is charged.

    Each pass verifies exact EOF/size/hash. Rewind is only between the two
    complete passes on the SAME supplied original stream, never a reopened path.
    """
    def __init__(self, stream, identity, budget):
        budget.check()
        if not isinstance(identity, tuple) or len(identity) != 2:
            budget.failed = True
            raise Refusal('closed input identity')
        size, digest = identity
        if type(size) is not int or not 0 < size <= 64 * MIB or not isinstance(digest, str) or not re.fullmatch('[0-9a-f]{64}', digest):
            budget.failed = True
            raise Refusal('input identity')
        key = (size, digest)
        budget.passes[key] = budget.passes.get(key, 0) + 1
        if budget.passes[key] > 2:
            budget.failed = True
            raise Refusal('two complete original passes maximum')
        self.stream, self.budget, self.size, self.expected = stream, budget, size, digest
        self.identity, self.pass_number = key, budget.passes[key]
        self.total, self.digest, self.eof = 0, hashlib.sha256(), False

    def read(self, size=CHUNK):
        if type(size) is not int or not 0 <= size <= CHUNK:
            self.budget.failed = True
            raise Refusal('bounded positive compressed read')
        if size == 0:
            self.budget.check()
            return b''  # Section boundary, not an underlying EOF observation.
        self.budget.check()
        if self.eof:
            return b''
        reservation = self.budget.reserve_read(size)
        try:
            block = self.stream.read(size)
        except BaseException:
            self.budget.fail_read(reservation)
            raise
        if not isinstance(block, bytes) or len(block) > size:
            self.budget.fail_read(reservation)
            raise Refusal('stream read contract')
        self.budget.settle_read(reservation, len(block))
        self.total += len(block)
        if self.total > self.size:
            self.budget.failed = True
            raise Refusal('input exceeds exact size')
        self.digest.update(block)
        if not block:
            self.eof = True
        return block

    def finish(self):
        self.budget.check()
        if not self.eof or self.total != self.size or self.digest.hexdigest() != self.expected:
            self.budget.failed = True
            raise Refusal('incomplete/changed original')
        self.budget.finished.add((self.identity, self.pass_number))


def verified_pass(stream, identity, budget, kind):
    # Validate before class accounting; both passes subsequently share one key.
    original = Metered(stream, identity, budget)
    budget.begin_object(kind, identity)
    while original.read(CHUNK):
        pass
    original.finish()
    budget.check()
    stream.seek(0)
    return Metered(stream, identity, budget)


class Expansion:
    def __init__(self, aggregate, maximum, index=False):
        aggregate.check()
        self.aggregate, self.maximum, self.actual, self.index = aggregate, maximum, 0, index

    def check(self):
        self.aggregate.check()

    def charge(self, size):
        self.aggregate.charge(size)
        self.actual += size
        if self.index:
            self.aggregate.index_expanded += size
            if self.aggregate.index_expanded > 256 * MIB:
                self.aggregate.failed = True
                raise Refusal('aggregate index expansion')
        if self.actual > self.maximum:
            self.aggregate.failed = True
            raise Refusal('per-stream actual expansion')


def paragraphs(stream, budget):
    """Physical LF/CRLF, UTF8, case-folded names, case-preserved values.

    Strip SP/TAB at value edges. Each continuation strips its SP/TAB edges and
    joins with exactly ONE ASCII space. Internal whitespace/case/operators stay
    unchanged. Bare CR, other ASCII controls and DEL refuse. Raw bytes retained.
    """
    pending, raw, fields, previous = bytearray(), bytearray(), {}, None
    offset = start = count = 0

    def consume(physical):
        nonlocal offset, start, raw, fields, previous, count
        if len(physical) > 16384:
            raise Refusal('physical line bound')
        line = physical[:-1] if physical.endswith(b'\n') else physical
        if line.endswith(b'\r') and physical.endswith(b'\n'):
            line = line[:-1]
        if any(c < 32 and c != 9 or c == 127 for c in line):
            raise Refusal('physical control character')
        text = line.decode('utf-8', 'strict')
        position = offset
        offset += len(physical)
        if not line:
            if not fields:
                start = offset
                return None
            result = (start, bytes(raw), fields)
            raw, fields, previous, start = bytearray(), {}, None, offset
            count += 1
            budget.paragraphs += 1
            if count > 150000 or budget.paragraphs > 250000:
                raise Refusal('paragraph count')
            return result
        if not raw:
            start = position
        raw.extend(physical)
        if len(raw) > 65536:
            raise Refusal('paragraph byte bound')
        if text[0] in ' \t':
            if previous is None:
                raise Refusal('orphan continuation')
            fields[previous] += ' ' + text.strip(' \t')
        else:
            name, separator, value = text.partition(':')
            if not separator or not re.fullmatch('[A-Za-z0-9-]{1,64}', name):
                raise Refusal('field syntax')
            name = name.lower()  # field NAME only
            if name in fields or len(fields) >= 128:
                raise Refusal('case-alias duplicate/field count')
            fields[name], previous = value.strip(' \t'), name
        if len(fields[previous].encode('utf-8')) > 8192:
            raise Refusal('field byte bound')
        return None

    while True:
        budget.check()
        block = stream.read(CHUNK)
        if not isinstance(block, bytes) or len(block) > CHUNK:
            raise Refusal('decoded stream contract')
        if not block:
            break
        pending.extend(block)
        while True:
            end = pending.find(b'\n')
            if end < 0:
                if len(pending) > 16384:
                    raise Refusal('physical line bound')
                break
            physical = bytes(pending[:end + 1])
            del pending[:end + 1]
            result = consume(physical)
            if result is not None:
                yield result
    if pending:
        result = consume(bytes(pending))
        if result is not None:
            yield result
    result = consume(b'')
    if result is not None:
        yield result


def tuple_key(fields):
    return tuple(fields.get(k) for k in FIELD_NAMES[:3])


def scan_packages(stream, identity, expected, budget, readers):
    try:
        return _scan_packages(stream, identity, expected, budget, readers)
    except BaseException:
        budget.failed = True
        raise


def _scan_packages(stream, identity, expected, budget, readers):
    """One unique compressed Packages object, complete EOF even after matches.

    expected is a finite exact tuple map, not a URL/source/authority callback.
    Logical signed scope remains an API-owner check, never an inference here.
    """
    if not 0 < len(expected) <= 174:
        raise Refusal('expected tuple count')
    original = verified_pass(stream, identity, budget, 'index')
    expansion = Expansion(budget, 128 * MIB, index=True)
    decoded = readers.Decoded(original, 'xz', expansion)
    matches = {}
    for offset, raw, fields in paragraphs(decoded, budget):
        key = tuple_key(fields)
        item = expected.get(key)
        if item is None:
            continue
        if key in matches or fields.get('size') != str(item['bytes']) or fields.get('sha256') != item['sha256'] or fields.get('filename') != item['filename']:
            raise Refusal('ambiguous/foreign package paragraph')
        budget.retain(len(raw))
        matches[key] = {'offset': offset, 'raw': raw, 'fields': fields}
    original.finish()
    return {'decodedBytes': expansion.actual, 'decodedSHA256': decoded.digest.hexdigest(), 'matches': matches}


def drain(stream, readers, size=None):
    if size is None:
        while stream.read(CHUNK):
            pass
    else:
        while size:
            block = readers.exact(stream, min(CHUNK, size), CHUNK)
            size -= len(block)


def control_tar(stream, budget, readers):
    seen, control, members = set(), None, 0
    while True:
        header = readers.exact(stream, 512)
        if header == bytes(512):
            if readers.exact(stream, 512) != bytes(512):
                raise Refusal('tar terminator')
            while True:
                trailing = stream.read(CHUNK)
                if not trailing:
                    break
                if any(trailing):
                    raise Refusal('tar trailing payload')
            break
        members += 1
        if members > 64 or sum(header[:148]) + 256 + sum(header[156:]) != readers.octal(header[148:156]) or header[257:265] != b'ustar\x0000':
            raise Refusal('control tar count/checksum/dialect')
        name = header[:100].split(b'\0', 1)[0].decode('utf-8', 'strict')
        prefix = header[345:500].split(b'\0', 1)[0].decode('utf-8', 'strict')
        if any(ord(c) < 32 or ord(c) == 127 for c in name + prefix):
            raise Refusal('control path control characters')
        name = readers.path_name((prefix + '/' if prefix else '') + name)
        kind, size = header[156:157], readers.octal(header[124:136])
        if name in seen or kind not in (b'0', b'\0', b'5') or size > MIB or (kind == b'5' and size) or (not name and kind != b'5'):
            raise Refusal('control path/type/size')
        seen.add(name)
        if name == 'control':
            if kind == b'5' or size > 65536:
                raise Refusal('one bounded regular control')
            control = readers.exact(stream, size, 65536)
        else:
            drain(stream, readers, size)
        if any(readers.exact(stream, (-size) % 512)):
            raise Refusal('tar nonzero padding')
    if control is None:
        raise Refusal('missing control')
    budget.retain(len(control))
    return control


def read_deb_control(stream, identity, expected, budget, readers):
    try:
        return _read_deb_control(stream, identity, expected, budget, readers)
    except BaseException:
        budget.failed = True
        raise


def _read_deb_control(stream, identity, expected, budget, readers):
    # Tuple identity alone cannot bind a byte subject. Refuse before any original
    # read/seek/decoder when the separately selected locked size/SHA differs.
    if not isinstance(expected, dict) or type(expected.get('bytes')) is not int or not 0 < expected['bytes'] <= 64 * MIB or not isinstance(expected.get('sha256'), str) or not re.fullmatch('[0-9a-f]{64}', expected['sha256']) or identity != (expected['bytes'], expected['sha256']):
        raise Refusal('expected exact package byte subject')
    if any(not isinstance(expected.get(key), str) or not expected[key] or len(expected[key].encode('utf-8')) > maximum for key, maximum in (('name', 128), ('version', 256), ('architecture', 5))) or expected['architecture'] not in ('amd64', 'all'):
        raise Refusal('expected closed package tuple')
    original = verified_pass(stream, identity, budget, 'package')
    if readers.exact(original, 8) != b'!<arch>\n':
        raise Refusal('ar magic')
    control = None
    for ordinal in range(3):
        header = readers.exact(original, 60)
        name = header[:16].rstrip(b' ').rstrip(b'/').decode('ascii', 'strict')
        length = header[48:58].strip()
        if header[58:] != b'`\n' or not length.isdigit():
            raise Refusal('ar framing')
        size = int(length)
        if size > 64 * MIB:
            raise Refusal('ar member bound')
        section = readers.Section(original, size)
        if ordinal == 0:
            if name != 'debian-binary' or size != 4 or readers.exact(section, 4) != b'2.0\n':
                raise Refusal('ar version/order')
        elif ordinal == 1:
            if name not in ('control.tar.gz', 'control.tar.xz', 'control.tar.bz2'):
                raise Refusal('control member/order')
            expanded = Expansion(budget, MIB)
            control = control_tar(readers.Decoded(section, name.rsplit('.', 1)[1], expanded), budget, readers)
        else:
            if name not in ('data.tar.gz', 'data.tar.xz', 'data.tar.bz2'):
                raise Refusal('data member/order')
            drain(section, readers)  # compressed bytes only, never decode data.tar
        if section.remaining or (size % 2 and readers.exact(original, 1) != b'\n'):
            raise Refusal('ar section/alignment')
    if original.read(1):
        raise Refusal('extra ar member/trailing bytes')
    original.finish()
    # Reuse the same strict physical record reader on already bounded raw control.
    import io
    rows = list(paragraphs(io.BytesIO(control), budget))
    if len(rows) != 1 or tuple_key(rows[0][2]) != (expected['name'], expected['version'], expected['architecture']):
        raise Refusal('exact control identity')
    return {'raw': control, 'fields': rows[0][2]}


def selected_claims(indexes, control):
    """Closed case-preserved values; no dependency/source/install evaluation."""
    if not 0 < len(indexes) <= 4:
        raise Refusal('matched index observations')
    chosen = {k: control.get(k) for k in FIELD_NAMES}

    def bounded_values(fields):
        for value in fields.values():
            if value is not None and (not isinstance(value, str) or len(value.encode('utf-8')) > 8192 or any(ord(c) < 32 and c != '\t' or ord(c) == 127 for c in value)):
                raise Refusal('selected scalar value')

    bounded_values(chosen)
    selected = []
    for index in indexes:
        fields = {k: index['fields'].get(k) for k in FIELD_NAMES}
        bounded_values(fields)
        if any(fields[k] != chosen[k] for k in FIELD_NAMES[:3] + RELATIONS):
            raise Refusal('index/control identity or relation mismatch')
        ordinals = index['bindingOrdinals']
        if not ordinals or ordinals != sorted(set(ordinals)) or any(type(n) is not int or not 0 <= n < 12 for n in ordinals):
            raise Refusal('logical binding ordinals')
        selected.append({'bindingOrdinals': ordinals, 'fields': fields})
    result = bytearray()
    encoder = json.JSONEncoder(ensure_ascii=False, separators=(',', ':'), sort_keys=True)
    for text in encoder.iterencode({'schema': 1, 'control': chosen, 'indexes': selected}):
        block = text.encode('utf-8')
        if len(block) > 65536-len(result):
            raise Refusal('selected claims bound')
        result.extend(block)
    return bytes(result)


def frame_bundle(segments, first_ordinal, count, budget):
    """Closed framing: ordinal u16, kind u8, slot u8, size u32, raw SHA32.

    kind1 paragraph slots0..3; kind2 control/kind3 claims require slot0.
    Strict sorted unique keys. All rows require control+claims+>=1 paragraph.
    """
    if type(first_ordinal) is not int or first_ordinal % 16 or not 0 <= first_ordinal <= 160 or count != (14 if first_ordinal == 160 else 16):
        raise Refusal('part range')
    output, spans, previous, seen, slots = bytearray(BUNDLE_MAGIC), [], None, {}, {}
    for ordinal, kind, slot, raw in segments:
        key = ordinal, kind, slot
        if not first_ordinal <= ordinal < first_ordinal + count or kind not in (1, 2, 3) or not 0 <= slot < (4 if kind == 1 else 1) or (previous is not None and key <= previous) or not isinstance(raw, bytes) or not 0 < len(raw) <= 65536:
            raise Refusal('closed bundle segment/order')
        previous = key
        if kind == 1:
            if slot != slots.get(ordinal, 0):
                raise Refusal('contiguous paragraph slots')
            slots[ordinal] = slot + 1
        seen.setdefault(ordinal, set()).add(kind)
        digest = hashlib.sha256(raw).digest()
        if BUNDLE_HEADER.size + len(raw) > 4 * MIB-len(output):
            raise Refusal('bundle byte limit')
        output.extend(BUNDLE_HEADER.pack(ordinal, kind, slot, len(raw), digest))
        spans.append({'ordinal': ordinal, 'kind': kind, 'slot': slot, 'offset': len(output), 'bytes': len(raw), 'sha256': digest.hex()})
        output.extend(raw)
    if len(seen) != count or any(kinds != {1, 2, 3} for kinds in seen.values()):
        raise Refusal('incomplete part observations')
    # Raw paragraphs/control have already charged retain(); framing/selected
    # claims also charge here. This intentionally never resets between parts.
    budget.retain(len(output))
    return bytes(output), spans
