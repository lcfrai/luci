"""Owned release-producer registry transport; never ordinary consumer setup.

The helper has a hard Docker allocation. Daemon compression and host conversion
are separately estimated/observed, not falsely represented as container-capped.
"""
from __future__ import annotations

import json
from pathlib import Path
import re
import hashlib
import http.client
import shutil
import socket
import time
import os
import argparse
import uuid

import archive
from common import read_json, plain_directory, write_json
import producer

HELPER_MEMORY = 512 << 20
DAEMON_MEMORY_FLOOR = 1 << 30
DAEMON_LAYER_COPIES = 1  # Provisional source-based estimate, not a daemon cap.


def verify_distribution(directory):
    """Only the reviewed official release binary/grants can enter the helper."""
    directory = Path(directory)
    pinned = read_json(Path(__file__).with_name('distribution-v3.0.0.json'))
    if pinned['component'] != 'distribution/distribution' or pinned['versionString'] != '3.0.0':
        raise ValueError('unreviewed Distribution version')
    for row in pinned['files']:
        actual = archive.file_record(directory / row['file'])
        if actual['sha256'] != row['sha256'] or actual['bytes'] != row['bytes']:
            raise ValueError('pinned Distribution binary/grant bytes differ')
    raw = archive.file_record(directory / 'registry_3.0.0_linux_amd64.tar.gz')
    if raw['sha256'] != pinned['archiveSHA256'] or raw['bytes'] != pinned['archiveBytes']:
        raise ValueError('pinned Distribution archive differs')
    proof = archive.file_record(directory / 'upstream-provenance.json')
    if proof['sha256'] != pinned['upstreamProvenanceSHA256'] or proof['bytes'] > 1 << 20:
        raise ValueError('pinned Distribution release provenance differs')
    value = read_json(directory / 'upstream-provenance.json')
    if not any(row.get('name') == 'registry_3.0.0_linux_amd64.tar.gz' and row.get('digest') == {'sha256': raw['sha256']} for row in value.get('subject', [])):
        raise ValueError('Distribution provenance does not bind archive')
    return pinned


def validate_history(image_id, image_size, rows):
    if not isinstance(image_id, str) or not archive.HEX.fullmatch(image_id.removeprefix('sha256:')) or not image_id.startswith('sha256:'):
        raise ValueError('exact original image ID required')
    if type(image_size) is not int or not 0 < image_size <= archive.MAX_ARCHIVE or not isinstance(rows, list) or not 1 <= len(rows) <= 2048 or not all(isinstance(row, dict) for row in rows) or rows[0].get('ID') != image_id:
        raise ValueError('image history is not the exact original image')
    sizes = []
    for row in rows:
        value = row.get('Size')
        if not isinstance(value, str) or not re.fullmatch(r'0|[1-9][0-9]{0,11}', value):
            raise ValueError('image history must use exact --human=false integer sizes')
        sizes.append(int(value))
    if sum(sizes) != image_size or max(sizes) == 0:
        raise ValueError('history layer sizes do not sum to exact inspected image size')
    return {'imageID': image_id, 'imageSizeBytes': image_size, 'largestLayerBytes': max(sizes), 'historySizeBytes': sizes}


def original_history(image_id, inspected):
    if inspected.get('Id') != image_id:
        raise ValueError('original image changed before history read')
    raw = producer.command(['docker', 'image', 'history', '--no-trunc', '--human=false', '--format', '{{json .}}', image_id])
    if len(raw) > 1 << 20:
        raise ValueError('image history exceeds source bound')
    rows = [archive.json_bytes(line) for line in raw.splitlines()]
    return validate_history(image_id, inspected['Size'], rows)


