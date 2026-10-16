"""No-module, no-render source and refusal checks for the held pilot."""

import json
import os
from pathlib import Path
import re
import tempfile
import unittest

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


if __name__ == "__main__":
    unittest.main()
