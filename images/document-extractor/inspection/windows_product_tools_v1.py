"""Exact two candidate wrappers only. No install, payload imports or full closure."""
import argparse
from contextlib import ExitStack
import hashlib
import http.client
import io
import json
import os
from pathlib import Path
import ssl
import stat
import time
from urllib.parse import urlsplit

CHUNK = 65536
MANIFEST_SHA256 = '04d9bf5b016ae904c2a3dc890a9c7977996bd12bc425cdd2502a6ed11a8b1d6a'
DONOR = Path('E:/Luci-local-acceptance/artifacts/extractor-wheels-65a8dafb-source-20261016/inspect_inputs.py')
DONOR_SHA256 = '06b77ce9d7f2d6d06ec5b532cfc965acccd4619322aaf5e17cfa138e6a90acba'
SOURCES = {
    'windows_product_tools_zip_v1.py': '57f9425451e9da39c44222d240936cd37e6453fcd1e6c600d4b1f2a2f02160ea',
    'windows_product_tools_pe_v1.py': '49c0f20562ae8020b9b9d0f90238ba01b107d7ee64a0053dac53cc5e4e09bab6',
}
OBJECTS = (
    {'id': 'cpython', 'version': '3.13.16', 'file': 'python-3.13.16-embed-amd64.zip', 'url': 'https://www.python.org/ftp/python/3.13.16/python-3.13.16-embed-amd64.zip', 'bytes': 11439094, 'sha256': '97dae5274cc54867065e8d5a3226e48c35017ed332a0fdb0e27d5b5821961297'},
    {'id': 'gnupg', 'version': '2.5.24_20260923', 'file': 'gnupg-w32-2.5.24_20260923.exe', 'url': 'https://www.gnupg.org/ftp/gcrypt/binary/gnupg-w32-2.5.24_20260923.exe', 'bytes': 5804864, 'sha256': 'ea6f9dee2cccd83287432625b8b399173805add81d27067fcaf019c0c4da1ad8'},
)


class Refusal(ValueError):
    pass


def json_bound(value, maximum, depth=0):
    """Bound ensure_ascii compact JSON BEFORE serializer allocation."""
    if depth > 16:
        raise Refusal('json_depth')
    if isinstance(value, str):
        size = 2
        for char in value:
            n = ord(char)
            size += 2 if char in ('"', '\\') else 6 if n < 32 or 127 < n <= 65535 else 12 if n > 65535 else 1
            if size > maximum:
                raise Refusal('json_allocation_bound')
        return size
    if value is None or isinstance(value, bool):
        return 5
    if isinstance(value, int):
        return len(str(value))
    if isinstance(value, (list, tuple, dict)):
        total = 2
        entries = value.items() if isinstance(value, dict) else enumerate(value)
        for index, (key, entry) in enumerate(entries):
            total += int(index > 0)
            if isinstance(value, dict):
                total += json_bound(key, maximum - total, depth + 1) + 1
            total += json_bound(entry, maximum - total, depth + 1)
            if total > maximum:
                raise Refusal('json_allocation_bound')
        return total
    raise Refusal('json_closed_type')


class Ticket:
    def __init__(self, owner, kind, amount):
        self.owner, self.kind, self.amount, self.done = owner, kind, amount, False


class Budget:
    def __init__(self, deadline):
        self.deadline = deadline
        self.original = self.expanded = self.calls = self.retained = 0
        self.failed = False

    def fail(self, code):
        self.failed = True
        raise Refusal(code)

    def check(self):
        if self.failed or time.monotonic() >= self.deadline:
            self.fail('sticky_or_deadline')

    def allocate(self, amount, maximum):
        self.check()
        if not isinstance(amount, int) or not 0 <= amount <= maximum:
            self.fail('allocation_bound')

    def effect(self, count=1):
        self.check()
        if self.calls > 8192 - count:
            self.fail('effect_call_bound')
        self.calls += count

    def reserve(self, kind, amount, calls=1):
        self.check()
        if kind not in ('original', 'expanded') or not isinstance(amount, int) or not 1 <= amount <= CHUNK:
            self.fail('request_bound')
        current = getattr(self, kind)
        if amount > (32 << 20) - current or amount > (64 << 20) - self.original - self.expanded:
            self.fail('lifetime_byte_bound')
        self.effect(calls)
        setattr(self, kind, current + amount)
        return Ticket(self, kind, amount)

    def finish(self, ticket, actual):
        self.check()
        if ticket.owner is not self or ticket.done or not isinstance(actual, int) or not 0 <= actual <= ticket.amount:
            self.fail('read_ack_bound')
        ticket.done = True
        charge = max(1, actual) if ticket.kind == 'original' else actual
        setattr(self, ticket.kind, getattr(self, ticket.kind) - ticket.amount + charge)

    def reserve_expansion(self, amount):
        return self.reserve('expanded', amount)

    def finish_expansion(self, ticket, actual):
        self.finish(ticket, actual)

    def retain(self, row):
        self.check()
        remaining = (2 << 20) - 65536 - self.retained
        try:
            size = json_bound(row, remaining)
        except Refusal:
            self.fail('observation_bound')
        if size > remaining:
            self.fail('observation_bound')
        self.retained += size


