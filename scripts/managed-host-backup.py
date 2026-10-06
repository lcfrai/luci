#!/usr/bin/env python3
"""Offline, private recovery of one Linux managed-team installation (stdlib only)."""
from __future__ import annotations
import argparse
import contextlib
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import PurePosixPath
import re
import shutil
import stat
import subprocess
import sys
import uuid
import zipfile

LOCK = '.luci-state.lock'
INCOMPLETE = '.luci-host-restore-incomplete'
FORMAT = 'luci-managed-host'
MAX_ENTRIES = 250_000
MAX_BYTES = 64 * 1024**3
MAX_MANIFEST = 64 * 1024**2
BLOCK = 1024**2
TEAM = re.compile(r'team-[A-Za-z0-9_-]{22}\Z')
VERSION = re.compile(r'[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?\Z')
SHA = re.compile(r'[a-f0-9]{64}\Z')
EXCLUDED = ['process lease files', 'Docker volumes and images', 'external databases and services', 'personal projects and other files outside the state directory', 'host environment, systemd units and TLS/SSH configuration outside the state directory']

class Refusal(Exception):
    pass

def require(ok, message):
    if not ok:
        raise Refusal(message)

def absolute(path):
    require(isinstance(path, str) and path.startswith('/') and path != '/' and os.path.normpath(path) == path and '\x00' not in path, 'Use a canonical absolute path other than /.')
    return path

def relative(path):
    require(isinstance(path, str) and path and '\\' not in path and '\x00' not in path and not path.startswith('/') and all(p not in ('', '.', '..') for p in path.split('/')), 'Archive contains an unsafe path.')
    require(len(path.encode('utf-8')) <= 4096 and all(ord(c) >= 32 for c in path), 'Archive path is invalid or too long.')
    require(not ({LOCK, INCOMPLETE, '.luci-host-deleted'} & set(path.split('/'))), 'Archive must not contain process lease, incomplete-restore or retired-host markers.')
    return path

def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, 'JSON contains duplicate fields.')
        result[key] = value
    return result

def decode(data):
    try:
        return json.loads(data, object_pairs_hook=unique_object)
    except (ValueError, UnicodeError) as error:
        raise Refusal('Private state or manifest contains invalid JSON.') from error

def json_bytes(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()

def directory(path):
    """Walk via directory descriptors; never resolve a symbolic-link component."""
    absolute(path)
    fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
    try:
        for part in path.strip('/').split('/'):
            new = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
            os.close(fd)
            fd = new
        return fd
    except BaseException:
        os.close(fd)
        raise

def owned(info, private=False, regular=False):
    require(info.st_uid == os.geteuid(), 'Run recovery as the service account that owns the private state/archive.')
    require(not (stat.S_IMODE(info.st_mode) & (0o077 if private else 0o022)), 'Private state/archive permissions are too broad (use 0700 directories and 0600 private files).')
    if regular:
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1, 'Only owned regular files with one hard link are supported.')

def same(a, b):
    return (a.st_dev, a.st_ino, a.st_size, a.st_mtime_ns, a.st_ctime_ns, a.st_mode) == (b.st_dev, b.st_ino, b.st_size, b.st_mtime_ns, b.st_ctime_ns, b.st_mode)

