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

    def test_helper_argv_has_no_socket_privilege_or_public_publish(self):
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

    def test_actual_publication_and_owned_ordinary_bridge_required(self):
        owner = registry.identity('a' * 32, 19379)
        ports = {'5000/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '19379'}], '8080/tcp': None}
        self.assertEqual(registry.validate_published_ports({'NetworkSettings': {'Ports': ports}}, owner), ports)
        for bad in ({}, None, {'5000/tcp': None},
                    {'5000/tcp': [{'HostIp': '0.0.0.0', 'HostPort': '19379'}]},
                    {'5000/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '19378'}]},
                    dict(ports, **{'8080/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '19380'}]})):
            with self.assertRaisesRegex(ValueError, 'actual helper publication'):
                registry.validate_published_ports({'NetworkSettings': {'Ports': bad}}, owner)
        net = {'Name': owner['networkName'], 'Driver': 'bridge', 'Internal': False,
               'Labels': {producer.SESSION_LABEL: owner['session']}, 'Containers': {}}
        registry.validate_network(net, owner)
        for key, value in (('Internal', True), ('Name', 'bridge'), ('Driver', 'host'), ('Containers', {'foreign': {}})):
            with self.assertRaises(ValueError):
                registry.validate_network(dict(net, **{key: value}), owner)

    def test_daemon_probe_requires_independent_exact_manifest_404_witness(self):
        owner = registry.identity('a' * 32, 19379)
        result = {'version': 1, 'engineID': 'engine', 'session': owner['session'], 'reference': owner['tag'],
                  'operation': 'Engine.DistributionInspect', 'outcome': 'not_found',
                  'registryAuthProvided': False, 'tagOrPushPerformed': False}
        log = ('time="2026-10-14T07:00:00Z" http.request.uri=/v2/' + owner['repository'] + '/manifests/payload http.response.status=404').encode()
        self.assertEqual(registry.validate_daemon_witness(result, owner, 'engine', log)['matchingRequestCount'], 1)
        for bad in (b'', log.replace(b'payload', b'foreign'), log.replace(b'status=404', b'status=500'), b'GET /v2/ 200'):
            with self.assertRaisesRegex(ValueError, 'witness unavailable'):
                registry.validate_daemon_witness(result, owner, 'engine', bad)
        for key, value in (('engineID', 'foreign'), ('operation', 'Windows.HTTPGet'), ('outcome', 'success'), ('registryAuthProvided', True), ('tagOrPushPerformed', True)):
            with self.assertRaises(ValueError):
                registry.validate_daemon_witness(dict(result, **{key: value}), owner, 'engine', log)

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

    def test_failed_prewrite_requires_positive_nonzero_exit_and_no_push_intent(self):
        owner = registry.identity('a' * 32, 19379)
        image, container = 'sha256:' + 'b' * 64, 'c' * 64
        row = {'Id': container, 'State': {'Status': 'exited', 'Running': False, 'Pid': 0, 'ExitCode': 1,
               'FinishedAt': '2026-10-14T06:42:46.268942792Z', 'OOMKilled': False}}
        intent = {'owner': owner, 'imageID': image, 'engineID': 'engine'}
        created = {'containerID': container, 'engineID': 'engine'}
        place = {'session': owner['session'], 'imageID': image, 'pushStarted': False}
        unknown = {'containerID': container, 'session': owner['session'], 'pushStarted': False}
        operations = [{'session': owner['session'], 'engineID': 'engine', 'argv': ['docker', 'start', container]}]
        with patch.object(registry, 'validate_helper'):
            self.assertEqual(registry.validate_failed_prewrite(row, intent, created, place, unknown, operations), row['State'])
            for key, value in (('Status', 'running'), ('Running', True), ('Pid', 100), ('ExitCode', 0), ('FinishedAt', '0001-01-01T00:00:00Z')):
                changed = copy.deepcopy(row); changed['State'][key] = value
                with self.assertRaises(ValueError):
                    registry.validate_failed_prewrite(changed, intent, created, place, unknown, operations)
            operations[0]['argv'] = ['docker', 'push', owner['tag']]
            with self.assertRaisesRegex(ValueError, 'possible push/tag'):
                registry.validate_failed_prewrite(row, intent, created, place, unknown, operations)
            with self.assertRaises(ValueError):
                registry.validate_failed_prewrite(row, intent, created, dict(place, pushStarted=True), unknown, [])


if __name__ == '__main__':
    unittest.main()