def resource_estimate(history):
    daemon = max(DAEMON_MEMORY_FLOOR, history['largestLayerBytes'] * DAEMON_LAYER_COPIES)
    return {'helperMemoryBytes': HELPER_MEMORY, 'daemonEstimatedMemoryBytes': daemon,
            'daemonLayerCopiesEstimate': DAEMON_LAYER_COPIES, 'daemonHardMemoryCap': False,
            'requiredBytes': producer.RESERVE + producer.MARGIN + HELPER_MEMORY + daemon,
            'reserveBytes': producer.RESERVE, 'marginBytes': producer.MARGIN}


def admit_registry(estimate):
    running = producer.docker_ids()
    cpu = producer.cpu_busy()
    available = producer.available_memory()
    observation = {'nativeAvailableBytes': available, 'cpuBusyPercent': cpu, 'runningCount': len(running), **estimate}
    if running or cpu > 90 or available < estimate['requiredBytes']:
        raise ValueError('registry producer admission refused: ' + json.dumps(observation, sort_keys=True))
    return observation


def identity(session, port):
    if not isinstance(session, str) or not re.fullmatch('[0-9a-f]{32}', session) or type(port) is not int or not 1024 <= port <= 65535:
        raise ValueError('invalid producer registry generation or port')
    repository = 'luci/runtime-export-' + session
    return {'session': session, 'repository': repository, 'networkName': 'luci-runtime-export-' + session,
            'containerName': 'luci-producer-' + session + '-registry', 'port': port,
            'tag': '127.0.0.1:' + str(port) + '/' + repository + ':payload',
            'retainedTag': 'luci/producer-retained:' + session}


def helper_args(image_id, owner, tools, data):
    tools, data = plain_directory(tools), plain_directory(data)
    if any(character in str(path) for path in (tools, data) for character in (',', '\n', '\r')):
        raise ValueError('helper bind path cannot use mount delimiters')
    env = {'LUCI_REGISTRY_SESSION': owner['session'], 'LUCI_REGISTRY_DATA': '/registry-data',
           'LUCI_REGISTRY_BINARY': '/tools/registry', 'LUCI_REGISTRY_LICENSE': '/tools/LICENSE',
           'LUCI_REGISTRY_BACKEND_PORT': '5001', 'LUCI_REGISTRY_FRONT_PORT': '5000',
           'LUCI_REGISTRY_PUBLIC_PORT': str(owner['port'])}
    args = ['docker', 'create', '--name', owner['containerName'], '--label', producer.SESSION_LABEL + '=' + owner['session'],
            '--network', owner['networkName'], '--publish', '127.0.0.1:' + str(owner['port']) + ':5000',
            '--memory', str(HELPER_MEMORY), '--memory-swap', str(HELPER_MEMORY), '--cpus', '1', '--pids-limit', '64',
            '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
            '--mount', 'type=bind,src=' + str(tools) + ',dst=/tools,readonly',
            '--mount', 'type=bind,src=' + str(data) + ',dst=/registry-data',
            '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=33554432',
            '--log-driver', 'json-file', '--log-opt', 'max-size=10m', '--log-opt', 'max-file=1']
    for key, value in env.items():
        args.extend(['--env', key + '=' + value])
    return args + ['--entrypoint', 'node', image_id, '--max-old-space-size=128', '/tools/registry_gate.mjs']


