package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CommercialModelInput is a user-authored scenario. Costs and units use the
// selected interval; amounts are integers in explicitly selected minor units.
// It contains no currency conversion, inferred market price or observed revenue.
type CommercialModelInput struct {
	Currency                 string   `json:"currency"`
	MinorUnitDigits          int      `json:"minorUnitDigits"`
	Interval                 string   `json:"interval"`
	PricePerUnitMinor        int64    `json:"pricePerUnitMinor"`
	VariableCostPerUnitMinor int64    `json:"variableCostPerUnitMinor"`
	FixedCostMinor           int64    `json:"fixedCostMinor"`
	Units                    int64    `json:"units"`
	Assumptions              []string `json:"assumptions"`
}

type CommercialModelProjection struct {
	Version                       int                  `json:"version"`
	Input                         CommercialModelInput `json:"input"`
	Digest                        string               `json:"digest"`
	RevenueMinor                  int64                `json:"revenueMinor"`
	VariableCostMinor             int64                `json:"variableCostMinor"`
	TotalCostMinor                int64                `json:"totalCostMinor"`
	OperatingProfitMinor          int64                `json:"operatingProfitMinor"`
	ContributionPerUnitMinor      int64                `json:"contributionPerUnitMinor"`
	ContributionMarginBasisPoints *int64               `json:"contributionMarginBasisPoints"`
	BreakEvenUnits                *int64               `json:"breakEvenUnits"`
	BreakEvenRevenueMinor         *int64               `json:"breakEvenRevenueMinor"`
	Decisions                     []string             `json:"decisions"`
}

var commercialCurrency = regexp.MustCompile("^[A-Z]{3}$")

// CompileCommercialModel computes a bounded, deterministic scenario. Every
// possible intermediate is below the exact-integer limit in JSON/JavaScript:
// amounts <=1e9, units <=1e6, totals <=1e15+1e9, margin numerator <=1e13.
// Break-even revenue has its own exact-integer check before multiplication.
func CompileCommercialModel(input CommercialModelInput) (CommercialModelProjection, error) {
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if !commercialCurrency.MatchString(input.Currency) || input.MinorUnitDigits < 0 || input.MinorUnitDigits > 6 {
		return CommercialModelProjection{}, fmt.Errorf("Choose a three-letter currency code and between zero and six minor-unit digits.")
	}
	if input.Interval != "month" && input.Interval != "year" && input.Interval != "one_time" {
		return CommercialModelProjection{}, fmt.Errorf("Choose monthly, yearly or one-time scenario costs.")
	}
	for _, amount := range []int64{input.PricePerUnitMinor, input.VariableCostPerUnitMinor, input.FixedCostMinor} {
		if amount < 0 || amount > 1_000_000_000 {
			return CommercialModelProjection{}, fmt.Errorf("Scenario amounts must be between zero and one billion minor units.")
		}
	}
	if input.Units < 0 || input.Units > 1_000_000 || len(input.Assumptions) > 30 {
		return CommercialModelProjection{}, fmt.Errorf("Use at most one million units and thirty explicit assumptions.")
	}
	assumptions := make([]string, 0, len(input.Assumptions))
	for _, assumption := range input.Assumptions {
		assumption = strings.TrimSpace(assumption)
		if !validArtifactStoredText(assumption) || assumption == "" || len(assumption) > 1000 {
			return CommercialModelProjection{}, fmt.Errorf("Each assumption needs between one and one thousand UTF-8 bytes.")
		}
		assumptions = append(assumptions, assumption)
	}
	input.Assumptions = assumptions
	out := CommercialModelProjection{Version: 1, Input: input, Decisions: []string{}}
	out.RevenueMinor = input.PricePerUnitMinor * input.Units
	out.VariableCostMinor = input.VariableCostPerUnitMinor * input.Units
	out.TotalCostMinor = out.VariableCostMinor + input.FixedCostMinor
	out.OperatingProfitMinor = out.RevenueMinor - out.TotalCostMinor
	out.ContributionPerUnitMinor = input.PricePerUnitMinor - input.VariableCostPerUnitMinor
	if input.PricePerUnitMinor == 0 {
		out.Decisions = append(out.Decisions, "Set a price to calculate the contribution margin.")
	} else {
		margin := out.ContributionPerUnitMinor * 10_000 / input.PricePerUnitMinor
		out.ContributionMarginBasisPoints = &margin
	}
	if out.ContributionPerUnitMinor <= 0 {
		out.Decisions = append(out.Decisions, "Price must exceed variable cost before additional units can cover fixed costs.")
	} else {
		units := input.FixedCostMinor / out.ContributionPerUnitMinor
		if input.FixedCostMinor%out.ContributionPerUnitMinor != 0 {
			units++
		}
		out.BreakEvenUnits = &units
		if units > 9_007_199_254_740_991/input.PricePerUnitMinor {
			out.Decisions = append(out.Decisions, "Break-even revenue exceeds the supported exact-integer range; review the pricing scenario.")
		} else {
			revenue := units * input.PricePerUnitMinor
			out.BreakEvenRevenueMinor = &revenue
		}
	}
	if input.Units == 0 {
		out.Decisions = append(out.Decisions, "Add a unit or customer assumption to compare revenue and costs.")
	}
	if len(input.Assumptions) == 0 {
		out.Decisions = append(out.Decisions, "Record the demand, price and cost assumptions behind this scenario.")
	}
	canonical, err := json.Marshal(struct {
		Version int                  `json:"version"`
		Input   CommercialModelInput `json:"input"`
	}{out.Version, input})
	if err != nil {
		return CommercialModelProjection{}, err
	}
	digest := sha256.Sum256(canonical)
	out.Digest = hex.EncodeToString(digest[:])
	return out, nil
}
