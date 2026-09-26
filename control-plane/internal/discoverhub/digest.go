package discoverhub

import (
	"encoding/json"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The local envelope builder and digest (task 08), until HA4-9 lands.
//
// A branch revision pins {key, digest} of the project revision written in
// the same batch, so the digest must be known before the Hub computes it.
// revisionEnvelope builds contract §3.4's envelope with hubapi.Canonical and
// revisionDigest hashes it. Both the pin and writeBatch's size check (review
// M6, S4) call revisionEnvelope, so neither builds its own.
//
// Why this file is still here although H0 exports hubapi.BriefEnvelope with
// Canonical and Digest (the digest-only shape of HA4-9): Canonical refuses
// an envelope over hubapi.MaxEnvelopeBytes and returns no bytes, so it
// cannot say how large a refused envelope is, which RevisionTooLargeError
// and FanoutTooLargeError report. The file goes only when the Hub exports an
// envelope builder that returns the bytes at any size (b10 re-review R-M3);
// TestLocalEnvelopeIsTheHubsEnvelope holds the two byte for byte until then.

// revisionEnvelope is the canonical §3.4 envelope of one revision, at any
// size: {"v":1,"kind":"brief-revision","briefId",…,"content"}, members
// sorted, content canonicalised with it (hubapi.Canonical re-reads a
// json.RawMessage token by token, so content need not be canonical yet).
// Content that is not a JSON object is refused, as the Hub refuses it.
func revisionEnvelope(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) ([]byte, error) {
	if len(content) == 0 || content[0] != '{' {
		return nil, errors.New("discoverhub: revision content must be a JSON object")
	}
	return hubapi.Canonical(map[string]any{
		"v": 1, "kind": "brief-revision", "briefId": briefID, "revision": revision,
		"projectId": projectID, "scope": scope, "branchRef": branchRef,
		"content": content,
	})
}

// revisionDigest is "sha256:" + the hex SHA-256 of revisionEnvelope.
func revisionDigest(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) (string, error) {
	env, err := revisionEnvelope(briefID, revision, projectID, scope, branchRef, content)
	if err != nil {
		return "", err
	}
	return hubapi.DigestOf(env).String(), nil
}
