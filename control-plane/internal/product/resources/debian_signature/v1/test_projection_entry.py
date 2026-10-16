"""Synthetic source fixtures only; no native tools, keys or releases are read.

These cases do not prove the still-missing Go copied-original lifetime seam,
Windows containment, packaged tool eligibility or an actual signature result.
"""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import stat
import types
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location('product_projection_entry', HERE / 'projection_entry.py')
ENTRY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ENTRY)


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode() + b'\n'


def budget(maximum=ENTRY.READ_MAX, calls=ENTRY.CALL_MAX):
    return ENTRY.Budget(10, maximum, calls, clock=lambda: 0)


def request():
    refs = []
    for role, _, _ in ENTRY.ROLES:
        data = ('synthetic:' + role).encode()
        refs.append(dict(role=role, bytes=len(data), sha256=hashlib.sha256(data).hexdigest()))
    refs[0].update(bytes=ENTRY.CONFIG_BYTES, sha256=ENTRY.CONFIG_SHA)
    refs[1].update(bytes=372, sha256=ENTRY.SNAPSHOT_SHA)
    return dict(schema=ENTRY.REQUEST_SCHEMA, protocol=ENTRY.PROTOCOL,
                observedUnix=1792195200,
                children=[dict(ordinal=i, exit=0) for i in range(9)], files=refs)


class CountedFile(io.BytesIO):
    def __init__(self, data, chunk=None, broken=False, bad_close=False):
        super().__init__(data)
        self.chunk, self.broken, self.bad_close = chunk, broken, bad_close
        self.read_calls, self.closed_calls = 0, 0

    def fileno(self):
        return 123

    def read(self, size=-1):
        self.read_calls += 1
        if self.broken:
            raise OSError('synthetic read refusal')
        return super().read(min(size, self.chunk) if self.chunk else size)

    def close(self):
        self.closed_calls += 1
        if self.bad_close:
            raise OSError('synthetic close refusal')
        super().close()


class FakeOpen:
    def __init__(self, stream):
        self.stream, self.calls = stream, []

    def __call__(self, path, mode, **kwargs):
        self.calls.append((path, mode, kwargs))
        return self.stream