class Tree:
    def __init__(self, path):
        self.path = absolute(path)
        self.fd = directory(path)
        owned(os.fstat(self.fd), private=True)
        self.locks = {}

    def close(self):
        for fd in self.locks.values():
            os.close(fd)
        os.close(self.fd)

    def parent(self, path):
        fd = os.dup(self.fd)
        try:
            for part in path.split('/')[:-1]:
                nxt = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
                os.close(fd)
                fd = nxt
            return fd
        except BaseException:
            os.close(fd)
            raise

    def open(self, path, flags=os.O_RDONLY, mode=0o600):
        fd = self.parent(path)
        try:
            return os.open(path.split('/')[-1], flags | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, mode, dir_fd=fd)
        finally:
            os.close(fd)

    def lease(self, path, create=False):
        if path in self.locks:
            return
        fd = self.open(path, os.O_RDWR | (os.O_CREAT if create else 0))
        try:
            owned(os.fstat(fd), private=True, regular=True)
            try:
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as error:
                raise Refusal('Luci or an orphaned worker is still running, or another recovery operation holds its state lease. Stop it before retrying.') from error
            self.locks[path] = fd
        except BaseException:
            os.close(fd)
            raise

    def read(self, path, limit=8 * 1024**2, private=False):
        fd = self.open(path)
        with os.fdopen(fd, 'rb') as stream:
            info = os.fstat(fd)
            owned(info, private=private, regular=True)
            require(info.st_size <= limit, 'Private metadata exceeds its supported size.')
            value = stream.read(limit + 1)
            require(len(value) <= limit and same(info, os.fstat(fd)), 'Private metadata changed during recovery.')
            return value

    def inventory(self):
        result = {}
        size = 0
        def walk(fd, prefix=''):
            nonlocal size
            for name in sorted(os.listdir(fd)):
                path = f'{prefix}/{name}' if prefix else name
                info = os.stat(name, dir_fd=fd, follow_symlinks=False)
                if name == LOCK:
                    require(stat.S_ISREG(info.st_mode), 'State lease path is not a regular file.')
                    self.lease(path)
                    continue
                relative(path)
                owned(info)
                require(stat.S_ISREG(info.st_mode) or stat.S_ISDIR(info.st_mode), 'Symbolic links, sockets and special files are not followed or archived; remove generated links after stopping workloads, or handle that state separately.')
                if stat.S_ISREG(info.st_mode):
                    owned(info, regular=True)
                    require(info.st_mode & stat.S_IRUSR, 'A state file is not readable by its owner.')
                    size += info.st_size
                result[path] = info
                require(len(result) <= MAX_ENTRIES and size <= MAX_BYTES, 'Host state exceeds the supported 250,000 entries or 64 GiB limit.')
                if stat.S_ISDIR(info.st_mode):
                    child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
                    try:
                        require(os.fstat(child).st_ino == info.st_ino, 'State directory changed during inventory.')
                        walk(child, path)
                    finally:
                        os.close(child)
        walk(self.fd)
        return result

    def mkdir(self, path):
        fd = self.parent(path)
        try:
            os.mkdir(path.split('/')[-1], 0o700, dir_fd=fd)
        finally:
            os.close(fd)


def identity_check(binding, identity, team_id):
    require(isinstance(binding, dict) and type(binding.get('version')) is int and binding['version'] == 1 and binding.get('teamId') == team_id, 'Managed-team binding does not match the requested team.')
    try:
        datetime.datetime.fromisoformat(binding['createdAt'].replace('Z', '+00:00'))
    except (KeyError, TypeError, ValueError) as error:
        raise Refusal('Managed-team binding has an invalid timestamp.') from error
    require(isinstance(identity, dict) and identity.get('version') == 1 and isinstance(identity.get('users'), dict) and isinstance(identity.get('teams'), dict) and set(identity['teams']) == {team_id}, 'Private identity must contain exactly the bound team.')
    users = identity['users']
    require(identity.get('ownerId') in users and isinstance(identity.get('sessions'), dict), 'Private account state is incomplete.')
    for key, record in users.items():
        require(isinstance(record, dict) and isinstance(record.get('user'), dict) and record['user'].get('id') == key and isinstance(record.get('passwordHash'), str) and record['passwordHash'], 'Private account records are invalid.')
    team = identity['teams'][team_id]
    require(isinstance(team, dict) and isinstance(team.get('team'), dict) and team['team'].get('id') == team_id and isinstance(team.get('members'), dict) and isinstance(team.get('projects'), dict), 'Private team state is invalid.')
    owners = 0
    for key, member in team['members'].items():
        require(key in users and isinstance(member, dict) and member.get('userId') == key and member.get('role') in ('owner', 'editor', 'viewer'), 'Private team membership is invalid.')
        if member['role'] == 'owner' and not users[key]['user'].get('disabled'):
            owners += 1
    require(owners > 0, 'Managed team has no enabled owner.')
    for grants in team['projects'].values():
        require(isinstance(grants, dict) and all(key in team['members'] and isinstance(value, dict) and value.get('userId') == key for key, value in grants.items()), 'Private project grants are invalid.')


