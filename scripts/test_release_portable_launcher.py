"""Cross-package admission through the real launcher and a bounded HTTP fixture.

No process execution is allowed behind this already-running service fixture.
The installed acceptance separately exercises actual cold launch and cleanup.
"""
import http.server
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parent.parent


class PortableLauncherIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.package = self.root / 'new-package'
        self.package.mkdir()
        self.state = self.root / 'state'
        (self.state / 'control-plane').mkdir(parents=True)
        self.proof = secrets.token_urlsafe(32)
        (self.state / 'control-plane/owner-access-token').write_text(self.proof + '\n')
        self.saved = self.state / 'saved-work'
        self.saved.write_text('The existing workspace must remain unchanged.')
        self.identity = 'a' * 64
        self.served_identity = self.identity
        self.requests = []
        self.windows = os.name == 'nt' or os.environ.get('LUCI_LAUNCHER_TEST_PLATFORM') == 'powershell'
        self.powershell = 'powershell.exe' if os.name == 'nt' else shutil.which('pwsh')
        if self.windows and not self.powershell:
            self.skipTest('PowerShell is unavailable')
        self.launcher = self.package / ('Start-Luci.ps1' if self.windows else 'start-luci.sh')
        shutil.copyfile(ROOT / 'scripts' / ('launch-windows.ps1' if self.windows else 'launch-unix.sh'), self.launcher)
        binary = self.package / ('luci.exe' if self.windows else 'luci')
        binary.write_text('#!/bin/sh\nexit 89\n')
        binary.chmod(0o700)
        (self.package / 'package-identity').write_text(self.identity + '\n')
        outer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                outer.requests.append((self.command, self.path))
                status, body, kind = 200, b'', 'application/json'
                if self.path == '/v1/control-plane/status':
                    body = json.dumps({'product': 'Luci', 'version': '0.5.0', 'bootstrapRequired': False}).encode()
                elif self.headers.get('X-Luci-Local-Access-Token') != outer.proof:
                    status = 403
                elif self.path == '/v1/auth/local-owner':
                    body = b'{"owner":null}'
                elif self.path == '/v1/auth/local-package' and outer.served_identity is not None:
                    body, kind = (outer.served_identity + '\n').encode(), 'text/plain; charset=utf-8'
                else:
                    status = 404
                self.send_response(status)
                self.send_header('Content-Type', kind)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_POST(self):
                outer.requests.append((self.command, self.path))
                self.send_error(500, 'No mutation is permitted')

        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(thread.join, 5)
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)

    def launch(self):
        retained_proof = (self.state / 'control-plane/owner-access-token').read_text()
        if self.windows:
            command = [self.powershell, '-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', str(self.launcher), '-Port', str(self.server.server_port), '-StateDir', str(self.state), '-NoBrowser']
        else:
            command = ['bash', str(self.launcher), '--port', str(self.server.server_port), '--state-dir', str(self.state), '--no-browser']
        result = subprocess.run(command, input='\n', text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=30)
        self.assertNotIn(self.proof, result.stdout)
        self.assertNotIn(retained_proof.strip(), result.stdout)
        self.assertEqual(self.saved.read_text(), 'The existing workspace must remain unchanged.')
        self.assertTrue(all(method == 'GET' for method, _ in self.requests), 'Launcher must not stop or mutate the existing service')
        self.assertEqual((self.state / 'control-plane/owner-access-token').read_text(), retained_proof)
        return result

    def test_same_package_reuses_existing_service(self):
        result = self.launch()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('already running', result.stdout)
        self.assertIn(('GET', '/v1/auth/local-package'), self.requests)

    def test_different_package_same_version_requires_explicit_close(self):
        self.served_identity = 'b' * 64
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('different or older Luci package', result.stdout)
        self.assertNotIn('Luci is running at', result.stdout)

    def test_legacy_service_without_identity_requires_explicit_close(self):
        self.served_identity = None
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Close its launcher', result.stdout)

    def test_foreign_state_proof_still_refused_before_package_lookup(self):
        (self.state / 'control-plane/owner-access-token').write_text('b' * 43 + '\n')
        result = self.launch()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('does not belong to this launcher', result.stdout)
        self.assertNotIn(('GET', '/v1/auth/local-package'), self.requests)
        self.assertTrue(all(method == 'GET' for method, _ in self.requests))

    def test_incomplete_or_malformed_package_fails_before_contacting_service(self):
        file = self.package / 'package-identity'
        for invalid in ('', 'a' * 64 + '\n' * 100, 'A' * 64, 'not-an-identity'):
            with self.subTest(invalidLength=len(invalid)):
                file.write_text(invalid)
                self.assertNotEqual(self.launch().returncode, 0)
                self.assertEqual(self.requests, [])
        file.unlink()
        self.assertNotEqual(self.launch().returncode, 0)
        self.assertEqual(self.requests, [])


if __name__ == '__main__':
    unittest.main()
