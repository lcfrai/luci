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


class GuidedTest(unittest.TestCase):
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

    def test_wizard_passes_ordinary_choices_to_existing_validator(self):
        root, core, selected, _, _ = self.fixture()
        calls = []
        core.configure_account = lambda *args, **kwargs: calls.append(kwargs) or {"providerEffects": False}
        result = guided.wizard(core, root, selected, ask=lambda question: " selected ")
        self.assertFalse(result["providerEffects"])
        self.assertEqual(len(calls[0]), 12)
        self.assertEqual(calls[0]["ssh_public_key_file"], "selected")


if __name__ == "__main__":
    unittest.main()
