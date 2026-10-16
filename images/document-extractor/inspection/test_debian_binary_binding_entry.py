"""Synthetic source fixtures; no native factory, acquisition or positive run mint.

Future permitted tests import project modules only. Fake metadata sizes are NOT
real payloads, hashes, original registry ownership or a full174package journey.
"""
import hashlib
import importlib.util
import io
from pathlib import Path
import struct
import unittest


def module(name):
    path = Path(__file__).parent / name
    spec = importlib.util.spec_from_file_location(name.replace('.', '_'), path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


entry = module('debian_binary_binding_entry.py')
reader = module('debian_binary_bindings.py')  # Definitions only, no06bsource load.


class FakeRaw:
    def __init__(self, native, ordinal, size):
        self.native, self.ordinal, self.position = native, ordinal, size  # EOF
        self.data, self.closed, self.bad_seek = b'xyz', False, None
        self.fail_read = self.fail_close = False

    def seek(self, offset, whence=0):
        self.native.events.append(('seek', self.ordinal, offset, whence))
        if self.native.expire_on_seek == self.ordinal:
            self.native.now = self.native.deadline
        if self.bad_seek is not None:
            return self.bad_seek
        self.position = offset
        return offset

    def read(self, size):
        self.native.events.append(('read', self.ordinal, size))
        if self.fail_read:
            raise OSError('synthetic failed original read')
        out = self.data[self.position:self.position + size]
        self.position += len(out)
        return out

    def close(self):
        self.native.events.append(('close', self.ordinal))
        if self.fail_close:
            raise OSError('synthetic unknown original close')
        self.closed = True


class FakeNative:
    def __init__(self):
        self.now, self.deadline, self.events = 100, 200100, []
        self.expire_on_seek = None
        self.fail_convert, self.flags_value = None, 1
        self.handles = tuple(range(1000, 1181))
        self.infos, self.raws = {}, {}
        quotient, remainder = divmod(144257468, 174)
        sizes = [quotient + int(i < remainder) for i in range(174)] + [2372358] * 4 + [1, entry.READER[0], entry.INSPECTOR[0]]
        for ordinal, (handle, size) in enumerate(zip(self.handles, sizes)):
            self.infos[handle] = {'fileAttributes': 128, 'volumeSerial': 7,
                                  'fileIndex': ordinal + 1, 'bytes': size,
                                  'creationFileTime': 1000, 'lastWriteFileTime': 2000, 'links': 1}
            self.raws[handle] = FakeRaw(self, ordinal, size)

    def tick(self):
        self.events.append(('clock',))
        return self.now

    def file_type(self, handle):
        self.events.append(('type', handle))
        return 1

    def flags(self, handle):
        self.events.append(('flags', handle))
        return self.flags_value

    def info(self, handle):
        self.events.append(('info', handle))
        return dict(self.infos[handle])

    def convert_fd(self, handle):
        self.events.append(('convert', handle))
        if handle == self.fail_convert:
            raise OSError('synthetic conversion failed')
        return handle

    def stream(self, fd):
        return self.raws[fd]

    def close_fd(self, fd):
        self.raws[fd].close()


def command(native):
    info = dict(native.infos[native.handles[178]], fileType=1)
    return {'handles': native.handles, 'deadlineTick': native.deadline,
            'selectionIdentity': info, 'selectionBytes': 1,
            '--operation-id': '1' * 32, '--loan-id': '2' * 32,
            '--source-binding-sha256': '3' * 64, '--selection-sha256': '4' * 64}


def prepared():
    native = FakeNative()
    attempt = entry.EntryAttempt(command(native), native)
    attempt.prepare()
    return native, attempt


def argv(native):
    value = command(native)
    identity = ','.join(str(value['selectionIdentity'][key]) for key in entry.IDENTITY_KEYS)
    values = (entry.ABI, '1' * 32, '2' * 32, '3' * 64, '4' * 64,
              '1', identity, str(native.deadline), ','.join(format(h, 'x') for h in native.handles))
    return [field for pair in zip(entry.FLAGS, values) for field in pair]


class BinaryEntrySourceTests(unittest.TestCase):
    def test_fixed_argv_layout_rejects_foreign_duplicate_and_unbounded_values(self):
        native = FakeNative()
        self.assertEqual(entry.parse_argv(argv(native))['handles'], native.handles)
        for kind in ('zero', 'invalid', 'duplicate', 'unknown', 'extra', 'identity-flags'):
            values = argv(native)
            if kind in ('zero', 'invalid', 'duplicate'):
                handles = values[-1].split(',')
                handles[0] = '0' if kind == 'zero' else 'ffffffffffffffff' if kind == 'invalid' else handles[1]
                values[-1] = ','.join(handles)
            elif kind == 'unknown':
                values[0] = '--arbitrary-path'
            elif kind == 'extra':
                values += ['--handles', values[-1]]
            else:
                values[13] += ',1'  # parent shared identity has NO handle flags
            with self.assertRaises(entry.Refusal):
                entry.parse_argv(values)

    def test_all181_eof_cursors_seek_before_first_input_read(self):
        native, attempt = prepared()
        self.assertFalse(any(v[0] == 'read' for v in native.events))
        self.assertEqual([v[1] for v in native.events if v[0] == 'seek'], list(range(181)))
        self.assertEqual((attempt.seeks, attempt.calls, attempt.compressed), (181, 181, 181))
        self.assertEqual(attempt.classes['index'][:2], [4, 4])
        self.assertEqual(attempt.classes['small'][:2], [3, 3])
        attempt.slots[178].read(1)
        first = next(i for i, v in enumerate(native.events) if v[0] == 'read')
        self.assertEqual(sum(v[0] == 'seek' for v in native.events[:first]), 181)

    def test_initial_seek_failure_forbids_all_input_reads_and_later_seeks(self):
        for failed in (0, 174, 178, 180):
            native = FakeNative()
            native.raws[native.handles[failed]].bad_seek = 7
            attempt = entry.EntryAttempt(command(native), native)
            with self.assertRaises(entry.Refusal):
                attempt.prepare()
            self.assertTrue(attempt.failed)
            self.assertFalse(any(v[0] == 'read' for v in native.events))
            self.assertEqual(sum(v[0] == 'seek' for v in native.events), failed + 1)
            self.assertEqual((attempt.seeks, attempt.calls), (failed + 1, failed + 1))

    def test_clock_expiry_inside_initial_seek_keeps_debit_and_stops(self):
        native = FakeNative()
        native.expire_on_seek = 174
        attempt = entry.EntryAttempt(command(native), native)
        with self.assertRaises(entry.Refusal):
            attempt.prepare()
        self.assertEqual((attempt.calls, attempt.compressed, attempt.seeks), (175, 175, 175))
        self.assertEqual(attempt.classes['index'][:2], [1, 1])
        self.assertFalse(any(v[0] == 'read' for v in native.events))

    def test_child_local_flags_do_not_enter_parent_shared_identity(self):
        native, attempt = prepared()
        self.assertNotIn('handleFlags', attempt.slots[0].initial)
        self.assertEqual(attempt.slots[178].initial, command(native)['selectionIdentity'])
        for flags in (0, 3):
            native = FakeNative()
            native.flags_value = flags
            attempt = entry.EntryAttempt(command(native), native)
            with self.assertRaises(entry.Refusal):
                attempt.prepare()
            self.assertFalse(any(v[0] in ('seek', 'read') for v in native.events))

    def test_metadata_and_clock_caps_refuse_before_next_fake_call(self):
        native = FakeNative()
        attempt = entry.EntryAttempt(command(native), native)
        attempt.metadata = 1086
        before = sum(v[0] == 'info' for v in native.events)
        with self.assertRaises(entry.Refusal):
            attempt.metadata_call(native.info, native.handles[0])
        self.assertEqual(sum(v[0] == 'info' for v in native.events), before)
        native = FakeNative()
        attempt = entry.EntryAttempt(command(native), native)
        attempt.clocks = 1000000
        before = len(native.events)
        with self.assertRaises(entry.Refusal):
            attempt.check()
        self.assertEqual(len(native.events), before)

    def test_bootstrap_hash_and_source_class_failure_are_sticky(self):
        native, attempt = prepared()
        slot = attempt.slots[178]
        slot.initial['bytes'] = 3  # lower fake edge; no positive selection proof
        with self.assertRaises(entry.Refusal):
            entry.read_bootstrap(slot, (3, 'a' * 64))
        before = list(native.events)
        with self.assertRaises(entry.Refusal):
            slot.read(1)
        self.assertEqual(native.events, before)
        native, attempt = prepared()
        attempt.classes['small'][1] = 16
        before = sum(v[0] == 'read' for v in native.events)
        with self.assertRaises(entry.Refusal):
            attempt.slots[178].read(1)
        self.assertEqual(sum(v[0] == 'read' for v in native.events), before)

    def test_measured_prelude_seeds_same179_budget_once_without_reset(self):
        native, attempt = prepared()
        for ordinal in (178, 179, 180):
            slot = attempt.slots[ordinal]
            slot.initial['bytes'] = 3
            self.assertEqual(entry.read_bootstrap(slot, (3, hashlib.sha256(b'xyz').hexdigest())), b'xyz')
        measured = attempt.calls, attempt.compressed
        value = attempt.seed(reader)
        self.assertEqual((value.calls, value.compressed), measured)
        self.assertEqual(measured, (187, 193))
        with self.assertRaises(entry.Refusal):
            attempt.seed(reader)
        self.assertTrue(value.failed)

    def test_179_rewind_checks_offset_and_forbids_third_seek(self):
        native, attempt = prepared()
        value = attempt.seed(reader)  # lower counter fixture, not real bootstrap
        slot = attempt.slots[174]
        slot.initial['bytes'] = 3
        slot.expected = (3, hashlib.sha256(b'xyz').hexdigest())
        second = reader.verified_pass(slot, slot.expected, value, 'index')
        self.assertTrue(slot.interpass)
        self.assertEqual(second.read(65536), b'xyz')
        self.assertEqual(second.read(65536), b'')
        second.finish()
        calls = list(native.events)
        clocks = attempt.clocks

        def debits():
            return (attempt.compressed, attempt.calls, attempt.seeks, attempt.metadata,
                    value.compressed, value.calls, value.expanded, value.paragraphs,
                    {key: tuple(row) for key, row in attempt.classes.items()})

        before = debits()
        with self.assertRaises(entry.Refusal):
            slot.seek(0)
        self.assertEqual(native.events, calls + [('clock',)])
        self.assertEqual(attempt.clocks, clocks + 1)
        self.assertEqual(debits(), before)
        self.assertTrue(attempt.failed)
        self.assertTrue(value.failed)
        calls = list(native.events)
        with self.assertRaises(entry.Refusal):
            slot.seek(0)
        with self.assertRaises(entry.Refusal):
            slot.read(1)
        self.assertEqual(native.events, calls)
        self.assertEqual(attempt.clocks, clocks + 1)
        self.assertEqual(debits(), before)

    def test_interpass_wrong_offset_and_failed_eof_keep_native_charge(self):
        native, attempt = prepared()
        value = attempt.seed(reader)
        slot = attempt.slots[174]
        slot.initial['bytes'] = 3
        slot.expected = (3, hashlib.sha256(b'xyz').hexdigest())
        slot.raw.bad_seek = 9
        with self.assertRaises(entry.Refusal):
            reader.verified_pass(slot, slot.expected, value, 'index')
        self.assertTrue(value.failed)
        self.assertEqual(attempt.classes['index'][0], 9)  # initial4+data3+EOF1+rewind1
        native, attempt = prepared()
        value = attempt.seed(reader)
        slot = attempt.slots[174]
        slot.initial['bytes'] = slot.offset = 3
        slot.raw.fail_read = True
        metered = reader.Metered(slot, (3, hashlib.sha256(b'xyz').hexdigest()), value)
        with self.assertRaises(OSError):
            metered.read(65536)
        self.assertEqual(native.events[-1], ('read', 174, 1))
        self.assertEqual(attempt.classes['index'][0], 5)
        self.assertEqual(value.compressed, 181 + 65536)

    def test_partial_conversion_and_failed_close_retain_exact_backing(self):
        native = FakeNative()
        native.fail_convert = native.handles[3]
        attempt = entry.EntryAttempt(command(native), native)
        with self.assertRaises(OSError):
            attempt.prepare()
        self.assertFalse(attempt.close())
        self.assertEqual(len(attempt.slots), 4)
        self.assertIsNone(attempt.slots[3].fd)
        self.assertTrue(all(v.closed for v in attempt.slots[:3]))
        native, attempt = prepared()
        attempt.slots[2].raw.fail_close = True
        self.assertFalse(attempt.close())
        self.assertIsNotNone(attempt.slots[2].raw)
        self.assertFalse(attempt.slots[2].closed)

    def test_closed_selection_preflight_refuses_before_decoder_allocation(self):
        for raw in (b'[' * 17 + b']' * 17, b'[' + b'0,' * 8193 + b'0]', b'{"a":1,"a":2}', b'{} trailing', b'{"a":1.2}'):
            with self.assertRaises((entry.Refusal, ValueError)):
                entry.decode_selection(raw)
        self.assertEqual(entry.decode_selection(b'{"case":"X (>= 1A)"}'), {'case': 'X (>= 1A)'})

    def test_output_json_estimator_counts_del_before_serialization(self):
        with self.assertRaises(entry.Refusal):
            entry.json_bound('\x7f' * 32, 100)
        with self.assertRaises(entry.Refusal):
            entry.encode_json({'value': 'x' * 65536})

    def test_wire_has_exact23_ordered_frames_and_no_complete_on_error(self):
        native = FakeNative()
        attempt = entry.EntryAttempt(command(native), native)
        stream = io.BytesIO()
        output = entry.OutputFrames(stream, attempt)
        for part in range(11):
            output.emit(1, part, b'LUCI-DBB1\nraw')
            output.emit(2, part, b'{}')
        output.emit(3, 255, b'{}')
        self.assertEqual(output.ordinal, 23)
        raw, offset = stream.getvalue(), len(entry.WIRE_MAGIC)
        self.assertTrue(raw.startswith(entry.WIRE_MAGIC))
        for frame in output.refs:
            kind, part, size, digest = entry.WIRE_HEADER.unpack_from(raw, offset)
            offset += entry.WIRE_HEADER.size
            body = raw[offset:offset + size]
            offset += size
            self.assertEqual((kind, part, size, digest.hex()), (frame['kind'], frame['part'], frame['bytes'], frame['sha256']))
            self.assertEqual(hashlib.sha256(body).digest(), digest)
        self.assertEqual(offset, len(raw))
        before = stream.getvalue()
        with self.assertRaises(entry.Refusal):
            output.emit(3, 255, b'{}')
        self.assertEqual(stream.getvalue(), before)

    def test_output_admission_and_partial_ack_are_sticky_before_next_write(self):
        class Bad:
            calls = 0
            def write(self, _):
                self.calls += 1
                return 0
        native = FakeNative()
        attempt = entry.EntryAttempt(command(native), native)
        bad = Bad()
        output = entry.OutputFrames(bad, attempt)
        with self.assertRaises(entry.Refusal):
            output.emit(1, 0, b'abc')
        self.assertEqual((bad.calls, output.ordinal), (1, 0))
        with self.assertRaises(entry.Refusal):
            output.emit(1, 0, b'abc')
        self.assertEqual(bad.calls, 1)
        native = FakeNative()
        attempt = entry.EntryAttempt(command(native), native)
        output = entry.OutputFrames(io.BytesIO(), attempt)
        output.bytes = 20 << 20
        with self.assertRaises(entry.Refusal):
            output.emit(1, 0, b'abc')
        self.assertEqual(output.stream.getvalue(), b'')

    def test_absolute_owner_deadline_does_not_restart_after_bootstrap(self):
        native, attempt = prepared()
        native.now = native.deadline - 1000
        value = attempt.seed(reader)
        native.now += 1000
        before = len(native.events)
        with self.assertRaises(entry.Refusal):
            value.check()
        self.assertEqual(len(native.events), before + 1)
        self.assertTrue(attempt.failed)


if __name__ == '__main__':
    unittest.main()
