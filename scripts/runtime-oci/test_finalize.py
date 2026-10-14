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
