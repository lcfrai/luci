"""Synthetic byte fixtures only; not real signed-index/174-package acceptance.

Run later under an explicitly approved owner with --reader-source pointing to
the exact frozen06b source. No implicit path search, downloads or payload imports.
"""
import argparse
import gzip
import hashlib
import io
import json
import lzma
from pathlib import Path
import struct
import time
import unittest

import debian_binary_bindings as b

READERS = None


def identity(data):
    return len(data), hashlib.sha256(data).hexdigest()


def budget():
    return b.Budget(time.monotonic() + 20)


def item():
    return {'name': 'synthetic', 'version': '1:2.0+BinNMU1', 'architecture': 'all',
            'filename': 'pool/main/s/synthetic/synthetic.deb', 'bytes': 42,
            'sha256': 'a' * 64}


def paragraph(p=None):
    p = p or item()
    return ('Package: '+p['name']+'\nVersion: '+p['version']+'\nArchitecture: '+p['architecture']+
            '\nSize: '+str(p['bytes'])+'\nSHA256: '+p['sha256']+'\nFilename: '+p['filename']+'\n').encode()


def tar_entry(name, data, kind=b'0'):
    header = bytearray(512)
    header[:len(name)] = name.encode()
    for start, width, value in ((100, 8, 0o644), (108, 8, 0), (116, 8, 0), (124, 12, len(data)), (136, 12, 0)):
        header[start:start+width] = format(value, 'o').encode().rjust(width-1, b'0') + b'\0'
    header[148:156] = b' ' * 8
    header[156:157] = kind
    header[257:265] = b'ustar\x0000'
    header[148:156] = format(sum(header), 'o').encode().rjust(6, b'0') + b'\0 '
    return bytes(header) + data + bytes((-len(data)) % 512)


def ar_member(name, data):
    header = (name+'/').encode().ljust(16) + b'0'.ljust(12) + b'0'.ljust(6) + b'0'.ljust(6) + b'100644'.ljust(8) + str(len(data)).encode().ljust(10) + b'`\n'
    return header + data + (b'\n' if len(data) % 2 else b'')


def deb(control, extra=b'', data=b'not a compressed frame; must never decode me'):
    tar = tar_entry('control', control) + extra + bytes(1024)
    return b'!<arch>\n' + ar_member('debian-binary', b'2.0\n') + ar_member('control.tar.gz', gzip.compress(tar)) + ar_member('data.tar.xz', data)


class Fragmented(io.BytesIO):
    def read(self, size=-1):
        if size < 0:
            raise AssertionError('unbounded fixture read')
        return super().read(min(size, 7))


