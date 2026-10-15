"""Inert regression sources. No network, subprocess, native import or Docker."""
import base64
import csv
import ctypes as C
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import tarfile
import tempfile
import time
import types
import unittest
from unittest.mock import patch
import zipfile


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).parent/file)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


worker = module('inspector_under_test', 'inspect_inputs.py')
controller = module('controller_under_test', 'windows_inspector_controller.py')


def budget():
    return worker.Budget(time.monotonic()+30)


def tar(rows):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for name, content, kind, target in rows:
            info = tarfile.TarInfo(name)
            info.type, info.linkname, info.size = kind, target, len(content)
            archive.addfile(info, io.BytesIO(content))
    return stream.getvalue()


def wheel(path, extra=None, bad_record=False):
    rows = {'example.py': b'pass\n', 'example-1.dist-info/METADATA': b'Name: example\nVersion: 1\n'}
    if extra:
        rows.update(extra)
    output = io.StringIO(newline='');writer = csv.writer(output, lineterminator='\n')
    for name, data in rows.items():
        digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b'=').decode()
        writer.writerow([name, 'sha256='+('wrong' if bad_record else digest), len(data)])
    writer.writerow(['example-1.dist-info/RECORD', '', ''])
    rows['example-1.dist-info/RECORD'] = output.getvalue().encode()
    with zipfile.ZipFile(path, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
        for name, data in rows.items():
            archive.writestr(name, data)


class StaticInspectionTests(unittest.TestCase):
    def test_exact_manifest_batches_no_source_or_generic_batch(self):
        self.assertEqual(len(worker.load_manifest('primary')), 4)
        self.assertEqual(len(worker.load_manifest('debian-controls')), 174)
        self.assertEqual(sum(item['bytes'] for item in worker.load_manifest('wheels')), 10733989)
        self.assertEqual(len(worker.load_manifest('missing-notice')), 1)
        self.assertEqual(len(worker.load_manifest('base-layer')), 1)
        with self.assertRaises(worker.Refusal):
            worker.load_manifest('sources')

    def test_stream_frame_is_complete_and_exclusive(self):
        raw = gzip.compress(b'bounded')
        stream = worker.Decoded(io.BytesIO(raw), 'gz', budget())
        self.assertEqual(stream.read(32), b'bounded')
        self.assertEqual(stream.read(1), b'')
        for bad in (raw[:-1], raw+b'extra', raw+gzip.compress(b'second')):
            with self.subTest(length=len(bad)), self.assertRaises(worker.Refusal):
                worker.Decoded(io.BytesIO(bad), 'gz', budget()).read(32)

    def test_tar_valid_metadata_and_confined_link_are_data(self):
        raw = tar([('usr/share/doc/a/copyright', b'notice', tarfile.REGTYPE, ''),
                   ('usr/share/doc/b/copyright', b'', tarfile.SYMTYPE, '../a/copyright')])
        value = worker.tar_inventory(io.BytesIO(raw), budget(), 'fake')
        alias = value['members'][1]['resolvedMetadata']
        self.assertTrue(alias['available'])
        self.assertEqual(alias['sha256'], hashlib.sha256(b'notice').hexdigest())

    def test_tar_traversal_special_whiteout_and_checksum_refused(self):
        for rows in ([('../escape', b'x', tarfile.REGTYPE, '')],
                     [('device', b'', tarfile.CHRTYPE, '')],
                     [('.wh.hidden', b'', tarfile.REGTYPE, '')],
                     [('alias', b'', tarfile.SYMTYPE, '../../escape')]):
            with self.subTest(rows=rows), self.assertRaises(worker.Refusal):
                worker.tar_inventory(io.BytesIO(tar(rows)), budget(), 'fake')
        raw = bytearray(tar([('safe', b'x', tarfile.REGTYPE, '')]));raw[0] ^= 1
        with self.assertRaises(worker.Refusal):
            worker.tar_inventory(io.BytesIO(raw), budget(), 'fake')

    def test_tar_trailing_payload_and_metadata_budget_refused(self):
        raw = tar([('safe', b'x', tarfile.REGTYPE, '')])
        with self.assertRaises(worker.Refusal):
            worker.tar_inventory(io.BytesIO(raw+b'payload'), budget(), 'fake')
        b = budget();b.retained = worker.MAX_RESULT
        with self.assertRaises(worker.Refusal):
            worker.tar_inventory(io.BytesIO(raw), b, 'fake')

    def test_actual_expansion_brake_precedes_retention(self):
        b = budget();b.expanded = worker.MAX_EXPANDED
        with self.assertRaises(worker.Refusal):
            worker.Decoded(io.BytesIO(gzip.compress(b'x')), 'gz', b).read(1)

    def test_wheel_record_hashes_and_local_extent(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp)/'test.whl';wheel(path)
            value = worker.zip_inventory(path, budget())
            self.assertEqual(len(value['members']), 3)
            self.assertFalse(value['compiledFeaturesVerified'])
            wheel(path, bad_record=True)
            with self.assertRaises(worker.Refusal):
                worker.zip_inventory(path, budget())
            wheel(path);raw = bytearray(path.read_bytes());raw[8:10] = struct.pack('<H', 0);path.write_bytes(raw)
            with self.assertRaises(worker.Refusal):
                worker.zip_inventory(path, budget())

    def test_zip_traversal_case_collision_trailing_and_elf_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp)/'test.whl'
            for extra in ({'../escape': b'x'}, {'EXAMPLE.py': b'x'}, {'native.so': b'\x7fELFwrong'}):
                wheel(path, extra)
                with self.subTest(extra=extra), self.assertRaises(worker.Refusal):
                    worker.zip_inventory(path, budget())
            wheel(path);path.write_bytes(path.read_bytes()+b'extra')
            with self.assertRaises(worker.Refusal):
                worker.zip_inventory(path, budget())

    def test_ar_incomplete_and_unsupported_members_refused(self):
        for raw in (b'!<arch>\n', b'!<arch>\nshort', b'not ar'):
            with self.subTest(raw=raw), self.assertRaises(worker.Refusal):
                worker.deb_inventory(io.BytesIO(raw), budget())

    def test_elf_hostile_segment_extent_refused_without_loading(self):
        data = bytearray(64)
        data[:7] = b'\x7fELF\x02\x01\x01'
        struct.pack_into('<HHIQQQIHHHHHH', data, 16, 3, 62, 1, 0, (1<<64)-1, 0, 0, 64, 56, 1, 0, 0, 0)
        with self.assertRaises(worker.Refusal):
            worker.elf(data)

    def test_control_duplicate_and_orphan_continuation_refused(self):
        for raw in (b'Package: x\nPackage: y\n', b' continuation\n'):
            with self.assertRaises(worker.Refusal):
                worker.control_records(raw)

    def test_json_duplicate_and_nonfinite_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp)/'input.json'
            for raw in ('{"a":1,"a":2}', '{"x":NaN}'):
                path.write_text(raw)
                with self.assertRaises(worker.Refusal):
                    worker.json_read(path)

    def test_input_identity_refuses_before_archive_parse(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp)/'input';path.write_bytes(b'foreign')
            with patch.object(worker, 'zip_inventory', side_effect=AssertionError('no parser admission')):
                with self.assertRaises(worker.Refusal):
                    worker.inspect({'bytes': 7, 'sha256': '0'*64, 'kind': 'wheel'}, path, budget())

    def test_prior_cleanup_unknown_or_changed_receipt_never_admits(self):
        with tempfile.TemporaryDirectory() as temp:
            source = Path(temp)
            prior = {'state': 'all_transfers_exactly_verified', 'batch': 'primary', 'manifestSha256': worker.MANIFEST_SHA256}
            path = source/'worker-result.json';path.write_text(json.dumps(prior))
            settled = {'state': 'completed_owned_worker_retired', 'phase': 'acquire', 'batch': 'primary', 'manifestSha256': worker.MANIFEST_SHA256, 'treeSettled': True, 'pdhClosed': True, 'workerResultSha256': worker.sha(path.read_bytes())}
            target = source/'controller-receipt.json';target.write_text(json.dumps(settled))
            worker.prior_acquisition(source, 'primary')
            settled['treeSettled'] = False;target.write_text(json.dumps(settled))
            with self.assertRaises(worker.Refusal):
                worker.prior_acquisition(source, 'primary')
            settled['treeSettled'] = True;target.write_text(json.dumps(settled));path.write_text(json.dumps(dict(prior, extra='changed')))
            with self.assertRaises(worker.Refusal):
                worker.prior_acquisition(source, 'primary')


