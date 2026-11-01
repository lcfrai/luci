"""Offline transport regressions; no subprocess/provider calls are made."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest import mock

def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
    return module

HERE = Path(__file__).parent
guided = load('guided_transport', HERE / 'guided.py')
operator = load('operator_transport', HERE / 'operator_setup.py')
mount = load('mount_transport', HERE.parent / 'azure-controller-disk/mount.py')
recover = load('recover_transport', HERE.parent / 'azure-controller-disk/recover.py')

class OperatorTransportTests(unittest.TestCase):
    def test_large_private_operator_and_signed_envelope_keep_distinct_bounds(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'operator.json'
            path.write_bytes(b'x' * (96 << 10)); path.chmod(0o600)
            self.assertEqual(len(mount.private(path, mount.OPERATOR_MAX_BYTES)), 96 << 10)
            with self.assertRaises(ValueError): mount.private(path)
            path.write_bytes(b'x' * ((96 << 10) + 1))
            with self.assertRaises(ValueError): mount.private(path, mount.OPERATOR_MAX_BYTES)
            path.write_bytes(b'x' * (128 << 10))
            self.assertEqual(len(mount.private(path, mount.FINALIZATION_MAX_BYTES)), 128 << 10)
            path.write_bytes(b'x' * ((128 << 10) + 1))
            with self.assertRaises(ValueError): mount.private(path, mount.FINALIZATION_MAX_BYTES)

    def test_only_exact_signed_envelope_row_gets_larger_transport(self):
        for module in (operator, recover):
            self.assertEqual(module.successor_file_limit('/etc/luci/azure-operator-finalization.json'), 128 << 10)
            for path in ('/etc/luci/azure-private-ca.crt', '/etc/luci/operator-template.json', '/etc/luci/azure-controller.env'):
                self.assertEqual(module.successor_file_limit(path), 16384)

    def test_large_run_command_uses_exact_owned_file_then_removes_it(self):
        script = 'echo fixture\n' + ('#' * (180 << 10))
        retained = []
        def cli(arguments, timeout):
            path = Path(arguments[arguments.index('--scripts') + 1][1:]); retained.append(path)
            self.assertTrue(path.is_file()); self.assertEqual(path.read_text(), script)
            self.assertEqual(timeout, 300)
            return {'fixture': True}
        args = ['vm', 'run-command', 'invoke', '--ids', 'original', '--command-id', 'RunShellScript', '--scripts', script]
        with mock.patch.object(guided, '_az_json', side_effect=cli) as provider:
            self.assertEqual(guided.az_json(args), {'fixture': True})
            provider.assert_called_once()
        self.assertFalse(retained[0].exists()); self.assertEqual(args[-1], script)
        with mock.patch.object(guided, '_az_json') as provider:
            with self.assertRaises(ValueError): guided.az_json([*args[:-1], 'x' * ((1 << 20) + 1)])
            provider.assert_not_called()
