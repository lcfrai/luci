"""Owned runtime retirement source; Docker/systemd are fixtures only."""
import copy
import importlib.util
from pathlib import Path
import stat
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('host_retirement', Path(__file__).with_name('service.py'))
service = importlib.util.module_from_spec(spec); spec.loader.exec_module(service)


class HostRetirementTests(unittest.TestCase):
    def fixture(self):
        selected = {'scope': 'scope-one', 'generation': 'gen-one', 'runtimePackageSHA256': 'a' * 64,
            'secretRefs': {'retained-task': 'fixture'}}
        record = {'state': 'active', 'scope': selected['scope'], 'generation': selected['generation'],
            'packageSHA256': selected['runtimePackageSHA256'], 'secretRefs': selected['secretRefs'], 'prepared': {'stage': '/run/luci_embed/luci-embed-owned'}}
        calls = []; flags = {'running': True, 'lost': False, 'foreign': False}
        def command(args, timeout=30):
            calls.append(args)
            if args[:2] == ['systemctl', 'stop']:
                if flags['lost']: raise TimeoutError('lost stop ACK')
            if 'stop' in args and args[:2] == ['docker', 'compose']: flags['running'] = False
            if args[:2] == ['systemctl', 'show']: return 'inactive'
            if 'ps' in args and args[:2] == ['docker', 'compose']: return 'api\nclient-proxy\norchestrator' if flags['running'] else ''
            if args[:2] == ['docker', 'ps']: return 'unrelated-running' if flags['foreign'] else ''
            if args[:2] == ['docker', 'info']: return '/var/lib/docker'
            return ''
        return selected, record, calls, flags, command

    def run_case(self, selected, record, command, inspect=False):
        with patch.object(service, 'pinned_package', return_value={'luciRevision': 'b' * 40}), patch.object(service, 'guest_source_ready'), \
                patch.object(service, 'journal', return_value=record), patch.object(service, 'save'), patch.object(service, 'command', side_effect=command), \
                patch.object(service.Path, 'exists', return_value=True), patch.object(service.Path, 'is_dir', return_value=True), \
                patch.object(service.Path, 'is_symlink', return_value=False), patch.object(service.os, 'stat', return_value=SimpleNamespace(st_dev=1, st_mode=stat.S_IFDIR)):
            return service.retirement(selected, 'c' * 64, inspect)

    def test_stops_only_owned_stack_preserves_volumes_and_verifies_os_disk_storage(self):
        selected, record, calls, _, command = self.fixture()
        result = self.run_case(selected, record, command)
        self.assertEqual(result['state'], 'retirement_stopped')
        self.assertTrue(result['runtimeStorageOnOSDiskVerified']); self.assertTrue(result['taskAndTemplateVolumesPreserved'])
        self.assertFalse(result['computeDeallocationVerified'])
        self.assertTrue(any('stop' in args and args[:2] == ['docker', 'compose'] for args in calls))
        self.assertFalse(any('down' in args or 'rm' in args or '--volumes' in args for args in calls))

    def test_unknown_gateway_stop_inspects_without_repeating_stop(self):
        selected, record, calls, flags, command = self.fixture(); flags['lost'] = True
        with self.assertRaises(TimeoutError): self.run_case(selected, record, command)
        self.assertEqual(record['state'], 'retirement_stop_pending')
        with self.assertRaisesRegex(RuntimeError, 'still run'): self.run_case(selected, record, command, True)
        self.assertEqual(sum(args[:2] == ['systemctl', 'stop'] for args in calls), 1)
        flags['running'] = False
        self.assertEqual(self.run_case(selected, record, command, True)['state'], 'retirement_stopped')
        self.assertFalse(any('stop' in args and args[:2] == ['docker', 'compose'] for args in calls))

    def test_unrelated_running_container_refuses_backup_and_compute_claim(self):
        selected, record, calls, flags, command = self.fixture(); flags['foreign'] = True
        with self.assertRaisesRegex(RuntimeError, 'unrelated work'): self.run_case(selected, record, command)
        self.assertEqual(record['state'], 'retirement_stop_pending')


if __name__ == '__main__': unittest.main()
