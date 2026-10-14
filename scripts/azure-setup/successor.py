"""Guided signed controller replacement; actual effects require explicit review."""
import base64
import copy
import importlib.util
import ipaddress
import json
from pathlib import Path
import re
import uuid


def module(bundle, name, directory='azure-setup'):
    spec = importlib.util.spec_from_file_location('luci_successor_' + name, Path(bundle) / (directory + '/' + name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


def current_host(core, parent, infrastructure, selected):
    """Use authenticated current delivery after recovery, never stale Terraform."""
    ids = infrastructure['resourceIDs']
    if infrastructure.get('predecessorGeneration'):
        saved = core.private_json(parent / 'azure-active-host-config.json')
        host = saved.get('hostConfig', {})
        if (saved.get('generationReceiptSHA256') != ids.get('controller_generation_receipt_sha256') or
                not re.fullmatch(r'[0-9a-f]{64}', saved.get('hostReceiptSHA256', '')) or
                host.get('generation') != selected['terraformVariables']['generation'] or
                host.get('controllerPrivateIP') != selected['terraformVariables']['controller_private_ip'] or
                core.digest(host) != ids['runtime_host_config_sha256']):
            raise ValueError('Authenticated current generation host configuration changed')
    else:
        outputs = json.loads(core.terraform(parent / 'azure-terraform', 'output', '-json'))
        host = json.loads(outputs['runtime_host_config_json']['value'])
        if core.digest(host) != ids['runtime_host_config_sha256']:
            raise ValueError('Retained original runtime configuration differs from provider state')
    result = copy.deepcopy(host)
    result.pop('controllerSuccessorLink', None)
    return result


def target(core, bundle, selected, recovery, provider, selection_parent):
    variables = core.requested(selected)
    identifier = recovery['newControllerID']
    read = lambda path: provider(['rest', '--method', 'get', '--url', 'https://management.azure.com' + path + '?api-version=2023-09-01'])
    vm = read(identifier)
    if (vm.get('id', '').lower() != identifier or vm.get('properties', {}).get('provisioningState') != 'Succeeded' or
            vm.get('properties', {}).get('storageProfile', {}).get('imageReference', {}).get('id', '').lower() != variables['controller_image_version_id'].lower()):
        raise ValueError('Replacement is not the exact selected generic controller image/VM')
    identities = {key.lower() for key in vm.get('identity', {}).get('userAssignedIdentities', {})}
    if identities not in ({recovery['recoveryIdentityID']}, {recovery['recoveryIdentityID'], recovery['controllerIdentityID']}) or vm.get('properties', {}).get('storageProfile', {}).get('dataDisks', []) and identities != {recovery['recoveryIdentityID'], recovery['controllerIdentityID']}:
        raise ValueError('Replacement standby identity/data disk is unavailable; preserve recovery prerequisites')
    interfaces = vm.get('properties', {}).get('networkProfile', {}).get('networkInterfaces', [])
    if len(interfaces) != 1: raise ValueError('Exact single replacement controller NIC is required')
    nic_id = interfaces[0].get('id', '').lower()
    prefix = identifier.split('/providers/')[0] + '/providers/microsoft.network/networkinterfaces/'
    if not nic_id.startswith(prefix): raise ValueError('Replacement NIC is outside the owned resource group')
    nic = read(nic_id)
    configs = nic.get('properties', {}).get('ipConfigurations', [])
    if nic.get('id', '').lower() != nic_id or nic.get('properties', {}).get('virtualMachine', {}).get('id', '').lower() != identifier or not nic.get('etag') or len(configs) != 1:
        raise ValueError('Replacement NIC readback is incomplete')
    config = configs[0].get('properties', {})
    subnet_id = config.get('subnet', {}).get('id', '').lower()
    vnet = core.private_json(Path(selection_parent) / 'azure-private-location.json')['vnetID'].lower()
    if subnet_id != vnet + '/subnets/' + selected['name'] + '-controller' or config.get('publicIPAddress'):
        raise ValueError('Replacement NIC is not in the exact selected private controller subnet')
    subnet = read(subnet_id)
    subnet_properties = subnet.get('properties', {})
    prefixes = subnet_properties.get('addressPrefixes') or [subnet_properties.get('addressPrefix')]
    if (subnet.get('id', '').lower() != subnet_id or prefixes != [variables['controller_subnet_cidr']] or
            subnet_properties.get('addressPrefix', variables['controller_subnet_cidr']) != variables['controller_subnet_cidr']):
        raise ValueError('Replacement controller subnet identity or address range changed')
    address = str(ipaddress.ip_address(config.get('privateIPAddress', '')))
    if ipaddress.ip_address(address) not in ipaddress.ip_network(variables['controller_subnet_cidr'], strict=True):
        raise ValueError('Replacement NIC address differs from selected private subnet')
    recovery_module = module(bundle, 'recover', 'azure-controller-disk')
    response = provider(['vm', 'run-command', 'invoke', '--ids', identifier, '--command-id', 'RunShellScript', '--scripts', recovery_module.target_identity_script()])
    rows = [line.removeprefix('LUCI_AZURE_SUCCESSOR_TARGET=') for item in response.get('value', []) for line in item.get('message', '').splitlines() if line.startswith('LUCI_AZURE_SUCCESSOR_TARGET=')]
    if len(rows) != 1 or len(rows[0].split('|')) != 2: raise RuntimeError('Authenticated replacement boot/IP is unavailable')
    boot, observed_ip = rows[0].split('|')
    if not recovery_module.UUID.fullmatch(boot) or observed_ip != address:
        raise ValueError('Replacement guest boot/IP differs from exact ARM NIC readback')
    return {'newControllerID': identifier, 'newBootID': boot, 'newPrivateIP': address,
        'newNICID': nic_id, 'newNICETag': nic['etag'], 'controllerSubnetID': subnet_id}


def prepare(core, bundle, selection_path, provider, ask=input, emit=print):
    manifest = core.bundle(bundle)
    selected = core.private_json(selection_path)
    variables = core.requested(selected)
    parent = Path(selection_path).parent
    path = parent / 'azure-successor-state.json'
    retained = core.private_json(path) if path.exists() else {}
    if retained.get('state') in ('successor_prepared', 'successor_effect_pending', 'successor_active'):
        return retained
    infrastructure = core.private_json(parent / 'azure-setup-state.json')
    if infrastructure.get('state') != 'controller_finalized_private_route_unverified' or infrastructure.get('selectionDigest') != core.digest(selected):
        raise ValueError('Recovery requires the exact installed predecessor selection')
    recovery = core.initial_recovery_selection(selected, infrastructure['resourceIDs'])
    for key in ('oldControllerID', 'newControllerID', 'diskID', 'controllerIdentityID', 'recoveryIdentityID'):
        recovery[key] = recovery[key].lower()
    recovery_module = module(bundle, 'recover', 'azure-controller-disk')
    recovery_module.validate(recovery)
    observed = target(core, bundle, selected, recovery, provider, parent)
    # Reuse the established private Blob transport, with the user's AAD token
    # instead of IMDS. No new managed-identity grant or token is persisted.
    transport = recovery_module.AzureHTTP('')
    transport.token = lambda resource: provider(['account', 'get-access-token', '--resource', resource])['accessToken']
    fence, etag = recovery_module.FenceBlob(transport, recovery['fenceBlobURL']).read()
    if (not fence or not etag or fence.get('state') != 'active' or any(fence.get(key) != recovery[value] for key, value in (
            ('ownerID', 'ownerID'), ('scope', 'scope'), ('diskID', 'diskID'), ('controllerID', 'oldControllerID'),
            ('generation', 'oldGeneration'), ('generationReceiptSHA256', 'oldGenerationReceiptSHA256')))):
        raise ValueError('Exact active predecessor fence/ETag is unavailable')
    proposal = dict(observed, sourceCommit=manifest['luciRevision'], scope=variables['scope'], ownerID=variables['controller_owner_id'],
        oldControllerID=recovery['oldControllerID'], oldGeneration=recovery['oldGeneration'], newGeneration=recovery['newGeneration'],
        diskID=recovery['diskID'], predecessorFenceETag=etag, predecessorFenceSHA256=core.digest(fence),
        oldPrivateIP=variables['controller_private_ip'])
    predecessor = module(bundle, 'standby').capture(core, bundle, selected, recovery, provider, parent)
    if retained and (retained.get('proposal') != proposal or retained.get('selectionDigest') != core.digest(selected)):
        raise ValueError('Retained successor target/fence/source changed; no automatic fallback or resigning')
    guided = module(bundle, 'guided')
    if not guided.approve_stage(core, parent, manifest, selected, 'controller-recovery', proposal, ask, emit):
        return {'state': 'successor_review_paused', 'connected': False}
    if retained.get('standbyPredecessor', predecessor) != predecessor:
        raise ValueError('Retained predecessor immutable VM/OS-disk identity changed')
    journal = dict(retained, version=1, state='successor_preparation_pending', sourceCommit=manifest['luciRevision'], standbyPredecessor=predecessor,
        scope=variables['scope'], generation=recovery['newGeneration'], selectionDigest=core.digest(selected), proposal=proposal)
    core.atomic_json(path, journal)
    credentials = module(bundle, 'credentials')
    operator = module(bundle, 'operator_setup')
    next_selection = copy.deepcopy(selected)
    next_selection['terraformVariables']['generation'] = recovery['newGeneration']
    next_selection['terraformVariables']['controller_private_ip'] = observed['newPrivateIP']
    next_selection['nextGeneration'] = 'gen-' + uuid.uuid4().hex[:12]
    credential_root = parent / ('azure-successor-credentials-' + recovery['newGeneration'])
    if not credential_root.exists():
        credentials.generate(next_selection, credential_root, core.atomic_json, predecessor=parent / 'azure-credentials')
    credential_journal = core.private_json(credential_root / 'journal.json')
    if credential_journal.get('state') == 'generation_pending':
        raise RuntimeError('Successor private credential generation interrupted; preserve without regeneration')
    if credential_journal.get('pending'):
        credentials.reconcile_pending(next_selection, credential_root, core.atomic_json, az_call=provider)
    credentials.publish(next_selection, credential_root, core.atomic_json, acknowledge=True, az_call=provider)
    next_path = parent / ('azure-successor-selection-' + recovery['newGeneration'] + '.json')
    credentials.bind_selection(next_selection, credential_root, core.atomic_json, next_path)
    recovery_path = parent / 'azure-signed-recovery-selection.json'
    if not recovery_path.exists(): core.atomic_json(recovery_path, recovery)
    prior_envelope = core.private_json(parent / 'azure-operator-finalization.json')
    signed_path = parent / ('azure-successor-' + recovery['newGeneration'] + '.json')
    if not signed_path.exists():
        operator.prepare_successor(core, bundle, selection_path, recovery_path, core.digest(prior_envelope), observed['newBootID'], credential_root)
    successor = core.private_json(signed_path)
    recovery['controllerSuccessor'] = successor
    recovery_module.validate_successor(recovery, successor)
    host = current_host(core, parent, infrastructure, selected)
    host.update(generation=recovery['newGeneration'], previousGeneration=recovery['oldGeneration'],
        controllerPrivateIP=observed['newPrivateIP'], secretRefs=next_selection['terraformVariables']['host_secret_refs'])
    link_payload = dict(proposal, version=1, hostConfigSHA256=core.digest(host))
    host['controllerSuccessorLink'] = {'payload': link_payload, 'signatureAlgorithm': 'ecdsa-sha256',
        'signature': operator.sign(core, link_payload, parent / 'azure-credentials')}
    old_operator = prior_envelope['payload']['operator']
    mount = variables['controller_mount_path']
    recovery['controllerHostDelivery'] = {'version': 1, 'controllerClientID': infrastructure['resourceIDs']['controller_identity_client_id'],
        'controllerInstanceID': recovery['newControllerID'], 'azureDataDiskID': recovery['diskID'],
        'scaleSetID': infrastructure['resourceIDs']['runtime_scale_set_id'].lower(), 'instanceID': old_operator['deployment']['azureInstanceId'],
        'controllerGenerationReceiptSHA256': recovery['oldGenerationReceiptSHA256'], 'controllerGenerationReceiptPath': mount + '/luci/generation.json',
        'journalPath': mount + '/luci/host-delivery-' + recovery['newGeneration'] + '.json', 'controllerMountPath': mount,
        'controllerFilesystemUUID': recovery['filesystemUUID'], 'hostConfig': host, 'hostConfigSHA256': core.digest(host)}
    recovery_module.validate(recovery)
    core.atomic_json(recovery_path, recovery)
    journal.update(state='successor_prepared', recoverySelectionPath=str(recovery_path), recoverySelectionSHA256=core.digest(recovery),
        credentialDirectory=str(credential_root), nextSelectionPath=str(next_path))
    core.atomic_json(path, journal)
    return journal


def run(core, bundle, selection_path, provider, ask=input, emit=print):
    journal = prepare(core, bundle, selection_path, provider, ask, emit)
    if journal['state'] in ('successor_review_paused', 'successor_active'): return journal
    parent = Path(selection_path).parent
    path = parent / 'azure-successor-state.json'
    recovery = core.private_json(journal['recoverySelectionPath'])
    if core.digest(recovery) != journal['recoverySelectionSHA256']:
        raise ValueError('Retained signed recovery selection changed')
    selected = core.private_json(selection_path)
    if core.digest(selected) != journal['selectionDigest'] or core.bundle(bundle)['luciRevision'] != journal['sourceCommit']:
        raise ValueError('Recovery source or predecessor plan changed')
    observed = target(core, bundle, selected, recovery, provider, parent)
    if any(observed[key] != journal['proposal'][key] for key in observed):
        raise ValueError('Successor ARM NIC/boot changed before protected transfer')
    inspect = journal['state'] == 'successor_effect_pending'
    encoded = base64.b64encode(core.canonical(recovery)).decode()
    script = """set -eu
python3 - <<'PY'
import base64,os,pathlib,stat,tempfile
data=base64.b64decode(%s)
target=pathlib.Path('/etc/luci/azure-controller-successor-selection.json')
if target.exists() or target.is_symlink():
    if target.is_symlink() or not stat.S_ISREG(target.stat().st_mode) or target.stat().st_mode & 0o077 or target.read_bytes()!=data:
        raise SystemExit('retained public recovery selection differs')
elif %s:
    raise SystemExit('successor transfer remains uncertain; no repeat write')
else:
    fd,name=tempfile.mkstemp(dir=target.parent);os.fchmod(fd,0o600)
    with os.fdopen(fd,'wb') as output: output.write(data);output.flush();os.fsync(output.fileno())
    os.replace(name,target)
PY
receipt=$(python3 /opt/luci/azure-controller/bin/recover.py %s /etc/luci/azure-controller-successor-selection.json)
printf 'LUCI_AZURE_RECOVERED=%%s\\n' "$receipt"
""" % (repr(encoded), repr(inspect), 'reconcile-recover' if inspect else 'recover')
    if not inspect:
        journal['state'] = 'successor_effect_pending'; core.atomic_json(path, journal)
    response = provider(['vm', 'run-command', 'invoke', '--ids', recovery['newControllerID'], '--command-id', 'RunShellScript', '--scripts', script])
    rows = [line.removeprefix('LUCI_AZURE_RECOVERED=') for item in response.get('value', []) for line in item.get('message', '').splitlines() if line.startswith('LUCI_AZURE_RECOVERED=')]
    result = json.loads(rows[0]) if len(rows) == 1 else {}
    if (any(result.get(key) != value for key, value in {'state': 'active', 'generation': recovery['newGeneration'],
            'diskID': recovery['diskID'], 'controllerID': recovery['newControllerID'], 'sourceCommit': journal['sourceCommit'],
            'hostConfigSHA256': recovery['controllerHostDelivery']['hostConfigSHA256']}.items()) or
            not all(re.fullmatch(r'[0-9a-f]{64}', result.get(key, '')) for key in ('generationReceiptSHA256', 'fenceReceiptDigest', 'hostReceiptSHA256'))):
        raise RuntimeError('Controller recovery remains uncertain; retain exact successor without repeated mutation')
    journal.update(state='successor_active', recoveryReceipt=result, connected=False, oldControllerDeallocationObserved=True,
        oldControllerIdentityRevocationObserved=True, finalInvoiceVerified=False,
        message='Exact recovered generation observed. Import a fresh protected connection; retained disks/gallery/storage and private standby replacement remain separate.')
    core.atomic_json(path, journal)
    return journal


def finish(core, bundle, selection_path, journal, provider, ask=input, emit=print):
    """Create a new active generation folder; never overwrite predecessor data."""
    if journal.get('state') != 'successor_active': return journal
    if core.bundle(bundle)['luciRevision'] != journal['sourceCommit'] or core.digest(core.private_json(selection_path)) != journal['selectionDigest']:
        raise ValueError('Recovered source or predecessor selection changed; no active generation substitution')
    parent = Path(selection_path).parent
    recovery = core.private_json(journal['recoverySelectionPath'])
    if core.digest(recovery) != journal['recoverySelectionSHA256']: raise ValueError('Retained successor changed')
    directory = parent / ('azure-generation-' + recovery['newGeneration'])
    if directory.is_symlink(): raise ValueError('Active generation path is unsafe')
    directory.mkdir(mode=0o700, exist_ok=True)
    if directory.stat().st_mode & 0o077: raise ValueError('Active generation path is not owner-private')
    def exact(path, value):
        if path.exists():
            if core.private_json(path) != value: raise ValueError('Retained active generation file changed')
        else: core.atomic_json(path, value)
    selected = core.private_json(journal['nextSelectionPath'])
    selected['terraformVariables']['controller_operator_template_sha256'] = recovery['controllerSuccessor']['newTemplateSHA256']
    active_path = directory / 'selection.json'
    exact(active_path, selected)
    files = {item['path']: item['content'] for item in recovery['controllerSuccessor']['files']}
    for name in ('operator-template.json', 'azure-operator-finalization.json'):
        exact(directory / name, json.loads(files['/etc/luci/' + name]))
    infrastructure = core.private_json(parent / 'azure-setup-state.json')
    infrastructure['resourceIDs']['controller_instance_id'] = recovery['newControllerID']
    infrastructure['resourceIDs']['controller_recovery_instance_id'] = recovery['oldControllerID']
    receipt = journal['recoveryReceipt']
    infrastructure['resourceIDs']['controller_generation_receipt_sha256'] = receipt['generationReceiptSHA256']
    infrastructure['resourceIDs']['runtime_host_config_sha256'] = receipt['hostConfigSHA256']
    exact(directory / 'azure-active-host-config.json', {'hostConfig': recovery['controllerHostDelivery']['hostConfig'],
        'generationReceiptSHA256': receipt['generationReceiptSHA256'], 'hostReceiptSHA256': receipt['hostReceiptSHA256']})
    infrastructure.update(selectionDigest=core.digest(selected), generation=recovery['newGeneration'], state='controller_finalized_private_route_unverified',
        predecessorGeneration=recovery['oldGeneration'], predecessorControllerID=recovery['oldControllerID'])
    exact(directory / 'azure-setup-state.json', infrastructure)
    exact(directory / 'azure-private-location.json', core.private_json(parent / 'azure-private-location.json'))
    exact(directory / 'azure-operator-install-state.json', {'state': 'operator_finalized_private_route_unverified',
        'sourceCommit': journal['sourceCommit'], 'scope': journal['scope'], 'generation': journal['generation'],
        'finalizationDigest': recovery['controllerSuccessor']['newEnvelopeSHA256']})
    credentials = module(bundle, 'credentials')
    source = Path(journal['credentialDirectory'])
    target = directory / 'azure-credentials'
    if source.is_symlink() or not source.is_dir() or source.stat().st_mode & 0o077 or target.is_symlink():
        raise ValueError('Active credential path is unsafe')
    target.mkdir(mode=0o700, exist_ok=True)
    if target.stat().st_mode & 0o077: raise ValueError('Active credential directory is not owner-private')
    for name in set(credentials.MATERIAL.values()) | {'ca.key', 'journal.json'}:
        data = credentials.private_file(source / name, 64 << 10)
        destination = target / name
        if destination.exists():
            if credentials.private_file(destination, 64 << 10) != data: raise ValueError('Retained active credential bytes changed')
        else: credentials.save(destination, data)
    journal['activeSelectionPath'] = str(active_path)
    core.atomic_json(parent / 'azure-successor-state.json', journal)
    descriptor = module(bundle, 'guided').connection_descriptor(core, bundle, active_path, provider=provider)
    restored = module(bundle, 'standby').restore(core, bundle, selection_path, provider, ask, emit)
    return dict(journal, connectionDescriptor=descriptor, standbyRestoration=restored, connected=False,
        nextRecoveryCommand=['python3', str(Path(bundle) / 'azure-setup/installer.py'), 'recover-controller', '--bundle', str(bundle), '--selection', str(active_path)],
        nextRecoveryPrerequisite='Exact same-VM standby restoration must finish before a later replacement; retained old/new OS disks and snapshot can charge.')
