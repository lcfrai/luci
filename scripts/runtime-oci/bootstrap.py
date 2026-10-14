"""Source staging and strict archive normalization for the tiny v2 helper.

This module never starts Docker or invents a successful build/probe receipt.
Callers must supply actual builder and probe evidence before publication.
"""
from __future__ import annotations

import hashlib
import io
from pathlib import Path
import re
import tarfile
import time
import tempfile
import argparse
import json
import shutil

import archive
import canonical_inputs
from common import plain_directory, read_json
import producer
import pull_metadata
import registry_producer

PREFIX = 'scripts/runtime-oci/pull-gate/'
MAX_BYTES = 256 << 20
MAX_EXPANDED = 512 << 20
NAME = 'runtime-registry-bootstrap-linux-amd64.tar'
RECEIPT = 'runtime-bootstrap-build-receipt-v1.json'


def stage_gate(repo, revision, destination):
    """Build from immutable complete Git blobs, never CRLF checkout bytes."""
    repo, destination = Path(repo), Path(destination)
    producer.clean_source(repo, revision)
    plain_directory(destination.parent)
    if destination.exists() or destination.is_symlink():
        raise ValueError('bootstrap staging destination exists')
    entries = canonical_inputs.git(repo, 'ls-tree', '-rz', '--full-tree', revision, PREFIX)
    rows, blobs, total = [], {}, 0
    for entry in entries.split(b'\0'):
        if not entry:
            continue
        metadata, raw_path = entry.split(b'\t', 1)
        mode, kind, oid = metadata.decode('ascii').split(' ')
        path = raw_path.decode('ascii')
        if not path.startswith(PREFIX) or kind != 'blob' or mode not in ('100644', '100755'):
            raise ValueError('bootstrap source is not a regular scoped Git blob')
        archive.safe_path(path)
        size = int(canonical_inputs.git(repo, 'cat-file', '-s', oid))
        if len(rows) >= 256 or size < 0 or total + size > 4 << 20:
            raise ValueError('bootstrap source exceeds bounded subtree')
        blob = canonical_inputs.git(repo, 'cat-file', 'blob', oid)
        if len(blob) != size:
            raise ValueError('bootstrap source changed')
        total += size
        rows.append({'path': path, 'mode': '0644', 'bytes': size, 'sha256': hashlib.sha256(blob).hexdigest()})
        blobs[path.removeprefix(PREFIX)] = blob
    if not {'Dockerfile', 'go.mod', 'main.go', 'main_test.go', 'README.md'} <= set(blobs):
        raise ValueError('bootstrap source subtree incomplete')
    destination.mkdir(parents=True, mode=0o700)
    for path, blob in blobs.items():
        target = destination / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(blob)
    return {'sourceCommit': revision, 'gateSourceSHA256': canonical_inputs.inventory_digest(rows),
            'buildRecipeSHA256': hashlib.sha256(blobs['Dockerfile']).hexdigest(),
            'sourceFiles': sorted(rows, key=lambda row: row['path'].encode('ascii'))}


def config_policy(config):
    if not isinstance(config, dict):
        raise ValueError('bootstrap execution configuration is not an object')
    c, root = config.get('config', {}), config.get('rootfs', {})
    if (not isinstance(c, dict) or not isinstance(root, dict) or
            config.get('os') != 'linux' or config.get('architecture') != 'amd64' or
            c.get('User') != '65532:65532' or c.get('WorkingDir') != '/' or
            c.get('Entrypoint') != ['/luci-registry-gate'] or c.get('Env') not in (None, []) or
            c.get('ExposedPorts') != {'5000/tcp': {}} or c.get('Cmd') or c.get('Volumes') or
            c.get('OnBuild') or c.get('Labels') or root.get('type') != 'layers'):
        raise ValueError('bootstrap execution configuration differs from fixed scratch helper')
    ids = root.get('diff_ids')
    if (not isinstance(ids, list) or not 1 <= len(ids) <= 8 or
            any(not isinstance(x, str) or not re.fullmatch('sha256:[a-f0-9]{64}', x) for x in ids) or len(set(ids)) != len(ids)):
        raise ValueError('bootstrap layer identity invalid')
    return ids


