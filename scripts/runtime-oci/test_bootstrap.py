import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import archive
import bootstrap
import pull_metadata


def packed(files):
    out = io.BytesIO()
    with tarfile.open(fileobj=out, mode='w:', format=tarfile.USTAR_FORMAT) as target:
        for name, value in files.items():
            info = tarfile.TarInfo(name)
            info.mode, info.size = 0o755, len(value)
            target.addfile(info, io.BytesIO(value))
    return out.getvalue()


class BootstrapTests(unittest.TestCase):
    def fixture(self, directory, extra=None, env=None, split=False, tail=b''):
        gate, registry = b'fixed-static-gate', b'pinned-distribution'
        files = {'luci-registry-gate': gate, 'usr/local/bin/registry': registry}
        files.update(extra or {})
        layers = [packed({name: value}) for name, value in files.items()] if split else [packed(files) + tail]
        # Deliberately opposite lexical digest order: archive order must still
        # match config order, as the streaming Go importer requires.
        layers.sort(key=lambda value: hashlib.sha256(value).hexdigest(), reverse=True)
        diffs = ['sha256:' + hashlib.sha256(layer).hexdigest() for layer in layers]
        config = archive.encoded({'os': 'linux', 'architecture': 'amd64',
            'config': {'User': '65532:65532', 'WorkingDir': '/', 'Entrypoint': ['/luci-registry-gate'],
                       'Env': env or [], 'ExposedPorts': {'5000/tcp': {}}},
            'rootfs': {'type': 'layers', 'diff_ids': diffs}})
        image = 'sha256:' + hashlib.sha256(config).hexdigest()
        saved = {'Config': image[7:] + '.json', 'RepoTags': ['producer/owned:bootstrap'], 'Layers': ['old/' + str(i) + '/layer.tar' for i in range(len(layers))]}
        source = directory / 'raw.tar'
        source.write_bytes(packed({saved['Config']: config, **dict(zip(saved['Layers'], layers)),
                                   'manifest.json': archive.encoded([saved])}))
        return source, image, hashlib.sha256(gate).hexdigest(), hashlib.sha256(registry).hexdigest()

    def test_normalization_preserves_identity_and_strips_transport_tag(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root)
            output = root / bootstrap.NAME
            result = bootstrap.normalize(args[0], output, *args[1:])
            with tarfile.open(output, 'r:') as saved:
                names = saved.getnames()
                manifest = json.load(saved.extractfile('manifest.json'))
                self.assertEqual(names, [result['imageID'][7:] + '.json', 'manifest.json', *manifest[0]['Layers']])
                self.assertEqual(manifest[0]['RepoTags'], [])
                self.assertEqual(manifest[0]['Config'], result['imageID'][7:] + '.json')
                self.assertTrue(all(x.isfile() and x.mode == 0o644 for x in saved.getmembers()))
            self.assertEqual(result['environment'], [])
            self.assertEqual(result['configSHA256'], args[1][7:])

    def test_two_layers_retain_config_order_in_archive(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root, split=True)
            output = root / bootstrap.NAME
            result = bootstrap.normalize(args[0], output, *args[1:])
            with tarfile.open(output, 'r:') as saved:
                manifest = json.load(saved.extractfile('manifest.json'))[0]
                names = [x.name for x in saved if x.name.endswith('/layer.tar')]
                self.assertEqual(names, manifest['Layers'])
                self.assertEqual(names, [x['diffID'][7:] + '/layer.tar' for x in result['layers']])
                self.assertNotEqual(names, sorted(names))

    def test_nonzero_tail_after_layer_end_marker_refuses(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root, tail=b'concealed-file')
            with self.assertRaisesRegex(ValueError, 'trailing bytes'):
                bootstrap.normalize(args[0], root / bootstrap.NAME, *args[1:])

    def test_extra_runtime_file_cannot_hide_inside_bootstrap(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root, {'usr/bin/node': b'not allowed'})
            with self.assertRaisesRegex(ValueError, 'extra/linked'):
                bootstrap.normalize(args[0], root / bootstrap.NAME, *args[1:])

    def test_changed_binary_and_config_refuse(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root)
            with self.assertRaisesRegex(ValueError, 'executable changed'):
                bootstrap.normalize(args[0], root / bootstrap.NAME, args[1], '0' * 64, args[3])
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root, env=['SECRET=untrusted'])
            with self.assertRaisesRegex(ValueError, 'execution configuration'):
                bootstrap.normalize(args[0], root / bootstrap.NAME, *args[1:])

    def test_archive_bound_and_linked_source_refuse_before_read(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root)
            with patch.object(bootstrap, 'MAX_BYTES', 1024):
                with self.assertRaisesRegex(ValueError, 'bound'):
                    bootstrap.normalize(args[0], root / bootstrap.NAME, *args[1:])
            with self.assertRaisesRegex(ValueError, 'output bound'):
                bootstrap.normalize(args[0], args[0], *args[1:])

    def test_gate_staging_hashes_complete_immutable_git_blobs(self):
        files = {'Dockerfile': b'FROM scratch\n', 'go.mod': b'module gate\n',
                 'main.go': b'package main\n', 'main_test.go': b'package main\n', 'README.md': b'input\n'}
        entries = b'\0'.join(('100644 blob ' + name + '\t' + bootstrap.PREFIX + name).encode() for name in files) + b'\0'
        def git(repo, *args):
            if args[0] == 'ls-tree': return entries
            if args[1] == '-s': return str(len(files[args[2]])).encode()
            return files[args[2]]
        with tempfile.TemporaryDirectory() as temp, patch.object(bootstrap.producer, 'clean_source') as clean, patch.object(bootstrap.canonical_inputs, 'git', side_effect=git):
            root = Path(temp)
            result = bootstrap.stage_gate(root, 'a' * 40, root / 'stage')
            clean.assert_called_once_with(root, 'a' * 40)
            self.assertEqual(len(result['sourceFiles']), 5)
            self.assertEqual(result['gateSourceSHA256'], bootstrap.canonical_inputs.inventory_digest(result['sourceFiles']))
            self.assertEqual((root / 'stage/main.go').read_bytes(), files['main.go'])

    def evidence(self, directory):
        directory.mkdir()
        value = {'builder': {'argv': ['docker', 'build', '--network=none', '.'], 'version': 'Docker 28.5.1',
                            'memoryMiB': 256, 'cpuMillis': 1000, 'exitStatus': 0},
                 'probe': {'argv': ['docker', 'run', '--network=none', 'sha256:fixture', '--version'], 'exitStatus': 0}}
        (directory / 'bootstrap-execution.json').write_bytes(pull_metadata.encoded(value))
        for phase in ('builder', 'probe'):
            (directory / (phase + '.stdout.log')).write_bytes(b'luci-registry-gate 1\n' if phase == 'probe' else b'build complete\n')
            (directory / (phase + '.stderr.log')).write_bytes(b'')
        return value

    def test_receipt_publication_binds_actual_logs_and_binary_bytes(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            args = self.fixture(root)
            gate = root / 'gate'
            gate.write_bytes(b'fixed-static-gate')
            evidence = root / 'evidence'
            self.evidence(evidence)
            pinned = {'archiveSHA256': 'a' * 64, 'files': [
                {'file': 'registry', 'sha256': args[3]}, {'file': 'LICENSE', 'sha256': 'b' * 64}]}
            staged = {'gateSourceSHA256': 'c' * 64, 'buildRecipeSHA256': 'd' * 64}
            (root / 'distribution').mkdir()
            (root / 'distribution/LICENSE').write_bytes(b'original grant')
            compile_proof = {'sourceCommit': 'e' * 40, 'gateSourceSHA256': 'c' * 64, 'gateBinarySHA256': args[2],
                'argv': ['go', 'build', '-trimpath', '-buildvcs=false', '-o', 'luci-registry-gate', '.'],
                'environment': {'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': 'amd64', 'GOWORK': 'off', 'GOTOOLCHAIN': 'local'},
                'exitStatus': 0, 'stdoutSHA256': hashlib.sha256(b'').hexdigest(), 'stderrSHA256': hashlib.sha256(b'').hexdigest()}
            (evidence / 'gate-compile.json').write_bytes(pull_metadata.encoded(compile_proof))
            for side in ('stdout', 'stderr'): (evidence / ('gate-compile.' + side + '.log')).write_bytes(b'')
            with patch.object(bootstrap.producer, 'clean_source'), patch.object(bootstrap, 'stage_gate', return_value=staged), patch.object(bootstrap.registry_producer, 'verify_distribution', return_value=pinned):
                result = bootstrap.publish(root / 'repo', 'e' * 40, args[0], args[1], gate, root / 'distribution', evidence, root / 'output')
            raw = (root / 'output' / bootstrap.RECEIPT).read_bytes()
            receipt = json.loads(raw)
            self.assertEqual(raw, pull_metadata.encoded(receipt))
            self.assertFalse(raw.endswith(b'\n'))
            self.assertEqual(result['buildReceiptSHA256'], hashlib.sha256(raw).hexdigest())
            self.assertEqual(receipt['image']['id'], args[1])
            self.assertEqual(receipt['builder']['stdoutSHA256'], hashlib.sha256(b'build complete\n').hexdigest())
            self.assertEqual(result['gateBinarySHA256'], args[2])

    def test_failed_or_changed_probe_cannot_publish_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            value = self.evidence(root / 'evidence')
            value['builder']['exitStatus'] = 1
            (root / 'evidence/bootstrap-execution.json').write_bytes(pull_metadata.encoded(value))
            with self.assertRaisesRegex(ValueError, 'successful builder/probe terminal'):
                bootstrap.execution_evidence(root / 'evidence')
            value['builder']['exitStatus'] = 0
            (root / 'evidence/bootstrap-execution.json').write_bytes(pull_metadata.encoded(value))
            (root / 'evidence/probe.stdout.log').write_bytes(b'luci-registry-gate 2\n')
            with self.assertRaisesRegex(ValueError, 'actual gate version'):
                bootstrap.execution_evidence(root / 'evidence')


if __name__ == '__main__': unittest.main()
