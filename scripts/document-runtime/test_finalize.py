"""Inert source regressions for the maintainer-only document finalizer."""

import importlib.util
import gzip
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
from types import SimpleNamespace
import unittest


SOURCE = Path(__file__).with_name("finalize.py")
SPEC = importlib.util.spec_from_file_location("document_runtime_finalize", SOURCE)
FINALIZER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(FINALIZER)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def tar_bytes(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for name, data in entries:
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(data), 0o444, 0
            archive.addfile(info, io.BytesIO(data))
    return output.getvalue()


def fixture(root):
    """Model writeOCITar: four sorted metadata entries then one small layer."""
    commit, source, retention, closure = "5" * 40, "6" * 64, "7" * 64, "2" * 64
    tool, notice = b"print('extract protocol 1')\n", b"Original notice and grant text\n"
    assembly = {"version": 1, "profile": "document-flat-oci-v1", "sourceCommit": commit,
                "sourceDigest": source, "sourceRetentionDigest": retention,
                "sbomClosed": True, "authenticated": True, "distributionNoticesClosed": True,
                "extractor": {"digest": digest(tool), "bytes": len(tool)},
                "notices": [{"path": "notices/NOTICE.txt", "digest": digest(notice), "bytes": len(notice)}]}
    layer = tar_bytes([
        ("opt/luci/assembly-manifest.json", FINALIZER.encoded(assembly)),
        ("opt/luci/document_extractor.py", tool),
        ("opt/luci/licenses/NOTICE.txt", notice),
    ])
    compressed = gzip.compress(layer, mtime=0)
    assert len(compressed) < FINALIZER.MAX_META
    config = {"architecture": "amd64", "os": "linux", "config": {
        "User": "1001:1001", "WorkingDir": "/tmp", "Entrypoint": FINALIZER.ENTRY,
        "Labels": {"org.opencontainers.image.revision": commit, "luci.document-source": source,
                   "luci.document-input-manifest": "8" * 64, "luci.document-source-retention": retention}},
        "rootfs": {"type": "layers", "diff_ids": ["sha256:" + digest(layer)]}}
    config_raw = FINALIZER.encoded(config)
    descriptor = lambda media, data: {"mediaType": media, "digest": "sha256:" + digest(data), "size": len(data)}
    layer_desc = descriptor("application/vnd.oci.image.layer.v1.tar+gzip", compressed)
    manifest_raw = FINALIZER.encoded({"schemaVersion": 2, "config": descriptor("application/vnd.oci.image.config.v1+json", config_raw), "layers": [layer_desc]})
    index_raw = FINALIZER.encoded({"schemaVersion": 2, "manifests": [descriptor("application/vnd.oci.image.manifest.v1+json", manifest_raw)]})
    metadata = {"oci-layout": b'{"imageLayoutVersion":"1.0.0"}', "index.json": index_raw,
                "blobs/sha256/" + digest(config_raw): config_raw,
                "blobs/sha256/" + digest(manifest_raw): manifest_raw}
    capture = tar_bytes([(name, metadata[name]) for name in sorted(metadata)] +
                        [("blobs/sha256/" + digest(compressed), compressed)])
    receipt = {"captureExitVerified": True, "outcome": {
        "state": "assembled_owned_resources_retired_not_registered", "exitCode": 0,
        "oomKilled": False, "readerSettled": True, "watchdogSettled": True,
        "cleanupConfirmed": True, "resourcesHeld": False,
        "capture": {"digest": digest(capture), "bytes": len(capture)},
        "inputClosureRef": {"schema": 1, "digest": closure},
        "identity": {"protocol": 1, "profile": "document-flat-oci-v1", "sourceCommit": commit,
                     "sourceDigest": source, "inputDigest": "8" * 64,
                     "helperDigest": "9" * 64, "recipeDigest": "a" * 64}}}
    notices = {"schema": 1, "kind": "luci-document-notice-source-closure-v1",
               "inputClosureSha256": closure, "sourceRetentionDigest": retention,
               "notices": [{"path": "notices/NOTICE.txt", "sha256": digest(notice),
                            "bytes": len(notice), "component": "fixture", "version": "1",
                            "grant": "retained original notice", "provenance": "fixture source"}]}
    capture_path, receipt_path, notice_path = (root / name for name in ("capture.oci.partial", "outcome.json", "notice-closure.json"))
    capture_path.write_bytes(capture)
    receipt_raw, notice_raw = FINALIZER.encoded(receipt), FINALIZER.encoded(notices)
    receipt_path.write_bytes(receipt_raw)
    notice_path.write_bytes(notice_raw)
    return SimpleNamespace(capture=str(capture_path), capture_sha256=digest(capture),
                           assembly_receipt=str(receipt_path), assembly_receipt_sha256=digest(receipt_raw),
                           notice_closure=str(notice_path), notice_closure_sha256=digest(notice_raw),
                           output_dir=str(root / "candidate")), notices


class DocumentFinalizerSourceTest(unittest.TestCase):
    def test_sidecar_names_are_portable_ordinary_paths(self):
        self.assertTrue(FINALIZER.notice_name("notices/component/LICENCE.txt"))
        for name in ("notices/NOTICE:stream", "notices/CON.txt", "notices/aux/a",
                     "notices/item.", "notices/item ", "notices/../other"):
            with self.subTest(name=name):
                self.assertFalse(FINALIZER.notice_name(name))

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

    def test_bare_retained_oci_capture_produces_verified_notice_sidecar(self):
        with tempfile.TemporaryDirectory() as temporary:
            args, notices = fixture(Path(temporary))
            FINALIZER.finalize(args)
            candidate = Path(args.output_dir)
            sidecar = candidate / "notices/NOTICE.txt"
            self.assertEqual(sidecar.read_bytes(), b"Original notice and grant text\n")
            manifest = json.loads((candidate / "document-runtime-manifest-v1.json").read_bytes())
            receipt = json.loads((candidate / "document-runtime-finalizer-receipt-v1.json").read_bytes())
            self.assertEqual(manifest["provenance"]["captureSha256"], args.capture_sha256)
            self.assertEqual(receipt["noticeSidecars"], [{"path": "notices/NOTICE.txt",
                "sha256": notices["notices"][0]["sha256"], "bytes": len(sidecar.read_bytes())}])
            self.assertEqual(receipt["releaseAuthority"], "none")

    def test_changed_capture_and_wrong_notice_are_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            args, notices = fixture(Path(temporary))
            capture = Path(args.capture)
            raw = bytearray(capture.read_bytes())
            raw[-1] ^= 1
            capture.write_bytes(raw)
            with self.assertRaises(ValueError):
                FINALIZER.finalize(args)
        with tempfile.TemporaryDirectory() as temporary:
            args, notices = fixture(Path(temporary))
            notices["notices"][0]["sha256"] = "f" * 64
            changed = FINALIZER.encoded(notices)
            Path(args.notice_closure).write_bytes(changed)
            args.notice_closure_sha256 = digest(changed)
            with self.assertRaises(ValueError):
                FINALIZER.finalize(args)
            self.assertFalse((Path(args.output_dir) / "notices/NOTICE.txt").exists())


if __name__ == "__main__":
    unittest.main()
