"""User-AAD owned retirement with exact settlement, backups and no unknown replay."""
import base64
from datetime import datetime, timezone
import importlib.util
import json
from pathlib import Path
import re
import uuid


def module(bundle, name, directory='azure-setup'):
    spec = importlib.util.spec_from_file_location('luci_retirement_' + name, Path(bundle) / (directory + '/' + name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result); return result


def exact_resource(arm, identifier, api):
    row = arm('GET', identifier, api)
    if not row or row.get('id', '').lower() != identifier.lower(): raise ValueError('Exact owned retirement resource is unavailable')
    return row


def plan(core, bundle, selection_path, arm, provider):
    manifest = core.bundle(bundle); selected = core.private_json(selection_path); variables = core.requested(selected)
    parent = Path(selection_path).parent; path = parent / 'azure-retirement-plan.json'
    standby = parent / 'azure-standby-state.json'
    if standby.exists() and core.private_json(standby).get('state') != 'fresh_private_standby_observed':
        raise ValueError('Standby restoration remains pending; retirement cannot overlap it')
    if path.exists():
        result = core.private_json(path)
        if (result.get('sourceCommit') != manifest['luciRevision'] or result.get('selectionDigest') != core.digest(selected) or
                core.digest({key: value for key, value in result.items() if key != 'planDigest'}) != result.get('planDigest')):
            raise ValueError('Retained retirement source/resource selection changed')
        return result
    infrastructure = core.private_json(parent / 'azure-setup-state.json')
    if infrastructure.get('state') != 'controller_finalized_private_route_unverified' or infrastructure.get('selectionDigest') != core.digest(selected):
        raise ValueError('Exact installed controller generation is required before retirement')
    ids = infrastructure['resourceIDs']; controller = ids['controller_instance_id'].lower(); standby = ids['controller_recovery_instance_id'].lower()
    scale = ids['runtime_scale_set_id'].lower(); disk_id = ids['controller_disk_id'].lower()
    prefix = '/subscriptions/' + variables['subscription_id'].lower() + '/resourcegroups/' + variables['resource_group_name'].lower() + '/providers/microsoft.compute/'
    if (not all(value.startswith(prefix + 'virtualmachines/') for value in (controller, standby)) or
            not scale.startswith(prefix + 'virtualmachinescalesets/') or not disk_id.startswith(prefix + 'disks/')):
        raise ValueError('Retirement resources differ from exact selected account/group')
    scale_row = exact_resource(arm, scale, '2023-09-01')
    if scale_row.get('properties', {}).get('virtualMachineProfile', {}).get('storageProfile', {}).get('imageReference', {}).get('id', '').lower() != variables['runtime_image_version_id'].lower():
        raise ValueError('Retirement runtime image/source boundary changed')
    instances = arm('GET', scale + '/virtualMachines', '2023-09-01')
    if not instances or instances.get('nextLink') or len(instances.get('value', [])) != 1:
        raise ValueError('Runtime host inventory is ambiguous; no broad scale-set deallocation')
    runtime = instances['value'][0]
    instance_id = runtime.get('instanceId', '')
    if not re.fullmatch(r'[0-9]{1,12}', instance_id) or runtime.get('id', '').lower() != scale + '/virtualmachines/' + instance_id:
        raise ValueError('Runtime instance identity changed')
    runtime_id = scale + '/virtualMachines/' + instance_id
    rows = {runtime_id: runtime}
    for identifier in dict.fromkeys((controller, standby)):
        rows[identifier] = exact_resource(arm, identifier, '2023-09-01')
    recovery_identity = ids['controller_recovery_identity_resource_id'].lower()
    execution = next((identifier for identifier in (standby, controller) if recovery_identity in {
        key.lower() for key in rows[identifier].get('identity', {}).get('userAssignedIdentities', {})}), '')
    if not execution: raise ValueError('Exact existing recovery identity host is unavailable; no new grant is created')
    immutable = {}
    for identifier, row in rows.items():
        value = row.get('properties', {}).get('vmId', '')
        if not re.fullmatch(r'[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}', value):
            raise ValueError('Immutable owned compute identity is unavailable')
        immutable[identifier] = value.lower()
    runtime_disk = runtime.get('properties', {}).get('storageProfile', {}).get('osDisk', {}).get('managedDisk', {}).get('id', '').lower()
    if not runtime_disk.startswith(prefix + 'disks/') or not disk_id.startswith(prefix + 'disks/'):
        raise ValueError('Retirement backup disks are outside the exact owned group')
    operation = uuid.uuid4().hex
    snapshots = {}
    for role, identifier, owner in (('controller-state', disk_id, controller), ('runtime-templates', runtime_disk, runtime_id)):
        row = exact_resource(arm, identifier, '2023-10-02')
        prop = row.get('properties', {})
        if (row.get('managedBy', '').lower() != owner.lower() or row.get('managedByExtended', []) not in ([], [row.get('managedBy')]) or
                prop.get('diskState') != 'Attached' or (prop.get('maxShares') or 1) != 1 or not prop.get('uniqueId')):
            raise ValueError('Backup source disk is foreign, shared, unattached or lacks immutable identity')
        snapshots[role] = {'sourceDiskID': identifier, 'sourceUniqueID': prop['uniqueId'], 'sourceOwnerID': owner,
            'snapshotID': prefix + 'snapshots/' + selected['name'] + '-' + role + '-' + operation[:12]}
    template = core.private_json(parent / 'template/operator.json') if (parent / 'template/operator.json').exists() else core.private_json(parent / 'operator-template.json')
    if 'cpuCount' in template:
        cpu, memory, guest = template['cpuCount'], template['memoryMB'], template['fromImage']
    else:
        cpu, memory, guest = template['template']['cpuMillis'] // 1000, template['template']['memoryMiB'], template['templateFromImage']
    result = {'version': 1, 'state': 'installation_retirement_review_required', 'sourceCommit': manifest['luciRevision'],
        'scope': variables['scope'], 'generation': variables['generation'], 'ownerID': variables['controller_owner_id'],
        'selectionDigest': core.digest(selected), 'operationID': operation, 'region': selected['region'], 'snapshots': snapshots,
        'controllerID': controller, 'recoveryID': execution, 'recoveryClientID': ids['controller_recovery_identity_client_id'],
        'diskID': disk_id, 'filesystemUUID': variables['controller_filesystem_uuid'], 'fenceBlobURL': ids['controller_fence_blob_url'],
        'runtimeID': runtime_id, 'computeIDs': list(dict.fromkeys([runtime_id] + [v for v in (controller, standby) if v != execution] + [execution])),
        'immutableComputeIDs': immutable, 'priceVerified': False, 'finalInvoiceVerified': False,
        'requiredActions': ['Microsoft.Compute/virtualMachines/read,instanceView/read,deallocate/action,runCommand/action at exact controller/standby IDs',
            'Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read,instanceView/read,deallocate/action,runCommand/action at the exact runtime instance',
            'Microsoft.Compute/disks/read at the two source disks; Microsoft.Compute/snapshots/read,write at the two planned private IDs',
            'existing private off-disk fence CAS using recovery identity; no new managed-identity grant'],
        'retainedCharges': ['authoritative and OS managed disks', 'two private backup snapshots', 'gallery images and replicas',
            'Blob/state/backup bytes and Key Vault versions', 'private endpoints/DNS/network charges where applicable', 'uncertain compute until exact observed deallocation'],
        'message': 'No lease is released by retirement metadata. Actual authoritative settlement, quiesced source disks and observed snapshots must precede deallocation. Retained storage is not a zero bill.'}
    result['toolSelection'] = {'version': 1, 'sourceCommit': manifest['luciRevision'], 'scope': variables['scope'], 'generation': variables['generation'],
        'controllerResourceID': controller, 'runtimeScaleSetID': scale, 'dataDiskID': disk_id, 'controllerClientID': ids['controller_identity_client_id'],
        'fenceBlobURL': ids['controller_fence_blob_url'], 'filesystemUUID': variables['controller_filesystem_uuid'], 'mountPath': variables['controller_mount_path'],
        'operatorTemplatePath': variables['controller_operator_template_path'], 'operatorTemplateSHA256': variables['controller_operator_template_sha256'],
        'templateName': template.get('name', selected['name']), 'cpuCount': cpu, 'memoryMiB': memory, 'fromImage': guest}
    if standby.exists():
        restored = core.private_json(standby)
        result['retainedStandbyResources'] = {'oldOSDiskID': restored['proposal']['predecessor']['osDiskID'],
            'oldOSSnapshotID': restored['proposal']['snapshot']['snapshotID'], 'replacementOSDisk': restored['replacementOSDisk']}
        result['retainedCharges'].extend(value for value in restored['retainedCharges'] if value not in result['retainedCharges'])
    result['planDigest'] = core.digest(result)
    core.atomic_json(path, result); return result


def guest_selection(proposal):
    result = {key: proposal[key] for key in ('version', 'sourceCommit', 'scope', 'ownerID', 'generation', 'diskID',
        'filesystemUUID', 'controllerID', 'recoveryID', 'recoveryClientID', 'fenceBlobURL', 'planDigest')}
    result['toolSelection'] = dict(proposal['toolSelection'], retirementPlanDigest=proposal['planDigest'])
    return result


def owned_compute(arm, proposal, identifier):
    row = exact_resource(arm, identifier, '2023-09-01')
    if row.get('properties', {}).get('vmId', '').lower() != proposal['immutableComputeIDs'][identifier]:
        raise ValueError('Compute was replaced; preserve the foreign resource before any guest effect')
    return row


def guest(core, proposal, provider, arm, action, inspect=False, backups_sha=''):
    for identifier in dict.fromkeys((proposal['recoveryID'], proposal['controllerID'])):
        owned_compute(arm, proposal, identifier)
    data = base64.b64encode(core.canonical(guest_selection(proposal))).decode()
    mode = ('inspect' if action == 'quiesce' else 'cancel-inspect') if inspect and action in ('quiesce', 'cancel') else action
    options = (' --backups-sha256 ' + backups_sha + (' --inspect' if inspect else '')) if action == 'close' else ''
    script = """set -eu
python3 - <<'PY'
import base64,os,pathlib,stat,tempfile
data=base64.b64decode(%s)
target=pathlib.Path('/etc/luci/azure-installation-retirement.json')
if target.is_symlink() or target.exists():
    if target.is_symlink() or not stat.S_ISREG(target.stat().st_mode) or target.stat().st_mode & 0o077 or target.read_bytes()!=data:
        raise SystemExit('retained installation retirement selection changed')
elif %s:
    raise SystemExit('retirement transfer remains uncertain; no repeat write')
else:
    fd,name=tempfile.mkstemp(dir=target.parent);os.fchmod(fd,0o600)
    with os.fdopen(fd,'wb') as output: output.write(data);output.flush();os.fsync(output.fileno())
    os.replace(name,target)
PY
receipt=$(python3 /opt/luci/azure-controller/bin/retire.py %s /etc/luci/azure-installation-retirement.json%s)
printf 'LUCI_AZURE_INSTALLATION_RETIREMENT=%%s\\n' "$receipt"
""" % (repr(data), repr(inspect or action == 'close'), mode, options)
    response = provider(['vm', 'run-command', 'invoke', '--ids', proposal['recoveryID'], '--command-id', 'RunShellScript', '--scripts', script])
    rows = [line.removeprefix('LUCI_AZURE_INSTALLATION_RETIREMENT=') for item in response.get('value', []) for line in item.get('message', '').splitlines() if line.startswith('LUCI_AZURE_INSTALLATION_RETIREMENT=')]
    if len(rows) != 1: raise RuntimeError('Exact installation retirement readback remains uncertain')
    result = json.loads(rows[0])
    if any(result.get(key) != proposal[key] for key in ('scope', 'generation', 'planDigest')):
        raise ValueError('Installation retirement source/generation/plan changed')
    if action == 'quiesce' and (result.get('sourceCommit') != proposal['sourceCommit'] or result.get('taskReservationsSettled') is not True or result.get('state') != 'retirement_settled'):
        raise ValueError('Actual task/provider/reservation settlement is unavailable')
    if action == 'close' and (result.get('state') != 'retirement_closed_pending_compute' or result.get('backupsSHA256') != backups_sha):
        raise ValueError('Exact backup-bound off-disk retirement closure is unavailable')
    if action == 'cancel' and (result.get('sourceCommit') != proposal['sourceCommit'] or result.get('state') != 'retirement_cancelled_active' or result.get('taskReservationsReleased') is not False):
        raise ValueError('Exact active cancellation readback preserving task leases is unavailable')
    return result


def runtime(core, proposal, provider, arm, mode):
    owned_compute(arm, proposal, proposal['runtimeID'])
    script = 'set -eu\nreceipt=$(python3 /opt/luci/azure-embed/package/bin/service.py retirement-' + mode + ' /etc/luci/azure-embed-deployment.json ' + proposal['planDigest'] + ')\nprintf "LUCI_AZURE_RUNTIME_RETIRED=%s\\n" "$receipt"'
    response = provider(['vmss', 'run-command', 'invoke', '--ids', proposal['runtimeID'], '--command-id', 'RunShellScript', '--scripts', script])
    rows = [line.removeprefix('LUCI_AZURE_RUNTIME_RETIRED=') for item in response.get('value', []) for line in item.get('message', '').splitlines() if line.startswith('LUCI_AZURE_RUNTIME_RETIRED=')]
    if len(rows) != 1: raise RuntimeError('Runtime retirement/resume remains uncertain')
    result = json.loads(rows[0])
    resume = mode.startswith('resume')
    if (any(result.get(key) != proposal[key] for key in ('sourceCommit', 'scope', 'generation', 'planDigest')) or
            result.get('state') != ('retirement_cancelled_active' if resume else 'retirement_stopped') or
            (result.get('taskReservationsReleased') is not False if resume else result.get('runtimeStorageOnOSDiskVerified') is not True)):
        raise ValueError('Exact runtime retirement/resume proof is unavailable')
    return result


def effect(core, path, state, key, operation, observe):
    if key in state['completed']: return state
    if state.get('pending'):
        if state['pending'] != key: raise ValueError('Another retirement effect is retained; no new mutation')
    else:
        state.update(state='retirement_effect_pending', pending=key); core.atomic_json(path, state)
        operation()
    receipt = observe()
    if receipt is None: raise RuntimeError('Retirement effect remains uncertain; exact readback only, no mutation retry')
    state['completed'].append(key); state.setdefault('receipts', {})[key] = receipt
    state.pop('pending', None); state['state'] = 'retirement_stage_observed'
    state['observedAtUTC'] = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')
    core.atomic_json(path, state)
    return state


def snapshot_receipt(core, proposal, item, arm):
    row = arm('GET', item['snapshotID'], '2023-10-02')
    properties = (row or {}).get('properties', {}); creation = properties.get('creationData', {})
    tags = (row or {}).get('tags', {})
    if (row and row.get('id', '').lower() == item['snapshotID'].lower() and properties.get('provisioningState') == 'Succeeded' and
            properties.get('networkAccessPolicy') == 'DenyAll' and properties.get('publicNetworkAccess') == 'Disabled' and
            creation.get('createOption') == 'Copy' and creation.get('sourceResourceId', '').lower() == item['sourceDiskID'].lower() and
            creation.get('sourceUniqueId') == item['sourceUniqueID'] and properties.get('uniqueId') and
            all(tags.get('luci-' + key) == proposal[value] for key, value in (('source', 'sourceCommit'), ('owner', 'ownerID'), ('scope', 'scope'), ('generation', 'generation'), ('operation', 'operationID')))):
        return {'snapshotID': item['snapshotID'], 'snapshotUniqueID': properties['uniqueId'], 'sourceDiskID': item['sourceDiskID'],
            'sourceUniqueID': item['sourceUniqueID'], 'providerReadbackSHA256': core.digest(row), 'restorationTested': False}
    return None


def run(core, bundle, selection_path, ask=input, emit=print, provider=None, arm=None):
    guided = module(bundle, 'guided'); provider = provider or guided.az_json
    arm = arm or module(bundle, 'foundation').AzureARM(provider)
    parent = Path(selection_path).parent
    # The predecessor's immutable source remains available after recovery; use
    # the newly active generation's exact local files for final retirement.
    successor = parent / 'azure-successor-state.json'
    if successor.exists():
        active = core.private_json(successor).get('activeSelectionPath')
        if not active: raise ValueError('Controller recovery is still pending; retirement cannot replace its fence')
        return run(core, bundle, Path(active), ask, emit, provider, arm)
    if (parent / 'azure-retirement-cancel-state.json').exists():
        return cancel(core, bundle, selection_path, ask, emit, provider, arm)
    proposal = plan(core, bundle, selection_path, arm, provider)
    selected = core.private_json(selection_path)
    if not guided.approve_stage(core, parent, core.bundle(bundle), selected, 'installation-retirement', proposal, ask, emit):
        return {'state': 'installation_retirement_review_paused', 'connected': False}
    path = parent / 'azure-retirement-state.json'
    state = core.private_json(path) if path.exists() else {'version': 1, 'planDigest': proposal['planDigest'], 'sourceCommit': proposal['sourceCommit'],
        'scope': proposal['scope'], 'generation': proposal['generation'], 'completed': [], 'receipts': {}}
    if state.get('planDigest') != proposal['planDigest']: raise ValueError('Retained retirement journal changed')
    if 'quiesce' not in state['completed']:
        returned = {}
        effect(core, path, state, 'quiesce', lambda: returned.update(value=guest(core, proposal, provider, arm, 'quiesce')),
            lambda: returned.get('value') or guest(core, proposal, provider, arm, 'quiesce', inspect=True))
    if 'host-stop' not in state['completed']:
        returned = {}
        def host(inspect):
            return runtime(core, proposal, provider, arm, 'inspect' if inspect else 'stop')
        effect(core, path, state, 'host-stop', lambda: returned.update(value=host(False)), lambda: returned.get('value') or host(True))
    for role, item in proposal['snapshots'].items():
        key = 'backup-' + role
        if key in state['completed']: continue
        def operation(item=item):
            disk = exact_resource(arm, item['sourceDiskID'], '2023-10-02')
            if disk.get('properties', {}).get('uniqueId') != item['sourceUniqueID'] or disk.get('managedBy', '').lower() != item['sourceOwnerID'].lower():
                raise ValueError('Quiesced backup source disk was replaced or moved')
            if arm('GET', item['snapshotID'], '2023-10-02') is not None: raise ValueError('Snapshot ID already exists; no adoption/overwrite')
            arm('PUT', item['snapshotID'], '2023-10-02', {'location': proposal['region'], 'sku': {'name': 'Standard_LRS'},
                'tags': {'luci-' + key: proposal[value] for key, value in (('source', 'sourceCommit'), ('owner', 'ownerID'), ('scope', 'scope'), ('generation', 'generation'), ('operation', 'operationID'))},
                'properties': {'creationData': {'createOption': 'Copy', 'sourceResourceId': item['sourceDiskID']}, 'networkAccessPolicy': 'DenyAll', 'publicNetworkAccess': 'Disabled'}})
        if not state.get('pending') and arm('GET', item['snapshotID'], '2023-10-02') is not None:
            raise ValueError('Snapshot already exists before durable intent; no adoption')
        effect(core, path, state, key, operation, lambda item=item: snapshot_receipt(core, proposal, item, arm))
    backups = {role: snapshot_receipt(core, proposal, item, arm) for role, item in proposal['snapshots'].items()}
    if not all(backups.values()): raise RuntimeError('Actual retained backup readback changed; compute remains held')
    # Bind immutable copied bytes/resource identity, rather than mutable ARM
    # response metadata/ETags that may change on a later read of the same copy.
    backup_sha = core.digest({role: {key: value for key, value in receipt.items() if key != 'providerReadbackSHA256'}
        for role, receipt in backups.items()})
    if 'fence-close' not in state['completed']:
        returned = {}
        effect(core, path, state, 'fence-close', lambda: returned.update(value=guest(core, proposal, provider, arm, 'close', backups_sha=backup_sha)),
            lambda: returned.get('value') or guest(core, proposal, provider, arm, 'close', inspect=True, backups_sha=backup_sha))
    for identifier in proposal['computeIDs']:
        key = 'deallocate:' + identifier.lower()
        if key in state['completed']: continue
        def observe(identifier=identifier):
            row = arm('GET', identifier, '2023-09-01')
            if row is None: return {'id': identifier, 'providerAbsent': True, 'deallocated': False}
            if row.get('id', '').lower() != identifier.lower() or row.get('properties', {}).get('vmId', '').lower() != proposal['immutableComputeIDs'][identifier]:
                raise ValueError('Compute was replaced; preserve the foreign resource')
            view = arm('GET', identifier + '/instanceView', '2023-09-01')
            if view and any(value.get('code') == 'PowerState/deallocated' for value in view.get('statuses', [])):
                return {'id': identifier, 'immutableID': proposal['immutableComputeIDs'][identifier], 'deallocated': True}
            return None
        def operation(identifier=identifier):
            if observe(identifier) is None: arm('POST', identifier + '/deallocate', '2023-09-01')
        effect(core, path, state, key, operation, observe)
    state.update(state='owned_compute_deallocation_observed_storage_retained', backups=backups, retainedCharges=proposal['retainedCharges'],
        connected=False, providerInvoiceVerified=False, taskReservationsSettled=True, volumesDeleted=False,
        message='Owned compute closure and actual private backup resources observed. Disks, snapshots, gallery, storage and other retained resources can still charge; final invoice is separate. Backup restoration has not been tested.')
    core.atomic_json(path, state); return state


def cancel(core, bundle, selection_path, ask=input, emit=print, provider=None, arm=None):
    guided = module(bundle, 'guided'); provider = provider or guided.az_json
    arm = arm or module(bundle, 'foundation').AzureARM(provider)
    parent = Path(selection_path).parent
    successor = parent / 'azure-successor-state.json'
    if successor.exists():
        active = core.private_json(successor).get('activeSelectionPath')
        if not active: raise ValueError('Controller recovery is pending; cancellation cannot replace its fence')
        return cancel(core, bundle, Path(active), ask, emit, provider, arm)
    original_path = parent / 'azure-retirement-state.json'
    original = core.private_json(original_path)
    proposal = plan(core, bundle, selection_path, arm, provider)
    if original.get('planDigest') != proposal['planDigest']: raise ValueError('Original retirement actor plan changed')
    effects = original.get('completed', []) + [original.get('pending', '')]
    if any(key == 'fence-close' or key.startswith('deallocate:') for key in effects):
        raise ValueError('Backup-bound closure/deallocation has begun; cancellation cannot reopen it')
    review = dict(proposal, action='cancel-retirement', originalPending=original.get('pending'),
        message='Resume the same owned generation. Held task reservations remain held. Existing/uncertain snapshots and all original receipts are retained; storage and resumed compute may charge.')
    if not guided.approve_stage(core, parent, core.bundle(bundle), core.private_json(selection_path), 'installation-retirement-cancel', review, ask, emit):
        return {'state': 'installation_retirement_cancel_review_paused', 'connected': False}
    path = parent / 'azure-retirement-cancel-state.json'
    state = core.private_json(path) if path.exists() else {'version': 1, 'sourceCommit': proposal['sourceCommit'],
        'scope': proposal['scope'], 'generation': proposal['generation'], 'planDigest': proposal['planDigest'],
        'originalRetirementSHA256': core.digest(original), 'completed': [], 'receipts': {}}
    if state.get('planDigest') != proposal['planDigest'] or state.get('originalRetirementSHA256') != core.digest(original):
        raise ValueError('Original uncertain retirement receipt changed; preserve both actor journals')
    # Activate the existing private gateway before controller readback/start.
    # Both guest programs serialize stop/resume and reject late cancelled work.
    for key, operation, observe in (
            ('host-resume', lambda: runtime(core, proposal, provider, arm, 'resume'), lambda: runtime(core, proposal, provider, arm, 'resume-inspect')),
            ('controller-resume', lambda: guest(core, proposal, provider, arm, 'cancel'), lambda: guest(core, proposal, provider, arm, 'cancel', inspect=True))):
        if key in state['completed']: continue
        returned = {}
        effect(core, path, state, key, lambda operation=operation: returned.update(value=operation()),
            lambda observe=observe: returned.get('value') or observe())
    state.update(state='retirement_cancelled_predecessor_resumed_storage_retained', taskReservationsReleased=False,
        originalPending=original.get('pending'), retainedCharges=proposal['retainedCharges'], connected=False,
        providerInvoiceVerified=False, message='Exact owned runtime/controller resume observed. Original unknown effects and held task leases remain unchanged; refresh the protected connection. This actor plan cannot silently retire again.')
    core.atomic_json(path, state); return state
