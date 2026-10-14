#!/usr/bin/env python3
"""Build and restore bounded, exact Git source for Azure image builders.

This module only reads pinned local Git objects. It never fetches a repository,
uses a caller's Git configuration in the restored checkout, or runs hooks.
"""

import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import tempfile


VERSION = 1
RUNTIME_REVISION = 'cf536c284434fdd2f09bbab35ec807a5dacc87e4'
MAX_PACK = 4 << 20
MAX_MANIFEST = 2 << 20
MAX_PATHS = 2048
MAX_OBJECTS = 8192
MAX_EXPANDED = 64 << 20
MAX_FILE = 16 << 20
TIMEOUT = 120
HEX40 = re.compile(r'[0-9a-f]{40}\Z')
HEX64 = re.compile(r'[0-9a-f]{64}\Z')
SOURCE_FILES = ('LICENSE', 'NOTICE', 'go.work', 'go.work.sum')
SOURCE_PREFIXES = ('control-plane/', 'sandboxd/', 'images/microvm/', 'images/sandbox/',
                   'scripts/azure-controller-disk/', 'scripts/azure-embed-host/',
                   'third-party/licenses/')
SOURCE_EXTRA = ('scripts/runtime-oci/image_licenses.py', 'scripts/runtime-oci/grant_fallbacks.py',
                'scripts/runtime-oci/archive.py', 'scripts/runtime-oci/common.py')
RUNTIME_FILES = ('LICENSE',)
RUNTIME_PREFIXES = ('embed/compose/',)


def _git(repo, *args, input_bytes=None, output=None):
    env = os.environ.copy()
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_NO_REPLACE_OBJECTS='1', GIT_TERMINAL_PROMPT='0', GIT_OPTIONAL_LOCKS='0')
    command = ['git', '-C', str(repo), '-c', 'core.hooksPath=/dev/null', '-c', 'core.fsmonitor=false', *args]
    result = subprocess.run(command, input=input_bytes, stdout=output or subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=TIMEOUT, env=env, check=False)
    if result.returncode:
        raise ValueError('Exact local Git source is unavailable: ' + ' '.join(args[:2]))
    return result.stdout if output is None else b''


def _safe_path(name):
    path = PurePosixPath(name)
    if (not name or len(name) > 240 or name.startswith('/') or '\\' in name or
            path.as_posix() != name or any(part in ('', '.', '..') for part in path.parts) or
            not re.fullmatch(r'[A-Za-z0-9._/@+-]+(?:/[A-Za-z0-9._/@+-]+)*', name)):
        raise ValueError('Builder source path is unsafe')
    return name


def _selected(name, role):
    if role == 'luci':
        return name in SOURCE_FILES or name in SOURCE_EXTRA or name.startswith(SOURCE_PREFIXES)
    return name in RUNTIME_FILES or name.startswith(RUNTIME_PREFIXES)


