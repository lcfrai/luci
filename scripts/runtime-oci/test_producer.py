"""Controlled command-adapter checks, not real Docker execution evidence."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import producer
from common import write_json


class ProducerSafetyTests(unittest.TestCase):
    def test_assistance_policy_is_finite_and_cannot_lower_normal_reserve(self):
        self.assertEqual(producer.RESERVE, 1073741824)
        self.assertEqual(producer.BUILD_MEMORY, 1073741824)
        self.assertEqual(producer.CACHE_TRIGGER - producer.RESERVE, 1536 << 20)
        self.assertEqual(producer.MAX_CACHE_ACTIONS, 36)
        self.assertEqual(producer.CACHE_INTERVAL_SECONDS, 5)

    def test_failed_builder_terminal_is_recorded_with_exact_engine_and_cleanup(self):
        class Finished:
            returncode = 137
            def poll(self):
                return self.returncode
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            write_json(output / 'producer-engine.json', {'engineID': 'original'})
            with patch.object(producer, 'command', return_value=b'{"ID":"original"}'), patch.object(producer, 'assert_settled') as settled:
                with self.assertRaisesRegex(ValueError, 'failed'):
                    producer.monitor_build(Finished(), 'session', output, True)
                settled.assert_called_once_with('session')
            from common import read_json
            value = read_json(output / 'producer-terminal.json')
            self.assertEqual(value['builderExitCode'], 137)
            self.assertTrue(value['terminalStreamReturned'])
            self.assertFalse((output / 'producer-unknown.json').exists())
    def test_foreign_container_is_never_returned_for_owned_cancellation(self):
        def command(args, **kwargs):
            if args[1] == 'ps':
                return b'own\nforeign\n'
            identifier = args[-1]
            return json.dumps([{'Id': identifier, 'Config': {'Labels': {producer.SESSION_LABEL: 'session' if identifier == 'own' else 'another'}},
                                'HostConfig': {'Memory': 1 << 30, 'MemorySwap': 1 << 30, 'CpuPeriod': 100000, 'CpuQuota': 100000},
                                'State': {'OOMKilled': False}}]).encode()
        with patch.object(producer, 'command', side_effect=command):
            with self.assertRaisesRegex(ValueError, 'unrelated'):
                producer.owned_running('session')
            self.assertEqual([row['id'] for row in producer.owned_running('session', reject_unrelated=False)], ['own'])

    def test_cache_assistance_refuses_engine_change_before_wsl_mutation(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            write_json(output / 'producer-engine.json', {'engineID': 'original'})
            with patch.object(producer, 'command', return_value=b'{"ID":"replacement","OperatingSystem":"Docker Desktop"}') as command:
                with self.assertRaisesRegex(ValueError, 'same exact'):
                    producer.cache_assist('session', output)
                self.assertEqual(command.call_count, 1)
                self.assertEqual(command.call_args.args[0][:2], ['docker', 'info'])

    def test_context_uses_inspected_immutable_bases_and_only_producer_labels(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            canonical = root / 'canonical'
            (canonical / 'images/sandbox').mkdir(parents=True)
            original = b'FROM golang:1.26-bookworm AS build\nRUN tar -C /usr/local -czf /out/go-toolchain.tar.gz go\nFROM node:24-bookworm-slim\nCOPY --from=build /out/go-toolchain.tar.gz /tmp/go-toolchain.tar.gz\nRUN tar -C /usr/local -xzf /tmp/go-toolchain.tar.gz && rm /tmp/go-toolchain.tar.gz\n'
            (canonical / 'images/sandbox/Dockerfile').write_bytes(original)
            bases = [{'reference': name, 'imageID': 'sha256:' + char * 64} for name, char in [('golang:1.26-bookworm', 'a'), ('node:24-bookworm-slim', 'b')]]
            producer.context_tar(canonical, root / 'context.tar', bases, 'exactsession')
            import tarfile
            with tarfile.open(root / 'context.tar') as archive:
                value = archive.extractfile('images/sandbox/Dockerfile').read().decode()
            self.assertNotIn('FROM golang:', value)
            self.assertIn('FROM sha256:' + 'a' * 64 + ' AS build', value)
            self.assertEqual(value.count(producer.SESSION_LABEL + '="exactsession"'), 2)
            self.assertEqual([line for line in value.splitlines() if not line.startswith(('FROM ', 'LABEL '))],
                             [line for line in original.decode().splitlines() if not line.startswith('FROM ')])
            self.assertEqual((canonical / 'images/sandbox/Dockerfile').read_bytes(), original)


if __name__ == '__main__':
    unittest.main()
