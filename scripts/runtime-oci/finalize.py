"""Finish a positively completed owned producer image without recompilation.

Original source, builder terminal, context, image and logs remain unchanged.
This is never an arbitrary-image adoption or an unknown-builder recovery path.
"""
import argparse
import json
from pathlib import Path
import re
import shutil
import tarfile
import tempfile
import uuid

import archive
from canonical_inputs import stage
from common import plain_directory, read_json, write_json
import payload
import producer


def completed_proof(original):
    original = plain_directory(original)
    pending = read_json(original / 'producer-pending.json')
    terminal = read_json(original / 'producer-terminal.json')
    engine = read_json(original / 'producer-engine.json')
    if (original / 'producer-unknown.json').exists() or type(terminal.get('builderExitCode')) is not int or terminal.get('builderExitCode') != 0 or terminal.get('cancelRequested') is not False or terminal.get('terminalStreamReturned') is not True or terminal.get('exactOwnedContainersRemaining') != [] or terminal.get('session') != pending.get('session') or terminal.get('engineID') != engine.get('engineID'):
        raise ValueError('resume requires positive original builder terminal and exact owned cleanup; unknown/failed work refused')
    if not re.fullmatch(r'[a-f0-9]{32}', pending.get('session', '')) or not re.fullmatch(r'[a-f0-9]{40}', pending.get('sourceCommit', '')):
        raise ValueError('original producer source/session identity invalid')
    archive.regular(original / 'image.id')
    image_id = (original / 'image.id').read_text(encoding='utf-8').strip()
    if not re.fullmatch(r'sha256:[a-f0-9]{64}', image_id):
        raise ValueError('original producer immutable image ID missing')
    expected_argv = ['docker', 'build', '--memory', str(producer.BUILD_MEMORY), '--memory-swap', str(producer.BUILD_MEMORY), '--cpu-period', '100000', '--cpu-quota', '100000', '--rm', '--force-rm', '--iidfile', str(original / 'image.id'), '-f', 'images/sandbox/Dockerfile', '-']
    if pending.get('argv') != expected_argv:
        raise ValueError('original builder invocation does not establish the bounded build contract')
    monitor = original / 'producer-resource-monitor.jsonl'
    if archive.file_record(monitor)['bytes'] > 16 << 20:
        raise ValueError('original resource monitor exceeds bound')
    observed = [archive.json_bytes(line.encode('utf-8')) for line in monitor.read_text(encoding='utf-8').splitlines() if line]
    if not observed or any(row.get('availableBytes', 0) < producer.RESERVE or row.get('cancellationRequested') for row in observed):
        raise ValueError('original producer breached its reserve or was cancelled')
    build_containers = [item for row in observed if 'phase' not in row for item in row.get('containers', [])]
    if not build_containers or any(item.get('memory') != producer.BUILD_MEMORY or item.get('memorySwap') != producer.BUILD_MEMORY or item.get('cpuPeriod') != 100000 or item.get('cpuQuota') != 100000 or item.get('oomKilled') is not False for item in build_containers):
        raise ValueError('original RUN hard-limit/OOM evidence incomplete or inconsistent')
    # Preserve and check the original version witnesses even though finalization
    # repeats the short probes to produce durable actual argv/result receipts.
    for tool, version in (('sandboxd', '0.5.0'), ('codex', '0.160.0'), ('claudeCode', '2.1.274'), ('openCode', '1.18.34')):
        for suffix in ('.out.log', '.err.log'):
            record = archive.file_record(original / (tool + suffix))
            if record['bytes'] > 1 << 20:
                raise ValueError('original version witness exceeds bound')
        text = (original / (tool + '.out.log')).read_text(encoding='utf-8')
        if not re.search(r'(?<![0-9.])' + re.escape(version) + r'(?![0-9.])', text) or tool == 'sandboxd' and text != 'sandboxd 0.5.0\n':
            raise ValueError('original actual upstream version witness missing')
    return pending, terminal, engine, image_id


