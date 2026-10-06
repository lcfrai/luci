#!/usr/bin/env python3
"""Erase verified private state for one explicitly requested managed-team deletion."""
import argparse
import contextlib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import sys
import zipfile

spec = importlib.util.spec_from_file_location('luci_host_recovery', Path(__file__).with_name('managed-host-backup.py'))
recovery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recovery)
TOMBSTONE = '.luci-host-deleted'
REQUEST = re.compile(r'delete-[A-Za-z0-9_-]{22}\Z')

@contextlib.contextmanager
def verified_archive(args):
    recovery.require(isinstance(args.sha256, str) and recovery.SHA.fullmatch(args.sha256), 'A trusted backup SHA-256 is required before state erasure.')
    archive_path = recovery.absolute(args.archive)
    recovery.require(not archive_path.startswith(args.state_dir + '/'), 'The recovery archive must remain outside the state being erased.')
    parent = recovery.directory(os.path.dirname(archive_path))
    try:
        fd = os.open(os.path.basename(archive_path), os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, dir_fd=parent)
    finally:
        os.close(parent)
    with os.fdopen(fd, 'rb') as stream:
        before = os.fstat(fd)
        recovery.owned(before, private=True, regular=True)
        recovery.require(recovery.digest_stream(stream) == args.sha256, 'Backup SHA-256 does not match; no state was erased.')
        stream.seek(0)
        with zipfile.ZipFile(stream) as archive:
            manifest, entries, size, evidence = recovery.validate_manifest(archive, args)
            for path, row in entries.items():
                if row['type'] == 'file':
                    with archive.open('state/' + path) as source:
                        recovery.require(recovery.digest_stream(source) == row['sha256'], 'An archived file does not match its recorded checksum.')
            recovery.require(recovery.same(before, os.fstat(fd)), 'Backup changed during verification.')
            yield manifest, entries, size, evidence

def pending_request(identity, team_id, request_id):
    lifecycle = identity['teams'][team_id].get('dataLifecycle')
    recovery.require(isinstance(lifecycle, dict) and lifecycle.get('status') == 'requested' and lifecycle.get('requestId') == request_id and type(lifecycle.get('revision')) is int and lifecycle['revision'] >= 1, 'No matching pending owner deletion request exists. A missing, changed or cancelled request cannot be processed.')
    recovery.require(lifecycle.get('requestedBy') in identity['users'] and bool(lifecycle.get('requestedAt')), 'Deletion request attribution is incomplete.')
    actor = lifecycle['requestedBy']
    member = identity['teams'][team_id]['members'].get(actor)
    recovery.require(not identity['users'][actor]['user'].get('disabled') and isinstance(member, dict) and member.get('role') == 'owner', 'The requester must still be an enabled owner of this team. A current owner must submit a new deletion request.')


def write_receipt(fd, receipt):
    os.lseek(fd, 0, os.SEEK_SET)
    os.ftruncate(fd, 0)
    data = recovery.json_bytes(receipt)
    while data:
        written = os.write(fd, data)
        data = data[written:]
    os.fsync(fd)