class ProjectionEntryTests(unittest.TestCase):
    def test_request_exact_roles_and_crypto_exits_are_metadata_only(self):
        value = request()
        parsed = ENTRY.request_value(encoded(value), budget())
        self.assertEqual(parsed, value)
        self.assertEqual(len(parsed['files']), 13)
        self.assertFalse(any(k in parsed for k in ('paths', 'argv', 'environment', 'completion')))
        value['children'][4]['exit'] = 2
        self.assertEqual(ENTRY.request_value(encoded(value), budget())['children'][4]['exit'], 2)

    def test_request_rejects_unknown_duplicate_and_trailing_data(self):
        raw = encoded(request())
        cases = [raw[:-2] + b',"paths":{}}\n',
                 b'{"schema":"x","schema":"x"}', raw + b'{}', b'\xef\xbb\xbf' + raw]
        for value in cases:
            b = budget()
            with self.assertRaises(ENTRY.Refusal):
                ENTRY.request_value(value, b)
            self.assertTrue(b.failed)

    def test_request_integer_role_order_and_closed_shapes_refuse(self):
        for change in ('bool', 'float', 'foreign', 'swap', 'extra', 'exit', 'config'):
            value = request()
            if change == 'bool': value['observedUnix'] = True
            if change == 'float': value['observedUnix'] = 1.5
            if change == 'foreign': value['files'][2]['role'] = '../secret'
            if change == 'swap': value['files'][2], value['files'][3] = value['files'][3], value['files'][2]
            if change == 'extra': value['children'][0]['pid'] = 123
            if change == 'exit': value['children'][0]['exit'] = 2
            if change == 'config': value['files'][0]['sha256'] = 'a' * 64
            with self.assertRaises(ENTRY.Refusal, msg=change):
                ENTRY.request_value(encoded(value), budget())

    def test_actual_owned_config_resource_has_new_family_and_no_diagnostic_paths(self):
        raw = (HERE / 'producer_config.json').read_bytes()
        value = ENTRY.config_value(raw, budget())
        self.assertEqual(hashlib.sha256(raw).hexdigest(), ENTRY.CONFIG_SHA)
        self.assertEqual(value['schema'], ENTRY.CONFIG_SCHEMA)
        self.assertEqual(len(value['projectionConfig']['keys']), 3)
        self.assertEqual(len(value['projectionConfig']['releases']), 5)
        self.assertEqual(sum(len(v['candidates']) for v in value['projectionConfig']['releases']), 6)
        for forbidden in (b'E:/', b'E:\\', b'Program Files', b'retainedText', b'dllDirectory',
                          b'inner-receipt', b'outer-receipt', b'sourceOnly'):
            self.assertNotIn(forbidden, raw)
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.config_value(raw + b' ', budget())

    def test_frozen_pure_resource_keeps_exact_bytes_without_executing_it(self):
        raw = (HERE / 'status_projection.py').read_bytes()
        self.assertEqual(len(raw), ENTRY.PURE_BYTES)
        self.assertEqual(hashlib.sha256(raw).hexdigest(), ENTRY.PURE_SHA)
        with patch('builtins.compile', side_effect=AssertionError('must not compile')):
            with self.assertRaises(ENTRY.Refusal):
                ENTRY.load_pure(raw + b'\n', budget())

    def test_exhausted_aggregate_refuses_before_open_or_read(self):
        stream = CountedFile(b'abc')
        opener = FakeOpen(stream)
        b = budget(maximum=3)
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.read_file('fixed', 3, b, (3, hashlib.sha256(b'abc').hexdigest()), opener)
        self.assertEqual(opener.calls, [])
        self.assertEqual(stream.read_calls, 0)
        self.assertTrue(b.failed)

    def test_malformed_expected_subject_refuses_before_open(self):
        for expected in ((1,), (True, 'a' * 64), (4, 'a' * 64), (3, 'bad')):
            opener, b = FakeOpen(CountedFile(b'abc')), budget()
            with self.assertRaises(ENTRY.Refusal):
                ENTRY.read_file('fixed', 3, b, expected, opener)
            self.assertTrue(b.failed)
            self.assertEqual(opener.calls, [])

    def test_short_reads_and_eof_preserve_exact_byte_hash_subject(self):
        stream = CountedFile(b'abc', chunk=1)
        opener = FakeOpen(stream)
        b = budget()
        with patch.object(ENTRY.os, 'fstat', return_value=types.SimpleNamespace(st_mode=stat.S_IFREG)):
            result = ENTRY.read_file('fixed', 3, b, (3, hashlib.sha256(b'abc').hexdigest()), opener)
        self.assertEqual(result, b'abc')
        self.assertEqual((stream.read_calls, stream.closed_calls, b.used), (4, 1, 4))
        self.assertEqual(opener.calls[0], ('fixed', 'rb', {'buffering': 0}))

    def test_failed_read_charge_is_sticky_before_later_open_decode_or_exec(self):
        stream = CountedFile(b'', broken=True)
        opener = FakeOpen(stream)
        b = budget()
        with patch.object(ENTRY.os, 'fstat', return_value=types.SimpleNamespace(st_mode=stat.S_IFREG)):
            with self.assertRaises(ENTRY.Refusal):
                ENTRY.read_file('fixed', 3, b, (3, hashlib.sha256(b'abc').hexdigest()), opener)
        charged = b.used
        with self.assertRaises(ENTRY.Refusal): ENTRY.read_file('other', 3, b, opener=opener)
        with self.assertRaises(ENTRY.Refusal): ENTRY.strict_json(b'{}', b)
        with self.assertRaises(ENTRY.Refusal): ENTRY.load_pure(b'x', b)
        self.assertEqual(len(opener.calls), 1)
        self.assertEqual((stream.read_calls, b.used), (1, charged))
        self.assertGreater(charged, 0)

    def test_size_hash_and_nonregular_refusal_latch_before_more_io(self):
        for data, expected, mode in ((b'ab', b'abc', stat.S_IFREG),
                                     (b'abd', b'abc', stat.S_IFREG),
                                     (b'abcd', b'abc', stat.S_IFREG),
                                     (b'abc', b'abc', stat.S_IFDIR)):
            stream, b = CountedFile(data), budget()
            opener = FakeOpen(stream)
            with patch.object(ENTRY.os, 'fstat', return_value=types.SimpleNamespace(st_mode=mode)):
                with self.assertRaises(ENTRY.Refusal):
                    ENTRY.read_file('fixed', 3, b, (3, hashlib.sha256(expected).hexdigest()), opener)
            self.assertTrue(b.failed)
            with self.assertRaises(ENTRY.Refusal): ENTRY.read_file('other', 3, b, opener=opener)
            self.assertEqual(len(opener.calls), 1)

    def test_call_budget_and_deadline_refuse_before_stream_effect(self):
        stream = CountedFile(b'abc')
        for b in (budget(calls=0), ENTRY.Budget(0, clock=lambda: 0)):
            with self.assertRaises(ENTRY.Refusal): b.read(stream, 1)
        self.assertEqual(stream.read_calls, 0)

    def test_close_failure_refuses_without_claiming_native_handle_settlement(self):
        stream, b = CountedFile(b'abc', bad_close=True), budget()
        with patch.object(ENTRY.os, 'fstat', return_value=types.SimpleNamespace(st_mode=stat.S_IFREG)):
            with self.assertRaises(ENTRY.Refusal):
                ENTRY.read_file('fixed', 3, b, (3, hashlib.sha256(b'abc').hexdigest()), FakeOpen(stream))
        self.assertTrue(b.failed)
        self.assertFalse(stream.closed)
        stream.bad_close = False
        stream.close()

    def test_project_calls_five_serial_pure_semantics_and_keeps_missing_facts(self):
        value = ENTRY.config_value((HERE / 'producer_config.json').read_bytes(), budget())
        calls = []
        def keys(data, expected, now):
            calls.append(('keys', data, now))
            return {'synthetic-record': {'primary': 'synthetic'}}
        def signatures(data, code, records, eligible, now):
            calls.append(('signature', data, code))
            return [{'state': 'synthetic-only'}]
        def release(data, candidates, now, anchor, snapshot):
            calls.append(('release', data, snapshot))
            return [{'candidateID': c['id']} for c in candidates]
        payloads = {role: ('synthetic:' + role).encode() for role, _, _ in ENTRY.ROLES}
        result = ENTRY.project(request(), value, payloads,
                               dict(keys=keys, signatures=signatures, release=release), budget())
        self.assertEqual([c[0] for c in calls], ['keys'] + ['signature', 'release'] * 5)
        self.assertEqual(result['schema'], ENTRY.OUTPUT_SCHEMA)
        self.assertEqual(len(result['releases']), 5)
        self.assertEqual(sum(len(r['logicalBindings']) for r in result['releases']), 6)
        self.assertEqual(result['missing'], ENTRY.MISSING)
        self.assertNotIn('productOwner', result)  # Only the real Go completion adds ownership.

    def test_pure_refusal_is_sticky_and_stops_later_projection(self):
        value = ENTRY.config_value((HERE / 'producer_config.json').read_bytes(), budget())
        calls, b = [], budget()
        def refuse(*args):
            calls.append('keys')
            raise ValueError('synthetic sensitive data must not be exposed')
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.project(request(), value, {'key-list': b'synthetic'}, {'keys': refuse}, b)
        self.assertEqual(calls, ['keys'])
        self.assertTrue(b.failed)

    def test_output_cap_precedes_exclusive_open_and_error_is_fixed_small(self):
        opener = FakeOpen(CountedFile(b''))
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.write_output('fixed', b'x' * (ENTRY.OUTPUT_MAX + 1), budget(), opener)
        self.assertEqual(opener.calls, [])
        self.assertLessEqual(len(ENTRY.ERROR_BYTES), 2048)
        self.assertEqual(json.loads(ENTRY.ERROR_BYTES)['code'], 'projection_refused')
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.output_bytes({'large': 'x' * ENTRY.OUTPUT_MAX}, budget())

    def test_output_is_canonical_and_never_opens_replace_mode(self):
        stream = CountedFile(b'')
        opener = FakeOpen(stream)
        data = ENTRY.output_bytes({'z': 1, 'a': 2}, budget())
        self.assertEqual(data, b'{"a":2,"z":1}\n')
        with patch.object(ENTRY.os, 'fsync', return_value=None):
            ENTRY.write_output('fixed', data, budget(), opener)
        self.assertEqual(opener.calls, [('fixed', 'xb', {'buffering': 0})])
        self.assertEqual(stream.closed_calls, 1)

    def test_partial_output_failure_does_not_retry_or_open_another_file(self):
        class Partial(CountedFile):
            def __init__(self):
                super().__init__(b'')
                self.writes = 0
            def write(self, data):
                self.writes += 1
                if self.writes == 1:
                    return super().write(data[:2])
                raise OSError('synthetic partial write')
        stream, b = Partial(), budget()
        opener = FakeOpen(stream)
        with self.assertRaises(ENTRY.Refusal):
            ENTRY.write_output('fixed', b'abcdef', b, opener)
        self.assertTrue(b.failed)
        with self.assertRaises(ENTRY.Refusal): ENTRY.write_output('other', b'abc', b, opener)
        self.assertEqual((len(opener.calls), stream.writes), (1, 2))
        self.assertEqual(stream.closed_calls, 1)

    def test_argument_refusal_performs_no_stdin_or_file_reads(self):
        stdin, b = CountedFile(b''), budget()
        with patch('builtins.open', side_effect=AssertionError('no filesystem open')):
            with self.assertRaises(ENTRY.Refusal): ENTRY.run(['wrong'], stdin, b)
        self.assertEqual(stdin.read_calls, 0)

    def test_source_receipt_stays_incomplete_and_pins_only_actual_other_resources(self):
        receipt = json.loads((HERE / 'producer_source_receipt.json').read_bytes())
        self.assertFalse(receipt['complete'])
        self.assertTrue(receipt['missing'])
        self.assertEqual(receipt['protocol'], ENTRY.PROTOCOL)
        self.assertNotIn('profileSHA256', receipt)
        self.assertNotIn('receiptSHA256', receipt)
        for ref in receipt['resourceFiles']:
            self.assertNotEqual(ref['file'], 'producer_source_receipt.json')
            raw = (HERE / ref['file']).read_bytes()
            self.assertEqual((len(raw), hashlib.sha256(raw).hexdigest()), (ref['bytes'], ref['sha256']))


if __name__ == '__main__':
    unittest.main()
