"""Synthetic in-memory source fixtures only. No payload/tool/native/network calls."""
import hashlib
import importlib.util
import io
from pathlib import Path
import struct
import time
import unittest
import zipfile


def source_module(filename):
    path = Path(__file__).parent / filename
    spec = importlib.util.spec_from_file_location(filename.replace('.', '_'), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


worker = source_module('windows_product_tools_v1.py')
pe = source_module('windows_product_tools_pe_v1.py')
zip_reader = source_module('windows_product_tools_zip_v1.py')


def budget():
    return worker.Budget(time.monotonic() + 30)


class Counted(io.BytesIO):
    def __init__(self, data=b'', fail=False):
        super().__init__(data)
        self.effects, self.fail = [], fail

    def seek(self, *args):
        self.effects.append('seek')
        return super().seek(*args)

    def read(self, size=-1):
        self.effects.append('read')
        if self.fail:
            raise OSError('synthetic read failure')
        return super().read(size)


def original(data, b=None, stream=None):
    # Exercise actual bounded read methods without fstat or native acquisition.
    value = worker.Original.__new__(worker.Original)
    value.stream = stream if stream is not None else Counted(data)
    value.expected = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
    value.budget = b if b is not None else budget()
    return value


def synthetic_pe(machine=0x8664, certificate=False):
    value = bytearray(1040 if certificate else 1024)
    value[:2] = b'MZ'
    struct.pack_into('<I', value, 60, 64)
    value[64:68] = b'PE\0\0'
    struct.pack_into('<HH', value, 68, machine, 1)
    struct.pack_into('<H', value, 84, 240)
    optional = 88
    struct.pack_into('<H', value, optional, 0x20b)
    struct.pack_into('<I', value, optional + 60, 512)
    struct.pack_into('<I', value, optional + 108, 16)
    struct.pack_into('<IIII', value, optional + 240 + 8, 512, 0x1000, 512, 512)
    if certificate:
        # Security directory is a FILE OFFSET, deliberately not section RVA.
        struct.pack_into('<II', value, optional + 112 + 4 * 8, 1024, 16)
    return bytes(value)


def synthetic_zip(extra=()):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_STORED) as archive:
        archive.writestr('python.exe', synthetic_pe())
        archive.writestr('LICENSE.txt', b'synthetic notice, not upstream licence proof')
        archive.writestr('python313.zip', b'opaque nested archive, never decoded')
        for name, data in extra:
            archive.writestr(name, data)
    return stream.getvalue()