def _tree(repo, revision, role):
    if not isinstance(revision, str) or not HEX40.fullmatch(revision):
        raise ValueError('Builder source revision is invalid')
    commit = _git(repo, 'cat-file', 'commit', revision)
    if hashlib.sha1(b'commit ' + str(len(commit)).encode() + b'\0' + commit).hexdigest() != revision:
        raise ValueError('Builder source commit changed')
    first = commit.splitlines()[0]
    if not first.startswith(b'tree ') or not HEX40.fullmatch(first[5:].decode('ascii')):
        raise ValueError('Builder source root tree is invalid')
    root_tree = first[5:].decode('ascii')
    trees = {root_tree}
    paths = []
    for row in _git(repo, 'ls-tree', '-r', '-t', '-z', '--full-tree', revision).split(b'\0'):
        if not row:
            continue
        header, raw_name = row.split(b'\t', 1)
        mode, kind, raw_oid = header.decode('ascii').split(' ')
        name = _safe_path(raw_name.decode('utf-8', 'strict'))
        oid = raw_oid
        if not HEX40.fullmatch(oid):
            raise ValueError('Builder source object identity is invalid')
        if kind == 'tree':
            trees.add(oid)
        elif kind == 'blob' and _selected(name, role):
            if mode not in ('100644', '100755'):
                raise ValueError('Builder source link or special file refused')
            paths.append({'path': name, 'mode': mode, 'blob': oid})
        elif kind != 'blob':
            raise ValueError('Builder source tree contains an unsupported object')
    names = {row['path'] for row in paths}
    required = set(SOURCE_FILES + SOURCE_EXTRA) if role == 'luci' else set(RUNTIME_FILES)
    if not required <= names or (role == 'luci' and not any(name.startswith('scripts/azure-embed-host/') for name in names)) or (role == 'e2b' and not any(name.startswith('embed/compose/') for name in names)):
        raise ValueError('Builder source selection is incomplete')
    if len(paths) > MAX_PATHS or len({row['path'] for row in paths}) != len(paths):
        raise ValueError('Builder source path inventory exceeds its bound')
    ids = {revision, *trees, *(row['blob'] for row in paths)}
    if len(ids) > MAX_OBJECTS:
        raise ValueError('Builder source object inventory exceeds its bound')
    info = _git(repo, 'cat-file', '--batch-check=%(objectname) %(objecttype) %(objectsize)',
                input_bytes=('\n'.join(sorted(ids)) + '\n').encode()).decode().splitlines()
    measured = {}
    for line in info:
        oid, kind, size = line.split()
        measured[oid] = (kind, int(size))
    if set(measured) != ids or measured[revision][0] != 'commit' or any(measured[oid][0] != 'tree' for oid in trees):
        raise ValueError('Builder source Git objects differ')
    for row in paths:
        kind, size = measured[row['blob']]
        if kind != 'blob' or size > MAX_FILE:
            raise ValueError('Builder source file is oversized or invalid')
        row['bytes'] = size
    expanded = sum(size for _, size in measured.values())
    if expanded > MAX_EXPANDED:
        raise ValueError('Builder source expanded objects exceed their bound')
    return root_tree, paths, ids, expanded


def _open_regular(path, limit):
    path = Path(path).absolute()
    for item in (path, *path.parents):
        info = item.lstat()
        if stat.S_ISLNK(info.st_mode) or getattr(info, 'st_file_attributes', 0) & 1024:
            raise ValueError('Linked builder source path refused')
    original = path.lstat()
    handle = os.fdopen(os.open(path, os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0)), 'rb')
    opened = os.fstat(handle.fileno())
    if (not stat.S_ISREG(opened.st_mode) or not 0 < opened.st_size <= limit or
            (opened.st_dev, opened.st_ino, opened.st_size) !=
            (original.st_dev, original.st_ino, original.st_size)):
        handle.close()
        raise ValueError('Builder source input is not an ordinary bounded file')
    return handle


def _canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True) + '\n').encode()


