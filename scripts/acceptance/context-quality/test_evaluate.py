"""Controlled parser/rubric checks only. Oracle-derived data is NOT model output.

No process, provider, network, browser, application or Docker is started.
"""
import copy
import json
import tempfile
import unittest
from pathlib import Path

import evaluate as ev


CASES = [path.stem for path in sorted((ev.ROOT / "cases").glob("*.json"))]


def specimen(packet):
    """Positive structured-schema specimen; never a benchmark result."""
    gold = ev.load(ev.ROOT / "evaluator-only/oracles.json")["cases"][packet["caseId"]]
    answer = {key: packet[key] for key in ("version", "caseId", "inputSHA256", "projectId", "role")}
    for field in ("constraints", "actions", "evidence"):
        answer[field] = [{"id": key, "classification" if field == "evidence" else "disposition": value,
                          "sourceRefs": [source], "reason": "Controlled parser specimen, not a model explanation."}
                         for key, (value, source) in gold[field].items()]
    answer["questions"] = [{"topic": topic, "blocking": row["blocking"], "ownerRole": row["ownerRole"],
                            "sourceRefs": [row["source"]], "text": "Controlled parser question, not a semantic-quality example?"}
                           for topic, row in gold["questions"].items()]
    answer["handoff"] = {"retained": gold["handoff"], "dependencies": gold["dependencies"],
                         "nextActions": [key for key, (value, _) in gold["actions"].items() if value == "do-now"]}
    answer["message"] = "CONTROLLED PARSER DATA ONLY. Not produced by a model or executed through Luci."
    return answer


class RunFixture:
    def __init__(self, root, case_id="05-dependency-handoff"):
        self.root = root
        self.packet = ev.prepare(case_id, "full-authorized")
        self.artifacts = {}
        self.put("input", self.packet)
        self.answer = specimen(self.packet)
        self.put("answer", self.answer)
        self.model = {"provider": "synthetic-test", "name": "not-a-model", "snapshot": None}
        self.put("settings", {"model": self.model, "temperature": None, "tools": "none"})
        self.put("native", {"scope": "controlled test only; not native Luci bindings"})
        delivered = {}
        for index, row in enumerate(self.packet["sources"]):
            artifact = "source-" + str(index)
            self.put(artifact, row["text"].encode())
            delivered[row["id"]] = {"artifact": artifact, "originArtifact": "input", "originPointer": "/sources/" + str(index) + "/text"}
        source = {"caseProjectId": self.packet["projectId"], "projectId": "actual-fixture-project", "sourceCommit": "a"*40,
                  "contextRevision": "test-r1", "sourceArtifactHashes": {"input": self.artifacts["input"]["sha256"], "native": self.artifacts["native"]["sha256"]}}
        self.put("source-snapshot", source)
        trace = {key: source[key] for key in ("projectId", "sourceCommit", "contextRevision")}
        trace.update(harnessRevision="b"*40, captureKind="controlled-parser-fixture", nativeBindingsArtifact="native",
                     runId="synthetic-run", taskId="synthetic-task", operationId="synthetic-op",
                     contextDigest=ev.digest(ev.canonical({r["id"]: ev.digest(r["text"].encode()) for r in self.packet["sources"]})),
                     deliveredSources=delivered, requestIds=["request-1"])
        self.put("context", trace)
        self.put("request", {"model": self.model["name"], "input": [{"role": "user", "content": ev.canonical(self.packet).decode()}]})
        self.put("response", {"model": self.model["name"], "output": [{"content": [{"text": (root / "answer.json").read_text()}]}],
                              "usage": {"input_tokens": 12, "output_tokens": 8}})
        self.manifest = {"version": 1, "kind": "controlled-parser-fixture", "caseId": case_id, "variant": "full-authorized",
                         "conditionId": "parser-only", "trialId": "test-1", "execution": {"mode": "serial", "concurrencyLimit": 1, "batchId": "synthetic-batch"},
                         "harness": {"name": "synthetic", "revision": "b"*40, "adapterRevision": "c"*64}, "model": self.model,
                         "artifacts": self.artifacts, "inputArtifact": "input", "answerArtifact": "answer", "settingsArtifact": "settings",
                         "sourceSnapshotArtifact": "source-snapshot", "contextTraceArtifact": "context", "requests": [{"id": "request-1",
                         "requestArtifact": "request", "responseArtifact": "response", "sourceIds": [r["id"] for r in self.packet["sources"]],
                         "startedAt": "2026-01-01T00:00:00Z", "finishedAt": "2026-01-01T00:00:01Z", "outputPointer": "/output/0/content/0/text",
                         "usagePointers": {"inputTokens": "/usage/input_tokens", "outputTokens": "/usage/output_tokens", "cachedInputTokens": None}, "billingArtifact": None}]}

    def put(self, name, value):
        data = value if isinstance(value, bytes) else ev.canonical(value)
        path = name + ".json"
        (self.root / path).write_bytes(data)
        self.artifacts[name] = {"path": path, "sha256": ev.digest(data)}

    def finish(self):
        (self.root / "run.json").write_bytes(ev.canonical(self.manifest))
        return ev.validate_run(self.root)


