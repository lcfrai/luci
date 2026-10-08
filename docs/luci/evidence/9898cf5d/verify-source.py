#!/usr/bin/env python3
"""Verify recorded matches AND historical baseline differences, without tests."""
import hashlib
import json
from pathlib import Path
import subprocess

folder = Path(__file__).resolve().parent
repo = folder.parents[3]
binding = json.loads((folder / "source-binding.json").read_text())
commit = binding["commit"]
assert len(commit) == 40 and all(c in "0123456789abcdef" for c in commit)
entries = {}
for record in subprocess.check_output(
    ["git", "ls-tree", "-r", "-z", commit], cwd=repo
).split(b"\0"):
    if record:
        header, name = record.split(b"\t", 1)
        entries[name.decode()] = header.split()[2].decode()
cache, results = {}, {}
with subprocess.Popen(
    ["git", "cat-file", "--batch"], cwd=repo,
    stdin=subprocess.PIPE, stdout=subprocess.PIPE
) as process:
    for scope, info in binding["scopes"].items():
        manifest = json.loads((folder / info["manifest"]).read_text())
        differences = []
        for name, expected in manifest.items():
            identity = entries.get(name)
            if identity is not None and identity not in cache:
                process.stdin.write((identity + "\n").encode())
                process.stdin.flush()
                header = process.stdout.readline().split()
                assert header[1] == b"blob"
                remaining, digest = int(header[2]), hashlib.sha256()
                while remaining:
                    chunk = process.stdout.read(min(remaining, 1 << 20))
                    assert chunk, "Truncated Git blob"
                    remaining -= len(chunk)
                    digest.update(chunk)
                assert process.stdout.read(1) == b"\n"
                cache[identity] = digest.hexdigest()
            actual = cache.get(identity)
            if actual != expected:
                differences.append({
                    "path": name, "testedSHA256": expected, "commitSHA256": actual
                })
        verified = (
            len(manifest) == info["inputCount"]
            and differences == info["differences"]
            and (not differences) == info["allListedBytesMatchCommit"]
            and len(manifest) - len(differences) == info["matchedInputs"]
        )
        results[scope] = {
            "inputs": len(manifest), "recordedDifferences": len(differences),
            "receiptVerified": verified,
        }
    process.stdin.close()
    assert process.wait() == 0
print(json.dumps({"commit": commit, "sourceBindings": results}, indent=2))
raise SystemExit(0 if all(r["receiptVerified"] for r in results.values()) else 1)
