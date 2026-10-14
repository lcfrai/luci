"""Offline adapter tests. Provider effects are fixtures, never Azure calls."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest

spec = importlib.util.spec_from_file_location("guided", Path(__file__).with_name("guided.py"))
guided = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guided)
credential_spec = importlib.util.spec_from_file_location("credentials", Path(__file__).with_name("credentials.py"))
credentials = importlib.util.module_from_spec(credential_spec)
credential_spec.loader.exec_module(credentials)


class GuidedTest(unittest.TestCase):
    def test_generated_team_key_matches_pinned_embed_host_format(self):
        first, second = credentials.team_api_key(), credentials.team_api_key()
        self.assertRegex(first, r"^e2b_[0-9a-f]{32}$")
        self.assertNotEqual(first, second)

    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        digest = lambda value: hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        def write(path, value):
            Path(path).write_text(json.dumps(value))
        core = SimpleNamespace(bundle=lambda _: {"luciRevision": "a" * 40},
            private_json=lambda path: json.loads(Path(path).read_text()),
            atomic_json=write, digest=digest, requested=lambda selection: selection["terraformVariables"],
            package_ready=lambda package: package == {"path": "fixture", "sha256": "b" * 64})
        selected = root / "selection.json"
        write(selected, {"region": "australiaeast", "stateBackend": {"subscriptionId": "1" * 36},
                         "terraformVariables": {"scope": "luci-fixture"}})
        write(root / "azure-image-prep-state.json", {"state": "image_roots_prepared_pending_gallery_publication",
              "runtimePackage": {"path": "fixture", "sha256": "b" * 64}})
        prefix = "/subscriptions/" + "1" * 36 + "/resourceGroups/luci-images/providers/Microsoft.Compute/"
        plan = guided.gallery_plan(core, root, selected, "runtime", prefix + "galleries/luci/images/runtime", "1.0.1", prefix + "images/runtime-source")
        observed = {"id": plan["imageVersionID"], "tags": plan["body"]["tags"],
                    "properties": {"provisioningState": "Succeeded", "storageProfile": {"source": {"id": plan["managedImageID"]}}}}
        return root, core, selected, plan, observed

    def test_rfc1918_only_including_full_range_not_merely_start_address(self):
        for value in ("10.20.1.0/24", "172.16.0.0/12", "192.168.0.4/32"):
            self.assertEqual(guided.private_cidr(value), value)
        for value in ("0.0.0.0/0", "127.0.0.0/8", "169.254.0.0/16", "100.64.0.0/10", "172.0.0.0/8", "192.168.0.0/15", "::1/128", "10.1.0.1/24"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                guided.private_cidr(value)

    def test_gallery_success_requires_exact_provider_readback(self):
        root, core, selected, plan, observed = self.fixture()
        calls = []
        def provider(arguments):
            calls.append(arguments)
            if "put" in arguments:
                self.assertEqual(core.private_json(root / "azure-gallery-runtime.json")["state"], "publication_pending")
                return {}
            return {"value": []} if arguments[arguments.index("--url") + 1].endswith("/versions?api-version=2023-07-03") else observed
        result = guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], True, provider=provider)
        self.assertEqual(result["state"], "gallery_observed")
        self.assertFalse(result["finalInvoiceVerified"])
        self.assertEqual(sum("put" in call for call in calls), 1)

    def test_lost_ack_restart_reads_without_repeating_put(self):
        root, core, selected, plan, observed = self.fixture()
        def lost(arguments):
            if "put" in arguments:
                raise TimeoutError("response lost")
            return {"value": []}
        with self.assertRaises(TimeoutError):
            guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], True, provider=lost)
        with self.assertRaisesRegex(ValueError, "retained operation"):
            guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], True, provider=lost)
        calls = []
        def readback(arguments):
            calls.append(arguments)
            return observed
        settled = guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], reconcile=True, provider=readback)
        self.assertEqual(settled["state"], "gallery_observed")
        self.assertEqual(len(calls), 1)
        self.assertIn("get", calls[0])

    def test_stale_selection_absence_failure_and_wrong_source_refuse(self):
        root, core, selected, plan, observed = self.fixture()
        with self.assertRaisesRegex(ValueError, "absence"):
            guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], True,
                provider=lambda _: {"value": [], "nextLink": "more"})
        self.assertFalse((root / "azure-gallery-runtime.json").exists())
        core.atomic_json(root / "azure-gallery-runtime.json", {"state": "publication_pending", "planDigest": plan["planDigest"], "imageVersionID": plan["imageVersionID"]})
        observed["properties"]["storageProfile"]["source"]["id"] += "-other"
        with self.assertRaisesRegex(RuntimeError, "source"):
            guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], reconcile=True, provider=lambda _: observed)
        selection = core.private_json(selected)
        selection["region"] = "other"
        core.atomic_json(selected, selection)
        with self.assertRaisesRegex(ValueError, "stale"):
            guided.gallery_publish(core, root, selected, "runtime", plan["planDigest"], reconcile=True, provider=lambda _: observed)

    def test_progress_retains_uncertainty_without_claiming_connection(self):
        root, core, selected, plan, _ = self.fixture()
        core.atomic_json(root / "azure-gallery-runtime.json", {"state": "publication_pending", "planDigest": plan["planDigest"]})
        state = guided.progress(core, root, selected)
        self.assertFalse(state["connected"])
        self.assertTrue(next(row for row in state["stages"] if row["id"] == "runtime-gallery")["uncertain"])

    def test_wizard_pending_foundation_returns_review_without_queries_or_replay(self):
        root, core, selected, _, _ = self.fixture()
        calls = []
        core.atomic_json(root / 'azure-foundation-plan.json', {'planDigest': 'pinned-plan', 'resources': [{'id': 'private-resource'}], 'retainedCharges': ['private endpoints']})
        core.atomic_json(root / 'azure-foundation-state.json', {'state': 'foundation_write_pending', 'pending': 'private-resource'})
        result = guided.wizard(core, root, selected, ask=lambda _: self.fail('pending stage asked new choices'), cli=lambda _: self.fail('pending stage called provider'))
        self.assertFalse(result['connected'])
        self.assertEqual(result['nextAction'], 'foundation-reconcile')
        self.assertEqual(result['planDigest'], 'pinned-plan')

    def test_wizard_discovers_private_location_and_stops_before_new_foundation_effects(self):
        root, core, selected, _, _ = self.fixture()
        selection = core.private_json(selected)
        selection.update(name='luci-fixture')
        core.atomic_json(selected, selection)
        answers = iter(['1', 'endpoints', '10.20.0.5', '', '10.30.0.0/24', 'no', ''])
        calls = []
        module = SimpleNamespace(AzureARM=lambda cli: 'fixture-arm',
            inventory=lambda *a: calls.append(a),
            plan=lambda *a: {'state': 'foundation_review_required', 'connected': False, 'priceVerified': False})
        def cli(arguments):
            if arguments[:2] == ['account', 'show']: return {'id': 'subscription', 'user': {'type': 'user'}}
            if arguments[:2] == ['ad', 'signed-in-user']: return 'operator-id'
            if arguments[:3] == ['network', 'vnet', 'list']: return [{'id': 'existing-private-vnet', 'name': 'private'}]
            self.fail('unexpected CLI mutation/query')
        result = guided.wizard(core, root, selected, ask=lambda _: next(answers), cli=cli, foundation=module)
        self.assertEqual(result['state'], 'foundation_review_required')
        self.assertFalse(result['connected'])
        self.assertEqual(calls[0][3:8], ('existing-private-vnet', 'endpoints', '10.20.0.5', 22, '10.30.0.0/24'))

    def test_private_location_requires_route_ack_and_refuses_retained_preparation(self):
        root, core, selected, _, _ = self.fixture()
        (root / "azure-image-prep-state.json").unlink()
        with self.assertRaisesRegex(ValueError, "acknowledge"):
            guided.configure_private(core, root, selected, "10.240.0.0/16", "192.168.1.0/24", False)
        result = guided.configure_private(core, root, selected, "10.240.0.0/16", "192.168.1.0/24", True)
        self.assertFalse(result["connected"])
        self.assertEqual(core.private_json(selected)["terraformVariables"]["controller_private_ip"], "10.240.1.10")
        core.atomic_json(root / "azure-setup-state.json", {"state": "apply_pending"})
        with self.assertRaisesRegex(ValueError, "retained"):
            guided.configure_private(core, root, selected, "10.241.0.0/16", "192.168.1.0/24", True)

    def test_authenticated_host_key_binding_and_stale_generation_refusal(self):
        root, core, selected, _, _ = self.fixture()
        selection = core.private_json(selected)
        selection["terraformVariables"].update(management_source_cidr="192.168.1.0/24", controller_private_ip="10.240.1.10",
            resource_group_name="luci-rg", controller_mount_path="/mnt/luci-controller", generation="gen-one", controller_owner_id="owner-one")
        core.atomic_json(selected, selection)
        controller_id = "/subscriptions/" + "1" * 36 + "/resourcegroups/luci-rg/providers/microsoft.compute/virtualmachines/luci-controller"
        disk_id = controller_id.replace("virtualmachines/luci-controller", "disks/luci-state")
        core.atomic_json(root / "azure-setup-state.json", {"state": "controller_bootstrapped_gateway_unverified",
            "selectionDigest": core.digest(selection), "resourceIDs": {"controller_instance_id": controller_id, "controller_disk_id": disk_id}})
        vm_id = "12345678-1234-1234-1234-123456789abc"
        raw = b'\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20' + b'K' * 32
        observed = {"vmID": vm_id, "bootID": vm_id, "generation": {"generation": "gen-one", "controllerInstanceId": controller_id, "azureDataDiskId": disk_id},
            "disk": {"scope": "luci-fixture", "ownerId": "owner-one", "azureDataDiskId": disk_id},
            "hostKey": "ssh-ed25519 " + guided.base64.b64encode(raw).decode() + " fixture"}
        def provider(arguments):
            if arguments[0] == "rest":
                return {"id": controller_id, "properties": {"vmId": vm_id}}
            self.assertEqual(core.private_json(root / "azure-connection-state.json")["state"], "host_identity_readback_pending")
            return {"value": [{"message": "LUCI_AZURE_HOST_IDENTITY=" + json.dumps(observed)}]}
        receipt = guided.connection_descriptor(core, root, selected, provider=provider)
        self.assertFalse(receipt["connected"])
        self.assertEqual(receipt["host"], "10.240.1.10")
        self.assertTrue(receipt["fingerprint"].startswith("SHA256:"))
        observed["generation"]["generation"] = "stale-generation"
        with self.assertRaisesRegex(RuntimeError, "fenced generation"):
            guided.connection_descriptor(core, root, selected, provider=provider)
        self.assertEqual(core.private_json(root / "azure-connection-state.json")["state"], "host_identity_readback_pending")


if __name__ == "__main__":
    unittest.main()