def erase(args):
    with contextlib.closing(recovery.Tree(args.state_dir)) as tree:
        tree.lease(recovery.LOCK)
        recovery.require(TOMBSTONE not in os.listdir(tree.fd), 'State erasure was already started. Inspect the retained tombstone and private receipt; automatic retry is disabled.')
        lease = recovery.decode(tree.read(recovery.LOCK, 4096, private=True))
        recovery.require(lease == {'version': 1, 'releaseVersion': args.release_version}, 'State lease does not match the requested release version.')
        # Inventory refuses symlinks, foreign ownership and any incomplete restore.
        snapshot = tree.inventory()
        recovery.require(TOMBSTONE not in snapshot, 'State erasure was already started. Inspect the retained tombstone and private receipt; automatic retry is disabled.')
        identity = recovery.decode(tree.read('control-plane/identity.json', private=True))
        binding = recovery.decode(tree.read('control-plane/managed-team.json', 4096, private=True))
        recovery.identity_check(binding, identity, args.team_id)
        pending_request(identity, args.team_id, args.request_id)
        with verified_archive(args) as (_, entries, size, evidence):
            recovery.require(set(snapshot) == set(entries), 'Current state differs from the backup inventory. Create a new stopped-host backup after the deletion request.')
            for path, row in entries.items():
                info = snapshot[path]
                is_dir = stat.S_ISDIR(info.st_mode)
                recovery.require(is_dir == (row['type'] == 'directory'), 'Current state type differs from its backup.')
                if not is_dir:
                    recovery.require(info.st_size == row['bytes'] and stat.S_IMODE(info.st_mode) & 0o700 == row['mode'], 'Current state size or permissions differ from its backup.')
                    fd = tree.open(path)
                    with os.fdopen(fd, 'rb') as source:
                        recovery.require(recovery.same(info, os.fstat(fd)) and recovery.digest_stream(source) == row['sha256'] and recovery.same(info, os.fstat(fd)), 'Current state bytes differ from the backup; no state was erased.')
            current = tree.inventory()
            recovery.require(set(current) == set(snapshot) and all(recovery.same(snapshot[p], current[p]) for p in current), 'State changed during backup comparison.')
            recovery.docker_quiet(args.state_dir, args.docker_host, evidence)
            receipt_path = recovery.absolute(args.receipt)
            recovery.require(not receipt_path.startswith(args.state_dir + '/') and receipt_path != args.archive, 'Retain the private erasure receipt outside the state root and backup archive.')
            parent = recovery.directory(os.path.dirname(receipt_path))
            try:
                recovery.owned(os.fstat(parent))
                receipt_fd = os.open(os.path.basename(receipt_path), os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=parent)
                os.fsync(parent)
            finally:
                os.close(parent)
            receipt = {'version': 1, 'status': 'state-erasure-in-progress', 'stateRoot': args.state_dir, 'teamId': args.team_id, 'requestId': args.request_id, 'releaseVersion': args.release_version, 'backupSHA256': args.sha256, 'stateEntries': len(entries), 'stateBytes': size, 'resourceCleanupComplete': False, 'retained': ['minimal private tombstone and root process lease', 'this private operator receipt', 'backup archives and off-host replicas', *recovery.EXCLUDED]}
            with os.fdopen(receipt_fd, 'r+b'):
                write_receipt(receipt_fd, receipt)
                # This barrier survives a crash and prevents automatic fresh bootstrap.
                marker = tree.open(TOMBSTONE, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
                with os.fdopen(marker, 'wb') as stream:
                    stream.write(recovery.json_bytes({'version': 1, 'requestId': args.request_id, 'backupSHA256': args.sha256}))
                    stream.flush()
                    os.fsync(marker)
                os.fsync(tree.fd)
                try:
                    files = [p for p, row in entries.items() if row['type'] == 'file']
                    # Keep current request/binding available until other state bytes are gone.
                    files.sort(key=lambda p: (p in ('control-plane/identity.json', 'control-plane/managed-team.json'), p))
                    for path in files:
                        parent_fd = tree.parent(path)
                        try:
                            observed = os.stat(path.split('/')[-1], dir_fd=parent_fd, follow_symlinks=False)
                            recovery.require(recovery.same(snapshot[path], observed), 'A state file changed immediately before erasure; operator inspection is required.')
                            os.unlink(path.split('/')[-1], dir_fd=parent_fd)
                        finally:
                            os.close(parent_fd)
                    for path, held in tree.locks.items():
                        if path == recovery.LOCK:
                            continue
                        parent_fd = tree.parent(path)
                        try:
                            observed = os.stat(recovery.LOCK, dir_fd=parent_fd, follow_symlinks=False)
                            expected = os.fstat(held)
                            recovery.require((observed.st_dev, observed.st_ino) == (expected.st_dev, expected.st_ino), 'A worker lease changed during erasure.')
                            os.unlink(recovery.LOCK, dir_fd=parent_fd)
                        finally:
                            os.close(parent_fd)
                    for path in sorted((p for p, row in entries.items() if row['type'] == 'directory'), key=lambda p: len(p.split('/')), reverse=True):
                        parent_fd = tree.parent(path)
                        try:
                            observed = os.stat(path.split('/')[-1], dir_fd=parent_fd, follow_symlinks=False)
                            expected = snapshot[path]
                            recovery.require(stat.S_ISDIR(observed.st_mode) and (observed.st_dev, observed.st_ino) == (expected.st_dev, expected.st_ino), 'A state directory changed during erasure.')
                            os.rmdir(path.split('/')[-1], dir_fd=parent_fd)
                        finally:
                            os.close(parent_fd)
                    recovery.docker_quiet(args.state_dir, args.docker_host, evidence)
                    recovery.require(set(os.listdir(tree.fd)) == {recovery.LOCK, TOMBSTONE}, 'Unexpected state remains; operator inspection is required.')
                    os.fsync(tree.fd)
                    receipt['status'] = 'state-erased'
                    write_receipt(receipt_fd, receipt)
                    return receipt
                except BaseException:
                    receipt['status'] = 'state-erasure-incomplete'
                    write_receipt(receipt_fd, receipt)
                    raise


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ['state-dir', 'team-id', 'request-id', 'release-version', 'archive', 'sha256', 'docker-host', 'receipt']:
        parser.add_argument('--' + key, required=True)
    parser.add_argument('--acknowledge-state-erasure', action='store_true')
    args = parser.parse_args(argv)
    try:
        recovery.require(sys.platform == 'linux', 'Managed host state erasure currently supports Linux only.')
        recovery.absolute(args.state_dir)
        recovery.require(args.acknowledge_state_erasure, 'Explicit state-erasure acknowledgement is required.')
        recovery.require(recovery.TEAM.fullmatch(args.team_id) and REQUEST.fullmatch(args.request_id) and recovery.VERSION.fullmatch(args.release_version), 'Team ID, deletion request ID or release version is invalid.')
        print(json.dumps(erase(args), indent=2))
        return 0
    except (recovery.Refusal, OSError, zipfile.BadZipFile, RuntimeError, ValueError) as error:
        print(f'State erasure refused or interrupted: {error}', file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(main())