def normalize(source, destination, image_id, gate_sha, distribution_sha):
    """Verify both executable bytes and every diffID before tagless output."""
    source, destination = Path(source), Path(destination)
    archive.regular(source)
    plain_directory(destination.parent)
    if destination.exists() or destination.is_symlink() or not 1024 <= source.stat().st_size <= MAX_BYTES or not re.fullmatch('sha256:[a-f0-9]{64}', image_id):
        raise ValueError('bootstrap source/output bound or identity invalid')
    expected = {'luci-registry-gate': gate_sha, 'usr/local/bin/registry': distribution_sha}
    if any(not re.fullmatch('[a-f0-9]{64}', x) for x in expected.values()):
        raise ValueError('bootstrap executable hash invalid')
    deadline, seen, total = time.monotonic() + 300, set(), 0
    with tarfile.open(source, 'r:') as incoming:
        members = archive.scan(incoming)
        if len(members) > 64:
            raise ValueError('bootstrap archive member count exceeded')
        _, manifests = archive.read_metadata(incoming, members, 'manifest.json')
        if not isinstance(manifests, list) or len(manifests) != 1 or not isinstance(manifests[0], dict) or set(manifests[0]) != {'Config', 'RepoTags', 'Layers'}:
            raise ValueError('bootstrap save manifest invalid')
        manifest = manifests[0]
        if not isinstance(manifest['Config'], str) or not isinstance(manifest['RepoTags'], (list, type(None))):
            raise ValueError('bootstrap save manifest invalid')
        config_raw, config = archive.read_metadata(incoming, members, archive.safe_path(manifest['Config']))
        if hashlib.sha256(config_raw).hexdigest() != image_id[7:]:
            raise ValueError('bootstrap config identity changed')
        ids = config_policy(config)
        names = manifest['Layers']
        if not isinstance(names, list) or not all(isinstance(x, str) for x in names) or len(names) != len(ids) or len(set(names)) != len(names):
            raise ValueError('bootstrap save layers invalid')
        layers = []
        for name, diff in zip(names, ids):
            archive.safe_path(name)
            member = members.get(name)
            if member is None or not member.isfile() or not 0 < member.size <= MAX_EXPANDED - total:
                raise ValueError('bootstrap layer size exceeded')
            total += member.size
            with incoming.extractfile(member) as stream:
                checksum, count = archive.stream_hash(stream, member.size)
            if checksum != diff[7:]:
                raise ValueError('bootstrap diffID mismatch')
            with incoming.extractfile(member) as stream, tarfile.open(fileobj=stream, mode='r|') as layer:
                for index, file in enumerate(layer):
                    if index >= 64 or time.monotonic() > deadline:
                        raise ValueError('bootstrap verification bound exceeded')
                    path = archive.safe_path(file.name.rstrip('/') if file.isdir() else file.name)
                    if file.isdir() and not file.linkname and not file.pax_headers:
                        continue
                    if not file.isfile() or file.linkname or file.pax_headers or path not in expected or path in seen or file.size <= 0:
                        raise ValueError('bootstrap includes extra/linked/duplicate binary')
                    with layer.extractfile(file) as content:
                        digest, _ = archive.stream_hash(content, file.size)
                    if digest != expected[path]:
                        raise ValueError('bootstrap executable changed')
                    seen.add(path)
                # tarfile stops at its first end marker; reject concealed data
                # after that marker, as the consumer's Go verifier does.
                stream.seek(layer.offset)
                if any(block.strip(b'\0') for block in iter(lambda: stream.read(1 << 20), b'')):
                    raise ValueError('bootstrap layer trailing bytes changed')
            layers.append({'diffID': diff, 'uncompressedBytes': count, 'sourcePath': name})
        if seen != set(expected):
            raise ValueError('bootstrap executable inventory incomplete')
        config_name = image_id[7:] + '.json'
        payloads = {config_name: config_raw}
        normalized_layers = [x['diffID'][7:] + '/layer.tar' for x in layers]
        payloads['manifest.json'] = pull_metadata.encoded([{'Config': config_name, 'RepoTags': [], 'Layers': normalized_layers}])
        if total + sum(len(raw) for raw in payloads.values()) > MAX_EXPANDED:
            raise ValueError('bootstrap aggregate expanded archive exceeds bound')
        layer_map = {name: row for name, row in zip(normalized_layers, layers)}
        with tarfile.open(destination, 'x:', format=tarfile.USTAR_FORMAT) as target:
            # The consumer verifies layers sequentially in original config
            # order. Sorting diffID paths would silently reorder this chain.
            for name in [*sorted(payloads), *normalized_layers]:
                row = layer_map.get(name)
                info = tarfile.TarInfo(name)
                info.mode, info.mtime, info.uid, info.gid = 0o644, 0, 0, 0
                info.size = row['uncompressedBytes'] if row else len(payloads[name])
                if row:
                    with incoming.extractfile(members[row['sourcePath']]) as stream:
                        target.addfile(info, stream)
                else:
                    target.addfile(info, io.BytesIO(payloads[name]))
    result = archive.file_record(destination)
    if result['bytes'] > MAX_BYTES:
        raise ValueError('normalized bootstrap exceeds bound')
    return {'fileName': NAME, 'sha256': result['sha256'], 'bytes': result['bytes'], 'imageID': image_id,
            'configSHA256': image_id[7:], 'configBytes': len(config_raw),
            'layers': [{k: row[k] for k in ('diffID', 'uncompressedBytes')} for row in layers],
            'user': '65532:65532', 'workingDir': '/', 'entrypoint': ['/luci-registry-gate'],
            'exposedPorts': ['5000/tcp'], 'environment': []}


