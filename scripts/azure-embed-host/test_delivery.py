import importlib.util
import base64
import json
from pathlib import Path
import tempfile
import subprocess
import unittest
from unittest.mock import patch


def load(name):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


service = load("service")
delivery = load("deliver")


class HostDeliveryTest(unittest.TestCase):
    def test_controller_ip_change_requires_exact_predecessor_ca_signed_link(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); stage = root / 'luci-embed-owned'; stage.mkdir()
            subprocess.run(['openssl', 'ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', str(root / 'ca.key')], check=True, capture_output=True)
            subprocess.run(['openssl', 'req', '-new', '-x509', '-key', str(root / 'ca.key'), '-subj', '/CN=predecessor', '-out', str(stage / 'client-ca.crt')], check=True, capture_output=True)
            current = {'scope': 'scope-one', 'generation': 'gen-one', 'controllerPrivateIP': '10.2.0.5'}
            incoming = dict(current, generation='gen-two', previousGeneration='gen-one', controllerPrivateIP='10.2.0.9')
            payload = {'version': 1, 'sourceCommit': 'a' * 40, 'scope': 'scope-one', 'ownerID': 'owner-one',
                'oldGeneration': 'gen-one', 'newGeneration': 'gen-two', 'oldControllerID': '/owned/old', 'newControllerID': '/owned/new',
                'newBootID': '12345678-1234-1234-1234-123456789abc', 'diskID': '/owned/disk', 'oldPrivateIP': '10.2.0.5',
                'newPrivateIP': '10.2.0.9', 'newNICID': '/owned/nic', 'newNICETag': 'exact-nic', 'controllerSubnetID': '/owned/subnet',
                'predecessorFenceETag': 'exact-fence', 'predecessorFenceSHA256': 'b' * 64,
                'hostConfigSHA256': delivery.digest(json.dumps(incoming, sort_keys=True, separators=(',', ':')).encode())}
            (root / 'message').write_bytes(json.dumps(payload, sort_keys=True, separators=(',', ':')).encode())
            subprocess.run(['openssl', 'dgst', '-sha256', '-sign', str(root / 'ca.key'), '-out', str(root / 'signature'), str(root / 'message')], check=True, capture_output=True)
            link = {'payload': payload, 'signatureAlgorithm': 'ecdsa-sha256', 'signature': base64.b64encode((root / 'signature').read_bytes()).decode()}
            incoming['controllerSuccessorLink'] = link
            with patch.object(service, 'PRIVATE', root), patch.object(service, 'pinned_package', return_value={'luciRevision': 'a' * 40}):
                service.verify_successor_link(current, incoming, {'prepared': {'stage': str(stage)}})
                payload['newNICETag'] = 'substituted'
                with self.assertRaisesRegex(RuntimeError, 'owned host command failed'):
                    service.verify_successor_link(current, incoming, {'prepared': {'stage': str(stage)}})
                payload['newNICETag'] = 'exact-nic'
                incoming['controllerPrivateIP'] = '10.2.0.10'
                with self.assertRaisesRegex(ValueError, 'differs from exact'):
                    service.verify_successor_link(current, incoming, {'prepared': {'stage': str(stage)}})
                incoming.pop('controllerSuccessorLink')
                with self.assertRaisesRegex(ValueError, 'differs from exact'):
                    service.verify_successor_link(current, incoming, {'prepared': {'stage': str(stage)}})
    def test_exact_delivery_rotates_only_gateway_and_persists_config(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "deployment.json"
            old = {"version": 1, "scope": "owner-one", "generation": "g-one", "runtimePackageSHA256": "a" * 64,
                   "vaultURL": "https://example.vault.azure.net", "gatewayHost": "embed.private.example",
                   "gatewayPrivateIP": "10.1.0.4", "controllerPrivateIP": "10.2.0.5", "runtimeSubnetCIDR": "10.1.0.0/24",
                   "hostClientID": "0" * 8 + "-" + "0" * 4 + "-" + "0" * 4 + "-" + "0" * 4 + "-" + "0" * 12,
                   "controllerClientID": "1" * 8 + "-" + "1" * 4 + "-" + "1" * 4 + "-" + "1" * 4 + "-" + "1" * 12,
                   "secretRefs": {key: {"name": key.lower(), "version": "a" * 32} for key in service.REFS}}
            new = dict(old, generation="g-two", previousGeneration="g-one")
            incoming = Path(root) / "incoming.json"
            for target, value in ((path, old), (incoming, new)):
                target.write_text(json.dumps(value))
                target.chmod(0o600)
            record = {"state": "active", "generation": "g-one"}
            def finish(_):
                record["generation"] = "g-two"
            with patch.object(service, "HOST_CONFIG", path), patch.object(service, "private_tmpfs"), \
                    patch.object(service, "journal", return_value=record), patch.object(service, "rotate", side_effect=finish) as rotate, \
                    patch.object(service, "save") as save:
                result = service.deliver(path, incoming)
                rotate.assert_called_once_with(new)
                self.assertEqual(result["generation"], "g-two")
                self.assertEqual(result["configSHA256"], delivery.digest(path.read_bytes()))
                self.assertEqual(json.loads(path.read_text()), new)
                save.assert_called_once()
            next_bad = dict(new, generation="g-three", previousGeneration="g-two", scope="foreign")
            incoming.write_text(json.dumps(next_bad))
            with patch.object(service, "HOST_CONFIG", path), patch.object(service, "private_tmpfs"), \
                    patch.object(service, "journal", return_value={"state": "active", "generation": "g-two"}), \
                    patch.object(service, "rotate") as rotate:
                with self.assertRaisesRegex(ValueError, "boundary changed"):
                    service.deliver(path, incoming)
                rotate.assert_not_called()


class FakeTransport:
    def __init__(self, selection, lost=False, receipt=True):
        self.selection = selection
        self.lost = lost
        self.receipt = receipt
        self.posts = 0

    def request(self, method, url, body=None):
        if method == "GET" and "/disks/" in url:
            return 200, {}, {"id": self.selection["azureDataDiskID"], "managedBy": self.selection["controllerInstanceID"],
                             "properties": {"diskState": "Attached"}}
        if method == "GET" and "/virtualMachines/" in url:
            return 200, {}, {"id": self.selection["scaleSetID"] + "/virtualMachines/" + self.selection["instanceID"],
                             "properties": {"provisioningState": "Succeeded"}}
        if method == "POST":
            self.posts += 1
            if self.lost:
                raise TimeoutError("lost reply")
            self.script = body["script"][0]
            message = ""
            if self.receipt:
                host = self.selection["hostConfig"]
                host_hash = delivery.digest(json.dumps(host, sort_keys=True, separators=(",", ":")).encode())
                message = delivery.RECEIPT_PREFIX + json.dumps({"state": "active", "generation": host["generation"], "configSHA256": host_hash})
            return 200, {}, {"value": [{"message": message}]}
        raise AssertionError((method, url))


class ControllerDeliveryTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.TemporaryDirectory()
        self.addCleanup(self.root.cleanup)
        self.journal = Path(self.root.name) / "delivery.json"
        self.host = {"generation": "g-two", "previousGeneration": "g-one"}
        self.selection = {"journalPath": str(self.journal), "azureDataDiskID": "/subscriptions/" + "a" * 36 + "/resourcegroups/rg/providers/microsoft.compute/disks/disk",
                          "controllerInstanceID": "/subscriptions/" + "a" * 36 + "/resourcegroups/rg/providers/microsoft.compute/virtualmachines/controller",
                          "scaleSetID": "/subscriptions/" + "a" * 36 + "/resourcegroups/rg/providers/microsoft.compute/virtualmachinescalesets/host",
                          "instanceID": "0", "hostConfig": self.host,
                          "hostConfigSHA256": delivery.digest(json.dumps(self.host, sort_keys=True, separators=(",", ":")).encode())}

    def test_acknowledged_exact_host_receipt_and_no_second_dispatch(self):
        transport = FakeTransport(self.selection)
        with patch.object(delivery, "validate", side_effect=lambda *_: None):
            self.assertEqual(delivery.run(self.selection, transport)["state"], "host_rotation_observed")
            self.assertEqual(transport.posts, 1)
            self.assertIn("service.py deliver", transport.script)
            with self.assertRaisesRegex(ValueError, "requires exact reconciliation"):
                delivery.run(self.selection, transport)
            self.assertEqual(transport.posts, 1)

    def test_lost_ack_and_missing_receipt_remain_uncertain(self):
        for lost in (True, False):
            if self.journal.exists():
                self.journal.unlink()
            transport = FakeTransport(self.selection, lost=lost, receipt=False)
            with patch.object(delivery, "validate", side_effect=lambda *_: None):
                with self.assertRaisesRegex(RuntimeError, "unknown|exact settled"):
                    delivery.run(self.selection, transport)
            self.assertEqual(transport.posts, 1)
            self.assertEqual(json.loads(self.journal.read_text())["state"], "run_command_pending")

    def test_foreign_attachment_refuses_before_runcommand(self):
        transport = FakeTransport(self.selection)
        old_request = transport.request
        def foreign(method, url, body=None):
            result = old_request(method, url, body)
            if method == "GET" and "/disks/" in url:
                result[2]["managedBy"] = "foreign"
            return result
        transport.request = foreign
        with patch.object(delivery, "validate", side_effect=lambda *_: None):
            with self.assertRaisesRegex(ValueError, "attachment"):
                delivery.run(self.selection, transport)
        self.assertFalse(self.journal.exists())
        self.assertEqual(transport.posts, 0)

    def test_unknown_delivery_reconciles_by_inspection_without_second_rotation(self):
        transport = FakeTransport(self.selection, lost=True)
        with patch.object(delivery, 'validate'), patch.object(delivery, 'private_json', side_effect=lambda path: json.loads(path.read_text())):
            with self.assertRaisesRegex(RuntimeError, 'unknown'): delivery.run(self.selection, transport)
            transport.lost = False
            result = delivery.reconcile(self.selection, transport)
        self.assertEqual(result['state'], 'host_rotation_observed')
        self.assertEqual(transport.posts, 2)
        self.assertIn('service.py inspect', transport.script)
        self.assertNotIn('service.py deliver', transport.script)

    def test_current_controller_generation_is_exact_or_fenced_successor(self):
        selected = self.selection
        predecessor = {"generation": "g-one", "controllerInstanceId": selected["controllerInstanceID"],
                       "azureDataDiskId": selected["azureDataDiskID"]}
        self.assertTrue(delivery.bound_generation_receipt(predecessor, self.host, selected))
        successor = dict(predecessor, generation="g-two", previousGeneration="g-one", previousFenceReceiptDigest="f" * 64)
        self.assertTrue(delivery.bound_generation_receipt(successor, self.host, selected))
        for changed in (dict(successor, previousFenceReceiptDigest=""), dict(successor, previousGeneration="foreign"),
                        dict(successor, controllerInstanceId="foreign"), dict(successor, azureDataDiskId="foreign")):
            self.assertFalse(delivery.bound_generation_receipt(changed, self.host, selected))


if __name__ == "__main__":
    unittest.main()
