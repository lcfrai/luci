"""Fixed build verification, never an intake/parser or package resolver.

Source only: this file has not been executed. The current unresolved lock fails
before installation. No network, shell command, forms or document input is used.
"""
import hashlib
import importlib.metadata
import json
from pathlib import Path
import re
import resource
import subprocess
import sys
import tempfile


def fail(message):
    raise SystemExit(message)


def main():
    if len(sys.argv) != 2:
        fail("exact reviewed lock path required")
    path = Path(sys.argv[1])
    if not path.is_file() or path.stat().st_size > 2 * 1024 * 1024:
        fail("lock missing or oversized")
    lock = json.loads(path.read_bytes())
    if lock.get("state") != "closed-reviewed" or lock.get("unresolved"):
        fail("dependency and licence closure is not reviewed")
    expected = {}
    for package in lock["debianPackages"]:
        name, version = package["name"], package["version"]
        if not re.fullmatch(r"[a-z0-9][a-z0-9+.-]{0,127}", name) or name in expected:
            fail("invalid or duplicate package identity")
        expected[name] = version
    # Fixed subprocess inside the separately bounded image build. No caller
    # executable/argv, shell, network or document controls this verification.
    def bound_query_file():
        resource.setrlimit(resource.RLIMIT_FSIZE, (1024 * 1024, 1024 * 1024))

    # Single-threaded Linux build verifier: exact child, finite file output and
    # subprocess.run's timeout kills/waits that owned child. No unbounded pipe.
    with tempfile.TemporaryFile() as output:
        subprocess.run(
            ["/usr/bin/dpkg-query", "-W", "-f=${binary:Package}\t${Version}\n"],
            check=True, stdout=output, stderr=subprocess.DEVNULL, timeout=10,
            preexec_fn=bound_query_file, env={"PATH": "/usr/bin:/bin"},
        )
        output.seek(0)
        observed = output.read(1024 * 1024 + 1)
    if len(observed) > 1024 * 1024:
        fail("installed package inventory exceeds bound")
    actual = {}
    for row in observed.decode("utf-8", "strict").splitlines():
        name, version = row.split("\t", 1)
        name = name.removesuffix(":amd64")
        if name in actual:
            fail("duplicate installed package")
        actual[name] = version
    if actual != expected:
        fail("installed package set differs from reviewed complete lock")
    if sys.version_info[:2] != (3, 11):
        fail("Python ABI differs from reviewed wheel")
    import pypdfium2 as pdfium
    import PIL
    if str(pdfium.PYPDFIUM_INFO.api_tag) != "5.14.0" or pdfium.PDFIUM_INFO.build != 8076:
        fail("PDFium/helper version differs from reviewed profile")
    if list(pdfium.PDFIUM_INFO.flags):
        fail("V8/XFA or another non-default PDFium build is not permitted")
    if PIL.__version__ != "12.3.0":
        fail("Pillow version differs from reviewed profile")
    for distribution in ("pypdfium2", "Pillow"):
        if importlib.metadata.distribution(distribution).version != {"pypdfium2": "5.14.0", "Pillow": "12.3.0"}[distribution]:
            fail("installed wheel metadata differs")
    # Filled only from reviewed bounded archive/member inspection, never guessed
    # from an upstream tag. Includes native libraries, trained data and notices.
    files = lock.get("verifiedInstalledFiles", [])
    if not files:
        fail("native libraries, trained data and exact notices are not pinned")
    for record in files:
        member = Path(record["path"])
        if not member.is_absolute() or member.is_symlink() or not member.is_file():
            fail("reviewed installed member missing or unsafe")
        maximum = record["bytes"]
        if not isinstance(maximum, int) or maximum < 1 or maximum > 128 * 1024 * 1024 or member.stat().st_size != maximum:
            fail("reviewed member size changed")
        digest = hashlib.sha256()
        with member.open("rb") as stream:
            for data in iter(lambda: stream.read(65536), b""):
                digest.update(data)
        if digest.hexdigest() != record["sha256"]:
            fail("reviewed member digest changed")


if __name__ == "__main__":
    main()
