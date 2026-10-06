#!/usr/bin/env python3
"""Destructive operations here are restricted to fresh TemporaryDirectory fixtures."""
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
from types import SimpleNamespace
import unittest
from unittest import mock

def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value

delete = module('hostdelete', 'managed-host-delete.py')
fixture = module('hostrecoverytests', 'test-managed-host-backup.py')
REQUEST = 'delete-abcdefghijklmnopqrstuv'

class HostDeletion(unittest.TestCase):
    def setUp(self):
        fixture.HostRecovery.setUp(self)
        self.args.request_id = REQUEST
        self.args.receipt = str(self.root / 'private-deletion-receipt.json')
        identity_path = self.state / 'control-plane/identity.json'
        identity = json.loads(identity_path.read_text())
        identity['teams'][fixture.TEAM]['dataLifecycle'] = {'revision': 1, 'status': 'requested', 'requestId': REQUEST, 'requestedBy': 'user-owner', 'requestedAt': '2026-10-06T00:00:00Z', 'updatedAt': '2026-10-06T00:00:00Z'}
        fixture.put(identity_path, identity)

    def save(self):
        return fixture.HostRecovery.save(self)

    def unchanged(self):
        self.assertTrue((self.state / 'control-plane/identity.json').exists())
        self.assertTrue((self.state / self.project / 'context.json').exists())
        self.assertFalse((self.state / delete.TOMBSTONE).exists())

    def test_erases_only_backed_up_requested_state_and_retains_private_proof(self):
        self.save()
        backup = self.archive.read_bytes()
        outside = self.root / 'unrelated-private-file'
        outside.write_text('outside must stay')
        result = delete.erase(self.args)
        self.assertEqual(result['status'], 'state-erased')
        self.assertFalse(result['resourceCleanupComplete'])
        self.assertEqual(result['requestId'], REQUEST)
        self.assertEqual(set(p.name for p in self.state.iterdir()), {delete.TOMBSTONE, delete.recovery.LOCK})
        self.assertEqual(self.archive.read_bytes(), backup)
        self.assertEqual(outside.read_text(), 'outside must stay')
        receipt = Path(self.args.receipt)
        self.assertEqual(stat.S_IMODE(receipt.stat().st_mode), 0o600)
        self.assertEqual(json.loads(receipt.read_text()), result)
        self.assertNotIn('private-provider-key', receipt.read_text())
        self.assertNotIn('Owner', receipt.read_text())

    def test_missing_cancelled_or_different_request_never_erases(self):
        original = json.loads((self.state / 'control-plane/identity.json').read_text())
        for change in ['missing', 'cancelled', 'different']:
            with self.subTest(change=change):
                identity = json.loads(json.dumps(original))
                lifecycle = identity['teams'][fixture.TEAM]['dataLifecycle']
                if change == 'missing':
                    del identity['teams'][fixture.TEAM]['dataLifecycle']
                elif change == 'cancelled':
                    lifecycle['status'] = 'cancelled'
                else:
                    lifecycle['requestId'] = 'delete-vutsrqponmlkjihgfedcba'
                fixture.put(self.state / 'control-plane/identity.json', identity)
                with self.assertRaisesRegex(delete.recovery.Refusal, 'pending owner deletion request'):
                    delete.erase(self.args)
                self.unchanged()

    def test_disabled_removed_or_demoted_requester_cannot_reauthorize_with_fresh_backup(self):
        original = json.loads((self.state / 'control-plane/identity.json').read_text())
        for change in ['disabled', 'removed', 'demoted']:
            with self.subTest(change=change):
                identity = json.loads(json.dumps(original))
                members = identity['teams'][fixture.TEAM]['members']
                members['user-editor']['role'] = 'owner'
                identity['users']['user-editor']['user']['role'] = 'super-admin'
                if change == 'disabled':
                    identity['users']['user-owner']['user']['disabled'] = True
                elif change == 'removed':
                    del members['user-owner']
                else:
                    members['user-owner']['role'] = 'viewer'
                fixture.put(self.state / 'control-plane/identity.json', identity)
                if self.archive.exists():
                    self.archive.unlink()
                self.save()
                with self.assertRaisesRegex(delete.recovery.Refusal, 'still be an enabled owner'):
                    delete.erase(self.args)
                self.unchanged()

    def test_cancellation_after_backup_cannot_be_ignored(self):
        self.save()
        identity = json.loads((self.state / 'control-plane/identity.json').read_text())
        identity['teams'][fixture.TEAM]['dataLifecycle']['status'] = 'cancelled'
        fixture.put(self.state / 'control-plane/identity.json', identity)
        with self.assertRaisesRegex(delete.recovery.Refusal, 'pending owner deletion request'):
            delete.erase(self.args)
        self.unchanged()

    def test_new_or_changed_state_after_backup_blocks_erasure(self):
        self.save()
        target = self.state / self.project / 'context.json'
        before = target.read_bytes()
        target.write_bytes(before.replace(b'Shared', b'Edited'))
        with self.assertRaisesRegex(delete.recovery.Refusal, 'bytes differ'):
            delete.erase(self.args)
        self.unchanged()
        target.write_bytes(before)
        extra = self.state / 'new-private-token'
        extra.write_text('new state')
        with self.assertRaisesRegex(delete.recovery.Refusal, 'inventory'):
            delete.erase(self.args)
        self.unchanged()

    def test_root_and_orphan_worker_leases_block_erasure(self):
        self.save()
        for relative in [delete.recovery.LOCK, self.worker + '/' + delete.recovery.LOCK]:
            with self.subTest(path=relative), (self.state / relative).open('rb') as lock:
                fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                with self.assertRaisesRegex(delete.recovery.Refusal, 'still running'):
                    delete.erase(self.args)
                self.unchanged()

    def test_writable_docker_mount_or_unavailable_engine_blocks_erasure(self):
        self.save()
        with mock.patch.object(delete.recovery.subprocess, 'run', side_effect=[SimpleNamespace(returncode=0, stdout=('a'*64).encode()), SimpleNamespace(returncode=0, stdout=json.dumps([{'Source': str(self.state), 'RW': True}]).encode())]):
            with self.assertRaisesRegex(delete.recovery.Refusal, 'writable mount'):
                delete.erase(self.args)
        self.unchanged()
        with mock.patch.object(delete.recovery.subprocess, 'run', return_value=SimpleNamespace(returncode=1, stdout=b'')):
            with self.assertRaisesRegex(delete.recovery.Refusal, 'Docker inspection'):
                delete.erase(self.args)
        self.unchanged()

    def test_bad_backup_or_foreign_archive_never_erases(self):
        self.save()
        good = self.args.sha256
        self.args.sha256 = '0' * 64
        with self.assertRaisesRegex(delete.recovery.Refusal, 'SHA-256'):
            delete.erase(self.args)
        self.unchanged()
        self.args.sha256 = good
        self.args.release_version = '0.6.0'
        with self.assertRaisesRegex(delete.recovery.Refusal, 'release version'):
            delete.erase(self.args)
        self.unchanged()

    def test_existing_receipt_is_not_overwritten(self):
        self.save()
        Path(self.args.receipt).write_text('retain previous operator evidence')
        with self.assertRaises(FileExistsError):
            delete.erase(self.args)
        self.assertEqual(Path(self.args.receipt).read_text(), 'retain previous operator evidence')
        self.unchanged()

    def test_source_symlink_never_erases_its_target(self):
        self.save()
        target = self.root / 'external-secret'
        target.write_text('keep')
        (self.state / 'link').symlink_to(target)
        with self.assertRaises(delete.recovery.Refusal):
            delete.erase(self.args)
        self.assertEqual(target.read_text(), 'keep')
        self.unchanged()

    def test_interruption_retains_fail_closed_marker_receipt_and_recovery_archive(self):
        self.save()
        original_unlink = os.unlink
        def fail(name, *args, **kwargs):
            if name == 'run.sh':
                raise OSError('simulated storage failure')
            return original_unlink(name, *args, **kwargs)
        with mock.patch.object(delete.os, 'unlink', side_effect=fail):
            with self.assertRaisesRegex(OSError, 'simulated storage failure'):
                delete.erase(self.args)
        self.assertTrue((self.state / delete.TOMBSTONE).exists())
        self.assertEqual(json.loads(Path(self.args.receipt).read_text())['status'], 'state-erasure-incomplete')
        self.assertEqual(hashlib.sha256(self.archive.read_bytes()).hexdigest(), self.args.sha256)
        with self.assertRaisesRegex(delete.recovery.Refusal, 'already started'):
            delete.erase(self.args)

if __name__ == '__main__':
    unittest.main()