class EvaluationTests(unittest.TestCase):
    def test_six_frozen_case_specimens_score_structure_only(self):
        self.assertEqual(len(CASES), 6)
        for case_id in CASES:
            with self.subTest(case=case_id), tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
                result = RunFixture(Path(temp), case_id).finish()
                self.assertEqual(result["status"], "controlled-parser-only-not-model-quality")
                self.assertIsNone(result["observations"])
                self.assertIsNone(result["structuredScores"]["overallQualityPass"])
                self.assertEqual(result["humanReview"]["status"], "not-scored")
                for key in ("constraints", "actions", "evidence"):
                    self.assertEqual(result["structuredScores"][key]["fullContextExpectation"]["fraction"], 1)

    def test_duplicate_unknown_trailing_and_nonfinite_json_refused(self):
        for data in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{}{}', b'{"x":"\\ud800"}', b'[[[[[' + b'['*32 + b'0' + b']'*37):
            with self.subTest(data=data), self.assertRaises(ev.Invalid):
                ev.parse(data)
        packet = ev.prepare("01-ambiguity", "full-authorized")
        answer = specimen(packet)
        answer["verified"] = True
        with self.assertRaises(ev.Invalid):
            ev.validate_answer(answer, packet)

    def test_no_gold_or_canary_in_worker_packet(self):
        for case_id in CASES:
            packet = ev.prepare(case_id, "full-authorized")
            raw = ev.canonical(packet)
            self.assertNotIn(b"humanNotes", raw)
            self.assertNotIn(b"criticalActions", raw)
            self.assertNotIn(b"SYNTHETIC_ONLY", raw)

    def test_correct_ids_do_not_hide_false_verification(self):
        packet = ev.prepare("05-dependency-handoff", "full-authorized")
        answer = specimen(packet)
        answer["evidence"][0]["classification"] = "recorded-pass"
        gold = ev.load(ev.ROOT / "evaluator-only/oracles.json")["cases"][packet["caseId"]]
        result = ev.score(answer, packet, gold)
        self.assertEqual(len(result["criticalStructuredViolations"]), 1)
        self.assertLess(result["evidence"]["fullContextExpectation"]["fraction"], 1)

    def test_unnecessary_question_and_retired_dependency_are_visible(self):
        packet = ev.prepare("05-dependency-handoff", "full-authorized")
        answer = specimen(packet)
        answer["questions"].append({"topic": "button-color", "blocking": True, "ownerRole": "owner", "sourceRefs": ["stock-brief-r9"], "text": "What color?"})
        answer["handoff"]["dependencies"] = ["api-op-1"]
        gold = ev.load(ev.ROOT / "evaluator-only/oracles.json")["cases"][packet["caseId"]]
        result = ev.score(answer, packet, gold)
        self.assertEqual(result["ambiguity"]["unnecessaryQuestionTopics"], ["button-color"])
        self.assertFalse(result["handoff"]["currentDependenciesExact"])

    def test_ablations_are_explicit_and_missing_sources_cannot_be_cited(self):
        packet = ev.prepare("05-dependency-handoff", "omit-handoff")
        self.assertNotIn("dependency-packet", {r["id"] for r in packet["sources"]})
        with self.assertRaises(ev.Invalid):
            ev.validate_answer(specimen(packet), packet)
        with self.assertRaises(ev.Invalid):
            ev.prepare("01-ambiguity", "omit-handoff")

    def test_artifact_tampering_link_and_path_escape_refused(self):
        for mutation in ("hash", "escape", "symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
                root = Path(temp)
                fixture = RunFixture(root)
                if mutation == "hash":
                    (root / "request.json").write_bytes(b"{}")
                elif mutation == "escape":
                    fixture.artifacts["request"]["path"] = "../request.json"
                else:
                    (root / "linked.json").symlink_to(root / "request.json")
                    fixture.artifacts["request"]["path"] = "linked.json"
                with self.assertRaises((ev.Invalid, OSError)):
                    fixture.finish()

    def test_answer_must_match_captured_response(self):
        with tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
            fixture = RunFixture(Path(temp))
            fixture.answer["message"] = "Edited after the model returned."
            fixture.put("answer", fixture.answer)
            with self.assertRaises(ev.Invalid):
                fixture.finish()

    def test_source_origin_and_actual_request_are_required(self):
        for mutation in ("origin", "request", "task"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
                fixture = RunFixture(Path(temp))
                if mutation == "request":
                    fixture.put("request", {"model": fixture.model["name"], "input": "Undelivered context"})
                elif mutation == "task":
                    fixture.put("request", {"model": fixture.model["name"], "input": [r["text"] for r in fixture.packet["sources"]]})
                else:
                    trace = ev.load(Path(temp) / "context.json")
                    trace["deliveredSources"]["stock-brief-r9"]["originPointer"] = "/task"
                    fixture.put("context", trace)
                with self.assertRaises(ev.Invalid):
                    fixture.finish()

    def test_unknown_usage_cost_and_resource_values_stay_unknown(self):
        with tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
            result = RunFixture(Path(temp)).finish()
            metrics = result["controlledObservations"]
            self.assertEqual(metrics["usage"]["inputTokens"]["total"], 12)
            self.assertIsNone(metrics["usage"]["cachedInputTokens"]["total"])
            self.assertIsNone(metrics["billedCost"]["total"])
            self.assertIsNone(metrics["ownedProcessMemoryBytes"])
            self.assertIsNone(metrics["endToEndVerifiedBuildLatencyMs"])

    def test_live_luci_requires_native_capture_not_relabelled_parser_context(self):
        with tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
            fixture = RunFixture(Path(temp))
            fixture.manifest["kind"] = "live-provider-capture"
            fixture.manifest["harness"]["name"] = "luci"
            with self.assertRaises(ev.Invalid):
                fixture.finish()

    def test_isolation_canary_detected_in_raw_request_even_if_answer_omits_it(self):
        with tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
            fixture = RunFixture(Path(temp), "02-isolation")
            request = ev.load(Path(temp) / "request.json")
            request["extra"] = ev.load(ev.ROOT / "evaluator-only/isolation-seeds.json")["forbiddenMarkers"][0]
            fixture.put("request", request)
            result = fixture.finish()
            self.assertFalse(result["structuredScores"]["isolationCanary"]["passed"])

    def test_different_role_project_and_duplicate_ids_refused(self):
        packet = ev.prepare("02-isolation", "full-authorized")
        for key, value in [("role", "owner"), ("projectId", "atlas-agency"), ("inputSHA256", "0"*64)]:
            answer = specimen(packet)
            answer[key] = value
            with self.assertRaises(ev.Invalid):
                ev.validate_answer(answer, packet)
        answer = specimen(packet)
        answer["constraints"].append(copy.deepcopy(answer["constraints"][0]))
        with self.assertRaises(ev.Invalid):
            ev.validate_answer(answer, packet)

    def test_handoff_cannot_recommend_a_refused_or_missing_action(self):
        packet = ev.prepare("06-restart", "full-authorized")
        answer = specimen(packet)
        answer["handoff"]["nextActions"].append("duplicate-upload-worker")
        with self.assertRaises(ev.Invalid):
            ev.validate_answer(answer, packet)

    def test_overlapping_serial_requests_and_wrong_provider_model_refused(self):
        for mutation in ("overlap", "model"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
                fixture = RunFixture(Path(temp))
                if mutation == "model":
                    request = ev.load(Path(temp) / "request.json")
                    request["model"] = "different-model"
                    fixture.put("request", request)
                else:
                    other = copy.deepcopy(fixture.manifest["requests"][0])
                    other["id"] = "request-2"
                    other["outputPointer"] = None
                    fixture.manifest["requests"].append(other)
                    trace = ev.load(Path(temp) / "context.json")
                    trace["requestIds"].append("request-2")
                    fixture.put("context", trace)
                with self.assertRaises(ev.Invalid):
                    fixture.finish()

    def test_parallel_overlap_cannot_exceed_declared_limit(self):
        for limit in (1, 2):
            with self.subTest(limit=limit), tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
                fixture = RunFixture(Path(temp))
                fixture.manifest["execution"].update(mode="parallel", concurrencyLimit=limit)
                other = copy.deepcopy(fixture.manifest["requests"][0])
                other.update(id="request-2", outputPointer=None)
                fixture.manifest["requests"].append(other)
                trace = ev.load(Path(temp) / "context.json")
                trace["requestIds"].append("request-2")
                fixture.put("context", trace)
                if limit == 1:
                    with self.assertRaises(ev.Invalid):
                        fixture.finish()
                else:
                    result = fixture.finish()
                    self.assertEqual(result["controlledObservations"]["peakObservedRequestOverlap"], 2)

    def test_adjacent_requests_do_not_overlap_and_rule_hashes_are_retained(self):
        with tempfile.TemporaryDirectory(prefix="c014-parser-") as temp:
            fixture = RunFixture(Path(temp))
            other = copy.deepcopy(fixture.manifest["requests"][0])
            other.update(id="request-2", outputPointer=None, startedAt="2026-01-01T00:00:01Z", finishedAt="2026-01-01T00:00:02Z")
            fixture.manifest["requests"].append(other)
            trace = ev.load(Path(temp) / "context.json")
            trace["requestIds"].append("request-2")
            fixture.put("context", trace)
            result = fixture.finish()
            self.assertEqual(result["controlledObservations"]["peakObservedRequestOverlap"], 1)
            hashes = result["scoringSourceHashes"]
            self.assertEqual(hashes["evaluate.py"], ev.digest(ev.read_regular(ev.ROOT / "evaluate.py")))
            self.assertEqual(hashes["evaluator-only/oracles.json"], ev.digest(ev.read_regular(ev.ROOT / "evaluator-only/oracles.json")))
            self.assertEqual(len(hashes), 4)


if __name__ == "__main__":
    unittest.main()
