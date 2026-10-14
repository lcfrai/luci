"""Inert HTTP/process fixtures: no Docker CLI, daemon, network or credentials."""
import io
import unittest
from unittest.mock import patch

import archive
import engine_info


def response(value=None, headers=None, status=b'200 OK', body=None):
    body = archive.encoded(value or {'ID': 'engine-original', 'OSType': 'linux', 'OperatingSystem': 'Docker Desktop'}) if body is None else body
    rows = [('Content-Type', 'application/json'), ('Content-Length', str(len(body)))] if headers is None else headers
    return b'HTTP/1.1 ' + status + b'\r\n' + b''.join((k + ': ' + v + '\r\n').encode() for k, v in rows) + b'\r\n' + body


class Process:
    def __init__(self, raw, code=0, stderr=b''):
        self.stdin = engine_info._HeldBytes()
        self.stdout, self.stderr = io.BytesIO(raw), io.BytesIO(stderr)
        self.returncode = code
        self.terminated, self.killed = False, False
    def poll(self): return self.returncode
    def terminate(self): self.terminated = True; self.returncode = -15
    def kill(self): self.killed = True; self.returncode = -9
    def wait(self, timeout): return self.returncode


class EngineInfoTests(unittest.TestCase):
    def test_direct_transport_retains_context_environment_and_only_sends_fixed_get(self):
        process = Process(response())
        with patch.object(engine_info.subprocess, 'Popen', return_value=process) as start:
            value = engine_info.read('engine-original')
        self.assertEqual(value['ID'], 'engine-original')
        self.assertEqual(start.call_args.args[0], ['docker', 'system', 'dial-stdio'])
        self.assertNotIn('env', start.call_args.kwargs)
        self.assertNotIn('shell', start.call_args.kwargs)
        self.assertEqual(process.stdin.getvalue(), engine_info.REQUEST)
        self.assertNotIn(b'Authorization', process.stdin.getvalue())
        self.assertFalse(process.terminated)

    def test_chunked_selected_engine_reply_is_bounded_and_preserves_id_fence(self):
        body = archive.encoded({'ID': 'engine-original', 'OSType': 'linux', 'OperatingSystem': 'Docker Desktop'})
        chunks = (hex(len(body))[2:] + '\r\n').encode() + body + b'\r\n0\r\n\r\n'
        raw = response(headers=[('Content-Type', 'application/json'), ('Transfer-Encoding', 'chunked')], body=chunks)
        self.assertEqual(engine_info.decode(raw, 'engine-original')['ID'], 'engine-original')
        with self.assertRaisesRegex(ValueError, 'identity changed'):
            engine_info.decode(raw, 'replacement')

    def test_redirects_duplicate_framing_wrong_content_and_trailing_bytes_refuse(self):
        body = archive.encoded({'ID': 'engine-original', 'OSType': 'linux', 'OperatingSystem': 'Docker Desktop'})
        variants = [response(status=b'302 Found'),
            response(headers=[('Content-Type', 'application/json'), ('Content-Length', str(len(body))), ('Location', 'http://foreign/')]),
            response(headers=[('Content-Type', 'application/json'), ('Content-Length', str(len(body))), ('Content-Length', str(len(body)))]),
            response(headers=[('Content-Type', 'application/json'), ('Content-Length', str(len(body))), ('Transfer-Encoding', 'chunked')]),
            response(headers=[('Content-Type', 'text/plain')]), response() + b'extra',
            response(headers=[('Content-Type', 'application/json'), ('Content-Length', str(len(body) + 1))])]
        for raw in variants:
            with self.subTest(raw=raw[:100]), self.assertRaises(ValueError):
                engine_info.decode(raw, 'engine-original')

    def test_duplicate_json_and_non_linux_engine_cannot_establish_identity(self):
        for raw in (response(body=b'{"ID":"engine-original","ID":"replacement","OSType":"linux","OperatingSystem":"Docker"}'),
                    response(value={'ID': 'engine-original', 'OSType': 'windows', 'OperatingSystem': 'Windows'})):
            with self.assertRaises(ValueError): engine_info.decode(raw, 'engine-original')

    def test_output_and_stderr_overflow_refuse_without_unbounded_buffering(self):
        for process in (Process(b'x' * (engine_info.MAX_BODY + engine_info.MAX_HEADERS + 1)),
                        Process(response(), stderr=b'x' * (engine_info.MAX_STDERR + 1))):
            with patch.object(engine_info.subprocess, 'Popen', return_value=process), self.assertRaisesRegex(ValueError, 'transport failed'):
                engine_info.read('engine-original')

    def test_deadline_terminates_exact_owned_client_no_transport_fallback(self):
        process = Process(response(), code=None)
        with patch.object(engine_info.subprocess, 'Popen', return_value=process) as start, patch.object(engine_info.time, 'monotonic', side_effect=[0, 6]):
            with self.assertRaisesRegex(ValueError, 'deadline exceeded'):
                engine_info.read('engine-original', timeout=5)
        self.assertTrue(process.terminated)
        self.assertEqual(start.call_count, 1)

    def test_nonzero_cli_exit_and_invalid_deadline_refuse(self):
        with patch.object(engine_info.subprocess, 'Popen', return_value=Process(response(), code=1)), self.assertRaisesRegex(ValueError, 'no fallback'):
            engine_info.read('engine-original')
        with patch.object(engine_info.subprocess, 'Popen') as start:
            for timeout in (0, 16, True):
                with self.assertRaises(ValueError): engine_info.read('engine-original', timeout)
            start.assert_not_called()


if __name__ == '__main__': unittest.main()