def resource_evidence(paths, read):
    for path in paths:
        if '/sessions/platform/' in '/' + path or path.startswith('sessions/platform/'):
            return True
        if path.endswith('/sessions/sessions.json') or path == 'sessions/sessions.json':
            value = decode(read(path))
            require(isinstance(value, dict) and isinstance(value.get('sessions'), list), 'Session resource history is invalid; Docker inspection is required.')
            if value['sessions']:
                return True
        if '/sessions/' in '/' + path and ('check-run' in path or 'check-results' in path):
            return True
    return False


def docker_quiet(state_dir, docker_host, evidence):
    require(docker_host.startswith('unix:///') and '\x00' not in docker_host and '\n' not in docker_host, 'Managed host recovery currently requires an explicit local unix Docker socket.')
    absolute(docker_host[len('unix://'):])
    env = {key: value for key, value in os.environ.items() if not key.startswith(('DOCKER_', 'LUCI_'))}
    def command(args):
        completed = subprocess.run(['docker', '--host', docker_host, *args], capture_output=True, timeout=30, env=env, check=False)
        require(completed.returncode == 0 and len(completed.stdout) <= 16 * 1024**2, 'Docker inspection is unavailable.')
        return completed.stdout
    try:
        ids = command(['ps', '--quiet', '--no-trunc']).decode().split()
        require(len(ids) <= 4096 and all(re.fullmatch('[a-f0-9]{64}', value) for value in ids), 'Docker returned an invalid running-container inventory.')
        for container in ids:
            mounts = decode(command(['inspect', '--format', '{{json .Mounts}}', container]))
            require(isinstance(mounts, list), 'Docker returned invalid mount information.')
            for mount in mounts:
                require(isinstance(mount, dict) and isinstance(mount.get('Source'), str) and type(mount.get('RW')) is bool, 'Docker returned incomplete mount information.')
                source = os.path.normpath(mount['Source'])
                # Both a direct child bind and a writable ancestor expose the state.
                if mount['RW'] and source.startswith('/') and (source == state_dir or source.startswith(state_dir + '/') or state_dir.startswith(source.rstrip('/') + '/')):
                    raise Refusal('A running Docker container has a writable mount overlapping this state directory. Stop the owning session/application before backup or restore.')
        return {'checked': True, 'runningContainersInspected': len(ids), 'dockerHost': docker_host}
    except (OSError, subprocess.TimeoutExpired, UnicodeError) as error:
        raise Refusal('Docker could not be inspected. Restore access to the managed host Docker engine and stop writers before recovery.') from error



def digest_stream(stream):
    digest = hashlib.sha256()
    size = 0
    while data := stream.read(BLOCK):
        size += len(data)
        require(size <= MAX_BYTES, 'Archive exceeds the supported 64 GiB limit.')
        digest.update(data)
    return digest.hexdigest()