class FakeHeaders:
    def __init__(self, values):
        self.values = values

    def get_all(self, key, default):
        return self.values.get(key, default)


class FakeResponse:
    def __init__(self, payload, status=200, lengths=None):
        self.status, self.fp, self.closed = status, io.BytesIO(payload), False
        self.headers = FakeHeaders({'Content-Length': lengths or [str(len(payload))]})

    def getheader(self, key):
        return None

    def close(self):
        self.closed = True


class TransferTests(unittest.TestCase):
    def test_header_failure_closes_exact_reader_before_response_handoff(self):
        raw = io.BytesIO(b'HTTP/1.1 200 OK\r\nX-Long: '+b'x'*65536+b'\r\n\r\n')
        response = worker.BoundedHTTPResponse(types.SimpleNamespace(makefile=lambda *a: raw))
        with self.assertRaises(worker.Refusal):
            response.begin()
        self.assertTrue(raw.closed)

    def test_exact_body_and_trailing_foreign_data(self):
        for payload, success in ((b'exact', True), (b'exactextra', False), (b'exac', False)):
            with tempfile.TemporaryDirectory() as temp:
                response = FakeResponse(payload, lengths=['5']);calls = []
                class Connection:
                    def __init__(self, host, **kwargs):
                        calls.append(('host', host))
                    def request(self, method, path, headers):
                        calls.append(('headers', headers))
                    def getresponse(self):
                        return response
                    def close(self):
                        calls.append(('closed', True))
                item = {'file': 'input.whl', 'url': 'https://files.pythonhosted.org/exact', 'bytes': 5, 'sha256': hashlib.sha256(b'exact').hexdigest()}
                with patch.dict('os.environ', {'HTTPS_PROXY': 'http://must-not-connect.invalid', 'HTTP_PROXY': 'http://must-not-connect.invalid'}):
                    if success:
                        worker.acquire(item, Path(temp), budget(), Connection)
                        self.assertEqual((Path(temp)/item['file']).read_bytes(), b'exact')
                    else:
                        with self.assertRaises(worker.Refusal):
                            worker.acquire(item, Path(temp), budget(), Connection)
                self.assertTrue(response.closed)
                self.assertIn(('host', 'files.pythonhosted.org'), calls)
                self.assertNotIn('Authorization', dict(calls)['headers'])

    def test_redirect_challenge_and_duplicate_lengths_never_publish(self):
        for status, lengths in ((302, ['1']), (401, ['1']), (200, ['1', '1'])):
            response = FakeResponse(b'x', status, lengths)
            connection = types.SimpleNamespace(request=lambda *a, **k: None, getresponse=lambda: response, close=lambda: None)
            with tempfile.TemporaryDirectory() as temp:
                with self.assertRaises(worker.Refusal):
                    worker.acquire({'file': 'x', 'url': 'https://files.pythonhosted.org/x', 'bytes': 1, 'sha256': worker.sha(b'x')}, Path(temp), budget(), lambda *a, **k: connection)
                self.assertFalse((Path(temp)/'x').exists())


