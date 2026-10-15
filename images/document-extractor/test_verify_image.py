"""Inert fail-closed source regressions; no installed native dependencies needed."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import verify_image


class VerifyImageClosureTests(unittest.TestCase):
    def test_unresolved_lock_never_queries_or_imports_native_runtime(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "lock.json"
            path.write_text(json.dumps({"state": "unresolved-source-metadata", "unresolved": ["missing wheel member hashes"]}))
            with patch("sys.argv", ["verify_image.py", str(path)]), patch.object(verify_image.subprocess, "run") as query:
                with self.assertRaisesRegex(SystemExit, "not reviewed"):
                    verify_image.main()
                query.assert_not_called()

    def test_closed_label_cannot_hide_unresolved_dependencies(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "lock.json"
            path.write_text(json.dumps({"state": "closed-reviewed", "unresolved": ["missing notices"]}))
            with patch("sys.argv", ["verify_image.py", str(path)]), patch.object(verify_image.subprocess, "run") as query:
                with self.assertRaisesRegex(SystemExit, "not reviewed"):
                    verify_image.main()
                query.assert_not_called()


if __name__ == "__main__":
    unittest.main()
