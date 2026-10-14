"""Inert client/gate fixtures verify publication and uncertainty boundaries."""
from pathlib import Path
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import archive
import producer
import pull_producer


class PullProducerTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.helper = SimpleNamespace(output=self.root, session='a' * 32, engine='engine', image_id='sha256:' + 'b' * 64,
            owner={'tag': '127.0.0.1:37891/luci/runtime-export-' + 'a' * 32 + ':payload', 'port': 37891,
                   'session': 'a' * 32, 'repository': 'luci/runtime-export-' + 'a' * 32}, fence=Mock())
        self.status = {'version': 1, 'session': self.helper.session, 'repository': self.helper.owner['repository'],
            'childReadyAtStart': True, 'ownershipSettled': False, 'upload': 'idle', 'activeUpload': None,
            'manifest': 'committed', 'manifestDigest': 'sha256:' + 'c' * 64}
        self.client = SimpleNamespace(returncode=0, poll=lambda: 0)

    def test_success_requires_cli_terminal_and_owned_committed_gate_readback(self):
        with patch.object(producer.subprocess, 'Popen', return_value=self.client), patch.object(pull_producer.registry_producer, 'read_loopback', return_value=archive.encoded(self.status)):
            result = pull_producer.push(self.helper)
        self.assertEqual(result['commandExitStatus'], 0)
        self.assertEqual(result['manifestDigest'], self.status['manifestDigest'])
        self.assertTrue((self.root / 'registry-push-terminal.json').is_file())
        self.assertFalse(result['engineRetired'])

    def test_lost_gate_response_or_pending_upload_cannot_create_terminal_receipt(self):
        with patch.object(producer.subprocess, 'Popen', return_value=self.client), patch.object(pull_producer.registry_producer, 'read_loopback', side_effect=OSError('lost response')):
            with self.assertRaises(OSError): pull_producer.push(self.helper)
        self.assertFalse((self.root / 'registry-push-terminal.json').exists())
        self.assertTrue((self.root / 'registry-push-intent.json').exists())

    def test_native_pressure_terminates_only_owned_client_and_retains_uncertainty(self):
        client = Mock()
        client.poll.return_value = None
        for file in ('push.stdout.log', 'push.stderr.log'): (self.root / file).write_bytes(b'')
        failures = []
        with patch.object(producer, 'available_memory', return_value=producer.RESERVE), patch.object(producer, 'cpu_sample', return_value=(0, 1)):
            pull_producer.watch_push(client, threading.Event(), self.root, failures)
        client.terminate.assert_called_once()
        self.assertTrue(failures)
        self.assertFalse((self.root / 'registry-push-terminal.json').exists())

    def test_push_admission_allows_only_owned_placed_helper_and_fresh_margin(self):
        self.helper.estimate = {'requiredBytes': 4 << 30}
        with patch.object(producer, 'available_memory', return_value=5 << 30), patch.object(producer, 'cpu_busy', return_value=10):
            pull_producer.admit_push(self.helper)
        self.helper.fence.assert_called_once()
        with patch.object(producer, 'available_memory', return_value=3 << 30), patch.object(producer, 'cpu_busy', return_value=10):
            with self.assertRaisesRegex(ValueError, 'fresh host reserve'):
                pull_producer.admit_push(self.helper)


if __name__ == '__main__': unittest.main()
