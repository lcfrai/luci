"""Offline producer stages: real local bytes, mocked Azure effects only."""
import copy
from datetime import datetime, timezone, timedelta
import hashlib
import importlib.util
import io
import json
import shutil
import subprocess
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
import tarfile

HERE = Path(__file__).parent
def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    value = importlib.util.module_from_spec(spec); spec.loader.exec_module(value); return value
builder, guest = module('builder'), module('builder_guest')


class BuilderTests(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory(); self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        def write(path, value): Path(path).write_text(json.dumps(value))
        canonical = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        core = SimpleNamespace(bundle=lambda _: {'luciRevision': 'a' * 40}, canonical=canonical,
            digest=lambda value: hashlib.sha256(canonical(value)).hexdigest(), atomic_json=write,
            private_json=lambda path: json.loads(Path(path).read_text()), requested=lambda value: value['terraformVariables'])
        subscription = '12345678-1234-1234-1234-123456789abc'
        selection = {'name': 'luci-fixture', 'region': 'australiaeast', 'monthlyBudgetUsd': 100,
            'terraformVariables': {'scope': 'scope-one', 'generation': 'gen-one', 'controller_owner_id': 'owner-one', 'resource_group_name': 'owned',
                'controller_admin_ssh_public_key': 'ssh-ed25519 ' + 'A' * 48},
            'stateBackend': {'subscriptionId': subscription, 'resourceGroupName': 'state', 'storageAccountName': 'lucistate'}}
        path = root / 'selection.json'; write(path, selection)
        payload = root / 'source.tar'; payload.write_bytes(b'fixture actual source payload')
        write(root / 'azure-builder-source.json', {'state': 'source_archive_prepared', 'sourceCommit': 'a' * 40, 'payloadPath': str(payload), 'payloadSHA256': builder.file_sha(payload), 'size': payload.stat().st_size})
        write(root / 'azure-private-location.json', {'endpointSubnetID': '/subscriptions/' + subscription + '/resourceGroups/existing/providers/Microsoft.Network/virtualNetworks/private/subnets/endpoints'})
        write(root / 'azure-foundation-state.json', {'state': 'foundation_bound_private_data_route_observed'})
        proposal = builder.plan(core, root, path, {'publisher': 'Canonical', 'offer': 'ubuntu-24_04-lts', 'sku': 'server', 'version': '24.04.202610010'}, 'Standard_D4s_v5', 'luciGallery', '1.0.1', 7200)
        resources, calls = {}, []
        storage_id = proposal['containerID'].split('/blobServices/')[0]
        resources[storage_id] = {'properties': {'publicNetworkAccess': 'Disabled', 'allowSharedKeyAccess': False}}
        def arm(method, identifier, api, body=None, headers=None):
            calls.append((method, identifier))
            if '/artifacttypes/vmimage/' in identifier:
                return {'properties': {'osDiskImage': {'operatingSystem': 'Linux'}, 'hyperVGeneration': 'V2', 'architecture': 'x64'}}
            if method == 'PUT':
                self.assertTrue(core.private_json(root / 'azure-builder-state.json').get('pending'))
                resources[identifier] = {'id': identifier, **copy.deepcopy(body)}
                if '/virtualMachines/' in identifier:
                    role = next(role for role, row in proposal['roles'].items() if row['vmID'] == identifier)
                    resources[identifier]['identity']['principalId'] = '87654321-1234-1234-1234-123456789abc'
                    resources[identifier]['properties']['storageProfile']['osDisk']['managedDisk']['id'] = proposal['roles'][role]['diskID']
                    resources[proposal['roles'][role]['diskID']] = {'id': proposal['roles'][role]['diskID'], 'etag': 'one', 'managedBy': identifier, 'properties': {'diskState': 'Attached', 'uniqueId': '55555555-5555-5555-5555-555555555555'}}
            elif method == 'PATCH':
                patched = copy.deepcopy(body)
                resources[identifier].setdefault('properties', {}).update(patched.pop('properties', {}))
                resources[identifier].update(patched)
            elif method == 'POST':
                vm = identifier.rsplit('/', 1)[0]
                code = 'PowerState/deallocated' if identifier.endswith('/deallocate') else 'OSState/generalized'
                resources.setdefault(vm + '/instanceView', {}).setdefault('statuses', []).append({'code': code})
            elif method == 'DELETE':
                resources.pop(identifier, None)
                for row in resources.values():
                    if row.get('managedBy') == identifier:
                        row['managedBy'] = ''; row['properties']['diskState'] = 'Unattached'
            return copy.deepcopy(resources.get(identifier))
        blob = {}
        def cli(arguments):
            if arguments[:3] == ['storage', 'blob', 'exists']: return {'exists': bool(blob)}
            if arguments[:3] == ['storage', 'blob', 'upload']:
                self.assertEqual(core.private_json(root / 'azure-builder-state.json')['pending'], 'source:upload')
                blob.update(metadata={'luci_operation': proposal['operationID'], 'sha256': proposal['payloadSHA256']}, properties={'contentLength': proposal['payloadSize'], 'etag': 'one'})
            if arguments[:3] == ['storage', 'blob', 'show']: return copy.deepcopy(blob)
            if arguments[:3] == ['storage', 'blob', 'download']: Path(arguments[arguments.index('--file') + 1]).write_bytes(payload.read_bytes())
            if arguments[:3] == ['storage', 'blob', 'delete']: blob.clear()
            if arguments[:3] == ['storage', 'blob', 'list']: return []
            if arguments[:3] == ['vm', 'run-command', 'invoke']:
                vm = arguments[arguments.index('--ids') + 1]
                role = next(role for role, row in proposal['roles'].items() if row['vmID'] == vm)
                script = arguments[-1]
                state = 'role_deprovisioned_capture_pending' if 'builder.py deprovision' in script else 'role_prepared_guest_unverified'
                value = {'version': 1, 'state': state, 'role': role, 'sourceCommit': 'a' * 40, 'runtimeRevision': builder.RUNTIME,
                    'payloadSHA256': proposal['payloadSHA256'], 'packageSHA256': ('b' if role == 'runtime' else 'c') * 64}
                return {'value': [{'message': 'LUCI_AZURE_BUILDER_RECEIPT=' + json.dumps(value)}]}
            return {}
        # Script bytes are source-bound by the real bundle validator in production.
        (root / 'azure-setup').mkdir(); (root / 'azure-setup/builder_guest.py').write_text('fixture source')
        return root, core, path, proposal, resources, calls, arm, cli

    def test_complete_sequential_private_roles_capture_and_owned_cleanup(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        for _ in range(2): builder.staging(*common, arm, cli, acknowledge=True)
        for role in ('runtime', 'controller'):
            for _ in range(3): builder.allocate(*common, role, arm, acknowledge=True)
            builder.grant(*common, role, arm, acknowledge=True)
            builder.install(*common, role, arm, cli, acknowledge=True)
            builder.install(*common, role, arm, cli, acknowledge=True, deprovision=True)
            builder.deallocate(*common, role, arm, acknowledge=True)
            builder.generalize(*common, role, arm, acknowledge=True)
            for _ in range(3 if role == 'runtime' else 2): builder.capture(*common, role, arm, acknowledge=True)
            for _ in range(5): builder.retire(*common, role, arm, acknowledge=True)
            self.assertNotIn(proposal['roles'][role]['vmID'], resources)
            self.assertIn(proposal['roles'][role]['imageVersionID'], resources)
        result = builder.bind_images(*common)
        self.assertFalse(result['connected'])
        self.assertTrue(builder.observed_packages(core, path, {'luciRevision': 'a' * 40}))
        selected = core.private_json(path)
        original = copy.deepcopy(selected)
        selected['terraformVariables']['controller_secret_ids'] = ['later exact private credential reference']
        core.atomic_json(path, selected)
        self.assertTrue(builder.observed_packages(core, path, {'luciRevision': 'a' * 40}))
        for key in ('generation', 'runtime_image_version_id'):
            selected = copy.deepcopy(original); selected['terraformVariables'][key] += '-substituted'
            core.atomic_json(path, selected)
            self.assertFalse(builder.observed_packages(core, path, {'luciRevision': 'a' * 40}))
        core.atomic_json(path, original)
        for _ in range(3): builder.retire_staging(*common, arm, cli, acknowledge=True)
        self.assertNotIn(proposal['containerID'], resources)
        for role in ('runtime', 'controller'): self.assertIn(proposal['roles'][role]['imageVersionID'], resources)

    def test_private_single_active_and_cost_refusals_precede_mutations(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        with self.assertRaisesRegex(ValueError, 'max one active'): builder.allocate(*common, 'controller', arm, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'acknowledgment'): builder.allocate(*common, 'runtime', arm)
        self.assertFalse(any(method != 'GET' for method, _ in calls))
        resources[proposal['containerID'].split('/blobServices/')[0]]['properties']['publicNetworkAccess'] = 'Enabled'
        with self.assertRaisesRegex(ValueError, 'private AAD'): builder.staging(*common, arm, cli, acknowledge=True)

    def test_lost_create_ack_reconciles_exact_id_without_put_replay(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        def lost(method, *args, **kwargs):
            value = arm(method, *args, **kwargs)
            if method == 'PUT': raise TimeoutError('fixture lost response')
            return value
        with self.assertRaises(TimeoutError): builder.allocate(*common, 'runtime', lost, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'uncertain'): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        before = len(calls)
        builder.allocate(*common, 'runtime', arm, reconcile=True)
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))
        self.assertEqual(sum(method == 'PUT' for method, _ in calls), 1)

    def test_blob_actual_bytes_and_reader_scope_are_required(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        builder.staging(*common, arm, cli, acknowledge=True)
        def drift(arguments):
            result = cli(arguments)
            if arguments[:3] == ['storage', 'blob', 'download']: Path(arguments[arguments.index('--file') + 1]).write_bytes(b'altered remote bytes')
            return result
        with self.assertRaisesRegex(RuntimeError, 'uncertain'): builder.staging(*common, arm, drift, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'uncertain'): builder.staging(*common, arm, cli, acknowledge=True)
        builder.staging(*common, arm, cli, reconcile=True)
        for _ in range(3): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        builder.grant(*common, 'runtime', arm, acknowledge=True)
        grant = resources[proposal['roles']['runtime']['grantID']]['properties']
        self.assertTrue(grant['roleDefinitionId'].endswith('/' + builder.READER))
        self.assertTrue(proposal['roles']['runtime']['grantID'].startswith(proposal['containerID'] + '/providers/'))
        self.assertEqual(grant['principalId'], resources[proposal['roles']['runtime']['vmID']]['identity']['principalId'])

    def test_source_archive_traversal_is_refused_before_extraction(self):
        root, *_ = self.fixture()
        archive = root / 'unsafe.tar'
        with tarfile.open(archive, 'w') as sink:
            row = tarfile.TarInfo('../escape'); row.size = 1; sink.addfile(row, io.BytesIO(b'x'))
        with self.assertRaisesRegex(ValueError, 'unsafe'): guest.extract(archive, root / 'extracted')
        self.assertFalse((root.parent / 'escape').exists())

    def test_unknown_install_and_generalization_never_repeat_mutations(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        for _ in range(2): builder.staging(*common, arm, cli, acknowledge=True)
        for _ in range(3): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        builder.grant(*common, 'runtime', arm, acknowledge=True)
        invoked = []
        def lost(arguments): invoked.append(arguments); raise TimeoutError('fixture lost install response')
        with self.assertRaises(TimeoutError): builder.install(*common, 'runtime', arm, lost, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'uncertain'): builder.install(*common, 'runtime', arm, lost, acknowledge=True)
        self.assertEqual(len(invoked), 1)
        def inspect(arguments):
            self.assertIn('builder.py inspect', arguments[-1])
            self.assertIn('elif True:', arguments[-1])
            return cli(arguments)
        builder.install(*common, 'runtime', arm, inspect, reconcile=True)
        builder.install(*common, 'runtime', arm, cli, acknowledge=True, deprovision=True)
        builder.deallocate(*common, 'runtime', arm, acknowledge=True)
        def lost_post(method, *args, **kwargs):
            result = arm(method, *args, **kwargs)
            if method == 'POST': raise TimeoutError('fixture lost generalization')
            return result
        with self.assertRaises(TimeoutError): builder.generalize(*common, 'runtime', lost_post, acknowledge=True)
        before = len(calls)
        builder.generalize(*common, 'runtime', arm, reconcile=True)
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))

    def test_changed_identity_and_foreign_grant_refuse_ownership_cleanup(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        for _ in range(2): builder.staging(*common, arm, cli, acknowledge=True)
        for _ in range(3): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        builder.grant(*common, 'runtime', arm, acknowledge=True)
        entry = proposal['roles']['runtime']
        resources[entry['vmID']]['identity']['principalId'] = '87654321-1234-1234-1234-123456789abd'
        before = len(calls)
        with self.assertRaisesRegex(ValueError, 'principal changed'): builder.grant(*common, 'runtime', arm, acknowledge=True)
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))
        journal = core.private_json(root / 'azure-builder-state.json')
        journal['completed'].append('runtime:capture'); core.atomic_json(root / 'azure-builder-state.json', journal)
        resources[entry['grantID']]['properties']['roleDefinitionId'] = '/subscriptions/foreign/providers/Microsoft.Authorization/roleDefinitions/' + builder.READER
        with self.assertRaisesRegex(ValueError, 'principal/role/scope'): builder.retire(*common, 'runtime', arm, acknowledge=True)
        self.assertIn(entry['grantID'], resources)


    def test_abort_incomplete_vm_deallocates_and_cleans_exact_untagged_disk(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        for _ in range(2): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        entry = proposal['roles']['runtime']
        self.assertNotIn('tags', resources[entry['diskID']])
        result = builder.abort(*common, 'runtime', arm, acknowledge=True)
        self.assertIn('runtime:abort-deallocate', result['completed'])
        with self.assertRaisesRegex(ValueError, 'abort is retained'):
            builder.allocate(*common, 'runtime', arm, acknowledge=True)
        for _ in range(4): result = builder.abort(*common, 'runtime', arm, acknowledge=True)
        self.assertIn('runtime:aborted', result['completed'])
        for identifier in (entry['vmID'], entry['diskID'], entry['nicID']): self.assertNotIn(identifier, resources)
        builder.abort(*common, 'controller', arm, acknowledge=True)
        result = builder.retire_staging(*common, arm, cli, acknowledge=True)
        self.assertEqual(result['state'], 'builder_aborted_staging_retired_gallery_retained')
        self.assertFalse(result['finalInvoiceVerified'])
        self.assertFalse(builder.observed_packages(core, path, {'luciRevision': 'a' * 40}))

    def test_abort_unknown_create_preserves_intent_and_only_exact_reconcile_may_continue(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        def lost(method, *arguments, **kwargs):
            value = arm(method, *arguments, **kwargs)
            if method == 'PUT': raise TimeoutError('lost owned NIC acknowledgment')
            return value
        with self.assertRaises(TimeoutError): builder.allocate(*common, 'runtime', lost, acknowledge=True)
        before = len(calls)
        blocked = builder.abort(*common, 'runtime', arm, acknowledge=True)
        self.assertEqual(blocked['blockedEffect'], 'runtime:nic')
        self.assertEqual(core.private_json(root / 'azure-builder-state.json')['pending'], 'runtime:nic')
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))
        builder.allocate(*common, 'runtime', arm, reconcile=True)
        before = len(calls)
        def lost_delete(method, *arguments, **kwargs):
            value = arm(method, *arguments, **kwargs)
            if method == 'DELETE': raise TimeoutError('lost owned delete acknowledgment')
            return value
        with self.assertRaises(TimeoutError): builder.abort(*common, 'runtime', lost_delete, acknowledge=True)
        self.assertEqual(sum(method == 'DELETE' for method, _ in calls[before:]), 1)
        before = len(calls)
        builder.abort(*common, 'runtime', arm, reconcile=True)
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))
        builder.abort(*common, 'runtime', arm, acknowledge=True)
        with self.assertRaisesRegex(ValueError, 'abort is retained'):
            builder.allocate(*common, 'runtime', arm, acknowledge=True)

    def test_abort_refuses_replaced_disk_and_preserves_published_gallery(self):
        root, core, path, proposal, resources, calls, arm, cli = self.fixture()
        common = (core, root, path, proposal['planDigest'])
        for _ in range(3): builder.allocate(*common, 'runtime', arm, acknowledge=True)
        entry = proposal['roles']['runtime']
        resources[entry['imageVersionID']] = {'id': entry['imageVersionID'], 'tags': entry['tags']}
        builder.abort(*common, 'runtime', arm, acknowledge=True)
        builder.abort(*common, 'runtime', arm, acknowledge=True)
        resources[entry['diskID']]['properties']['uniqueId'] = '66666666-6666-6666-6666-666666666666'
        before = len(calls)
        with self.assertRaisesRegex(ValueError, 'replaced'):
            builder.abort(*common, 'runtime', arm, acknowledge=True)
        self.assertTrue(all(method == 'GET' for method, _ in calls[before:]))
        self.assertIn(entry['diskID'], resources)
        self.assertIn(entry['imageVersionID'], resources)

    @unittest.skipUnless(shutil.which('git'), 'Git is a setup prerequisite')
    def test_real_source_bytes_restore_exact_clean_git_revision_without_history(self):
        root, core, path, proposal, *_ = self.fixture()
        (root / 'azure-builder-source.json').unlink()
        trees = []
        for name in ('luci', 'upstream'):
            source = root / name; source.mkdir()
            subprocess.run(['git', 'init', '-q', str(source)], check=True, stdout=subprocess.DEVNULL)
            (source / 'LICENSE').write_text('fixture MIT licence\n')
            (source / 'NOTICE').write_text('fixture attribution\n')
            subprocess.run(['git', '-C', str(source), 'add', '-A'], check=True, stdout=subprocess.DEVNULL)
            subprocess.run(['git', '-C', str(source), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture'], check=True)
            revision = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
            trees.append((source, revision))
        core.bundle = lambda _: {'luciRevision': trees[0][1]}
        core.local_command = lambda arguments: subprocess.check_output(arguments, text=True).strip()
        envoy = root / 'envoy'; envoy.write_bytes(b'fixture binary not executed')
        licence = root / 'envoy-license'; licence.write_text('fixture original licence')
        with mock.patch.object(builder, 'RUNTIME', trees[1][1]):
            result = builder.source_payload(core, root, path, str(trees[0][0]), str(trees[1][0]), str(envoy), str(licence))
        unpacked = root / 'payload'; guest.extract(Path(result['payloadPath']), unpacked)
        restored = root / 'restored'
        guest.checkout(unpacked / 'luci.tar', (unpacked / 'luci.commit').read_bytes(), trees[0][1], restored)
        self.assertEqual(subprocess.check_output(['git', '-C', str(restored), 'rev-parse', 'HEAD'], text=True).strip(), trees[0][1])
        self.assertEqual(subprocess.check_output(['git', '-C', str(restored), 'status', '--porcelain'], text=True).strip(), '')
        self.assertEqual((restored / 'NOTICE').read_bytes(), (trees[0][0] / 'NOTICE').read_bytes())


if __name__ == '__main__': unittest.main()