class Original:
    """One already locked original; every seek/read pre-admitted, no path reopen."""
    def __init__(self, stream, expected, budget):
        self.stream, self.expected, self.budget = stream, expected, budget
        budget.effect()
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_size != expected['bytes'] or info.st_nlink != 1 or os.name == 'nt' and (not hasattr(info, 'st_file_attributes') or info.st_file_attributes & 1024):
            budget.fail('original_identity')
        self.identity = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)

    def exact_at(self, offset, size):
        b = self.budget
        b.allocate(size, 16 << 20)
        if not 0 <= offset <= self.expected['bytes'] or size > self.expected['bytes'] - offset:
            b.fail('original_range')
        result = bytearray()
        while len(result) < size:
            amount = min(CHUNK, size - len(result))
            ticket = b.reserve('original', amount, calls=2)
            try:
                if self.stream.seek(offset + len(result)) != offset + len(result):
                    b.fail('original_seek_ack')
                block = self.stream.read(amount)
                if not isinstance(block, bytes):
                    b.fail('original_read_type')
                if not block:
                    b.fail('original_short_eof')
                b.finish(ticket, len(block))
            except BaseException:
                b.failed = True  # pending full demand stays charged on failure
                raise
            result.extend(block)
        return bytes(result)

    def verify(self, keep=False):
        b, digest, offset, body = self.budget, hashlib.sha256(), 0, bytearray()
        if keep:
            b.allocate(self.expected['bytes'], 16 << 20)
        while offset < self.expected['bytes']:
            block = self.exact_at(offset, min(CHUNK, self.expected['bytes'] - offset))
            digest.update(block)
            offset += len(block)
            if keep:
                body.extend(block)
        ticket = b.reserve('original', 1, calls=2)
        try:
            if self.stream.seek(offset) != offset:
                b.fail('original_seek_ack')
            extra = self.stream.read(1)
            b.finish(ticket, len(extra))
            if extra:
                b.fail('original_exact_eof')
        except BaseException:
            b.failed = True
            raise
        if digest.hexdigest() != self.expected['sha256']:
            b.fail('original_sha256')
        self.current()
        return bytes(body) if keep else None

    def current(self):
        self.budget.effect()
        v = os.fstat(self.stream.fileno())
        if self.identity != (v.st_dev, v.st_ino, v.st_size, v.st_mtime_ns, v.st_ctime_ns) or v.st_nlink != 1:
            self.budget.fail('original_changed')


def _source_bytes_owned(path, expected, maximum=65536, budget=None):
    # Source is consumed from these exact checked bytes, never imported by path.
    if budget is not None:
        budget.allocate(maximum + 1, 65537)
        budget.effect()
    with path.open('rb', buffering=0) as stream:
        blocks, size = [], 0
        while True:
            amount = min(CHUNK, maximum + 1 - size)
            ticket = budget.reserve('original', amount) if budget is not None else None
            try:
                block = stream.read(amount)
                if not isinstance(block, bytes):
                    raise Refusal('source_read_type')
                if budget is not None:
                    budget.finish(ticket, len(block))
            except BaseException:
                if budget is not None:
                    budget.failed = True
                raise
            size += len(block)
            if size > maximum:
                if budget is not None:
                    budget.fail('source_bound')
                raise Refusal('source_bound')
            blocks.append(block)
            if not block:
                break
        raw = b''.join(blocks)
    if len(raw) > maximum or hashlib.sha256(raw).hexdigest() != expected:
        if budget is not None:
            budget.fail('source_pin')
        raise Refusal('source_pin')
    return raw


