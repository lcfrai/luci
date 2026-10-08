# CLOUD-019: lost pre-allocation reason and failed-start wording

The installed dependency attempt on clean
`3b8054e69340bce37afe3aec17189a33c2fb2991` completed task A, then failed before
task B reached container creation. B's registry record retained only a generic
access/capacity-change message. The dispatcher discarded the underlying error,
and later queue observation labelled the failed start completed/review-required.
The specific rejected condition is unknown; it must not be invented from later
resource observations.

The unchanged [summary](summary.json), [resource observations](resource-summary.json),
[screenshot](failed-stage.png) and [manifest](manifest.json) preserve that failure.
[Publication hashes](publication.json) bind the copied files. Exactly one actual
native Codex worker used a scripted provider. Independent cleanup recorded zero
owned resources and unchanged package, runtime and helpers. No integration, real
conflict, dependency join or sustained-performance result was reached.

The CLOUD-019 correction and its controlled/installed verification are separate
evidence. A diagnostic must never substitute for launch or resource-release proof.