def backup(args):
    with contextlib.closing(Tree(args.state_dir)) as tree:
        tree.lease(LOCK)
        require('.luci-host-deleted' not in os.listdir(tree.fd), 'Host state was marked for erasure; inspect its private operator receipt before any recovery action.')
        lease = decode(tree.read(LOCK, 4096, private=True))
        require(lease == {'version': 1, 'releaseVersion': args.release_version}, 'Start and stop this exact managed Luci release once before creating its offline backup.')
        binding = decode(tree.read('control-plane/managed-team.json', 4096, private=True))
        identity = decode(tree.read('control-plane/identity.json', private=True))
        identity_check(binding, identity, args.team_id)
        entries = tree.inventory()
        for path in entries:
            if path.startswith('control-plane/teams/'):
                require(path.split('/')[2] == args.team_id, 'State contains another team workspace.')
        evidence = resource_evidence([p for p, info in entries.items() if stat.S_ISREG(info.st_mode)], tree.read)
        observed = docker_quiet(args.state_dir, args.docker_host, evidence)
        output = absolute(args.archive)
        require(not (output == args.state_dir or output.startswith(args.state_dir + '/')), 'Write the private archive outside the state directory.')
        parent_path, name = os.path.split(output)
        parent = directory(parent_path)
        temporary = '.luci-backup-' + uuid.uuid4().hex
        try:
            owned(os.fstat(parent))
            require(name not in os.listdir(parent), 'Archive already exists; refusing to overwrite it.')
            fd = os.open(temporary, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=parent)
            with os.fdopen(fd, 'w+b') as stream:
                rows = []
                with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=6, allowZip64=True) as archive:
                    for path, info in entries.items():
                        if stat.S_ISDIR(info.st_mode):
                            rows.append({'path': path, 'type': 'directory', 'mode': 0o700})
                            continue
                        row = {'path': path, 'type': 'file', 'mode': stat.S_IMODE(info.st_mode) & 0o700, 'bytes': info.st_size}
                        sha = hashlib.sha256()
                        source_fd = tree.open(path)
                        with os.fdopen(source_fd, 'rb') as source:
                            require(same(info, os.fstat(source_fd)), 'State changed after inventory; archive refused.')
                            meta = zipfile.ZipInfo('state/' + path)
                            meta.compress_type = zipfile.ZIP_DEFLATED
                            meta.external_attr = (stat.S_IFREG | 0o600) << 16
                            with archive.open(meta, 'w', force_zip64=True) as target:
                                while data := source.read(BLOCK):
                                    sha.update(data)
                                    target.write(data)
                            require(same(info, os.fstat(source_fd)), 'State changed while being archived; archive refused.')
                        row['sha256'] = sha.hexdigest()
                        rows.append(row)
                    current = tree.inventory()
                    require(set(current) == set(entries) and all(same(entries[p], current[p]) for p in current), 'State changed during backup; archive refused.')
                    docker_quiet(args.state_dir, args.docker_host, evidence)
                    manifest = {'format': FORMAT, 'version': 1, 'releaseVersion': args.release_version, 'sourceRoot': args.state_dir, 'teamId': args.team_id, 'createdAt': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'includesSecrets': True, 'excluded': EXCLUDED, 'dockerObservation': observed, 'entries': rows}
                    payload = json_bytes(manifest)
                    require(len(payload) <= MAX_MANIFEST, 'Archive manifest exceeds its supported size.')
                    archive.writestr('manifest.json', payload)
                stream.flush()
                os.fsync(fd)
                stream.seek(0)
                checksum = digest_stream(stream)
            os.link(temporary, name, src_dir_fd=parent, dst_dir_fd=parent, follow_symlinks=False)
            os.fsync(parent)
            return {'archive': output, 'sha256': checksum, 'teamId': args.team_id, 'releaseVersion': args.release_version, 'entries': len(rows), 'bytes': sum(row.get('bytes', 0) for row in rows), 'includesSecrets': True, 'excluded': EXCLUDED, 'dockerObservation': observed}
        finally:
            try:
                os.unlink(temporary, dir_fd=parent)
            except FileNotFoundError:
                pass
            os.close(parent)


