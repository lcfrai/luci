"""Offline tests only: no Azure request or secret value is printed."""

import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("azure_credentials", HERE / "credentials.py")
credentials = importlib.util.module_from_spec(spec)
spec.loader.exec_module(credentials)
installer_spec = importlib.util.spec_from_file_location("azure_installer", HERE / "installer.py")
installer = importlib.util.module_from_spec(installer_spec)
installer_spec.loader.exec_module(installer)


class FakeVault:
    def __init__(self):
        self.vault = "luciownedvault"
        self.subscription = "1" * 36
        self.vault_id = "/subscriptions/" + self.subscription + "/resourceGroups/luci-rg/providers/Microsoft.KeyVault/vaults/" + self.vault
        self.records = {}
        self.set_calls = 0
        self.lose_ack = False

    def __call__(self, args):
        if args == ["account", "show"]:
            return {"id": self.subscription}
        if args == ["keyvault", "show", "--name", self.vault]:
            return {"id": self.vault_id}
        prefix = ["keyvault", "secret"]
        if args[:3] == prefix + ["list"]:
            return [{"id": "https://" + self.vault + ".vault.azure.net/secrets/" + name}
                    for name in self.records]
        if args[:3] == prefix + ["list-versions"]:
            name = args[-1]
            return [{"id": row["id"]} for key, row in self.records.items() if key == name]
        if args[:3] == prefix + ["show"]:
            name = args[args.index("--name") + 1]
            row = self.records[name]
            if '--version' in args and row["id"].rsplit("/", 1)[1] != args[args.index('--version') + 1]:
                raise ValueError("wrong version")
            return row
        if args[:3] == prefix + ["set"]:
            self.set_calls += 1
            name = args[args.index("--name") + 1]
            row = {"id": "https://" + self.vault + ".vault.azure.net/secrets/" + name + "/" + format(self.set_calls, "032x"),
                   "value": Path(args[args.index("--file") + 1]).read_text()}
            self.records[name] = row
            if self.lose_ack:
                self.lose_ack = False
                raise RuntimeError("simulated lost acknowledgement")
            return row
        raise AssertionError("unexpected Azure request: " + str(args))


class AzureCredentialTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.parent = Path(self.temporary.name) / "private"
        self.parent.mkdir(mode=0o700)
        self.root = self.parent / "azure-credentials"
        self.selection_path = self.parent / "selection.json"
        self.fake = FakeVault()
        self.selection = {"terraformVariables": {
            "scope": "luci-team-a1", "generation": "gen-a1", "gateway_zone_name": "luci-team-a1.internal",
            "subscription_id": self.fake.subscription, "key_vault_name": self.fake.vault,
            "key_vault_id": self.fake.vault_id}}
        credentials.generate(self.selection, self.root, installer.atomic_json)

    def test_generation_has_private_certs_and_exact_sans(self):
        server = subprocess.check_output(["openssl", "x509", "-in", str(self.root / "server.crt"),
                                          "-noout", "-ext", "subjectAltName"], text=True)
        client = subprocess.check_output(["openssl", "x509", "-in", str(self.root / "client.crt"),
                                          "-noout", "-ext", "subjectAltName"], text=True)
        self.assertIn("DNS:*.luci-team-a1.internal", server)
        self.assertIn("URI:spiffe://luci/controller/luci-team-a1/gen-a1", client)
        self.assertEqual((self.root / "client.key").stat().st_mode & 0o777, 0o600)
        self.assertEqual(json.loads((self.root / "journal.json").read_text())["state"], "generated_unpublished")
        self.assertRegex((self.root / "team-api.txt").read_text().strip(), r"^e2b_[0-9a-f]{32}$")

    def test_successor_preserves_four_task_refs_and_appends_five_fresh_tls_versions_at_owned_names(self):
        credentials.publish(self.selection, self.root, installer.atomic_json, acknowledge=True, az_call=self.fake)
        before = json.loads((self.root / 'journal.json').read_text())
        original_key = (self.root / 'client.key').read_bytes()
        self.selection['terraformVariables']['generation'] = 'gen-next'
        target = self.parent / 'successor-credentials'
        credentials.generate(self.selection, target, installer.atomic_json, predecessor=self.root)
        generated = json.loads((target / 'journal.json').read_text())
        self.assertEqual(set(generated['published']), {'team-api', 'admin-token', 'sandbox-seed', 'attach-key'})
        self.assertNotEqual((target / 'client.key').read_bytes(), original_key)
        # This provider fixture keeps only latest values. Exact predecessor
        # reads happen before each version append; production retains versions.
        credentials.publish(self.selection, target, installer.atomic_json, acknowledge=True, az_call=self.fake)
        after = json.loads((target / 'journal.json').read_text())
        for kind in credentials.MATERIAL:
            self.assertEqual(after['names'][kind], before['names'][kind])
            if kind in generated['published']:
                self.assertEqual(after['published'][kind], before['published'][kind])
                self.assertEqual((target / credentials.MATERIAL[kind]).read_bytes(), (self.root / credentials.MATERIAL[kind]).read_bytes())
            else:
                self.assertNotEqual(after['published'][kind]['version'], before['published'][kind]['version'])
        self.assertEqual(self.fake.set_calls, 14)

    def test_exact_publication_and_binding_do_not_copy_values_to_selection(self):
        with self.assertRaisesRegex(ValueError, "acknowledgement"):
            credentials.publish(self.selection, self.root, installer.atomic_json, az_call=self.fake)
        outcome = credentials.publish(self.selection, self.root, installer.atomic_json,
                                      acknowledge=True, az_call=self.fake)
        self.assertEqual(outcome["versionCount"], 9)
        self.assertEqual(self.fake.set_calls, 9)
        credentials.bind_selection(self.selection, self.root, installer.atomic_json, self.selection_path)
        saved = self.selection_path.read_text()
        self.assertIn("host_secret_refs", saved)
        self.assertNotIn((self.root / "team-api.txt").read_text().strip(), saved)
        self.assertEqual(len(json.loads(saved)["terraformVariables"]["controller_secret_ids"]), 5)
        credentials.publish(self.selection, self.root, installer.atomic_json,
                            acknowledge=True, az_call=self.fake)
        self.assertEqual(self.fake.set_calls, 9)

    def test_lost_acknowledgement_reconciles_read_only_without_replay(self):
        self.fake.lose_ack = True
        with self.assertRaisesRegex(RuntimeError, "retained as unknown"):
            credentials.publish(self.selection, self.root, installer.atomic_json,
                                acknowledge=True, az_call=self.fake)
        self.assertEqual(self.fake.set_calls, 1)
        with self.assertRaisesRegex(RuntimeError, "outcome is unknown"):
            credentials.publish(self.selection, self.root, installer.atomic_json,
                                acknowledge=True, az_call=self.fake)
        self.assertEqual(self.fake.set_calls, 1)
        credentials.reconcile_pending(self.selection, self.root, installer.atomic_json, az_call=self.fake)
        credentials.publish(self.selection, self.root, installer.atomic_json,
                            acknowledge=True, az_call=self.fake)
        self.assertEqual(self.fake.set_calls, 9)

    def test_wrong_account_and_existing_secret_are_refused_before_set(self):
        self.fake.subscription = "2" * 36
        with self.assertRaisesRegex(ValueError, "account"):
            credentials.publish(self.selection, self.root, installer.atomic_json,
                                acknowledge=True, az_call=self.fake)
        self.fake.subscription = "1" * 36
        self.fake.records["luci-team-a1-admin-token"] = {"id": "https://" + self.fake.vault + ".vault.azure.net/secrets/luci-team-a1-admin-token/" + "f" * 32,
                                                           "value": "foreign"}
        with self.assertRaisesRegex(ValueError, "already exists"):
            credentials.publish(self.selection, self.root, installer.atomic_json,
                                acknowledge=True, az_call=self.fake)
        self.assertEqual(self.fake.set_calls, 0)


if __name__ == "__main__":
    unittest.main()
