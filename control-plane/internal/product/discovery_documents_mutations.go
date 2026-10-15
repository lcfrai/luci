package product

import (
	"errors"
	"strings"
	"time"
)

type discoveryCorrection struct {
	Page               int    `json:"page"`
	SegmentID          string `json:"segmentId"`
	ExpectedTextDigest string `json:"expectedTextDigest"`
	Text               string `json:"text"`
	Reason             string `json:"reason"`
}

func discoveryRequestReadback(saved storedDiscoveryDocument, key, digest, action string) (discoveryRequestReceipt, bool, error) {
	if !validRunKey(key) || !hexDigest(digest) {
		return discoveryRequestReceipt{}, false, errDiscoveryDocument
	}
	keyDigest := reviewDigest(key)
	for _, receipt := range saved.Requests {
		if receipt.KeyDigest == keyDigest {
			if receipt.RequestDigest != digest || receipt.Action != action {
				return discoveryRequestReceipt{}, false, ErrProjectRunKeyConflict
			}
			return receipt, true, nil
		}
	}
	return discoveryRequestReceipt{}, false, nil
}
func discoveryRememberRequest(saved *storedDiscoveryDocument, key, digest, action, attemptID string, extraction int) error {
	if len(saved.Requests) >= 256 {
		return errDiscoveryCapacity
	}
	saved.Requests = append(saved.Requests, discoveryRequestReceipt{KeyDigest: reviewDigest(key), RequestDigest: digest, Action: action, AttemptID: attemptID, ExtractionRevision: extraction})
	return nil
}

// A fresh explicit attempt retains expired predecessors as interrupted. A GET
// never calls this function or reserves worker capacity.
func discoveryStartAttempt(saved *storedDiscoveryDocument, expected int, key string, actor ContextActor, now time.Time) (DiscoveryDocumentAttempt, error) {
	digest := reviewDigest(struct {
		Original string
		Actor    string
	}{saved.Document.Original.Digest, actor.ID})
	if prior, found, err := discoveryRequestReadback(*saved, key, digest, "extract"); err != nil {
		return DiscoveryDocumentAttempt{}, err
	} else if found {
		for _, attempt := range saved.Document.Attempts {
			if attempt.ID == prior.AttemptID {
				return attempt, nil
			}
		}
		return DiscoveryDocumentAttempt{}, errDiscoveryDocument
	}
	if saved.Document.Revision != expected {
		return DiscoveryDocumentAttempt{}, ErrProjectRunConflict
	}
	if len(saved.Document.Attempts) >= 128 {
		return DiscoveryDocumentAttempt{}, errDiscoveryCapacity
	}
	for i := range saved.Document.Attempts {
		attempt := &saved.Document.Attempts[i]
		if attempt.Status != "extracting" {
			continue
		}
		expires, err := time.Parse(time.RFC3339Nano, attempt.ExpiresAt)
		if err != nil || now.Before(expires) {
			return DiscoveryDocumentAttempt{}, errors.New("This document already has an active or unconfirmed extraction. Cancel or observe that exact attempt before restarting.")
		}
		attempt.Status = "interrupted"
		attempt.FinishedAt = now.UTC().Format(time.RFC3339Nano)
	}
	attempt := DiscoveryDocumentAttempt{ID: newID("extract-"), Status: "extracting", StartedAt: now.UTC().Format(time.RFC3339Nano), ExpiresAt: now.Add(discoveryAttemptLifetime).UTC().Format(time.RFC3339Nano), Actor: actor}
	if err := discoveryRememberRequest(saved, key, digest, "extract", attempt.ID, 0); err != nil {
		return DiscoveryDocumentAttempt{}, err
	}
	saved.Document.Attempts = append(saved.Document.Attempts, attempt)
	saved.Document.Revision++
	return attempt, nil
}

func discoveryFinishAttempt(saved *storedDiscoveryDocument, expected int, attemptID, key string, extraction DiscoveryDocumentExtraction, actor ContextActor, now time.Time) error {
	// Caller identity and receipt metadata are authoritative, never client supplied.
	extraction.Revision = 0
	extraction.AttemptID = attemptID
	extraction.Actor = actor
	extraction.At = ""
	extraction.Digest = ""
	extraction.CorrectsRevision = 0
	digest := reviewDigest(extraction)
	if _, found, err := discoveryRequestReadback(*saved, key, digest, "finish"); err != nil {
		return err
	} else if found {
		return nil
	}
	if expected != saved.Document.Revision {
		return ErrProjectRunConflict
	}
	at := -1
	for i, attempt := range saved.Document.Attempts {
		if attempt.ID == attemptID {
			at = i
			break
		}
	}
	if at < 0 {
		return errDiscoveryDocument
	}
	attempt := saved.Document.Attempts[at]
	expires, err := time.Parse(time.RFC3339Nano, attempt.ExpiresAt)
	if err != nil || attempt.Actor.ID != actor.ID || attempt.Status != "extracting" || !now.Before(expires) {
		return errors.New("The exact extraction owner expired, was cancelled or changed. Its late output cannot become current evidence; explicitly restart after preserving the receipt.")
	}
	extraction.Revision = len(saved.Document.Extractions) + 1
	extraction.At = now.UTC().Format(time.RFC3339Nano)
	if err := validateDiscoveryExtraction(extraction, saved.Document.Original); err != nil {
		return err
	}
	extraction.Digest = discoveryExtractionDigest(extraction)
	if err := discoveryRememberRequest(saved, key, digest, "finish", attemptID, extraction.Revision); err != nil {
		return err
	}
	saved.Document.Attempts[at].Status = extraction.Status
	saved.Document.Attempts[at].FinishedAt = extraction.At
	saved.Document.Extractions = append(saved.Document.Extractions, extraction)
	saved.Document.Revision++
	return nil
}

