import importlib.util
import json
from pathlib import Path
import tempfile
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


if __name__ == "__main__":
    unittest.main()
