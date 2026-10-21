import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("azure_setup_installer", Path(__file__).with_name("installer.py"))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)
package_spec = importlib.util.spec_from_file_location('azure_setup_package', Path(__file__).with_name('package.py'))
package = importlib.util.module_from_spec(package_spec)
package_spec.loader.exec_module(package)
builder_spec = importlib.util.spec_from_file_location('azure_setup_builder', Path(__file__).with_name('builder.py'))
builder = importlib.util.module_from_spec(builder_spec)
builder_spec.loader.exec_module(builder)


class AzureSetupTest(unittest.TestCase):
    @staticmethod
    def generated_json(path, value):
        # These transport fixtures do not exercise the Linux-only private-file
        # or directory-fsync contract. Never replace that production contract.
        Path(path).write_bytes(installer.canonical(value) + b'\n')

    def transport_fixture(self):
        with mock.patch.object(installer, 'atomic_json', self.generated_json):
            return self.fixture()

    def test_private_selection_writers_retire_original_descriptor_if_mode_restriction_fails(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        # Only the generated Python block is exercised, with a nonsecret value
        # and its one fixed /etc path replaced by the owned temporary fixture.
        script = installer.initial_guest_script({'fixture': True})
        block = script.split("python3 - <<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
        original_path = "pathlib.Path('/etc/luci/azure-controller-recovery.json')"
        self.assertEqual(block.count(original_path), 1)
        block = block.replace(original_path, 'pathlib.Path(' + repr(str(root / 'guest.json')) + ')')
        for writer in ('atomic-json', 'guest-selection'):
            with self.subTest(writer=writer):
                captured = []
                def refuse_mode(fd, mode):
                    self.assertEqual(mode, 0o600)
                    os.fstat(fd)  # The live original descriptor, not an invented ID.
                    captured.append(fd)
                    raise OSError('fixture mode refused')
                with mock.patch.object(installer.os, 'fchmod', refuse_mode, create=True):
                    with self.assertRaisesRegex(OSError, 'fixture mode refused'):
                        if writer == 'atomic-json':
                            installer.atomic_json(root / 'selection.json', {'fixture': True})
                        else:
                            exec(compile(block, '<owned-guest-selection-fixture>', 'exec'), {})
                self.assertEqual(len(captured), 1)
                with self.assertRaises(OSError):
                    os.fstat(captured[0])
                self.assertEqual(list(root.iterdir()), [])

    def test_large_luci_source_pack_survives_package_installer_and_private_builder_stage(self):
        bundle, selected, _ = self.transport_fixture()
        packed = b'L' * (20 << 20)
        (bundle / 'builder-source/luci.pack').write_bytes(packed)
        source = json.loads((bundle / 'builder-source/source.json').read_bytes())
        source['luci']['pack'] = {'sha256': hashlib.sha256(packed).hexdigest(), 'bytes': len(packed)}
        (bundle / 'builder-source/source.json').write_text(json.dumps(source))
        files = {name: (bundle / name).read_bytes() for name in installer.EXPECTED_FILES}
        manifest = {'version': 1, 'luciRevision': 'a' * 40,
            'files': {name: {'sha256': hashlib.sha256(data).hexdigest(), 'size': len(data)} for name, data in files.items()}}
        files['manifest.json'] = json.dumps(manifest).encode()
        (bundle / 'manifest.json').write_bytes(files['manifest.json'])
        archive = bundle.parent / 'installer.tar'
        digest = package.write_archive(files, archive)
        self.assertGreater(archive.stat().st_size, 8 << 20)
        self.assertEqual(package.verify_archive(archive, digest), files)
        self.assertEqual(installer.bundle(bundle), manifest)
        core = SimpleNamespace(bundle=installer.bundle, private_json=lambda path: json.loads(Path(path).read_bytes()),
            requested=installer.requested, bounded_json=installer.bounded_json,
            canonical=installer.canonical, atomic_json=self.generated_json)
        stage = builder.source_payload(core, bundle, selected)
        self.assertGreater(stage['size'], 12 << 20)
        self.assertLessEqual(stage['size'], 64 << 20)
        self.assertEqual(stage['inputs']['files']['builder-source/luci.pack']['sha256'], source['luci']['pack']['sha256'])

    def test_role_bounds_and_ordinary_aggregate_refuse_before_member_reads(self):
        for name, size in (('builder-source/luci.pack', (40 << 20) + 1),
                ('builder-source/e2b.pack', (4 << 20) + 1), ('LICENSE', (4 << 20) + 1),
                ('builder-source/source.json', (2 << 20) + 1)):
            with self.subTest(name=name):
                bundle, _, _ = self.transport_fixture()
                manifest_path = bundle / 'manifest.json'
                manifest = json.loads(manifest_path.read_bytes())
                manifest['files'][name]['size'] = size
                manifest_path.write_text(json.dumps(manifest))
                # No matching oversized file exists: refusal is on declared bounds.
                with self.assertRaisesRegex(ValueError, 'member bound'):
                    installer.bundle(bundle)
                with self.assertRaises(ValueError):
                    package.check_sizes({name: size})
        bundle, _, _ = self.transport_fixture()
        manifest_path = bundle / 'manifest.json'
        manifest = json.loads(manifest_path.read_bytes())
        for name in ('LICENSE', 'azure-setup/README.md', 'azure-setup/installer.py'):
            manifest['files'][name]['size'] = 3 << 20
        manifest_path.write_text(json.dumps(manifest))
        with self.assertRaisesRegex(ValueError, 'bounded source inventory'):
            installer.bundle(bundle)
        with self.assertRaises(ValueError):
            package.check_sizes({name: 3 << 20 for name in ('LICENSE', 'azure-setup/README.md', 'azure-setup/installer.py')})

    def test_runtime_and_both_controller_nics_bind_dsv5_acceleration_without_other_family_enablement(self):
        compute = (ROOT / 'infra/terraform/azure_embed_private/compute.tf').read_text()
        blocks = re.split(r'(?m)(?=^resource ")', compute)
        for resource, setting, size in (
                ('azurerm_network_interface" "controller"', 'accelerated_networking_enabled', 'controller_vm_size'),
                ('azurerm_network_interface" "controller_recovery"', 'accelerated_networking_enabled', 'controller_vm_size'),
                ('azurerm_linux_virtual_machine_scale_set" "runtime"', 'enable_accelerated_networking', 'runtime_vm_size')):
            with self.subTest(resource=resource):
                block, = [block for block in blocks if block.startswith('resource "' + resource)]
                expression = re.search(r'(?m)^\s*' + setting + r'\s*=\s*can\(regex\("([^"]+)", var\.([a-z_]+)\)\)', block)
                self.assertIsNotNone(expression)
                self.assertEqual(expression[2], size)
                for sku in ('Standard_D2s_v5', 'Standard_D4s_v5', 'Standard_D8s_v5'):
                    self.assertIsNotNone(re.search(expression[1], sku))
                for sku in ('Standard_B2s', 'Standard_D4s_v3', 'Standard_D4_v5'):
                    self.assertIsNone(re.search(expression[1], sku))

    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        bundle = root / "bundle"
        bundle.mkdir()
        files = {}
        packed = b'fixture packed source'
        source_header = {role: {'revision': 'a'*40 if role == 'luci' else 'cf536c284434fdd2f09bbab35ec807a5dacc87e4',
            'pack': {'sha256': hashlib.sha256(packed).hexdigest(), 'bytes': len(packed)}} for role in ('luci','e2b')}
        for name in installer.EXPECTED_FILES:
            if name.startswith("terraform/"):
                source = ROOT / "infra/terraform/azure_embed_private" / Path(name).name
            elif name == "LICENSE":
                source = ROOT / "LICENSE"
            else:
                source = ROOT / "scripts" / name
            target = bundle / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((json.dumps(source_header).encode() if name.endswith('source.json') else packed) if name in installer.SOURCE_FILES else source.read_bytes())
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
        self.assertEqual(len(installer.plan_command(bundle, path)["nextSteps"]), 5)
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

    def test_account_command_sets_exact_selection_and_refuses_after_preparation(self):
        bundle, previous, _ = self.fixture()
        target = previous.parent / "new-selection.json"
        installer.initial_selection(bundle, target, "luci-team", "australiaeast", 200, 4)
        public = previous.parent / "owner.pub"
        public.write_text("ssh-ed25519 " + "A" * 48 + " luci\n")
        choices = dict(subscription_id="1" * 36, resource_group="luci-rg",
                       state_resource_group="luci-state-rg", state_storage_account="lucistateowned",
                       state_container="terraform-state", zone="1", runtime_size="Standard_D8s_v5",
                       controller_size="Standard_D4s_v5", ssh_public_key_file=str(public))
        result = installer.configure_account(bundle, target, **choices)
        self.assertFalse(result["providerEffects"])
        selected = installer.private_json(target)
        self.assertEqual(selected["terraformVariables"]["runtime_vm_size"], "Standard_D8s_v5")
        self.assertTrue(selected["stateBackend"]["key"].startswith("luci/luci-team-"))
        (target.parent / "azure-image-prep-state.json").write_text("retained")
        with self.assertRaisesRegex(ValueError, "retained work"):
            installer.configure_account(bundle, target, **choices)

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
                return json.dumps({"resource_changes": [{"change": {"actions": ["create"]}}, {'mode': 'data', 'change': {'actions': ['no-op']}}]}).encode()
            if args[0] == "apply":
                raise RuntimeError("lost response")
            return b""

        with mock.patch.object(installer, "terraform", fake_terraform):
            prepared = installer.prepare(bundle, selected, plan["planDigest"])
            self.assertEqual(prepared["selectionDigest"], plan["selectionDigest"])
            self.assertEqual(prepared["sourceCommit"], plan["luciRevision"])
            self.assertEqual(prepared['resourceCreates'], 1)
            self.assertEqual(prepared['resourceNoops'], 1)
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
        self.assertEqual(result["selectionDigest"], plan["selectionDigest"])
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