class ProductToolSourceTests(unittest.TestCase):
    def test_sticky_read_admission_precedes_seek_and_stream_call(self):
        b = budget()
        b.original = 32 << 20
        value = original(b'x', b)
        with self.assertRaises(worker.Refusal):
            value.exact_at(0, 1)
        self.assertEqual(value.stream.effects, [])
        self.assertTrue(b.failed)
        with self.assertRaises(worker.Refusal):
            value.exact_at(0, 1)
        self.assertEqual(value.stream.effects, [])

    def test_failed_and_nonprogress_reads_keep_full_pending_demand(self):
        for stream in (Counted(b'', fail=True), Counted(b'')):
            b = budget()
            value = original(b'12345678', b, stream)
            with self.assertRaises((OSError, worker.Refusal)):
                value.exact_at(0, 8)
            self.assertEqual(b.original, 8)
            self.assertTrue(b.failed)
            prior = list(stream.effects)
            with self.assertRaises(worker.Refusal):
                value.exact_at(0, 1)
            self.assertEqual(stream.effects, prior)

    def test_short_valid_reads_reconcile_and_exact_call_limit_is_sticky(self):
        class Short(Counted):
            def read(self, size=-1):
                return super().read(min(2, size))
        b = budget()
        value = original(b'12345678', b, Short(b'12345678'))
        self.assertEqual(value.exact_at(0, 8), b'12345678')
        self.assertEqual(b.original, 8)
        self.assertEqual(b.calls, 8)
        b.calls = 8192
        previous = list(value.stream.effects)
        with self.assertRaises(worker.Refusal):
            value.exact_at(0, 1)
        self.assertEqual(value.stream.effects, previous)

    def test_combined_and_expansion_reserve_before_decoder(self):
        b = budget()
        b.original, b.expanded = 32 << 20, (32 << 20) - 1
        ticket = b.reserve_expansion(1)
        b.finish_expansion(ticket, 1)
        self.assertEqual(b.original + b.expanded, 64 << 20)
        with self.assertRaises(worker.Refusal):
            b.reserve_expansion(1)
        self.assertTrue(b.failed)

    def test_pe_distinguishes_file_certificate_offset_from_rva(self):
        observed = pe.inspect_pe_v1(synthetic_pe(certificate=True), budget())
        self.assertEqual(observed['certificateFileOffset'], 1024)
        self.assertEqual(observed['overlayBytes'], 0)
        self.assertFalse(observed['authenticated'])
        self.assertFalse(observed['requiredFunctionExportsProven'])

    def test_pe_refuses_offset_section_machine_and_unbacked_import(self):
        for kind in ('offset', 'sections', 'machine', 'import', 'certificate-rva'):
            raw = bytearray(synthetic_pe())
            if kind == 'offset':
                struct.pack_into('<I', raw, 60, 0xffffffff)
            elif kind == 'sections':
                struct.pack_into('<H', raw, 70, 97)
            elif kind == 'machine':
                struct.pack_into('<H', raw, 68, 0xaa64)
            elif kind == 'import':
                struct.pack_into('<II', raw, 88 + 112 + 8, 0x9000, 20)
            else:
                struct.pack_into('<II', raw, 88 + 112 + 4 * 8, 0x1000, 16)
            with self.assertRaises(worker.Refusal):
                pe.inspect_pe_v1(bytes(raw), budget())

    def test_nonwheel_outer_zip_and_nested_blob_remain_incomplete(self):
        raw = synthetic_zip()
        value = original(raw)
        observed = zip_reader.inspect_cpython_embed_v1(value, value.expected, value.budget, pe.inspect_pe_v1)
        self.assertEqual(observed['memberCount'], 3)
        self.assertFalse(observed['nestedStdlibDecoded'])
        self.assertFalse(observed['authenticated'])
        self.assertEqual(observed['members'][2]['nestedArchive'], 'opaque_not_decoded')

    def test_raw_zip_names_refuse_nul_control_alias_and_local_mismatch(self):
        cases = [synthetic_zip((('badx.dll', synthetic_pe()),)).replace(b'badx.dll', b'bad\0.dll'),
                 synthetic_zip((('badx.dll', synthetic_pe()),)).replace(b'badx.dll', b'bad\n.dll'),
                 synthetic_zip((('PYTHON.EXE', synthetic_pe()),))]
        raw = synthetic_zip()
        cases.append(raw.replace(b'python.exe', b'python.exx', 1))
        for raw in cases:
            value = original(raw)
            with self.assertRaises(worker.Refusal):
                zip_reader.inspect_cpython_embed_v1(value, value.expected, value.budget, pe.inspect_pe_v1)

    def test_zip_trailing_oversized_expansion_and_corrupt_body_refuse(self):
        raw = synthetic_zip()
        cases = [raw + b'x', raw[:-1], raw.replace(b'synthetic notice', b'SYNTHETIC NOTICE', 1)]
        for raw in cases:
            value = original(raw)
            with self.assertRaises(worker.Refusal):
                zip_reader.inspect_cpython_embed_v1(value, value.expected, value.budget, pe.inspect_pe_v1)
        value = original(synthetic_zip())
        value.budget.expanded = 32 << 20
        with self.assertRaises(worker.Refusal):
            zip_reader.inspect_cpython_embed_v1(value, value.expected, value.budget, pe.inspect_pe_v1)

    def test_gpg_container_probe_never_claims_payload_architecture(self):
        raw = synthetic_pe() + b'opaque unsupported NSIS bytes'
        observed = worker.inspect_gnupg_wrapper_v1(raw, {'file': 'synthetic.exe', 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}, budget(), pe.inspect_pe_v1)
        self.assertFalse(observed['nsisPayloadDecoded'])
        self.assertFalse(observed['payloadArchitectureProven'])
        self.assertEqual(observed['opaqueOverlayPrefixBytes'], 29)

    def test_json_allocation_refuses_before_serializer_and_pins_are_exact(self):
        with self.assertRaises(worker.Refusal):
            worker.json_bound({'value': 'x' * 1024}, 100)
        self.assertEqual(sum(v['bytes'] for v in worker.OBJECTS), 17243958)
        self.assertEqual({v['id'] for v in worker.OBJECTS}, {'cpython', 'gnupg'})
        self.assertNotIn('runtimeProfile', worker.OBJECTS[0])

    def test_arbitrary_acquisition_object_refuses_before_connection(self):
        calls = []
        class NeverOutput:
            def __truediv__(self, name):
                calls.append('output')
                raise AssertionError('unselected object reached output')
        def never_connection(*args, **kwargs):
            calls.append('network')
            raise AssertionError('unselected object reached connection')
        with self.assertRaises(worker.Refusal):
            worker.acquire({'url': 'https://example.invalid/payload'}, NeverOutput(), budget(), None, never_connection)
        self.assertEqual(calls, [])

    def test_member_ceiling_refuses_before_any_member_data(self):
        raw = bytearray(synthetic_zip())
        end = len(raw) - 22
        struct.pack_into('<HH', raw, end + 8, 212, 212)
        value = original(bytes(raw))
        with self.assertRaises(worker.Refusal):
            zip_reader.inspect_cpython_embed_v1(value, value.expected, value.budget, pe.inspect_pe_v1)
        self.assertEqual(value.budget.expanded, 0)


if __name__ == '__main__':
    unittest.main()
