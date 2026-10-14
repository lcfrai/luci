"""Explicit sequential private VM-source image production stages.

No calls occur on import. Every write has a durable intent; unknown writes
reconcile by authenticated reads or an inspect-only guest command.
"""
import base64
from datetime import datetime, timezone, timedelta
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import uuid

RUNTIME = 'cf536c284434fdd2f09bbab35ec807a5dacc87e4'
READER = '2a2b9908-6ea1-4ae2-8e65-a410df84e7d1'
HEX = re.compile(r'[0-9a-f]{64}\Z')
REV = re.compile(r'[0-9a-f]{40}\Z')


def file_sha(path):
    with Path(path).open('rb') as source: return hashlib.file_digest(source, 'sha256').hexdigest()


def source_payload(core, bundle_root, selection_path, luci_source, runtime_source, envoy_binary, envoy_license):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    core.requested(selected)
    parent = Path(selection_path).parent
    target = parent / 'azure-builder-source.tar'
    if target.exists() or (parent / 'azure-builder-source.json').exists():
        raise ValueError('Builder source operation is retained; preserve its exact archive')
    roots = [(Path(luci_source), manifest['luciRevision'], 'luci'), (Path(runtime_source), RUNTIME, 'runtime')]
    for root, revision, _ in roots:
        if not root.is_absolute() or root.is_symlink() or core.local_command(['git', '-C', str(root), 'rev-parse', 'HEAD']) != revision or core.local_command(['git', '-C', str(root), 'status', '--porcelain']):
            raise ValueError('Builder source checkouts must be exact and clean')
    for path in (Path(envoy_binary), Path(envoy_license)):
        if not path.is_absolute() or path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 128 << 20:
            raise ValueError('Select bounded Envoy binary and original licence files')
    core.atomic_json(parent / 'azure-builder-source.json', {'version': 1, 'state': 'source_archive_pending', 'sourceCommit': manifest['luciRevision']})
    with tempfile.TemporaryDirectory(dir=parent) as directory:
        work = Path(directory)
        files = {}
        for root, revision, name in roots:
            archive = work / (name + '.tar')
            # Stream Git's exact tree directly; preserve tracked modes, links and
            # original commit bytes, but never transfer .git history or secrets
            # from an untracked working directory.
            with archive.open('xb') as sink:
                subprocess.run(['git', '-C', str(root), 'archive', '--format=tar.gz', revision], stdout=sink, stderr=subprocess.DEVNULL, check=True, timeout=120)
            if archive.stat().st_size > 128 << 20: raise ValueError('Source archive exceeds builder bound')
            commit = work / (name + '.commit')
            commit.write_bytes(subprocess.check_output(['git', '-C', str(root), 'cat-file', 'commit', revision], timeout=10))
            files[archive.name], files[commit.name] = archive, commit
        files.update(envoy=Path(envoy_binary), **{'envoy-license': Path(envoy_license)})
        source_binding = {'version': 1, 'sourceCommit': manifest['luciRevision'], 'runtimeRevision': RUNTIME,
            'files': {name: {'sha256': file_sha(path), 'size': path.stat().st_size} for name, path in files.items()}}
        binding = work / 'manifest.json'
        binding.write_bytes(core.canonical(source_binding))
        files['manifest.json'] = binding
        with tarfile.open(target, 'w') as archive:
            for name, path in files.items():
                info = tarfile.TarInfo(name)
                info.size, info.mode = path.stat().st_size, 0o755 if name == 'envoy' else 0o600
                with path.open('rb') as source: archive.addfile(info, source)
        target.chmod(0o600)
    if target.stat().st_size > 256 << 20: raise ValueError('Builder payload exceeds its bound')
    receipt = {'version': 1, 'state': 'source_archive_prepared', 'sourceCommit': manifest['luciRevision'],
        'payloadPath': str(target), 'payloadSHA256': file_sha(target), 'size': target.stat().st_size,
        'inputs': source_binding, 'connected': False}
    core.atomic_json(parent / 'azure-builder-source.json', receipt)
    return receipt


