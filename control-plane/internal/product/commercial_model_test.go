package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func commercialFixture() CommercialModelInput {
	return CommercialModelInput{Currency: "AUD", MinorUnitDigits: 2, Interval: "month", PricePerUnitMinor: 2900, VariableCostPerUnitMinor: 900, FixedCostMinor: 100000, Units: 1000, Assumptions: []string{"1,000 paying customers at the proposed monthly price."}}
}

func TestCommercialModelComputesRevenueContributionAndBreakEven(t *testing.T) {
	input := commercialFixture()
	out, err := CompileCommercialModel(input)
	if err != nil {
		t.Fatal(err)
	}
	if out.RevenueMinor != 2900000 || out.VariableCostMinor != 900000 || out.TotalCostMinor != 1000000 || out.OperatingProfitMinor != 1900000 || out.ContributionPerUnitMinor != 2000 {
		t.Fatalf("wrong economics: %+v", out)
	}
	if out.ContributionMarginBasisPoints == nil || *out.ContributionMarginBasisPoints != 6896 || out.BreakEvenUnits == nil || *out.BreakEvenUnits != 50 || out.BreakEvenRevenueMinor == nil || *out.BreakEvenRevenueMinor != 145000 {
		t.Fatalf("wrong contribution/break-even: %+v", out)
	}
	if len(out.Decisions) != 0 || len(out.Digest) != 64 || out.Version != 1 {
		t.Fatalf("invalid scenario identity: %+v", out)
	}
	input.FixedCostMinor++
	rounded, err := CompileCommercialModel(input)
	if err != nil || rounded.BreakEvenUnits == nil || *rounded.BreakEvenUnits != 51 {
		t.Fatalf("must round break-even units up: %+v %v", rounded, err)
	}
	input.FixedCostMinor = 0
	free, err := CompileCommercialModel(input)
	if err != nil || free.BreakEvenUnits == nil || *free.BreakEvenUnits != 0 {
		t.Fatalf("zero fixed cost: %+v %v", free, err)
	}
}

func TestCommercialModelKeepsMissingAndUnprofitableScenariosExplicit(t *testing.T) {
	input := commercialFixture()
	input.PricePerUnitMinor = 0
	input.Units = 0
	input.Assumptions = nil
	out, err := CompileCommercialModel(input)
	if err != nil || out.ContributionMarginBasisPoints != nil || out.BreakEvenUnits != nil || out.BreakEvenRevenueMinor != nil || len(out.Decisions) != 4 {
		t.Fatalf("zero price must stay unresolved: %+v %v", out, err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"\"contributionMarginBasisPoints\":null", "\"breakEvenUnits\":null", "\"breakEvenRevenueMinor\":null"} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("unresolved API field missing: %s", raw)
		}
	}
	input.PricePerUnitMinor = 900
	out, err = CompileCommercialModel(input)
	if err != nil || out.ContributionMarginBasisPoints == nil || *out.ContributionMarginBasisPoints != 0 || out.BreakEvenUnits != nil {
		t.Fatalf("zero contribution: %+v %v", out, err)
	}
	input.PricePerUnitMinor = 800
	input.Units = 1000
	out, err = CompileCommercialModel(input)
	if err != nil || out.OperatingProfitMinor != -200000 || out.ContributionPerUnitMinor != -100 || out.BreakEvenUnits != nil {
		t.Fatalf("negative contribution must not invent break-even: %+v %v", out, err)
	}
}

