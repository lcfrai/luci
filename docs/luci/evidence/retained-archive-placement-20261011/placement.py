#!/usr/bin/env python3
"""Place four pinned retained archives on /tmp; run only with explicit --apply."""
import argparse
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import stat
import sys
import time
import uuid

SOURCE_ROOT = Path('/workspace/luci/artifacts/releases')
TARGET_PARENT = Path('/tmp/luci-retained-tools')
TARGET_NAME = 'release-archives-299-b94'
RECEIPT_PARENT = Path('/workspace/.luci-tools/release-archive-placement-299-b94/operations')
RESERVE = 400 * 1024 * 1024
CHUNK = 1024 * 1024
RELEASES = [
    {'directory': '0.5.0-29931afc-preserved-before-b94ad37e', 'short': '29931afc',
     'commit': '29931afc5b31ae9b9bd7b8973a7e3fdb840609a3',
     'manifest': 'fa68f997a3f1db984b1fd6e6284e3e0d36c11e7ee9148eb977c99cfb095e7b77',
     'archives': [
         ('luci-cloud-linux-amd64.tar.gz', 34651655, '85d91fd12ece7d6edbaa4ec5d364526ee0c73c4af731202c366a4a1b38f2c5da'),
         ('luci-linux-amd64.tar.gz', 69117092, '0b6eb1719c1ce4051eeb1cec55eef8625961f79cd2ffc1e14fb953a568fcefa0')]},
    {'directory': '0.5.0-b94ad37e-preserved-before-78665270', 'short': 'b94ad37e',
     'commit': 'b94ad37e53181e360ac8910ee980570de622728c',
     'manifest': '10c16c8668e677937c5f7595307bb240daa77063c26c36b105f806ebcaa76983',
     'archives': [
         ('luci-cloud-linux-amd64.tar.gz', 34672996, 'db12693db126c858cfeb5214cdbcc70af1d5b56fbc8aa1469ccbc234c2a81b1b'),
         ('luci-linux-amd64.tar.gz', 69160779, '4a5ab15e2cbb926f6ad70609507429641dc65ee4719ef462c50df96f04cffe0f')]},
]
TOTAL = sum(item[1] for release in RELEASES for item in release['archives'])


