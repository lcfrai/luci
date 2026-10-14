"""Offline original-object and sparse-checkout guards for builder source."""

import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock


SPEC = importlib.util.spec_from_file_location('source_bundle', Path(__file__).with_name('source_bundle.py'))
source_bundle = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(source_bundle)


class SourceBundleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / 'bundle').mkdir()
        self.luci = self.repository('luci', {
            **{name: (name + '\n') for name in source_bundle.SOURCE_FILES + source_bundle.SOURCE_EXTRA},
            'control-plane/cmd/api/main.go': 'package main\nfunc main() {}\n',
            'scripts/azure-embed-host/package.py': 'print("source")\n',
            'sandboxd/vendor/modules.txt': 'fixture modules\n',
            'third-party/licenses/openai-codex-0.160.0/LICENSE': 'original grant\n',
            'third-party/licenses/opencode-exact/NOTICE': 'other original grant\n',
            'outside/ignored.txt': 'out of scope\n',
        })
        self.e2b = self.repository('e2b', {'LICENSE': 'E2B original licence\n',
            'embed/compose/compose.yaml': 'services: {}\n', 'other/ignored.txt': 'out of scope\n'})

    def repository(self, name, files):
        root = self.root / name
        root.mkdir()
        subprocess.run(['git', 'init', '-q', str(root)], check=True)
        for path, body in files.items():
            target = root / path
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(body)
        subprocess.run(['git', '-C', str(root), 'add', '-A'], check=True)
        subprocess.run(['git', '-C', str(root), '-c', 'user.name=Source Fixture',
                        '-c', 'user.email=source@example.invalid', 'commit', '-qm', 'source'], check=True)
        return root

    def revision(self, repo):
        return subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip()

    def collect(self):
        with mock.patch.object(source_bundle, 'RUNTIME_REVISION', self.revision(self.e2b)):
            return source_bundle.collect(self.luci, self.revision(self.luci), self.e2b,
                                         self.revision(self.e2b), self.root / 'bundle' / 'builder-source')

    def restore(self):
        with mock.patch.object(source_bundle, 'RUNTIME_REVISION', self.revision(self.e2b)):
            return source_bundle.restore(self.root / 'bundle', self.root / 'restored')

    def manifest(self):
        return self.root / 'bundle' / 'builder-source' / 'source.json'

    def test_clean_sparse_original_commit_and_scope(self):
        self.collect()
        result = self.restore()
        for role, original in (('luci', self.luci), ('e2b', self.e2b)):
            target = result[role]
            self.assertEqual(self.revision(original), self.revision(target))
            self.assertEqual(subprocess.check_output(['git', '-C', str(target), 'status', '--porcelain']), b'')
            self.assertFalse((target / ('outside' if role == 'luci' else 'other')).exists())
        self.assertEqual((result['luci'] / 'NOTICE').read_text(), 'NOTICE\n')
        self.assertEqual((result['luci'] / 'third-party/licenses/opencode-exact/NOTICE').read_text(), 'other original grant\n')

    def test_changed_pack_and_manifest_refused(self):
        paths = self.collect()
        with paths['luci.pack'].open('ab') as sink:
            sink.write(b'changed')
        with self.assertRaisesRegex(ValueError, 'pack changed'):
            self.restore()
        paths['luci.pack'].write_bytes(paths['luci.pack'].read_bytes()[:-7])
        value = json.loads(self.manifest().read_text())
        value['luci']['paths'][0]['path'] = 'outside/ignored.txt'
        value['luci']['paths'].sort(key=lambda row: row['path'])
        self.manifest().write_text(json.dumps(value))
        with self.assertRaisesRegex(ValueError, 'outside scope'):
            self.restore()

    def test_duplicate_or_missing_path_and_source_revision_refused(self):
        self.collect()
        original = self.manifest().read_bytes()
        self.manifest().write_bytes(original.replace(b'"version":1', b'"version":1,"version":1', 1))
        with self.assertRaisesRegex(ValueError, 'Duplicate'):
            self.restore()
        value = json.loads(original)
        value['luci']['paths'] = [row for row in value['luci']['paths'] if row['path'] != 'NOTICE']
        self.manifest().write_text(json.dumps(value))
        with self.assertRaisesRegex(ValueError, 'required path missing'):
            self.restore()
        value = json.loads(original)
        value['luci']['revision'] = 'f' * 40
        self.manifest().write_text(json.dumps(value))
        with self.assertRaises(ValueError):
            self.restore()

    def test_dirty_or_linked_source_and_oversized_inventory_refused(self):
        (self.luci / 'new-untracked').write_text('dirty')
        with self.assertRaisesRegex(ValueError, 'exact clean'):
            self.collect()
        (self.luci / 'new-untracked').unlink()
        with mock.patch.object(source_bundle, 'MAX_EXPANDED', 1):
            with self.assertRaisesRegex(ValueError, 'expanded objects'):
                self.collect()
        (self.luci / 'control-plane/cmd/api/link').symlink_to('main.go')
        subprocess.run(['git', '-C', str(self.luci), 'add', '-A'], check=True)
        subprocess.run(['git', '-C', str(self.luci), '-c', 'user.name=Source Fixture',
                        '-c', 'user.email=source@example.invalid', 'commit', '-qm', 'link'], check=True)
        with self.assertRaisesRegex(ValueError, 'link or special'):
            self.collect()

    def test_pack_with_extra_git_object_refused_even_if_rebound(self):
        paths = self.collect()
        extra = subprocess.check_output(['git', '-C', str(self.luci), 'hash-object', '-w', '--stdin'],
                                        input=b'unrelated extra object').decode().strip()
        value = json.loads(self.manifest().read_text())
        ids = {row['blob'] for row in value['luci']['paths']}
        ids.add(value['luci']['revision']); ids.add(value['luci']['tree']); ids.add(extra)
        for line in subprocess.check_output(['git', '-C', str(self.luci), 'ls-tree', '-r', '-t', '-z', 'HEAD']).split(b'\0'):
            if line and line.startswith(b'040000 tree '):
                ids.add(line.split(b'\t', 1)[0].split()[-1].decode())
        pack = subprocess.check_output(['git', '-C', str(self.luci), 'pack-objects', '--stdout'],
                                       input=('\n'.join(sorted(ids)) + '\n').encode())
        paths['luci.pack'].write_bytes(pack)
        import hashlib
        value['luci']['pack'] = {'sha256': hashlib.sha256(pack).hexdigest(), 'bytes': len(pack)}
        self.manifest().write_text(json.dumps(value))
        with self.assertRaisesRegex(ValueError, 'extra or missing'):
            self.restore()

    def test_linked_or_oversized_bundle_member_and_execution_timeout_refused(self):
        paths = self.collect()
        original = self.manifest().read_bytes()
        self.manifest().unlink()
        alias = self.root / 'manifest-alias'
        alias.write_bytes(original)
        self.manifest().symlink_to(alias)
        with self.assertRaisesRegex(ValueError, 'Linked'):
            self.restore()
        self.manifest().unlink()
        self.manifest().write_bytes(original)
        with paths['luci.pack'].open('ab') as output:
            output.truncate(source_bundle.MAX_PACK + 1)
        with self.assertRaisesRegex(ValueError, 'bounded file'):
            self.restore()
        with mock.patch.object(source_bundle.subprocess, 'run', side_effect=subprocess.TimeoutExpired(['git'], 1)):
            with self.assertRaises(subprocess.TimeoutExpired):
                source_bundle._git(self.luci, 'status', '--porcelain')


if __name__ == '__main__':
    unittest.main()
