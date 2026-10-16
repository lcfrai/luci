"""No-module, no-render source and refusal checks for the held pilot."""

import json
import os
from pathlib import Path
import re
import tempfile
import unittest
import hashlib
import zipfile
from unittest.mock import patch

import preflight
import verify_pdf


ROOT = Path(__file__).resolve().parent


class PilotSourceTests(unittest.TestCase):
    def test_manifest_matches_isolated_go_requirements(self):
        manifest = json.loads((ROOT / "inputs-v1.json").read_text())
        source = (ROOT / "go.mod").read_text()
        requirements = dict(re.findall(r"^\s*([^\s()]+) (v[^\s]+)$", source, re.MULTILINE))
        selected = {m["name"]: m["version"] for m in manifest["modules"]}
        self.assertEqual(requirements, selected)
        self.assertEqual(manifest["faceIndex"], 0)
        self.assertEqual(manifest["originalText"], "日本語")
        self.assertEqual(manifest["jobMemoryMaxBytes"], 512 << 20)
        self.assertEqual(manifest["renderDeadlineSeconds"], 30)
        self.assertEqual(manifest["pdfMaxBytes"], 16 << 20)
        self.assertEqual(manifest["intermediateMaxBytes"], 32 << 20)
        self.assertEqual(manifest["moduleZipExpandedMaxBytes"], 64 << 20)
        self.assertEqual(manifest["moduleZipExpandedTotalMaxBytes"], 128 << 20)
        self.assertEqual(manifest["moduleZipEntriesMax"], 4096)
        for module in manifest["modules"]:
            self.assertTrue(module["url"].endswith("/@v/" + module["version"] + ".zip"))
            self.assertTrue(module["url"].startswith("https://proxy.golang.org/"))
            self.assertEqual(len(module["h1"]), 47)
            self.assertTrue(all(len(module[k]["sha256"]) == 64 for k in ("mod", "info")))
            self.assertTrue(all(len(grant["sha256"]) == 64 for grant in module["grants"]))

    def test_missing_inputs_refuse_before_effects(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("LUCI_PDF_PILOT_TMPDIR")) as folder:
            stage = Path(folder) / ("pdf-inputs-" + "a" * 32)
            stage.mkdir()
            with self.assertRaises((OSError, ValueError)):
                preflight.preflight(stage, "linux-amd64")
            self.assertEqual(list(stage.iterdir()), [])

    def test_reader_rejects_missing_xref_and_non_cff(self):
        for raw in (b"", b"%PDF-1.7\ntrailer\n%%EOF\n", b"%PDF-1.7\n/CIDFontType2\nstartxref\n3\n%%EOF\n"):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                verify_pdf.verify(raw)

    def test_reader_input_is_bounded_before_pdf_parsing(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("LUCI_PDF_PILOT_TMPDIR")) as folder:
            stage = Path(folder) / ("pilot-" + "b" * 32)
            stage.mkdir()
            target = stage / "pilot.pdf"
            target.write_bytes(b"123456789")
            with patch.object(verify_pdf, "MAX_PDF", 8), self.assertRaisesRegex(ValueError, "oversized"):
                verify_pdf.read_owned_pdf(stage)
            target.unlink()
            outside = Path(folder) / "outside.pdf"
            outside.write_bytes(b"small")
            target.symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "oversized"):
                verify_pdf.read_owned_pdf(stage)

    def test_bounded_original_read_rejects_oversize_and_link(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("LUCI_PDF_PILOT_TMPDIR")) as folder:
            root = Path(folder)
            (root / "input").write_bytes(b"sixteen characters")
            with self.assertRaisesRegex(ValueError, "oversized"):
                preflight.regular(root, "input", 8)
            (root / "link").symlink_to(root / "input")
            with self.assertRaisesRegex(ValueError, "link"):
                preflight.regular(root, "link", 32)

    def test_module_h1_uses_exact_zip_members_and_rejects_unsafe_names(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("LUCI_PDF_PILOT_TMPDIR")) as folder:
            root = Path(folder)
            module = {"name": "example.com/pilot", "version": "v1.0.0", "input": "pilot.zip",
                      "h1": "h1:6KaR5zTUH6NBmlemR5O37UQDM/or6nySF/n5mk8OQOg="}

            def write_zip(second_name):
                with zipfile.ZipFile(root / "pilot.zip", "w", zipfile.ZIP_DEFLATED) as archive:
                    archive.writestr("example.com/pilot@v1.0.0/a.txt", b"alpha")
                    archive.writestr(second_name, b"beta")
                return hashlib.sha256((root / "pilot.zip").read_bytes()).hexdigest()

            original = write_zip("example.com/pilot@v1.0.0/b.txt")
            row = preflight.module_zip_h1(root, module, 1024, 1024, 8, original)
            self.assertEqual((row["h1"], row["expandedBytes"], row["entries"]), (module["h1"], 9, 2))
            with self.assertRaisesRegex(ValueError, "h1 differs"):
                preflight.module_zip_h1(root, {**module, "h1": "h1:" + "A" * 44}, 1024, 1024, 8, original)
            changed = write_zip("example.com/pilot@v1.0.0/../b.txt")
            with self.assertRaisesRegex(ValueError, "outside exact module"):
                preflight.module_zip_h1(root, module, 1024, 1024, 8, changed)
            with self.assertRaisesRegex(ValueError, "changed after"):
                preflight.module_zip_h1(root, module, 1024, 1024, 8, original)

    def test_go_mod_h1_is_separate_from_raw_sha(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("LUCI_PDF_PILOT_TMPDIR")) as folder:
            root = Path(folder)
            raw = b"module example.com/pilot\n"
            (root / "pilot.mod").write_bytes(raw)
            metadata = {"input": "pilot.mod", "bytes": 25,
                        "sha256": "06fb02169f83c3932204ffda198b09f07d173953a964bd5281694745c23a0e9b"}
            self.assertEqual(preflight.module_mod_h1(root, metadata),
                             "h1:qeid9ljA2Oid6LSmdC/6A8nKCiQK0WjG21wWadnI89M=")
            with self.assertRaisesRegex(ValueError, "changed after"):
                preflight.module_mod_h1(root, {**metadata, "bytes": 24})


if __name__ == "__main__":
    unittest.main()