def validate_helper(row, owner, image_id, tools=None, data=None):
    host = row.get('HostConfig', {})
    ports = host.get('PortBindings')
    if (row.get('Image') != image_id or row.get('Config', {}).get('Labels', {}).get(producer.SESSION_LABEL) != owner['session'] or
            host.get('Memory') != HELPER_MEMORY or host.get('MemorySwap') != HELPER_MEMORY or host.get('NanoCpus') != 1000000000 or
            host.get('PidsLimit') != 64 or host.get('ReadonlyRootfs') is not True or host.get('Privileged') is not False or
            set(host.get('CapDrop') or []) != {'ALL'} or host.get('CapAdd') or host.get('Devices') or
            'no-new-privileges:true' not in (host.get('SecurityOpt') or []) or host.get('NetworkMode') != owner['networkName'] or
            ports != {'5000/tcp': [{'HostIp': '127.0.0.1', 'HostPort': str(owner['port'])}]}):
        raise ValueError('helper allocation, isolation, identity or loopback binding differs')
    mounts = row.get('Mounts', [])
    if len(mounts) != 2 or {item.get('Destination') for item in mounts} != {'/tools', '/registry-data'} or any(item.get('Type') != 'bind' or item.get('RW') != (item.get('Destination') == '/registry-data') for item in mounts):
        raise ValueError('helper mounts expose unexpected or writable code')
    if tools is not None and data is not None:
        for item in mounts:
            expected = str(tools if item['Destination'] == '/tools' else data).replace('\\', '/')
            actual = str(item.get('Source', '')).replace('\\', '/')
            if re.match(r'^[A-Za-z]:/', expected):
                choices = {expected.lower(), '/run/desktop/mnt/host/' + expected[0].lower() + expected[2:].lower()}
                actual = actual.lower()
            else:
                choices = {expected}
            if actual not in choices:
                raise ValueError('helper bind source differs from exact private generation')
    config = row.get('Config', {})
    if config.get('Entrypoint') != ['node'] or config.get('Cmd') != ['--max-old-space-size=128', '/tools/registry_gate.mjs'] or config.get('User') != 'dev':
        raise ValueError('helper executable or unprivileged user changed')
    return row


def validate_published_ports(row, owner):
    """HostConfig is intent; only NetworkSettings proves enforced publication."""
    ports = row.get('NetworkSettings', {}).get('Ports')
    expected = [{'HostIp': '127.0.0.1', 'HostPort': str(owner['port'])}]
    if not isinstance(ports, dict) or ports.get('5000/tcp') != expected or any(value for key, value in ports.items() if key != '5000/tcp'):
        raise ValueError('actual helper publication is missing, public or has extra bindings')
    return ports


def validate_network(row, owner, *, internal=False):
    if row.get('Name') != owner['networkName'] or row.get('Driver') != 'bridge' or row.get('Internal') is not internal or row.get('Labels', {}).get(producer.SESSION_LABEL) != owner['session'] or row.get('Containers'):
        raise ValueError('owned empty registry bridge identity changed')


def validate_daemon_witness(value, owner, engine, logs):
    if (not isinstance(value, dict) or value.get('version') != 1 or value.get('engineID') != engine or
            value.get('session') != owner['session'] or value.get('reference') != owner['tag'] or
            value.get('operation') != 'Engine.DistributionInspect' or value.get('outcome') != 'not_found' or
            value.get('registryAuthProvided') is not False or value.get('tagOrPushPerformed') is not False):
        raise ValueError('daemon-originated distribution inspection proof differs')
    path = ('/v2/' + owner['repository'] + '/manifests/payload').encode('ascii')
    matched = [line for line in logs.splitlines() if path in line and
               (b'http.response.status=404' in line or b'"http.response.status":404' in line)]
    if not matched:
        raise ValueError('fresh exact manifest not-found request witness unavailable')
    return {'matchingRequestCount': len(matched), 'requestPath': path.decode('ascii'),
            'rawLogsSHA256': hashlib.sha256(logs).hexdigest()}


def validate_status(value, owner, *, completed=False):
    if not isinstance(value, dict):
        raise ValueError('registry status must be an owned object')
    if value.get('version') != 1 or value.get('session') != owner['session'] or value.get('repository') != owner['repository'] or value.get('childReadyAtStart') is not True or value.get('ownershipSettled') is not False:
        raise ValueError('registry status identity changed')
    if completed:
        if value.get('upload') != 'idle' or value.get('activeUpload') is not None or value.get('manifest') != 'committed' or not re.fullmatch(r'sha256:[0-9a-f]{64}', value.get('manifestDigest') or ''):
            raise ValueError('registry push outcome remains uncertain')
    elif value.get('upload') != 'idle' or value.get('activeUpload') is not None or value.get('completedUploads') != 0 or value.get('manifest') != 'none':
        raise ValueError('new registry helper retained prior write state')
    return value