func discoveryCancelAttempt(saved *storedDiscoveryDocument, expected int, attemptID, key string, actor ContextActor, now time.Time) error {
	digest := reviewDigest(struct {
		Attempt string
		Actor   string
	}{attemptID, actor.ID})
	if _, found, err := discoveryRequestReadback(*saved, key, digest, "cancel"); err != nil {
		return err
	} else if found {
		return nil
	}
	if saved.Document.Revision != expected {
		return ErrProjectRunConflict
	}
	for i, attempt := range saved.Document.Attempts {
		if attempt.ID == attemptID {
			if attempt.Actor.ID != actor.ID || attempt.Status != "extracting" {
				return ErrProjectRunConflict
			}
			if err := discoveryRememberRequest(saved, key, digest, "cancel", attemptID, 0); err != nil {
				return err
			}
			saved.Document.Attempts[i].Status = "cancelled"
			saved.Document.Attempts[i].FinishedAt = now.UTC().Format(time.RFC3339Nano)
			saved.Document.Revision++
			return nil
		}
	}
	return errDiscoveryDocument
}

func discoveryCorrectExtraction(saved *storedDiscoveryDocument, expected, extractionRevision int, key string, changes []discoveryCorrection, actor ContextActor, now time.Time) error {
	digest := reviewDigest(struct {
		Revision int
		Changes  []discoveryCorrection
		Actor    string
	}{extractionRevision, changes, actor.ID})
	if _, found, err := discoveryRequestReadback(*saved, key, digest, "correct"); err != nil {
		return err
	} else if found {
		return nil
	}
	if saved.Document.Revision != expected || extractionRevision < 1 || extractionRevision != len(saved.Document.Extractions) {
		return ErrProjectRunConflict
	}
	if len(changes) == 0 || len(changes) > 200 || len(saved.Document.Extractions) >= 128 {
		return errDiscoveryCapacity
	}
	old := saved.Document.Extractions[extractionRevision-1]
	if old.Status != "ready" && old.Status != "needs-review" {
		return errors.New("Only a complete retained extraction can be corrected. Preserve its original before restarting.")
	}
	// Deep copy: approved/source-linked old revisions never change in memory or on disk.
	next := old
	next.Pages = make([]DiscoveryDocumentPage, len(old.Pages))
	for i, page := range old.Pages {
		next.Pages[i] = page
		next.Pages[i].Segments = append([]DiscoveryDocumentSegment{}, page.Segments...)
	}
	seen := map[string]bool{}
	for _, change := range changes {
		if seen[change.SegmentID] || !discoveryPlainText(change.Text, 64<<10) || !discoveryPlainText(change.Reason, 2000) || strings.TrimSpace(change.Reason) == "" {
			return errDiscoveryDocument
		}
		seen[change.SegmentID] = true
		found := false
		for i, page := range next.Pages {
			if page.Page != change.Page {
				continue
			}
			for j, segment := range page.Segments {
				if segment.ID == change.SegmentID && segment.TextDigest == change.ExpectedTextDigest {
					next.Pages[i].Segments[j].Text = change.Text
					next.Pages[i].Segments[j].TextDigest = discoveryTextDigest(change.Text)
					next.Pages[i].Segments[j].Kind = "correction"
					next.Pages[i].Segments[j].Confidence = nil
					next.Pages[i].Segments[j].Uncertainty = "Human correction: " + change.Reason
					found = true
				}
			}
		}
		if !found {
			return ErrProjectRunConflict
		}
	}
	next.Revision = len(saved.Document.Extractions) + 1
	next.CorrectsRevision = old.Revision
	next.Actor = actor
	next.At = now.UTC().Format(time.RFC3339Nano)
	next.Digest = ""
	next.Status = "needs-review"
	if err := validateDiscoveryExtraction(next, saved.Document.Original); err != nil {
		return err
	}
	next.Digest = discoveryExtractionDigest(next)
	if err := discoveryRememberRequest(saved, key, digest, "correct", next.AttemptID, next.Revision); err != nil {
		return err
	}
	saved.Document.Extractions = append(saved.Document.Extractions, next)
	saved.Document.Revision++
	return nil
}
