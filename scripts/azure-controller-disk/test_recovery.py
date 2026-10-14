import base64
import importlib.util
import json
from pathlib import Path
import re
import stat
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


def load(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


mount = load("mount")
recover = load("recover")
SUB = "a" * 8 + "-" + "a" * 4 + "-" + "a" * 4 + "-" + "a" * 4 + "-" + "a" * 12
ROOT = "/subscriptions/" + SUB + "/resourcegroups/luci/providers/microsoft.compute/"
OLD = ROOT + "virtualmachines/old"
NEW = ROOT + "virtualmachines/new"
DISK = ROOT + "disks/authoritative"
CONTROLLER_IDENTITY = ROOT.replace("microsoft.compute/", "microsoft.managedidentity/") + "userassignedidentities/controller"
RECOVERY_IDENTITY = ROOT.replace("microsoft.compute/", "microsoft.managedidentity/") + "userassignedidentities/recovery"
UUID = "12345678-1234-1234-1234-123456789abc"


class DiskMountTest(unittest.TestCase):
    def test_successor_inspection_requires_exact_signed_operator_and_retained_journal(self):
        disk = {'version': 1, 'azureDataDiskId': DISK, 'filesystemUuid': UUID, 'ownerId': 'owner-one',
                'scope': 'scope-one', 'controllerHostId': 'controller-one', 'runtimeHostId': 'runtime-one'}
        generation = {'version': 1, 'generation': 'new-generation', 'controllerInstanceId': NEW,
                      'azureDataDiskId': DISK, 'previousGeneration': 'old-generation',
                      'previousFenceReceiptDigest': 'f' * 64}
        disk_data, generation_data = mount.canonical(disk), mount.canonical(generation)
        template = b'{"setupDeferred":{"sourceCommit":"' + b'a' * 40 + b'"}}'
        value = {'mode': 'restore', 'device': '/dev/disk/azure/scsi1/lun0', 'mountPath': '/mnt/luci-controller',
                 'filesystemUuid': UUID, 'azureDataDiskId': DISK, 'ownerId': 'owner-one', 'scope': 'scope-one',
                 'controllerHostId': 'controller-one', 'runtimeHostId': 'runtime-one',
                 'controllerInstanceId': NEW, 'generation': 'new-generation',
                 'previousGeneration': 'old-generation', 'previousFenceReceiptDigest': 'f' * 64,
                 'diskReceiptPath': '/mnt/luci-controller/luci/disk.json',
                 'generationReceiptPath': '/mnt/luci-controller/luci/generation.json',
                 'operatorOutputPath': '/etc/luci/operator.json', 'operatorTemplatePath': '/etc/luci/operator-template.json',
                 'operatorTemplateSHA256': mount.sha(template),
                 'fenceBlobURL': 'https://lucifence.blob.core.windows.net/controller-fence/controller-fence.json',
                 'previousFinalizationSHA256': 'c' * 64, 'finalizationSHA256': mount.sha(b'{}'),
                 'targetBootID': UUID}
        signed = {'store': {}, 'deployment': {'azureGatewayHost': 'private.example'}}
        journal_path = Path('/mnt/luci-controller/luci/template/create.json')
        expected_journal = {'reservation': 'retained'}
        output = mount.canonical(mount.bound_operator(signed, value, Path(value['diskReceiptPath']),
                                                     Path(value['generationReceiptPath']), disk_data, generation_data))
        records = {value['diskReceiptPath']: disk_data, value['generationReceiptPath']: generation_data,
                   value['operatorOutputPath']: output, value['operatorTemplatePath']: template,
                   '/etc/luci/azure-operator-finalization.json': b'{}', str(journal_path): mount.canonical(expected_journal),
                   '/mnt/luci-controller/luci/operator-finalization-receipt.json': mount.canonical({
                       'version': 1, 'envelopeSHA256': value['finalizationSHA256'], 'generation': 'new-generation',
                       'controllerInstanceId': NEW, 'previousEnvelopeSHA256': 'c' * 64})}

        def blkid(arguments):
            return (0, 'ext4' if arguments[5] == 'TYPE' else UUID)

        with patch.object(mount, 'selected'), patch.object(mount.os.path, 'ismount', return_value=True), \
                patch.object(mount.Path, 'exists', return_value=True), \
                patch.object(mount.Path, 'stat', return_value=SimpleNamespace(st_mode=stat.S_IFBLK, st_rdev=99)), \
                patch.object(mount.os, 'stat', return_value=SimpleNamespace(st_dev=99, st_rdev=99, st_mode=stat.S_IFBLK)), \
                patch.object(mount, 'call', side_effect=blkid), \
                patch.object(mount, 'boot_identity', return_value=UUID), \
                patch.object(mount, 'private', side_effect=lambda path, *args: records[str(path)]), \
                patch.object(mount, 'finalized_operator', return_value=(signed, [(journal_path, expected_journal)])):
            self.assertEqual(mount.inspect(value)['state'], 'mounted')
            records[value['operatorOutputPath']] = output.replace(b'private.example', b'foreign.example')
            with self.assertRaisesRegex(ValueError, 'signed source'):
                mount.inspect(value)
            records[value['operatorOutputPath']] = output
            records[str(journal_path)] = b'{}'
            with self.assertRaisesRegex(ValueError, 'task template journal'):
                mount.inspect(value)

    def test_signed_successor_preserves_durable_artifacts_and_replaces_only_fenced_receipts(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            volume = base / 'volume'
            (volume / 'luci' / 'projects').mkdir(parents=True)
            artifact = volume / 'luci' / 'projects' / 'retained-task.json'
            artifact.write_bytes(b'{"reservation":"retained"}')
            artifact.chmod(0o600)
            disk = {'version': 1, 'azureDataDiskId': DISK, 'filesystemUuid': UUID, 'ownerId': 'owner-one',
                    'scope': 'scope-one', 'controllerHostId': 'logical-controller', 'runtimeHostId': 'runtime-host'}
            old = {'version': 1, 'generation': 'old-generation', 'controllerInstanceId': OLD, 'azureDataDiskId': DISK}
            disk_path, gen_path = volume / 'disk.json', volume / 'generation.json'
            for path, value in ((disk_path, disk), (gen_path, old)):
                path.write_bytes(mount.canonical(value)); path.chmod(0o600)
            prior_path = volume / 'luci/operator-finalization-receipt.json'
            prior = {'version': 1, 'envelopeSHA256': 'c' * 64, 'generation': 'old-generation',
                     'controllerInstanceId': OLD}
            prior_path.write_bytes(mount.canonical(prior)); prior_path.chmod(0o600)
            template = base / 'template.json'
            template.write_bytes(b'{}'); template.chmod(0o600)
            envelope = b'{"signed":"fixture"}'
            value = {'version': 1, 'mode': 'restore', 'device': str(mount.DEVICE), 'mountPath': str(volume),
                     'filesystemUuid': UUID, 'azureDataDiskId': DISK, 'ownerId': 'owner-one', 'scope': 'scope-one',
                     'controllerHostId': 'logical-controller', 'runtimeHostId': 'runtime-host',
                     'controllerInstanceId': NEW, 'generation': 'new-generation',
                     'previousGeneration': 'old-generation', 'previousGenerationReceiptSHA256': mount.sha(mount.canonical(old)),
                     'previousFenceReceiptDigest': 'f' * 64, 'diskReceiptPath': str(disk_path),
                     'generationReceiptPath': str(gen_path), 'operatorTemplatePath': str(template),
                     'operatorTemplateSHA256': mount.sha(template.read_bytes()),
                     'operatorOutputPath': str(base / 'operator.json'), 'fenceBlobURL': 'https://fence.example',
                     'previousControllerInstanceID': OLD, 'previousFinalizationSHA256': 'c' * 64,
                     'previousBootID': UUID, 'targetBootID': 'aaaaaaaa-1234-1234-1234-123456789abc',
                     'finalizationSHA256': mount.sha(envelope)}
            original_private, original_atomic = mount.private, mount.atomic

            def staged_private(path, limit=16384):
                return envelope if str(path) == '/etc/luci/azure-operator-finalization.json' else original_private(path, limit)

            def staged_atomic(path, data):
                if str(path) == '/etc/luci/azure-controller-disk-active.json':
                    return original_atomic(base / 'active.json', data)
                return original_atomic(path, data)

            selected_operator = {'store': {}, 'deployment': {'azureGatewayHost': 'private.example'}}
            with patch.object(mount, 'selected'), patch.object(mount, 'mount_exact') as exact, \
                    patch.object(mount, 'private', side_effect=staged_private), \
                    patch.object(mount, 'atomic', side_effect=staged_atomic), \
                    patch.object(mount, 'finalized_operator', return_value=(selected_operator, [])):
                prior_path.write_bytes(mount.canonical(dict(prior, envelopeSHA256='0' * 64)))
                with self.assertRaisesRegex(ValueError, 'predecessor/operator receipt'):
                    mount.bind(value)
                prior_path.write_bytes(mount.canonical(prior))
                receipt = mount.bind(value)
                self.assertEqual(receipt['generation'], 'new-generation')
            self.assertEqual(artifact.read_bytes(), b'{"reservation":"retained"}')
            self.assertEqual(json.loads(prior_path.read_bytes())['previousEnvelopeSHA256'], 'c' * 64)
            self.assertEqual(json.loads((base / 'operator.json').read_bytes())['deployment']['restoreReceiptDigest'], 'f' * 64)
            self.assertTrue(all(call.args[0]['mode'] == 'restore' for call in exact.call_args_list))

    def test_restore_binds_receipts_and_operator_once_without_format(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            volume = base / "volume"
            volume.mkdir()
            disk = {"version": 1, "azureDataDiskId": DISK, "filesystemUuid": UUID, "ownerId": "owner-one",
                    "scope": "scope-one", "controllerHostId": "logical-controller", "runtimeHostId": "runtime-host"}
            disk_path = volume / "disk.json"
            disk_path.write_bytes(mount.canonical(disk))
            disk_path.chmod(0o600)
            old = {"version": 1, "generation": "old-generation", "controllerInstanceId": OLD, "azureDataDiskId": DISK}
            gen_path = volume / "generation.json"
            gen_path.write_bytes(mount.canonical(old))
            gen_path.chmod(0o600)
            template = base / "template.json"
            template.write_bytes(mount.canonical({"store": {}, "deployment": {"azureGatewayHost": "embed.private.example"}}))
            template.chmod(0o600)
            value = {"version": 1, "mode": "restore", "device": str(mount.DEVICE), "mountPath": str(volume),
                     "filesystemUuid": UUID, "azureDataDiskId": DISK, "ownerId": "owner-one", "scope": "scope-one",
                     "controllerHostId": "logical-controller", "runtimeHostId": "runtime-host", "controllerInstanceId": NEW,
                     "generation": "new-generation", "previousGeneration": "old-generation",
                     "previousGenerationReceiptSHA256": mount.sha(mount.canonical(old)),
                     "previousFenceReceiptDigest": "f" * 64, "diskReceiptPath": str(disk_path),
                     "generationReceiptPath": str(gen_path), "operatorTemplatePath": str(template),
                     "operatorTemplateSHA256": mount.sha(template.read_bytes()), "operatorOutputPath": str(base / "operator.json"),
                     "fenceBlobURL": "https://lucifence.blob.core.windows.net/controller-fence/controller-fence.json"}
            active = base / "azure-controller-disk-active.json"
            original_atomic = mount.atomic

            def fixture_atomic(path, data):
                if path == Path('/etc/luci/azure-controller-disk-active.json'):
                    return original_atomic(active, data)
                return original_atomic(path, data)

            with patch.object(mount, "selected"), patch.object(mount, "mount_exact") as exact, \
                    patch.object(mount, "atomic", side_effect=fixture_atomic):
                receipt = mount.bind(value)
                self.assertEqual(receipt["generation"], "new-generation")
                self.assertEqual(mount.bind(value), receipt)
                exact.assert_called()
            self.assertEqual(json.loads(active.read_bytes())["generation"], "new-generation")
            self.assertEqual(json.loads(gen_path.read_bytes())["previousFenceReceiptDigest"], "f" * 64)
            self.assertEqual(json.loads((base / "operator.json").read_bytes())["azureMount"]["generationReceiptDigest"], receipt["generationReceiptSHA256"])
            value["previousFenceReceiptDigest"] = "e" * 64
            with patch.object(mount, "selected"), patch.object(mount, "mount_exact"), \
                    patch.object(mount, "atomic", side_effect=fixture_atomic):
                with self.assertRaisesRegex(ValueError, "predecessor generation"):
                    mount.bind(value)


class FakeStore:
    def __init__(self, record=None):
        self.record = record
        self.revision = 0

    def read(self):
        return self.record, str(self.revision) if self.record is not None else None

    def write(self, record, etag):
        if etag != (str(self.revision) if self.record is not None else None):
            raise RuntimeError("fence CAS collision")
        self.record = record
        self.revision += 1
        return str(self.revision)


class FakeCompute:
    def __init__(self):
        self.owner = OLD
        self.powered_off = False
        self.rows = {OLD: [{"lun": 0, "managedDisk": {"id": DISK}}], NEW: []}
        self.deallocations = 0
        self.patches = 0
        self.identity_patches = 0
        self.lost_identity = False
        self.identities = {OLD: {CONTROLLER_IDENTITY}, NEW: {RECOVERY_IDENTITY}}
        self.lost_deallocate = False
        self.mount_commands = 0
        self.lost_mount = False
        self.stage_writes = 0
        self.lost_stage = False
        self.staged = False

    def get(self, resource):
        return {"id": resource, "properties": {"provisioningState": "Succeeded", "storageProfile": {"dataDisks": self.rows[resource]}}}, "etag"

    def disk_owner(self, _):
        return self.owner, "Attached" if self.owner else "Unattached"

    def power(self, _):
        return self.powered_off

    def vm_disks(self, vm):
        return self.rows[vm], "etag"

    def vm_identities(self, vm):
        return self.identities[vm], "etag"

    def patch_identities(self, vm, identities, _):
        self.identity_patches += 1
        self.identities[vm] = set(identities)
        if self.lost_identity:
            raise TimeoutError("lost identity ACK")

    def deallocate(self, _):
        self.deallocations += 1
        if self.lost_deallocate:
            raise TimeoutError("lost deallocate ACK")
        self.powered_off = True

    def patch_disks(self, vm, rows, _):
        self.patches += 1
        self.rows[vm] = rows
        self.owner = vm if rows else ""

    def run_command(self, vm, script):
        if 'LUCI_AZURE_SUCCESSOR_STAGED=' in script:
            if 'elif True:' not in script:
                self.stage_writes += 1
                self.staged = True
                if self.lost_stage:
                    raise TimeoutError('lost successor staging ACK')
            elif not self.staged:
                raise RuntimeError('replacement public files absent')
            return 'LUCI_AZURE_SUCCESSOR_STAGED=' + recover.digest(self.successor)
        self.mount_commands += 1
        if self.lost_mount and "--inspect" not in script:
            raise TimeoutError("lost initial mount ACK")
        receipt = {"state": "mounted", "generation": "old-generation", "generationReceiptSHA256": "a" * 64,
                   "diskReceiptSHA256": "b" * 64}
        return "LUCI_AZURE_CONTROLLER_MOUNT_RECEIPT=" + json.dumps(receipt)


class Service:
    def __init__(self):
        self.starts = 0

    def start(self):
        self.starts += 1

    def ready(self):
        return self.starts == 1


class DiskRecoveryTest(unittest.TestCase):
    def setUp(self):
        self.selection = {"version": 1, "recoveryClientID": UUID, "oldControllerID": OLD,
                          "newControllerID": NEW, "diskID": DISK,
                          "controllerIdentityID": CONTROLLER_IDENTITY, "recoveryIdentityID": RECOVERY_IDENTITY,
                          "fenceBlobURL": "https://lucifence.blob.core.windows.net/controller-fence/controller-fence.json",
                          "oldGeneration": "old-generation", "newGeneration": "new-generation", "filesystemUUID": UUID,
                          "oldGenerationReceiptSHA256": "a" * 64, "mountSelectionPath": "/etc/luci/recovery-mount.json",
                          "hostDeliverySelectionPath": "/etc/luci/host-delivery.json", "apiService": "luci-api.service",
                          "apiHealthURL": "http://127.0.0.1:19730/v1/healthz", "ownerID": "owner-one", "scope": "scope-one"}
        self.store = FakeStore({"version": 1, "state": "active", "diskID": DISK, "filesystemUUID": UUID,
                                "controllerID": OLD, "generation": "old-generation", "generationReceiptSHA256": "a" * 64,
                                "ownerID": "owner-one", "scope": "scope-one"})
        self.compute = FakeCompute()
        self.service = Service()

    def signed_successor(self):
        draft = {'setupDeferred': {'sourceCommit': 'a' * 40, 'generation': 'new-generation'}}
        payload = {'sourceCommit': 'a' * 40, 'scope': 'scope-one', 'generation': 'new-generation',
                   'controllerInstanceId': NEW, 'controllerBootID': 'aaaaaaaa-1234-1234-1234-123456789abc',
                   'diskID': DISK, 'templateSHA256': recover.digest(draft),
                   'previousFinalizationSHA256': 'c' * 64, 'previousControllerInstanceID': OLD,
                   'previousGeneration': 'old-generation', 'previousBootID': UUID}
        envelope = {'payload': payload, 'signatureAlgorithm': 'ecdsa-sha256', 'signature': 'fixture-only'}
        contents = {'/etc/luci/operator-template.json': recover.canonical(draft).decode(),
                    '/etc/luci/azure-private-ca.crt': 'public-ca-fixture',
                    '/etc/luci/azure-controller.env': 'LUCI_ADDR=127.0.0.1:8730\nLUCI_BACKING=azure\nLUCI_AZURE_OPERATOR_TEMPLATE=/etc/luci/operator.json\n',
                    '/etc/luci/azure-operator-finalization.json': recover.canonical(envelope).decode()}
        successor = {'version': 1, 'sourceCommit': 'a' * 40, 'scope': 'scope-one', 'ownerID': 'owner-one',
                     'diskID': DISK, 'oldControllerID': OLD, 'newControllerID': NEW,
                     'oldGeneration': 'old-generation', 'newGeneration': 'new-generation',
                     'oldBootID': UUID, 'newBootID': payload['controllerBootID'],
                     'oldGenerationReceiptSHA256': 'a' * 64, 'priorEnvelopeSHA256': 'c' * 64,
                     'newEnvelopeSHA256': recover.digest(envelope), 'newTemplateSHA256': recover.digest(draft),
                     'newPublicCASHA256': recover.hashlib.sha256(b'public-ca-fixture').hexdigest(),
                     'files': [{'path': path, 'content': content,
                                'sha256': recover.hashlib.sha256(content.encode()).hexdigest()}
                               for path, content in contents.items()]}
        successor['mountTemplate'] = {
            'version': 1, 'mode': 'restore', 'device': '/dev/disk/azure/scsi1/lun0',
            'mountPath': '/mnt/luci-controller', 'filesystemUuid': UUID, 'azureDataDiskId': DISK,
            'ownerId': 'owner-one', 'scope': 'scope-one', 'controllerHostId': 'controller-one',
            'runtimeHostId': 'runtime-one', 'controllerInstanceId': NEW, 'generation': 'new-generation',
            'diskReceiptPath': '/mnt/luci-controller/luci/disk-receipt.json',
            'generationReceiptPath': '/mnt/luci-controller/luci/generation.json',
            'operatorTemplatePath': '/etc/luci/operator-template.json',
            'operatorTemplateSHA256': successor['newTemplateSHA256'],
            'operatorOutputPath': '/etc/luci/operator.json', 'fenceBlobURL': self.selection['fenceBlobURL'],
            'previousGeneration': 'old-generation', 'previousGenerationReceiptSHA256': 'a' * 64,
            'previousControllerInstanceID': OLD, 'previousFinalizationSHA256': 'c' * 64,
            'previousBootID': UUID, 'targetBootID': payload['controllerBootID'],
            'finalizationSHA256': successor['newEnvelopeSHA256']}
        self.selection['controllerSuccessor'] = successor
        self.compute.successor = successor
        return successor

    def test_exact_lossless_recovery_and_no_old_writer_replay(self):
        with patch.object(recover, "restore_local", return_value={"generation": "new-generation", "generationReceiptSHA256": "b" * 64, "operatorSHA256": "c" * 64}) as restore:
            result = recover.run(self.selection, self.store, self.compute,
                                 lambda _s, _g: {"state": "host_rotation_observed", "generation": "new-generation"}, self.service)
        self.assertEqual(result["state"], "active")
        self.assertEqual(result["controllerID"], NEW)
        self.assertEqual(self.compute.owner, NEW)
        self.assertEqual(self.compute.deallocations, 1)
        self.assertEqual(self.compute.patches, 2)
        self.assertEqual(self.compute.identity_patches, 1)
        self.assertEqual(self.compute.identities[NEW], {RECOVERY_IDENTITY, CONTROLLER_IDENTITY})
        self.assertEqual(self.service.starts, 1)
        self.assertEqual(restore.call_args.args[1], result["fenceReceiptDigest"])
        with self.assertRaisesRegex(ValueError, "old controller generation"):
            recover.run(self.selection, self.store, self.compute)

    def test_lost_deallocate_ack_never_reissues_or_detaches(self):
        self.compute.lost_deallocate = True
        with self.assertRaisesRegex(RuntimeError, "deallocation is unknown"):
            recover.run(self.selection, self.store, self.compute)
        self.assertEqual(self.store.record["state"], "deallocate_pending")
        self.assertEqual(self.compute.deallocations, 1)
        self.assertEqual(self.compute.identity_patches, 0)
        self.assertEqual(self.compute.patches, 0)
        with self.assertRaisesRegex(RuntimeError, "deallocation is unknown"):
            recover.run(self.selection, self.store, self.compute)
        self.assertEqual(self.compute.deallocations, 1)
        self.compute.powered_off = True
        with patch.object(recover, "restore_local", return_value={"generation": "new-generation", "generationReceiptSHA256": "b" * 64, "operatorSHA256": "c" * 64}):
            self.assertEqual(recover.run(self.selection, self.store, self.compute,
                                         lambda _s, _g: {"state": "host_rotation_observed", "generation": "new-generation"}, self.service)["state"], "active")
        self.assertEqual(self.compute.deallocations, 1)

    def test_foreign_disk_prevents_fence_claim(self):
        self.compute.owner = "foreign"
        with self.assertRaisesRegex(ValueError, "ownership"):
            recover.run(self.selection, self.store, self.compute)
        self.assertEqual(self.store.record["state"], "active")
        self.assertEqual(self.compute.deallocations, 0)

    def test_lost_identity_ack_is_read_back_without_duplicate_grant(self):
        self.compute.lost_identity = True
        with patch.object(recover, "restore_local", return_value={"generation": "new-generation", "generationReceiptSHA256": "b" * 64, "operatorSHA256": "c" * 64}):
            result = recover.run(self.selection, self.store, self.compute,
                                 lambda _s, _g: {"state": "host_rotation_observed", "generation": "new-generation"}, self.service)
        self.assertEqual(result["state"], "active")
        self.assertEqual(self.compute.identity_patches, 1)

    def test_initial_attach_and_lost_mount_ack_use_read_only_inspection(self):
        self.store = FakeStore()
        self.compute.owner = ""
        self.compute.rows[OLD] = []
        self.compute.lost_mount = True
        with self.assertRaisesRegex(RuntimeError, "mount or format is uncertain"):
            recover.initialize(self.selection, self.store, self.compute)
        self.assertEqual(self.store.record["state"], "bootstrap_mount_pending")
        self.assertEqual(self.compute.patches, 1)
        self.assertEqual(self.compute.mount_commands, 1)
        self.assertEqual(recover.initialize(self.selection, self.store, self.compute)["state"], "active")
        self.assertEqual(self.compute.patches, 1)
        self.assertEqual(self.compute.mount_commands, 2)

    def test_signed_successor_lost_stage_ack_inspects_without_duplicate_write(self):
        successor = self.signed_successor()
        self.compute.lost_stage = True
        mounted = {'generation': 'new-generation', 'generationReceiptSHA256': 'b' * 64,
                   'operatorSHA256': 'c' * 64}
        with patch.object(recover, 'restore_local', return_value=mounted) as restore:
            with self.assertRaisesRegex(RuntimeError, 'staging uncertain'):
                recover.run(self.selection, self.store, self.compute)
            self.assertEqual(self.store.record['state'], 'successor_stage_pending')
            self.assertEqual(self.store.record['successorDigest'], recover.digest(successor))
            self.assertEqual(self.compute.stage_writes, 1)
            self.assertEqual(restore.call_count, 0)
            self.assertEqual(recover.run(self.selection, self.store, self.compute,
                lambda _s, _g: {'state': 'host_rotation_observed', 'generation': 'new-generation'},
                self.service)['state'], 'active')
            self.assertEqual(self.compute.stage_writes, 1)
            self.assertEqual([call.kwargs.get('inspect', False) for call in restore.call_args_list], [False, True])
        self.assertEqual(self.compute.owner, NEW)
        self.assertEqual(self.compute.deallocations, 1)

    def test_successor_stage_contains_only_exact_public_files_and_fenced_mount(self):
        successor = self.signed_successor()
        script = recover.successor_script(successor, 'f' * 64,
            mount_path=self.selection['mountSelectionPath'])
        encoded = re.search(r"item = json.loads\(base64.b64decode\('([^']+)'\)\)", script)[1]
        payload = json.loads(base64.b64decode(encoded))
        paths = {row['path'] for row in payload['files']}
        self.assertEqual(paths, {'/etc/luci/operator-template.json', '/etc/luci/azure-private-ca.crt',
                                 '/etc/luci/azure-controller.env', '/etc/luci/azure-operator-finalization.json',
                                 self.selection['mountSelectionPath'],
                                 '/etc/luci/azure-controller-disk-initial.json'})
        mount_selection = json.loads(next(row['content'] for row in payload['files']
                                         if row['path'] == self.selection['mountSelectionPath']))
        self.assertEqual(mount_selection['previousFenceReceiptDigest'], 'f' * 64)
        self.assertEqual(mount_selection['previousFinalizationSHA256'], successor['priorEnvelopeSHA256'])
        self.assertEqual(mount_selection, json.loads(next(row['content'] for row in payload['files']
            if row['path'] == '/etc/luci/azure-controller-disk-initial.json')))
        self.assertNotIn('PRIVATE KEY-----', json.dumps(payload))
        self.assertIn('elif True:', recover.successor_script(successor, 'f' * 64, inspect=True,
            mount_path=self.selection['mountSelectionPath']))
        successor['mountTemplate']['previousBootID'] = successor['newBootID']
        with self.assertRaisesRegex(ValueError, 'mount template'):
            recover.validate(self.selection)

    def test_signed_successor_fences_stale_selection_and_wrong_attached_disk(self):
        successor = self.signed_successor()
        self.compute.owner = 'foreign'
        with self.assertRaisesRegex(ValueError, 'ownership'):
            recover.run(self.selection, self.store, self.compute)
        self.compute.owner = OLD
        successor['newBootID'] = successor['oldBootID']
        with self.assertRaisesRegex(ValueError, 'signed successor'):
            recover.run(self.selection, self.store, self.compute)
        successor['newBootID'] = 'aaaaaaaa-1234-1234-1234-123456789abc'
        self.compute.lost_stage = True
        with self.assertRaisesRegex(RuntimeError, 'staging uncertain'):
            recover.run(self.selection, self.store, self.compute)
        successor['newGeneration'] = 'stale-generation'
        with self.assertRaisesRegex(ValueError, 'signed successor'):
            recover.run(self.selection, self.store, self.compute)

    def test_successor_restore_lost_ack_is_inspected_and_never_replayed(self):
        self.signed_successor()
        effects = []
        mounted = {'generation': 'new-generation', 'generationReceiptSHA256': 'b' * 64,
                   'operatorSHA256': 'c' * 64}

        def restore(_selection, _fence, inspect=False):
            effects.append(inspect)
            if not inspect:
                raise TimeoutError('lost mounted receipt')
            return mounted

        with patch.object(recover, 'restore_local', side_effect=restore):
            with self.assertRaisesRegex(RuntimeError, 'restore uncertain'):
                recover.run(self.selection, self.store, self.compute)
            self.assertEqual(self.store.record['state'], 'restore_observe_pending')
            self.assertEqual(effects, [False])
            self.assertEqual(recover.run(self.selection, self.store, self.compute,
                lambda _s, _g: {'state': 'host_rotation_observed', 'generation': 'new-generation'},
                self.service)['state'], 'active')
            self.assertEqual(effects, [False, True])
            self.assertEqual(self.compute.stage_writes, 1)
            self.assertEqual(self.compute.deallocations, 1)

    def test_successor_held_when_old_controller_still_runs_or_attachment_changes(self):
        self.signed_successor()
        self.compute.lost_deallocate = True
        with self.assertRaisesRegex(RuntimeError, 'deallocation is unknown'):
            recover.run(self.selection, self.store, self.compute)
        self.assertEqual(self.compute.stage_writes, 0)
        self.assertEqual(self.store.record['state'], 'deallocate_pending')
        self.compute.powered_off = True
        self.compute.lost_stage = True
        with self.assertRaisesRegex(RuntimeError, 'staging uncertain'):
            recover.run(self.selection, self.store, self.compute)
        self.compute.owner = 'foreign'
        with self.assertRaisesRegex(RuntimeError, 'single-writer'):
            recover.run(self.selection, self.store, self.compute)
        self.assertEqual(self.compute.stage_writes, 1)
        self.assertEqual(self.store.record['state'], 'successor_stage_pending')


if __name__ == "__main__":
    unittest.main()