def source_bytes(path, expected, maximum=65536, budget=None):
    try:
        return _source_bytes_owned(path, expected, maximum, budget)
    except BaseException:
        if budget is not None:
            budget.failed = True
        raise


def load_source(path, expected, maximum=65536, budget=None):
    raw = source_bytes(path, expected, maximum, budget)
    namespace = {'__name__': 'luci_fixed_tool_inspection', '__file__': str(path)}
    exec(compile(raw, str(path), 'exec'), namespace)
    return namespace


class FixedHTTPReader:
    """Logical input accounting, not SSL/OS buffering or socket cancellation."""
    def __init__(self, raw, close_owner, budget):
        self.raw, self.close_owner, self.budget = raw, close_owner, budget
        self.pending, self.header_bytes, self.headers = b'', 0, True

    def readline(self, maximum=-1):
        self.budget.check()
        if not self.headers or not isinstance(maximum, int) or maximum == 0:
            self.budget.fail('header_read_mode')
        maximum = min(65537, maximum) if maximum >= 0 else 65537
        result = bytearray()
        while len(result) < maximum:
            self.budget.check()
            if self.pending:
                newline = self.pending.find(b'\n')
                count = min(len(self.pending) if newline < 0 else newline + 1, maximum - len(result))
                result.extend(self.pending[:count])
                self.pending = self.pending[count:]
                if result.endswith(b'\n') or len(result) == maximum:
                    break
            amount = min(8192, 65536 - self.header_bytes)
            if amount <= 0:
                self.budget.fail('header_lifetime_bound')
            ticket = self.budget.reserve('original', amount)
            self.header_bytes += amount
            try:
                block = self.raw.read(amount)
                if not isinstance(block, bytes) or len(block) > amount:
                    self.budget.fail('header_read_ack')
                self.budget.finish(ticket, len(block))
            except BaseException:
                self.budget.failed = True
                raise
            self.header_bytes -= amount - max(1, len(block))
            if not block:
                break
            self.pending = block
        return bytes(result)

    def read(self, size):
        # The body caller reserves its whole request before reaching this edge.
        # Header-prefetched body stays conservatively charged a second time.
        self.budget.check()
        if self.headers or not isinstance(size, int) or not 1 <= size <= CHUNK:
            self.budget.fail('body_read_mode')
        if self.pending:
            block, self.pending = self.pending[:size], self.pending[size:]
            return block
        return self.raw.read(size)

    @property
    def closed(self):
        return self.close_owner.closed

    def close(self):
        self.close_owner.close()


def fixed_response_class(response_class, budget):
    class FixedResponse(response_class):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, **kwargs)
            try:
                # Donor constructs its HeaderReader over an untouched stdlib
                # BufferedReader. Retain that sole close owner, but bypass its
                # prefetch: all logical header reads use the same raw SocketIO.
                owner = self.fp.stream
                if not isinstance(owner, io.BufferedReader) or not isinstance(owner.raw, io.RawIOBase):
                    budget.fail('http_raw_reader_required')
                self.input_reader = FixedHTTPReader(owner.raw, owner, budget)
                self.fp.stream = self.input_reader
            except BaseException:
                budget.failed = True
                self.close()
                raise

        def begin(self):
            try:
                result = super().begin()
                budget.check()
                self.input_reader.headers = False
                return result
            except BaseException:
                budget.failed = True
                self.close()  # getresponse may never return this owned reader.
                raise
    return FixedResponse


