"""Source-level bounds/claim tests. These do not prove native PDF/OCR or E2E."""
import importlib.util
import hashlib
from pathlib import Path
import tempfile
import struct
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("document_extractor", Path(__file__).with_name("document_extractor.py"))
EXTRACTOR = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EXTRACTOR)


class DocumentEvidenceBounds(unittest.TestCase):
    def test_live_stream_has_exact_complete_length_digest_and_trailer(self):
        value = {"status": "needs-review", "text": "Source evidence"}
        frame = EXTRACTOR.result_frame(value)
        prefix = b"LUCI-DOCUMENT-RESULT/1\n"
        size = struct.unpack(">I", frame[len(prefix):len(prefix)+4])[0]
        payload_at = len(prefix)+4+32
        payload = frame[payload_at:payload_at+size]
        self.assertEqual(payload, EXTRACTOR.encoded(value))
        self.assertEqual(frame[len(prefix)+4:payload_at], hashlib.sha256(payload).digest())
        self.assertEqual(frame[payload_at+size:], b"\nLUCI-DOCUMENT-END/1\n")
        with self.assertRaises(EXTRACTOR.EvidenceFailure):
            EXTRACTOR.result_frame({"text": "x" * EXTRACTOR.MAX_RESULT})

    def test_complete_atomic_result_and_over_limit_never_publish_partial_ready_data(self):
        with tempfile.TemporaryDirectory() as root:
            output = str(Path(root) / "result.json")
            EXTRACTOR.publish(output, {"status": "needs-review", "text": "Untrusted evidence"})
            before = Path(output).read_bytes()
            with self.assertRaises(EXTRACTOR.EvidenceFailure):
                EXTRACTOR.publish(output, {"text": "x" * EXTRACTOR.MAX_RESULT})
            self.assertEqual(Path(output).read_bytes(), before)

    def test_actual_ocr_tsv_claim_geometry_and_confidence_remain_bounded(self):
        header = "level\tleft\ttop\twidth\theight\tconf\ttext\n"
        valid = (header + "5\t10\t20\t30\t40\t87.5\tFact\n").encode()
        process = mock.Mock(returncode=0)
        with mock.patch.object(EXTRACTOR.subprocess, "Popen", return_value=process), mock.patch.object(EXTRACTOR, "read_regular", return_value=valid):
            claims = EXTRACTOR.ocr("/tmp/ocr-page.png", 1, 100, 100)
        self.assertEqual(claims[0]["location"], {"x": 1000, "y": 2000, "width": 3000, "height": 4000, "firstLine": 0, "lastLine": 0})
        self.assertIn("recognition claim", claims[0]["uncertainty"])
        hostile = (header + "5\t999999999999999999999\t0\t30\t40\t90\tFact\n").encode()
        with mock.patch.object(EXTRACTOR.subprocess, "Popen", return_value=process), mock.patch.object(EXTRACTOR, "read_regular", return_value=hostile):
            with self.assertRaises(EXTRACTOR.EvidenceFailure):
                EXTRACTOR.ocr("/tmp/ocr-page.png", 1, 100, 100)

    def test_pixel_budget_and_null_text_refuse_without_clipping(self):
        EXTRACTOR.checked_pixels(2048, 2048)
        for width, height in ((2049, 2048), (4097, 1), (0, 2)):
            with self.assertRaises(EXTRACTOR.EvidenceFailure):
                EXTRACTOR.checked_pixels(width, height)
        with self.assertRaises(EXTRACTOR.EvidenceFailure):
            EXTRACTOR.segment(1, "text", 1, "bad\0text", {"x": 0, "y": 0, "width": 0, "height": 0})


if __name__ == "__main__":
    unittest.main()
