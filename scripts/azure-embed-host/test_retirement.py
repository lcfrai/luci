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
        calls = []; flags = {'running': True, 'lost': False, 'foreign': False, 'active': False, 'lost_start': False}
        def command(args, timeout=30):
            calls.append(args)
            if args[:2] == ['systemctl', 'stop']:
                if flags['lost']: raise TimeoutError('lost stop ACK')
            if 'stop' in args and args[:2] == ['docker', 'compose']: flags['running'] = False
            if 'start' in args and args[:2] == ['docker', 'compose']:
                flags['running'] = True
                if flags['lost_start']: raise TimeoutError('lost start ACK')
            if args[:2] == ['systemctl', 'restart']: flags['active'] = True
            if args[:2] == ['systemctl', 'is-active']: return 'active' if flags['active'] else 'inactive'
            if args[:2] == ['systemctl', 'show']: return 'inactive'
            if 'ps' in args and args[:2] == ['docker', 'compose']: return 'api\nclient-proxy\norchestrator' if flags['running'] else ''
            if args[:2] == ['docker', 'ps']: return 'unrelated-running' if flags['foreign'] else ''
            if args[:2] == ['docker', 'info']: return '/var/lib/docker'
            return ''
        return selected, record, calls, flags, command

    def run_case(self, selected, record, command, inspect=False, resume=False):
        with patch.object(service, 'pinned_package', return_value={'luciRevision': 'b' * 40}), patch.object(service, 'guest_source_ready'), \
                patch.object(service, 'journal', return_value=record), patch.object(service, 'save'), patch.object(service, 'command', side_effect=command), \
                patch.object(service.Path, 'exists', return_value=True), patch.object(service.Path, 'is_dir', return_value=True), \
                patch.object(service.Path, 'is_symlink', return_value=False), patch.object(service.os, 'stat', return_value=SimpleNamespace(st_dev=1, st_mode=stat.S_IFDIR)), \
                patch.object(service, 'switch_gateway', side_effect=lambda _: command(['systemctl', 'restart', 'luci-azure-embed-gateway.service'])):
            return (service.retirement_resume if resume else service.retirement)(selected, 'c' * 64, inspect)

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

    def test_actor_resume_starts_only_retained_containers_and_refuses_late_stop(self):
        selected, record, calls, flags, command = self.fixture()
        self.run_case(selected, record, command)
        result = self.run_case(selected, record, command, resume=True)
        self.assertEqual(result['state'], 'retirement_cancelled_active'); self.assertFalse(result['taskReservationsReleased'])
        self.assertTrue(any('start' in args and args[:2] == ['docker', 'compose'] for args in calls))
        self.assertFalse(any('up' in args or 'down' in args or '--volumes' in args for args in calls))
        with self.assertRaisesRegex(ValueError, 'cancelled'): self.run_case(selected, record, command)

    def test_unknown_resume_does_not_start_twice(self):
        selected, record, calls, flags, command = self.fixture()
        self.run_case(selected, record, command); flags['lost_start'] = True
        with self.assertRaises(TimeoutError): self.run_case(selected, record, command, resume=True)
        with self.assertRaisesRegex(RuntimeError, 'uncertain'): self.run_case(selected, record, command, True, True)
        self.assertEqual(sum('start' in args and args[:2] == ['docker', 'compose'] for args in calls), 1)
        flags['active'] = True
        self.assertEqual(self.run_case(selected, record, command, True, True)['state'], 'retirement_cancelled_active')


if __name__ == '__main__': unittest.main()
