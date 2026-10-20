"""Source regressions for the verifier; no package or inspected module executes."""
import base64
import hashlib
import importlib.util
import os
from pathlib import Path
import sys
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("document_installed_verifier", Path(__file__).with_name("verify_image.py"))
verifier = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(verifier)


class VerifierPureTests(unittest.TestCase):
    def test_paths_are_confined_without_rewriting(self):
        self.assertTrue(verifier.safe_path("usr/share/fonts/日本語.ttf"))
        for value in ("", "/etc/passwd", "../x", "a/../x", "a//x", "a/./x", "a\\x", "a:x", "a\n", "a\0", "a"*1025):
            with self.subTest(value=repr(value)):
                self.assertFalse(verifier.safe_path(value))

    def test_generated_metadata_never_exempts_an_etc_or_var_tree(self):
        packages = {"dpkg", "libc6"}
        for path in ("var/lib/dpkg/status", "var/lib/dpkg/info/libc6:amd64.list", "etc/ld.so.cache"):
            self.assertTrue(verifier.generated_path(path, packages))
        for path in ("etc/shadow.evil", "etc/ld.so.preload", "var/cache/payload", "var/lib/dpkg/info/not-selected.postinst", "var/lib/dpkg/info/libc6.arbitrary"):
            self.assertFalse(verifier.generated_path(path, packages))

    def test_pyc_requires_the_exact_observed_immutable_python_source(self):
        row = {"kind": "regular", "policy": "", "bytes": 5}
        inventory = {"usr/lib/python3.11/example.py": row}
        self.assertEqual(verifier.pyc_source("usr/lib/python3.11/__pycache__/example.cpython-311.pyc", inventory), "usr/lib/python3.11/example.py")
        self.assertIsNone(verifier.pyc_source("usr/lib/python3.11/__pycache__/unseen.cpython-311.pyc", inventory))
        self.assertIsNone(verifier.pyc_source("usr/lib/python3.11/__pycache__/example.cpython-312.pyc", inventory))

    def test_library_alias_resolution_uses_only_immutable_paths(self):
        links = {"lib": "usr/lib", "usr/lib/libexample.so": "libexample.so.1"}
        self.assertEqual(verifier.resolve_inventory_link("lib/libexample.so", links), "usr/lib/libexample.so.1")
        for links in ({"lib": "../../outside"}, {"lib": "lib"}, {"lib": "other", "other": "lib"}):
            with self.assertRaises(verifier.Refused):
                verifier.resolve_inventory_link("lib/example.so", links)

    def test_duplicate_or_nonfinite_json_cannot_supply_closed_input(self):
        for data in (b'{"state":"closed-reviewed","state":"unresolved"}', b'{"x":NaN}', b'{"x":Infinity}'):
            with self.assertRaises(verifier.Refused):
                verifier.decode(data, 1024)

    def test_target_record_mapping_is_exact_and_has_no_location_fallback(self):
        prefix = "usr/local/lib/python3.11/dist-packages/"
        self.assertEqual(verifier.installed_wheel_record_path("pillow/core.py"), prefix+"pillow/core.py")
        self.assertEqual(verifier.installed_wheel_record_path("../../bin/tool"), prefix+"bin/tool")
        for value in ("../../../bin/tool", "/bin/tool", "../../etc/passwd"):
            with self.assertRaises(verifier.Refused):
                verifier.installed_wheel_record_path(value)

    def test_stable_abi_and_glibc_tags_are_checked_for_the_actual_target(self):
        self.assertTrue(verifier.compatible_wheel_tags(["cp39-abi3-manylinux_2_17_x86_64"], 11, 36))
        self.assertFalse(verifier.compatible_wheel_tags(["cp312-abi3-manylinux_2_28_x86_64"], 11, 36))
        for tag in ("cp311-cp311-manylinux_2_17_aarch64", "cp311-cp310-manylinux_2_17_x86_64", "py3-none-any"):
            with self.assertRaises(verifier.Refused):
                verifier.compatible_wheel_tags([tag], 11, 36)