def acquire(item, output, budget, response_class, connection_factory=http.client.HTTPSConnection):
    if item not in OBJECTS:
        budget.fail('fixed_object_only')
    url = urlsplit(item['url'])
    budget.check()
    client = connection_factory(url.hostname, port=443, timeout=min(30, max(.001, budget.deadline - time.monotonic())), context=ssl.create_default_context())
    client.response_class = fixed_response_class(response_class, budget)
    response = None
    partial = output / (item['file'] + '.partial')
    try:
        client.request('GET', url.path, headers={'Accept-Encoding': 'identity', 'Connection': 'close', 'User-Agent': 'Luci-static-product-tool-inspection/1'})
        response = client.getresponse()
        lengths = response.headers.get_all('Content-Length', [])
        if response.status != 200 or lengths != [str(item['bytes'])] or response.getheader('Content-Encoding') or response.getheader('Transfer-Encoding') or response.getheader('Content-Range'):
            budget.fail('http_exact_framing')
        total, digest = 0, hashlib.sha256()
        with partial.open('xb') as target:
            while True:
                amount = min(CHUNK, item['bytes'] - total + 1)
                ticket = budget.reserve('original', amount)
                try:
                    block = response.fp.read(amount)
                    if not isinstance(block, bytes) or not block and total != item['bytes']:
                        budget.fail('transfer_short_or_invalid_read')
                    budget.finish(ticket, len(block))
                except BaseException:
                    budget.failed = True
                    raise
                if not block:
                    break
                if len(block) > item['bytes'] - total:
                    budget.fail('transfer_overrun')
                budget.allocate(len(block), CHUNK)
                written = target.write(block)
                if written != len(block):
                    budget.fail('transfer_write_ack')
                total += len(block)
                digest.update(block)
        if total != item['bytes'] or digest.hexdigest() != item['sha256']:
            budget.fail('transfer_identity')
        budget.check()
        os.link(partial, output / item['file'])
        partial.unlink()
        return {'file': item['file'], 'bytes': total, 'sha256': digest.hexdigest()}
    except BaseException:
        budget.failed = True
        raise
    finally:
        if response is not None:
            response.close()
        client.close()


def inspect_gnupg_wrapper_v1(data, wrapper, budget, pe_inspector):
    pe = pe_inspector(data, budget)
    start, size = pe['overlayFileOffset'], pe['overlayBytes']
    prefix = min(65536, size)
    budget.allocate(prefix, CHUNK)
    return {'file': wrapper['file'], 'bytes': wrapper['bytes'], 'sha256': wrapper['sha256'],
            'installerPE': pe, 'opaqueOverlayPrefixBytes': prefix,
            'opaqueOverlayPrefixSHA256': hashlib.sha256(data[start:start + prefix]).hexdigest(),
            'nsisPayloadDecoded': False, 'payloadArchitectureProven': False}


def _bounded_json_owned(path, budget):
    # Three fixed metadata roles only; smaller than the donor's2MiB ceiling.
    budget.effect()
    with path.open('rb', buffering=0) as stream:
        blocks, size = [], 0
        try:
            while True:
                amount = min(CHUNK, CHUNK + 1 - size)
                ticket = budget.reserve('original', amount)
                block = stream.read(amount)
                if not isinstance(block, bytes):
                    budget.fail('receipt_read_type')
                budget.finish(ticket, len(block))
                size += len(block)
                if size > CHUNK:
                    budget.fail('receipt_bound')
                blocks.append(block)
                if not block:
                    break
        except BaseException:
            budget.failed = True
            raise
    try:
        return json.loads(b''.join(blocks))
    except BaseException:
        budget.failed = True
        raise


