#!/usr/bin/env python3
"""Filesystem/format/quiescence acceptance for the offline operator command."""
import contextlib
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
import zipfile

SCRIPT = Path(__file__).with_name('managed-host-backup.py')
spec = importlib.util.spec_from_file_location('hostbackup', SCRIPT)
hostbackup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hostbackup)
TEAM = 'team-abcdefghijklmnopqrstuv'
OTHER = 'team-vutsrqponmlkjihgfedcba'

def put(path, data, mode=0o600):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    path.write_bytes(data if isinstance(data, bytes) else json.dumps(data).encode())
    path.chmod(mode)

class HostRecovery(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='luci-host-recovery-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.state = self.root / 'state'
        self.state.mkdir(mode=0o700)
        self.archive = self.root / 'host.zip'
        self.worker = f'control-plane/teams/{TEAM}/runtime/state'
        self.project = f'control-plane/teams/{TEAM}/runtime/projects/app'
        owner = {'id': 'user-owner', 'username': 'owner', 'displayName': 'Owner', 'role': 'super-admin', 'disabled': False}
        editor = {'id': 'user-editor', 'username': 'editor', 'displayName': 'Editor', 'role': 'member', 'disabled': False}
        grants = {'execute': True, 'secrets': False, 'deploy': False, 'admin': True}
        identity = {'version': 1, 'ownerId': owner['id'], 'users': {u['id']: {'user': u, 'passwordHash': 'private-argon2-hash-' + u['id']} for u in [owner, editor]}, 'sessions': {'private-session-hash': {'userId': owner['id'], 'expiresAt': '2027-01-01T00:00:00Z'}}, 'teams': {TEAM: {'team': {'id': TEAM, 'name': 'Recovery team', 'revision': 3}, 'members': {owner['id']: {'userId': owner['id'], 'role': 'owner', 'revision': 1, 'grants': grants}, editor['id']: {'userId': editor['id'], 'role': 'editor', 'revision': 2, 'grants': grants}}, 'projects': {'app': {editor['id']: {'userId': editor['id'], 'role': 'viewer', 'revision': 3, 'grants': grants}}}}}}
        put(self.state / 'control-plane/identity.json', identity)
        put(self.state / 'control-plane/managed-team.json', {'version': 1, 'teamId': TEAM, 'createdAt': '2026-10-06T00:00:00Z'})
        self.lease = {'version': 1, 'releaseVersion': '0.5.0'}
        put(self.state / hostbackup.LOCK, self.lease)
        put(self.state / self.worker / hostbackup.LOCK, self.lease)
        put(self.state / self.project / 'context.json', {'brief': {'revision': 2, 'purpose': 'Shared recovery context'}, 'design': {'revision': 3}, 'plan': {'tasks': [{'id': 'task-1', 'title': 'Restore safely'}]}})
        put(self.state / self.project / 'run.sh', b'#!/bin/sh\nprintf restored\n', 0o700)
        put(self.state / self.worker / 'sessions/application-config-app.json', {'privateKey': 'fixture-private-provider-key'})
        self.args = SimpleNamespace(state_dir=str(self.state), archive=str(self.archive), team_id=TEAM, release_version='0.5.0', docker_host='unix:///var/run/docker.sock', sha256='')
        self.docker = mock.patch.object(hostbackup.subprocess, 'run', return_value=SimpleNamespace(returncode=0, stdout=b'', stderr=b''))
        self.docker.start()
        self.addCleanup(self.docker.stop)

    def save(self):
        report = hostbackup.backup(self.args)
        self.args.sha256 = report['sha256']
        return report

    def empty(self):
        original = self.root / ('original-' + str(len(list(self.root.iterdir()))))
        self.state.rename(original)
        self.state.mkdir(mode=0o700)
        return original

    def rewrite(self, change):
        with zipfile.ZipFile(self.archive) as z:
            contents = {info.filename: z.read(info) for info in z.infolist()}
            manifest = json.loads(contents['manifest.json'])
        change(manifest, contents)
        contents['manifest.json'] = json.dumps(manifest).encode()
        with zipfile.ZipFile(self.archive, 'w') as z:
            for name, value in contents.items():
                z.writestr(name, value)
        self.archive.chmod(0o600)
        self.args.sha256 = hashlib.sha256(self.archive.read_bytes()).hexdigest()

    def test_actual_bytes_accounts_grants_context_keys_executable_restore(self):
        report = self.save()
        self.assertTrue(report['includesSecrets'])
        self.assertEqual(stat.S_IMODE(self.archive.stat().st_mode), 0o600)
        original = self.empty()
        restored = hostbackup.restore(self.args)
        self.assertFalse(restored['serviceStarted'])
        for path in original.rglob('*'):
            if path.is_file() and path.name != hostbackup.LOCK:
                target = self.state / path.relative_to(original)
                self.assertEqual(target.read_bytes(), path.read_bytes())
                self.assertEqual(stat.S_IMODE(target.stat().st_mode) & 0o077, 0)
        self.assertTrue((self.state / self.project / 'run.sh').stat().st_mode & stat.S_IXUSR)
        self.assertEqual(json.loads((self.state / 'control-plane/identity.json').read_text())['teams'][TEAM]['projects']['app']['user-editor']['role'], 'viewer')
        self.assertFalse((self.state / self.worker / hostbackup.LOCK).exists(), 'Worker process leases must be reacquired, not copied.')

    def test_root_or_orphan_worker_blocks_backup(self):
        for relative in [hostbackup.LOCK, self.worker + '/' + hostbackup.LOCK]:
            with self.subTest(path=relative), (self.state / relative).open('rb') as lock:
                fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                with self.assertRaisesRegex(hostbackup.Refusal, 'still running'):
                    self.save()
                self.assertFalse(self.archive.exists())

    def test_running_docker_writer_is_refused_for_child_or_ancestor_mount(self):
        for source in [str(self.state / self.project), str(self.root)]:
            with self.subTest(source=source):
                hostbackup.subprocess.run.side_effect = [SimpleNamespace(returncode=0, stdout=('a' * 64 + '\n').encode()), SimpleNamespace(returncode=0, stdout=json.dumps([{'Type': 'bind', 'Source': source, 'RW': True}]).encode())]
                with self.assertRaisesRegex(hostbackup.Refusal, 'writable mount'):
                    self.save()
                self.assertFalse(self.archive.exists())

    def test_docker_unavailable_refused_even_without_recorded_resources(self):
        hostbackup.subprocess.run.return_value = SimpleNamespace(returncode=1, stdout=b'', stderr=b'private daemon diagnostic')
        with self.assertRaisesRegex(hostbackup.Refusal, 'Docker inspection is unavailable'):
            self.save()
        self.assertFalse(self.archive.exists())

    def test_new_docker_writer_during_snapshot_removes_partial_archive(self):
        hostbackup.subprocess.run.side_effect = [SimpleNamespace(returncode=0, stdout=b''), SimpleNamespace(returncode=0, stdout=('a' * 64).encode()), SimpleNamespace(returncode=0, stdout=json.dumps([{'Source': str(self.state), 'RW': True}]).encode())]
        with self.assertRaisesRegex(hostbackup.Refusal, 'writable mount'):
            self.save()
        self.assertFalse(self.archive.exists())
        self.assertFalse(list(self.root.glob('.luci-backup-*')))

    def test_source_links_special_files_and_foreign_team_are_refused(self):
        for kind in ['symlink', 'hardlink', 'fifo', 'foreign-team']:
            path = self.state / ('control-plane/teams/' + OTHER if kind == 'foreign-team' else 'unsafe')
            with self.subTest(kind=kind):
                if kind == 'symlink':
                    path.symlink_to(self.root / 'external-secret')
                elif kind == 'hardlink':
                    os.link(self.state / 'control-plane/identity.json', path)
                elif kind == 'fifo':
                    os.mkfifo(path, 0o600)
                else:
                    path.mkdir(mode=0o700)
                with self.assertRaises(hostbackup.Refusal):
                    self.save()
                (path.rmdir if path.is_dir() else path.unlink)()
                self.assertFalse(self.archive.exists())

    def test_private_permissions_required(self):
        (self.state / 'control-plane/identity.json').chmod(0o644)
        with self.assertRaisesRegex(hostbackup.Refusal, 'permissions'):
            self.save()
        (self.state / 'control-plane/identity.json').chmod(0o600)
        self.save()
        self.empty()
        self.archive.chmod(0o644)
        with self.assertRaisesRegex(hostbackup.Refusal, 'permissions'):
            hostbackup.restore(self.args)

    def test_wrong_checksum_version_team_or_relocation_preserves_empty_destination(self):
        self.save()
        self.empty()
        for attr, bad in [('sha256', '0' * 64), ('release_version', '0.6.0'), ('team_id', OTHER)]:
            good = getattr(self.args, attr)
            with self.subTest(attr=attr):
                setattr(self.args, attr, bad)
                with self.assertRaises(hostbackup.Refusal):
                    hostbackup.restore(self.args)
                setattr(self.args, attr, good)
                self.assertEqual(list(self.state.iterdir()), [])
        other = self.root / 'relocated'
        other.mkdir(mode=0o700)
        self.args.state_dir = str(other)
        with self.assertRaisesRegex(hostbackup.Refusal, 'original absolute'):
            hostbackup.restore(self.args)
        self.assertEqual(list(other.iterdir()), [])

    def test_no_archive_or_state_overwrite(self):
        self.save()
        before = self.archive.read_bytes()
        with self.assertRaisesRegex(hostbackup.Refusal, 'already exists'):
            self.save()
        self.assertEqual(self.archive.read_bytes(), before)
        identity = (self.state / 'control-plane/identity.json').read_bytes()
        with self.assertRaisesRegex(hostbackup.Refusal, 'not empty'):
            hostbackup.restore(self.args)
        self.assertEqual((self.state / 'control-plane/identity.json').read_bytes(), identity)

    def test_path_traversal_unlisted_entries_and_invalid_parents_never_extract(self):
        for kind in ['traversal', 'absolute', 'unlisted', 'file-parent', 'lease', 'retired', 'duplicate']:
            with self.subTest(kind=kind):
                if self.archive.exists():
                    self.archive.unlink()
                self.save()
                def change(manifest, content):
                    if kind in ['traversal', 'absolute', 'lease', 'retired']:
                        path = {'traversal': '../outside', 'absolute': '/outside', 'lease': hostbackup.LOCK, 'retired': '.luci-host-deleted'}[kind]
                        manifest['entries'].append({'path': path, 'type': 'directory', 'mode': 0o700})
                    elif kind == 'unlisted':
                        content['state/unlisted'] = b'extra'
                    elif kind == 'file-parent':
                        manifest['entries'].append({'path': 'control-plane/identity.json/child', 'type': 'directory', 'mode': 0o700})
                    else:
                        manifest['entries'].append(manifest['entries'][0])
                self.rewrite(change)
                original = self.empty()
                with self.assertRaises(hostbackup.Refusal):
                    hostbackup.restore(self.args)
                self.assertEqual(list(self.state.iterdir()), [])
                self.state.rmdir()
                original.rename(self.state)

    def test_bad_file_hash_is_checked_before_any_state_file_is_written(self):
        self.save()
        def change(manifest, content):
            content['state/' + self.project + '/context.json'] = b'changed context'
            row = next(row for row in manifest['entries'] if row['path'] == self.project + '/context.json')
            row['bytes'] = len(b'changed context')
        self.rewrite(change)
        self.empty()
        with self.assertRaisesRegex(hostbackup.Refusal, 'SHA-256'):
            hostbackup.restore(self.args)
        self.assertEqual([p.name for p in self.state.iterdir()], [hostbackup.LOCK])

    def test_linked_destination_parent_never_receives_files(self):
        self.save()
        self.empty()
        link = self.root / 'linked'
        link.symlink_to(self.state, target_is_directory=True)
        self.args.state_dir = str(link)
        with self.assertRaises(OSError):
            hostbackup.restore(self.args)
        self.assertEqual(list(self.state.iterdir()), [])

    def test_cli_requires_acknowledgements(self):
        self.docker.stop()
        result = subprocess.run([sys.executable, str(SCRIPT), 'backup', '--state-dir', str(self.state), '--archive', str(self.archive), '--team-id', TEAM, '--release-version', '0.5.0', '--docker-host', 'unix:///var/run/docker.sock'], capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn('acknowledgements', result.stderr)
        self.assertFalse(self.archive.exists())

if __name__ == '__main__':
    unittest.main()
