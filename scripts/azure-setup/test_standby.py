"""Same-VM restoration sequencing, identity refusal and no unknown replay."""
import copy
import ast
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest

spec = importlib.util.spec_from_file_location('setup_standby', Path(__file__).with_name('standby.py'))
standby = importlib.util.module_from_spec(spec); spec.loader.exec_module(standby)


class StandbyTests(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory(); self.addCleanup(temporary.cleanup)
        root = Path(temporary.name); active = root / 'active'; active.mkdir()
        canonical = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        core = SimpleNamespace(bundle=lambda _: {'luciRevision': 'a' * 40}, canonical=canonical,
            digest=lambda value: hashlib.sha256(canonical(value)).hexdigest(),
            atomic_json=lambda path, value: Path(path).write_text(json.dumps(value)),
            private_json=lambda path: json.loads(Path(path).read_text()))
        prefix = '/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/owned/providers/'
        old, current = (prefix + 'microsoft.compute/virtualmachines/' + name for name in ('old', 'current'))
        os_disk, new_disk, data_disk = (prefix + 'microsoft.compute/disks/' + name for name in ('old-os', 'new-os', 'authoritative'))
        nic = prefix + 'microsoft.network/networkinterfaces/old'
        subnet = prefix + 'microsoft.network/virtualnetworks/private/subnets/example-controller'
        image = prefix + 'microsoft.compute/galleries/source/images/controller/versions/1.0.0'
        identity = prefix + 'microsoft.managedidentity/userassignedidentities/recovery'
        previous_boot, new_boot, vm_id, nic_guid = ('12345678-1234-1234-1234-123456789ab' + value for value in 'cdef')
        pinned = {'id': old, 'vmID': vm_id, 'bootID': previous_boot, 'nicID': nic, 'nicGUID': nic_guid, 'subnetID': subnet,
            'privateIP': '10.2.0.8', 'imageVersionID': image, 'osDiskID': os_disk, 'osDiskUniqueID': 'old-unique',
            'sourceCommit': 'a' * 40, 'generation': 'gen-old'}
        recovery = {'oldControllerID': old, 'newControllerID': current, 'oldGeneration': 'gen-old', 'newGeneration': 'gen-current',
            'diskID': data_disk, 'scope': 'scope-one', 'ownerID': 'owner-one', 'recoveryIdentityID': identity,
            'controllerSuccessor': {'sourceCommit': 'a' * 40}}
        receipt = {'generationReceiptSHA256': 'b' * 64}
        selected = {'name': 'example', 'region': 'australiaeast', 'terraformVariables': {'scope': 'scope-one', 'generation': 'gen-current'},
            'controllerPackage': {'sha256': 'c' * 64}}
        core.atomic_json(active / 'selection.json', selected)
        recovery_path = root / 'recovery.json'; core.atomic_json(recovery_path, recovery)
        journal = {'state': 'successor_active', 'sourceCommit': 'a' * 40, 'standbyPredecessor': pinned,
            'activeSelectionPath': str(active / 'selection.json'), 'recoverySelectionPath': str(recovery_path),
            'recoverySelectionSHA256': core.digest(recovery), 'recoveryReceipt': receipt}
        core.atomic_json(root / 'azure-successor-state.json', journal)
        fence = {'state': 'active', 'controllerID': current, 'diskID': data_disk, 'generation': 'gen-current', 'scope': 'scope-one',
            'ownerID': 'owner-one', 'generationReceiptSHA256': 'b' * 64, 'successorDigest': core.digest(recovery['controllerSuccessor'])}
        store = SimpleNamespace(read=lambda: (copy.deepcopy(fence), 'fence-etag'))
        resources = {
            old: {'id': old, 'etag': 'vm-etag', 'identity': {'type': 'None'}, 'properties': {'vmId': vm_id, 'provisioningState': 'Succeeded',
                'storageProfile': {'imageReference': {'id': image}, 'dataDisks': [], 'osDisk': {'managedDisk': {'id': os_disk}, 'deleteOption': 'Delete'}},
                'networkProfile': {'networkInterfaces': [{'id': nic}]}}},
            nic: {'id': nic, 'properties': {'resourceGuid': nic_guid, 'virtualMachine': {'id': old},
                'ipConfigurations': [{'properties': {'privateIPAddress': '10.2.0.8', 'subnet': {'id': subnet}}}]}},
            os_disk: {'id': os_disk, 'etag': 'disk-etag', 'managedBy': old, 'properties': {'uniqueId': 'old-unique'}},
            data_disk: {'id': data_disk, 'managedBy': current, 'properties': {'maxShares': 1}}}
        calls = []; flags = {'power': 'deallocated', 'boot': new_boot, 'lost_reimage': False, 'lost_assign': False, 'wrong_lineage': False}
        def arm(method, identifier, api, body=None, headers=None):
            calls.append((method, identifier))
            if method == 'GET' and identifier.endswith('/instanceView'):
                return {'statuses': [{'code': 'PowerState/' + flags['power']}]}
            if method == 'GET': return copy.deepcopy(resources.get(identifier))
            if method == 'PUT':
                row = copy.deepcopy(body); row['id'] = identifier
                row['properties'].update(provisioningState='Succeeded', uniqueId='snapshot-unique')
                row['properties']['creationData']['sourceUniqueId'] = 'old-unique'
                resources[identifier] = row; return row
            if method == 'PATCH':
                if '/disks/' in identifier:
                    self.assertEqual(headers, {'If-Match': 'disk-etag'})
                    resources[identifier]['properties'].update(body['properties']); return {}
                self.assertEqual(headers, {'If-Match': 'vm-etag'})
                if 'identity' in body:
                    resources[old]['identity'] = body['identity']
                    if flags['lost_assign']: flags['lost_assign'] = False; raise TimeoutError('lost assignment ACK')
                else: resources[old]['properties']['storageProfile']['osDisk'] = body['properties']['storageProfile']['osDisk']
                return {}
            if method == 'POST' and identifier.endswith('/reimage'):
                self.assertEqual(resources[old]['identity']['type'], 'None')
                self.assertEqual(resources[old]['properties']['storageProfile']['osDisk']['deleteOption'], 'Detach')
                self.assertEqual(body['exactVersion'], '1.0.0')
                resources[os_disk].pop('managedBy')
                resources[new_disk] = {'id': new_disk, 'etag': 'disk-etag', 'managedBy': old, 'properties': {'uniqueId': 'new-unique', 'provisioningState': 'Succeeded',
                    'creationData': {'createOption': 'FromImage', 'galleryImageReference': {'id': image + ('-foreign' if flags['wrong_lineage'] else '')}}}}
                resources[old]['properties']['storageProfile']['osDisk']['managedDisk']['id'] = new_disk
                if flags['lost_reimage']: flags['lost_reimage'] = False; raise TimeoutError('lost reimage ACK')
                return {}
            if method == 'POST' and identifier.endswith('/start'): flags['power'] = 'running'; return {}
            self.fail('unexpected provider mutation ' + method + identifier)
        def provider(args):
            calls.append(('guest', args[-1]))
            self.assertIn('manifest', args[-1]); self.assertIn('/proc/sys/kernel/random/boot_id', args[-1])
            value = {'vmID': vm_id, 'bootID': flags['boot'], 'sourceCommit': 'a' * 40, 'packageSHA256': 'c' * 64}
            return {'value': [{'message': 'LUCI_AZURE_CLEAN_STANDBY=' + json.dumps(value)}]}
        return root, core, selected, journal, recovery, pinned, resources, calls, flags, arm, provider, store, fence

    def execute(self, fixture, ask=None):
        root, core = fixture[:2]
        approve = lambda prompt: prompt.split('type ', 1)[1].split(' (anything', 1)[0]
        return standby.restore(core, Path(__file__).parent.parent, root / 'selection.json', fixture[10], ask or approve, lambda _: None, fixture[9], fixture[11])

    def test_backup_detach_reimage_clean_boot_then_existing_identity_and_retained_costs(self):
        fixture = self.fixture(); result = self.execute(fixture); calls = fixture[7]
        backup = next(i for i, (method, _) in enumerate(calls) if method == 'PUT')
        reimage = next(i for i, (_, identifier) in enumerate(calls) if identifier.endswith('/reimage'))
        patches = [i for i, (method, _) in enumerate(calls) if method == 'PATCH']
        clean = next(i for i, (method, _) in enumerate(calls) if method == 'guest')
        self.assertLess(backup, patches[0]); self.assertLess(patches[0], reimage); self.assertLess(reimage, clean); self.assertLess(clean, patches[-1])
        self.assertEqual(result['state'], 'fresh_private_standby_observed')
        self.assertTrue(result['predecessorOSDiskRetained']); self.assertFalse(result['providerInvoiceVerified'])
        self.assertIn('new managed OS disk observed from reimage', result['retainedCharges'])
        self.assertNotIn('managedBy', fixture[6][fixture[5]['osDiskID']])
        self.execute(fixture)
        self.assertEqual(sum(identifier.endswith('/reimage') for method, identifier in calls if method == 'POST'), 1)
        self.assertEqual(sum(method == 'PATCH' for method, _ in calls), 4)

    def test_cost_refusal_has_no_mutations(self):
        fixture = self.fixture(); self.assertEqual(self.execute(fixture, lambda _: 'no')['state'], 'standby_restoration_review_paused')
        self.assertTrue(all(method == 'GET' for method, _ in fixture[7]))

    def test_lost_reimage_ack_reads_new_disk_and_never_reimages_twice(self):
        fixture = self.fixture(); fixture[8]['lost_reimage'] = True
        with self.assertRaises(TimeoutError): self.execute(fixture)
        self.assertEqual(self.execute(fixture)['state'], 'fresh_private_standby_observed')
        self.assertEqual(sum(identifier.endswith('/reimage') for method, identifier in fixture[7] if method == 'POST'), 1)

    def test_wrong_source_lineage_and_old_boot_never_assign_identity(self):
        for wrong in ('wrong_lineage', 'boot'):
            fixture = self.fixture()
            fixture[8][wrong] = True if wrong == 'wrong_lineage' else fixture[5]['bootID']
            with self.assertRaises((ValueError, RuntimeError)): self.execute(fixture)
            self.assertEqual(fixture[6][fixture[5]['id']]['identity']['type'], 'None')

    def test_lost_identity_assignment_ack_only_reads_assignment_on_restart(self):
        fixture = self.fixture(); fixture[8]['lost_assign'] = True
        with self.assertRaises(TimeoutError): self.execute(fixture)
        self.assertEqual(self.execute(fixture)['state'], 'fresh_private_standby_observed')
        self.assertEqual(sum(method == 'PATCH' for method, _ in fixture[7]), 4)

    def test_foreign_or_active_predecessor_and_changed_fence_never_mutate(self):
        for wrong in ('power', 'identity', 'vmID', 'fence', 'nic'):
            fixture = self.fixture(); pinned, resources = fixture[5:7]
            if wrong == 'power': fixture[8]['power'] = 'running'
            if wrong == 'identity': resources[pinned['id']]['identity'] = {'type': 'UserAssigned', 'userAssignedIdentities': {'foreign': {}}}
            if wrong == 'vmID': resources[pinned['id']]['properties']['vmId'] = 'foreign'
            if wrong == 'fence': fixture[12]['generation'] = 'stale'
            if wrong == 'nic': resources[pinned['nicID']]['properties']['ipConfigurations'][0]['properties']['publicIPAddress'] = {'id': 'public'}
            with self.subTest(wrong=wrong), self.assertRaises(ValueError): self.execute(fixture)
            self.assertTrue(all(method == 'GET' for method, _ in fixture[7]))

    def test_capture_binds_actual_predecessor_descriptor_disk_and_private_nic_before_effect(self):
        fixture = self.fixture(); root, core, selected, journal, recovery, pinned = fixture[:6]
        selected['terraformVariables'].update(controller_image_version_id=pinned['imageVersionID'], controller_private_ip=pinned['privateIP'])
        core.atomic_json(root / 'azure-connection.json', {'sourceCommit': 'a' * 40, 'controllerResourceID': pinned['id'],
            'generation': 'gen-old', 'vmID': pinned['vmID'], 'bootID': pinned['bootID']})
        core.atomic_json(root / 'azure-private-location.json', {'vnetID': pinned['subnetID'].split('/subnets/')[0]})
        def provider(args):
            identifier, api = args[-1].split('management.azure.com', 1)[1].split('?api-version=')
            return fixture[9]('GET', identifier, api)
        self.assertEqual(standby.capture(core, Path(__file__).parent.parent, selected, recovery, provider, root), pinned)
        descriptor = core.private_json(root / 'azure-connection.json'); descriptor['vmID'] = 'ffffffff-ffff-ffff-ffff-ffffffffffff'
        core.atomic_json(root / 'azure-connection.json', descriptor)
        with self.assertRaisesRegex(ValueError, 'predecessor VM/boot'):
            standby.capture(core, Path(__file__).parent.parent, selected, recovery, provider, root)
        self.assertTrue(all(method == 'GET' for method, _ in fixture[7]))

    def test_clean_guest_probe_has_bounded_python_and_no_mutating_guest_commands(self):
        fixture = self.fixture()
        script = standby.clean_script(fixture[5], 'a' * 40, 'c' * 64)
        ast.parse(script.split("python3 - <<'PY'\n", 1)[1].rsplit('\nPY', 1)[0])
        self.assertIn('stream.read(1<<20)', script); self.assertIn('response.read(65537)', script)
        self.assertNotIn('systemctl start', script); self.assertNotIn('get-access-token', script)


if __name__ == '__main__': unittest.main()