func TestCommercialModelCanonicalIdentityTracksAllScenarioAssumptions(t *testing.T) {
	input := commercialFixture()
	before, err := CompileCommercialModel(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Currency = " aud "
	input.Assumptions[0] = "  " + input.Assumptions[0] + "  "
	same, err := CompileCommercialModel(input)
	if err != nil || same.Digest != before.Digest || same.Input.Currency != "AUD" {
		t.Fatalf("canonical input differs: %+v %v", same, err)
	}
	input.Assumptions[0] = "An unvalidated lower demand estimate."
	changed, err := CompileCommercialModel(input)
	if err != nil || changed.Digest == before.Digest {
		t.Fatal("assumptions must change the scenario digest")
	}
	input.Interval = "year"
	annual, err := CompileCommercialModel(input)
	if err != nil || annual.Digest == changed.Digest || annual.RevenueMinor != changed.RevenueMinor {
		t.Fatal("interval must be explicit; never silently annualize values")
	}
	if _, err := json.Marshal(annual); err != nil {
		t.Fatal(err)
	}
}

func TestCommercialModelRetainsExactIntegersAtSupportedBounds(t *testing.T) {
	input := commercialFixture()
	input.PricePerUnitMinor = 1000000000
	input.VariableCostPerUnitMinor = 999999999
	input.FixedCostMinor = 1000000000
	input.Units = 1000000
	out, err := CompileCommercialModel(input)
	if err != nil || out.RevenueMinor != 1000000000000000 || out.BreakEvenUnits == nil || *out.BreakEvenUnits != 1000000000 || out.BreakEvenRevenueMinor != nil {
		t.Fatalf("unsafe break-even revenue must stay absent: %+v %v", out, err)
	}
	if !strings.Contains(strings.Join(out.Decisions, " "), "exact-integer range") {
		t.Fatal("missing numeric-range decision")
	}
	input.PricePerUnitMinor = 1
	input.VariableCostPerUnitMinor = 1000000000
	out, err = CompileCommercialModel(input)
	if err != nil || out.ContributionMarginBasisPoints == nil || *out.ContributionMarginBasisPoints != -9999999990000 {
		t.Fatalf("large negative margin lost precision: %+v %v", out, err)
	}
}

func TestCommercialModelRejectsInvalidScenarioInputs(t *testing.T) {
	cases := []struct {
		name   string
		change func(*CommercialModelInput)
	}{
		{"currency", func(x *CommercialModelInput) { x.Currency = "US dollars" }},
		{"negative digits", func(x *CommercialModelInput) { x.MinorUnitDigits = -1 }},
		{"too many digits", func(x *CommercialModelInput) { x.MinorUnitDigits = 7 }},
		{"interval", func(x *CommercialModelInput) { x.Interval = "weekly" }},
		{"negative price", func(x *CommercialModelInput) { x.PricePerUnitMinor = -1 }},
		{"oversize price", func(x *CommercialModelInput) { x.PricePerUnitMinor = 1000000001 }},
		{"negative variable cost", func(x *CommercialModelInput) { x.VariableCostPerUnitMinor = -1 }},
		{"oversize variable cost", func(x *CommercialModelInput) { x.VariableCostPerUnitMinor = 1000000001 }},
		{"negative fixed cost", func(x *CommercialModelInput) { x.FixedCostMinor = -1 }},
		{"oversize fixed cost", func(x *CommercialModelInput) { x.FixedCostMinor = 1000000001 }},
		{"negative units", func(x *CommercialModelInput) { x.Units = -1 }},
		{"oversize units", func(x *CommercialModelInput) { x.Units = 1000001 }},
		{"blank assumption", func(x *CommercialModelInput) { x.Assumptions = []string{"  "} }},
		{"oversize assumption", func(x *CommercialModelInput) { x.Assumptions = []string{strings.Repeat("a", 1001)} }},
		{"invalid UTF8", func(x *CommercialModelInput) { x.Assumptions = []string{string([]byte{0xff})} }},
		{"control character", func(x *CommercialModelInput) { x.Assumptions = []string{"Demand\x00assumption"} }},
		{"too many assumptions", func(x *CommercialModelInput) { x.Assumptions = make([]string, 31) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := commercialFixture()
			test.change(&input)
			if _, err := CompileCommercialModel(input); err == nil {
				t.Fatal("invalid scenario accepted")
			}
		})
	}
}
