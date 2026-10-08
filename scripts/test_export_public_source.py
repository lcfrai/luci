"""Source-export boundary tests against disposable, synthetic Git repositories."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile
from retained_ui import SHADCN_ROOT, SHADCN_MANIFEST, SHADCN_METADATA, SHADCN_COMPONENTS


sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("exporter", Path(__file__).with_name("export-public-source.py"))
exporter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(exporter)


class ExportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="luci-source-export-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "source"
        self.repo.mkdir()
        self.run_git("init", "--quiet")
        self.run_git("config", "user.name", "Source export fixture")
        self.run_git("config", "user.email", "fixture@example.invalid")
        self.run_git("config", "core.autocrlf", "false")
        # Fixture modes are deliberately staged with update-index. Subsequent
        # add calls must preserve those modes on both Windows and Unix hosts.
        self.run_git("config", "core.filemode", "false")
        for name in exporter.REQUIRED:
            self.write(name, b"fixture\n")
        self.write("LICENSE", b"MIT License\nPermission is hereby granted\n")
        self.write("desktop/package.json", exporter.json_bytes({"version": "0.4.0", "license": "MIT"}))
        self.write("desktop/package-lock.json", exporter.json_bytes({"version": "0.4.0", "packages": {"": {"version": "0.4.0"}}}))
        self.write("desktop/src-tauri/Cargo.toml", b'[package]\nname = "luci"\nversion = "0.4.0"\nlicense = "MIT"\n')
        self.write("desktop/src-tauri/Cargo.lock", b'[[package]]\nname = "luci"\nversion = "0.4.0"\n')
        self.write("desktop/src-tauri/tauri.conf.json", b'{"version":"0.4.0"}')
        self.write("control-plane/internal/product/server.go", b'const Version = "0.4.0"\n')
        self.write(".github/workflows/luci-ci.yml", b"env:\n  LUCI_VERSION: '0.4.0'\n")
        self.grant = b"Original upstream grant\r\nExact CRLF bytes\r\n"
        self.write("third-party/licenses/fixture/LICENSE", self.grant)
        self.write("third-party/licenses/manifest.json", exporter.json_bytes([
            {"files": [{"path": "fixture/LICENSE", "sha256": exporter.sha(self.grant)}]},
        ]))
        self.assets = {
            "LICENSE": b"Synthetic Apache-2.0 grant fixture\r\n",
            "volumes/functions/main/index.ts": b"// retained main fixture\r\n",
            "volumes/functions/_vendor/jose/dist/webapi/index.js": b"// retained jose fixture\r\n",
        }
        self.asset_manifest = {
            "upstream": "https://github.com/supabase/supabase",
            "revision": "564eab8ad7840b13324f68b1bfac074ef8d51c21",
            "version": "self-hosted/v0.8.2", "licence": "Apache-2.0",
            "files": [{"path": name, "bytes": len(data), "sha256": exporter.sha(data)}
                      for name, data in self.assets.items()],
        }
        self.write(exporter.SUPABASE_MANIFEST, exporter.json_bytes(self.asset_manifest))
        self.write(exporter.SUPABASE_ROOT + "NOTICE.txt", b"Synthetic asset provenance fixture\n")
        for name, data in self.assets.items():
            self.write(exporter.SUPABASE_ROOT + name, data)
        self.write("docs/legacy/private.md", b"excluded archive\n")
        self.write("desktop/.env.local", b"excluded secret state\n")
        self.write("desktop/src/seams.test.ts", b"historical audit\n")
        self.write("desktop/src/active.test.ts", b"retained test\n")
        self.run_git("add", "--all")
        self.run_git("update-index", "--chmod=+x", "scripts/build-release.sh")
        self.commit()

    def run_git(self, *args):
        return subprocess.run(["git", "-C", str(self.repo), *args], check=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout

    def write(self, name, data):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def commit(self):
        self.run_git("add", "--all")
        self.run_git("commit", "--quiet", "-m", "Synthetic export fixture")
        self.revision = self.run_git("rev-parse", "HEAD").decode().strip()

    def export(self, name="candidate", plan=False):
        return exporter.export(self.repo, self.revision, "0.4.0", self.root / name, plan)

    def add_fluid(self):
        grant = (Path(__file__).resolve().parent.parent / exporter.FLUID_ROOT / "LICENSE").read_bytes()
        name = exporter.FLUID_SOURCE + "components/button.tsx"
        source = b"// Synthetic adapted Fluid component\r\nexport const Button = 1\r\n"
        self.write(exporter.FLUID_ROOT + "LICENSE", grant)
        self.write(exporter.FLUID_ROOT + "README.md", b"Fixture attribution\n")
        self.write(name, source)
        self.write(exporter.FLUID_MANIFEST, exporter.json_bytes({
            "name": "fluid-functionalism",
            "upstream": "https://github.com/mickadesign/fluid-functionalism",
            "revision": "97183f8018e619722edffd2f9fccefbea0effe8d", "license": "MIT",
            "files": [{"path": name, "upstreamPath": "registry/radix/button.tsx",
                       "upstreamSha256": exporter.sha(b"Synthetic upstream source\n"),
                       "normalizedSha256": exporter.sha(source.replace(b"\r\n", b"\n"))}],
        }))
        return name, source, grant

    def add_shadcn(self):
        source_root = Path(__file__).resolve().parent.parent
        for name in SHADCN_METADATA | SHADCN_COMPONENTS:
            self.write(name, (source_root / name).read_bytes())

    def test_shadcn_original_grant_and_sources_survive_export_and_zip(self):
        self.add_shadcn()
        # Unreviewed adjacent metadata must not broaden the export boundary.
        self.write(SHADCN_ROOT + 'unreviewed.txt', b'not part of selected provenance')
        self.commit()
        result = self.export()
        output = Path(result['output'])
        source_root = Path(__file__).resolve().parent.parent
        for name in SHADCN_METADATA | SHADCN_COMPONENTS:
            self.assertEqual((output / name).read_bytes(), (source_root / name).read_bytes())
        self.assertFalse((output / SHADCN_ROOT / 'unreviewed.txt').exists())
        checks = json.loads((output / 'EXPORT-CHECKS.json').read_bytes())
        self.assertEqual(checks['retainedShadcnProvenanceVerified']['originalSourceFilesVerified'], 5)
        with zipfile.ZipFile(result['archive']) as archive:
            prefix = archive.namelist()[0].split('/')[0] + '/'
            self.assertEqual(archive.read(prefix + SHADCN_ROOT + 'LICENSE'), (source_root / SHADCN_ROOT / 'LICENSE').read_bytes())

    def test_shadcn_missing_or_changed_provenance_refuses_before_export_writes(self):
        self.add_shadcn()
        for name in [SHADCN_MANIFEST, SHADCN_ROOT + 'LICENSE', SHADCN_ROOT + 'upstream/button.tsx', 'desktop/src/components/ui/button.tsx']:
            original = (self.repo / name).read_bytes()
            for mode in ('missing', 'changed'):
                with self.subTest(name=name, mode=mode):
                    try:
                        if mode == 'missing':
                            (self.repo / name).unlink()
                        else:
                            self.write(name, original + b'\nchanged\n')
                        self.commit()
                        with self.assertRaisesRegex(exporter.ExportError, 'shadcn'):
                            self.export()
                        self.assertFalse((self.root / 'candidate').exists())
                    finally:
                        self.write(name, original)

    def test_shadcn_malicious_manifest_cannot_select_another_file(self):
        self.add_shadcn()
        manifest = json.loads((self.repo / SHADCN_MANIFEST).read_bytes())
        manifest['files'][0]['sourcePath'] = '../private-file'
        self.write(SHADCN_MANIFEST, exporter.json_bytes(manifest))
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, 'shadcn'):
            self.export()
        self.assertFalse((self.root / 'candidate').exists())

    def test_exact_bytes_modes_selection_and_checksums(self):
        result = self.export()
        output = Path(result["output"])
        self.assertEqual((output / "third-party/licenses/fixture/LICENSE").read_bytes(), self.grant)
        for name in (".git", "docs/legacy", "desktop/.env.local", "desktop/src/seams.test.ts"):
            self.assertFalse((output / name).exists(), name)
        self.assertTrue((output / "desktop/src/active.test.ts").is_file())
        checks = json.loads((output / "EXPORT-CHECKS.json").read_bytes())
        self.assertFalse(checks["fullCleanRebuildRerun"])
        self.assertFalse(checks["publicPublicationAuthorizedOrPerformed"])
        with zipfile.ZipFile(result["archive"]) as bundle:
            self.assertEqual(bundle.getinfo("candidate/scripts/build-release.sh").external_attr >> 16 & 0o777, 0o755)
            self.assertEqual(bundle.read("candidate/third-party/licenses/fixture/LICENSE"), self.grant)
        self.assertEqual(exporter.sha(Path(result["archive"]).read_bytes()), result["archiveSha256"])
        for row in (output / "EXPORT-SHA256SUMS").read_text().splitlines():
            digest, name = row.split("  ", 1)
            self.assertEqual(exporter.sha((output / name).read_bytes()), digest)
        with self.assertRaisesRegex(exporter.ExportError, "already exists"):
            self.export()

    def test_plan_writes_nothing(self):
        self.assertTrue(self.export(plan=True)["planOnly"])
        self.assertFalse((self.root / "candidate").exists())

    def test_manifest_bound_runtime_assets_survive_source_zip_without_dist_or_private_leaks(self):
        extras = ["desktop/dist/build.js", exporter.SUPABASE_DIST + "unlisted.js",
                  exporter.SUPABASE_DIST + ".env", exporter.SUPABASE_DIST + ".aws/credentials",
                  exporter.SUPABASE_ROOT + "unlisted.txt"]
        for name in extras:
            self.write(name, b"excluded fixture\n")
        self.commit()
        result = self.export()
        output = Path(result["output"])
        checks = json.loads((output / "EXPORT-CHECKS.json").read_bytes())
        self.assertEqual(checks["retainedSupabaseAssetsVerified"], len(self.assets))
        with zipfile.ZipFile(result["archive"]) as bundle:
            for name, data in self.assets.items():
                path = exporter.SUPABASE_ROOT + name
                self.assertEqual((output / path).read_bytes(), data)
                self.assertEqual(bundle.read("candidate/" + path), data)
            for name in extras:
                self.assertFalse((output / name).exists(), name)
                self.assertNotIn("candidate/" + name, bundle.namelist())

    def test_missing_or_changed_manifest_runtime_asset_refuses_export_before_writes(self):
        name = exporter.SUPABASE_DIST + "webapi/index.js"
        self.write(name, b"changed runtime bytes\n")
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained Supabase asset"):
            self.export()
        self.assertFalse((self.root / "candidate").exists())
        self.run_git("rm", "--", name)
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained Supabase asset"):
            self.export()

    def test_manifest_cannot_exempt_private_paths_or_other_dist_outputs(self):
        for relative in ["volumes/functions/_vendor/jose/dist/.env",
                         "volumes/functions/_vendor/jose/dist/.ssh/key",
                         "volumes/functions/_vendor/jose/dist/../outside.js",
                         "dist/unreviewed.js"]:
            with self.subTest(relative=relative):
                manifest = {**self.asset_manifest, "files": [*self.asset_manifest["files"],
                    {"path": relative, "bytes": 1, "sha256": exporter.sha(b"x")}]}
                self.write(exporter.SUPABASE_MANIFEST, exporter.json_bytes(manifest))
                self.commit()
                with self.assertRaisesRegex(exporter.ExportError, "Unsafe or invalid retained"):
                    self.export()

    def test_pinned_jose_parser_literal_is_exact_blob_only_not_a_private_key_exception(self):
        name = exporter.SUPABASE_DIST + "webapi/key/import.js"
        data = (Path(__file__).resolve().parent.parent / name).read_bytes()
        relative = name.removeprefix(exporter.SUPABASE_ROOT)
        row = {"path": relative, "bytes": len(data), "sha256": exporter.sha(data)}
        self.write(name, data)
        self.write(exporter.SUPABASE_MANIFEST, exporter.json_bytes({
            **self.asset_manifest, "files": [*self.asset_manifest["files"], row],
        }))
        self.commit()
        result = self.export()
        checks = json.loads((Path(result["output"]) / "EXPORT-CHECKS.json").read_bytes())
        finding = next(item for item in checks["credentialSignatureFindings"] if item["path"] == name)
        self.assertEqual(finding["sourceBlobSha256"], exporter.sha(data))
        with zipfile.ZipFile(result["archive"]) as bundle:
            self.assertEqual(bundle.read("candidate/" + name), data)
        # Updating the retained manifest must not extend the immutable parser exception.
        changed = data + b"\n// altered parser blob\n"
        self.write(name, changed)
        row.update(bytes=len(changed), sha256=exporter.sha(changed))
        self.write(exporter.SUPABASE_MANIFEST, exporter.json_bytes({
            **self.asset_manifest, "files": [*self.asset_manifest["files"], row],
        }))
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export("changed-parser")
        self.assertFalse((self.root / "changed-parser").exists())

    def test_fluid_grant_and_normalized_source_survive_zip_with_exact_bytes(self):
        name, source, grant = self.add_fluid()
        self.write(exporter.FLUID_ROOT + "unreviewed.txt", b"excluded\n")
        self.commit()
        result = self.export()
        output = Path(result["output"])
        checks = json.loads((output / "EXPORT-CHECKS.json").read_bytes())
        self.assertEqual(checks["retainedFluidProvenanceVerified"]["adaptedSourceFilesVerified"], 1)
        self.assertTrue(checks["retainedFluidProvenanceVerified"]["present"])
        with zipfile.ZipFile(result["archive"]) as bundle:
            self.assertEqual(bundle.read("candidate/" + name), source)
            self.assertEqual(bundle.read("candidate/" + exporter.FLUID_ROOT + "LICENSE"), grant)
            self.assertIn("candidate/" + exporter.FLUID_MANIFEST, bundle.namelist())
            self.assertNotIn("candidate/" + exporter.FLUID_ROOT + "unreviewed.txt", bundle.namelist())

    def test_fluid_source_cannot_silently_omit_manifest_or_original_grant(self):
        self.add_fluid()
        self.commit()
        for name in [exporter.FLUID_MANIFEST, exporter.FLUID_ROOT + "LICENSE"]:
            with self.subTest(name=name):
                data = (self.repo / name).read_bytes()
                self.run_git("rm", "--", name)
                self.commit()
                with self.assertRaisesRegex(exporter.ExportError, "requires its manifest and original MIT grant"):
                    self.export()
                self.assertFalse((self.root / "candidate").exists())
                self.write(name, data)
                self.commit()

    def test_changed_fluid_source_and_grant_refuse_export_before_writes(self):
        name, source, grant = self.add_fluid()
        self.write(name, source + b"// unrecorded change\n")
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained Fluid source"):
            self.export()
        self.assertFalse((self.root / "candidate").exists())
        self.write(name, source)
        self.write(exporter.FLUID_ROOT + "LICENSE", grant + b"changed grant\n")
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained Fluid MIT grant"):
            self.export()
        self.assertFalse((self.root / "candidate").exists())

    def test_owned_docker_distribution_inputs_survive_source_zip(self):
        packaging_spec = importlib.util.spec_from_file_location(
            "docker_packaging", Path(__file__).with_name("prepare-docker-release.py"))
        packaging = importlib.util.module_from_spec(packaging_spec)
        packaging_spec.loader.exec_module(packaging)
        self.assertEqual(exporter.DOCKER_PACKAGING_FILES, packaging.PACKAGING_FILES)
        expected = {}
        for relative in packaging.PACKAGING_FILES:
            name = "distribution/docker/" + relative
            expected[name] = ("Synthetic owned Docker input: " + relative + "\r\n").encode()
            self.write(name, expected[name])
        extras = ["distribution/private.txt", "distribution/docker/dist/build.js",
                  "distribution/docker/artifacts/context.tar", "distribution/docker/.env",
                  "distribution/docker/.aws/credentials"]
        for name in extras:
            self.write(name, b"excluded\n")
        self.run_git("add", "--all")
        self.run_git("update-index", "--chmod=+x", "distribution/docker/start-luci.sh")
        self.commit()
        result = self.export()
        with zipfile.ZipFile(result["archive"]) as bundle:
            for name, data in expected.items():
                self.assertEqual(bundle.read("candidate/" + name), data)
            for name in extras:
                self.assertNotIn("candidate/" + name, bundle.namelist())
            self.assertEqual(bundle.getinfo("candidate/distribution/docker/start-luci.sh").external_attr >> 16 & 0o777, 0o755)

    def test_missing_owned_docker_distribution_input_refuses_before_writes(self):
        self.run_git("rm", "--", "distribution/docker/workspace-init.sh")
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Required source files are missing.*workspace-init.sh"):
            self.export()
        self.assertFalse((self.root / "candidate").exists())

    def test_native_installer_hook_survives_export_at_its_configured_path(self):
        hook_name = "desktop/src-tauri/windows/hooks.nsh"
        hook = b"!macro NSIS_HOOK_PREUNINSTALL\n  ClearErrors\n!macroend\n"
        self.write(hook_name, hook)
        self.write("desktop/src-tauri/tauri.conf.json", exporter.json_bytes({
            "version": "0.4.0",
            "bundle": {"windows": {"nsis": {"installerHooks": "./windows/hooks.nsh"}}},
        }))
        self.commit()
        result = self.export()
        output = Path(result["output"])
        config_path = output / "desktop/src-tauri/tauri.conf.json"
        config = json.loads(config_path.read_bytes())
        self.assertEqual((config_path.parent / config["bundle"]["windows"]["nsis"]["installerHooks"]).read_bytes(), hook)
        with zipfile.ZipFile(result["archive"]) as bundle:
            self.assertEqual(bundle.read("candidate/" + hook_name), hook)

    def test_dirty_and_wrong_revision_refused(self):
        self.write("new-untracked.txt", b"uncommitted\n")
        with self.assertRaisesRegex(exporter.ExportError, "dirty"):
            self.export()
        old_revision = self.revision
        self.commit()
        self.revision = old_revision
        with self.assertRaisesRegex(exporter.ExportError, "HEAD"):
            self.export()

    def test_changed_grant_refused(self):
        self.write("third-party/licenses/fixture/LICENSE", self.grant.replace(b"\r\n", b"\n"))
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained third-party grant"):
            self.export()

    def test_secrets_and_version_mismatch_refused(self):
        self.write("desktop/src/key.ts", b"ghp_" + b"A" * 36)
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export()
        self.write("desktop/src/key.ts", b"removed\n")
        self.write("desktop/package.json", b'{"version":"0.5.0","license":"MIT"}')
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "version mismatch"):
            self.export()

    def test_unsafe_paths_and_symlinks_refused(self):
        self.assertFalse(exporter.safe_path("desktop/../private"))
        self.assertFalse(exporter.safe_path("desktop/con.txt"))
        # update-index creates a tracked symlink on Windows without requiring OS symlink privileges.
        blob = subprocess.run(["git", "-C", str(self.repo), "hash-object", "-w", "--stdin"],
                              input=b"../outside", check=True, stdout=subprocess.PIPE).stdout.decode().strip()
        self.run_git("update-index", "--add", "--cacheinfo", "120000," + blob + ",desktop/escape")
        self.run_git("commit", "--quiet", "-m", "Symlink fixture")
        revision = self.run_git("rev-parse", "HEAD").decode().strip()
        with self.assertRaisesRegex(exporter.ExportError, "symlink"):
            exporter.read_source(self.repo, revision)

    def test_synthetic_key_exception_is_exact_and_recorded(self):
        name = "control-plane/internal/api/sessions_agent_credentials_test.go"
        value = b"sk-ant-" + b"fixture-personal-key-do-not-serialize-123456"
        self.write(name, b'key := "' + value + b'"\n')
        self.commit()
        result = self.export()
        checks = json.loads((Path(result["output"]) / "EXPORT-CHECKS.json").read_bytes())
        self.assertEqual(checks["credentialSignatureFindings"][0]["matchedValueSha256"], exporter.sha(value))
        self.write(name, b'key := "' + value[:-1] + b'7"\n')
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export("changed-value")
        self.write(name, b'key := "' + value + b'"\nsecond := "' + value + b'"\n')
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export("second-value")
        self.write(name, b"removed\n")
        self.write("desktop/src/unrelated.ts", value)
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export("wrong-path")


if __name__ == "__main__":
    unittest.main()