def validate_terminal(value, owner):
    if not isinstance(value, dict) or not isinstance(value.get('child'), dict) or value.get('manifest') is not None and not isinstance(value['manifest'], dict):
        raise ValueError('positive terminal receipt has an invalid shape')
    child = value.get('child') or {}
    if (value.get('version') != 1 or value.get('session') != owner['session'] or value.get('repository') != owner['repository'] or
            value.get('socketsClosed') is not True or value.get('upload') != 'idle' or
            value.get('verifiedExport') is not False or value.get('engineRetired') is not False or
            (value.get('manifest') or {}).get('phase') == 'publishing' or
            not (type(child.get('exitCode')) is int and child['exitCode'] == 0 and child.get('signal') is None or child.get('exitCode') is None and child.get('signal') == 'SIGTERM')):
        raise ValueError('positive gate/registry terminal receipt unavailable; retain unknown generation')
    return value


def read_loopback(owner, path, limit=1 << 20):
    connection = http.client.HTTPConnection('127.0.0.1', owner['port'], timeout=5)
    try:
        connection.request('GET', path, headers={'Connection': 'close'})
        response = connection.getresponse()
        raw = response.read(limit + 1)
        if response.status != 200 or response.getheader('Location') or len(raw) > limit:
            raise ValueError('owned loopback registry response refused')
        return raw
    finally:
        connection.close()


