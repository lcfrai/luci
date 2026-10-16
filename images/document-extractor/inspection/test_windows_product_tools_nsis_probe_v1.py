"""Synthetic fixed-size framing source cases only; no selected payload proof."""
import importlib.util
import hashlib
import json
from pathlib import Path
import struct
import time
import unittest
from unittest import mock


def module(name):
    path = Path(__file__).parent / name
    spec = importlib.util.spec_from_file_location(name.replace('.', '_'), path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


worker = module('windows_product_tools_v1.py')
framing = module('windows_product_tools_nsis_framing_v1.py')
probe = module('windows_product_tools_nsis_probe_v1.py')


def fixture(offset=1024, flags=0, certificate=0):
    raw = bytearray(5804864)
    end = certificate or len(raw)
    header = (flags, 0xDEADBEEF, 0x6C6C754E, 0x74666F73, 0x74736E49, 100, end - offset)
    struct.pack_into('<7I', raw, offset, *header)
    struct.pack_into('<I', raw, offset + 28, 0x80000020)
    raw[offset + 32:offset + 37] = b'\x5d\x00\x00\x80\x00'
    pe = {'overlayFileOffset': 1000, 'overlayBytes': end - 1000,
          'imageFileEnd': 1000, 'certificateFileOffset': certificate, 'trailingBytes': 0}
    return raw, pe


def inspect(raw, pe, budget=None):
    return framing.inspect_nsis_framing_v1(bytes(raw), pe, budget or worker.Budget(time.monotonic() + 30))


class NSISProbeSourceTests(unittest.TestCase):
    def test_current_shape_identifies_only_structural_header(self):
        raw, pe = fixture()
        observed = inspect(raw, pe)
        self.assertEqual(observed['firstheaderFileOffset'], 1024)
        self.assertEqual(observed['followingBytesIncludingFirstheaderAndCRC'], len(raw) - 1024)
        self.assertEqual(observed['possibleCRCBytes'], 4)
        for field in ('variantIdentified', 'codecIdentified', 'payloadDecoded', 'authenticated', 'productProfileAvailable', 'crcVerified'):
            self.assertIs(observed[field], False)

    def test_unaligned_or_absent_magic_cannot_select_variant(self):
        for mode in ('unaligned', 'absent'):
            raw, pe = fixture(offset=1025 if mode == 'unaligned' else 1024)
            if mode == 'absent':
                raw[1028:1032] = b'xxxx'
            with self.assertRaisesRegex(worker.Refusal, 'nsis_firstheader_absent'):
                inspect(raw, pe)

    def test_unique_candidate_and_absolute_512_sites(self):
        raw, pe = fixture()
        struct.pack_into('<7I', raw, 1536, 0, 0xDEADBEEF, 0x6C6C754E, 0x74666F73, 0x74736E49, 100, len(raw) - 1536)
        with self.assertRaisesRegex(worker.Refusal, 'nsis_ambiguous_firstheader'):
            inspect(raw, pe)

    def test_following_length_includes_firstheader_and_crc(self):
        for delta in (-28, -4, 28, 0x80000000):
            raw, pe = fixture()
            struct.pack_into('<I', raw, 1024 + 24, len(raw) - 1024 + delta)
            with self.assertRaises(worker.Refusal):
                inspect(raw, pe)
        raw, pe = fixture(flags=4)
        self.assertEqual(inspect(raw, pe)['possibleCRCBytes'], 0)

    def test_unsupported_flags_and_header_lengths_refuse(self):
        for flags in (1, 12, 16, 0xffffffff):
            raw, pe = fixture(flags=flags)
            with self.assertRaisesRegex(worker.Refusal, 'nsis_unsupported_flags'):
                inspect(raw, pe)
        for size in (0, 1048577, 0xffffffff):
            raw, pe = fixture()
            struct.pack_into('<I', raw, 1024 + 20, size)
            with self.assertRaisesRegex(worker.Refusal, 'nsis_declared_length'):
                inspect(raw, pe)

    def test_pe_certificate_and_unclassified_trailing_extents(self):
        raw, pe = fixture(certificate=5804800)
        observed = inspect(raw, pe)
        self.assertEqual(observed['possibleCRCFileOffset'] + 4, pe['certificateFileOffset'])
        for key in ('trailingBytes', 'overlayBytes', 'imageFileEnd'):
            changed = dict(pe)
            changed[key] += 1
            with self.assertRaises(worker.Refusal):
                inspect(raw, changed)

    def test_plausible_lzma_properties_never_identify_codec(self):
        raw, pe = fixture()
        observed = inspect(raw, pe)
        self.assertTrue(observed['possibleLengthHighBit'])
        self.assertIn('5d00008000', observed['prefixHex'])
        self.assertFalse(observed['codecIdentified'])
        self.assertFalse(observed['variantIdentified'])

    def test_sticky_refusal_precedes_further_in_memory_scan(self):
        raw, pe = fixture()
        budget = worker.Budget(time.monotonic() + 30)
        budget.failed = True
        with self.assertRaisesRegex(worker.Refusal, 'sticky_or_deadline'):
            inspect(raw, pe, budget)
        self.assertEqual((budget.original, budget.expanded, budget.calls), (0, 0, 0))

    def test_manifest_pins_actual_framing_bytes_without_payload_claim(self):
        here = Path(__file__).parent
        raw = (here / 'windows_product_tools_nsis_probe_v1_manifest.json').read_bytes()
        manifest = json.loads(raw)
        self.assertEqual(hashlib.sha256(raw).hexdigest(), probe.MANIFEST_SHA256)
        self.assertEqual(manifest['framingSourceSHA256'], hashlib.sha256((here / 'windows_product_tools_nsis_framing_v1.py').read_bytes()).hexdigest())
        self.assertEqual(manifest['input'], probe.WRAPPER)
        self.assertEqual(manifest['limits']['expandedBytes'], 0)
        self.assertEqual(manifest['limits']['observationBytes'], 65536)
        self.assertFalse(any(probe.ACQUISITION_PINS.values()))
        self.assertIn('selected-NSIS-version-and-build-source', manifest['unavailable'])

    def test_unset_acquisition_pins_refuse_before_source_or_wrapper_io(self):
        argv = ['fixed-probe', '--phase', 'probe', '--batch', probe.BATCH,
                '--input', str(probe.INPUT), '--output', str(probe.OUTPUT),
                '--deadline', str(time.monotonic() + 30)]
        with mock.patch.object(probe.sys, 'argv', argv), mock.patch.object(probe, 'bootstrap_library') as bootstrap:
            with self.assertRaisesRegex(ValueError, 'future_acquisition_pins_unset'):
                probe.main()
            bootstrap.assert_not_called()

    def test_foreign_input_refuses_before_source_or_wrapper_io(self):
        argv = ['fixed-probe', '--phase', 'probe', '--batch', probe.BATCH,
                '--input', str(probe.INPUT / 'foreign'), '--output', str(probe.OUTPUT),
                '--deadline', str(time.monotonic() + 30)]
        with mock.patch.object(probe.sys, 'argv', argv), mock.patch.object(probe, 'bootstrap_library') as bootstrap:
            with self.assertRaisesRegex(ValueError, 'fixed_probe_arguments'):
                probe.main()
            bootstrap.assert_not_called()


if __name__ == '__main__':
    unittest.main()
