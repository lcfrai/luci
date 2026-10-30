package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

var errLaunchGTM = errors.New("Save a GTM revision linked to the current project context before preparing launch copy.")

func launchArtifactKind(kind string) bool {
	return kind == "landing-copy" || kind == "launch-brief" || kind == "channel-copy"
}

// Caller holds the existing s.mu -> teamControlsMu order. Neither prepare nor
// readback writes a strategy or changes its inputs to manufacture freshness.
func (s *Server) bindLaunchGTMSourceLocked(p project.Project, source DistributeArtifactSource) (DistributeArtifactSource, GTMWorkspace, error) {
	_, ledger, err := s.readGTMLedger(p)
	if err != nil {
		return source, GTMWorkspace{}, err
	}
	if len(ledger.Revisions) == 0 {
		return source, GTMWorkspace{}, errLaunchGTM
	}
	saved := ledger.Revisions[len(ledger.Revisions)-1]
	// An optional accepted run remains a separate exact evidence join. GTM is
	// saved against the same underlying project/control context without a run.
	base := source
	base.GTMRevision, base.GTMDigest = 0, ""
	base.SelectedEvidence = nil
	base.Checks, base.Release = "not-selected", "not-selected"
	if saved.ProjectID != p.ID || saved.SourceDigest != reviewDigest(base) {
		return source, GTMWorkspace{}, errLaunchGTM
	}
	source.GTMRevision, source.GTMDigest = saved.Revision, saved.Digest
	return source, saved, nil
}

func (s *Server) artifactSourceForKindLocked(ctx context.Context, p project.Project, kind, runID string) (DistributeArtifactSource, journeyState, *TeamControlRevision, *ControlAssessment, error) {
	source, state, library, assessment, err := s.artifactSource(ctx, p, runID)
	if err == nil && launchArtifactKind(kind) {
		source, _, err = s.bindLaunchGTMSourceLocked(p, source)
	}
	return source, state, library, assessment, err
}

func (s *Server) artifactDraftForKindLocked(ctx context.Context, p project.Project, kind, runID string) (DistributeArtifact, error) {
	source, state, library, assessment, err := s.artifactSource(ctx, p, runID)
	if err != nil {
		return DistributeArtifact{}, err
	}
	if !launchArtifactKind(kind) {
		return generatedArtifact(kind, p.ID, source, state, library, assessment), nil
	}
	source, gtm, err := s.bindLaunchGTMSourceLocked(p, source)
	if err != nil {
		return DistributeArtifact{}, err
	}
	return generatedLaunchAsset(generatedArtifactBase(kind, p.ID, source, state, library, assessment), gtm), nil
}