def collect(luci_repo, luci_revision, e2b_repo, e2b_revision, output_dir):
    """Write source.json and two Git packs, returning their exact bundle paths.

    Both input repositories must be clean and at the named HEAD. The pack is
    generated from original commit/tree objects plus only selected Git blobs.
    """
    if e2b_revision != RUNTIME_REVISION:
        raise ValueError('E2B source revision differs from the pinned runtime')
    output = Path(output_dir)
    if output.exists() or output.is_symlink():
        raise ValueError('Builder source destination already exists')
    data = {}
    with tempfile.TemporaryDirectory(dir=output.parent) as temporary:
        stage = Path(temporary) / 'builder-source'
        stage.mkdir(mode=0o700)
        for role, repo, revision in (('luci', Path(luci_repo), luci_revision), ('e2b', Path(e2b_repo), e2b_revision)):
            if (not repo.is_absolute() or repo.is_symlink() or
                    _git(repo, 'rev-parse', '--show-object-format=storage').strip() != b'sha1' or
                    _git(repo, 'rev-parse', 'HEAD').strip().decode() != revision or
                    _git(repo, 'status', '--porcelain', '--untracked-files=all').strip()):
                raise ValueError('Builder source requires an exact clean local checkout')
            tree, paths, ids, expanded = _tree(repo, revision, role)
            target = stage / (role + '.pack')
            with target.open('xb') as sink:
                _git(repo, 'pack-objects', '--stdout', input_bytes=('\n'.join(sorted(ids)) + '\n').encode(), output=sink)
            if not 0 < target.stat().st_size <= MAX_PACK:
                raise ValueError('Builder source pack exceeds 4 MiB')
            data[role] = {'revision': revision, 'tree': tree, 'pack': {'sha256': hashlib.sha256(target.read_bytes()).hexdigest(),
                'bytes': target.stat().st_size}, 'objectCount': len(ids), 'expandedBytes': expanded, 'paths': paths}
        manifest = {'version': VERSION, 'kind': 'luci-azure-builder-source', 'luci': data['luci'], 'e2b': data['e2b']}
        raw = _canonical(manifest)
        if len(raw) > MAX_MANIFEST:
            raise ValueError('Builder source manifest exceeds 2 MiB')
        (stage / 'source.json').write_bytes(raw)
        stage.rename(output)
    return {'source.json': output / 'source.json', 'luci.pack': output / 'luci.pack', 'e2b.pack': output / 'e2b.pack'}


def _manifest(root):
    with _open_regular(root / 'source.json', MAX_MANIFEST) as source:
        raw = source.read(MAX_MANIFEST + 1)
    if len(raw) > MAX_MANIFEST:
        raise ValueError('Builder source manifest exceeds its bound')
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('Duplicate builder source manifest field')
            result[key] = value
        return result
    value = json.loads(raw, object_pairs_hook=unique)
    if set(value) != {'version', 'kind', 'luci', 'e2b'} or value['version'] != VERSION or value['kind'] != 'luci-azure-builder-source':
        raise ValueError('Builder source manifest shape changed')
    for role in ('luci', 'e2b'):
        item = value[role]
        if not isinstance(item, dict) or set(item) != {'revision', 'tree', 'pack', 'objectCount', 'expandedBytes', 'paths'} or not isinstance(item['pack'], dict) or set(item['pack']) != {'sha256', 'bytes'}:
            raise ValueError('Builder source role manifest changed')
        if not HEX40.fullmatch(item['revision']) or not HEX40.fullmatch(item['tree']) or not HEX64.fullmatch(item['pack']['sha256']) or not isinstance(item['pack']['bytes'], int) or isinstance(item['pack']['bytes'], bool) or not 0 < item['pack']['bytes'] <= MAX_PACK or not isinstance(item['objectCount'], int) or isinstance(item['objectCount'], bool) or not 0 < item['objectCount'] <= MAX_OBJECTS or not isinstance(item['expandedBytes'], int) or isinstance(item['expandedBytes'], bool) or not 0 < item['expandedBytes'] <= MAX_EXPANDED:
            raise ValueError('Builder source identity or bound changed')
        paths = item['paths']
        if not isinstance(paths, list) or not 0 < len(paths) <= MAX_PATHS or paths != sorted(paths, key=lambda row: row['path']):
            raise ValueError('Builder source path inventory changed')
        seen = set()
        for row in paths:
            if not isinstance(row, dict) or set(row) != {'path', 'mode', 'blob', 'bytes'} or not isinstance(row['path'], str):
                raise ValueError('Builder source path binding changed')
            name = _safe_path(row['path'])
            if name in seen or not _selected(name, role) or row['mode'] not in ('100644', '100755') or not HEX40.fullmatch(row['blob']) or not isinstance(row['bytes'], int) or isinstance(row['bytes'], bool) or not 0 <= row['bytes'] <= MAX_FILE:
                raise ValueError('Builder source path is duplicate or outside scope')
            seen.add(name)
        required = set(SOURCE_FILES + SOURCE_EXTRA) if role == 'luci' else set(RUNTIME_FILES)
        if not required <= seen:
            raise ValueError('Builder source required path missing')
    if value['e2b']['revision'] != RUNTIME_REVISION:
        raise ValueError('E2B runtime revision changed')
    return value