class ControllerOrderingTests(unittest.TestCase):
    def atomic_fixture(self, reject_job_list=False):
        calls, state = [], {'created': False, 'stopped': False, 'nextHandle': 301}
        class Basic(C.Structure):
            _fields_ = [('LimitFlags', C.c_uint32), ('ActiveProcessLimit', C.c_uint32)]
        class Limits(C.Structure):
            _fields_ = [('BasicLimitInformation', Basic), ('ProcessMemoryLimit', C.c_size_t), ('JobMemoryLimit', C.c_size_t)]
        class Security(C.Structure):
            _fields_ = [('nLength', C.c_uint32), ('lpSecurityDescriptor', C.c_void_p), ('bInheritHandle', C.c_int)]
        class Process(C.Structure):
            _fields_ = [('hProcess', C.c_void_p), ('hThread', C.c_void_p), ('dwProcessId', C.c_uint32)]
        class Startup(C.Structure):
            _fields_ = [('cb', C.c_uint32), ('dwFlags', C.c_uint32), ('hStdInput', C.c_void_p), ('hStdOutput', C.c_void_p), ('hStdError', C.c_void_p)]
        class StartupEx(C.Structure):
            _fields_ = [('StartupInfo', Startup), ('lpAttributeList', C.c_void_p)]
        m = types.SimpleNamespace(EXTENDED_LIMIT=Limits, SECURITY=Security, PROCESS=Process, STARTUP_EX=StartupEx, S=C.c_size_t,
                                  LIMIT_FLAGS=0x2308, MAX_PROCESSES=4, PROCESS_MEMORY=1536<<20, JOB_MEMORY=2<<30, ACCOUNTING=object)
        def initialize(buffer, count, flags, size):
            self.assertEqual(count, 2);size._obj.value = 64;return True
        def update(attrs, flags, key, values, size, previous, returned):
            calls.append(key)
            if key == 0x2000D:
                self.assertEqual(values[0], 101)
                return not reject_job_list
            self.assertEqual(key, 0x20002)
            return True
        def create(*args):
            self.assertIn(0x2000D, calls)
            self.assertTrue(args[5] & 4)  # suspended, no Python execution
            calls.append('create');state['created'] = True
            args[-1]._obj.hProcess, args[-1]._obj.hThread, args[-1]._obj.dwProcessId = 201, 202, 123
            return True
        def terminate(*args):
            calls.append('stop');state['stopped'] = True;return True
        def private_handle(*args):
            state['nextHandle'] += 1
            return state['nextHandle']
        def check(ok, label):
            if not ok:
                raise ValueError(label)
        def membership(process, job, result):
            result._obj.value = True;return True
        kernel = types.SimpleNamespace(CreateJobObjectW=lambda *a: 101,
            GetHandleInformation=lambda handle, flags: True,
            SetInformationJobObject=lambda *a: True, CreateFileW=private_handle,
            InitializeProcThreadAttributeList=initialize, UpdateProcThreadAttribute=update,
            CreateProcessW=create, IsProcessInJob=membership, TerminateJobObject=terminate,
            WaitForSingleObject=lambda *a: 0, DeleteProcThreadAttributeList=lambda *a: calls.append('delete-attrs'),
            CloseHandle=lambda *a: True)
        win = types.SimpleNamespace(k=kernel, check=check, limits=lambda *a: None,
            members=lambda *a: [123] if state['created'] and not state['stopped'] else [],
            query=lambda *a: types.SimpleNamespace(ActiveProcesses=0))
        return m, win, calls

    def test_atomic_assignment_and_attribute_value_lifetime(self):
        m, win, calls = self.atomic_fixture();owned = []
        auth = types.SimpleNamespace(OwnedJob=types.SimpleNamespace(resume=lambda self: (_ for _ in ()).throw(AssertionError('not resumed'))))
        owner = controller.AtomicOwnedJob(m, auth, win, Path('.'), ['fixed-python'], {}, lambda: False, owned.append)
        self.assertEqual(calls[:3], [0x20002, 0x2000D, 'create'])
        self.assertIs(owned[0], owner)
        self.assertIsNotNone(owner.inherit_values)
        self.assertIsNotNone(owner.job_values)
        owner.close()
        self.assertIsNone(owner.inherit_values)
        self.assertLess(calls.index('stop'), calls.index('delete-attrs'))

    def test_unsupported_atomic_attribute_never_creates_process(self):
        m, win, calls = self.atomic_fixture(True);owned = []
        with self.assertRaises(ValueError):
            controller.AtomicOwnedJob(m, None, win, Path('.'), ['fixed-python'], {}, lambda: False, owned.append)
        self.assertNotIn('create', calls)
        self.assertTrue(owned[0].close()['jobEmpty'])

    def test_pressure_and_cpu_unknown_request_stop_before_io(self):
        for host, cpu in ((lambda: {'availableBytes': 0, 'commitAvailableBytes': 8<<30}, lambda: None),
                          (lambda: {'availableBytes': 8<<30, 'commitAvailableBytes': 8<<30}, lambda: (_ for _ in ()).throw(ValueError('unknown')))):
            calls = []
            brake = controller.Brake(time.monotonic()+30, host, types.SimpleNamespace(snapshot=cpu))
            brake.job = types.SimpleNamespace(kill_now=lambda: calls.append('stop'))
            self.assertTrue(brake.check())
            self.assertEqual(calls, ['stop'])

    def test_deadline_precedes_host_or_wait_or_disk_reads(self):
        calls = []
        brake = controller.Brake(time.monotonic()-1, lambda: (_ for _ in ()).throw(AssertionError('host after deadline')), None)
        job = types.SimpleNamespace(kill_now=lambda: calls.append('stop'))
        brake.job = job
        with self.assertRaises(ValueError):
            controller.supervise(job, brake, None, None, time.monotonic()+100)
        self.assertTrue(calls)

    def test_independent_deadline_does_not_call_blocked_observers(self):
        calls = []
        brake = controller.Brake(time.monotonic()-1, lambda: (_ for _ in ()).throw(AssertionError('observer must not run')), None)
        brake.job = types.SimpleNamespace(kill_now=lambda: calls.append('stop'))
        brake.deadline()
        self.assertEqual(calls, ['stop'])

    def test_job_retirement_precedes_inventory_and_handle_checks(self):
        calls = []
        job = object.__new__(controller.AtomicOwnedJob)
        job.closed, job.job, job.initialized, job.attrs = False, 1, False, None
        job.process = types.SimpleNamespace(hThread=2, hProcess=3)
        job.handles = [4, 5, 6]
        job.m = types.SimpleNamespace(ACCOUNTING=object)
        kernel = types.SimpleNamespace(TerminateJobObject=lambda *a: calls.append('stop') or True,
            WaitForSingleObject=lambda *a: calls.append('wait') or 0,
            CloseHandle=lambda h: calls.append('close') or True)
        job.w = types.SimpleNamespace(k=kernel, check=lambda ok, label: None,
            members=lambda handle: calls.append('inventory') or [],
            query=lambda *a: types.SimpleNamespace(ActiveProcesses=0))
        result = job.close()
        self.assertEqual(calls[0], 'stop')
        self.assertTrue(result['jobEmpty'] and result['terminal'] and result['closedHandlesAcknowledged'])
        with self.assertRaises(ValueError):
            job.close()


if __name__ == '__main__':
    unittest.main()