// No model, provider, publishing engine or external channel is invoked. These
// are editable starting drafts from the exact saved scenario, not new evidence.
func generatedLaunchAsset(value DistributeArtifact, saved GTMWorkspace) DistributeArtifact {
	content := saved.Content
	ref := fmt.Sprintf("gtm:%d:%s", saved.Revision, saved.Digest)
	copySections := []DistributeArtifactSection{}
	add := func(id, heading, body string, bullets []string) {
		if bullets == nil {
			bullets = []string{}
		}
		copySections = append(copySections, DistributeArtifactSection{ID: "launch-" + id, Heading: heading, Body: body, Bullets: bullets, SourceRefs: []string{ref}})
	}
	names, problems, values := []string{}, []string{}, []string{}
	for _, segment := range content.Segments {
		names = append(names, segment.ID+": "+segment.Name)
		problems = append(problems, segment.ID+": "+segment.Problem)
		values = append(values, segment.ID+": "+segment.Value)
	}
	switch value.Kind {
	case "landing-copy":
		add("headline", "Headline draft", content.Positioning, nil)
		add("offer", "Offer and next step", content.Offer, []string{"Call to action draft: Ask about the offer."})
		add("audience", "Audience candidates", "Choose the intended audience before using this copy.", names)
		add("benefits", "Value statements to review", "These are the saved segment value assumptions, not verified outcomes.", values)
	case "channel-copy":
		add("subject", "Subject or opening line", content.Positioning, nil)
		message := strings.TrimSpace(content.Positioning + "\n\n" + content.Offer)
		if message != "" {
			message += "\n\nReply if you would like to discuss this idea."
		}
		add("message", "Channel or email body draft", message, nil)
		add("audience", "Audience candidates", "Choose the intended recipient and channel before using this draft.", names)
		channels := []string{}
		for _, channel := range content.Channels {
			channels = append(channels, channel.Name+": "+channel.Hypothesis)
		}
		add("channels", "Saved channel hypotheses", "No channel was contacted and no delivery is claimed.", channels)
	case "launch-brief":
		add("positioning", "Positioning and offer", content.Positioning+"\n\n"+content.Offer, nil)
		add("audience", "Customer segments", "Saved intended audiences; demand remains a hypothesis.", names)
		add("problems", "Problems to investigate", "Use these saved problems to guide launch feedback.", problems)
		add("value", "Value to test", "Review these promises against product capability before external use.", values)
		channels, milestones, experiments := []string{}, []string{}, []string{}
		for _, channel := range content.Channels {
			channels = append(channels, channel.Name+": "+channel.Hypothesis)
		}
		for _, milestone := range content.Milestones {
			milestones = append(milestones, milestone.ID+": "+milestone.Title+" — "+milestone.Outcome)
		}
		for _, experiment := range content.Experiments {
			experiments = append(experiments, experiment.ID+": "+experiment.Title+" — "+experiment.Hypothesis)
		}
		add("channels", "Channels to test", "Saved channel hypotheses; no campaign has been launched.", channels)
		add("milestones", "Launch milestones", "These are planned outcomes, not completed releases.", milestones)
		add("experiments", "Experiments to review", "Read the linked GTM revision for methods, success criteria and any separately reported results.", experiments)
	}
	if len(content.ArtifactLinks) > 0 {
		links := []string{}
		for _, link := range content.ArtifactLinks {
			links = append(links, fmt.Sprintf("%s · revision %d · %s", link.ID, link.Revision, link.Digest))
		}
		add("artifact-links", "Saved GTM artifact references", "These exact references were retained in the saved strategy; review their contents before making an external claim.", links)
	}
	value.Sections = append(copySections, value.Sections...)
	value.Assumptions = append(value.Assumptions, "Customer, competitor, channel and pricing statements are saved GTM assumptions; this draft is not market validation.")
	value.Assumptions = append(value.Assumptions, content.PricingAssumptions...)
	if model := content.CommercialModel; model != nil {
		value.Assumptions = append(value.Assumptions, model.Assumptions...)
		if model.PricePerUnitMinor > 0 {
			interval := map[string]string{"month": "per month", "year": "per year", "one_time": "one time"}[model.Interval]
			section := DistributeArtifactSection{ID: "launch-pricing", Heading: "Pricing scenario for review", Body: fmt.Sprintf("%s %s %s per unit. This is a saved price assumption, not a quote or revenue result.", model.Currency, launchMoney(model.PricePerUnitMinor, model.MinorUnitDigits), interval), Bullets: []string{}, SourceRefs: []string{ref}}
			value.Sections = append(value.Sections, section)
		} else {
			value.Gaps = append(value.Gaps, "No positive price has been chosen for this offer.")
		}
	} else {
		value.Gaps = append(value.Gaps, "No commercial price scenario was selected; review price and terms before making a paid offer.")
	}
	if strings.TrimSpace(content.Positioning) == "" {
		value.Gaps = append(value.Gaps, "Write and save the GTM positioning before finalizing the headline.")
	}
	if strings.TrimSpace(content.Offer) == "" {
		value.Gaps = append(value.Gaps, "Write and save the GTM offer before finalizing this copy.")
	}
	if len(content.Segments) == 0 {
		value.Gaps = append(value.Gaps, "No intended customer segment is saved.")
	}
	value.Gaps = append(value.Gaps, "Choose the exact audience, destination and next step before external use. Nothing has been published or sent.")
	return value
}

func launchMoney(minor int64, digits int) string {
	scale := int64(1)
	for i := 0; i < digits; i++ {
		scale *= 10
	}
	if digits == 0 {
		return fmt.Sprintf("%d", minor)
	}
	return fmt.Sprintf("%d.%0*d", minor/scale, digits, minor%scale)
}
