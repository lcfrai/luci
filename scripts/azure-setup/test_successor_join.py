"""Exact ARM NIC/guest target joins and stale predecessor refusal, offline only."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest


def load(name):
    spec = importlib.util.spec_from_file_location('join_' + name, Path(__file__).with_name(name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result); return result


successor, installer = load('successor'), load('installer')


class SuccessorJoinTests(unittest.TestCase):
    def fixture(self):
        prefix = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/'
        controller = prefix + 'microsoft.compute/virtualmachines/standby'
        nic = prefix + 'microsoft.network/networkinterfaces/standby'
        vnet = prefix + 'microsoft.network/virtualnetworks/private'
        subnet = vnet + '/subnets/example-controller'
        image = prefix + 'microsoft.compute/galleries/source/images/controller/versions/1.0.0'
        identity = prefix + 'microsoft.managedidentity/userassignedidentities/recovery'
        boot = '12345678-1234-1234-1234-123456789abc'
        selected = {'name': 'example', 'terraformVariables': {'controller_image_version_id': image, 'controller_subnet_cidr': '10.2.0.0/24'}}
        recovery = {'newControllerID': controller, 'recoveryIdentityID': identity, 'controllerIdentityID': identity + '-controller'}
        rows = {
            controller: {'id': controller, 'identity': {'userAssignedIdentities': {identity: {}}}, 'properties': {'provisioningState': 'Succeeded',
                'storageProfile': {'imageReference': {'id': image}, 'dataDisks': []}, 'networkProfile': {'networkInterfaces': [{'id': nic}]}}},
            nic: {'id': nic, 'etag': 'exact-nic-etag', 'properties': {'virtualMachine': {'id': controller}, 'ipConfigurations': [{'properties': {'subnet': {'id': subnet}, 'privateIPAddress': '10.2.0.9'}}]}},
            subnet: {'id': subnet, 'properties': {'addressPrefix': '10.2.0.0/24'}}}
        calls = []
        def provider(args):
            calls.append(args)
            if args[0] == 'rest':
                identifier = args[-1].split('management.azure.com', 1)[1].split('?', 1)[0]
                return copy.deepcopy(rows[identifier])
            return {'value': [{'message': 'LUCI_AZURE_SUCCESSOR_TARGET=' + boot + '|10.2.0.9'}]}
        core = SimpleNamespace(requested=lambda s: s['terraformVariables'], private_json=lambda _: {'vnetID': vnet})
        return core, selected, recovery, rows, provider, calls

    def test_target_binds_exact_nic_subnet_ip_and_guest_boot_without_new_roles(self):
        core, selected, recovery, _, provider, calls = self.fixture()
        result = successor.target(core, Path(__file__).parent.parent, selected, recovery, provider, Path('private'))
        self.assertEqual(result['newNICETag'], 'exact-nic-etag')
        self.assertEqual(result['newPrivateIP'], '10.2.0.9')
        self.assertEqual(len(calls), 4)
        self.assertTrue(all(args[0] == 'rest' and args[2] == 'get' or args[:3] == ['vm', 'run-command', 'invoke'] for args in calls))
        self.assertIn('/proc/sys/kernel/random/boot_id', calls[-1][-1])

    def test_public_nic_foreign_subnet_changed_image_and_identity_refuse_before_guest(self):
        for change in ('public', 'subnet', 'image', 'identity', 'etag', 'nic-owner'):
            core, selected, recovery, rows, provider, calls = self.fixture()
            vm = rows[recovery['newControllerID']]
            nic = rows[vm['properties']['networkProfile']['networkInterfaces'][0]['id']]
            if change == 'public': nic['properties']['ipConfigurations'][0]['properties']['publicIPAddress'] = {'id': 'public'}
            if change == 'subnet': nic['properties']['ipConfigurations'][0]['properties']['subnet']['id'] += '-foreign'
            if change == 'image': vm['properties']['storageProfile']['imageReference']['id'] += '-foreign'
            if change == 'identity': vm['identity']['userAssignedIdentities'] = {'foreign': {}}
            if change == 'etag': nic.pop('etag')
            if change == 'nic-owner': nic['properties']['virtualMachine']['id'] += '-foreign'
            with self.subTest(change=change), self.assertRaises(ValueError):
                successor.target(core, Path(__file__).parent.parent, selected, recovery, provider, Path('private'))
            self.assertFalse(any(args[0] == 'vm' for args in calls))

    def test_guest_ip_substitution_refuses_authenticated_target(self):
        core, selected, recovery, _, provider, _ = self.fixture()
        def substituted(args):
            result = provider(args)
            if args[0] == 'vm': result['value'][0]['message'] = result['value'][0]['message'].replace('10.2.0.9', '10.2.0.10')
            return result
        with self.assertRaisesRegex(ValueError, 'differs from exact ARM NIC'):
            successor.target(core, Path(__file__).parent.parent, selected, recovery, substituted, Path('private'))

    def test_retained_recovery_blocks_stale_terraform_before_any_source_or_provider_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'azure-successor-state.json').write_text(json.dumps({'state': 'successor_effect_pending'}))
            with self.assertRaisesRegex(ValueError, 'no stale apply'):
                installer.selected_workspace(root / 'absent-bundle', root / 'selection.json', {})
            self.assertFalse((root / 'azure-terraform').exists())

    def test_repeated_recovery_uses_authenticated_current_host_not_original_terraform(self):
        digest = lambda value: hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()
        host = {'generation': 'gen-current', 'controllerPrivateIP': '10.2.0.8',
            'secretRefs': {'retained-task': 'same'}, 'controllerSuccessorLink': {'prior': 'signature'}}
        saved = {'hostConfig': host, 'generationReceiptSHA256': 'c' * 64, 'hostReceiptSHA256': 'd' * 64}
        ids = {'controller_generation_receipt_sha256': 'c' * 64, 'runtime_host_config_sha256': digest(host)}
        core = SimpleNamespace(private_json=lambda _: saved, digest=digest,
            terraform=lambda *args: self.fail('stale predecessor Terraform was read'))
        selected = {'terraformVariables': {'generation': 'gen-current', 'controller_private_ip': '10.2.0.8'}}
        result = successor.current_host(core, Path('private'), {'predecessorGeneration': 'gen-old', 'resourceIDs': ids}, selected)
        self.assertNotIn('controllerSuccessorLink', result)
        self.assertIn('controllerSuccessorLink', host)
        self.assertEqual(result['secretRefs'], host['secretRefs'])
        saved['generationReceiptSHA256'] = 'e' * 64
        with self.assertRaisesRegex(ValueError, 'current generation'): successor.current_host(core, Path('private'), {'predecessorGeneration': 'gen-old', 'resourceIDs': ids}, selected)

    def test_recovery_selection_uses_actual_current_mounted_receipt(self):
        selected = {'terraformVariables': {'generation': 'gen-current', 'controller_filesystem_uuid': 'uuid',
            'controller_owner_id': 'owner', 'scope': 'scope'}, 'nextGeneration': 'gen-next', 'controllerHealthURL': 'loopback'}
        ids = {key: key for key in ('controller_recovery_identity_client_id', 'controller_instance_id', 'controller_recovery_instance_id',
            'controller_disk_id', 'controller_fence_blob_url', 'controller_identity_resource_id', 'controller_recovery_identity_resource_id')}
        ids['controller_generation_receipt_sha256'] = 'f' * 64
        self.assertEqual(installer.initial_recovery_selection(selected, ids)['oldGenerationReceiptSHA256'], 'f' * 64)


if __name__ == '__main__': unittest.main()
