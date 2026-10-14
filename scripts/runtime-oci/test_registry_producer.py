"""Offline producer refusal/ownership tests; never registry or Docker effects."""
import copy
from pathlib import Path
import tempfile
import shutil
import unittest
from unittest.mock import patch

import archive
import producer
import registry_producer as registry


class RegistryProducerTests(unittest.TestCase):
    def test_history_is_exact_integer_source_bound_and_complete(self):
        image = 'sha256:' + 'a' * 64
        rows = [{'ID': image, 'Size': '1678792826'}, {'ID': '<missing>', 'Size': '688444193'}]
        value = registry.validate_history(image, 2367237019, rows)
        self.assertEqual(value['largestLayerBytes'], 1678792826)
        for change in ('1.679GB', '-1', '1678792826.0', 1678792826):
            bad = copy.deepcopy(rows)
            bad[0]['Size'] = change
            with self.assertRaisesRegex(ValueError, 'integer sizes'):
                registry.validate_history(image, 2367237019, bad)
        with self.assertRaisesRegex(ValueError, 'sum to exact'):
            registry.validate_history(image, 2367237018, rows)
        rows[0]['ID'] = 'sha256:' + 'b' * 64
        with self.assertRaisesRegex(ValueError, 'exact original'):
            registry.validate_history(image, 2367237019, rows)

    def test_helper_and_largest_layer_remain_separate_from_real_reserve(self):
        estimate = registry.resource_estimate({'largestLayerBytes': 1678792826})
        self.assertEqual(estimate['helperMemoryBytes'], 512 << 20)
        self.assertFalse(estimate['daemonHardMemoryCap'])
        self.assertEqual(estimate['requiredBytes'], 3826276474)
        self.assertEqual(producer.RESERVE, 1 << 30)
        with patch.object(producer, 'docker_ids', return_value=[]), patch.object(producer, 'cpu_busy', return_value=10):
            with patch.object(producer, 'available_memory', return_value=estimate['requiredBytes'] - 1):
                with self.assertRaisesRegex(ValueError, 'nativeAvailableBytes'):
                    registry.admit_registry(estimate)
            with patch.object(producer, 'available_memory', return_value=estimate['requiredBytes']):
                self.assertEqual(registry.admit_registry(estimate)['runningCount'], 0)
        with patch.object(producer, 'docker_ids', return_value=['foreign']), patch.object(producer, 'cpu_busy', return_value=10), patch.object(producer, 'available_memory', return_value=8 << 30):
            with self.assertRaisesRegex(ValueError, 'runningCount'):
                registry.admit_registry(estimate)

    def test_missing_release_binary_is_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            with self.assertRaises((ValueError, FileNotFoundError)):
                registry.verify_distribution(Path(temporary))

    def test_helper_argv_has_no_socket_privilege_public_publish_or_extra_egress(self):
        owner = registry.identity('a' * 32, 19379)
        with tempfile.TemporaryDirectory() as temporary:
            tools, data = Path(temporary) / 'tools', Path(temporary) / 'data'
            tools.mkdir(); data.mkdir()
            args = registry.helper_args('sha256:' + 'b' * 64, owner, tools, data)
        self.assertNotIn('--privileged', args)
        self.assertNotIn('/var/run/docker.sock', ' '.join(args))
        self.assertEqual(args[args.index('--publish') + 1], '127.0.0.1:19379:5000')
        self.assertEqual(args[args.index('--memory') + 1], '536870912')
        self.assertEqual(args[args.index('--memory-swap') + 1], '536870912')
        self.assertEqual(args[-2:], ['--max-old-space-size=128', '/tools/registry_gate.mjs'])

    def test_terminal_signal_or_absence_alone_never_settles(self):
        owner = registry.identity('a' * 32, 19379)
        receipt = {'version': 1, 'session': owner['session'], 'repository': owner['repository'], 'socketsClosed': True,
                   'upload': 'idle', 'manifest': None, 'verifiedExport': False, 'engineRetired': False, 'child': {'exitCode': 0, 'signal': None}}
        self.assertEqual(registry.validate_terminal(receipt, owner), receipt)
        for field, value in (('session', 'b' * 32), ('socketsClosed', False), ('upload', 'active'), ('manifest', {'phase': 'publishing'}), ('child', {'exitCode': None, 'signal': 'SIGKILL'})):
            with self.assertRaisesRegex(ValueError, 'positive gate'):
                registry.validate_terminal(dict(receipt, **{field: value}), owner)
        with self.assertRaises(ValueError):
            registry.validate_terminal({}, owner)

    def test_reused_generation_or_foreign_status_refused_before_push(self):
        owner = registry.identity('a' * 32, 19379)
        status = {'version': 1, 'session': owner['session'], 'repository': owner['repository'], 'childReadyAtStart': True,
                  'ownershipSettled': False, 'upload': 'idle', 'activeUpload': None, 'completedUploads': 0, 'manifest': 'none'}
        self.assertEqual(registry.validate_status(status, owner), status)
        for field, value in (('session', 'b' * 32), ('activeUpload', 'pending'), ('completedUploads', 1), ('manifest', 'publishing')):
            with self.assertRaises(ValueError):
                registry.validate_status(dict(status, **{field: value}), owner)

    def test_public_binding_writable_tools_or_missing_caps_are_refused(self):
        owner, image = registry.identity('a' * 32, 19379), 'sha256:' + 'b' * 64
        row = {'Image': image, 'Config': {'Labels': {producer.SESSION_LABEL: owner['session']}, 'Entrypoint': ['node'],
                    'Cmd': ['--max-old-space-size=128', '/tools/registry_gate.mjs'], 'User': 'dev'},
               'HostConfig': {'Memory': 512 << 20, 'MemorySwap': 512 << 20, 'NanoCpus': 1000000000, 'PidsLimit': 64,
                    'ReadonlyRootfs': True, 'Privileged': False, 'CapDrop': ['ALL'], 'SecurityOpt': ['no-new-privileges:true'],
                    'NetworkMode': owner['networkName'], 'PortBindings': {'5000/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '19379'}]}},
               'Mounts': [{'Destination': '/tools', 'Source': '/private/tools', 'Type': 'bind', 'RW': False},
                    {'Destination': '/registry-data', 'Source': '/private/data', 'Type': 'bind', 'RW': True}]}
        registry.validate_helper(row, owner, image, '/private/tools', '/private/data')
        for key, value in (('Memory', 0), ('MemorySwap', -1), ('ReadonlyRootfs', False), ('CapDrop', []), ('Privileged', True),
                           ('PortBindings', {'5000/tcp': [{'HostIp': '0.0.0.0', 'HostPort': '19379'}]})):
            changed = copy.deepcopy(row); changed['HostConfig'][key] = value
            with self.assertRaises(ValueError):
                registry.validate_helper(changed, owner, image)
        changed = copy.deepcopy(row); changed['Mounts'][0]['RW'] = True
        with self.assertRaises(ValueError):
            registry.validate_helper(changed, owner, image)
        changed = copy.deepcopy(row); changed['Mounts'][0]['Source'] = '/private/unrelated'
        with self.assertRaises(ValueError):
            registry.validate_helper(changed, owner, image, '/private/tools', '/private/data')


if __name__ == '__main__':
    unittest.main()
