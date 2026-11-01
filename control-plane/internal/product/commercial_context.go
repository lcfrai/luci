package product

import (
	"encoding/json"
	"fmt"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type OrchestratorCommercialRecord struct {
	Snapshot          CommercialSnapshot `json:"snapshot"`
	CurrenciesOmitted int                `json:"currenciesOmitted"`
	ProjectionDigest  string             `json:"projectionDigest"`
}
type OrchestratorCommercialContext struct {
	ProjectID          string                         `json:"projectId"`
	Records            []OrchestratorCommercialRecord `json:"records"`
	SourcesOmitted     int                            `json:"sourcesOmitted"`
	UnallocatedOmitted int                            `json:"unallocatedOmitted"`
	PendingSources     int                            `json:"pendingSources"`
	Unavailable        bool                           `json:"unavailable"`
	EvidenceOnly       bool                           `json:"evidenceOnly"`
}

func commercialProjectionDigest(v OrchestratorCommercialRecord) string {
	v.ProjectionDigest = ""
	return reviewDigest(v)
}
func validOrchestratorCommercial(v *OrchestratorCommercialContext, projectID string) bool {
	if v == nil {
		return true
	}
	if v.ProjectID != projectID || !v.EvidenceOnly || v.Records == nil || len(v.Records) > 8 || v.SourcesOmitted < 0 || v.SourcesOmitted > commercialSourceSafetyLimit+1 || v.UnallocatedOmitted < 0 || v.UnallocatedOmitted > 1 || v.PendingSources < 0 || v.PendingSources > commercialSourceSafetyLimit+1 {
		return false
	}
	if v.Unavailable && (len(v.Records) > 0 || v.PendingSources > 0 || v.UnallocatedOmitted > 0 || v.SourcesOmitted > 0) {
		return false
	}
	seen := map[string]bool{}
	for _, r := range v.Records {
		s := r.Snapshot
		if !validCommercialSnapshot(s, false) || !feedbackTimestamp(s.ObservedAt) || !hexDigest(s.Digest) || s.ID != "commercial-"+s.Digest || s.Attribution == "account-unallocated" || seen[s.ID] || r.CurrenciesOmitted < 0 || len(s.Currencies) > 8 || len(s.Currencies)+r.CurrenciesOmitted > 180 || r.ProjectionDigest != commercialProjectionDigest(r) {
			return false
		}
		seen[s.ID] = true
	}
	raw, err := json.Marshal(v)
	return err == nil && len(raw) <= 12<<10
}
func commercialContextForLedger(ledger commercialLedger) *OrchestratorCommercialContext {
	keys := commercialEnabledKeys(ledger)
	if len(keys) == 0 {
		return nil
	}
	value := &OrchestratorCommercialContext{ProjectID: ledger.ProjectID, Records: []OrchestratorCommercialRecord{}, EvidenceOnly: true}
	for _, key := range keys {
		latest := ""
		for _, status := range ledger.Status {
			if status.SourceKey == key {
				latest = status.LatestID
			}
		}
		var selected *CommercialSnapshot
		for i := range ledger.Snapshots {
			if ledger.Snapshots[i].ID == latest {
				selected = &ledger.Snapshots[i]
				break
			}
		}
		if selected == nil {
			value.PendingSources++
			continue
		}
		if selected.Attribution == "account-unallocated" {
			value.UnallocatedOmitted++
			continue
		}
		if len(value.Records) >= 8 {
			value.SourcesOmitted++
			continue
		}
		projected := *selected
		projected.Currencies = append([]CommercialCurrency{}, selected.Currencies...)
		omitted := 0
		if len(projected.Currencies) > 8 {
			omitted = len(projected.Currencies) - 8
			projected.Currencies = projected.Currencies[:8]
		}
		row := OrchestratorCommercialRecord{Snapshot: projected, CurrenciesOmitted: omitted}
		row.ProjectionDigest = commercialProjectionDigest(row)
		value.Records = append(value.Records, row)
		if raw, err := json.Marshal(value); err != nil || len(raw) > 12<<10 {
			value.Records = value.Records[:len(value.Records)-1]
			value.SourcesOmitted++
		}
	}
	return value
}

// Caller holds s.mu. This reads only local atomic snapshots; credentials and
// private resumable cursors/dedup material are never captured by the assistant.
func (s *Server) orchestratorCommercialLocked(p project.Project) *OrchestratorCommercialContext {
	_, ledger, err := s.readCommercialLocked(p)
	if err != nil {
		return &OrchestratorCommercialContext{ProjectID: p.ID, Records: []OrchestratorCommercialRecord{}, Unavailable: true, EvidenceOnly: true}
	}
	return commercialContextForLedger(ledger)
}
func commercialContextBytes(v *OrchestratorCommercialContext) int {
	if v == nil {
		return 0
	}
	raw, _ := json.Marshal(v)
	return len(raw)
}
func commercialObservation(value *OrchestratorCommercialContext) (any, []string, bool) {
	if value == nil {
		return nil, nil, true
	}
	if value.Unavailable {
		return nil, nil, false
	}
	parts := []string{}
	for _, r := range value.Records {
		row := r.Snapshot
		if row.Provider == "stripe" {
			parts = append(parts, fmt.Sprintf("%s / Stripe %s / CHARGE metadata %s=%s / UTC %s to %s exclusive / %d matching charges, %d paying customer references. Exact per-currency minor-unit rows are in captured commercialSignals; not accounting revenue.", row.ID, row.AccountID, row.StripeFilter.Key, row.StripeFilter.Value, row.PeriodStart, row.PeriodEndExclusive, row.MatchedCharges, row.PayingCustomerReferences))
		} else {
			parts = append(parts, fmt.Sprintf("%s / %s / original release %d / %d asset downloads across %d returned assets; not unique visitors or revenue.", row.ID, row.Repository, row.ReleaseID, row.AssetDownloads, row.Records))
		}
	}
	if len(value.Records) == 0 {
		return nil, parts, true
	}
	return value.Records, parts, true
}