def execution_evidence(directory):
    """Bind externally captured successful commands to their actual log bytes."""
    directory = plain_directory(directory)
    value = read_json(directory / 'bootstrap-execution.json')
    if not isinstance(value, dict) or set(value) != {'builder', 'probe'}:
        raise ValueError('bootstrap execution evidence shape invalid')
    for phase in ('builder', 'probe'):
        row = value[phase]
        required = {'argv', 'exitStatus'} | ({'version', 'memoryMiB', 'cpuMillis'} if phase == 'builder' else set())
        if not isinstance(row, dict) or set(row) != required or type(row.get('exitStatus')) is not int or row['exitStatus'] != 0:
            raise ValueError('bootstrap successful builder/probe terminal required')
        args = row['argv']
        if not isinstance(args, list) or not 1 <= len(args) <= 32 or any(not isinstance(x, str) or not 1 <= len(x.encode('utf-8')) <= 256 or any(c in x for c in '\0\r\n') for x in args):
            raise ValueError('bootstrap builder/probe argv invalid')
        for side in ('stdout', 'stderr'):
            record = archive.file_record(directory / (phase + '.' + side + '.log'))
            if record['bytes'] > 16 << 20:
                raise ValueError('bootstrap command log exceeds bound')
            row[side + 'SHA256'] = record['sha256']
    builder = value['builder']
    if (not isinstance(builder['version'], str) or not 1 <= len(builder['version']) <= 256 or
            type(builder['memoryMiB']) is not int or not 128 <= builder['memoryMiB'] <= 2048 or
            type(builder['cpuMillis']) is not int or not 100 <= builder['cpuMillis'] <= 4000):
        raise ValueError('bootstrap builder allocation/version invalid')
    if (directory / 'probe.stdout.log').read_bytes() != b'luci-registry-gate 1\n' or (directory / 'probe.stderr.log').read_bytes():
        raise ValueError('bootstrap actual gate version probe differs')
    return value


