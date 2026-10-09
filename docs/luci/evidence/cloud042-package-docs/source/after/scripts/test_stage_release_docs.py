"""Fixture-only checks for distribution guide retention, links and provenance."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name('stage-release-docs.py')
SPEC = importlib.util.spec_from_file_location('stage_release_docs', SCRIPT)
DOCS = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = DOCS
SPEC.loader.exec_module(DOCS)


class StageGuides(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='luci-docs-test-')
        self.addCleanup(self.temp.cleanup)
        self.parent = Path(self.temp.name)
        self.root = self.parent / 'source'
        self.root.mkdir()
        self.output = self.parent / 'package-guides'
        self.environment = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
        self.environment.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull)
        self.git('init', '-q')
        self.git('config', 'user.email', 'docs-fixture@example.invalid')
        self.git('config', 'user.name', 'Luci docs fixture')
        self.git('config', 'core.autocrlf', 'false')
        # Synthetic offline association fixture: no fetch or network request.
        self.git('remote', 'add', 'origin', 'https://github.com/lcfrai/luci.git')
        for name in DOCS.FIXED:
            self.write(name, '# ' + name + '\n')
        self.write('docs/luci/SETUP.md', '# Setup\n[Local](../RUNBOOK.md#start)\n')
        self.write('docs/luci/media/local image.png', b'local image bytes\x00')
        self.write('docs/luci/evidence/result.json', b'{"passed": false}\n')
        self.write('docs/luci/evidence/report.md', '# Failed attempt\n')
        self.write('docs/luci/evidence/render shot.png', b'not-an-executed-image\x00')
        self.write('scripts/fixture-check.py', '# source-only helper\n')
        self.write('source/with space.md', '# source-only document\n')
        self.write('source/with(parentheses).md', '# source-only document\n')
        self.write('source/query.md', '# source-only document\n')
        self.write('distribution/docker/README.md', '# Docker source instructions\n')
        self.commit()

    def write(self, name, value):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(value if isinstance(value, bytes) else value.encode())

    def git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], env=self.environment,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True,
                              timeout=15).stdout.decode().strip()

    def commit(self, published=True):
        self.git('add', '--all')
        self.git('commit', '-qm', 'Documentation fixture')
        self.revision = self.git('rev-parse', 'HEAD')
        if published:
            self.git('update-ref', 'refs/remotes/origin/main', self.revision)

    def stage(self):
        return DOCS.stage(self.root, self.output, self.revision, 'false')

    def test_compact_omits_only_evidence_retains_guides_assets_and_licences(self):
        self.write('README.md', '[Runtime guide](docs/luci/SETUP.md)\n[Container source](distribution/docker/README.md)\n')
        self.write('docs/luci/SETUP.md', '[Failed receipt](evidence/result.json#failure)\n[Run](../../scripts/fixture-check.py)\n')
        self.commit()
        before = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file() and '.git' not in p.parts}
        result = self.stage()
        self.assertTrue(result['compact'])
        self.assertFalse((self.output / DOCS.EVIDENCE).exists())
        for name in DOCS.FIXED:
            self.assertTrue((self.output / name).is_file())
        self.assertEqual((self.output / 'LICENSE').read_bytes(), before[Path('LICENSE')])
        self.assertEqual((self.output / 'NOTICE').read_bytes(), before[Path('NOTICE')])
        self.assertEqual((self.output / 'docs/luci/media/local image.png').read_bytes(), b'local image bytes\x00')
        self.assertIn(f'/blob/{self.revision}/docs/luci/evidence/result.json#failure', (self.output / 'docs/luci/SETUP.md').read_text())
        self.assertIn(f'/blob/{self.revision}/distribution/docker/README.md', (self.output / 'README.md').read_text())
        self.assertIn('(docs/luci/SETUP.md)', (self.output / 'README.md').read_text())
        self.assertEqual(before, {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file() and '.git' not in p.parts})
        self.assertEqual(self.git('status', '--porcelain'), '')

    def test_spaces_percent_encoding_titles_queries_fragments_and_nested_labels(self):
        body = '''[nested [label]](<source/with space.md?download=1#some-heading> "A title")
[encoded](source/with%20space.md#an%20anchor)
[parentheses](source/with\\(parentheses\\).md 'Title')
[balanced](source/with(parentheses).md)
[query](source/query.md?foo=a%20b&amp;two=2#ok)
'''
        self.write('README.md', body)
        self.commit()
        self.stage()
        actual = (self.output / 'README.md').read_text()
        base = f'https://github.com/lcfrai/luci/blob/{self.revision}/'
        self.assertIn(f'<{base}source/with%20space.md?download=1#some-heading> "A title"', actual)
        self.assertIn(base + 'source/with%20space.md#an%20anchor', actual)
        self.assertEqual(actual.count(base + 'source/with%28parentheses%29.md'), 2)
        self.assertIn(base + 'source/query.md?foo=a%20b&amp;two=2#ok', actual)

    def test_images_references_directories_and_shipped_assets(self):
        self.write('docs/luci/SETUP.md', '''![Screenshot](<evidence/render shot.png?raw=1#view> "Image")
![Reference][image ref]
![image ref]
[Ordinary reference][report]
[image ref]: <evidence/render shot.png> "Reference image"
[report]: evidence/report.md#failed
[Directory](../../scripts/)
![Local](media/local%20image.png#still-local)
''')
        self.commit()
        self.stage()
        actual = (self.output / 'docs/luci/SETUP.md').read_text()
        raw = f'https://raw.githubusercontent.com/lcfrai/luci/{self.revision}/docs/luci/evidence/render%20shot.png'
        self.assertIn(raw + '?raw=1#view', actual)
        self.assertIn(f'[image ref]: <{raw}>', actual)
        self.assertIn(f'/blob/{self.revision}/docs/luci/evidence/report.md#failed', actual)
        self.assertIn(f'/tree/{self.revision}/scripts', actual)
        self.assertIn('![Local](media/local%20image.png#still-local)', actual)

    def test_code_comments_escaped_links_and_external_destinations_untouched(self):
        body = '''`[inline](missing.md)` and ``[code `tick`](missing.md)``
```md
[fenced](missing.md)
```
~~~
![tilde](missing.png)
~~~
> ```
> [quoted](missing.md)
> ```
    [indented](missing.md)
<!-- [comment](missing.md) -->
\\[escaped](missing.md)
[External](https://example.invalid/path?q=1#part)
[Mail](mailto:test@example.invalid)
[Network](//example.invalid/file)
[Here](#section)
[Query](?view=1#section)
Run scripts/fixture-check.py. Plain evidence/result.json remains plain text.
'''
        self.write('README.md', body)
        self.commit()
        self.stage()
        self.assertTrue((self.output / 'README.md').read_text().startswith(body))

    def test_crlf_and_local_destinations_preserved(self):
        body = b'# Guide\r\n[Local](docs/luci/SETUP.md#start)\r\n'
        self.write('README.md', body)
        self.commit()
        self.stage()
        self.assertTrue((self.output / 'README.md').read_bytes().startswith(body))

    def test_clean_autocrlf_checkout_stages_committed_bytes_without_source_changes(self):
        self.git('config', 'core.autocrlf', 'true')
        body = b'# Windows guide\r\n[Local](docs/luci/SETUP.md#start)\r\n'
        licence = b'Original fixture grant\r\nAll rights retained.\r\n'
        self.write('README.md', body)
        self.write('LICENSE', licence)
        self.commit()
        self.assertEqual(self.git('status', '--porcelain'), '')
        result = self.stage()
        self.assertTrue(result['compact'])
        self.assertTrue((self.output / 'README.md').read_bytes().startswith(body.replace(b'\r\n', b'\n')))
        self.assertEqual((self.output / 'LICENSE').read_bytes(), licence.replace(b'\r\n', b'\n'))
        self.assertEqual((self.root / 'README.md').read_bytes(), body)
        self.assertEqual((self.root / 'LICENSE').read_bytes(), licence)
        self.assertEqual(self.git('status', '--porcelain'), '')

    def test_entity_delimiters_keep_query_and_fragment_meaning(self):
        self.write('README.md', '''[Fragment](source/query.md&#35;failed)
[Query](source/query.md&quest;raw=1&amp;two=2#failed)
''')
        self.commit()
        self.stage()
        actual = (self.output / 'README.md').read_text()
        base = f'https://github.com/lcfrai/luci/blob/{self.revision}/source/query.md'
        self.assertIn(base + '#failed)', actual)
        self.assertIn(base + '?raw=1&amp;two=2#failed)', actual)

    def test_literal_and_escaped_entity_filenames_follow_commonmark(self):
        self.write('source/&copy.md', '# literal semicolon-less name\n')
        self.write('source/&copy;.md', '# escaped ampersand name\n')
        self.write('README.md', '[Literal](source/&copy.md)\n[Escaped](source/\\&copy;.md)\n')
        self.commit()
        self.stage()
        actual = (self.output / 'README.md').read_text()
        self.assertIn(f'/blob/{self.revision}/source/%26copy.md', actual)
        self.assertIn(f'/blob/{self.revision}/source/%26copy%3B.md', actual)

    def test_ambiguous_nested_list_links_retain_evidence_instead_of_breaking_links(self):
        for index, body in enumerate([
            '- Results\n    - [Report](docs/luci/evidence/report.md)\n',
            '- Results\n\n    [Report](docs/luci/evidence/report.md)\n',
        ]):
            with self.subTest(index=index):
                self.write('README.md', body)
                self.commit()
                target = self.parent / f'nested-{index}'
                result = DOCS.stage(self.root, target, self.revision, 'false')
                self.assertFalse(result['compact'])
                self.assertEqual(result['retainedReason'], 'ambiguous-markdown-indentation')
                self.assertTrue((target / 'README.md').read_text().startswith(body))
                self.assertTrue((target / 'docs/luci/evidence/report.md').is_file())

    def test_fenced_list_example_remains_literal_without_disabling_compaction(self):
        body = '```md\n- Results\n    - [Example](missing.md)\n```\n'
        self.write('README.md', body)
        self.commit()
        result = self.stage()
        self.assertTrue(result['compact'])
        self.assertTrue((self.output / 'README.md').read_text().startswith(body))

    def test_comment_openers_in_inline_code_do_not_hide_following_links(self):
        self.write('README.md', '''`<!--` followed by [real](source/query.md).
<!-- a ` tick --> [also real](source/query.md) then an unmatched `.
''')
        self.commit()
        self.stage()
        actual = (self.output / 'README.md').read_text()
        self.assertTrue(actual.startswith('`<!--` followed by '))
        self.assertEqual(actual.count(f'/blob/{self.revision}/source/query.md'), 2)

    def test_dirty_and_unknown_keep_complete_validation_bytes(self):
        for mode, revision in [('true', self.revision), ('unknown', ''), ('false', '')]:
            with self.subTest(mode=mode, revision=bool(revision)):
                target = self.parent / ('output-' + mode)
                self.write('docs/luci/evidence/result.json', b'{"changed": true}\n')
                result = DOCS.stage(self.root, target, revision, mode)
                self.assertFalse(result['compact'])
                self.assertEqual((target / 'docs/luci/evidence/result.json').read_bytes(), b'{"changed": true}\n')
                self.assertNotIn('github.com/lcfrai/luci', (target / 'docs/PACKAGE-VALIDATION.md').read_text())

    def test_clean_unpublished_commit_retains_full_docs_without_official_links(self):
        self.write('README.md', '[Local evidence](docs/luci/evidence/report.md)\n')
        self.commit(published=False)
        result = self.stage()
        self.assertFalse(result['compact'])
        self.assertIsNone(result['officialSourceAssociation'])
        self.assertTrue((self.output / 'docs/luci/evidence/report.md').is_file())
        self.assertIn('(docs/luci/evidence/report.md)', (self.output / 'README.md').read_text())
        self.assertNotIn('github.com/lcfrai/luci', (self.output / 'docs/PACKAGE-VALIDATION.md').read_text())

    def test_fork_or_missing_tracking_ref_preserves_full_docs(self):
        cases = [('https://github.com/some-fork/luci.git', False),
                 ('https://github.com/lcfrai/luci.git', True),
                 ('https://github.com.evil.invalid/lcfrai/luci.git', False)]
        for index, (url, remove_ref) in enumerate(cases):
            with self.subTest(case=index):
                self.git('remote', 'set-url', 'origin', url)
                if remove_ref:
                    self.git('update-ref', '-d', 'refs/remotes/origin/main')
                result = DOCS.stage(self.root, self.parent / f'fallback-{index}', self.revision, 'false')
                self.assertFalse(result['compact'])
                self.assertIsNone(result['officialSourceAssociation'])
                self.assertTrue((self.parent / f'fallback-{index}/docs/luci/evidence/result.json').is_file())

    def test_official_ssh_tracking_ancestor_is_accepted_without_network(self):
        self.git('remote', 'set-url', 'origin', 'git@github.com:lcfrai/luci.git')
        earlier = self.revision
        self.write('source/query.md', 'later tracked source')
        self.commit()
        tracked = self.revision
        self.git('checkout', '--detach', earlier)
        result = DOCS.stage(self.root, self.output, earlier, 'false')
        self.assertTrue(result['compact'])
        self.assertEqual(result['officialSourceAssociation']['trackingCommit'], tracked)

    def test_offline_probe_failure_falls_back_without_raw_url_diagnostics(self):
        original_git = DOCS.git

        def fail_probe(root, *args, **kwargs):
            if args and args[0] == 'remote':
                raise subprocess.TimeoutExpired('offline-association', 10)
            return original_git(root, *args, **kwargs)

        with mock.patch.object(DOCS, 'git', side_effect=fail_probe):
            result = self.stage()
        self.assertFalse(result['compact'])
        self.assertIsNone(result['officialSourceAssociation'])
        self.assertTrue((self.output / 'docs/luci/evidence/result.json').exists())

    def test_claimed_clean_but_changed_or_wrong_revision_rejected(self):
        self.write('README.md', 'changed')
        with self.assertRaisesRegex(ValueError, 'changed checkout'):
            self.stage()
        self.assertFalse(self.output.exists())
        with self.assertRaisesRegex(ValueError, 'exact checkout'):
            DOCS.stage(self.root, self.output, '1' * 40, 'false')

    def test_missing_source_object_and_escaping_links_rejected_without_output(self):
        for destination in ['missing.md', '../outside.md', '%2E%2E/outside.md', '/outside.md', 'source/%00bad']:
            with self.subTest(destination=destination):
                self.write('README.md', f'[Link]({destination})\n')
                self.commit()
                with self.assertRaises((ValueError, FileNotFoundError)):
                    self.stage()
                self.assertFalse(self.output.exists())

    def test_ignored_untracked_and_assume_unchanged_bytes_cannot_enter_clean_package(self):
        self.write('.gitignore', 'docs/luci/private.md\n')
        self.commit()
        self.write('docs/luci/private.md', 'untracked fixture content')
        self.assertEqual(self.git('status', '--porcelain'), '')
        with self.assertRaisesRegex(ValueError, 'absent from the exact'):
            self.stage()
        self.assertFalse(self.output.exists())
        (self.root / 'docs/luci/private.md').unlink()
        self.git('update-index', '--assume-unchanged', 'README.md')
        self.write('README.md', 'hidden modification')
        self.assertEqual(self.git('status', '--porcelain'), '')
        with self.assertRaisesRegex(ValueError, 'differs from the exact'):
            self.stage()
        self.assertFalse(self.output.exists())

    def test_symlink_input_and_destination_ancestor_refused(self):
        target = self.parent / 'outside.md'
        target.write_text('preserve me')
        link = self.root / 'docs/luci/linked.md'
        try:
            link.symlink_to(target)
        except OSError as error:
            self.skipTest(f'Native symlink privilege unavailable: {error}')
        with self.assertRaisesRegex(ValueError, 'Linked'):
            DOCS.stage(self.root, self.output, '', 'unknown')
        self.assertEqual(target.read_text(), 'preserve me')
        link.unlink()
        alias = self.parent / 'alias'
        alias.symlink_to(self.root, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, 'Linked'):
            DOCS.stage(self.root, alias / 'output', '', 'unknown')
        self.assertFalse((self.root / 'output').exists())

    def test_existing_destination_not_overwritten(self):
        self.output.mkdir()
        retained = self.output / 'keep'
        retained.write_bytes(b'unchanged')
        with self.assertRaisesRegex(ValueError, 'new staging'):
            self.stage()
        self.assertEqual(retained.read_bytes(), b'unchanged')


class BuildWiring(unittest.TestCase):
    def test_both_packagers_and_native_use_shared_staging(self):
        root = SCRIPT.parent.parent
        unix = (root / 'scripts/build-release.sh').read_text()
        windows = (root / 'scripts/build-release.ps1').read_text()
        self.assertIn('stage-release-docs.py', unix)
        self.assertIn('stage-release-docs.py', windows)
        self.assertNotIn('cp -R "$root/docs/luci/."', unix)
        self.assertNotIn("Copy-Tree (Join-Path $luciRoot 'docs/luci')", windows)
        resources = json.loads((root / 'desktop/src-tauri/tauri.conf.json').read_text())['bundle']['resources']
        self.assertEqual(resources['guides/docs/'], 'docs/')
        self.assertEqual(resources['guides/README.md'], 'README.md')
        self.assertEqual(resources['runtime/'], 'runtime/')
        self.assertEqual(resources['licenses/'], 'licenses/')
        self.assertEqual(resources['cloud/'], 'cloud/')
        self.assertFalse(any(key.startswith('../../docs') for key in resources))
        self.assertIn("@('runtime','licenses','cloud','guides')", windows)
        self.assertIn('/guides/', (root / 'desktop/src-tauri/.gitignore').read_text())
        self.assertEqual((root / 'desktop/src-tauri/build.rs').read_text().strip(), 'fn main() {\n    tauri_build::build()\n}')
        self.assertIn('stage-release-docs.py', (root / 'scripts/export-public-source.py').read_text())
        self.assertIn('desktop/src-tauri/guides/', (root / 'scripts/export-public-source.py').read_text())


if __name__ == '__main__':
    unittest.main()