def plan(core, bundle_root, selection_path, base_image, builder_size, gallery_name, image_version, lifetime_seconds):
    manifest = core.bundle(bundle_root)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    source = core.private_json(parent / 'azure-builder-source.json')
    location = core.private_json(parent / 'azure-private-location.json')
    foundation = core.private_json(parent / 'azure-foundation-state.json')
    if (parent / 'azure-builder-plan.json').exists() or source.get('state') != 'source_archive_prepared' or source.get('sourceCommit') != manifest['luciRevision'] or file_sha(source['payloadPath']) != source['payloadSHA256'] or foundation.get('state') != 'foundation_bound_private_data_route_observed':
        raise ValueError('Exact source archive and privately bound foundation are required')
    if (not isinstance(base_image, dict) or set(base_image) != {'publisher', 'offer', 'sku', 'version'} or
            any(not re.fullmatch(r'[A-Za-z0-9._-]{1,90}', v) for v in base_image.values()) or base_image['version'].lower() == 'latest' or
            base_image['publisher'] != 'Canonical' or base_image['offer'] != 'ubuntu-24_04-lts' or base_image['sku'] != 'server' or
            not re.fullmatch(r'Standard_[A-Za-z0-9_]{2,80}', builder_size) or not re.fullmatch(r'[a-zA-Z][a-zA-Z0-9_.]{1,60}', gallery_name) or
            not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', image_version) or not 600 <= lifetime_seconds <= 14400):
        raise ValueError('Select an exact Canonical Ubuntu image, available VM size, gallery version and bounded lifetime')
    key = variables.get('controller_admin_ssh_public_key', '')
    if not re.fullmatch(r'(ssh-ed25519|ssh-rsa) [A-Za-z0-9+/=]{32,4096}(?: [^\r\n]{0,128})?', key):
        raise ValueError('An existing public SSH key is required; no SSH inbound rule is created')
    group = '/subscriptions/' + selected['stateBackend']['subscriptionId'] + '/resourceGroups/' + variables['resource_group_name']
    storage = '/subscriptions/' + selected['stateBackend']['subscriptionId'] + '/resourceGroups/' + selected['stateBackend']['resourceGroupName'] + '/providers/Microsoft.Storage/storageAccounts/' + selected['stateBackend']['storageAccountName']
    operation = uuid.uuid4().hex
    container_name = 'luci-build-' + operation[:16]
    container = storage + '/blobServices/default/containers/' + container_name
    tag = {'luci-source': manifest['luciRevision'], 'luci-owner': variables['controller_owner_id'], 'luci-scope': variables['scope'], 'luci-operation': operation, 'luci-purpose': 'temporary-private-image-builder'}
    roles = {}
    for role in ('runtime', 'controller'):
        name = selected['name'][:24] + '-build-' + role + '-' + operation[:8]
        vm = group + '/providers/Microsoft.Compute/virtualMachines/' + name
        nic = group + '/providers/Microsoft.Network/networkInterfaces/' + name
        disk = group + '/providers/Microsoft.Compute/disks/' + name + '-os'
        roles[role] = {'vmID': vm, 'nicID': nic, 'diskID': disk, 'tags': dict(tag, **{'luci-role': role}),
            'grantID': container + '/providers/Microsoft.Authorization/roleAssignments/' + str(uuid.uuid5(uuid.NAMESPACE_URL, vm.lower() + container.lower())),
            'imageDefinitionID': group + '/providers/Microsoft.Compute/galleries/' + gallery_name + '/images/' + role,
            'imageVersionID': group + '/providers/Microsoft.Compute/galleries/' + gallery_name + '/images/' + role + '/versions/' + image_version}
    result = {'version': 1, 'state': 'private_builder_review_required', 'sourceCommit': manifest['luciRevision'],
        'scope': variables['scope'], 'generation': variables['generation'], 'selectionDigest': core.digest(selected),
        'operationID': operation, 'baseImage': base_image, 'builderSize': builder_size, 'region': selected['region'], 'roles': roles,
        'payloadPath': source['payloadPath'], 'payloadSHA256': source['payloadSHA256'], 'payloadSize': source['size'],
        'containerID': container, 'containerName': container_name, 'storageAccountName': selected['stateBackend']['storageAccountName'],
        'payloadURL': 'https://' + selected['stateBackend']['storageAccountName'] + '.blob.core.windows.net/' + container_name + '/source.tar',
        'endpointSubnetID': location['endpointSubnetID'], 'galleryID': group + '/providers/Microsoft.Compute/galleries/' + gallery_name,
        'tags': tag, 'adminPublicKey': key, 'maxActiveBuilders': 1, 'lifetimeSeconds': lifetime_seconds,
        'monthlyBudgetUsd': selected['monthlyBudgetUsd'], 'priceVerified': False, 'finalInvoiceVerified': False,
        'retainedCharges': ['temporary VM compute until observed deallocated/deleted', 'temporary managed OS disk and NIC until exact cleanup', 'staging bytes/container and temporary reader grant until exact retirement', 'published gallery image/version replicas remain retained'],
        'requiredActions': ['VM read/write/runCommand/deallocate/generalize/delete', 'disk and NIC read/write/delete', 'selected existing subnet join', 'gallery/image/version read/write', 'owned private container/blob read/write/delete', 'exact container-scoped temporary reader role assignment read/write/delete'],
        'message': 'Review account quote and egress/permissions. Lifetime is an admission deadline, not an automatic stop or billing cap; pending effects can retain charges.'}
    result['planDigest'] = core.digest(result)
    core.atomic_json(parent / 'azure-builder-plan.json', result)
    return result


def load(core, bundle_root, selection_path, plan_digest):
    manifest = core.bundle(bundle_root)
    parent = Path(selection_path).parent
    proposal = core.private_json(parent / 'azure-builder-plan.json')
    current_digest = core.digest(core.private_json(selection_path))
    path = parent / 'azure-builder-state.json'
    retained = core.private_json(path) if path.exists() else {}
    bound = retained.get('selectionDigest') == current_digest and all(role + ':retired' in retained.get('completed', []) for role in ('runtime', 'controller'))
    if proposal['sourceCommit'] != manifest['luciRevision'] or proposal['planDigest'] != plan_digest or core.digest({k: v for k, v in proposal.items() if k != 'planDigest'}) != plan_digest or proposal['selectionDigest'] != current_digest and not bound:
        raise ValueError('Builder source/resource plan changed')
    journal = core.private_json(path) if path.exists() else {'version': 1, 'state': 'not_started', 'planDigest': plan_digest, 'completed': [], 'roles': {}}
    if 'admissionDeadlineUTC' not in journal:
        journal['admissionDeadlineUTC'] = (datetime.now(timezone.utc) + timedelta(seconds=proposal['lifetimeSeconds'])).isoformat().replace('+00:00', 'Z')
    if journal['planDigest'] != plan_digest: raise ValueError('Retained builder operation differs')
    return parent, proposal, path, journal


def write_stage(core, path, journal, key, operation, observe, acknowledge, reconcile):
    if key in journal['completed']: return journal
    if journal.get('abortRequested') and not journal.get('pending') and ':abort-' not in key and not key.startswith('source:delete-'):
        raise ValueError('Builder abort is retained; no new production effect is allowed')
    if journal.get('pending'):
        if journal['pending'] != key or not reconcile: raise ValueError('Builder effect uncertain; use exact read-only reconciliation')
    else:
        if reconcile or not acknowledge: raise ValueError('Exact resource and recurring-cost acknowledgment is required')
        if not any(marker in key for marker in (':delete-', ':retired', ':deallocate', ':abort-')) and datetime.now(timezone.utc) >= datetime.fromisoformat(journal['admissionDeadlineUTC'].replace('Z', '+00:00')):
            raise ValueError('Builder admission deadline elapsed; reconcile/retire retained operations and review a fresh plan')
        journal.update(state='builder_effect_pending', pending=key)
        core.atomic_json(path, journal)
        operation()
    receipt = observe()
    if receipt is None: raise RuntimeError('Builder effect remains uncertain; do not repeat mutation')
    journal['completed'].append(key)
    journal.pop('pending', None)
    journal.update(state='builder_stage_observed', observedAtUTC=datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z'))
    journal.setdefault('receipts', {})[key] = receipt
    core.atomic_json(path, journal)
    return journal


def arm_create(core, path, journal, key, identifier, api, body, arm, acknowledge, reconcile):
    def operation():
        if arm('GET', identifier, api) is not None:
            raise ValueError('Builder resource already exists; no adoption or overwrite')
        arm('PUT', identifier, api, body)
    def observe():
        row = arm('GET', identifier, api)
        if not row or row.get('id', '').lower() != identifier.lower() or row.get('properties', {}).get('provisioningState', 'Succeeded') != 'Succeeded': return None
        spec = importlib.util.spec_from_file_location('luci_foundation_match', Path(__file__).with_name('foundation.py'))
        module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
        if not module.matches(row, body): return None
        return {'id': identifier, 'readbackDigest': core.digest(row)}
    # Absence must precede the durable intent; otherwise a known existing
    # resource could incorrectly become a pending operation eligible for adopt.
    if not reconcile and key not in journal['completed'] and not journal.get('pending') and arm('GET', identifier, api) is not None:
        raise ValueError('Builder resource already exists; no adoption permitted')
    return write_stage(core, path, journal, key, operation, observe, acknowledge, reconcile)


def allocate(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    parent, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in ('runtime', 'controller'): raise ValueError('Select one exact image role')
    if role == 'controller' and 'runtime:retired' not in journal['completed']:
        raise ValueError('Retire the first role builder before allocating the second; max one active')
    entry = proposal['roles'][role]
    base = proposal['baseImage']
    image_id = '/subscriptions/' + entry['vmID'].split('/')[2] + '/providers/Microsoft.Compute/locations/' + proposal['region'] + '/publishers/' + base['publisher'] + '/artifacttypes/vmimage/offers/' + base['offer'] + '/skus/' + base['sku'] + '/versions/' + base['version']
    image = arm('GET', image_id, '2023-09-01')
    properties = (image or {}).get('properties', {})
    if properties.get('osDiskImage', {}).get('operatingSystem') != 'Linux' or properties.get('hyperVGeneration') != 'V2' or properties.get('architecture') != 'x64':
        raise ValueError('Exact available x64 Gen2 Canonical Ubuntu base metadata is required')
    nic = {'location': proposal['region'], 'tags': entry['tags'], 'properties': {'ipConfigurations': [{'name': 'private', 'properties': {'subnet': {'id': proposal['endpointSubnetID']}, 'privateIPAllocationMethod': 'Dynamic'}}]}}
    key = role + ':nic'
    if key not in journal['completed']:
        return arm_create(core, path, journal, key, entry['nicID'], '2023-09-01', nic, arm, acknowledge, reconcile)
    key = role + ':vm'
    body = {'location': proposal['region'], 'tags': entry['tags'], 'identity': {'type': 'SystemAssigned'}, 'properties': {
        'hardwareProfile': {'vmSize': proposal['builderSize']}, 'storageProfile': {'imageReference': proposal['baseImage'],
            'osDisk': {'name': entry['diskID'].rsplit('/', 1)[-1], 'createOption': 'FromImage', 'diskSizeGB': 128,
                'managedDisk': {'storageAccountType': 'StandardSSD_LRS'}, 'deleteOption': 'Detach'}},
        'osProfile': {'computerName': entry['vmID'].rsplit('/', 1)[-1], 'adminUsername': 'luci-builder', 'linuxConfiguration': {
            'disablePasswordAuthentication': True, 'provisionVMAgent': True, 'ssh': {'publicKeys': [{'path': '/home/luci-builder/.ssh/authorized_keys', 'keyData': proposal['adminPublicKey']}]}}},
        'networkProfile': {'networkInterfaces': [{'id': entry['nicID'], 'properties': {'deleteOption': 'Detach'}}]}}}
    if key not in journal['completed']:
        return arm_create(core, path, journal, key, entry['vmID'], '2023-09-01', body, arm, acknowledge, reconcile)
    owned_vm(proposal, role, arm)
    def operation():
        disk = arm('GET', entry['diskID'], '2023-10-02')
        if not disk or disk.get('managedBy', '').lower() != entry['vmID'].lower() or not disk.get('etag'):
            raise ValueError('Exact VM-owned OS disk and ETag required before tagging')
        arm('PATCH', entry['diskID'], '2023-10-02', {'tags': entry['tags'], 'properties': {'networkAccessPolicy': 'DenyAll', 'publicNetworkAccess': 'Disabled'}}, {'If-Match': disk['etag']})
    def observe():
        disk = arm('GET', entry['diskID'], '2023-10-02')
        if disk and disk.get('tags') == entry['tags'] and disk.get('managedBy', '').lower() == entry['vmID'].lower() and disk.get('properties', {}).get('networkAccessPolicy') == 'DenyAll' and disk.get('properties', {}).get('publicNetworkAccess') == 'Disabled':
            return {'diskID': entry['diskID'], 'vmID': entry['vmID'], 'readbackDigest': core.digest(disk)}
        return None
    return write_stage(core, path, journal, role + ':disk-tag', operation, observe, acknowledge, reconcile)


def owned_vm(proposal, role, arm):
    entry = proposal['roles'][role]
    row = arm('GET', entry['vmID'], '2023-09-01')
    if not row or row.get('id', '').lower() != entry['vmID'].lower() or row.get('tags') != entry['tags'] or row.get('properties', {}).get('storageProfile', {}).get('osDisk', {}).get('managedDisk', {}).get('id', '').lower() != entry['diskID'].lower():
        raise ValueError('Exact owned builder VM/OS disk readback differs')
    nic = arm('GET', entry['nicID'], '2023-09-01')
    addresses = (nic or {}).get('properties', {}).get('ipConfigurations', [])
    if not nic or nic.get('tags') != entry['tags'] or len(addresses) != 1 or any(value.get('properties', {}).get('publicIPAddress') or value.get('properties', {}).get('subnet', {}).get('id', '').lower() != proposal['endpointSubnetID'].lower() for value in addresses):
        raise ValueError('Builder NIC must retain its exact existing private subnet with no public address')
    return row


def staging(core, bundle_root, selection_path, plan_digest, arm, cli, acknowledge=False, reconcile=False):
    parent, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    storage = arm('GET', proposal['containerID'].split('/blobServices/', 1)[0], '2023-05-01')
    if not storage or storage.get('properties', {}).get('publicNetworkAccess') != 'Disabled' or storage.get('properties', {}).get('allowSharedKeyAccess') is not False:
        raise ValueError('Builder staging must retain private AAD-only storage')
    if 'source:container' not in journal['completed']:
        return arm_create(core, path, journal, 'source:container', proposal['containerID'], '2023-05-01',
            {'properties': {'publicAccess': 'None', 'metadata': {'luci_operation': proposal['operationID'], 'luci_source': proposal['sourceCommit']}}}, arm, acknowledge, reconcile)
    if file_sha(proposal['payloadPath']) != proposal['payloadSHA256']: raise ValueError('Source archive changed before upload')
    arguments = ['--account-name', proposal['storageAccountName'], '--container-name', proposal['containerName'], '--name', 'source.tar', '--auth-mode', 'login']
    def operation():
        present = cli(['storage', 'blob', 'exists', *arguments])
        if present.get('exists') is not False: raise ValueError('Source blob already exists or absence unknown')
        cli(['storage', 'blob', 'upload', *arguments, '--file', proposal['payloadPath'], '--overwrite', 'false', '--if-none-match', '*', '--validate-content', '--max-connections', '1',
            '--metadata', 'luci_operation=' + proposal['operationID'], 'luci_source=' + proposal['sourceCommit'], 'sha256=' + proposal['payloadSHA256']])
    def observe():
        row = cli(['storage', 'blob', 'show', *arguments])
        if row.get('metadata', {}).get('luci_operation') != proposal['operationID'] or row.get('metadata', {}).get('sha256') != proposal['payloadSHA256'] or row.get('properties', {}).get('contentLength') != proposal['payloadSize']:
            return None
        # Metadata/ETag alone is not artifact proof. Download exact immutable
        # ETag through private AAD access and compare actual complete bytes.
        with tempfile.TemporaryDirectory(dir=parent) as directory:
            downloaded = Path(directory) / 'source.tar'
            cli(['storage', 'blob', 'download', *arguments, '--file', str(downloaded), '--if-match', row['properties']['etag'], '--validate-content', '--max-connections', '1'])
            if not downloaded.is_file() or file_sha(downloaded) != proposal['payloadSHA256']: return None
        return {'payloadSHA256': proposal['payloadSHA256'], 'etag': row['properties']['etag'], 'readbackVerified': True}
    if not journal.get('pending') and 'source:upload' not in journal['completed'] and not reconcile and cli(['storage', 'blob', 'exists', *arguments]).get('exists') is not False:
        raise ValueError('Source blob already exists; no overwrite/adoption')
    return write_stage(core, path, journal, 'source:upload', operation, observe, acknowledge, reconcile)


def grant(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':disk-tag' not in journal['completed'] or 'source:upload' not in journal['completed']:
        raise ValueError('Exact builder allocation and byte-verified private source are required')
    entry = proposal['roles'][role]
    vm = owned_vm(proposal, role, arm)
    principal = vm.get('identity', {}).get('principalId', '')
    if not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', principal): raise ValueError('Builder system identity readback unavailable')
    role_id = '/subscriptions/' + entry['vmID'].split('/')[2] + '/providers/Microsoft.Authorization/roleDefinitions/' + READER
    body = {'properties': {'principalId': principal, 'principalType': 'ServicePrincipal', 'roleDefinitionId': role_id}}
    binding = journal.setdefault('roles', {}).setdefault(role, {})
    if binding.get('principalID') not in (None, principal): raise ValueError('Retained temporary reader principal changed; preserve the original grant')
    binding['principalID'] = principal
    return arm_create(core, path, journal, role + ':grant', entry['grantID'], '2022-04-01', body, arm, acknowledge, reconcile)


def guest_script(core, bundle_root, proposal, role, mode):
    if role not in ('runtime', 'controller') or mode not in ('prepare', 'inspect', 'deprovision'): raise ValueError('Exact guest source/role action required')
    source = (Path(bundle_root) / 'azure-setup/builder_guest.py').read_bytes()
    config = {'version': 1, 'sourceCommit': proposal['sourceCommit'], 'role': role, 'payloadURL': proposal['payloadURL'], 'payloadSHA256': proposal['payloadSHA256']}
    encoded, selected = base64.b64encode(source).decode(), base64.b64encode(core.canonical(config)).decode()
    return """set -eu
python3 - <<'PY'
import base64, os, pathlib, tempfile
files = [('/opt/luci-image-builder.py',base64.b64decode(%s)),('/etc/luci-image-builder.json',base64.b64decode(%s))]
for name, data in files:
    target = pathlib.Path(name)
    if target.exists() or target.is_symlink():
        if target.is_symlink() or target.read_bytes() != data or target.stat().st_mode & 0o077:
            raise SystemExit('retained builder source/config changed')
    elif %s:
        raise SystemExit('builder installation remains uncertain')
    else:
        fd, temporary = tempfile.mkstemp(dir=target.parent); os.fchmod(fd,0o600)
        with os.fdopen(fd,'wb') as output: output.write(data); output.flush(); os.fsync(output.fileno())
        os.replace(temporary,target)
PY
python3 /opt/luci-image-builder.py %s /etc/luci-image-builder.json
""" % (repr(encoded), repr(selected), repr(mode == 'inspect'), mode)


def install(core, bundle_root, selection_path, plan_digest, role, arm, cli, acknowledge=False, reconcile=False, deprovision=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':grant' not in journal['completed']: raise ValueError('Exact scoped reader grant is required')
    if deprovision and role + ':install' not in journal['completed']: raise ValueError('Observed role preparation must precede deprovision')
    owned_vm(proposal, role, arm)
    key = role + (':deprovision' if deprovision else ':install')
    def invoke(mode):
        return cli(['vm', 'run-command', 'invoke', '--ids', proposal['roles'][role]['vmID'], '--command-id', 'RunShellScript', '--scripts', guest_script(core, bundle_root, proposal, role, mode)])
    result = {}
    def operation(): result['value'] = invoke('deprovision' if deprovision else 'prepare')
    def observe():
        response = result.get('value') or invoke('inspect')
        markers = [line.removeprefix('LUCI_AZURE_BUILDER_RECEIPT=') for row in response.get('value', []) for line in row.get('message', '').splitlines() if line.startswith('LUCI_AZURE_BUILDER_RECEIPT=')]
        if len(markers) != 1: return None
        value = json.loads(markers[0])
        if any(value.get(k) != v for k, v in (('sourceCommit', proposal['sourceCommit']), ('role', role), ('payloadSHA256', proposal['payloadSHA256']), ('runtimeRevision', RUNTIME))) or not HEX.fullmatch(value.get('packageSHA256', '')) or value.get('state') != ('role_deprovisioned_capture_pending' if deprovision else 'role_prepared_guest_unverified'):
            return None
        return value
    return write_stage(core, path, journal, key, operation, observe, acknowledge, reconcile)


def deallocate(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':deprovision' not in journal['completed']: raise ValueError('Observed deprovision must precede capture deallocation')
    identifier = proposal['roles'][role]['vmID']
    owned_vm(proposal, role, arm)
    def observe():
        view = arm('GET', identifier + '/instanceView', '2023-09-01')
        if view and any(row.get('code') == 'PowerState/deallocated' for row in view.get('statuses', [])):
            return {'vmID': identifier, 'powerState': 'deallocated'}
        return None
    return write_stage(core, path, journal, role + ':deallocate', lambda: arm('POST', identifier + '/deallocate', '2023-09-01'), observe, acknowledge, reconcile)


def generalize(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':deallocate' not in journal['completed']: raise ValueError('Exact observed deallocation must precede generalization')
    identifier = proposal['roles'][role]['vmID']
    owned_vm(proposal, role, arm)
    def observe():
        view = arm('GET', identifier + '/instanceView', '2023-09-01')
        if view and any(row.get('code', '').lower() == 'osstate/generalized' for row in view.get('statuses', [])):
            return {'vmID': identifier, 'osState': 'generalized'}
        return None
    return write_stage(core, path, journal, role + ':generalize', lambda: arm('POST', identifier + '/generalize', '2023-09-01'), observe, acknowledge, reconcile)


def capture(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':generalize' not in journal['completed']: raise ValueError('Exact generalized role VM is required')
    entry = proposal['roles'][role]
    owned_vm(proposal, role, arm)
    if 'gallery:create' not in journal['completed']:
        return arm_create(core, path, journal, 'gallery:create', proposal['galleryID'], '2023-07-03',
            {'location': proposal['region'], 'tags': proposal['tags'], 'properties': {'description': 'Source-bound Luci private runtime/controller images'}}, arm, acknowledge, reconcile)
    gallery = arm('GET', proposal['galleryID'], '2023-07-03')
    if not gallery or gallery.get('tags') != proposal['tags']: raise ValueError('Owned gallery source changed')
    if role + ':definition' not in journal['completed']:
        return arm_create(core, path, journal, role + ':definition', entry['imageDefinitionID'], '2023-07-03',
            {'location': proposal['region'], 'tags': entry['tags'], 'properties': {'osType': 'Linux', 'osState': 'Generalized', 'hyperVGeneration': 'V2', 'architecture': 'x64',
                'identifier': {'publisher': 'lcfrai', 'offer': 'luci', 'sku': role}}}, arm, acknowledge, reconcile)
    prepared = journal['receipts'][role + ':install']
    body = {'location': proposal['region'], 'tags': dict(entry['tags'], **{'luci-package-sha256': prepared['packageSHA256']}),
        'properties': {'storageProfile': {'source': {'virtualMachineId': entry['vmID']}}, 'publishingProfile': {'replicaCount': 1, 'storageAccountType': 'Standard_LRS',
            'targetRegions': [{'name': proposal['region'], 'regionalReplicaCount': 1, 'storageAccountType': 'Standard_LRS'}]}}}
    return arm_create(core, path, journal, role + ':capture', entry['imageVersionID'], '2023-07-03', body, arm, acknowledge, reconcile)


def retire(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles'] or role + ':capture' not in journal['completed']: raise ValueError('Observed exact published image must precede source-builder retirement')
    entry = proposal['roles'][role]
    for name, identifier, api in (('grant', entry['grantID'], '2022-04-01'), ('vm', entry['vmID'], '2023-09-01'), ('disk', entry['diskID'], '2023-10-02'), ('nic', entry['nicID'], '2023-09-01')):
        key = role + ':delete-' + name
        if key in journal['completed']: continue
        def operation():
            row = arm('GET', identifier, api)
            if not row or row.get('id', '').lower() != identifier.lower(): raise ValueError('Owned builder retirement resource unavailable')
            if name == 'grant':
                properties = row.get('properties', {})
                expected_role = '/subscriptions/' + entry['vmID'].split('/')[2] + '/providers/Microsoft.Authorization/roleDefinitions/' + READER
                if properties.get('principalId') != journal['roles'][role]['principalID'] or properties.get('roleDefinitionId', '').lower() != expected_role.lower() or properties.get('scope', proposal['containerID']).lower() != proposal['containerID'].lower() or identifier.rsplit('/providers/', 1)[0].lower() != proposal['containerID'].lower(): raise ValueError('Temporary reader grant principal/role/scope differs')
            elif name == 'disk':
                if role + ':delete-vm' not in journal['completed'] or row.get('managedBy') or row.get('properties', {}).get('diskState') != 'Unattached' or row.get('tags') != entry['tags']: raise ValueError('Builder disk remains attached, foreign or source VM retirement uncertain')
            elif row.get('tags') != entry['tags']: raise ValueError('Builder resource ownership changed')
            if name == 'nic' and row.get('properties', {}).get('virtualMachine'): raise ValueError('Builder NIC remains attached')
            arm('DELETE', identifier, api)
        def observe():
            return {'id': identifier, 'absent': True} if arm('GET', identifier, api) is None else None
        return write_stage(core, path, journal, key, operation, observe, acknowledge, reconcile)
    if role + ':retired' not in journal['completed']:
        journal['completed'].append(role + ':retired')
        journal['state'] = 'role_builder_retired_image_retained'
        core.atomic_json(path, journal)
    return journal



def abort(core, bundle_root, selection_path, plan_digest, role, arm, acknowledge=False, reconcile=False):
    """Stop an incomplete producer using only exact, positively owned resources.

    A retained unknown create/install/capture must be reconciled first; abort
    never overwrites that intent or guesses that an in-flight operation ended.
    Each invocation performs at most one provider mutation.
    """
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if role not in proposal['roles']: raise ValueError('Exact builder role required')
    if not journal.get('abortRequested'):
        if reconcile or not acknowledge: raise ValueError('Exact builder abort and retained-charge acknowledgment is required')
        journal.update(abortRequested=True, abortRequestedAtUTC=datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z'))
        core.atomic_json(path, journal)
    pending = journal.get('pending', '')
    if pending and ':abort-' not in pending:
        # Do not replace a possible live RunCommand/capture/create with deletion.
        return dict(journal, state='builder_abort_blocked_uncertain_effect', nextAction='reconcile_exact_retained_effect',
                    blockedEffect=pending, connected=False, finalInvoiceVerified=False)
    entry = proposal['roles'][role]
    completed = journal['completed']
    if role + ':aborted' in completed: return journal
    if role + ':vm' in completed and role + ':abort-disk-binding' not in completed:
        owned_vm(proposal, role, arm)
        disk = arm('GET', entry['diskID'], '2023-10-02')
        unique = (disk or {}).get('properties', {}).get('uniqueId', '')
        if (not disk or disk.get('id', '').lower() != entry['diskID'].lower() or
                disk.get('managedBy', '').lower() != entry['vmID'].lower() or
                not re.fullmatch(r'[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}', unique) or
                disk.get('tags', {}) not in ({}, entry['tags'])):
            raise ValueError('Exact VM-owned immutable OS disk identity is required before abort')
        # The disk may precede its tag PATCH. Retain its immutable provider
        # uniqueId while the positively owned VM still proves attachment.
        journal.setdefault('receipts', {})[role + ':abort-disk-binding'] = {
            'diskID': entry['diskID'], 'uniqueID': unique, 'tags': disk.get('tags', {}), 'readbackDigest': core.digest(disk)}
        completed.append(role + ':abort-disk-binding')
        core.atomic_json(path, journal)
    if role + ':vm' in completed and role + ':abort-deallocate' not in completed:
        owned_vm(proposal, role, arm)
        def observe_power():
            view = arm('GET', entry['vmID'] + '/instanceView', '2023-09-01')
            if view and any(row.get('code') == 'PowerState/deallocated' for row in view.get('statuses', [])):
                return {'vmID': entry['vmID'], 'powerState': 'deallocated'}
            return None
        return write_stage(core, path, journal, role + ':abort-deallocate',
            lambda: arm('POST', entry['vmID'] + '/deallocate', '2023-09-01'), observe_power, acknowledge, reconcile)
    for name, identifier, api, created in (
            ('grant', entry['grantID'], '2022-04-01', role + ':grant'),
            ('vm', entry['vmID'], '2023-09-01', role + ':vm'),
            ('disk', entry['diskID'], '2023-10-02', role + ':vm'),
            ('nic', entry['nicID'], '2023-09-01', role + ':nic')):
        key = role + ':abort-delete-' + name
        if key in completed: continue
        if created not in completed:
            if arm('GET', identifier, api) is not None:
                raise ValueError('Resource has no completed owned creation; preserve it and reconcile the original intent')
            journal.setdefault('receipts', {})[key] = {'id': identifier, 'absent': True, 'neverAllocated': True}
            completed.append(key)
            core.atomic_json(path, journal)
            continue
        def operation():
            row = arm('GET', identifier, api)
            if not row or row.get('id', '').lower() != identifier.lower():
                raise ValueError('Owned abort resource unavailable; absence without our delete intent is not retirement proof')
            if name == 'grant':
                properties = row.get('properties', {})
                expected = '/subscriptions/' + entry['vmID'].split('/')[2] + '/providers/Microsoft.Authorization/roleDefinitions/' + READER
                if (properties.get('principalId') != journal.get('roles', {}).get(role, {}).get('principalID') or
                        properties.get('roleDefinitionId', '').lower() != expected.lower() or
                        properties.get('scope', proposal['containerID']).lower() != proposal['containerID'].lower()):
                    raise ValueError('Abort reader principal/role/scope changed')
            elif name == 'disk':
                binding = journal['receipts'][role + ':abort-disk-binding']
                if (role + ':abort-delete-vm' not in completed or row.get('managedBy') or
                        row.get('properties', {}).get('diskState') != 'Unattached' or
                        row.get('properties', {}).get('uniqueId') != binding['uniqueID'] or row.get('tags', {}) != binding['tags']):
                    raise ValueError('Abort OS disk is attached, replaced or ownership changed')
            elif row.get('tags') != entry['tags']:
                raise ValueError('Abort resource ownership changed')
            if name == 'nic' and row.get('properties', {}).get('virtualMachine'):
                raise ValueError('Abort NIC remains attached')
            arm('DELETE', identifier, api)
        def observe_absence():
            return {'id': identifier, 'absent': True} if arm('GET', identifier, api) is None else None
        return write_stage(core, path, journal, key, operation, observe_absence, acknowledge, reconcile)
    completed.append(role + ':aborted')
    journal.update(state='builder_role_aborted_retained_artifacts', connected=False, finalInvoiceVerified=False,
        retainedCharges=['owned staging bytes/container until observed cleanup',
                         'any previously published gallery image/version replicas remain retained',
                         'other foundation/runtime resources are outside this builder abort'])
    core.atomic_json(path, journal)
    return journal


def bind_images(core, bundle_root, selection_path, plan_digest):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if any(role + ':retired' not in journal['completed'] for role in ('runtime', 'controller')): raise ValueError('Both exact image publications and temporary builder retirements are required')
    if journal.get('abortRequested'): raise ValueError('Aborted production cannot bind images')
    selected = core.private_json(selection_path)
    if any((Path(selection_path).parent / name).exists() for name in ('azure-setup-state.json', 'azure-terraform')): raise ValueError('Infrastructure preparation already retained')
    for role in ('runtime', 'controller'):
        selected['terraformVariables'][role + '_image_version_id'] = proposal['roles'][role]['imageVersionID']
        if role == 'runtime': selected['terraformVariables']['runtime_package_sha256'] = journal['receipts'][role + ':install']['packageSHA256']
        selected[role + 'Package'] = {'path': '', 'sha256': journal['receipts'][role + ':install']['packageSHA256']}
    guest_source = journal['receipts']['runtime:install'].get('guestSource')
    if guest_source:
        if guest_source.get('sourceCommit') != proposal['sourceCommit'] or not HEX.fullmatch(guest_source.get('imageDigest', '')) or guest_source.get('fromImage') != '127.0.0.1:5055/luci/guest@sha256:' + guest_source['imageDigest'] or not HEX.fullmatch(guest_source.get('receiptSHA256', '')):
            raise ValueError('Measured guest source receipt differs from exact producer')
        core.atomic_json(Path(selection_path).parent / 'azure-guest-source.json', guest_source)
    core.atomic_json(selection_path, selected)
    journal.update(state='private_images_bound_staging_retired' if 'source:delete-container' in journal['completed'] else 'private_images_bound_staging_retirement_pending', selectionDigest=core.digest(selected), connected=False, finalInvoiceVerified=False)
    core.atomic_json(path, journal)
    return journal


def retire_staging(core, bundle_root, selection_path, plan_digest, arm, cli, acknowledge=False, reconcile=False):
    _, proposal, path, journal = load(core, bundle_root, selection_path, plan_digest)
    if any(not any(role + suffix in journal['completed'] for suffix in (':retired', ':aborted')) for role in ('runtime', 'controller')):
        raise ValueError('Both role builders and temporary grants must be retired or aborted first')
    arguments = ['--account-name', proposal['storageAccountName'], '--container-name', proposal['containerName'], '--name', 'source.tar', '--auth-mode', 'login']
    if journal.get('abortRequested'):
        if 'source:container' not in journal['completed']:
            if journal.get('pending') or arm('GET', proposal['containerID'], '2023-05-01') is not None:
                raise ValueError('Uncertain or unowned staging container must be preserved')
            for key in ('source:delete-blob', 'source:delete-container'):
                if key not in journal['completed']:
                    journal['completed'].append(key)
                    journal.setdefault('receipts', {})[key] = {'absent': True, 'neverAllocated': True}
            core.atomic_json(path, journal)
        elif 'source:upload' not in journal['completed'] and 'source:delete-blob' not in journal['completed']:
            if journal.get('pending'): raise ValueError('Uncertain upload must be reconciled before staging cleanup')
            container = arm('GET', proposal['containerID'], '2023-05-01')
            if not container or container.get('properties', {}).get('metadata', {}).get('luci_operation') != proposal['operationID']:
                raise ValueError('Staging container ownership changed')
            if cli(['storage', 'blob', 'exists', *arguments]).get('exists') is not False:
                raise ValueError('Source bytes lack an owned completed upload; preserve them')
            journal['completed'].append('source:delete-blob')
            journal.setdefault('receipts', {})['source:delete-blob'] = {'absent': True, 'neverUploaded': True}
            core.atomic_json(path, journal)
    if 'source:delete-blob' not in journal['completed']:
        def operation():
            row = cli(['storage', 'blob', 'show', *arguments])
            if row.get('metadata', {}).get('luci_operation') != proposal['operationID'] or row.get('metadata', {}).get('sha256') != proposal['payloadSHA256']:
                raise ValueError('Staging blob ownership changed; preserve unrelated bytes')
            cli(['storage', 'blob', 'delete', *arguments, '--if-match', row['properties']['etag']])
        def observe():
            return {'absent': True, 'payloadSHA256': proposal['payloadSHA256']} if cli(['storage', 'blob', 'exists', *arguments]).get('exists') is False else None
        return write_stage(core, path, journal, 'source:delete-blob', operation, observe, acknowledge, reconcile)
    if 'source:delete-container' not in journal['completed']:
        def operation():
            row = arm('GET', proposal['containerID'], '2023-05-01')
            if not row or row.get('properties', {}).get('metadata', {}).get('luci_operation') != proposal['operationID']:
                raise ValueError('Staging container ownership changed')
            remaining = cli(['storage', 'blob', 'list', '--account-name', proposal['storageAccountName'], '--container-name', proposal['containerName'], '--auth-mode', 'login'])
            if remaining != []: raise ValueError('Staging container has unrelated or unconfirmed bytes; preserve it')
            arm('DELETE', proposal['containerID'], '2023-05-01')
        def observe(): return {'absent': True} if arm('GET', proposal['containerID'], '2023-05-01') is None else None
        return write_stage(core, path, journal, 'source:delete-container', operation, observe, acknowledge, reconcile)
    journal.update(state='builder_aborted_staging_retired_gallery_retained' if journal.get('abortRequested') else 'private_images_bound_staging_retired', finalInvoiceVerified=False, connected=False)
    core.atomic_json(path, journal)
    return journal


def observed_packages(core, selection_path, manifest):
    parent = Path(selection_path).parent
    state = parent / 'azure-builder-state.json'
    if not state.exists(): return False
    journal = core.private_json(state)
    proposal = core.private_json(parent / 'azure-builder-plan.json')
    selected = core.private_json(selection_path)
    if journal.get('state') not in ('private_images_bound_staging_retirement_pending', 'private_images_bound_staging_retired') or proposal['sourceCommit'] != manifest['luciRevision'] or journal.get('planDigest') != proposal.get('planDigest') or journal.get('selectionDigest') != core.digest(selected): return False
    for role in ('runtime', 'controller'):
        receipt = journal.get('receipts', {}).get(role + ':install', {})
        if role + ':retired' not in journal.get('completed', []) or role + ':capture' not in journal.get('completed', []) or receipt.get('sourceCommit') != manifest['luciRevision'] or receipt.get('role') != role or receipt.get('packageSHA256') != selected[role + 'Package']['sha256'] or selected['terraformVariables'][role + '_image_version_id'] != proposal['roles'][role]['imageVersionID']:
            return False
    return True