class Helper:
    """Exact helper generation. Failed retirement retains identities and data."""

    def __init__(self, repo, image_id, session, output, distribution, daemon_probe=None):
        self.repo, self.output = Path(repo), Path(output)
        self.image_id, self.session = image_id, session
        self.engine = producer.command(['docker', 'info', '--format', '{{.ID}}']).strip().decode('ascii')
        producer.ENGINE_IDENTITIES[session] = self.engine
        inspected = json.loads(producer.command(['docker', 'image', 'inspect', image_id]))[0]
        self.history = original_history(image_id, inspected)
        self.estimate = resource_estimate(self.history)
        self.distribution = verify_distribution(distribution)
        self.distribution_dir = Path(distribution)
        with socket.socket() as probe:
            probe.bind(('127.0.0.1', 0))
            port = probe.getsockname()[1]
        self.owner = identity(session, port)
        self.container_id, self.network_id = None, None
        self.tools, self.data = self.output / 'registry-tools', self.output / 'registry-data'
        self.daemon_probe = Path(daemon_probe) if daemon_probe else None

    def fence(self):
        if producer.command(['docker', 'info', '--format', '{{.ID}}']).strip().decode('ascii') != self.engine:
            raise ValueError('registry producer engine changed; retain owned generation')
        producer.owned_running(self.session)

    def operation(self, args):
        self.fence()
        with (self.output / 'registry-operations.jsonl').open('a', encoding='utf-8', newline='\n') as stream:
            stream.write(json.dumps({'time': time.time(), 'engineID': self.engine, 'session': self.session, 'argv': args, 'phase': 'intent'}, sort_keys=True) + '\n')
        result = producer.command(args, timeout=30)
        return result

    def start(self, *, placement_only=False):
        if self.daemon_probe is None:
            raise ValueError('source-bound daemon probe binary required before helper effects')
        probe_record = archive.file_record(self.daemon_probe)
        probe_identity = archive.json_bytes(producer.command([str(self.daemon_probe), '--identity']))
        revision = producer.command(['git', '-C', str(self.repo), 'rev-parse', 'HEAD']).strip().decode('ascii')
        if probe_record['bytes'] > 64 << 20 or probe_identity != {'version': 1, 'sourceCommit': revision}:
            raise ValueError('daemon probe compiled source identity differs')
        context = producer.command(['docker', 'context', 'show']).strip().decode('utf-8')
        endpoint = archive.json_bytes(producer.command(['docker', 'context', 'inspect', context]))[0]['Endpoints']['docker']['Host']
        if endpoint not in ('npipe:////./pipe/dockerDesktopLinuxEngine', 'npipe:////./pipe/docker_engine', 'unix:///var/run/docker.sock'):
            raise ValueError('daemon probe requires a supported selected local endpoint')
        write_json(self.output / 'daemon-probe-binary.json', {'sourceCommit': revision, 'binary': probe_record,
                   'selectedContext': context, 'endpoint': endpoint})
        estimate = self.estimate
        if placement_only:
            estimate = {'helperMemoryBytes': HELPER_MEMORY, 'daemonEstimatedMemoryBytes': 0, 'daemonHardMemoryCap': False,
                        'requiredBytes': producer.RESERVE + producer.MARGIN + HELPER_MEMORY,
                        'reserveBytes': producer.RESERVE, 'marginBytes': producer.MARGIN, 'placementOnlyNoPush': True}
        observation = admit_registry(estimate)
        write_json(self.output / 'registry-admission.json', observation)
        self.tools.mkdir(mode=0o700)
        self.data.mkdir(mode=0o700)
        for name in ('registry', 'LICENSE'):
            shutil.copyfile(self.distribution_dir / name, self.tools / name)
        gate = producer.command(['git', '-C', str(self.repo), 'show', 'HEAD:scripts/runtime-oci/registry_gate.mjs'])
        (self.tools / 'registry_gate.mjs').write_bytes(gate)
        if os.name != 'nt':
            (self.tools / 'registry').chmod(0o555)
        write_json(self.output / 'registry-helper-intent.json', {'owner': self.owner, 'engineID': self.engine, 'imageID': self.image_id,
                   'distribution': self.distribution, 'gateSHA256': hashlib.sha256(gate).hexdigest(), 'history': self.history, 'resourceEstimate': self.estimate, 'networkInternal': False})
        self.network_id = self.operation(['docker', 'network', 'create', '--driver', 'bridge', '--label', producer.SESSION_LABEL + '=' + self.session, self.owner['networkName']]).strip().decode('ascii')
        network = json.loads(producer.command(['docker', 'network', 'inspect', self.network_id]))[0]
        validate_network(network, self.owner)
        self.container_id = self.operation(helper_args(self.image_id, self.owner, self.tools, self.data)).strip().decode('ascii')
        inspected = json.loads(producer.command(['docker', 'inspect', self.container_id]))[0]
        validate_helper(inspected, self.owner, self.image_id, self.tools, self.data)
        write_json(self.output / 'registry-helper-created.json', {'engineID': self.engine, 'containerID': self.container_id, 'networkID': self.network_id, 'limitsVerified': True})
        self.operation(['docker', 'start', self.container_id])
        published = json.loads(producer.command(['docker', 'inspect', self.container_id]))[0]
        validate_helper(published, self.owner, self.image_id, self.tools, self.data)
        actual_ports = validate_published_ports(published, self.owner)
        write_json(self.output / 'registry-actual-port-bindings.json', actual_ports)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if producer.available_memory() < producer.RESERVE + producer.MARGIN:
                raise ValueError('helper readiness reached host pressure margin')
            try:
                status = validate_status(archive.json_bytes(read_loopback(self.owner, '/luci/status')), self.owner)
                read_loopback(self.owner, '/v2/')
                break
            except (OSError, ValueError, http.client.HTTPException):
                time.sleep(.25)
        else:
            raise ValueError('Windows loopback helper readiness not established')
        version = self.operation(['docker', 'exec', self.container_id, '/tools/registry', '--version'])
        if len(version) > 4096 or b'github.com/distribution/distribution/v3' not in version or not version.rstrip().endswith(b' 3.0.0'):
            raise ValueError('actual pinned Distribution version differs')
        (self.output / 'distribution-version.out.log').write_bytes(version)
        write_json(self.output / 'distribution-version-result.json', {'argv': ['/tools/registry', '--version'], 'exitCode': 0,
                   'stdoutSHA256': hashlib.sha256(version).hexdigest(), 'binarySHA256': next(item['sha256'] for item in self.distribution['files'] if item['file'] == 'registry')})
        # Only this Engine API phase requests the fresh missing manifest. Host
        # checks above read status/ping only, and no tag/push has happened.
        self.capture_logs('before-daemon-probe')
        before_logs = b''.join((self.output / ('before-daemon-probe.' + key + '.log')).read_bytes() for key in ('out', 'err'))
        manifest_path = ('/v2/' + self.owner['repository'] + '/manifests/payload').encode('ascii')
        if manifest_path in before_logs:
            raise ValueError('fresh daemon manifest route was already requested')
        raw = self.operation([str(self.daemon_probe), '--endpoint', endpoint, '--engine', self.engine,
                              '--session', self.session, '--port', str(self.owner['port'])])
        daemon = archive.json_bytes(raw)
        self.capture_logs('daemon-probe')
        logs = b''.join((self.output / ('daemon-probe.' + key + '.log')).read_bytes() for key in ('out', 'err'))
        witness = validate_daemon_witness(daemon, self.owner, self.engine, logs)
        if daemon.get('sourceCommit') != revision:
            raise ValueError('daemon probe source identity changed')
        write_json(self.output / 'daemon-distribution-probe.json', {'result': daemon, 'witness': witness})
        write_json(self.output / 'registry-placement.json', {'engineID': self.engine, 'containerID': self.container_id, 'owner': self.owner,
                   'hostLoopback': True, 'daemonLoopback': True, 'status': status, 'pushStarted': False})
        return status

    def retire(self):
        self.fence()
        row = json.loads(producer.command(['docker', 'inspect', self.container_id]))[0]
        validate_helper(row, self.owner, self.image_id, self.tools, self.data)
        if row['State']['Running']:
            self.operation(['docker', 'stop', '--time', '10', self.container_id])
        row = json.loads(producer.command(['docker', 'inspect', self.container_id]))[0]
        if row['Id'] != self.container_id or row['State']['Running'] or row['State'].get('OOMKilled') or row['State'].get('ExitCode') != 0 or row['State'].get('Status') != 'exited':
            raise ValueError('helper did not positively exit; retain unknown generation')
        terminal = validate_terminal(read_json(self.data / 'luci-serial-gate-terminal-v1.json'), self.owner)
        self.capture_logs()
        self.operation(['docker', 'rm', self.container_id])
        producer.assert_settled(self.session)
        network = json.loads(producer.command(['docker', 'network', 'inspect', self.network_id]))[0]
        internal = read_json(self.output / 'registry-helper-intent.json').get('networkInternal', True)
        validate_network(network, self.owner, internal=internal)
        self.operation(['docker', 'network', 'rm', self.network_id])
        receipt = {'version': 1, 'engineID': self.engine, 'containerID': self.container_id, 'networkID': self.network_id,
                   'containerExitCode': row['State']['ExitCode'], 'gateTerminal': terminal, 'exactOwnedContainerAbsent': True,
                   'helperRetired': True, 'engineRetired': False, 'imagePreserved': self.image_id}
        write_json(self.output / 'registry-helper-retirement.json', receipt)
        return receipt

    def capture_logs(self, prefix='registry-helper'):
        result = producer.subprocess.run(['docker', 'logs', self.container_id], capture_output=True, timeout=10, check=False)
        if result.returncode or len(result.stdout) + len(result.stderr) > 12 << 20:
            raise ValueError('helper log exceeds bound')
        (self.output / (prefix + '.out.log')).write_bytes(result.stdout)
        (self.output / (prefix + '.err.log')).write_bytes(result.stderr)