def validate_manifest(archive, args):
    infos = archive.infolist()
    require(len(infos) <= MAX_ENTRIES + 1 and len({x.filename for x in infos}) == len(infos), 'Archive has duplicate or excessive entries.')
    by_name = {x.filename: x for x in infos}
    require('manifest.json' in by_name and by_name['manifest.json'].file_size <= MAX_MANIFEST, 'Archive manifest is missing or too large.')
    manifest = decode(archive.read('manifest.json'))
    require(isinstance(manifest, dict) and manifest.get('format') == FORMAT and type(manifest.get('version')) is int and manifest['version'] == 1 and manifest.get('includesSecrets') is True, 'Unsupported private host archive format.')
    require(manifest.get('releaseVersion') == args.release_version, 'Restore requires the exact archived Luci release version; upgrade separately after recovery.')
    require(manifest.get('teamId') == args.team_id, 'Archive belongs to another managed team.')
    require(manifest.get('sourceRoot') == args.state_dir, 'Restore requires the original absolute state path; automatic relocation is not supported.')
    rows = manifest.get('entries')
    require(isinstance(rows, list) and 1 <= len(rows) <= MAX_ENTRIES, 'Archive manifest entries are invalid.')
    entries, file_names, size = {}, {'manifest.json'}, 0
    for row in rows:
        require(isinstance(row, dict), 'Archive manifest entry is invalid.')
        path = relative(row.get('path'))
        require(path not in entries, 'Archive manifest repeats a path.')
        kind = row.get('type')
        require(kind in ('file', 'directory') and type(row.get('mode')) is int and row['mode'] in ((0o700,) if kind == 'directory' else (0o400, 0o500, 0o600, 0o700)), 'Archive has unsafe entry types or permissions.')
        entries[path] = row
        if path.startswith('control-plane/teams/'):
            require(path.split('/')[2] == args.team_id, 'Archive contains a foreign team workspace.')
        if kind == 'file':
            require(type(row.get('bytes')) is int and 0 <= row['bytes'] <= MAX_BYTES and isinstance(row.get('sha256'), str) and SHA.fullmatch(row['sha256']), 'Archive file checksum/size is invalid.')
            name = 'state/' + path
            info = by_name.get(name)
            require(info is not None and not info.is_dir() and info.file_size == row['bytes'] and not info.flag_bits & 1 and stat.S_IFMT(info.external_attr >> 16) in (0, stat.S_IFREG), 'Archive file metadata does not match its manifest.')
            require(info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED), 'Unsupported archive compression.')
            file_names.add(name)
            size += row['bytes']
    require(size <= MAX_BYTES and set(by_name) == file_names, 'Archive contains unlisted files or exceeds the size limit.')
    for path in entries:
        parent = str(PurePosixPath(path).parent)
        while parent != '.':
            require(parent in entries and entries[parent]['type'] == 'directory', 'Archive has a missing or non-directory parent.')
            parent = str(PurePosixPath(parent).parent)
    def read(path):
        row = entries.get(path)
        require(row is not None and row['type'] == 'file' and row['bytes'] <= 8 * 1024**2, 'Archive private metadata is missing or oversized.')
        return archive.read('state/' + path)
    identity_check(decode(read('control-plane/managed-team.json')), decode(read('control-plane/identity.json')), args.team_id)
    return manifest, entries, size, resource_evidence([path for path, row in entries.items() if row['type'] == 'file'], read)


