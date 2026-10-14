import importlib.util
import json
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch


def load(name):
    path = Path(__file__).with_name(name + ".py")
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


package = load("package")
service = load("service")


class PackageTest(unittest.TestCase):
    def test_exact_archive_and_image_install(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            source = root / "e2b/embed/compose"
            source.mkdir(parents=True)
            (root / "LICENSE").write_text("Luci test license")
            (root / "e2b/LICENSE").write_text("E2B test license")
            (source / "compose.yaml").write_text("services:\n  api: {image: upstream/api:tag}\n  host: {image: upstream/host:tag}\n  base-template: {image: upstream/node:tag}\n  ready: {image: upstream/node:tag}\n")
            (source / ".env").write_text("ENV=base\n")
            (source / "config").mkdir()
            (source / "config/config.txt").write_text("pinned\n")
            images = root / "images.json"
            images.write_text(json.dumps({"api": "upstream/api@sha256:" + "a" * 64,
                                          "host": "upstream/host@sha256:" + "b" * 64, "base-template": "upstream/node@sha256:" + "d"*64, "ready": "upstream/node@sha256:" + "d"*64}))
            binary = root / "binary"
            binary.write_bytes(b"binary fixture")
            licence = root / "envoy-license"
            licence.write_text("Envoy test license")
            args = types.SimpleNamespace(upstream=root / "e2b", luci=root, luci_revision="c" * 40,
                                         image_digests=images, bootstrap=binary, gateway=binary, envoy=binary,
                                         envoy_license=licence)
            with patch.object(package, "exact_checkout"):
                files = package.collect(args)
            archive = root / "host.tar"
            expected = package.write_archive(files, archive)
            self.assertEqual(files, package.read_archive(archive, expected))
            package.install(files, root / "image", expected, archive)
            self.assertEqual((root / "image/opt/luci/azure-embed/package.sha256").read_text().strip(), expected)
            self.assertIn("run-luci_embed.mount", [p.name for p in (root / "image/etc/systemd/system").iterdir()])
            self.assertIn("@sha256:", (root / "image/opt/luci/azure-embed/package/compose/image-lock.yaml").read_text())
            self.assertIn('guest_registry.py', '\n'.join(files))
            self.assertIn('azure-services.yaml', '\n'.join(files))
            excluded = package.yaml.safe_load(files['compose/azure-services.yaml'])['services']
            self.assertEqual(set(excluded), {'base-template','ready'})
            self.assertTrue(all(value['profiles'] == ['luci-manual-template-only'] for value in excluded.values()))
            with self.assertRaisesRegex(ValueError, "already contains"):
                package.install(files, root / "image", expected, archive)
            archive.write_bytes(archive.read_bytes() + b"tampered")
            with self.assertRaisesRegex(ValueError, "SHA-256"):
                package.read_archive(archive, expected)

    def test_missing_image_digest_and_link_refused(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            compose = root / "e2b/embed/compose"
            compose.mkdir(parents=True)
            (root / "LICENSE").write_text("Luci test license")
            (root / "e2b/LICENSE").write_text("E2B test license")
            (compose / "compose.yaml").write_text("services:\n  api: {image: api:tag}\n")
            (compose / ".env").write_text("ENV=base\n")
            binary = root / "binary"
            binary.write_bytes(b"x")
            licence = root / "envoy-license"
            licence.write_text("Envoy test license")
            images = root / "images.json"
            images.write_text("{}")
            args = types.SimpleNamespace(upstream=root / "e2b", luci=root, luci_revision="c" * 40,
                                         image_digests=images, bootstrap=binary, gateway=binary, envoy=binary,
                                         envoy_license=licence)
            with patch.object(package, "exact_checkout"):
                with self.assertRaisesRegex(ValueError, "every upstream service"):
                    package.collect(args)
                (compose / "unsafe").symlink_to(binary)
                with self.assertRaisesRegex(ValueError, "contains a link"):
                    package.collect(args)


class HostLifecycleTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.private = self.root / "private"
        self.private.mkdir(mode=0o700)
        self.journal = self.root / "journal.json"
        self.patches = [patch.object(service, "PRIVATE", self.private), patch.object(service, "JOURNAL", self.journal),
                        patch.object(service, "private_tmpfs"), patch.object(service, "pinned_package", return_value={
                            "files": {"compose/compose.yaml": {"sha256": "a" * 64}, "compose/.env": {"sha256": "b" * 64}}}),
                        patch.object(service, "host_ip", return_value="10.20.0.6"), patch.object(service, "has_kvm", return_value=True)]
        for entry in self.patches:
            entry.start()
            self.addCleanup(entry.stop)
        self.calls = []
        self.fail_compose = False
        self.running = False
        self.gateway = False
        self.fail_gateway = False
        self.command_patch = patch.object(service, "command", side_effect=self.fake_command)
        self.command_patch.start()
        self.addCleanup(self.command_patch.stop)
        self.refs = {name: {"name": name.lower(), "version": "a" * 32} for name in service.REFS}
        self.c = {"scope": "scope-one", "generation": "generation-one", "runtimePackageSHA256": "c" * 64,
                  "gatewayHost": "embed.private.example", "vaultURL": "https://example.vault.azure.net",
                  "controllerPrivateIP": "10.21.0.5",
                  "hostClientID": "0" * 8 + "-" + "0" * 4 + "-" + "0" * 4 + "-" + "0" * 4 + "-" + "0" * 12,
                  "controllerClientID": "1" * 8 + "-" + "1" * 4 + "-" + "1" * 4 + "-" + "1" * 4 + "-" + "1" * 12,
                  "secretRefs": self.refs}

    def fake_command(self, argv, timeout=30):
        self.calls.append(tuple(argv))
        text = " ".join(argv)
        if "azure-embed-host-bootstrap" in text and "--retire" not in text:
            stage = self.private / ("luci-embed-" + self.c["generation"])
            stage.mkdir(mode=0o700)
            for name in ("runtime.env", "gateway.json"):
                (stage / name).write_text("private")
            return str(stage)
        if "--retire" in text:
            stage = Path(argv[-1])
            for child in stage.iterdir():
                child.unlink()
            stage.rmdir()
        if "azure-gateway-config" in text:
            return "private artifact"
        if argv[:3] == ["docker", "ps", "--filter"]:
            if 'com.docker.compose.service' in argv[-1]:
                return "api\nclient-proxy\norchestrator" if self.running else ""
            return ""
        if argv[:3] == ["nft", "list", "tables"]:
            return ""
        if argv[:2] == ["docker", "compose"] and argv[-2:] == ["up", "-d"]:
            if self.fail_compose:
                raise RuntimeError("lost Compose acknowledgment")
            self.running = True
        if argv[:2] == ["docker", "compose"] and argv[-3:] == ["--status", "running", "--services"]:
            return "api\nclient-proxy\norchestrator" if self.running else ""
        if argv[:2] == ["systemctl", "restart"]:
            if self.fail_gateway:
                raise RuntimeError("lost gateway restart acknowledgment")
            self.gateway = True
        if argv[:2] == ["systemctl", "is-active"]:
            return "active" if self.gateway else "inactive"
        return ""

    def test_boot_retry_does_not_replay_and_gateway_rotation_keeps_compose(self):
        self.fail_compose = True
        with self.assertRaisesRegex(RuntimeError, "lost Compose acknowledgment"):
            service.boot(self.c)
        self.assertEqual(service.journal()["state"], "compose_pending")
        original_up = sum(call[-2:] == ("up", "-d") for call in self.calls)
        self.fail_compose = False
        with self.assertRaisesRegex(RuntimeError, "no startup replay"):
            service.boot(self.c)
        self.assertEqual(sum(call[-2:] == ("up", "-d") for call in self.calls), original_up)

    def test_exact_rotation_does_not_restart_runtime_or_rotate_task_keys(self):
        service.boot(self.c)
        self.assertEqual(service.journal()["state"], "active")
        self.assertTrue(self.running and self.gateway)
        old_up = sum(call[-2:] == ("up", "-d") for call in self.calls)
        service.boot(self.c)
        self.assertEqual(sum(call[-2:] == ("up", "-d") for call in self.calls), old_up)
        next_c = dict(self.c, generation="generation-two", previousGeneration="generation-one",
                      secretRefs={**self.refs, "gatewayServerCert": {"name": "gatewayservercert", "version": "b" * 32}})
        self.c = next_c
        service.rotate(next_c)
        self.assertEqual(service.journal()["generation"], "generation-two")
        self.assertEqual(service.journal()["state"], "active")
        self.assertEqual(sum(call[-2:] == ("up", "-d") for call in self.calls), old_up)
        next_c = dict(next_c, generation="generation-three", previousGeneration="generation-two",
                      secretRefs={**next_c["secretRefs"], "teamApiKey": {"name": "teamapikey", "version": "b" * 32}})
        with self.assertRaisesRegex(RuntimeError, "drained, fenced"):
            service.rotate(next_c)
        self.assertEqual(service.journal()["generation"], "generation-two")

    def test_reboot_restages_same_gateway_without_compose_dispatch(self):
        service.boot(self.c)
        old_up = sum(call[-2:] == ("up", "-d") for call in self.calls)
        old_stage = Path(service.journal()["prepared"]["stage"])
        for child in old_stage.iterdir():
            child.unlink()
        old_stage.rmdir()
        for artifact in self.private.glob("*generation-one*"):
            artifact.unlink()
        self.gateway = False
        service.boot(self.c)
        self.assertEqual(service.journal()["state"], "active")
        self.assertEqual(sum(call[-2:] == ("up", "-d") for call in self.calls), old_up)

    def test_lost_rotation_ack_preserves_both_generations_and_refuses_replay(self):
        service.boot(self.c)
        old_stage = Path(service.journal()["prepared"]["stage"])
        next_c = dict(self.c, generation="generation-two", previousGeneration="generation-one")
        self.c = next_c
        self.fail_gateway = True
        with self.assertRaisesRegex(RuntimeError, "lost gateway restart"):
            service.rotate(next_c)
        self.assertEqual(service.journal()["state"], "rotation_pending")
        self.assertTrue(old_stage.exists())
        self.assertTrue(Path(service.journal()["nextPrepared"]["stage"]).exists())
        old_restarts = sum(call[:2] == ("systemctl", "restart") for call in self.calls)
        self.fail_gateway = False
        with self.assertRaisesRegex(RuntimeError, "exact settled previous"):
            service.rotate(next_c)
        self.assertEqual(sum(call[:2] == ("systemctl", "restart") for call in self.calls), old_restarts)

    def test_registry_drift_blocks_initial_and_reboot_without_compose_effect(self):
        with patch.object(service, 'guest_source_ready', side_effect=ValueError('registry source drift')):
            with self.assertRaisesRegex(ValueError, 'registry source drift'): service.boot(self.c)
        self.assertFalse(any(call[-2:] == ('up', '-d') for call in self.calls))
        service.boot(self.c)
        before = len(self.calls)
        with patch.object(service, 'guest_source_ready', side_effect=ValueError('registry source drift')):
            with self.assertRaisesRegex(ValueError, 'registry source drift'): service.boot(self.c)
        self.assertEqual(len(self.calls),before)

    def test_nonsecret_deployment_selection_refuses_unsafe_address_or_identity(self):
        c = dict(self.c, version=1, gatewayPrivateIP="10.20.0.4", runtimeSubnetCIDR="10.20.0.0/24")
        path = self.root / "selected.json"
        def check(value):
            path.write_text(json.dumps(value))
            path.chmod(0o600)
            return service.config(path)
        self.assertEqual(check(c)["generation"], "generation-one")
        with self.assertRaisesRegex(ValueError, "host network"):
            check(dict(c, gatewayPrivateIP="127.0.0.1"))
        with self.assertRaisesRegex(ValueError, "runtime subnet"):
            check(dict(c, runtimeSubnetCIDR="10.21.0.0/24"))
        with self.assertRaisesRegex(ValueError, "Key Vault"):
            check(dict(c, vaultURL="https://example.vault.azure.net?token=unsafe"))
        with self.assertRaisesRegex(ValueError, "identities must differ"):
            check(dict(c, controllerClientID=c["hostClientID"]))


if __name__ == "__main__":
    unittest.main()