def publish(repo, revision, source, image_id, gate_binary, distribution, evidence, output):
    """Offline publication after an explicit external build and image probe.

    No command is executed except clean-source Git reads. The original build,
    save and probe logs remain caller-owned evidence; failures never synthesize
    a success receipt or trigger a new Docker build.
    """
    repo, output = Path(repo).absolute(), Path(output).absolute()
    producer.clean_source(repo, revision)
    plain_directory(output.parent)
    if output.exists() or output.is_symlink() or output.is_relative_to(repo):
        raise ValueError('bootstrap publication requires fresh output outside source')
    facts = execution_evidence(evidence)
    pinned = registry_producer.verify_distribution(distribution)
    gate = archive.file_record(Path(gate_binary))
    if not 1 <= gate['bytes'] <= 64 << 20:
        raise ValueError('bootstrap gate binary exceeds bound')
    files = {row['file']: row for row in pinned['files']}
    with tempfile.TemporaryDirectory(prefix='luci-bootstrap-source-', dir=output.parent) as temp:
        staged = stage_gate(repo, revision, Path(temp) / 'source')
    compile_proof = read_json(Path(evidence) / 'gate-compile.json')
    if (not isinstance(compile_proof, dict) or set(compile_proof) != {'sourceCommit', 'gateSourceSHA256', 'gateBinarySHA256', 'argv', 'environment', 'exitStatus', 'stdoutSHA256', 'stderrSHA256'} or
            compile_proof['sourceCommit'] != revision or compile_proof['gateSourceSHA256'] != staged['gateSourceSHA256'] or
            compile_proof['gateBinarySHA256'] != gate['sha256'] or type(compile_proof['exitStatus']) is not int or compile_proof['exitStatus'] != 0 or
            compile_proof['environment'] != {'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': 'amd64', 'GOWORK': 'off', 'GOTOOLCHAIN': 'local'} or
            not isinstance(compile_proof['argv'], list) or len(compile_proof['argv']) != 7 or
            compile_proof['argv'][1:] != ['build', '-trimpath', '-buildvcs=false', '-o', 'luci-registry-gate', '.']):
        raise ValueError('bootstrap successful source-bound static gate compile proof required')
    for side in ('stdout', 'stderr'):
        log = archive.file_record(Path(evidence) / ('gate-compile.' + side + '.log'))
        if log['bytes'] > 16 << 20 or log['sha256'] != compile_proof[side + 'SHA256']:
            raise ValueError('bootstrap compile log changed')
    output.mkdir(mode=0o700)
    result = normalize(source, output / NAME, image_id, gate['sha256'], files['registry']['sha256'])
    receipt = {'version': 1, 'kind': 'luci-runtime-registry-bootstrap', 'buildSourceCommit': revision,
        'gateSourceSHA256': staged['gateSourceSHA256'], 'buildRecipeSHA256': staged['buildRecipeSHA256'],
        'distribution': {'version': '3.0.0', 'archiveSHA256': pinned['archiveSHA256'],
                         'binarySHA256': files['registry']['sha256'], 'licenseSHA256': files['LICENSE']['sha256']},
        'builder': facts['builder'], 'probe': facts['probe'],
        'image': {'id': image_id, 'configSHA256': result['configSHA256'], 'configBytes': result['configBytes'],
                  'layers': result['layers'], 'archiveSHA256': result['sha256'], 'archiveBytes': result['bytes']}}
    producer.clean_source(repo, revision)
    pull_metadata.write(output / RECEIPT, receipt)
    pull_metadata.write(output / 'bootstrap-source-inventory.json', staged)
    pull_metadata.write(output / 'gate-compile.json', compile_proof)
    license_target = output / 'licenses/runtime-bootstrap/distribution/LICENSE'
    license_target.parent.mkdir(parents=True)
    shutil.copyfile(Path(distribution) / 'LICENSE', license_target)
    result.update(distributionVersion='3.0.0', distributionArchiveSHA256=pinned['archiveSHA256'],
                  distributionBinarySHA256=files['registry']['sha256'], distributionLicenseSHA256=files['LICENSE']['sha256'],
                  gateSourceSHA256=staged['gateSourceSHA256'], gateBinarySHA256=gate['sha256'],
                  buildRecipeSHA256=staged['buildRecipeSHA256'], buildReceiptSHA256=archive.file_record(output / RECEIPT)['sha256'])
    pull_metadata.write(output / 'bootstrap-manifest-fragment.json', result)
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='action', required=True)
    stage = commands.add_parser('stage-source')
    finish = commands.add_parser('publish')
    for command in (stage, finish):
        command.add_argument('--repo', type=Path, required=True)
        command.add_argument('--revision', required=True)
        command.add_argument('--output', type=Path, required=True)
    for key in ('source', 'gate-binary', 'distribution', 'evidence'):
        finish.add_argument('--' + key, type=Path, required=True)
    finish.add_argument('--image-id', required=True)
    args = parser.parse_args()
    try:
        if args.action == 'stage-source':
            result = stage_gate(args.repo, args.revision, args.output)
        else:
            result = publish(args.repo, args.revision, args.source, args.image_id, args.gate_binary,
                             args.distribution, args.evidence, args.output)
    except (ValueError, OSError, producer.subprocess.SubprocessError) as error:
        parser.exit(1, 'Bootstrap refused; retain original build/probe evidence: ' + str(error) + '\n')
    print(json.dumps(result, sort_keys=True))