def finalize(repo, revision, build_repo, original, output, assistance=False):
    repo, build_repo = plain_directory(repo), plain_directory(build_repo)
    original, output = plain_directory(original), Path(output).absolute()
    producer.clean_source(repo, revision)
    pending, terminal, engine, image_id = completed_proof(original)
    build_revision = pending['sourceCommit']
    producer.clean_source(build_repo, build_revision)
    plain_directory(output.parent)
    if output.exists() or output.is_relative_to(repo) or output.is_relative_to(build_repo) or output.is_relative_to(original):
        raise ValueError('finalization needs a fresh private output outside source/original evidence')
    if producer.docker_ids() or producer.available_memory() < producer.RESERVE + producer.BUILD_MEMORY + producer.MARGIN:
        raise ValueError('finalization requires idle Docker and unchanged reserve/demand/margin')
    busy = producer.cpu_busy()
    if busy > 90:
        raise ValueError('finalization requires observed CPU margin for one bounded CPU')
    if shutil.disk_usage(output.parent).free < 2 * archive.MAX_ARCHIVE + (2 << 30):
        raise ValueError('finalization needs raw plus normalized archive disk budget')
    current = json.loads(producer.command(['docker', 'info', '--format', '{{json .}}']))
    if current['ID'] != terminal['engineID']:
        raise ValueError('original builder engine differs; no arbitrary image adoption')
    producer.assert_settled(pending['session'])
    inspected = json.loads(producer.command(['docker', 'image', 'inspect', image_id]))[0]
    if inspected['Id'] != image_id or (inspected['Config'].get('Labels') or {}).get(producer.SESSION_LABEL) != pending['session']:
        raise ValueError('image is not the exact original owned producer result')
    archive.config_policy({'os': inspected['Os'], 'architecture': inspected['Architecture'], 'config': inspected['Config'], 'rootfs': {'type': 'layers', 'diff_ids': inspected['RootFS']['Layers']}})
    bases = []
    with tarfile.open(original / 'context.tar', 'r:') as context:
        members = archive.scan(context)
        item = members.get('images/sandbox/Dockerfile')
        if item is None or item.size > 1 << 20:
            raise ValueError('original generated Dockerfile missing')
        generated_file = context.extractfile(item).read().decode('utf-8')
    immutable_bases = re.findall(r'^FROM (sha256:[a-f0-9]{64})(?: AS build)?$', generated_file, re.M)
    if len(immutable_bases) != 2:
        raise ValueError('original generated context lacks exact immutable bases')
    for reference, image in zip(('golang:1.26-bookworm', 'node:24-bookworm-slim'), immutable_bases):
        value = json.loads(producer.command(['docker', 'image', 'inspect', image]))[0]
        if value['Id'] != image or value.get('Os') != 'linux' or value.get('Architecture') != 'amd64' or not value.get('RepoDigests'):
            raise ValueError('original immutable base evidence unavailable')
        bases.append({'reference': reference, 'imageID': image, 'repoDigest': sorted(value['RepoDigests'])[0]})
    output.mkdir(mode=0o700)
    inputs = stage(build_repo, build_revision, output / 'canonical')
    if payload.source_inventory(original / 'canonical') != inputs['sourceFiles'] or pending['canonicalInputDigest'] != inputs['canonicalInputDigest']:
        raise ValueError('original canonical source differs from immutable build revision')
    with tempfile.TemporaryDirectory(prefix='luci-finalizer-inputs-', dir=output.parent) as temporary:
        selected = stage(repo, revision, Path(temporary) / 'canonical')
        if selected['sourceFiles'] != inputs['sourceFiles'] or selected['canonicalInputDigest'] != inputs['canonicalInputDigest']:
            raise ValueError('finalizer changed runtime inputs; produce a new runtime')
    generated = producer.context_tar(output / 'canonical', output / 'context.tar', bases, pending['session'])
    if generated != archive.file_record(original / 'context.tar')['sha256']:
        raise ValueError('original generated builder context cannot be reconstructed exactly')
    for name in ('build.out.log', 'build.err.log'):
        if archive.file_record(original / name)['bytes'] > 16 << 20:
            raise ValueError('original builder log exceeds bound')
        shutil.copyfile(original / name, output / name)
    for file in ('producer-pending.json', 'producer-terminal.json', 'producer-engine.json'):
        shutil.copyfile(original / file, output / ('original-' + file))
    for tool in ('sandboxd', 'codex', 'claudeCode', 'openCode'):
        for suffix in ('.out.log', '.err.log'):
            shutil.copyfile(original / (tool + suffix), output / ('original-' + tool + suffix))
    log = original / 'producer-cache-assistance.jsonl'
    actions = 0
    if log.exists():
        if archive.file_record(log)['bytes'] > 16 << 20:
            raise ValueError('original cache assistance log exceeds bound')
        shutil.copyfile(log, output / 'original-producer-cache-assistance.jsonl')
        shutil.copyfile(log, output / 'producer-cache-assistance.jsonl')
        actions = sum(1 for line in log.read_text(encoding='utf-8').splitlines() if line)
    session = uuid.uuid4().hex
    producer.ENGINE_IDENTITIES[session] = current['ID']
    write_json(output / 'canonical-inputs.json', inputs)
    write_json(output / 'producer-engine.json', {'engineID': current['ID'], 'operatingSystem': current.get('OperatingSystem', ''), 'session': session, 'availableBytes': producer.available_memory(), 'cpuBusyPercent': busy})
    write_json(output / 'producer-finalization-intent.json', {'version': 1, 'finalizerSourceCommit': revision, 'buildSourceCommit': build_revision, 'originalSession': pending['session'], 'session': session, 'imageID': image_id, 'generatedContextSHA256': generated, 'originalTerminalSHA256': archive.file_record(original / 'producer-terminal.json')['sha256'], 'recompile': False})
    result = producer.finalize_image(build_repo, build_revision, output, session, inputs, bases, generated, pending['argv'], image_id, actions, assistance, license_repo=repo)
    producer.clean_source(repo, revision)
    producer.clean_source(build_repo, build_revision)
    write_json(output / 'producer-finalization-complete.json', {'version': 1, 'finalizerSourceCommit': revision, 'buildSourceCommit': build_revision, 'imageID': image_id, 'manifestSHA256': archive.file_record(output / payload.MANIFEST)['sha256'], 'recompile': False, 'ordinaryInstalledAcceptance': False})
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--revision', required=True, help='Full clean finalizer checkout HEAD')
    for name in ('build-repo', 'original', 'output'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--producer-wsl-cache-assistance', action='store_true')
    args = parser.parse_args()
    try:
        result = finalize(Path(__file__).resolve().parents[2], args.revision, args.build_repo, args.original, args.output, args.producer_wsl_cache_assistance)
    except (ValueError, OSError, producer.subprocess.SubprocessError) as error:
        parser.exit(1, 'Finalization refused/failed; retain original and new evidence: ' + str(error) + '\n')
    print(json.dumps({'imageID': result['image']['id'], 'archiveBytes': result['archive']['bytes'], 'output': str(args.output)}))
