"""Inert source regressions for the maintainer-only document finalizer."""

import importlib.util
from pathlib import Path
import unittest


SOURCE = Path(__file__).with_name("finalize.py")
SPEC = importlib.util.spec_from_file_location("document_runtime_finalize", SOURCE)
FINALIZER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(FINALIZER)


class DocumentFinalizerSourceTest(unittest.TestCase):
    def test_producer_layer_names_keep_directory_and_library_spelling(self):
        self.assertEqual(FINALIZER.layer_name("usr/", True), "usr")
        self.assertEqual(FINALIZER.layer_name("./opt/luci/document_extractor.py", False), "opt/luci/document_extractor.py")
        self.assertEqual(FINALIZER.layer_name("./usr/lib/libstdc++.so.6", False), "usr/lib/libstdc++.so.6")
        for name in ("/opt/luci/tool", "../opt/luci/tool", "opt//luci/tool", "opt/../luci/tool", "opt/./luci/tool", "opt\\luci/tool", "opt/luci/bad\nname"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                FINALIZER.layer_name(name, False)

    def test_unsettled_assembly_receipt_cannot_be_finalized(self):
        receipt = {"captureExitVerified": True, "outcome": {"state": "captured_exit_zero_not_registered", "exitCode": 0}}
        with self.assertRaises(ValueError):
            FINALIZER.exact_receipt(receipt, "a" * 64, 1024)


if __name__ == "__main__":
    unittest.main()
