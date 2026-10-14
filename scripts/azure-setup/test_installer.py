import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("azure_setup_installer", Path(__file__).with_name("installer.py"))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class AzureSetupTest(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        bundle = root / "bundle"
        bundle.mkdir()
        files = {}
        for name in installer.EXPECTED_FILES:
            if name.startswith("terraform/"):
                source = ROOT / "infra/terraform/azure_embed_private" / Path(name).name
            elif name == "LICENSE":
                source = ROOT / "LICENSE"
            else:
                source = ROOT / "scripts" / name
            target = bundle / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(source.read_bytes())
            files[name] = {"sha256": hashlib.sha256(target.read_bytes()).hexdigest(), "size": target.stat().st_size}
        (bundle / "manifest.json").write_text(json.dumps({"version": 1, "luciRevision": "a" * 40, "files": files}))
        work = root / "private"
        work.mkdir(mode=0o700)
        package = work / "host.tar"
        package.write_bytes(b"owned package")
        digest = hashlib.sha256(package.read_bytes()).hexdigest()
        declared = set(re.findall(r'^variable "([a-z0-9_]+)"', (bundle / "terraform/variables.tf").read_text(), re.M))
        variables = {key: "selected" for key in declared - {"previous_generation", "tags"}}
        variables.update(name="luci-test", location="australiaeast", subscription_id="1" * 36,
                         resource_group_name="luci-rg", runtime_package_sha256=digest,
                         runtime_image_version_id="/subscriptions/image-version", controller_image_version_id="/subscriptions/controller-version",
                         host_secret_refs={"teamApiKey": {"name": "test", "version": "a" * 32}},
                         host_secret_ids=["selected"], controller_secret_ids=["selected"])
        selection = {"version": 1, "mode": "byo-azure-develop", "name": "luci-test", "region": "australiaeast",
                     "monthlyBudgetUsd": 100, "maxConcurrentVMs": 4, "terraformVariables": variables,
                     "controllerHealthURL": "http://127.0.0.1:8730/v1/healthz", "nextGeneration": "gen-next",
                     "runtimePackage": {"path": str(package), "sha256": digest},
                     "controllerPackage": {"path": str(package), "sha256": digest},
                     "stateBackend": {"subscriptionId": "1" * 36, "resourceGroupName": "state-rg",
                                      "storageAccountName": "luciownedstate", "containerName": "terraform-state", "key": "luci/test.tfstate"}}
        selection_path = work / "selection.json"
        installer.atomic_json(selection_path, selection)
        return bundle, selection_path, selection

    def test_small_input_initializer_generates_private_ids_without_provider_effect(self):
        bundle, previous, _ = self.fixture()
        path = previous.parent / "new-selection.json"
        result = installer.initial_selection(bundle, path, "luci-team", "australiaeast", 200, 4,
                                             "10.240.0.0/16")
        created = installer.private_json(path)
        self.assertFalse(result["providerEffects"])
        self.assertEqual(created["terraformVariables"]["runtime_subnet_cidr"], "10.240.0.0/24")
        self.assertEqual(created["terraformVariables"]["controller_subnet_cidr"], "10.240.1.0/24")
        self.assertEqual(created["terraformVariables"]["controller_private_ip"], "10.240.1.10")
        self.assertTrue(created["terraformVariables"]["controller_filesystem_uuid"])
        self.assertIn("state_backend_subscriptionId", result["missing"])
        with self.assertRaisesRegex(ValueError, "new file"):
            installer.initial_selection(bundle, path, "luci-team", "australiaeast", 200, 4)
        self.assertEqual(installer.private_json(path), created)

    def test_image_input_command_binds_paths_without_provider_effect(self):
        bundle, previous, _ = self.fixture()
        target = previous.parent / "images.json"
        paths = dict(luci_source=str(ROOT), e2b_source=str(ROOT), image_digests=str(previous),
                     envoy_binary=str(previous), envoy_license=str(previous),
                     runtime_image_root=str(previous.parent / "runtime"),
                     controller_image_root=str(previous.parent / "controller"),
                     operator_template=str(previous))
        result = installer.initial_image_inputs(bundle, target, **paths)
        self.assertFalse(result["providerEffects"])
        self.assertEqual(installer.private_json(target)["luciSource"], str(ROOT))
        with self.assertRaisesRegex(ValueError, "new file"):
            installer.initial_image_inputs(bundle, target, **paths)

    def test_offline_plan_checks_bundle_and_existing_remote_state(self):
        bundle, selected, selection = self.fixture()
        plan = installer.plan_command(bundle, selected)
        self.assertEqual(plan["missing"], [])
        self.assertFalse(plan["priceVerified"])
        self.assertIn("stateBackend", selection)
        selection["stateBackend"]["containerName"] = ""
        installer.atomic_json(selected, selection)
        self.assertIn("state_backend_containerName", installer.plan_command(bundle, selected)["missing"])
        (bundle / "terraform/versions.tf").write_text("changed")
        with self.assertRaisesRegex(ValueError, "member changed"):
            installer.plan_command(bundle, selected)

    def test_apply_uncertainty_is_journaled_and_never_replayed(self):
        bundle, selected, _ = self.fixture()
        plan = installer.plan_command(bundle, selected)
        commands = []

        def fake_terraform(work, *args, timeout=120):
            commands.append(args[0])
            if args[0] == "plan":
                (work / "selected.tfplan").write_bytes(b"exact-plan")
            if args[0] == "show":
                return json.dumps({"resource_changes": [{"change": {"actions": ["create"]}}]}).encode()
            if args[0] == "apply":
                raise RuntimeError("lost response")
            return b""

        with mock.patch.object(installer, "terraform", fake_terraform):
            prepared = installer.prepare(bundle, selected, plan["planDigest"])
            backend = (selected.parent / "azure-terraform/backend.hcl").read_text()
            self.assertIn('use_azuread_auth = true', backend)
            self.assertNotIn('backend=false', backend)
            with self.assertRaisesRegex(RuntimeError, "lost response"):
                installer.apply(bundle, selected, plan["planDigest"], prepared["terraformPlanSHA256"], True, True)
            retained = installer.private_json(selected.parent / "azure-setup-state.json")
            self.assertEqual(retained["state"], "apply_pending")
            with self.assertRaisesRegex(ValueError, "exact prepared operation"):
                installer.apply(bundle, selected, plan["planDigest"], prepared["terraformPlanSHA256"], True, True)
        self.assertEqual(commands.count("apply"), 1)

    def test_image_preparation_requires_selected_roots_and_retains_uncertain_build(self):
        bundle, selected, selection = self.fixture()
        template = selected.parent / "operator-template.json"
        template.write_text(json.dumps({"deployment": {"provider": "azure"}, "store": {},
            "azureSecrets": {"vaultUrl": "https://selected.vault.azure.net", "clientId": "1" * 36,
                             "directory": "/run/luci", **{key: {"name": key, "version": "a" * 32}
                                  for key in ("apiKey", "adminKey", "attachKey", "clientCert", "clientKey")}}}))
        template.chmod(0o600)
        selection["terraformVariables"]["controller_operator_template_path"] = "/etc/luci/selected.json"
        selection["terraformVariables"]["controller_operator_template_sha256"] = hashlib.sha256(template.read_bytes()).hexdigest()
        installer.atomic_json(selected, selection)
        runtime = selected.parent / "runtime-image"
        controller = selected.parent / "controller-image"
        runtime.mkdir()
        controller.mkdir()
        inputs = selected.parent / "image-inputs.json"
        installer.atomic_json(inputs, {"luciSource": str(ROOT), "e2bSource": str(ROOT),
            "imageDigests": str(template), "envoyBinary": str(template), "envoyLicense": str(template),
            "runtimeImageRoot": str(runtime), "controllerImageRoot": str(controller),
            "operatorTemplate": str(template)})
        with self.assertRaisesRegex(ValueError, "acknowledged"):
            installer.prepare_images(bundle, selected, inputs)
        self.assertFalse((selected.parent / "azure-image-prep-state.json").exists())
        def fake_command(arguments, cwd=None, timeout=600):
            if arguments[0] == "git" and arguments[3] == "rev-parse":
                return "a" * 40
            if arguments[0] == "git":
                return ""
            raise RuntimeError("compiler result uncertain")
        with mock.patch.object(installer, "local_command", fake_command):
            with self.assertRaisesRegex(RuntimeError, "uncertain"):
                installer.prepare_images(bundle, selected, inputs, True)
            self.assertEqual(installer.private_json(selected.parent / "azure-image-prep-state.json")["state"],
                             "image_prep_pending")
            with self.assertRaisesRegex(ValueError, "retained"):
                installer.prepare_images(bundle, selected, inputs, True)

    def test_lost_apply_reconciles_only_exact_readback_without_retry(self):
        bundle, selected, selection = self.fixture()
        plan = installer.plan_command(bundle, selected)
        installer.atomic_json(selected.parent / "azure-setup-state.json", {
            "version": 1, "state": "apply_pending", "planDigest": plan["planDigest"],
            "terraformPlanSHA256": "a" * 64})
        scope = "/subscriptions/" + "1" * 36 + "/resourcegroups/luci-rg/providers/microsoft.compute/"
        values = {"controller_instance_id": scope + "virtualmachines/luci-test-controller",
                  "controller_recovery_instance_id": scope + "virtualmachines/luci-test-controller-recovery",
                  "runtime_scale_set_id": scope + "virtualmachinescalesets/luci-test-runtime",
                  "controller_disk_id": scope.replace("microsoft.compute/", "microsoft.compute/") + "disks/luci-test-authoritative",
                  "controller_fence_blob_url": "https://selected.blob.core.windows.net/controller-fence/controller-fence.json",
                  "runtime_package_sha256": selection["runtimePackage"]["sha256"],
                  "runtime_image_version_id": selection["terraformVariables"]["runtime_image_version_id"],
                  "runtime_host_config_sha256": "b" * 64, "controller_identity_client_id": "2" * 36,
                  "controller_recovery_identity_client_id": "3" * 36,
                  "controller_identity_resource_id": "/subscriptions/" + "1" * 36 + "/resourcegroups/luci-rg/providers/microsoft.managedidentity/userassignedidentities/luci-controller",
                  "controller_recovery_identity_resource_id": "/subscriptions/" + "1" * 36 + "/resourcegroups/luci-rg/providers/microsoft.managedidentity/userassignedidentities/luci-recovery",
                  "private_gateway_host": "gateway.private.example"}
        calls = []
        def fake_terraform(work, *args, timeout=120):
            calls.append(args[0])
            if args[0] == "plan":
                (work / "reconcile.tfplan").write_bytes(b"provider-readback")
                return b""
            if args[0] == "show":
                return json.dumps({"resource_drift": [],
                    "planned_values": {"outputs": {key: {"value": value} for key, value in values.items()}}}).encode()
            raise AssertionError("readback may only plan and show")
        with mock.patch.object(installer, "terraform", fake_terraform):
            result = installer.observed_outputs(bundle, selected, plan["planDigest"])
        self.assertEqual(result["state"], "infrastructure_observed_pending_private_handoff")
        self.assertEqual(calls, ["plan", "show"])

    def test_unknown_private_controller_bootstrap_cannot_dispatch_again(self):
        bundle, selected, selection = self.fixture()
        uuid = "11111111-1111-1111-1111-111111111111"
        selection["stateBackend"]["subscriptionId"] = uuid
        selection["terraformVariables"]["subscription_id"] = uuid
        selection["terraformVariables"]["controller_filesystem_uuid"] = uuid
        installer.atomic_json(selected, selection)
        plan = installer.plan_command(bundle, selected)
        prefix = "/subscriptions/" + uuid + "/resourcegroups/luci-rg/providers/"
        resource_ids = {"controller_instance_id": prefix + "microsoft.compute/virtualmachines/luci-controller",
                        "controller_recovery_instance_id": prefix + "microsoft.compute/virtualmachines/luci-controller-recovery",
                        "controller_disk_id": prefix + "microsoft.compute/disks/luci-authoritative",
                        "controller_fence_blob_url": "https://luciownedfence.blob.core.windows.net/controller-fence/controller-fence.json",
                        "controller_recovery_identity_client_id": "22222222-2222-2222-2222-222222222222",
                        "controller_identity_resource_id": prefix + "microsoft.managedidentity/userassignedidentities/luci-controller",
                        "controller_recovery_identity_resource_id": prefix + "microsoft.managedidentity/userassignedidentities/luci-recovery"}
        installer.atomic_json(selected.parent / "azure-setup-state.json", {
            "version": 1, "state": "infrastructure_observed_pending_private_handoff",
            "planDigest": plan["planDigest"], "resourceIDs": resource_ids})
        calls = []
        def lost(*args):
            calls.append(args)
            raise RuntimeError("transport lost")
        with mock.patch.object(installer.shutil, "which", return_value="/usr/bin/az"), \
                mock.patch.object(installer, "invoke_initial_controller", lost):
            with self.assertRaisesRegex(RuntimeError, "uncertain"):
                installer.bootstrap(bundle, selected, plan["planDigest"])
            self.assertEqual(installer.private_json(selected.parent / "azure-setup-state.json")["state"],
                             "bootstrap_pending")
            with self.assertRaisesRegex(ValueError, "readback"):
                installer.bootstrap(bundle, selected, plan["planDigest"])
        self.assertEqual(len(calls), 1)


if __name__ == "__main__":
    unittest.main()