def restore(args):
    require(isinstance(args.sha256, str) and SHA.fullmatch(args.sha256), 'Supply the archive SHA-256 recorded by backup through a trusted channel.')
    archive_path = absolute(args.archive)
    require(not archive_path.startswith(args.state_dir + '/'), 'Keep the archive outside the empty restore directory.')
    archive_parent = directory(os.path.dirname(archive_path))
    try:
        fd = os.open(os.path.basename(archive_path), os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK, dir_fd=archive_parent)
    finally:
        os.close(archive_parent)
    with os.fdopen(fd, 'rb') as stream, contextlib.closing(Tree(args.state_dir)) as tree:
        info = os.fstat(fd)
        owned(info, private=True, regular=True)
        require(digest_stream(stream) == args.sha256, 'Archive SHA-256 does not match; nothing was restored.')
        stream.seek(0)
        with zipfile.ZipFile(stream) as archive:
            manifest, entries, size, evidence = validate_manifest(archive, args)
            tree.lease(LOCK, create=True)
            require(os.listdir(tree.fd) == [LOCK], 'Restore directory is not empty; existing state is never overwritten.')
            require(shutil.disk_usage(args.state_dir).free > size + 16 * 1024**2, 'Insufficient free disk space for a safe restore.')
            docker_quiet(args.state_dir, args.docker_host, evidence)
            # Validate every byte before creating state paths. Never call extractall.
            for path, row in entries.items():
                if row['type'] == 'file':
                    with archive.open('state/' + path) as source:
                        require(digest_stream(source) == row['sha256'], 'An archived file does not match its recorded SHA-256; nothing was restored.')
            require(same(info, os.fstat(fd)), 'Archive changed during validation.')
            marker = tree.open(INCOMPLETE, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
            with os.fdopen(marker, 'wb') as output:
                output.write(json_bytes({'version': 1, 'sha256': args.sha256}))
                output.flush()
                os.fsync(marker)
            os.fsync(tree.fd)
            created = []
            try:
                for path, row in sorted(entries.items(), key=lambda item: (len(item[0].split('/')), item[0])):
                    if row['type'] == 'directory':
                        tree.mkdir(path)
                        created.append((path, True))
                    else:
                        output = tree.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL)
                        created.append((path, False))
                        with os.fdopen(output, 'wb') as target, archive.open('state/' + path) as source:
                            sha, written = hashlib.sha256(), 0
                            while data := source.read(BLOCK):
                                written += len(data)
                                require(written <= row['bytes'], 'Archive expanded beyond its manifest.')
                                sha.update(data)
                                target.write(data)
                            require(written == row['bytes'] and sha.hexdigest() == row['sha256'], 'Archive changed during restore.')
                            target.flush()
                            os.fchmod(output, row['mode'])
                            os.fsync(output)
                docker_quiet(args.state_dir, args.docker_host, evidence)
                lock = tree.locks[LOCK]
                os.ftruncate(lock, 0)
                os.write(lock, json_bytes({'version': 1, 'releaseVersion': args.release_version}))
                os.fsync(lock)
                # Fsync each directory bottom-up before reporting success.
                for path, is_dir in reversed(created):
                    if is_dir:
                        d = tree.open(path, os.O_RDONLY | os.O_DIRECTORY)
                        try:
                            os.fsync(d)
                        finally:
                            os.close(d)
                os.unlink(INCOMPLETE, dir_fd=tree.fd)
                os.fsync(tree.fd)
            except BaseException:
                # Only remove paths this invocation created, never a pre-existing file.
                for path, is_dir in reversed(created):
                    parent = tree.parent(path)
                    try:
                        (os.rmdir if is_dir else os.unlink)(path.split('/')[-1], dir_fd=parent)
                    finally:
                        os.close(parent)
                os.unlink(INCOMPLETE, dir_fd=tree.fd)
                os.fsync(tree.fd)
                raise
            return {'restored': args.state_dir, 'teamId': manifest['teamId'], 'releaseVersion': manifest['releaseVersion'], 'entries': len(entries), 'bytes': size, 'sha256': args.sha256, 'serviceStarted': False, 'excluded': EXCLUDED}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=('backup', 'restore'))
    parser.add_argument('--state-dir', required=True)
    parser.add_argument('--team-id', required=True)
    parser.add_argument('--release-version', required=True)
    parser.add_argument('--archive', required=True)
    parser.add_argument('--docker-host', required=True, help='Explicit local Docker unix socket used by this managed host.')
    parser.add_argument('--sha256', help='Required for restore; checksum recorded when backup completed.')
    parser.add_argument('--acknowledge-private-secrets', action='store_true', help='Archive contains password hashes, sessions, keys, provider credentials and private project data.')
    parser.add_argument('--acknowledge-stopped-workloads', action='store_true', help='Stop the gateway, workers and workload writers; leases and Docker mounts are checked independently.')
    args = parser.parse_args(argv)
    try:
        require(sys.platform == 'linux', 'Managed host recovery currently supports Linux only.')
        require(args.acknowledge_private_secrets and args.acknowledge_stopped_workloads, 'Both private-secrets and stopped-workloads acknowledgements are required.')
        absolute(args.state_dir)
        require(TEAM.fullmatch(args.team_id) and VERSION.fullmatch(args.release_version), 'Team ID or release version is invalid.')
        result = backup(args) if args.operation == 'backup' else restore(args)
        print(json.dumps(result, indent=2))
        return 0
    except (Refusal, OSError, zipfile.BadZipFile, RuntimeError, ValueError) as error:
        print(f'Host recovery refused: {error}', file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(main())
