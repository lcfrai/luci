"""Source regressions for packaging identity; fixture bytes are not real fonts."""
import hashlib
import json
import pathlib
import tempfile
import unittest

from retained_pdf_fonts import PDF_FONTS, SCHEMA, verify_pdf_fonts


class PDFBundleIdentityTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.directory = self.root / PDF_FONTS
        self.directory.mkdir(parents=True)
        self.manifest = {"schema": SCHEMA, "fonts": [], "notices": []}
        for role in ("latin", "arabic", "cjk"):
            body = ("identity-fixture-" + role).encode()
            name = role + ".ttf"
            (self.directory / name).write_bytes(body)
            self.manifest["fonts"].append({
                "id": role, "file": name, "bytes": len(body),
                "sha256": hashlib.sha256(body).hexdigest(), "faceIndex": 0,
                "outline": "truetype", "license": "OFL-1.1",
            })
        notice = b"Licence identity fixture, not a grant."
        (self.directory / "LICENSE.txt").write_bytes(notice)
        self.manifest["notices"].append({
            "file": "LICENSE.txt", "bytes": len(notice),
            "sha256": hashlib.sha256(notice).hexdigest(),
        })
        self.save()

    def save(self):
        (self.directory / "manifest.json").write_text(
            json.dumps(self.manifest), encoding="utf-8"
        )

    def test_complete_exact_bundle_identity(self):
        result = verify_pdf_fonts(self.root)
        self.assertEqual(result["manifest"], self.manifest)
        self.assertEqual(result["directory"], self.directory)

    def test_font_and_original_notice_tampering_refused(self):
        for filename in ("latin.ttf", "LICENSE.txt"):
            with self.subTest(filename=filename):
                path = self.directory / filename
                original = path.read_bytes()
                path.write_bytes(bytes([original[0] ^ 1]) + original[1:])
                with self.assertRaisesRegex(ValueError, "bytes changed"):
                    verify_pdf_fonts(self.root)
                path.write_bytes(original)

    def test_extra_entry_refused(self):
        (self.directory / "unlisted.ttf").write_bytes(b"unknown")
        with self.assertRaisesRegex(ValueError, "Unexpected"):
            verify_pdf_fonts(self.root)

    def test_missing_role_and_unknown_record_fields_refused(self):
        original = self.manifest["fonts"][2].copy()
        self.manifest["fonts"][2]["id"] = "latin"
        self.save()
        with self.assertRaisesRegex(ValueError, "roles"):
            verify_pdf_fonts(self.root)
        self.manifest["fonts"][2] = dict(original, fallbackPath="C:/Windows/Fonts")
        self.save()
        with self.assertRaisesRegex(ValueError, "Unexpected"):
            verify_pdf_fonts(self.root)

    def test_path_escape_refused_before_read(self):
        self.manifest["fonts"][0]["file"] = "../outside.ttf"
        self.save()
        with self.assertRaisesRegex(ValueError, "filename"):
            verify_pdf_fonts(self.root)

    def test_unsupported_outline_and_boolean_index_refused(self):
        for change in ({"outline": "cff2"}, {"faceIndex": True}, {"license": "MIT"}):
            with self.subTest(change=change):
                original = self.manifest["fonts"][0].copy()
                self.manifest["fonts"][0].update(change)
                self.save()
                with self.assertRaisesRegex(ValueError, "Unsupported"):
                    verify_pdf_fonts(self.root)
                self.manifest["fonts"][0] = original

    def test_missing_bundle_refused(self):
        (self.directory / "manifest.json").unlink()
        with self.assertRaisesRegex(ValueError, "regular"):
            verify_pdf_fonts(self.root)


if __name__ == "__main__":
    unittest.main()