def placement(repo, revision, build_repo, original, distribution, output, daemon_probe=None):
    """Minimal real helper/readiness/retirement; deliberately no tag or push."""
    from finalize import completed_proof
    repo, build_repo, original, output = map(lambda value: Path(value).absolute(), (repo, build_repo, original, output))
    producer.clean_source(repo, revision)
    pending, terminal, _, image_id = completed_proof(original)
    producer.clean_source(build_repo, pending['sourceCommit'])
    plain_directory(output.parent)
    if output.exists() or any(output.is_relative_to(value) for value in (repo, build_repo, original)):
        raise ValueError('fresh private placement output outside source/evidence required')
    engine = producer.command(['docker', 'info', '--format', '{{.ID}}']).strip().decode('ascii')
    inspected = json.loads(producer.command(['docker', 'image', 'inspect', image_id]))[0]
    if engine != terminal['engineID'] or inspected.get('Id') != image_id or inspected.get('Config', {}).get('Labels', {}).get(producer.SESSION_LABEL) != pending['session']:
        raise ValueError('placement source/image/engine proof changed')
    session = uuid.uuid4().hex
    output.mkdir(mode=0o700)
    helper = Helper(repo, image_id, session, output, distribution, daemon_probe)
    write_json(output / 'producer-engine.json', {'engineID': engine, 'session': session})
    write_json(output / 'registry-placement-intent.json', {'sourceCommit': revision, 'originalBuildSourceCommit': pending['sourceCommit'],
               'originalSession': pending['session'], 'imageID': image_id, 'session': session, 'pushStarted': False, 'imageRebuilt': False})
    try:
        result = helper.start(placement_only=True)
        retirement = helper.retire()
    except (ValueError, OSError, producer.subprocess.SubprocessError, http.client.HTTPException) as error:
        log_error = None
        if helper.container_id:
            try:
                helper.capture_logs()
            except (ValueError, OSError, producer.subprocess.SubprocessError) as logs:
                log_error = str(logs)
        write_json(output / 'registry-generation-unknown.json', {'session': session, 'containerID': helper.container_id, 'networkID': helper.network_id,
                   'engineID': engine, 'pushStarted': False, 'reason': str(error), 'logCaptureError': log_error, 'absenceOnlySettlement': False})
        raise
    producer.clean_source(repo, revision)
    producer.clean_source(build_repo, pending['sourceCommit'])
    write_json(output / 'registry-placement-complete.json', {'sourceCommit': revision, 'imageID': image_id, 'session': session,
               'hostAndDaemonLoopbackVerified': True, 'helperRetirementVerified': retirement['helperRetired'], 'pushStarted': False,
               'ordinaryInstalledAcceptance': False, 'status': result})
    return output


