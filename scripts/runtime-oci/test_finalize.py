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
                with self.assertRaisesRegex(ValueError, '"nativeAvailableBytes": 123.*"requiredBytes": 2147483648.*"runningCount": 1') as refused:
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

    def test_all_observed_missing_declarations_are_exact_and_bounded(self):
        values = grant_fallbacks.load(Path(__file__).resolve().parents[2])
        with patch.object(image_licenses, 'SOURCE_GRANT_FALLBACKS', values, create=True):
            for name, version, grant in (('@npmcli/agent', '4.0.2', 'ISC'), ('@sigstore/verify', '3.1.1', 'Apache-2.0'), ('spdx-exceptions', '2.5.0', 'CC-BY-3.0'), ('spdx-license-ids', '3.0.23', 'CC0-1.0')):
                value = image_licenses.find_fallback(name, version, grant)
                self.assertTrue(value['files'])
                self.assertIsNone(image_licenses.find_fallback(name, version + '-changed', grant))
                self.assertIsNone(image_licenses.find_fallback(name, version, 'MIT'))
            claude = image_licenses.find_fallback('@anthropic-ai/claude-code', '2.1.274', 'SEE LICENSE IN README.md')
            self.assertTrue(claude['supplemental'])

    def test_distribution_grant_never_inherits_from_unrelated_parent(self):
        repo = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary) / 'opencode-ai'
            child = parent / 'node_modules/opencode-linux-x64'
            child.mkdir(parents=True)
            package = {'name': 'opencode-linux-x64', 'version': '1.18.34'}
            metadata = child / 'package.json'
            write_json(metadata, package)
            (parent / 'LICENSE').write_bytes((repo / 'third-party/licenses/opencode-ai-1.18.34/LICENSE').read_bytes())
            declaration = {'name': 'opencode-ai', 'version': '1.18.34', 'license': 'MIT', 'optionalDependencies': {'opencode-linux-x64': '1.18.34'}}
            write_json(parent / 'package.json', declaration)
            self.assertEqual(image_licenses.distribution_notice(metadata, package), parent / 'LICENSE')
            for field, value in (('name', 'unrelated'), ('version', '1.18.35'), ('license', 'ISC'), ('optionalDependencies', {})):
                write_json(parent / 'package.json', dict(declaration, **{field: value}))
                self.assertIsNone(image_licenses.distribution_notice(metadata, package))
            write_json(parent / 'package.json', declaration)
            (parent / 'LICENSE').write_bytes(b'wrong grant')
            with self.assertRaisesRegex(ValueError, 'parent grant differs'):
                image_licenses.distribution_notice(metadata, package)

    def test_new_npm_grant_and_provenance_drift_are_refused(self):
        original = Path(__file__).resolve().parents[2] / 'third-party/licenses'
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            target = root / 'third-party/licenses'
            shutil.copytree(original, target)
            changed = target / 'npmcli-agent-4.0.2/LICENSE'
            changed.write_bytes(b'replaced upstream grant')
            with self.assertRaisesRegex(ValueError, 'bytes differ'):
                grant_fallbacks.load(root)
            shutil.copyfile(original / 'npmcli-agent-4.0.2/LICENSE', changed)
            (target / 'npmcli-agent-4.0.2/provenance.json').write_bytes(b'{}')
            with self.assertRaisesRegex(ValueError, 'provenance changed'):
                grant_fallbacks.load(root)

    def test_no_build_finalizer_and_export_fresh_admission_boundary(self):
        required = 2 << 30
        self.assertEqual(producer.BUILD_MEMORY, 1 << 30)
        self.assertEqual(finalize.FINALIZE_MEMORY, max(producer.PROBE_MEMORY, producer.EXPORT_MEMORY))
        self.assertEqual(producer.RESERVE + finalize.FINALIZE_MEMORY + producer.MARGIN, required)
        with patch.object(producer, 'docker_ids', return_value=[]), patch.object(producer, 'cpu_busy', return_value=10):
            with patch.object(producer, 'available_memory', return_value=required - 1):
                with self.assertRaisesRegex(ValueError, 'insufficient-native-memory'):
                    finalize.admit_finalize()
            with patch.object(producer, 'available_memory', return_value=required):
                self.assertEqual(finalize.admit_finalize(), (required, 10))
        with patch.object(producer, 'owned_running', return_value=[]), patch.object(producer, 'cache_assist') as cache:
            with patch.object(producer, 'available_memory', return_value=required - 1):
                with self.assertRaisesRegex(ValueError, 'lacks normal reserve'):
                    producer.prepare_effect('session', Path('unused'), producer.EXPORT_MEMORY, False)
            with patch.object(producer, 'available_memory', return_value=required):
                self.assertEqual(producer.prepare_effect('session', Path('unused'), producer.EXPORT_MEMORY, False), 0)
            cache.assert_not_called()

    def test_actual_probe_argv_keeps_512_mib_hard_memory_swap_and_cpu(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            def run(args, session, output, stdout, stderr, memory, timeout):
                self.assertEqual(memory, 512 << 20)
                self.assertEqual(args[args.index('--memory') + 1], '536870912')
                self.assertEqual(args[args.index('--memory-swap') + 1], '536870912')
                self.assertEqual(args[args.index('--cpus') + 1], '1')
                stdout.write_bytes(b'sandboxd 0.5.0\n')
                stderr.write_bytes(b'')
                return 0
            with patch.object(producer, 'scoped_run', side_effect=run) as run:
                self.assertEqual(producer.probe('image', 'sandboxd', '0.5.0', 'sandboxd', 'session', root)['exitCode'], 0)
                run.assert_called_once()

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