@unittest.skipUnless(sys.platform.startswith("linux") and sys.version_info[:2] == (3, 11), "actual verifier is fixed Linux CPython3.11 assembly-only")
class VerifierConfinedFileTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = Path(self.temp.name)
        self.root = os.open(self.path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)

    def tearDown(self):
        os.close(self.root)
        self.temp.cleanup()

    def write(self, name, data):
        target = self.path/name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)

    def test_read_refuses_a_symlink_ancestor_and_wrong_actual_hash(self):
        self.write("real/file", b"retained")
        (self.path/"alias").symlink_to("real", target_is_directory=True)
        with self.assertRaises(OSError):
            verifier.read_regular(self.root, "alias/file", 100)
        with self.assertRaises(verifier.Refused):
            verifier.read_regular(self.root, "real/file", 100, {"bytes": 8, "digest": "a"*64})
        self.assertEqual(verifier.read_regular(self.root, "real/file", 8), b"retained")

    def test_inventory_hash_refusal_preserves_input_vs_output_distinction(self):
        self.write("usr/bin/example", b"actual")
        row = {"path": "usr/bin/example", "owner": "debian/example", "kind": "regular", "bytes": 6, "sha256": hashlib.sha256(b"actual").hexdigest(), "target": "", "policy": ""}
        verifier.verify_inventory(self.root, [row], {"example"})
        self.write("usr/bin/example", b"edited")
        with self.assertRaises(verifier.Refused):
            verifier.verify_inventory(self.root, [row], {"example"})

    def test_namespace_refuses_new_payload_but_allows_empty_structural_directory(self):
        (self.path/"empty").mkdir()
        verifier.verify_namespace(self.root, {}, set(), set())
        self.write("unexpected", b"payload")
        with self.assertRaises(verifier.Refused):
            verifier.verify_namespace(self.root, {}, set(), set())

    def test_actual_alternative_must_resolve_to_an_inventoried_file(self):
        self.write("usr/bin/example", b"inert")
        self.write("var/lib/dpkg/alternatives/example", b"auto\n/usr/bin/alias\n\n/usr/bin/example\n10\n")
        (self.path/"etc/alternatives").mkdir(parents=True)
        (self.path/"etc/alternatives/example").symlink_to("/usr/bin/example")
        inventory = {"usr/bin/example": {"kind": "regular"}}
        allowed = verifier.verify_alternatives(self.root, inventory)
        self.assertIn("etc/alternatives/example", allowed)
        with self.assertRaises(verifier.Refused):
            verifier.verify_alternatives(self.root, {})

    def test_font_cache_exception_is_exact_location_type_and_name(self):
        self.write("var/cache/fontconfig/"+"a"*32+"-le64.cache-8", b"inert-cache-format-data")
        verifier.verify_font_caches(self.root)
        self.write("var/cache/fontconfig/arbitrary.py", b"untrusted")
        with self.assertRaises(verifier.Refused):
            verifier.verify_font_caches(self.root)

    def test_wheel_record_hashes_are_checked_without_importing_inspected_code(self):
        wheels = []
        inventory = {}
        for name, version in (("pillow", "12.3.0"), ("pypdfium2", "5.14.0")):
            site = "usr/local/lib/python3.11/dist-packages/"
            prefix = name+"-"+version+".dist-info/"
            payload = name+"/__init__.py"
            tag = "py3-none-manylinux_2_17_x86_64"
            members = {payload: b"raise RuntimeError('must never import')\n", prefix+"METADATA": ("Name: "+name+"\nVersion: "+version+"\n\n").encode(), prefix+"WHEEL": ("Wheel-Version: 1.0\nRoot-Is-Purelib: false\nTag: "+tag+"\n").encode(), prefix+"INSTALLER": b"pip\n", prefix+"REQUESTED": b""}
            records = []
            for path, data in members.items():
                self.write(site+path, data)
                inventory[site+path] = {}
                digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).decode().rstrip("=")
                records.append(path+",sha256="+digest+","+str(len(data)))
            records.append(prefix+"RECORD,,")
            self.write(site+prefix+"RECORD", ("\n".join(records)+"\n").encode())
            wheels.append({"id": "wheel/"+name, "version": version, "installedOwner": prefix[:-1], "requiresDist": [], "requiresPython": "", "tags": [tag], "rootIsPurelib": False, "entryPoints": []})
        verifier.verify_wheels(self.root, wheels, inventory)
        self.write("usr/local/lib/python3.11/dist-packages/pillow/__init__.py", b"changed")
        with self.assertRaises(verifier.Refused):
            verifier.verify_wheels(self.root, wheels, inventory)


if __name__ == "__main__":
    unittest.main()