def bounded_json(path, budget):
    try:
        return _bounded_json_owned(path, budget)
    except BaseException:
        budget.failed = True
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--phase', choices=('acquire', 'inspect'), required=True)
    parser.add_argument('--batch', choices=('windows-product-tools-v1',), required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--input', type=Path)
    parser.add_argument('--deadline', type=float, required=True)
    args = parser.parse_args()
    if not time.monotonic() < args.deadline <= time.monotonic() + 300:
        raise Refusal('deadline')
    budget = Budget(args.deadline)
    here = Path(__file__).parent
    donor = load_source(DONOR, DONOR_SHA256, budget=budget)
    output = donor['regular'](args.output, directory=True)
    intent = bounded_json(output / 'intent.json', budget)
    worker_raw = source_bytes(Path(__file__), intent.get('workerSha256'), budget=budget)
    worker_sha = hashlib.sha256(worker_raw).hexdigest()
    if intent.get('phase') != args.phase or intent.get('batch') != args.batch or intent.get('manifestSha256') != MANIFEST_SHA256 or intent.get('workerSha256') != worker_sha:
        raise Refusal('owned_intent')
    manifest = json.loads(source_bytes(here / 'windows_product_tools_v1_manifest.json', MANIFEST_SHA256, budget=budget))
    if manifest['objects'] != list(OBJECTS):
        raise Refusal('manifest_closed_objects')
    libraries = {name: load_source(here / name, pin, budget=budget) for name, pin in SOURCES.items()}
    receipt = {'version': 1, 'phase': args.phase, 'batch': args.batch, 'manifestSha256': MANIFEST_SHA256,
               'state': 'incomplete_no_replay', 'inputs': [], 'authenticated': False,
               'productProfileAvailable': False, 'sbomClosed': False}
    try:
        if args.phase == 'acquire':
            if args.input is not None:
                budget.fail('acquire_input_forbidden')
            for item in OBJECTS:
                receipt['inputs'].append(acquire(item, output, budget, donor['BoundedHTTPResponse']))
            receipt['state'] = 'all_transfers_exactly_verified'
        else:
            if args.input is None:
                budget.fail('prior_acquisition_required')
            source = donor['regular'](args.input, directory=True)
            prior = bounded_json(source / 'controller-receipt.json', budget)
            transfers = bounded_json(source / 'worker-result.json', budget)
            if prior.get('state') != 'completed_owned_worker_retired' or prior.get('phase') != 'acquire' or prior.get('batch') != args.batch or prior.get('manifestSha256') != MANIFEST_SHA256 or prior.get('workerSha256') != worker_sha or not prior.get('treeSettled') or not prior.get('pdhClosed') or transfers.get('state') != 'all_transfers_exactly_verified' or transfers.get('inputs') != [{'file': v['file'], 'bytes': v['bytes'], 'sha256': v['sha256']} for v in OBJECTS]:
                budget.fail('prior_owned_acquisition')
            # Both originals stay locked through rehash, parsing and final checks.
            with ExitStack() as stack:
                originals = []
                for item in OBJECTS:
                    path = donor['regular'](source / item['file'])
                    stream = stack.enter_context(donor['locked_input'](path))
                    # No BufferedReader prefetch beyond an admitted request.
                    raw = stream.raw
                    if not isinstance(raw, io.FileIO):
                        budget.fail('unbuffered_original_required')
                    originals.append(Original(raw, item, budget))
                originals[0].verify()
                gnupg_bytes = originals[1].verify(keep=True)
                pe = libraries['windows_product_tools_pe_v1.py']['inspect_pe_v1']
                python = libraries['windows_product_tools_zip_v1.py']['inspect_cpython_embed_v1'](originals[0], OBJECTS[0], budget, pe)
                gnupg = inspect_gnupg_wrapper_v1(gnupg_bytes, OBJECTS[1], budget, pe)
                for original in originals:
                    original.current()
                receipt['inputs'] = [dict(file=OBJECTS[0]['file'], bytes=OBJECTS[0]['bytes'], sha256=OBJECTS[0]['sha256'], inspection=python), gnupg]
                receipt['state'] = 'static_inspection_complete_with_explicit_evidence_gaps'
                candidate = {'kind': 'incomplete_inspection_not_product_profile', 'cpythonFiles': [{k: v[k] for k in ('path', 'bytes', 'sha256')} for v in python['members'] if not v['directory']], 'gnupgRuntimeFiles': [], 'missing': manifest['notEstablished']}
                budget.retain(candidate)
                receipt['manifestCandidate'] = candidate
        budget.check()
    except BaseException:
        budget.failed = True
        receipt['state'], receipt['inputs'] = 'incomplete_no_replay', []
        receipt.pop('manifestCandidate', None)
    receipt['accounting'] = {'originalBytes': budget.original, 'expandedBytes': budget.expanded, 'calls': budget.calls, 'stickyFailure': budget.failed}
    json_bound(receipt, (2 << 20) - 1)
    raw = json.dumps(receipt, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('utf-8') + b'\n'
    if len(raw) > 2 << 20:
        raise Refusal('final_observation_bound')
    with (output / 'worker-result.json').open('xb') as target:
        if target.write(raw) != len(raw):
            raise Refusal('receipt_write_ack')
    if budget.failed:
        raise SystemExit(2)


if __name__ == '__main__':
    main()