def require(value, message):
    if not value:
        raise RuntimeError(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def identity(s):
    return (s.st_dev, s.st_ino, s.st_mode, s.st_uid, s.st_nlink,
            s.st_size, s.st_mtime_ns, s.st_ctime_ns)


def open_directory(path):
    require(str(path.resolve(strict=True)) == str(path), 'Directory contains a symlink')
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    s = os.fstat(fd)
    try:
        require(stat.S_ISDIR(s.st_mode) and s.st_uid == os.getuid() and
                stat.S_IMODE(s.st_mode) == 0o700, 'Expected owned private directory')
        require((s.st_dev, s.st_ino) == (path.lstat().st_dev, path.lstat().st_ino), 'Directory identity changed')
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        return fd
    except BaseException:
        os.close(fd)
        raise


def regular(fd, expected_size):
    s = os.fstat(fd)
    require(stat.S_ISREG(s.st_mode) and s.st_uid == os.getuid() and s.st_nlink == 1 and
            stat.S_IMODE(s.st_mode) == 0o600 and s.st_size == expected_size,
            'Expected exact owned mode0600 single-link regular file')
    return identity(s)


def named_unchanged(fd, directory_fd, name, pinned):
    require(identity(os.fstat(fd)) == pinned and
            identity(os.stat(name, dir_fd=directory_fd, follow_symlinks=False)) == pinned,
            'Pinned source file changed')


def free_bytes(fd):
    s = os.fstatvfs(fd)
    return s.f_bavail * s.f_frsize


def write_all(fd, data):
    view = memoryview(data)
    while view:
        written = os.write(fd, view)
        require(written > 0, 'File write made no progress')
        view = view[written:]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--expected-script-sha256', required=True)
    args = parser.parse_args()
    require(args.apply, 'Explicit --apply required; preparation performs no placement')
    require(sys.platform == 'linux' and os.getuid() != 0, 'Ordinary owning Linux user required')
    script = Path(__file__).resolve(strict=True)
    script_hash = digest(script.read_bytes())
    require(script_hash == args.expected_script_sha256, 'Reviewed script pin does not match')
    operation = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ-') + uuid.uuid4().hex
    opened = []
    source_files = []
    manifests = []
    receipt = {'version': 1, 'operation': operation, 'scriptSHA256': script_hash,
               'startedAt': utc(), 'passed': False, 'totalArchiveBytes': TOTAL,
               'targetReserveBytes': RESERVE, 'minimumInitialTargetFreeBytes': TOTAL + RESERVE,
               'placed': [], 'temporarySymlinks': [], 'scope': 'Only four pinned generated archive paths; no manifest/receipt writes, archive extraction, Docker/config access or other cleanup.'}
    deadline = time.monotonic() + 180
    journal_fd = None
    operation_fd = None
    operation_path = None
    events = 0

    def check_time():
        require(time.monotonic() < deadline, 'Cooperative180s placement deadline exceeded')

    def event(kind, **fields):
        nonlocal events
        events += 1
        require(events <= 64, 'Operation journal event bound')
        data = (json.dumps({'at': utc(), 'kind': kind, **fields}, separators=(',', ':')) + '\n').encode()
        require(len(data) <= 16384, 'Operation journal row bound')
        write_all(journal_fd, data)
        os.fsync(journal_fd)

    def verify_bytes(fd, count):
        h = hashlib.sha256()
        os.lseek(fd, 0, os.SEEK_SET)
        size = 0
        while True:
            check_time()
            data = os.read(fd, min(CHUNK, count - size + 1))
            if not data:
                break
            size += len(data)
            require(size <= count, 'Archive exceeds pinned size')
            h.update(data)
        require(size == count, 'Archive shorter than pinned size')
        return h.hexdigest()

    try:
        receipts_fd = open_directory(RECEIPT_PARENT)
        opened.append(receipts_fd)
        os.mkdir(operation, mode=0o700, dir_fd=receipts_fd)
        os.fsync(receipts_fd)
        operation_path = RECEIPT_PARENT / operation
        operation_fd = open_directory(operation_path)
        opened.append(operation_fd)
        journal_fd = os.open('events.jsonl', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                             0o600, dir_fd=operation_fd)
        opened.append(journal_fd)
        os.fsync(operation_fd)  # Durably name the journal before any archive operation.
        event('operation-started', sourceRoot=str(SOURCE_ROOT), target=str(TARGET_PARENT / TARGET_NAME))
        source_root_fd = open_directory(SOURCE_ROOT)
        target_parent_fd = open_directory(TARGET_PARENT)
        opened.extend([source_root_fd, target_parent_fd])
        require(os.fstat(source_root_fd).st_dev != os.fstat(target_parent_fd).st_dev,
                'Placement target must be a different filesystem')
        require(not os.path.lexists(TARGET_PARENT / TARGET_NAME), 'Target must be fresh and absent')
        receipt['freeBefore'] = {'sourceBytes': free_bytes(source_root_fd), 'targetBytes': free_bytes(target_parent_fd)}
        require(receipt['freeBefore']['targetBytes'] >= TOTAL + RESERVE, 'Target lacks archive bytes plus400MiB reserve')

        # Validate all manifests and retain every exact original descriptor before copying.
        for release in RELEASES:
            directory = SOURCE_ROOT / release['directory']
            directory_fd = open_directory(directory)
            opened.append(directory_fd)
            mf = os.open('release-manifest.json', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=directory_fd)
            opened.append(mf)
            pin = regular(mf, 696)
            data = os.read(mf, 697)
            require(len(data) == 696 and digest(data) == release['manifest'], 'Pinned release manifest changed')
            value = json.loads(data)
            require(value['sourceCommit'] == release['commit'] and value['sourceDirty'] is False and
                    value['version'] == '0.5.0', 'Unexpected release provenance')
            expected = [{'name': name, 'bytes': size, 'sha256': checksum} for name, size, checksum in release['archives']]
            require(value['artifacts'] == expected, 'Manifest archive identities differ')
            named_unchanged(mf, directory_fd, 'release-manifest.json', pin)
            manifests.append((mf, directory_fd, pin, release))
            for name, size, checksum in release['archives']:
                fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=directory_fd)
                opened.append(fd)
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                pin = regular(fd, size)
                named_unchanged(fd, directory_fd, name, pin)
                source_files.append({'fd': fd, 'directoryFD': directory_fd, 'directory': directory,
                                     'name': name, 'size': size, 'sha256': checksum, 'pin': pin,
                                     'release': release['short']})
        event('all-originals-pinned', archives=4, manifests=2, free=receipt['freeBefore'])
        check_time()
        os.mkdir(TARGET_NAME, mode=0o700, dir_fd=target_parent_fd)
        os.fsync(target_parent_fd)
        target_root_fd = open_directory(TARGET_PARENT / TARGET_NAME)
        opened.append(target_root_fd)
        target_dirs = {}
        for release in RELEASES:
            os.mkdir(release['short'], mode=0o700, dir_fd=target_root_fd)
            fd = open_directory(TARGET_PARENT / TARGET_NAME / release['short'])
            opened.append(fd)
            target_dirs[release['short']] = fd
        os.fsync(target_root_fd)

        # Copy all four first. A failure here leaves every original path regular.
        remaining = TOTAL
        for row in source_files:
            check_time()
            require(free_bytes(target_parent_fd) >= remaining + RESERVE, 'Target reserve changed during copy')
            named_unchanged(row['fd'], row['directoryFD'], row['name'], row['pin'])
            target_dir_fd = target_dirs[row['release']]
            destination_fd = os.open(row['name'], os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                                     0o600, dir_fd=target_dir_fd)
            opened.append(destination_fd)
            row['targetFD'] = destination_fd
            row['targetDirectoryFD'] = target_dir_fd
            row['target'] = TARGET_PARENT / TARGET_NAME / row['release'] / row['name']
            source_hash = hashlib.sha256()
            copied = 0
            while copied < row['size']:
                check_time()
                data = os.read(row['fd'], min(CHUNK, row['size'] - copied))
                require(bool(data), 'Original archive truncated during copy')
                write_all(destination_fd, data)
                source_hash.update(data)
                copied += len(data)
            require(os.read(row['fd'], 1) == b'' and source_hash.hexdigest() == row['sha256'], 'Original archive hash/size mismatch')
            named_unchanged(row['fd'], row['directoryFD'], row['name'], row['pin'])
            os.fsync(destination_fd)
            row['targetPin'] = regular(destination_fd, row['size'])
            require(verify_bytes(destination_fd, row['size']) == row['sha256'], 'Independent copied archive hash mismatch')
            named_unchanged(destination_fd, target_dir_fd, row['name'], row['targetPin'])
            os.fsync(target_dir_fd)
            remaining -= row['size']
            event('copy-verified', source=str(row['directory'] / row['name']), target=str(row['target']),
                  bytes=copied, sourceSHA256=source_hash.hexdigest(), copiedSHA256=row['sha256'])
        require(free_bytes(target_parent_fd) >= RESERVE, 'Final target reserve below400MiB')
        for row in source_files:
            named_unchanged(row['fd'], row['directoryFD'], row['name'], row['pin'])
            named_unchanged(row['targetFD'], row['targetDirectoryFD'], row['name'], row['targetPin'])
        event('all-four-copies-verified-before-replacement')

        # Each original path changes atomically; the four-path operation is not a transaction.
        # No automatic rollback or deletion: partial completion preserves all verified copies.
        for row in source_files:
            check_time()
            for mf, directory_fd, pin, release in manifests:
                named_unchanged(mf, directory_fd, 'release-manifest.json', pin)
            named_unchanged(row['fd'], row['directoryFD'], row['name'], row['pin'])
            named_unchanged(row['targetFD'], row['targetDirectoryFD'], row['name'], row['targetPin'])
            temporary = '.' + row['name'] + '.' + operation + '.placement-link'
            os.symlink(str(row['target']), temporary, dir_fd=row['directoryFD'])
            temp_path = str(row['directory'] / temporary)
            receipt['temporarySymlinks'].append(temp_path)
            os.fsync(row['directoryFD'])
            named_unchanged(row['fd'], row['directoryFD'], row['name'], row['pin'])
            require(os.readlink(temporary, dir_fd=row['directoryFD']) == str(row['target']), 'Temporary symlink changed')
            os.replace(temporary, row['name'], src_dir_fd=row['directoryFD'], dst_dir_fd=row['directoryFD'])
            os.fsync(row['directoryFD'])
            receipt['temporarySymlinks'].remove(temp_path)
            require(stat.S_ISLNK(os.stat(row['name'], dir_fd=row['directoryFD'], follow_symlinks=False).st_mode), 'Original path is not a symlink')
            require(os.readlink(row['name'], dir_fd=row['directoryFD']) == str(row['target']), 'Placed symlink target differs')
            require(identity(os.stat(row['name'], dir_fd=row['directoryFD'])) == row['targetPin'], 'Original path does not resolve to verified copy')
            placed = {'originalPath': str(row['directory'] / row['name']), 'target': str(row['target']),
                      'bytes': row['size'], 'sha256': row['sha256'], 'originalPathAccessible': True}
            receipt['placed'].append(placed)
            event('original-path-atomically-replaced', **placed)
            os.close(row['fd'])
            opened.remove(row['fd'])  # Release the old inode only after verified accessible placement.
        for mf, directory_fd, pin, release in manifests:
            named_unchanged(mf, directory_fd, 'release-manifest.json', pin)
            require(verify_bytes(mf, 696) == release['manifest'], 'Release manifest bytes changed')
        receipt['manifestsUnchanged'] = True
        receipt['freeAfter'] = {'sourceBytes': free_bytes(source_root_fd), 'targetBytes': free_bytes(target_parent_fd)}
        receipt['passed'] = len(receipt['placed']) == 4 and not receipt['temporarySymlinks']
        event('placement-completed', placed=len(receipt['placed']), free=receipt['freeAfter'])
    except BaseException as error:
        receipt['passed'] = False
        receipt['failure'] = {'type': type(error).__name__, 'message': str(error)[:1024]}
        if journal_fd is not None:
            try:
                event('placement-failed', failure=receipt['failure'], placed=len(receipt['placed']))
            except BaseException:
                receipt['failureJournalUnconfirmed'] = True
    finally:
        # Settle ordinary handles before freezing the outcome in the result.
        # Keep only the receipt directory open for the final durable write.
        close_failures = []
        for fd in reversed(opened):
            if fd == operation_fd:
                continue
            try:
                os.close(fd)
            except OSError as error:
                close_failures.append({'errno': error.errno, 'type': type(error).__name__})
        opened = [operation_fd] if operation_fd is not None else []
        if close_failures:
            receipt['passed'] = False
            receipt['descriptorCloseFailures'] = close_failures
        receipt['endedAt'] = utc()
        if operation_fd is not None:
            try:
                data = (json.dumps(receipt, indent=2) + '\n').encode()
                require(len(data) <= 65536, 'Final receipt size bound')
                fd = os.open('result.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                             0o600, dir_fd=operation_fd)
                try:
                    write_all(fd, data)
                    os.fsync(fd)
                finally:
                    os.close(fd)
                os.fsync(operation_fd)
            except BaseException as error:
                receipt['passed'] = False
                receipt['receiptFailure'] = type(error).__name__
        else:
            receipt['passed'] = False
        for fd in reversed(opened):
            try:
                os.close(fd)
            except OSError as error:
                receipt['passed'] = False
                receipt['receiptFailure'] = 'receipt-directory-close:' + str(error.errno)
    print(json.dumps({'passed': receipt['passed'], 'receipt': str(operation_path / 'result.json') if operation_path else None,
                      'placed': len(receipt['placed']), 'failure': receipt.get('failure'),
                      'receiptFailure': receipt.get('receiptFailure')}))
    return 0 if receipt['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
