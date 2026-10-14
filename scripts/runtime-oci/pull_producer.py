"""Explicit owned v2 producer. Never called by installed setup or packaging."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import threading
import time
import uuid

import archive
import bootstrap
from canonical_inputs import stage
from common import plain_directory, read_json, schema_validate, write_json
from finalize import completed_proof
import payload as legacy
import producer
import pull_export
import pull_metadata
import pull_payload
import registry_export
import registry_producer
import registry_store


def fresh_tag(tag):
    try:
        producer.command(['docker', 'image', 'inspect', tag])
    except producer.CommandFailure as error:
        if error.operation == ['docker', 'image'] and re.search(rb'No such image:', error.stderr):
            return
        raise
    raise ValueError('fresh producer tag already exists; preserve it')


def watch_push(process, stop, output, failures):
    """100 ms native reserve watch independent of blocking Docker inventory."""
    deadline = time.monotonic() + 1200
    try:
        previous = producer.cpu_sample()
        with (output / 'push-fast-resource-monitor.jsonl').open('x', encoding='utf-8', newline='\n') as log:
            while not stop.is_set() and process.poll() is None:
                free, current = producer.available_memory(), producer.cpu_sample()
                row = {'time': time.time(), 'availableBytes': free}
                if current[1] > previous[1]:
                    row['cpuBusyPercent'] = 100 * (1 - (current[0] - previous[0]) / (current[1] - previous[1]))
                previous = current
                log.write(json.dumps(row, sort_keys=True) + '\n')
                log.flush()
                if free < producer.RESERVE + producer.MARGIN or time.monotonic() > deadline or any((output / name).stat().st_size > 16 << 20 for name in ('push.stdout.log', 'push.stderr.log')):
                    failures.append('producer push exceeded host margin, 20-minute deadline or output bound')
                    process.terminate()  # Client termination never settles daemon push.
                    return
                stop.wait(0.1)
    except (ValueError, OSError) as error:
        failures.append('producer push native watch failed: ' + str(error))
        if process.poll() is None:
            process.terminate()


def push(helper):
    output = helper.output
    args = ['docker', 'image', 'push', helper.owner['tag']]
    helper.fence()
    write_json(output / 'registry-push-intent.json', {'session': helper.session, 'engineID': helper.engine,
        'imageID': helper.image_id, 'argv': args, 'outcome': 'pending-no-automatic-retry'})
    failures, stop = [], threading.Event()
    with (output / 'push.stdout.log').open('xb') as stdout, (output / 'push.stderr.log').open('xb') as stderr:
        client = producer.subprocess.Popen(args, stdin=producer.subprocess.DEVNULL, stdout=stdout, stderr=stderr)
        watcher = threading.Thread(target=watch_push, args=(client, stop, output, failures), name='luci-push-reserve-watch')
        watcher.start()
        try:
            while client.poll() is None:
                helper.fence()  # Exact engine, full inventory and no foreign work.
                stop.wait(0.5)
            if failures or client.returncode:
                raise ValueError('producer push failed/uncertain: ' + '; '.join(failures))
        finally:
            if client.poll() is None:
                client.terminate()
                client.wait(timeout=5)
            stop.set()
            watcher.join(timeout=2)
            if watcher.is_alive():
                raise ValueError('producer push monitor did not retire')
    helper.fence()
    status = registry_producer.validate_status(archive.json_bytes(registry_producer.read_loopback(helper.owner, '/luci/status')), helper.owner, completed=True)
    terminal = {'session': helper.session, 'engineID': helper.engine, 'imageID': helper.image_id,
                'commandExitStatus': client.returncode, 'terminalStreamSHA256': archive.file_record(output / 'push.stdout.log')['sha256'],
                'stderrSHA256': archive.file_record(output / 'push.stderr.log')['sha256'], 'manifestDigest': status['manifestDigest'],
                'terminalStreamReturned': True, 'serialGateManifestCommitted': True, 'engineRetired': False}
    write_json(output / 'registry-push-terminal.json', terminal)
    return terminal


def admit_push(helper):
    # The placement helper is intentionally running now. Its fence admits
    # only the exact owned generation; the earlier idle-engine admission
    # cannot be reused after placement.
    helper.fence()
    if producer.available_memory() < helper.estimate['requiredBytes'] or producer.cpu_busy() > 90:
        raise ValueError('producer push lacks fresh host reserve/CPU margin after placement')


def produce(repo, revision, build_repo, original, v1_payload, bootstrap_dir, distribution, daemon_probe, instance, output):
    repo, build_repo, original, v1_payload, bootstrap_dir, output = map(lambda p: Path(p).absolute(),
        (repo, build_repo, original, v1_payload, bootstrap_dir, output))
    if not re.fullmatch(r'[\x21-\x7e]{1,160}', instance):
        raise ValueError('producer instance identity required')
    producer.clean_source(repo, revision)
    pending, terminal, _, image_id = completed_proof(original)
    producer.clean_source(build_repo, pending['sourceCommit'])
    v1 = legacy.verify(v1_payload)
    if v1['sourceCommit'] != pending['sourceCommit'] or v1['producer']['buildSourceCommit'] != pending['sourceCommit'] or v1['image']['id'] != image_id:
        raise ValueError('genuine original v1 manifest/build receipt/image required')
    plain_directory(output.parent)
    if output.exists() or output.is_symlink() or any(output.is_relative_to(p) for p in (repo, build_repo, original, v1_payload, bootstrap_dir)):
        raise ValueError('producer requires a fresh independent evidence output')
    if shutil.disk_usage(output.parent).free < 10 << 30:
        raise ValueError('v2 producer requires 10 GiB bounded scratch reserve')
    with tempfile_inputs(output.parent) as temp:
        inputs = stage(repo, revision, temp / 'canonical')
        if inputs['sourceFiles'] != v1['sourceFiles'] or inputs['canonicalInputDigest'] != v1['canonicalInputDigest']:
            raise ValueError('producer source changes original runtime inputs')
    b = read_json(bootstrap_dir / 'bootstrap-manifest-fragment.json')
    # Bootstrap publication is independently reverified before Docker effects.
    with tempfile_inputs(output.parent) as temp:
        normalized = bootstrap.normalize(bootstrap_dir / bootstrap.NAME, temp / bootstrap.NAME, b['imageID'], b['gateBinarySHA256'], b['distributionBinarySHA256'])
        if any(normalized[k] != b[k] for k in normalized) or archive.file_record(bootstrap_dir / bootstrap.RECEIPT)['sha256'] != b['buildReceiptSHA256']:
            raise ValueError('producer bootstrap identity changed')
    engine = producer.command(['docker', 'info', '--format', '{{.ID}}']).strip().decode('ascii')
    inspected = archive.json_bytes(producer.command(['docker', 'image', 'inspect', image_id]))[0]
    if (engine != terminal['engineID'] or inspected.get('Id') != image_id or inspected.get('Config', {}).get('Labels', {}).get(producer.SESSION_LABEL) != pending['session'] or
            inspected.get('RootFS', {}).get('Layers') != v1['image']['diffIDs']):
        raise ValueError('producer exact original engine/image/owner changed')
    session = uuid.uuid4().hex
    output.mkdir(mode=0o700)
    helper = registry_producer.Helper(repo, image_id, session, output, distribution, daemon_probe)
    write_json(output / 'producer-engine.json', {'session': session, 'engineID': engine, 'instanceID': instance})
    write_json(output / 'pull-producer-intent.json', {'sourceCommit': revision, 'buildSourceCommit': pending['sourceCommit'],
        'imageID': image_id, 'session': session, 'imageRebuilt': False, 'ordinaryInstalledAcceptance': False})
    try:
        helper.start()
        admit_push(helper)
        for tag in (helper.owner['retainedTag'], helper.owner['tag']):
            fresh_tag(tag)
            helper.operation(['docker', 'image', 'tag', image_id, tag])
        terminal_push = push(helper)
        digest = terminal_push['manifestDigest']
        converted = pull_export.readback('http://127.0.0.1:' + str(helper.owner['port']), helper.owner['repository'],
            digest, image_id, v1['image']['diffIDs'], output / 'readback')
        helper.retire()
        store = registry_store.create(output / 'readback', converted, image_id, v1['image']['diffIDs'], output / 'payload')
        # Keep a private retained tag on the selected image before removing its
        # transport tag. No shared image/config or layer is deleted.
        helper.fence()
        for tag in (helper.owner['tag'], helper.owner['retainedTag']):
            if archive.json_bytes(producer.command(['docker', 'image', 'inspect', tag]))[0]['Id'] != image_id:
                raise ValueError('producer tag identity changed; preserve both tags')
        helper.operation(['docker', 'image', 'rm', helper.owner['tag']])
        target = output / 'payload'
        stage(repo, revision, target / 'canonical')
        for name in (legacy.RECEIPT,):
            shutil.copyfile(v1_payload / name, target / name)
        shutil.copyfile(v1_payload / legacy.MANIFEST, target / legacy.PRIOR_MANIFEST)
        shutil.copyfile(v1_payload / legacy.RECEIPT, target / legacy.PRIOR_RECEIPT)
        for name in (bootstrap.NAME, bootstrap.RECEIPT):
            shutil.copyfile(bootstrap_dir / name, target / name)
        licenses = copy_licenses(v1_payload, bootstrap_dir, target, v1, b)
        write_json(target / legacy.SOURCE, {'version': 1, 'sourceCommit': revision, 'sourceDirty': False})
        proof = {'version': 2, 'kind': 'luci-runtime-pull-producer', 'sourceCommit': revision,
            'canonicalInputDigest': inputs['canonicalInputDigest'], 'selectedImageID': image_id,
            'originalBuildReceiptSHA256': archive.file_record(target / legacy.RECEIPT)['sha256'],
            'conversionReceiptSHA256': archive.file_record(target / registry_store.RECEIPT_NAME)['sha256'],
            'registryStoreSHA256': store['registryStore']['sha256'], 'registryManifestDigest': digest,
            'push': {'session': session, 'repository': helper.owner['repository'], 'engineID': engine, 'instanceID': instance,
                'imageID': image_id, 'commandExitStatus': 0, 'terminalStreamSHA256': terminal_push['terminalStreamSHA256'],
                'manifestReadbackDigest': digest, 'helperTerminalReceiptSHA256': archive.file_record(helper.data / 'luci-serial-gate-terminal-v1.json')['sha256'],
                'helperRetirementReceiptSHA256': archive.file_record(output / 'registry-helper-retirement.json')['sha256']}}
        pull_metadata.write(target / pull_payload.PULL_RECEIPT, proof)
        result = {'version': 2, 'kind': 'luci-runtime-registry-pull', 'sourceCommit': revision, 'target': v1['target'],
            'canonicalInputAlgorithm': v1['canonicalInputAlgorithm'], 'canonicalInputDigest': inputs['canonicalInputDigest'],
            'sourceFiles': inputs['sourceFiles'], 'runtime': v1['runtime'], 'registryStore': store['registryStore'], 'bootstrap': b,
            'image': {'id': image_id, 'configSHA256': image_id[7:], 'configBytes': store['receipt']['config']['bytes'],
                      'manifestDigest': digest, 'manifestMediaType': registry_export.DOCKER_MANIFEST,
                      'layers': [dict(row, mediaType=registry_export.DOCKER_LAYER) for row in store['receipt']['layers']]},
            'licenses': licenses, 'producer': {'originalBuildReceiptSHA256': proof['originalBuildReceiptSHA256'],
                'conversionReceiptSHA256': proof['conversionReceiptSHA256'], 'pullProducerReceiptSHA256': archive.file_record(target / pull_payload.PULL_RECEIPT)['sha256']}}
        if revision != pending['sourceCommit']:
            reuse = pull_payload.reuse_record(target, result, v1, read_json(target / legacy.RECEIPT))
            write_json(target / legacy.REUSE, reuse)
            result['producer']['reuseEvidenceSHA256'] = archive.file_record(target / legacy.REUSE)['sha256']
        schema_validate(result, 'runtime-oci-pull-manifest-v2.schema.json')
        pull_metadata.write(target / pull_payload.MANIFEST, result)
        pull_payload.verify(target)
        producer.clean_source(repo, revision)
        producer.clean_source(build_repo, pending['sourceCommit'])
        write_json(output / 'pull-producer-complete.json', {'version': 2, 'sourceCommit': revision, 'imageID': image_id,
            'manifestSHA256': archive.file_record(target / pull_payload.MANIFEST)['sha256'], 'helperRetired': True,
            'selectedImagePreserved': True, 'retainedTag': helper.owner['retainedTag'], 'ordinaryInstalledAcceptance': False})
        return target
    except BaseException as error:
        write_json(output / 'registry-generation-unknown.json', {'session': session, 'engineID': engine,
            'containerID': helper.container_id, 'networkID': helper.network_id, 'imageID': image_id,
            'pushStarted': (output / 'registry-push-intent.json').exists(), 'reason': str(error)[:2048],
            'automaticRetry': False, 'absenceOnlySettlement': False})
        raise


def tempfile_inputs(parent):
    import tempfile
    from contextlib import contextmanager
    @contextmanager
    def directory():
        with tempfile.TemporaryDirectory(prefix='luci-v2-check-', dir=parent) as value:
            yield Path(value)
    return directory()


def copy_licenses(v1_payload, bootstrap_dir, target, v1, b):
    licenses = [dict(row) for row in v1['licenses']]
    for row in licenses:
        shutil.copyfile(v1_payload / row['path'], legacy.destination_file(target, row['path']))
    notice = bootstrap_dir / 'licenses/runtime-bootstrap/distribution/LICENSE'
    record = archive.file_record(notice, 'licenses/runtime-bootstrap/distribution/LICENSE')
    if record['sha256'] != b['distributionLicenseSHA256']:
        raise ValueError('bootstrap original Distribution grant changed')
    shutil.copyfile(notice, legacy.destination_file(target, record['path']))
    licenses.append(dict(record, component='distribution/distribution', version='3.0.0', grant='Apache-2.0',
                         provenance='https://github.com/distribution/distribution/tree/v3.0.0'))
    return sorted(licenses, key=lambda row: row['path'])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--instance-id', required=True)
    for key in ('repo', 'build-repo', 'original', 'v1-payload', 'bootstrap', 'distribution', 'daemon-probe', 'output'):
        parser.add_argument('--' + key, type=Path, required=True)
    args = parser.parse_args()
    try:
        result = produce(args.repo, args.revision, args.build_repo, args.original, args.v1_payload,
                         args.bootstrap, args.distribution, args.daemon_probe, args.instance_id, args.output)
    except (ValueError, OSError, producer.subprocess.SubprocessError) as error:
        parser.exit(1, 'V2 producer refused/unknown; preserve all exact generation evidence: ' + str(error) + '\n')
    print(json.dumps({'payload': str(result), 'ordinaryInstalledAcceptance': False}))