def _restore_role(root, item, role, target):
    with _open_regular(root / (role + '.pack'), MAX_PACK) as source:
        data = source.read(MAX_PACK + 1)
        if len(data) != item['pack']['bytes'] or hashlib.sha256(data).hexdigest() != item['pack']['sha256']:
            raise ValueError('Builder source pack changed')
        source.seek(0)
        target.mkdir(mode=0o700)
        _git(target, 'init', '-q')
        env = os.environ.copy()
        env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull, GIT_NO_REPLACE_OBJECTS='1', GIT_TERMINAL_PROMPT='0')
        def bounded_git():
            import resource
            resource.setrlimit(resource.RLIMIT_FSIZE, (128 << 20, 128 << 20))
            resource.setrlimit(resource.RLIMIT_AS, (512 << 20, 512 << 20))
            resource.setrlimit(resource.RLIMIT_CPU, (TIMEOUT, TIMEOUT))
        result = subprocess.run(['git', '-C', str(target), 'index-pack', '--stdin'], stdin=source,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=TIMEOUT, env=env,
                                preexec_fn=bounded_git if os.name == 'posix' else None)
        if result.returncode:
            raise ValueError('Builder source Git pack is invalid')
    tree, paths, ids, expanded = _tree(target, item['revision'], role)
    if tree != item['tree'] or paths != item['paths'] or len(ids) != item['objectCount'] or expanded != item['expandedBytes']:
        raise ValueError('Builder source tree/path inventory changed')
    objects = _git(target, 'cat-file', '--batch-all-objects', '--batch-check=%(objectname)').decode().splitlines()
    if set(objects) != ids or len(objects) != len(ids):
        raise ValueError('Builder source pack contains extra or missing objects')
    _git(target, 'read-tree', item['revision'])
    selected = {row['path'] for row in paths}
    indexed = _git(target, 'ls-files', '-z').split(b'\0')
    omitted = [entry.decode('utf-8', 'strict') for entry in indexed if entry and entry.decode('utf-8', 'strict') not in selected]
    for offset in range(0, len(omitted), 200):
        _git(target, 'update-index', '--skip-worktree', '--', *omitted[offset:offset + 200])
    for row in paths:
        file = target / row['path']
        file.parent.mkdir(parents=True, exist_ok=True)
        content = _git(target, 'cat-file', 'blob', row['blob'])
        if len(content) != row['bytes']:
            raise ValueError('Builder source blob length changed')
        file.write_bytes(content)
        file.chmod(0o755 if row['mode'] == '100755' else 0o644)
    _git(target, 'update-ref', 'refs/heads/source', item['revision'])
    _git(target, 'symbolic-ref', 'HEAD', 'refs/heads/source')
    if _git(target, 'status', '--porcelain', '--untracked-files=all').strip() or _git(target, 'rev-parse', 'HEAD').strip().decode() != item['revision']:
        raise ValueError('Builder source checkout is not clean and exact')


def restore(bundle_root, target_root):
    """Verify original objects and atomically place clean sparse Luci/E2B trees."""
    root = Path(bundle_root).absolute() / 'builder-source'
    if Path(target_root).exists() or Path(target_root).is_symlink():
        raise ValueError('Builder source restoration target already exists')
    value = _manifest(root)
    target = Path(target_root).absolute()
    with tempfile.TemporaryDirectory(dir=target.parent) as temporary:
        stage = Path(temporary) / 'source'
        stage.mkdir(mode=0o700)
        for role in ('luci', 'e2b'):
            _restore_role(root, value[role], role, stage / role)
        stage.rename(target)
    return {'sourceCommit': value['luci']['revision'], 'runtimeRevision': value['e2b']['revision'],
            'luci': target / 'luci', 'e2b': target / 'e2b'}
