import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

import archive


class ArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        layer = io.BytesIO()
        with tarfile.open(fileobj=layer, mode="w") as output:
            info = tarfile.TarInfo("hello.txt")
            info.size = 5
            output.addfile(info, io.BytesIO(b"hello"))
        self.layer = layer.getvalue()
        self.diff = hashlib.sha256(self.layer).hexdigest()
        self.config = {"os": "linux", "architecture": "amd64", "config": {"User": "dev", "WorkingDir": "/home/dev", "Entrypoint": ["/usr/local/bin/entrypoint.sh"], "ExposedPorts": {"8080/tcp": {}}}, "rootfs": {"type": "layers", "diff_ids": ["sha256:" + self.diff]}}

    def raw(self, extra=None, config=None, manifests=None):
        raw = archive.encoded(config or self.config)
        image_id = "sha256:" + hashlib.sha256(raw).hexdigest()
        values = {"blobs/config": raw, "blobs/layer": self.layer,
                  "manifest.json": archive.encoded(manifests or [{"Config": "blobs/config", "Layers": ["blobs/layer"], "RepoTags": ["unrelated:must-not-import"]}]),
                  "index.json": b"{}"}
        values.update(extra or {})
        source = self.root / "raw.tar"
        with tarfile.open(source, "w") as output:
            for name, content in values.items():
                info = tarfile.TarInfo(name)
                info.size = len(content)
                output.addfile(info, io.BytesIO(content))
        return source, image_id

    def test_normalization_keeps_config_and_layers_but_removes_tags_and_auxiliary(self):
        source, image_id = self.raw()
        result = archive.normalize(source, self.root / "normalized.tar", image_id)
        self.assertEqual(result["image"]["id"], image_id)
        self.assertEqual(result["image"]["repoTags"], [])
        self.assertEqual(len(result["members"]), 3)
        self.assertEqual(result["members"], sorted(result["members"], key=lambda x: x["path"]))
        self.assertTrue(archive.verify(self.root / "normalized.tar", result["image"], result["members"]))
        result["members"][0]["sha256"] = "0" * 64
        with self.assertRaises(ValueError):
            archive.verify(self.root / "normalized.tar", result["image"], result["members"])

    def test_wrong_config_or_layer_identity_refused(self):
        source, image_id = self.raw()
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "bad.tar", "sha256:" + "0" * 64)
        self.layer = b"changed layer"
        source, image_id = self.raw()
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "bad-layer.tar", image_id)

    def test_unsafe_auxiliary_and_multiple_images_refused(self):
        source, image_id = self.raw({"../escape": b"bad"})
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "bad.tar", image_id)
        source, image_id = self.raw(manifests=[{}, {}])
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "two.tar", image_id)

    def test_execution_labels_and_duplicate_json_refused(self):
        self.config["config"]["Labels"] = {"luci.instance": "some-other-workspace"}
        source, image_id = self.raw()
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "owned.tar", image_id)
        with self.assertRaises(ValueError):
            archive.json_bytes(b'{"same":1,"same":2}')

    def test_linked_outer_archive_member_refused(self):
        source, image_id = self.raw()
        with tarfile.open(source, "a") as output:
            info = tarfile.TarInfo("link")
            info.type, info.linkname = tarfile.SYMTYPE, "manifest.json"
            output.addfile(info)
        with self.assertRaises(ValueError):
            archive.normalize(source, self.root / "linked.tar", image_id)


if __name__ == "__main__":
    unittest.main()
