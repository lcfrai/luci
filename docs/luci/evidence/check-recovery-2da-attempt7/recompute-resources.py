#!/usr/bin/env python3
"""Read only public pinned-attempt samples; emit recomputed counts and minima.

Run from any directory with Python 3. No application, daemon, process inspection,
fixture read, network or product test is performed. JSON is written to stdout.
"""
import collections
import gzip
import hashlib
import json
from pathlib import Path

base = Path(__file__).resolve().parent
compressed = (base / "observation/samples.jsonl.gz").read_bytes()
raw = gzip.decompress(compressed)
samples = [json.loads(line) for line in raw.decode().splitlines() if line]
summary = json.loads((base / "observation/summary.json").read_text())
derived = json.loads((base / "resource-observations.json").read_text())
status_counts = collections.Counter()
rows = []
observed_samples = set()
mismatches = []
for sample_index, sample in enumerate(samples):
    cgroup = sample.get("controlPlaneCgroupMemory", {})
    status_counts[cgroup.get("status", "missing")] += 1
    for row_index, row in enumerate(cgroup.get("observations", [])):
        if row.get("status") != "observed":
            continue
        total = row["physicalTotalBytes"]
        available = row["physicalAvailableBytes"]
        for node in row.get("nodes", []):
            if node.get("limitStatus") == "finite":
                total = min(total, node["limitBytes"])
                assert node.get("currentStatus") == "observed", "Missing finite-ancestor usage"
                available = min(available, max(0, node["limitBytes"] - node["currentBytes"]))
        if (total != row["productComparableEffectiveTotalBytes"]
                or available != row["productComparableEffectiveAvailableBytes"]):
            mismatches.append({"sample": sample_index, "row": row_index})
        observed_samples.add(sample_index)
        rows.append((row["physicalAvailableBytes"], available))

assert len(samples) == summary["samples"] == derived["sampleCount"]
assert len(rows) == derived["observedRows"]
assert len(observed_samples) == derived["samplesWithObservedRows"]
assert not mismatches
assert min(row[0] for row in rows) == derived["minObservedPhysicalAvailableBytes"]
assert min(row[1] for row in rows) == derived["minObservedEffectiveAvailableBytes"]
assert hashlib.sha256(raw).hexdigest() == derived["sourceInputs"]["observation/samples.jsonl"]["sha256"]
print(json.dumps({
    "sampleCount": len(samples),
    "samplesWithObservedRows": len(observed_samples),
    "observedRows": len(rows),
    "sampleStatusCounts": dict(status_counts),
    "formulaMismatches": mismatches,
    "minimumPhysicalAvailableBytes": min(row[0] for row in rows),
    "minimumEffectiveAvailableBytes": min(row[1] for row in rows),
    "checkCoverage": summary["checkCoverage"],
    "decodedSamplesSHA256": hashlib.sha256(raw).hexdigest(),
    "scope": "Read-only recalculation of retained sequential samples; no atomic admission or performance claim.",
}, indent=2))
