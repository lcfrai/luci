package product

import (
	"context"
	"fmt"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Source binding is deliberately separate from publication eligibility. Pricing
// and support documents do not become GitHub launch assets by using saved GTM.
func gtmArtifactKind(kind string) bool {
	return launchArtifactKind(kind) || kind == "pricing" || kind == "support-guide" || kind == "go-to-market"
}

func validArtifactGTMSource(kind string, source DistributeArtifactSource) bool {
	// Original Pricing and go-to-market rows had no GTM fields. Keep exact JSON, digest,
	// history and source semantics readable; do not migrate an immutable row.
	if kind == "pricing" || kind == "go-to-market" {
		return true
	}
	return gtmArtifactKind(kind) == (source.GTMRevision > 0)
}

// Retained legacy Pricing and GTM documents follow their original source
// contracts. New Pricing and GTM drafts always capture current saved strategy.
// Caller holds the normal s.mu -> teamControlsMu lock order.
func (s *Server) artifactSourceForRetainedLocked(ctx context.Context, p project.Project, row DistributeArtifact) (DistributeArtifactSource, journeyState, *TeamControlRevision, *ControlAssessment, error) {
	runID := ""
	if row.Source.SelectedEvidence != nil {
		runID = row.Source.SelectedEvidence.RunID
	}
	if (row.Kind == "pricing" || row.Kind == "go-to-market") && row.Source.GTMRevision == 0 {
		return s.artifactSource(ctx, p, runID)
	}
	return s.artifactSourceForKindLocked(ctx, p, row.Kind, runID)
}

func generatedCommercialAsset(value DistributeArtifact, saved GTMWorkspace, state journeyState) (DistributeArtifact, error) {
	content := saved.Content
	ref := fmt.Sprintf("gtm:%d:%s", saved.Revision, saved.Digest)
	sections := []DistributeArtifactSection{}
	add := func(id, heading, body string, bullets []string, refs ...string) {
		if bullets == nil {
			bullets = []string{}
		}
		if len(refs) == 0 {
			refs = []string{ref}
		}
		sections = append(sections, DistributeArtifactSection{ID: id, Heading: heading, Body: body, Bullets: bullets, SourceRefs: refs})
	}
	audiences := []string{}
	for _, segment := range content.Segments {
		audiences = append(audiences, segment.ID+": "+segment.Name+" — problem: "+segment.Problem+"; intended value: "+segment.Value)
	}
	add("commercial-offer", "Saved offer and audience", strings.TrimSpace(content.Positioning+"\n\n"+content.Offer), audiences)
	value.Assumptions = append(value.Assumptions, content.PricingAssumptions...)
	value.Assumptions = append(value.Assumptions, "This document uses the exact saved GTM revision. Demand, pricing and customer-value statements remain declarations or assumptions, not independently verified results.")

	switch value.Kind {
	case "go-to-market":
		competitors := []string{}
		for _, row := range content.Competitors {
			competitors = append(competitors, row.ID+": "+row.Name+" — alternative: "+row.Alternative+"; declared difference: "+row.Difference)
		}
		add("commercial-competitors", "Saved alternatives and positioning", "These are the declared alternatives and differences in the exact saved strategy, not independently checked competitor claims.", competitors)
		channels := []string{}
		for _, row := range content.Channels {
			channels = append(channels, row.ID+": "+row.Name+" — hypothesis: "+row.Hypothesis+"; segment IDs: "+strings.Join(row.SegmentIDs, ", "))
		}
		add("commercial-channels", "Saved channels and audience links", "Review these selected channel hypotheses against real campaign results before treating them as proven routes to customers.", channels)
		milestones := []string{}
		for _, row := range content.Milestones {
			date := row.TargetDate
			if date == "" {
				date = "not selected"
			}
			milestones = append(milestones, row.ID+": "+row.Title+" — intended outcome: "+row.Outcome+"; target date: "+date+"; dependencies: "+strings.Join(row.DependsOn, ", "))
		}
		add("commercial-milestones", "Saved launch milestones", "Dates and outcomes are saved intentions, not completed launches or verified delivery promises.", milestones)
		experiments := []string{}
		for _, row := range content.Experiments {
			experiments = append(experiments, row.ID+": "+row.Title+"\nHypothesis: "+row.Hypothesis+"\nMethod: "+row.Method+"\nSuccess criterion: "+row.SuccessCriterion+"\nSegment: "+row.SegmentID+"; channel: "+row.ChannelID+"\nReported status: "+row.Status+"\nReported result: "+row.Result)
		}
		add("commercial-experiments", "Saved experiments and reported outcomes", "Reported statuses and results remain attributed saved records. Preparing this document does not run experiments or establish observed demand.", experiments)
		if content.CommercialModel != nil {
			input := content.CommercialModel
			interval := map[string]string{"month": "per month", "year": "per year", "one_time": "one time"}[input.Interval]
			add("commercial-price", "Saved price scenario", fmt.Sprintf("%s %s %s per unit; %d assumed units. Read the exact Pricing artifact for cost, contribution and break-even calculations. This is not observed revenue or a published price.", input.Currency, launchMoney(input.PricePerUnitMinor, input.MinorUnitDigits), interval, input.Units), nil)
			value.Assumptions = append(value.Assumptions, input.Assumptions...)
		} else {
			value.Gaps = append(value.Gaps, "No commercial scenario is saved in the linked strategy; pricing and unit-economics inputs remain to be selected.")
		}
		if strings.TrimSpace(content.Positioning) == "" {
			value.Gaps = append(value.Gaps, "No positioning statement is saved.")
		}
		if strings.TrimSpace(content.Offer) == "" {
			value.Gaps = append(value.Gaps, "No offer is saved.")
		}
		if len(content.Segments) == 0 {
			value.Gaps = append(value.Gaps, "No customer segments are saved.")
		}
		if len(content.Channels) == 0 {
			value.Gaps = append(value.Gaps, "No channel hypotheses are saved.")
		}
		if len(content.Milestones) == 0 {
			value.Gaps = append(value.Gaps, "No launch milestones are saved.")
		}
		value.Gaps = append(value.Gaps, "The saved strategy does not record a distribution owner. Assign responsibility before external delivery.")
	case "pricing":
		if content.CommercialModel == nil {
			add("commercial-scenario", "Commercial scenario not supplied", "No commercial model is saved in this exact GTM revision. Add the currency, unit price, variable and fixed costs, interval, demand and assumptions in Go to market before using this as a priced offer.", nil)
			value.Gaps = append(value.Gaps, "The linked GTM revision has no commercial scenario; no price, cost, contribution or break-even amount has been calculated.")
		} else {
			projection, err := CompileCommercialModel(*content.CommercialModel)
			if err != nil {
				return DistributeArtifact{}, err
			}
			input := projection.Input
			money := func(amount int64) string {
				sign := ""
				if amount < 0 {
					sign = "-"
					amount = -amount
				}
				return input.Currency + " " + sign + launchMoney(amount, input.MinorUnitDigits)
			}
			interval := map[string]string{"month": "monthly", "year": "yearly", "one_time": "one-time"}[input.Interval]
			add("commercial-inputs", "Exact saved scenario inputs", "Scenario projection only. Costs and units use the same selected interval; no exchange rate, customer quote or observed revenue is inferred.", []string{
				"Currency: " + input.Currency + fmt.Sprintf("; minor-unit digits: %d", input.MinorUnitDigits),
				"Interval: " + interval + fmt.Sprintf("; assumed units: %d", input.Units),
				"Price per unit: " + money(input.PricePerUnitMinor),
				"Variable cost per unit: " + money(input.VariableCostPerUnitMinor),
				"Fixed cost per interval: " + money(input.FixedCostMinor),
				"Scenario input SHA256: " + projection.Digest,
			})
			results := []string{
				"Projected revenue: " + money(projection.RevenueMinor),
				"Projected variable cost: " + money(projection.VariableCostMinor),
				"Projected total cost: " + money(projection.TotalCostMinor),
				"Projected operating profit: " + money(projection.OperatingProfitMinor),
				"Contribution per unit: " + money(projection.ContributionPerUnitMinor),
			}
			if projection.ContributionMarginBasisPoints == nil {
				results = append(results, "Contribution margin: not established")
			} else {
				results = append(results, fmt.Sprintf("Contribution margin: %d basis points (integer-truncated scenario calculation)", *projection.ContributionMarginBasisPoints))
			}
			if projection.BreakEvenUnits == nil {
				results = append(results, "Break-even units: not established")
			} else {
				results = append(results, fmt.Sprintf("Break-even units: %d", *projection.BreakEvenUnits))
			}
			if projection.BreakEvenRevenueMinor == nil {
				results = append(results, "Break-even revenue: not established")
			} else {
				results = append(results, "Break-even revenue: "+money(*projection.BreakEvenRevenueMinor))
			}
			add("commercial-results", "Scenario unit economics", "Derived from the exact declared inputs above. These figures are not actual charges, sales, profit or a guarantee of break-even.", results)
			if len(projection.Decisions) > 0 {
				add("commercial-decisions", "Decisions before quoting", "Resolve these saved-scenario conditions before making a commercial commitment.", projection.Decisions)
			}
			value.Assumptions = append(value.Assumptions, input.Assumptions...)
		}
		experiments := []string{}
		for _, experiment := range content.Experiments {
			experiments = append(experiments, experiment.ID+": "+experiment.Title+"\nHypothesis: "+experiment.Hypothesis+"\nMethod: "+experiment.Method+"\nSuccess criterion: "+experiment.SuccessCriterion+"\nSegment: "+experiment.SegmentID+"; channel: "+experiment.ChannelID+"\nReported status: "+experiment.Status+"\nReported result: "+experiment.Result)
		}
		add("commercial-experiments", "Saved experiments to review", "These are the experiments and user-reported results in the linked GTM revision. No experiment, payment flow or market validation is executed by preparing this document.", experiments)
		value.Gaps = append(value.Gaps, "Choose the offer's billing unit, tax treatment, payment schedule, discounts, refund/cancellation rules and commercial approval before treating a scenario as a quote.")
	case "support-guide":
		workflows := []string{}
		for _, screen := range currentDesignPlan(state).Screens {
			workflows = append(workflows, screen.ID+": "+screen.Name+" — "+screen.Purpose)
		}
		add("support-start", "Getting started", "Use the saved planned screens below to write and review the supported user steps. They describe intended workflows, not proof that every step works in the selected release.", workflows, ref, fmt.Sprintf("design:%d:%s", value.Source.DesignRevision, value.Source.DesignDigest))
		add("support-audience", "FAQ: who is this for?", "The saved audience, offer and intended value above are the basis for this answer. Edit the wording to match the supported release; do not promise unverified outcomes.", audiences)
		if model := content.CommercialModel; model != nil {
			interval := map[string]string{"month": "per month", "year": "per year", "one_time": "one time"}[model.Interval]
			add("support-pricing", "FAQ: what does it cost?", fmt.Sprintf("Saved scenario: %s %s %s per unit. This is a declared pricing assumption, not a published offer. Review the exact billing unit, included usage, tax and terms before using it in customer guidance.", model.Currency, launchMoney(model.PricePerUnitMinor, model.MinorUnitDigits), interval), nil)
		} else {
			add("support-pricing", "FAQ: what does it cost?", "No commercial scenario is saved in this GTM revision. Choose and review the offer before answering pricing questions; no free plan or paid price is assumed.", nil)
		}
		add("support-troubleshooting", "Troubleshooting and known limitations", "Record the affected user step, expected and observed behavior, a redacted reproduction and a supported workaround. Use the source criteria and any selected historical check/release evidence below; do not describe an untested repair as resolved.", []string{"Ask users not to share passwords, tokens, private customer records or unredacted application logs.", "Choose an escalation path and supported recovery procedure for issues without a reviewed workaround."})
		add("support-contact", "FAQ: how do I get help?", "Draft choice: name the support contact, channel, availability and escalation owner. No support hours, response time or service-level commitment has been selected by this generator.", nil)
		add("support-refunds", "FAQ: refunds and cancellation", "Draft choice: review the applicable refund, cancellation and renewal terms with the responsible owner. No entitlement, deadline or legal commitment is inferred from the price scenario.", nil)
		add("support-privacy", "FAQ: data and legal terms", "Draft choice: link the reviewed privacy, retention, deletion and product terms that apply to the actual release. Saved requirements and imported controls do not establish legal compliance or a retention promise.", nil)
		value.Gaps = append(value.Gaps, "Review the getting-started steps, troubleshooting, support ownership, hours, refund/cancellation and privacy/legal answers before customer use. These draft choices are not commitments.")
	default:
		return DistributeArtifact{}, errArtifact
	}
	value.Sections = append(sections, value.Sections...)
	value.Gaps = append(value.Gaps, "This private draft has not been published or sent. Review claims against the actual supported product and chosen commercial terms.")
	return boundedCommercialArtifact(value, saved), nil
}
