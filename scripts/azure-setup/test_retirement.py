"""Offline user-AAD retirement order and unknown/refusal regressions."""
import copy
import hashlib
import importlib.util
import json
import re
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('setup_retirement', Path(__file__).with_name('retirement.py'))
retirement = importlib.util.module_from_spec(spec); spec.loader.exec_module(retirement)


class RetirementTests(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory(); self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        def write(path, value): Path(path).write_text(json.dumps(value))
        canonical = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        core = SimpleNamespace(bundle=lambda _: {'luciRevision': 'a' * 40}, canonical=canonical,
            digest=lambda value: hashlib.sha256(canonical(value)).hexdigest(), atomic_json=write,
            private_json=lambda path: json.loads(Path(path).read_text()), requested=lambda s: s['terraformVariables'])
        selection = root / 'selection.json'
        write(selection, {'terraformVariables': {'scope': 'scope-one', 'generation': 'gen-one'}})
        prefix = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/microsoft.compute/'
        controller, standby, runtime = (prefix + value for value in ('virtualmachines/controller', 'virtualmachines/standby', 'virtualmachinescalesets/runtime/virtualMachines/0'))
        proposal = {'version': 1, 'planDigest': 'b' * 64, 'sourceCommit': 'a' * 40, 'scope': 'scope-one', 'ownerID': 'owner-one',
            'generation': 'gen-one', 'region': 'australiaeast', 'operationID': 'owned-operation', 'controllerID': controller, 'recoveryID': standby,
            'recoveryClientID': '12345678-1234-1234-1234-123456789abc', 'diskID': prefix + 'disks/controller',
            'filesystemUUID': '12345678-1234-1234-1234-123456789abc', 'fenceBlobURL': 'https://owned.blob.core.windows.net/controller-fence/controller-fence.json',
            'toolSelection': {}, 'runtimeID': runtime, 'computeIDs': [runtime, controller, standby],
            'immutableComputeIDs': {value: '12345678-1234-1234-1234-123456789abc' for value in (runtime, controller, standby)},
            'snapshots': {}, 'retainedCharges': ['disks', 'private snapshots', 'gallery', 'storage and Key Vault']}
        resources, calls, powers = {}, [], set()
        for role, owner in (('controller-state', controller), ('runtime-templates', runtime)):
            source = prefix + 'disks/' + role
            proposal['snapshots'][role] = {'sourceDiskID': source, 'sourceUniqueID': role + '-immutable', 'sourceOwnerID': owner,
                'snapshotID': prefix + 'snapshots/' + role}
            resources[source] = {'id': source, 'managedBy': owner, 'properties': {'uniqueId': role + '-immutable'}}
        for identifier in proposal['computeIDs']:
            resources[identifier] = {'id': identifier, 'properties': {'vmId': proposal['immutableComputeIDs'][identifier]}}
        def arm(method, identifier, api, body=None):
            calls.append((method, identifier))
            if method == 'GET' and identifier.endswith('/instanceView'):
                return {'statuses': [{'code': 'PowerState/deallocated' if identifier.removesuffix('/instanceView') in powers else 'PowerState/running'}]}
            if method == 'GET': return copy.deepcopy(resources.get(identifier))
            if method == 'PUT':
                item = next(value for value in proposal['snapshots'].values() if value['snapshotID'] == identifier)
                row = copy.deepcopy(body); row['id'] = identifier
                row['properties'].update(provisioningState='Succeeded', uniqueId='snapshot-' + item['sourceUniqueID'])
                row['properties']['creationData']['sourceUniqueId'] = item['sourceUniqueID']
                resources[identifier] = row
                return row
            if method == 'POST': powers.add(identifier.removesuffix('/deallocate')); return {}
            self.fail('unexpected provider operation')
        def provider(args):
            script = args[-1]
            calls.append(('guest', script))
            identity = {'sourceCommit': proposal['sourceCommit'], 'scope': proposal['scope'], 'generation': proposal['generation'], 'planDigest': proposal['planDigest']}
            if args[0] == 'vmss':
                value = dict(identity, state='retirement_stopped', runtimeStorageOnOSDiskVerified=True)
                marker = 'LUCI_AZURE_RUNTIME_RETIRED='
            elif 'retire.py close' in script:
                backup_sha = re.search(r'--backups-sha256 ([0-9a-f]{64})', script)[1]
                value = dict(identity, state='retirement_closed_pending_compute', backupsSHA256=backup_sha)
                marker = 'LUCI_AZURE_INSTALLATION_RETIREMENT='
            else:
                value = dict(identity, state='retirement_settled', taskReservationsSettled=True, settledJournalDigests={'lease-ledger': 'c' * 64})
                marker = 'LUCI_AZURE_INSTALLATION_RETIREMENT='
            return {'value': [{'message': marker + json.dumps(value)}]}
        return root, core, selection, proposal, resources, calls, arm, provider

    def execute(self, fixture, ask=None, arm=None):
        root, core, selection, proposal, _, _, original_arm, provider = fixture
        approve = lambda prompt: prompt.split('type ', 1)[1].split(' (anything', 1)[0]
        with patch.object(retirement, 'plan', return_value=proposal):
            return retirement.run(core, Path(__file__).parent.parent, selection, ask or approve, lambda _: None, provider, arm or original_arm)

    def test_actual_backup_readback_precedes_every_deallocate_and_retains_charges(self):
        fixture = self.fixture(); result = self.execute(fixture)
        calls = fixture[5]
        backups = [i for i, value in enumerate(calls) if value[0] == 'PUT']
        deallocations = [i for i, value in enumerate(calls) if value[0] == 'POST']
        self.assertEqual(len(backups), 2); self.assertEqual(len(deallocations), 3)
        self.assertLess(max(backups), min(deallocations))
        self.assertEqual(result['state'], 'owned_compute_deallocation_observed_storage_retained')
        self.assertFalse(result['providerInvoiceVerified']); self.assertFalse(result['volumesDeleted'])
        self.assertFalse(any(value['restorationTested'] for value in result['backups'].values()))

    def test_refused_cost_review_has_zero_mutations(self):
        fixture = self.fixture(); result = self.execute(fixture, ask=lambda _: 'no')
        self.assertEqual(result['state'], 'installation_retirement_review_paused')
        self.assertEqual(fixture[5], [])

    def test_normal_generated_plan_binds_real_resource_inventory_without_mutation(self):
        root, core, path, sample, resources, calls, original, provider = self.fixture()
        selected = core.private_json(path)
        image = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/microsoft.compute/galleries/source/images/runtime/versions/1.0.0'
        selected.update(name='example', region='australiaeast')
        variables = selected['terraformVariables']
        variables.update(subscription_id='12345678-1234-1234-1234-123456789abc', resource_group_name='owned',
            runtime_image_version_id=image, controller_owner_id='owner-one', controller_filesystem_uuid=sample['filesystemUUID'],
            controller_mount_path='/mnt/luci-controller', controller_operator_template_path='/etc/luci/operator-template.json', controller_operator_template_sha256='d' * 64)
        core.atomic_json(path, selected)
        scale = sample['runtimeID'].split('/virtualMachines/')[0]
        identity = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/microsoft.managedidentity/userassignedidentities/recovery'
        resources[sample['recoveryID']]['identity'] = {'userAssignedIdentities': {identity: {}}}
        runtime_disk = sample['snapshots']['runtime-templates']['sourceDiskID']
        resources[sample['runtimeID']]['instanceId'] = '0'
        resources[sample['runtimeID']]['properties']['storageProfile'] = {'osDisk': {'managedDisk': {'id': runtime_disk}}}
        for item in sample['snapshots'].values(): resources[item['sourceDiskID']]['properties']['diskState'] = 'Attached'
        core.atomic_json(root / 'operator-template.json', {'template': {'cpuMillis': 2000, 'memoryMiB': 2048}, 'templateFromImage': '127.0.0.1:5055/luci/guest@sha256:' + 'e' * 64})
        core.atomic_json(root / 'azure-setup-state.json', {'state': 'controller_finalized_private_route_unverified', 'selectionDigest': core.digest(selected),
            'resourceIDs': {'controller_instance_id': sample['controllerID'], 'controller_recovery_instance_id': sample['recoveryID'],
                'runtime_scale_set_id': scale, 'controller_disk_id': sample['snapshots']['controller-state']['sourceDiskID'],
                'controller_recovery_identity_resource_id': identity, 'controller_recovery_identity_client_id': sample['recoveryClientID'],
                'controller_fence_blob_url': sample['fenceBlobURL'], 'controller_identity_client_id': sample['recoveryClientID']}})
        def arm(method, identifier, api, body=None):
            if identifier == scale:
                calls.append((method, identifier)); return {'id': scale, 'properties': {'virtualMachineProfile': {'storageProfile': {'imageReference': {'id': image}}}}}
            if identifier == scale + '/virtualMachines':
                calls.append((method, identifier)); return {'value': [copy.deepcopy(resources[sample['runtimeID']])]}
            return original(method, identifier, api, body)
        result = retirement.plan(core, Path(__file__).parent.parent, path, arm, provider)
        self.assertEqual(result['recoveryID'], sample['recoveryID'])
        self.assertEqual(len(result['snapshots']), 2)
        self.assertEqual(result['toolSelection']['memoryMiB'], 2048)
        self.assertTrue(all(method == 'GET' for method, _ in calls))
        self.assertEqual(core.digest({key: value for key, value in result.items() if key != 'planDigest'}), result['planDigest'])

    def test_lost_snapshot_ack_restart_reads_without_repeating_put(self):
        fixture = self.fixture(); original = fixture[6]; lost = [False]
        def arm(method, identifier, api, body=None):
            result = original(method, identifier, api, body)
            if method == 'PUT' and not lost[0]: lost[0] = True; raise TimeoutError('lost snapshot ACK')
            return result
        with self.assertRaises(TimeoutError): self.execute(fixture, arm=arm)
        self.assertFalse(any(value[0] == 'POST' for value in fixture[5]))
        result = self.execute(fixture, arm=arm)
        self.assertEqual(sum(value[0] == 'PUT' for value in fixture[5]), 2)
        self.assertEqual(result['state'], 'owned_compute_deallocation_observed_storage_retained')

    def test_foreign_backup_source_stays_unknown_and_never_deallocates(self):
        fixture = self.fixture(); original = fixture[6]
        def arm(method, identifier, api, body=None):
            row = original(method, identifier, api, body)
            if method == 'GET' and '/snapshots/' in identifier and row:
                row['properties']['creationData']['sourceUniqueId'] = 'foreign-disk-incarnation'
            return row
        with self.assertRaisesRegex(RuntimeError, 'remains uncertain'): self.execute(fixture, arm=arm)
        self.assertFalse(any(value[0] == 'POST' for value in fixture[5]))

    def test_lost_deallocation_ack_is_read_only_on_restart(self):
        fixture = self.fixture(); original = fixture[6]; lost = [False]
        def arm(method, identifier, api, body=None):
            result = original(method, identifier, api, body)
            if method == 'POST' and not lost[0]: lost[0] = True; raise TimeoutError('lost deallocation ACK')
            return result
        with self.assertRaises(TimeoutError): self.execute(fixture, arm=arm)
        self.execute(fixture, arm=arm)
        self.assertEqual(sum(value[0] == 'POST' for value in fixture[5]), 3)


if __name__ == '__main__': unittest.main()
