"""Offline signing/finalization tests; provider and gateway are strict fixtures."""
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("operator_setup", HERE / "operator_setup.py")
operator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(operator)
mount_spec = importlib.util.spec_from_file_location("mount_setup", HERE.parent / "azure-controller-disk/mount.py")
mount = importlib.util.module_from_spec(mount_spec)
mount_spec.loader.exec_module(mount)


@unittest.skipUnless(shutil.which("openssl"), "OpenSSL is a required Linux installer prerequisite")
class OperatorTests(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        canonical = lambda value: json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
        def write(path, value):
            path = Path(path)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(canonical(value) + b"\n")
        core = SimpleNamespace(bundle=lambda _: {"luciRevision": "a" * 40},
            private_json=lambda path: json.loads(Path(path).read_bytes()), canonical=canonical,
            atomic_json=write, digest=lambda value: hashlib.sha256(canonical(value)).hexdigest(),
            requested=lambda selection: selection["terraformVariables"],
            local_command=lambda args, **kwargs: subprocess.check_output(args, **kwargs).decode().strip())
        credentials = root / "azure-credentials"
        credentials.mkdir()
        subprocess.run(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", str(credentials / "ca.key")], check=True, capture_output=True)
        subprocess.run(["openssl", "req", "-new", "-x509", "-key", str(credentials / "ca.key"), "-subj", "/CN=offline-test", "-out", str(credentials / "ca.crt")], check=True, capture_output=True)
        write(credentials / "journal.json", {"state": "published", "scope": "luci-fixture", "generation": "gen-one",
            "files": {"client-ca": hashlib.sha256((credentials / "ca.crt").read_bytes()).hexdigest()},
            "published": {kind: {"id": "https://luci-vault.vault.azure.net/secrets/" + kind + "/" + "b" * 32} for kind in operator.REFS.values()}})
        selection = {"name": "luci-fixture", "region": "australiaeast", "monthlyBudgetUsd": 100, "maxConcurrentVMs": 4,
            "terraformVariables": {"scope": "luci-fixture", "generation": "gen-one", "key_vault_name": "luci-vault",
                "controller_mount_path": "/mnt/luci-controller", "gateway_zone_name": "luci-fixture.internal",
                "runtime_vm_size": "Standard_D8s_v5", "subscription_id": "12345678-1234-1234-1234-123456789abc",
                "resource_group_name": "luci-rg", "zone": "1", "controller_owner_id": "owner-one",
                "controller_filesystem_uuid": "12345678-1234-1234-1234-123456789abc", "controller_operator_template_path": "/etc/luci/operator-template.json",
                "controller_host_id": "controller-one", "runtime_host_id": "runtime-one"}}
        path = root / "selection.json"
        write(path, selection)
        result = operator.deferred(core, root, path, "registry.example/guest@sha256:" + "c" * 64, 2, 2048, 300, 10000, ["api.openai.com"])
        return root, core, path, result

    def finalized(self):
        root, core, path, result = self.fixture()
        selection = core.private_json(path)
        prefix = "/subscriptions/" + selection["terraformVariables"]["subscription_id"] + "/resourcegroups/luci-rg/providers/microsoft.compute/"
        identities = {"runtime_scale_set_id": prefix + "virtualmachinescalesets/runtime", "controller_disk_id": prefix + "disks/controller",
            "controller_instance_id": prefix + "virtualmachines/controller", "controller_identity_client_id": "12345678-1234-1234-1234-123456789abc",
            "controller_fence_blob_url": "https://lucifence.blob.core.windows.net/controller-fence/controller-fence.json"}
        core.atomic_json(root / "azure-setup-state.json", {"state": "controller_bootstrapped_gateway_unverified", "selectionDigest": core.digest(selection),
            "resourceIDs": identities, "readbackPlanSHA256": "d" * 64})
        core.atomic_json(root / "azure-template-state.json", {"state": "template_ready_provider_guest_unverified", "selectionDigest": core.digest(selection)})
        create = {"runtimeRevision": operator.RUNTIME, "name": "luci-fixture-" + "e" * 32, "cpuCount": 2, "memoryMB": 2048}
        core.atomic_json(root / "template/create.json", create)
        core.atomic_json(root / "template/create.json.response", dict(create, templateId="private-template", buildId="private-build"))
        core.atomic_json(root / "template/build.json", {"runtimeRevision": operator.RUNTIME, "templateId": "private-template", "buildId": "private-build",
            "fromImage": "registry.example/guest@sha256:" + "c" * 64, "imageDigest": "c" * 64})
        instance = {"id": identities["runtime_scale_set_id"] + "/virtualMachines/0", "instanceId": "0", "properties": {"provisioningState": "Succeeded"}}
        readback = {"version": 1, "state": "controller_setup_readback", "microVMExecutionVerified": False,
            "sourceCommit": "a" * 40, "scope": "luci-fixture", "generation": "gen-one",
            "controllerResourceID": identities["controller_instance_id"], "dataDiskID": identities["controller_disk_id"],
            "runtimeInstance": instance, "nodes": [{"id": "node-one", "status": "ready"}], "templateStatus": "ready",
            "bootID": "12345678-1234-1234-1234-123456789abc", "createJournal": create,
            "createResponse": core.private_json(root / "template/create.json.response"), "buildJournal": core.private_json(root / "template/build.json"),
            "gatewayReceiptDigest": core.digest(["luci-azure-e2b-gateway-v1", "luci-fixture.internal", "luci-fixture", "gen-one", "api:3000", "guest:3002", "envd:49983", "sandboxd:8080"])}
        finalized = operator.finalize(core, root, path, lambda _: self.fail("fixture used setup-host gateway"), readback)
        return root, core, path, finalized, instance, readback

    def test_deferred_operator_binds_signature_and_keeps_provider_ids_empty(self):
        root, core, path, result = self.fixture()
        draft = core.private_json(root / "operator-template.json")
        self.assertEqual(draft["azureSecrets"]["clientId"], "")
        self.assertEqual(draft["template"]["buildId"], "")
        self.assertFalse(result["connected"])
        signed = core.private_json(root / "signed-guest-selection.json")
        message, signature, public = (root / name for name in ("message", "signature", "public"))
        message.write_bytes(core.canonical(signed["selection"]))
        signature.write_bytes(base64.b64decode(signed["signature"]))
        public.write_text(signed["signerPublicKey"])
        self.assertEqual(subprocess.run(["openssl", "dgst", "-sha256", "-verify", str(public), "-signature", str(signature), str(message)], capture_output=True).returncode, 0)
        message.write_bytes(b"altered guest selection")
        self.assertNotEqual(subprocess.run(["openssl", "dgst", "-sha256", "-verify", str(public), "-signature", str(signature), str(message)], capture_output=True).returncode, 0)

    def test_finalized_operator_signature_is_checked_before_mount_effects(self):
        root, core, path, result, _, _ = self.finalized()
        selection = core.private_json(path)
        envelope = core.private_json(root / "azure-operator-finalization.json")
        value = {"scope": "luci-fixture", "generation": "gen-one", "operatorTemplateSHA256": selection["terraformVariables"]["controller_operator_template_sha256"],
            "controllerInstanceId": envelope["payload"]["controllerInstanceId"], "azureDataDiskId": envelope["payload"]["diskID"],
            "ownerId": "owner-one", "mountPath": "/mnt/luci-controller"}
        draft = core.private_json(root / "operator-template.json")
        with mock.patch.object(mount, "private", return_value=core.canonical(envelope)), mock.patch.object(mount, "boot_identity", return_value=envelope["payload"]["controllerBootID"]):
            selected, journals = mount.finalized_operator(value, draft)
        self.assertEqual(selected["hostNodeId"], "node-one")
        self.assertEqual(selected["deployment"]["azureInstanceId"], "0")
        self.assertEqual(len(journals), 3)
        envelope["payload"]["operator"]["hostNodeId"] = "tampered-node"
        with mock.patch.object(mount, "private", return_value=core.canonical(envelope)), mock.patch.object(mount, "boot_identity", return_value=envelope["payload"]["controllerBootID"]), self.assertRaisesRegex(ValueError, "signature"):
            mount.finalized_operator(value, draft)

    def test_finalization_refuses_wrong_host_or_gateway_generation(self):
        root, core, path, _, instance, readback = self.finalized()
        readback["runtimeInstance"] = dict(instance, id=instance["id"] + "-foreign")
        with self.assertRaisesRegex(RuntimeError, "instance"):
            operator.finalize(core, root, path, lambda _: self.fail("fixture called provider"), readback)
        readback["runtimeInstance"] = instance
        readback["gatewayReceiptDigest"] = "wrong-generation"
        with self.assertRaisesRegex(RuntimeError, "generation"):
            operator.finalize(core, root, path, lambda _: self.fail("fixture called provider"), readback)

    def test_lost_operator_install_ack_requires_read_only_reconciliation(self):
        root, core, path, result, _, _ = self.finalized()
        def lost(arguments):
            self.assertEqual(core.private_json(root / "azure-operator-install-state.json")["state"], "operator_install_pending")
            raise TimeoutError("fixture lost acknowledgement")
        with self.assertRaises(TimeoutError):
            operator.install(core, root, path, result["finalizationDigest"], lost)
        with self.assertRaisesRegex(ValueError, "retained"):
            operator.install(core, root, path, result["finalizationDigest"], lost)
        envelope = core.private_json(root / "azure-operator-finalization.json")
        expected = {"sha256": hashlib.sha256(core.canonical(envelope)).hexdigest(), "generation": "gen-one", "sourceCommit": "a" * 40}
        def readback(arguments):
            script = arguments[-1]
            self.assertIn("if not True:", script)
            return {"value": [{"message": "LUCI_AZURE_OPERATOR_INSTALLED=" + json.dumps(expected)}]}
        settled = operator.install(core, root, path, result["finalizationDigest"], readback, reconcile=True)
        self.assertEqual(settled["state"], "operator_finalized_private_route_unverified")
        with self.assertRaisesRegex(ValueError, "preserve and reconcile"):
            operator.finalize(core, root, path, lambda _: self.fail("retained finalization queried Azure"))

    def test_egress_selection_matches_runtime_refusals(self):
        for names in (["localhost"], ["127.0.0.1"], ["host.internal"], ["api.openai.com", "api.openai.com"], ["-bad.example"]):
            self.assertFalse(operator.domains_valid(names))

    def test_controller_template_lost_ack_resumes_by_readback_without_host_gateway_or_secrets(self):
        root, core, path, _, _, readback = self.finalized()
        for name in ("create.json", "create.json.response", "build.json"):
            (root / "template" / name).unlink()
        core.atomic_json(root / "azure-template-state.json", {"state": "not_started"})
        calls = []
        def lost(arguments):
            calls.append(arguments)
            self.assertNotIn(str(root / "azure-credentials"), arguments[-1])
            self.assertIn("luci-api azure-setup template-create", arguments[-1])
            self.assertEqual(core.private_json(root / "azure-template-state.json")["pendingAction"], "create")
            raise TimeoutError("fixture lost ARM response")
        with self.assertRaises(TimeoutError):
            operator.controller_step(core, root, path, "create", lost, acknowledge=True)
        with self.assertRaisesRegex(ValueError, "uncertain"):
            operator.controller_step(core, root, path, "create", lost, acknowledge=True)
        self.assertEqual(len(calls), 1)
        def inspect(arguments):
            self.assertIn("luci-api azure-setup template-status", arguments[-1])
            return {"value": [{"message": "LUCI_AZURE_SETUP_RECEIPT=" + json.dumps(readback)}]}
        result = operator.controller_step(core, root, path, "status", inspect)
        self.assertEqual(result["state"], "template_ready_provider_guest_unverified")
        self.assertNotIn("pendingAction", result)
        self.assertTrue((root / "template/build.json").is_file())

    def test_finalization_uses_existing_filesystem_without_rewriting_disk_receipts(self):
        template = {"store": {}, "deployment": {"azureGatewayHost": "luci-fixture.internal"}}
        template_bytes = mount.canonical(template)
        value = {"mode": "initialize", "mountPath": "/mnt/luci-controller", "operatorTemplatePath": "/etc/luci/operator-template.json",
            "operatorTemplateSHA256": mount.sha(template_bytes), "diskReceiptPath": "/mnt/luci-controller/luci/disk-receipt.json",
            "generationReceiptPath": "/mnt/luci-controller/luci/generation.json", "operatorOutputPath": "/etc/luci/operator.json",
            "azureDataDiskId": "disk-one", "filesystemUuid": "uuid-one", "ownerId": "owner-one", "scope": "luci-fixture",
            "controllerHostId": "controller-one", "runtimeHostId": "runtime-one", "generation": "gen-one", "controllerInstanceId": "controller-id", "fenceBlobURL": "https://fence.example"}
        disk = {"version": 1, "azureDataDiskId": "disk-one", "filesystemUuid": "uuid-one", "ownerId": "owner-one", "scope": "luci-fixture",
                "controllerHostId": "controller-one", "runtimeHostId": "runtime-one"}
        generation = {"version": 1, "generation": "gen-one", "controllerInstanceId": "controller-id", "azureDataDiskId": "disk-one"}
        def private(path, *args):
            if path.name == "operator-template.json": return template_bytes
            return mount.canonical(disk if path.name == "disk-receipt.json" else generation)
        with mock.patch.object(mount, "selected"), mock.patch.object(mount, "private", side_effect=private), \
                mock.patch.object(mount, "finalized_operator", return_value=(template, [])), \
                mock.patch.object(mount.Path, "mkdir"), mock.patch.object(mount, "mount_exact") as filesystem, \
                mock.patch.object(mount, "atomic") as write:
            result = mount.bind(value, finalize_only=True)
        self.assertEqual(result["state"], "mounted")
        self.assertEqual(filesystem.call_args.args[0]["mode"], "restore")
        self.assertEqual(write.call_count, 1)
        self.assertEqual(write.call_args.args[0].name, "operator.json")


if __name__ == "__main__":
    unittest.main()
