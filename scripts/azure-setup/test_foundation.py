"""Offline private foundation composition and interrupted-effect fixtures."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest

spec = importlib.util.spec_from_file_location('foundation', Path(__file__).with_name('foundation.py'))
foundation = importlib.util.module_from_spec(spec)
spec.loader.exec_module(foundation)


class FoundationTests(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        def write(path, data):
            Path(path).write_text(json.dumps(data))
        core = SimpleNamespace(bundle=lambda _: {'luciRevision': 'a' * 40},
            private_json=lambda p: json.loads(Path(p).read_text()), atomic_json=write,
            digest=lambda v: hashlib.sha256(json.dumps(v, sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
            requested=lambda v: v['terraformVariables'])
        subscription, tenant, principal = ('12345678-1234-1234-1234-123456789ab' + c for c in 'cde')
        path = root / 'selection.json'
        write(path, {'region': 'australiaeast', 'monthlyBudgetUsd': 100, 'stateBackend': {},
            'terraformVariables': {'scope': 'luci-fixture', 'generation': 'gen-one', 'controller_owner_id': 'owner-one'}})
        vnet_id = '/subscriptions/' + subscription + '/resourceGroups/existing/providers/Microsoft.Network/virtualNetworks/private'
        vnet = {'id': vnet_id, 'location': 'australiaeast', 'etag': 'one', 'properties': {
            'provisioningState': 'Succeeded', 'addressSpace': {'addressPrefixes': ['10.80.0.0/16']},
            'subnets': [{'id': vnet_id + '/subnets/endpoints', 'name': 'endpoints', 'properties': {
                'addressPrefix': '10.80.0.0/24', 'privateEndpointNetworkPolicies': 'Disabled', 'delegations': []}}]}}
        account = {'id': subscription, 'tenantId': tenant}
        resources = {vnet_id: vnet}
        calls = []
        def arm(method, identifier, api, body=None):
            calls.append((method, identifier))
            if identifier.endswith('/providers/Microsoft.Network/privateDnsZones'):
                return {'value': []}
            if method == 'PUT':
                self.assertEqual(core.private_json(root / 'azure-foundation-state.json')['pending'], identifier)
                resources[identifier] = {'id': identifier, **copy.deepcopy(body)}
            return copy.deepcopy(resources.get(identifier))
        foundation.inventory(core, root, path, vnet_id, 'endpoints', '10.80.0.5', 22, '10.90.1.0/24', principal,
                             arm, account, probe=lambda *_: True)
        plan = foundation.plan(core, root, path, 'luci-foundation')
        return root, core, path, plan, resources, calls, arm, vnet, account, principal

    def test_free_subnets_refuse_collisions_small_or_public_space(self):
        self.assertEqual(foundation.unused_subnets(['10.80.0.0/16'], ['10.80.0.0/23']), ['10.80.2.0/24', '10.80.3.0/24'])
        for spaces, used in ((['10.80.0.0/24'], []), (['10.80.0.0/23'], ['10.80.0.0/24']), (['0.0.0.0/0'], [])):
            with self.assertRaises(ValueError):
                foundation.unused_subnets(spaces, used)

    def test_complete_private_plan_hands_state_over_only_after_data_route(self):
        root, core, path, plan, resources, calls, arm, *_ = self.fixture()
        self.assertFalse(plan['priceVerified'])
        self.assertFalse(any(method == 'PUT' for method, _ in calls))
        for resource in plan['resources']:
            foundation.step(core, root, path, plan['planDigest'], arm, acknowledge=True)
        journal = core.private_json(root / 'azure-foundation-state.json')
        self.assertEqual(journal['state'], 'foundation_observed_pending_private_data_route')
        self.assertEqual(sum(method == 'PUT' for method, _ in calls), len(plan['resources']))
        self.assertEqual(resources[plan['storageID']]['properties']['publicNetworkAccess'], 'Disabled')
        self.assertFalse(resources[plan['storageID']]['properties']['allowSharedKeyAccess'])
        self.assertEqual(resources[plan['vaultID']]['properties']['publicNetworkAccess'], 'Disabled')
        data_calls = []
        def cli(arguments):
            data_calls.append(arguments)
            return {'name': 'luci-state', 'properties': {'publicAccess': None}}
        result = foundation.bind(core, root, path, arm, cli, resolver=lambda *a, **k: [(None, None, None, None, ('10.80.0.9', 443))])
        self.assertEqual(result['state'], 'foundation_bound_private_data_route_observed')
        selection = core.private_json(path)
        self.assertEqual(selection['stateBackend']['storageAccountName'], plan['storageName'])
        self.assertEqual(selection['terraformVariables']['existing_vnet_name'], 'private')
        self.assertEqual(len(data_calls), 2)
        self.assertFalse(result['connected'])

    def test_lost_ack_is_read_only_even_when_resource_absent(self):
        root, core, path, plan, resources, calls, arm, *_ = self.fixture()
        def lost(method, *args):
            result = arm(method, *args)
            if method == 'PUT':
                raise TimeoutError('fixture lost ACK')
            return result
        with self.assertRaises(TimeoutError):
            foundation.step(core, root, path, plan['planDigest'], lost, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'uncertain'):
            foundation.step(core, root, path, plan['planDigest'], arm, acknowledge=True)
        count = len(calls)
        settled = foundation.step(core, root, path, plan['planDigest'], arm, reconcile=True)
        self.assertEqual(calls[count:], [('GET', plan['resources'][0]['id'])])
        self.assertEqual(settled['state'], 'foundation_stage_observed')
        # The next operation loses its response before resource visibility.
        def no_effect(method, *args):
            if method == 'PUT': raise TimeoutError('fixture unknown')
            return arm(method, *args)
        with self.assertRaises(TimeoutError): foundation.step(core, root, path, plan['planDigest'], no_effect, acknowledge=True)
        with self.assertRaisesRegex(RuntimeError, 'uncertain'): foundation.step(core, root, path, plan['planDigest'], arm, reconcile=True)
        self.assertTrue(core.private_json(root / 'azure-foundation-state.json')['pending'])

    def test_cost_absence_and_stale_plan_refuse_before_mutation(self):
        root, core, path, plan, resources, calls, arm, *_ = self.fixture()
        with self.assertRaisesRegex(ValueError, 'acknowledge'): foundation.step(core, root, path, plan['planDigest'], arm)
        resources[plan['resources'][0]['id']] = {'id': plan['resources'][0]['id']}
        with self.assertRaisesRegex(ValueError, 'already exists'): foundation.step(core, root, path, plan['planDigest'], arm, acknowledge=True)
        selected = core.private_json(path)
        selected['monthlyBudgetUsd'] = 200
        core.atomic_json(path, selected)
        with self.assertRaisesRegex(ValueError, 'changed'): foundation.step(core, root, path, plan['planDigest'], arm, acknowledge=True)
        self.assertFalse(any(method == 'PUT' for method, _ in calls))

    def test_public_dns_or_private_drift_blocks_remote_backend_binding(self):
        root, core, path, plan, resources, calls, arm, *_ = self.fixture()
        for _ in plan['resources']: foundation.step(core, root, path, plan['planDigest'], arm, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'DNS'):
            foundation.bind(core, root, path, arm, lambda _: self.fail('public route reached data API'), resolver=lambda *a, **k: [(None, None, None, None, ('8.8.8.8', 443))])
        self.assertEqual(core.private_json(path)['stateBackend'], {})
        resources[plan['vaultID']]['properties']['publicNetworkAccess'] = 'Enabled'
        with self.assertRaisesRegex(ValueError, 'changed'):
            foundation.bind(core, root, path, arm, lambda _: self.fail('drift reached data API'))

    def test_inventory_requires_actual_private_route_and_endpoint_subnet(self):
        root, core, path, plan, resources, calls, arm, vnet, account, principal = self.fixture()
        (root / 'azure-foundation-plan.json').unlink()
        with self.assertRaisesRegex(ValueError, 'probe'):
            foundation.inventory(core, root, path, vnet['id'], 'endpoints', '10.80.0.5', 22, '10.90.1.0/24', principal, arm, account, probe=lambda *_: False)
        with self.assertRaisesRegex(ValueError, 'nondelegated'):
            foundation.inventory(core, root, path, vnet['id'], 'missing', '10.80.0.5', 22, '10.90.1.0/24', principal, arm, account, probe=lambda *_: True)

    def test_existing_private_resource_reuse_performs_no_put_and_keeps_external_ownership(self):
        root, core, path, plan, resources, calls, arm, vnet, *_ = self.fixture()
        (root / 'azure-foundation-plan.json').unlink()
        for resource in plan['resources']:
            resources[resource['id']] = {'id': resource['id'], **copy.deepcopy(resource['body'])}
        def existing(method, identifier, api, body=None):
            self.assertEqual(method, 'GET')
            if identifier.endswith('/virtualNetworkLinks'):
                return {'value': [{'id': identifier + '/existing', 'properties': {'virtualNetwork': {'id': vnet['id']}, 'virtualNetworkLinkState': 'Completed'}}]}
            return arm(method, identifier, api, body)
        proposal = foundation.reuse(core, root, path, plan['storageID'], plan['vaultID'], plan['resourceGroup'], existing)
        self.assertTrue(proposal['externallyOwned'])
        result = foundation.bind(core, root, path, existing, lambda _: {'name': 'luci-state', 'properties': {}}, resolver=lambda *a, **k: [(None, None, None, None, ('10.80.0.9', 443))])
        self.assertTrue(result['externallyOwned'])
        self.assertFalse(any(method == 'PUT' for method, _ in calls))
        with self.assertRaisesRegex(ValueError, 'changed'):
            foundation.step(core, root, path, proposal['planDigest'], existing, acknowledge=True)

    def test_new_foundation_reuses_linked_dns_without_recreating_foreign_zone(self):
        root, core, path, old, resources, calls, arm, vnet, *_ = self.fixture()
        (root / 'azure-foundation-plan.json').unlink()
        location = core.private_json(root / 'azure-private-location.json')
        zone = vnet['id'].split('/providers/')[0] + '/providers/Microsoft.Network/privateDnsZones/privatelink.blob.core.windows.net'
        location.update(existingLinkedPrivateDNSZoneIDs=[zone], existingDNSDependencies=[{'id': zone, 'apiVersion': '2020-06-01', 'body': {'location': 'global'}, 'externallyOwned': True}])
        core.atomic_json(root / 'azure-private-location.json', location)
        proposal = foundation.plan(core, root, path, 'another-foundation')
        self.assertEqual(proposal['dnsResourceGroup'], 'existing')
        self.assertFalse(any(resource['id'] == zone for resource in proposal['resources']))
        group = next(resource for resource in proposal['resources'] if resource['kind'] == 'dnsGroup' and '/luci-blob-' in resource['id'])
        self.assertEqual(group['body']['properties']['privateDnsZoneConfigs'][0]['properties']['privateDnsZoneId'], zone)
        self.assertTrue(proposal['dependencies'][0]['externallyOwned'])


if __name__ == '__main__':
    unittest.main()
