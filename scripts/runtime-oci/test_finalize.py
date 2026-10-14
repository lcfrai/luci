"""Offline finalization refusal/grant checks; no Docker or network effects."""
import copy
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import archive
from common import write_json
import finalize
import grant_fallbacks
import image_licenses
import producer


class GrantAndFinalizeTests(unittest.TestCase):
    def test_pre_effect_refusal_reports_native_memory_cpu_and_inventory(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo = root / 'finalizer'
            repo.mkdir()
            build, original, output = root / 'build', root / 'original', root / 'new'
            build.mkdir()
            original.mkdir()
            with patch.object(producer, 'clean_source'), patch.object(finalize, 'completed_proof', return_value=({'sourceCommit': 'a' * 40}, {}, {}, 'image')), patch.object(producer, 'docker_ids', return_value=['foreign']), patch.object(producer, 'cpu_busy', return_value=92.5), patch.object(producer, 'available_memory', return_value=123), patch.object(producer, 'command') as command:
                with self.assertRaisesRegex(ValueError, '"nativeAvailableBytes": 123.*"requiredBytes": 2684354560.*"runningCount": 1') as refused:
                    finalize.finalize(repo, 'b' * 40, build, original, output)
                self.assertIn('"cpuBusyPercent": 92.5', str(refused.exception))
                command.assert_not_called()
                self.assertFalse(output.exists())

    def test_short_rm_probe_disappearance_is_sampling_only_not_settlement(self):
        import subprocess
        failure = producer.CommandFailure(['docker', 'inspect', 'abc123'], subprocess.CompletedProcess([], 1, b'', b'Error: No such object: abc123\n'))
        def command(args):
            if args[1] == 'inspect':
                raise failure
            return b'original\n'
        with patch.dict(producer.ENGINE_IDENTITIES, {'session': 'original'}), patch.object(producer, 'docker_ids', side_effect=[['abc123'], []]), patch.object(producer, 'command', side_effect=command), patch.object(producer, 'assert_settled') as settle:
            self.assertEqual(producer.owned_running('session'), [])
            settle.assert_not_called()
        with patch.dict(producer.ENGINE_IDENTITIES, {'session': 'changed'}), patch.object(producer, 'docker_ids', side_effect=[['abc123'], []]), patch.object(producer, 'command', side_effect=command):
            with self.assertRaises(producer.CommandFailure):
                producer.owned_running('session')
        for stderr in (b'permission denied', b'connection refused'):
            error = producer.CommandFailure(['docker', 'inspect', 'abc123'], subprocess.CompletedProcess([], 1, b'', stderr))
            with patch.object(producer, 'docker_ids', return_value=['abc123']), patch.object(producer, 'command', side_effect=error):
                with self.assertRaises(producer.CommandFailure):
                    producer.owned_running('session')

    def test_version_and_spdx_must_match_original_authoritative_grant(self):
        repo = Path(__file__).resolve().parents[2]
        values = grant_fallbacks.load(repo)
        with patch.object(image_licenses, 'SOURCE_GRANT_FALLBACKS', values, create=True):
            self.assertIsNotNone(image_licenses.find_fallback('@openai/codex', '0.160.0', 'Apache-2.0'))
            self.assertIsNotNone(image_licenses.find_fallback('@openai/codex', '0.160.0-linux-x64', 'Apache-2.0'))
            for name, version, grant in (('@openai/codex', '0.160.1', 'Apache-2.0'), ('@openai/codex', '0.160.0', 'MIT'), ('unrelated', '0.160.0', 'Apache-2.0')):
                self.assertIsNone(image_licenses.find_fallback(name, version, grant))

    def test_upstream_grant_byte_drift_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            relative = 'third-party/licenses/openai-codex-0.160.0'
            shutil.copytree(Path(__file__).resolve().parents[2] / relative, root / relative)
            (root / relative / 'NOTICE').write_bytes(b'replaced original notice')
            with self.assertRaisesRegex(ValueError, 'bytes differ'):
                grant_fallbacks.load(root)

    def test_unknown_failed_cancelled_or_wrong_session_never_reaches_docker(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            pending = {'session': 'a' * 32, 'sourceCommit': 'b' * 40}
            terminal = {'builderExitCode': 0, 'cancelRequested': False, 'terminalStreamReturned': True,
                        'exactOwnedContainersRemaining': [], 'session': pending['session'], 'engineID': 'original'}
            write_json(root / 'producer-pending.json', pending)
            write_json(root / 'producer-engine.json', {'engineID': 'original'})
            for changes in ({'builderExitCode': 1}, {'builderExitCode': False}, {'cancelRequested': True}, {'terminalStreamReturned': False}, {'session': 'c' * 32}, {'exactOwnedContainersRemaining': ['retained']}):
                write_json(root / 'producer-terminal.json', dict(terminal, **changes))
                with patch.object(producer, 'command') as command:
                    with self.assertRaisesRegex(ValueError, 'positive original'):
                        finalize.completed_proof(root)
                    command.assert_not_called()
            write_json(root / 'producer-terminal.json', terminal)
            write_json(root / 'producer-unknown.json', {'reason': 'stream lost'})
            with self.assertRaisesRegex(ValueError, 'unknown/failed'):
                finalize.completed_proof(root)


if __name__ == '__main__':
    unittest.main()
