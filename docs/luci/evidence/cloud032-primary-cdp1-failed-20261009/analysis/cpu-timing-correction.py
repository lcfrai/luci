#!/usr/bin/env python3
"""Validate only the exact retained CPU timing arrays; never export call frames.

This supersedes the preliminary weighted CPU attribution. It does not repair
negative deltas, read the private timeline, or rerun the application.
"""
from pathlib import Path
import argparse
import hashlib
import json


def metadata(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cpu-profile", required=True, type=Path)
    parser.add_argument("--preliminary-analysis", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    raw = args.cpu_profile.read_bytes()
    expected = {"bytes": 1097185, "sha256": "265646ccaf4ef0742369958494d4c6656369283e8e5988eb331e0a9d0def3fe3"}
    if metadata(raw) != expected:
        raise ValueError("CPU bytes differ from the exact retained primary-cdp-1 profile")
    profile = json.loads(raw)
    previous_raw = args.preliminary_analysis.read_bytes()
    previous = json.loads(previous_raw)
    if previous["cpu"]["sha256"] != expected["sha256"]:
        raise ValueError("Preliminary analysis identifies another CPU profile")
    samples = profile["samples"]
    deltas = profile["timeDeltas"]
    if any(type(delta) is not int for delta in deltas):
        raise ValueError("Unexpected non-integer delta; no weighted analysis permitted")
    negative = [delta for delta in deltas if delta < 0]
    nodes = {node["id"] for node in profile["nodes"]}
    missing = sum(sample not in nodes for sample in samples)
    invalid = len(samples) != len(deltas) or bool(negative) or bool(missing)
    report = {
        "schemaVersion": 1,
        "scope": "Superseding allowlisted CPU timing validation of exact private primary-cdp-1 bytes. No raw source URLs, function names, arguments, trace events, runtime or tests.",
        "sourceCPU": {**expected, "independentlyRehashedForThisCorrection": True, "rawIncluded": False},
        "preliminaryAnalysis": {**metadata(previous_raw), "preservedUnchanged": True, "weightedCPUAttributionSuperseded": True},
        "analyzerSHA256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "timingValidation": {
            "status": "invalid" if invalid else "structurally-valid",
            "nodes": len(nodes),
            "samples": len(samples),
            "deltas": len(deltas),
            "sampleDeltaCountsMatch": len(samples) == len(deltas),
            "missingSampleNodeReferences": missing,
            "negativeDeltaCount": len(negative),
            "negativeDeltaSumUs": sum(negative),
            "minimumDeltaUs": min(deltas),
            "maximumDeltaUs": max(deltas),
            "zeroDeltaCount": sum(delta == 0 for delta in deltas),
            "positiveDeltaCount": sum(delta > 0 for delta in deltas),
            "positiveDeltaSumUs": sum(delta for delta in deltas if delta > 0),
            "signedDeltaSumUs": sum(deltas),
            "recordedStartUs": profile["startTime"],
            "recordedEndUs": profile["endTime"],
            "recordedEndpointSpanUs": profile["endTime"] - profile["startTime"],
            "endpointSpanMinusSignedDeltaSumUs": profile["endTime"] - profile["startTime"] - sum(deltas),
        },
        "weightedAttribution": {
            "status": "withheld-invalid-timing",
            "sampleClassesUs": None,
            "topFrameWeightedTimesUs": None,
            "inclusiveFrameOrBundleWeightedTimesUs": None,
            "weightedCPUPercentages": None,
            "perActionWeightedCPU": None,
            "invalidatedPreliminaryPointers": ["/cpu/sampleClassesUs", "/cpu/topFrames/*/sampleWeightUs"],
            "scope": "All CPU-time-weighted special-frame, exclusive or inclusive frame, script or bundle attribution derived from this profile is invalid. This includes any separate preliminary summaries; no inclusive-bundle field is claimed present in the retained prefix file.",
            "reason": "Negative sample time deltas prevent a valid monotonic duration allocation. A nearly matching signed total or equal array lengths does not make the weights usable.",
            "noRepairApplied": "No negative delta was dropped, clamped, absolutized, sorted, redistributed or replaced by the nominal sampling interval.",
        },
        "unaffectedEvidence": [
            "Original trace byte-cap failure and traceDataLoss=true remain unchanged.",
            "Raw CPU hash, node/sample counts and stored endpoint span are structural metadata, not valid CPU-time attribution.",
            "Prefix trace event/category counts and limited marker coverage retain their original incomplete-prefix scope; this correction did not reread the timeline.",
            "All 400 original action durations and their separate quantiles remain unchanged; this is not performance acceptance or a causal attribution.",
        ],
        "limitations": [
            "The cause of the negative deltas is not established.",
            "The original preliminary analysis and reconstructed preliminary helper remain byte-for-byte unchanged and must not be used as trusted weighted CPU analysis.",
            "The historical wholeWindowUs field is an endpoint difference, not a CPU utilization or weighted attribution result.",
        ],
    }
    if not invalid:
        raise ValueError("Expected invalid original timing was not reproduced")
    with args.output.open("x") as stream:
        stream.write(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    main()
