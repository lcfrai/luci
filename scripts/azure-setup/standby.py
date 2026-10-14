"""Restore the exact retired predecessor as a clean private standby, user AAD only."""
import base64
import importlib.util
import json
from pathlib import Path
import re
import uuid

API = '2024-07-01'
UUID = re.compile(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z')


def module(bundle, name, directory='azure-setup'):
    spec = importlib.util.spec_from_file_location('standby_' + name, Path(bundle) / directory / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result); return result


def capture(core, bundle, selected, recovery, provider, parent):
    """Pin predecessor identity before the original recovery mutates it."""
    read = lambda identifier, api: provider(['rest', '--method', 'get', '--url', 'https://management.azure.com' + identifier + '?api-version=' + api])
    descriptor = core.private_json(parent / 'azure-connection.json')
    identifier = recovery['oldControllerID']
    vm = read(identifier, API); properties = vm.get('properties', {})
    if (vm.get('id', '').lower() != identifier or descriptor.get('controllerResourceID') != identifier or
            descriptor.get('generation') != recovery['oldGeneration'] or descriptor.get('sourceCommit') != core.bundle(bundle)['luciRevision'] or
            descriptor.get('vmID') != properties.get('vmId') or not UUID.fullmatch(descriptor.get('vmID', '')) or not UUID.fullmatch(descriptor.get('bootID', ''))):
        raise ValueError('Authenticated predecessor VM/boot receipt is required before recovery')
    storage = properties.get('storageProfile', {})
    if storage.get('imageReference', {}).get('id', '').lower() != selected['terraformVariables']['controller_image_version_id'].lower():
        raise ValueError('Predecessor generic image source changed')
    interfaces = properties.get('networkProfile', {}).get('networkInterfaces', [])
    if len(interfaces) != 1: raise ValueError('Exact predecessor NIC is required')
    nic_id = interfaces[0]['id'].lower()
    nic = read(nic_id, '2023-09-01'); network = nic.get('properties', {})
    configs = network.get('ipConfigurations', [])
    expected_subnet = core.private_json(parent / 'azure-private-location.json')['vnetID'].lower() + '/subnets/' + selected['name'] + '-controller'
    if (nic.get('id', '').lower() != nic_id or network.get('virtualMachine', {}).get('id', '').lower() != identifier or
            not UUID.fullmatch(network.get('resourceGuid', '').lower()) or len(configs) != 1 or
            configs[0].get('properties', {}).get('subnet', {}).get('id', '').lower() != expected_subnet or
            configs[0]['properties'].get('privateIPAddress') != selected['terraformVariables']['controller_private_ip'] or
            configs[0]['properties'].get('publicIPAddress')):
        raise ValueError('Predecessor NIC/private subnet/source ownership changed')
    disk_id = storage.get('osDisk', {}).get('managedDisk', {}).get('id', '').lower()
    prefix = identifier.split('/providers/')[0] + '/providers/microsoft.compute/disks/'
    if not disk_id.startswith(prefix): raise ValueError('Predecessor OS disk is outside the owned group')
    disk = read(disk_id, '2023-10-02')
    if (disk.get('id', '').lower() != disk_id or disk.get('managedBy', '').lower() != identifier or
            disk.get('managedByExtended', []) not in ([], [disk.get('managedBy')]) or
            (disk.get('properties', {}).get('maxShares') or 1) != 1 or not disk.get('properties', {}).get('uniqueId')):
        raise ValueError('Exact predecessor OS disk ownership/immutable identity is unavailable')
    return {'id': identifier, 'vmID': properties['vmId'].lower(), 'bootID': descriptor['bootID'],
        'nicID': nic_id, 'nicGUID': network['resourceGuid'].lower(), 'subnetID': expected_subnet,
        'privateIP': configs[0]['properties']['privateIPAddress'], 'imageVersionID': storage['imageReference']['id'].lower(),
        'osDiskID': disk_id, 'osDiskUniqueID': disk['properties']['uniqueId'],
        'sourceCommit': descriptor['sourceCommit'], 'generation': recovery['oldGeneration']}


def fence_store(bundle, recovery, provider):
    recover = module(bundle, 'recover', 'azure-controller-disk')
    transport = recover.AzureHTTP('')
    transport.token = lambda resource: provider(['account', 'get-access-token', '--resource', resource])['accessToken']
    return recover.FenceBlob(transport, recovery['fenceBlobURL'])


def exact_fence(core, store, recovery, journal, proposal=None):
    record, etag = store.read()
    receipt = journal['recoveryReceipt']
    if (not record or not etag or record.get('state') != 'active' or
            any(record.get(key) != recovery[target] for key, target in (('controllerID', 'newControllerID'),
                ('diskID', 'diskID'), ('generation', 'newGeneration'), ('scope', 'scope'), ('ownerID', 'ownerID'))) or
            record.get('generationReceiptSHA256') != receipt['generationReceiptSHA256'] or
            record.get('successorDigest') != core.digest(recovery['controllerSuccessor'])):
        raise ValueError('Current active source/generation/controller fence changed; no standby effect')
    if proposal and (etag != proposal['activeFenceETag'] or core.digest(record) != proposal['activeFenceSHA256']):
        raise ValueError('Current active fence advanced; preserve pending standby operation')
    return record, etag


def owned_vm(arm, pinned, allow_recovery=False, recovery_identity=''):
    row = arm('GET', pinned['id'], API)
    prop = (row or {}).get('properties', {}); storage = prop.get('storageProfile', {})
    identity = (row or {}).get('identity', {})
    identities = set(key.lower() for key in identity.get('userAssignedIdentities', {}))
    allowed = ({recovery_identity}, set()) if allow_recovery == 'pending' else ({recovery_identity},) if allow_recovery else (set(),)
    if (not row or row.get('id', '').lower() != pinned['id'] or prop.get('vmId', '').lower() != pinned['vmID'] or
            storage.get('imageReference', {}).get('id', '').lower() != pinned['imageVersionID'] or storage.get('dataDisks', []) or
            [item.get('id', '').lower() for item in prop.get('networkProfile', {}).get('networkInterfaces', [])] != [pinned['nicID']] or
            identities not in allowed or identity.get('type', 'None') != ('UserAssigned' if identities else 'None')):
        raise ValueError('Retired predecessor is replaced, attached, foreign or still has authority')
    nic = arm('GET', pinned['nicID'], '2023-09-01')
    network = (nic or {}).get('properties', {}); configs = network.get('ipConfigurations', [])
    if (not nic or nic.get('id', '').lower() != pinned['nicID'] or network.get('resourceGuid', '').lower() != pinned['nicGUID'] or
            network.get('virtualMachine', {}).get('id', '').lower() != pinned['id'] or len(configs) != 1 or
            configs[0].get('properties', {}).get('subnet', {}).get('id', '').lower() != pinned['subnetID'] or
            configs[0]['properties'].get('privateIPAddress') != pinned['privateIP'] or configs[0]['properties'].get('publicIPAddress')):
        raise ValueError('Exact retained private predecessor NIC changed')
    return row


def power(arm, identifier, state):
    view = arm('GET', identifier + '/instanceView', API)
    return bool(view and any(row.get('code') == 'PowerState/' + state for row in view.get('statuses', [])))


def replacement_disk(arm, pinned, allow_recovery=False, recovery_identity=''):
    vm = owned_vm(arm, pinned, allow_recovery, recovery_identity)
    if vm.get('properties', {}).get('provisioningState') != 'Succeeded': return None
    identifier = vm['properties']['storageProfile'].get('osDisk', {}).get('managedDisk', {}).get('id', '').lower()
    if identifier == pinned['osDiskID']: return None
    if not identifier.startswith(pinned['osDiskID'].rsplit('/', 1)[0] + '/'):
        raise ValueError('Replacement OS disk is outside the exact owned group')
    row = arm('GET', identifier, '2023-10-02'); prop = (row or {}).get('properties', {})
    creation = prop.get('creationData', {})
    if (not row or row.get('id', '').lower() != identifier or row.get('managedBy', '').lower() != pinned['id'] or
            prop.get('provisioningState') != 'Succeeded' or not prop.get('uniqueId') or prop['uniqueId'] == pinned['osDiskUniqueID'] or
            (prop.get('maxShares') or 1) != 1 or row.get('managedByExtended', []) not in ([], [row.get('managedBy')]) or creation.get('createOption') != 'FromImage' or
            creation.get('galleryImageReference', {}).get('id', '').lower() != pinned['imageVersionID']):
        raise ValueError('Actual replacement OS disk/image lineage is not verified')
    old = arm('GET', pinned['osDiskID'], '2023-10-02')
    if (not old or old.get('id', '').lower() != pinned['osDiskID'] or old.get('managedBy') or old.get('managedByExtended') or
            old.get('properties', {}).get('uniqueId') != pinned['osDiskUniqueID']):
        raise ValueError('Detached predecessor OS disk is not preserved')
    return {'diskID': identifier, 'uniqueID': prop['uniqueId'], 'imageVersionID': pinned['imageVersionID'],
        'oldOSDiskRetained': True, 'providerReadbackVerified': True}


def clean_script(pinned, source, package_sha):
    expected = dict(pinned, sourceCommit=source, packageSHA256=package_sha)
    return """set -eu
test "$(systemctl show luci-api.service --property=ActiveState --value)" = inactive
test "$(systemctl show luci-api.service --property=SubState --value)" = dead
test "$(systemctl show luci-api.service --property=MainPID --value)" = 0
python3 - <<'PY'
import hashlib,json,os,pathlib,stat,urllib.request
expected=json.loads(%s)
def bounded(path,limit):
    info=path.lstat()
    if path.is_symlink() or not stat.S_ISREG(info.st_mode) or not 0<info.st_size<=limit: raise SystemExit('generic receipt bound differs')
    with path.open('rb') as stream: data=stream.read(limit+1)
    if len(data)>limit: raise SystemExit('generic receipt exceeded bound')
    return data
for name in ('operator-template.json','operator.json','azure-controller.env','azure-private-ca.crt','azure-controller-disk-initial.json','azure-controller-disk-restore.json','azure-controller-successor-selection.json'):
    path=pathlib.Path('/etc/luci')/name
    if path.exists() or path.is_symlink(): raise SystemExit('predecessor configuration remains')
receipt=json.loads(bounded(pathlib.Path('/opt/luci/image-source-receipt.json'),16384))
if receipt.get('sourceCommit')!=expected['sourceCommit'] or receipt.get('role')!='controller' or receipt.get('packageSHA256')!=expected['packageSHA256']:
    raise SystemExit('generic image source differs')
root=pathlib.Path('/opt/luci/azure-controller'); manifest=json.loads(bounded(root/'manifest.json',32768))
if manifest.get('luciRevision')!=expected['sourceCommit'] or not 1<=len(manifest['files'])<=16: raise SystemExit('generic package source differs')
total=0
for name,entry in manifest['files'].items():
    path=root/name
    if pathlib.PurePosixPath(name).is_absolute() or '..' in pathlib.PurePosixPath(name).parts or any(p.is_symlink() for p in (path,*path.parents)):
        raise SystemExit('generic package path differs')
    info=path.stat(); total+=info.st_size
    if not stat.S_ISREG(info.st_mode) or not 0<info.st_size<=96<<20 or info.st_size!=entry['size'] or total>112<<20: raise SystemExit('generic package bound differs')
    digest=hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda:stream.read(1<<20),b''): digest.update(block)
    if digest.hexdigest()!=entry['sha256']: raise SystemExit('generic package bytes differ')
request=urllib.request.Request('http://169.254.169.254/metadata/instance?api-version=2021-02-01',headers={'Metadata':'true'})
with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request,timeout=8) as response: data=response.read(65537)
if len(data)>65536: raise SystemExit('guest identity exceeded bound')
metadata=json.loads(data)
compute=metadata['compute']; boot=pathlib.Path('/proc/sys/kernel/random/boot_id').read_text().strip()
if compute.get('resourceId','').lower()!=expected['id'] or compute.get('vmId','').lower()!=expected['vmID'] or compute.get('storageProfile',{}).get('dataDisks',[]) or boot==expected['bootID']:
    raise SystemExit('new generic guest boot/identity is unavailable')
print('LUCI_AZURE_CLEAN_STANDBY='+json.dumps({'vmID':expected['vmID'],'bootID':boot,'sourceCommit':expected['sourceCommit'],'packageSHA256':expected['packageSHA256']}))
PY
""" % repr(json.dumps(expected))


def restore(core, bundle, selection_path, provider, ask=input, emit=print, arm=None, store=None):
    parent = Path(selection_path).parent
    journal = core.private_json(parent / 'azure-successor-state.json')
    if journal.get('state') != 'successor_active' or not journal.get('activeSelectionPath'):
        raise ValueError('Exact active signed successor is required before destructive standby restoration')
    active_path = Path(journal['activeSelectionPath']); active_parent = active_path.parent
    if (active_parent / 'azure-successor-state.json').exists():
        return restore(core, bundle, active_path, provider, ask, emit, arm, store)
    selected = core.private_json(active_path); manifest = core.bundle(bundle)
    if manifest['luciRevision'] != journal['sourceCommit'] or 'standbyPredecessor' not in journal:
        raise ValueError('Pinned predecessor source/identity receipt is missing; no adoption or reimage')
    recovery = core.private_json(journal['recoverySelectionPath'])
    if core.digest(recovery) != journal['recoverySelectionSHA256']:
        raise ValueError('Signed recovery selection changed')
    pinned = journal['standbyPredecessor']
    if pinned['id'] != recovery['oldControllerID'] or pinned['sourceCommit'] != manifest['luciRevision']:
        raise ValueError('Standby predecessor is outside the signed source/recovery chain')
    arm = arm or module(bundle, 'foundation').AzureARM(provider)
    store = store or fence_store(bundle, recovery, provider)
    fence, etag = exact_fence(core, store, recovery, journal)
    path = active_parent / 'azure-standby-state.json'
    state = core.private_json(path) if path.exists() else None
    retirement = module(bundle, 'retirement')
    if state:
        proposal = state['proposal']
        if (proposal['sourceCommit'] != manifest['luciRevision'] or proposal['predecessor'] != pinned or
                proposal['planDigest'] != core.digest({key: value for key, value in proposal.items() if key != 'planDigest'})):
            raise ValueError('Retained standby plan/source changed')
    else:
        vm = owned_vm(arm, pinned)
        if not power(arm, pinned['id'], 'deallocated'): raise ValueError('Only positively deallocated predecessor can be reimaged')
        disk = arm('GET', pinned['osDiskID'], '2023-10-02')
        if not disk or disk.get('managedBy', '').lower() != pinned['id'] or disk.get('properties', {}).get('uniqueId') != pinned['osDiskUniqueID']:
            raise ValueError('Predecessor OS disk changed before backup plan')
        operation = uuid.uuid4().hex
        snapshot_id = pinned['osDiskID'].rsplit('/disks/', 1)[0] + '/snapshots/' + selected['name'] + '-predecessor-os-' + operation[:12]
        proposal = {'version': 1, 'sourceCommit': manifest['luciRevision'], 'scope': recovery['scope'], 'ownerID': recovery['ownerID'],
            'generation': recovery['newGeneration'], 'operationID': operation, 'region': selected['region'], 'predecessor': pinned,
            'activeControllerID': recovery['newControllerID'], 'activeFenceETag': etag, 'activeFenceSHA256': core.digest(fence),
            'snapshot': {'sourceDiskID': pinned['osDiskID'], 'sourceUniqueID': pinned['osDiskUniqueID'], 'sourceOwnerID': pinned['id'], 'snapshotID': snapshot_id},
            'requiredActions': ['Microsoft.Compute/virtualMachines/read,write,instanceView/read,reimage/action,start/action,runCommand/action at exact predecessor',
                'existing recovery identity assign/action; no new identity or role grant', 'Microsoft.Compute/disks/read,write at owned old/replacement OS disks',
                'Microsoft.Compute/snapshots/read,write at exact planned private snapshot'],
            'retainedCharges': ['detached old OS disk', 'private old-OS snapshot preserving logs/receipts', 'new managed OS disk observed from reimage',
                'running private standby compute', 'existing authoritative disk/gallery/state/Key Vault/network charges'],
            'message': 'Destructive same-VM generic reimage only after exact active fence and detached authoritative data disk. Retain old disk/snapshot. Estimates are not final bills; no additional VM, roles or identity grants.'}
        proposal['planDigest'] = core.digest(proposal)
    exact_fence(core, store, recovery, journal, proposal)
    if not module(bundle, 'guided').approve_stage(core, active_parent, manifest, selected, 'standby-restoration', proposal, ask, emit):
        return {'state': 'standby_restoration_review_paused', 'connected': False}
    if state is None:
        state = {'version': 1, 'sourceCommit': manifest['luciRevision'], 'scope': recovery['scope'], 'generation': recovery['newGeneration'],
            'planDigest': proposal['planDigest'], 'proposal': proposal, 'completed': [], 'receipts': {}}
    def effect(key, operation, observe):
        exact_fence(core, store, recovery, journal, proposal)
        authoritative = arm('GET', recovery['diskID'], '2023-10-02')
        if (not authoritative or authoritative.get('id', '').lower() != recovery['diskID'] or
                authoritative.get('managedBy', '').lower() != recovery['newControllerID'] or
                (authoritative.get('properties', {}).get('maxShares') or 1) != 1):
            raise ValueError('Authoritative data disk moved; no predecessor effect')
        return retirement.effect(core, path, state, key, operation, observe)
    def snapshot_observe(): return retirement.snapshot_receipt(core, proposal, proposal['snapshot'], arm)
    def backup():
        owned_vm(arm, pinned)
        if not power(arm, pinned['id'], 'deallocated'): raise ValueError('Predecessor resumed; no backup/reimage')
        disk = arm('GET', pinned['osDiskID'], '2023-10-02')
        if (not disk or disk.get('id', '').lower() != pinned['osDiskID'] or disk.get('managedBy', '').lower() != pinned['id'] or
                disk.get('properties', {}).get('uniqueId') != pinned['osDiskUniqueID'] or (disk.get('properties', {}).get('maxShares') or 1) != 1):
            raise ValueError('Predecessor OS disk changed before snapshot effect')
        if arm('GET', proposal['snapshot']['snapshotID'], '2023-10-02') is not None: raise ValueError('Snapshot exists before intent; no adoption/overwrite')
        arm('PUT', proposal['snapshot']['snapshotID'], '2023-10-02', {'location': proposal['region'], 'sku': {'name': 'Standard_LRS'},
            'tags': {'luci-' + key: proposal[value] for key, value in (('source', 'sourceCommit'), ('owner', 'ownerID'), ('scope', 'scope'), ('generation', 'generation'), ('operation', 'operationID'))},
            'properties': {'creationData': {'createOption': 'Copy', 'sourceResourceId': pinned['osDiskID']}, 'networkAccessPolicy': 'DenyAll', 'publicNetworkAccess': 'Disabled'}})
    effect('backup-old-os', backup, snapshot_observe)
    if not snapshot_observe(): raise RuntimeError('Actual predecessor snapshot changed; preserve the old OS disk')
    def detach_observe():
        vm = owned_vm(arm, pinned)
        if vm['properties']['storageProfile']['osDisk'].get('deleteOption') == 'Detach': return {'deleteOption': 'Detach'}
        return None
    def detach():
        vm = owned_vm(arm, pinned)
        if not power(arm, pinned['id'], 'deallocated'): raise ValueError('Predecessor is not deallocated before Detach/reimage')
        os_disk = dict(vm['properties']['storageProfile']['osDisk'], deleteOption='Detach')
        arm('PATCH', pinned['id'], API, {'properties': {'storageProfile': {'osDisk': os_disk}}}, headers={'If-Match': vm['etag']})
    effect('retain-old-os', detach, detach_observe)
    def reimage():
        owned_vm(arm, pinned)
        if not power(arm, pinned['id'], 'deallocated') or not detach_observe() or not snapshot_observe():
            raise ValueError('Exact deallocated, backed-up, retained predecessor is required before reimage')
        body = {'exactVersion': pinned['imageVersionID'].rsplit('/', 1)[-1],
            'osProfile': {'customData': base64.b64encode(b'#cloud-config\n{}\n').decode()}}
        arm('POST', pinned['id'] + '/reimage', API, body)
    effect('reimage', reimage, lambda: replacement_disk(arm, pinned))
    allow_identity = 'pending' if state.get('pending') == 'assign-recovery' else True if 'assign-recovery' in state['completed'] else False
    observed_disk = replacement_disk(arm, pinned, allow_identity, recovery['recoveryIdentityID'])
    if not observed_disk or observed_disk != state['receipts']['reimage']: raise ValueError('Observed replacement OS disk changed')
    for identifier, unique_id, owner in ((pinned['osDiskID'], pinned['osDiskUniqueID'], ''),
            (observed_disk['diskID'], observed_disk['uniqueID'], pinned['id'])):
        def disk_read(identifier=identifier, unique_id=unique_id, owner=owner):
            row = arm('GET', identifier, '2023-10-02')
            if (not row or row.get('id', '').lower() != identifier or row.get('properties', {}).get('uniqueId') != unique_id or
                    row.get('managedBy', '').lower() != owner):
                raise ValueError('Owned retained/replacement OS disk changed before private export policy')
            return row
        def private_read(disk_read=disk_read):
            row = disk_read(); prop = row['properties']
            return {'diskID': row['id'].lower(), 'uniqueID': prop['uniqueId'], 'publicNetworkAccess': 'Disabled'} if (
                prop.get('networkAccessPolicy') == 'DenyAll' and prop.get('publicNetworkAccess') == 'Disabled') else None
        def make_private(identifier=identifier, disk_read=disk_read):
            row = disk_read()
            arm('PATCH', identifier, '2023-10-02', {'properties': {'networkAccessPolicy': 'DenyAll', 'publicNetworkAccess': 'Disabled'}}, headers={'If-Match': row['etag']})
        effect('private-os:' + identifier, make_private, private_read)
    def start():
        owned_vm(arm, pinned)
        if replacement_disk(arm, pinned) != observed_disk: raise ValueError('Replacement disk changed before startup')
        if not power(arm, pinned['id'], 'running'): arm('POST', pinned['id'] + '/start', API)
    effect('start-clean-guest', start, lambda: {'running': True} if power(arm, pinned['id'], 'running') else None)
    def clean():
        response = provider(['vm', 'run-command', 'invoke', '--ids', pinned['id'], '--command-id', 'RunShellScript', '--scripts',
            clean_script(pinned, manifest['luciRevision'], selected['controllerPackage']['sha256'])])
        rows = [line.removeprefix('LUCI_AZURE_CLEAN_STANDBY=') for item in response.get('value', []) for line in item.get('message', '').splitlines() if line.startswith('LUCI_AZURE_CLEAN_STANDBY=')]
        result = json.loads(rows[0]) if len(rows) == 1 else {}
        if (any(result.get(key) != value for key, value in {'vmID': pinned['vmID'], 'sourceCommit': manifest['luciRevision'],
                'packageSHA256': selected['controllerPackage']['sha256']}.items()) or
                not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', result.get('bootID', '')) or result['bootID'] == pinned['bootID']):
            raise RuntimeError('Positive new clean generic boot is unavailable; no identity reassignment')
        return result
    # This is a read-only guest proof; repeats inspect bytes/boot, never reimage.
    fresh = clean()
    if state['receipts'].get('clean-boot') and state['receipts']['clean-boot'] != fresh and 'assign-recovery' not in state['completed']:
        raise ValueError('Observed clean boot changed before identity assignment')
    state['receipts']['clean-boot'] = fresh; core.atomic_json(path, state)
    def assigned():
        owned_vm(arm, pinned, True, recovery['recoveryIdentityID'])
        return {'recoveryIdentityID': recovery['recoveryIdentityID'], 'bootID': fresh['bootID'], 'diskID': observed_disk['diskID']}
    def assign():
        vm = owned_vm(arm, pinned)
        if replacement_disk(arm, pinned) != observed_disk or clean() != fresh: raise ValueError('Clean predecessor changed before recovery identity assignment')
        arm('PATCH', pinned['id'], API, {'identity': {'type': 'UserAssigned', 'userAssignedIdentities': {recovery['recoveryIdentityID']: {}}}}, headers={'If-Match': vm['etag']})
    if 'assign-recovery' not in state['completed']:
        effect('assign-recovery', assign, assigned)
    else: assigned()
    exact_fence(core, store, recovery, journal, proposal)
    state.update(state='fresh_private_standby_observed', predecessorOSDiskRetained=True, snapshotRestorationTested=False,
        replacementOSDisk=observed_disk, newBootID=fresh['bootID'], retainedCharges=proposal['retainedCharges'], providerInvoiceVerified=False,
        message='Same owned predecessor is now a clean generic private standby with only existing recovery identity. Old OS disk/snapshot remain recoverable; current task identity/configuration is unchanged. Live recovery and invoices require separate evidence.')
    core.atomic_json(path, state); return state