class BinaryBindings(unittest.TestCase):
    def test_pinned_reader_source_refuses_arbitrary_code(self):
        with self.assertRaises(b.Refusal):
            b.load_pinned_readers(b'raise AssertionError("executed")')

    def test_physical_records_preserve_values_and_unfold_only_edges(self):
        raw = b'pAcKaGe: A\r\nVersion: 1:2+BinNMU1\r\nDepends: LibA:any (>= 1) |\r\n\t LibB (= 2) \t\r\n\r\n'
        rows = list(b.paragraphs(Fragmented(raw), budget()))
        self.assertEqual(rows[0][2]['version'], '1:2+BinNMU1')
        self.assertEqual(rows[0][2]['depends'], 'LibA:any (>= 1) | LibB (= 2)')
        self.assertEqual(rows[0][1], raw[:-2])
        for hostile in (b'Package: A\npackage: B\n', b' orphan\n', b'Package: A\rB\n', b'X: '+b'a'*8193+b'\n', b'X: \xff\n'):
            with self.assertRaises((b.Refusal, UnicodeError)):
                list(b.paragraphs(io.BytesIO(hostile), budget()))

    def test_packages_complete_eof_and_duplicate_or_foreign_tuple(self):
        expected = item()
        raw = paragraph()+b'\nPackage: unrelated\nVersion: 9\nArchitecture: amd64\n\n'
        packed = lzma.compress(raw)
        result = b.scan_packages(Fragmented(packed), identity(packed), {(expected['name'],expected['version'],expected['architecture']): expected}, budget(), READERS)
        # expected is a name/version/architecture dict, not field-name authority.
        self.assertEqual(len(result['matches']), 1)
        self.assertEqual(result['decodedSHA256'], hashlib.sha256(raw).hexdigest())
        for invalid in (packed[:-1], packed+b'trailing', packed+packed, lzma.compress(paragraph()+b'\n'+paragraph()), lzma.compress(paragraph().replace(b'Size: 42', b'Size: 43'))):
            with self.assertRaises((b.Refusal, READERS.Refusal, EOFError, lzma.LZMAError)):
                b.scan_packages(io.BytesIO(invalid), identity(invalid), {(expected['name'],expected['version'],expected['architecture']): expected}, budget(), READERS)

    def test_control_only_skips_hostile_data_and_rejects_control_links(self):
        raw = b'Package: synthetic\nVersion: 1:2.0+BinNMU1\nArchitecture: all\n'
        archive = deb(raw)
        actual = b.read_deb_control(Fragmented(archive), identity(archive), item(), budget(), READERS)
        self.assertEqual(actual['raw'], raw)
        for hostile in (deb(raw, tar_entry('control', b'')), deb(raw, tar_entry('../escape', b'')), deb(raw, tar_entry('linked', b'', b'2')), archive+b'extra'):
            with self.assertRaises((b.Refusal, READERS.Refusal)):
                b.read_deb_control(io.BytesIO(hostile), identity(hostile), item(), budget(), READERS)

    def test_actual_reads_failures_skips_and_expansion_share_one_budget(self):
        shared = budget()
        class Broken:
            def read(self, size):
                raise OSError('synthetic read failure')
        broken = b.Metered(Broken(), (100, 'a'*64), shared)
        with self.assertRaises(OSError):
            broken.read(17)
        self.assertEqual(shared.compressed, 17)
        self.assertTrue(shared.failed)
        archive = deb(b'Package: synthetic\nVersion: 1:2.0+BinNMU1\nArchitecture: all\n', data=b'x'*100000)
        b.read_deb_control(io.BytesIO(archive), identity(archive), item(), shared, READERS)
        self.assertGreaterEqual(shared.compressed, 2*len(archive)+17)
        self.assertGreater(shared.expanded, 1024)
        shared.index_expanded = 256*b.MIB
        with self.assertRaises(b.Refusal):
            b.Expansion(shared, 128*b.MIB, index=True).charge(1)
        shared.compressed = b.COMPRESSED_LIMIT
        with self.assertRaises(b.Refusal):
            shared.compressed_read(0)

    def test_no_per_part_reset_or_third_original_pass(self):
        shared = budget()
        data = b'abcd'
        for _ in range(2):
            stream = b.Metered(io.BytesIO(data), identity(data), shared)
            b.drain(stream, READERS)
            stream.finish()
        with self.assertRaises(b.Refusal):
            b.Metered(io.BytesIO(data), identity(data), shared)
        shared.retained = 16*b.MIB
        with self.assertRaises(b.Refusal):
            shared.retain(1)
        with self.assertRaises(b.Refusal):
            shared.observation_summary()

    def test_claims_are_case_sensitive_and_source_is_only_a_claim(self):
        control = {'package':'a','version':'1:BinNMU1','architecture':'all','depends':'LibA:any (>= 1)','source':'sourceA (2)'}
        index = dict(control, source='sourceB (3)')
        encoded = b.selected_claims([{'bindingOrdinals':[0,2],'fields':index}], control)
        claims = json.loads(encoded)
        self.assertEqual(claims['indexes'][0]['fields']['source'], 'sourceB (3)')
        index['depends'] = 'liba:any (>= 1)'
        with self.assertRaises(b.Refusal):
            b.selected_claims([{'bindingOrdinals':[0,2],'fields':index}], control)

    def test_bundle_closed_order_full_part_and_retention_limit(self):
        segments = [(n,k,0,b'raw') for n in range(16) for k in (1,2,3)]
        raw, spans = b.frame_bundle(segments, 0, 16, budget())
        self.assertEqual(raw[:len(b.BUNDLE_MAGIC)], b.BUNDLE_MAGIC)
        ordinal,kind,slot,length,digest = b.BUNDLE_HEADER.unpack_from(raw,len(b.BUNDLE_MAGIC))
        self.assertEqual((ordinal,kind,slot,length),(0,1,0,3))
        self.assertEqual(digest,hashlib.sha256(b'raw').digest())
        self.assertEqual(raw[spans[0]['offset']:spans[0]['offset']+3],b'raw')
        for invalid in (segments[:-1], [segments[0]]+segments, segments[::-1]):
            with self.assertRaises(b.Refusal):
                b.frame_bundle(invalid,0,16,budget())


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--reader-source', required=True, type=Path)
    args, remaining = parser.parse_known_args()
    # Read a fixed-size project-source fixture only. No payload imports/native
    # producer/credentials. Actual execution still requires separate release.
    with args.reader_source.open('rb') as source:
        READERS = b.load_pinned_readers(source.read(b.INSPECTOR_BYTES+1))
    unittest.main(argv=[__file__]+remaining)