def validate_failed_prewrite(row, intent, created, placement_intent, unknown, operations):
    owner, image_id = intent['owner'], intent['imageID']
    validate_helper(row, owner, image_id)
    state = row.get('State', {})
    if (row.get('Id') != created.get('containerID') or intent.get('engineID') != created.get('engineID') or
            placement_intent.get('session') != owner['session'] or placement_intent.get('imageID') != image_id or
            placement_intent.get('pushStarted') is not False or unknown.get('pushStarted') is not False or
            unknown.get('containerID') != row['Id'] or unknown.get('session') != owner['session'] or
            state.get('Status') != 'exited' or state.get('Running') is not False or state.get('Pid') != 0 or
            type(state.get('ExitCode')) is not int or state['ExitCode'] == 0 or
            not isinstance(state.get('FinishedAt'), str) or state['FinishedAt'].startswith('0001-') or not state['FinishedAt']):
        raise ValueError('failed prewrite helper lacks exact positive nonzero container terminal')
    for operation in operations:
        args = operation.get('argv')
        if operation.get('session') != owner['session'] or operation.get('engineID') != intent['engineID'] or not isinstance(args, list) or not args or (args[0] == 'docker' and any(value in ('push', 'tag') for value in args[1:3])):
            raise ValueError('failed-start generation has foreign or possible push/tag intent')
    return state


def retire_failed_start(repo, revision, output):
    """Positive exited prewrite container, never missing-process settlement."""
    producer.clean_source(repo, revision)
    output = plain_directory(output)
    intent = read_json(output / 'registry-helper-intent.json')
    created = read_json(output / 'registry-helper-created.json')
    place = read_json(output / 'registry-placement-intent.json')
    unknown = read_json(output / 'registry-generation-unknown.json')
    operations_file = output / 'registry-operations.jsonl'
    if archive.file_record(operations_file)['bytes'] > 1 << 20:
        raise ValueError('failed-start operation evidence exceeds bound')
    operations = [archive.json_bytes(line) for line in operations_file.read_bytes().splitlines()]
    if not operations or (output / 'registry-push-intent.json').exists() or (output / 'registry-placement-complete.json').exists():
        raise ValueError('failed-start generation may have crossed the prewrite boundary')
    helper = Helper.__new__(Helper)
    helper.repo, helper.output = Path(repo), output
    helper.owner, helper.image_id = intent['owner'], intent['imageID']
    if helper.owner != identity(helper.owner.get('session'), helper.owner.get('port')) or not all(isinstance(created.get(key), str) and re.fullmatch('[0-9a-f]{64}', created[key]) for key in ('containerID', 'networkID')):
        raise ValueError('failed-start resource identities are not exact owned IDs')
    helper.session, helper.engine = helper.owner['session'], intent['engineID']
    helper.container_id, helper.network_id = created['containerID'], created['networkID']
    helper.tools, helper.data = output / 'registry-tools', output / 'registry-data'
    producer.ENGINE_IDENTITIES[helper.session] = helper.engine
    helper.fence()
    row = json.loads(producer.command(['docker', 'inspect', helper.container_id]))[0]
    state = validate_failed_prewrite(row, intent, created, place, unknown, operations)
    validate_helper(row, helper.owner, helper.image_id, helper.tools, helper.data)
    helper.capture_logs('failed-start-retirement-helper')
    write_json(output / 'failed-start-retirement-intent.json', {'sourceCommit': revision, 'engineID': helper.engine,
               'containerID': helper.container_id, 'state': state, 'pushStarted': False, 'originalFailurePreserved': True})
    helper.operation(['docker', 'rm', helper.container_id])
    producer.assert_settled(helper.session)
    network = json.loads(producer.command(['docker', 'network', 'inspect', helper.network_id]))[0]
    if network.get('Id') != helper.network_id:
        raise ValueError('failed-start network ID changed; preserve it')
    validate_network(network, helper.owner, internal=intent.get('networkInternal', True))
    helper.operation(['docker', 'network', 'rm', helper.network_id])
    write_json(output / 'failed-start-retirement.json', {'version': 1, 'state': 'failed_before_ready_retired', 'sourceCommit': revision,
               'engineID': helper.engine, 'containerID': helper.container_id, 'networkID': helper.network_id,
               'positiveContainerTerminal': state, 'exactOwnedContainerAbsent': True, 'networkRemoved': True,
               'pushStarted': False, 'gateProtocolVerified': False, 'exportVerified': False, 'engineRetired': False,
               'originalFailurePreserved': True, 'imagePreserved': helper.image_id})
    return output


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['placement', 'retire-failed-start'])
    parser.add_argument('--revision', required=True)
    for key in ('build-repo', 'original', 'distribution-dir', 'daemon-probe'):
        parser.add_argument('--' + key, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        repo = Path(__file__).resolve().parents[2]
        if args.action == 'placement':
            if any(value is None for value in (args.build_repo, args.original, args.distribution_dir, args.daemon_probe)):
                parser.error('placement requires --build-repo, --original, --distribution-dir and --daemon-probe')
            result = placement(repo, args.revision, args.build_repo, args.original, args.distribution_dir, args.output, args.daemon_probe)
        else:
            result = retire_failed_start(repo, args.revision, args.output)
    except (ValueError, OSError, producer.subprocess.SubprocessError, http.client.HTTPException) as error:
        parser.exit(1, 'Placement refused/unknown; retain generation: ' + str(error) + '\n')
    print(json.dumps({'output': str(result), 'pushStarted': False}))
