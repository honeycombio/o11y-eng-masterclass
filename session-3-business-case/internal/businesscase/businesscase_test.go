package businesscase

import (
	"math"
	"testing"
)

// sample is a mid-sized org with round numbers, so the expected values below
// can be checked by hand rather than copied from a previous run.
func sample() Inputs {
	return Inputs{
		Engineers:                 100,
		LoadedCostPerEngineer:     260000, // $125/h at 2080h
		IncidentsPerMonth:         10,     // 120/yr
		MeanTimeToResolveHours:    4,
		RespondersPerIncident:     3,
		EscalationRate:            0.5,
		EscalationExtraResponders: 2,
		UnplannedWorkShare:        0.20,
		RevenuePerHourAtRisk:      0,
		ObservabilityAnnualSpend:  400000,
	}
}

func TestHourlyRate(t *testing.T) {
	if got, want := sample().HourlyRate(), 125.0; got != want {
		t.Errorf("HourlyRate() = %v, want %v", got, want)
	}
}

// TestLineItemArithmetic checks each line against a hand-computed figure. These
// numbers end up in front of a CFO, so they get checked individually rather
// than only in aggregate.
func TestLineItemArithmetic(t *testing.T) {
	c := Build(sample(), nil)

	tests := []struct {
		label string
		want  float64
		how   string
	}{
		{"Incident response labour", 180000, "120 incidents x 4h x 3 people x $125"},
		{"Escalation drag", 60000, "120 x 0.5 x 2 extra x 4h x $125"},
		{"Unplanned work and rework", 5200000, "100 engineers x $260k x 20%"},
	}

	byLabel := map[string]float64{}
	for _, it := range c.CurrentState {
		byLabel[it.Label] = it.Annual
	}

	for _, tc := range tests {
		got, ok := byLabel[tc.label]
		if !ok {
			t.Errorf("no line item %q", tc.label)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %.0f, want %.0f (%s)", tc.label, got, tc.want, tc.how)
		}
	}

	if want := 180000.0 + 60000 + 5200000; c.CurrentTotal != want {
		t.Errorf("CurrentTotal = %.0f, want %.0f", c.CurrentTotal, float64(want))
	}
}

// TestRevenueLineIsOptional guards the honesty of the default: a leader who
// cannot defend a revenue-at-risk figure should not get one invented for them.
func TestRevenueLineIsOptional(t *testing.T) {
	withoutRevenue := Build(sample(), nil)
	for _, it := range withoutRevenue.CurrentState {
		if it.Label == "Revenue exposure during degradation" {
			t.Fatal("revenue line present when RevenuePerHourAtRisk is zero")
		}
	}

	in := sample()
	in.RevenuePerHourAtRisk = 10000
	withRevenue := Build(in, nil)

	var found bool
	for _, it := range withRevenue.CurrentState {
		if it.Label == "Revenue exposure during degradation" {
			found = true
			if want := 120.0 * 4 * 10000; it.Annual != want {
				t.Errorf("revenue exposure = %.0f, want %.0f", it.Annual, want)
			}
		}
	}
	if !found {
		t.Error("revenue line missing when RevenuePerHourAtRisk is set")
	}
}

// TestScenarioSavingsSeparateTheLevers is the important structural property.
// MTTR improvements must not be applied to unplanned work, and vice versa —
// otherwise one optimistic assumption inflates the whole total, which is the
// failure mode that makes these cases easy to dismiss.
func TestScenarioSavingsSeparateTheLevers(t *testing.T) {
	in := sample()

	onlyMTTR := Build(in, []Scenario{{Name: "mttr only", MTTRReduction: 0.5, UnplannedWorkReduction: 0}})
	onlyUnplanned := Build(in, []Scenario{{Name: "unplanned only", MTTRReduction: 0, UnplannedWorkReduction: 0.5}})
	both := Build(in, []Scenario{{Name: "both", MTTRReduction: 0.5, UnplannedWorkReduction: 0.5}})

	// MTTR-sensitive lines total 240,000; half is 120,000.
	if got, want := onlyMTTR.Scenarios[0].AnnualSaving, 120000.0; got != want {
		t.Errorf("MTTR-only saving = %.0f, want %.0f", got, want)
	}
	// Unplanned work is 5,200,000; half is 2,600,000.
	if got, want := onlyUnplanned.Scenarios[0].AnnualSaving, 2600000.0; got != want {
		t.Errorf("unplanned-only saving = %.0f, want %.0f", got, want)
	}
	if got, want := both.Scenarios[0].AnnualSaving, 2720000.0; got != want {
		t.Errorf("combined saving = %.0f, want %.0f (the two levers must simply add)", got, want)
	}
}

// TestDefaultScenariosAreOrderedAndConservativeFirst keeps the output honest:
// if the conservative case were as good as the optimistic one, the range would
// be decoration.
func TestDefaultScenariosAreOrderedAndConservative(t *testing.T) {
	c := Build(sample(), nil)

	if len(c.Scenarios) != 3 {
		t.Fatalf("got %d scenarios, want 3", len(c.Scenarios))
	}
	for i := 1; i < len(c.Scenarios); i++ {
		if c.Scenarios[i].AnnualSaving <= c.Scenarios[i-1].AnnualSaving {
			t.Errorf("scenario %q does not exceed %q; the range is not ordered",
				c.Scenarios[i].Name, c.Scenarios[i-1].Name)
		}
	}

	spread := c.Scenarios[2].AnnualSaving / c.Scenarios[0].AnnualSaving
	if spread < 2 {
		t.Errorf("optimistic is only %.1fx conservative; the range is too narrow to be an honest range", spread)
	}
}

// TestNetOfCanBeNegative asserts the tool will report an unfavourable answer.
// A calculator that cannot conclude "this does not pay for itself" is a
// marketing instrument, not an analysis.
func TestNetOfCanBeNegative(t *testing.T) {
	in := sample()
	in.Engineers = 5
	in.IncidentsPerMonth = 1
	in.UnplannedWorkShare = 0.02
	in.ObservabilityAnnualSpend = 500000

	c := Build(in, nil)
	if net := c.NetOf(c.Scenarios[0]); net >= 0 {
		t.Errorf("conservative net = %.0f for a tiny org on a large contract; expected negative", net)
	}
}

// TestZeroInputsDoNotPanic covers the first-run case where a leader has only
// some of the numbers to hand.
func TestZeroInputsDoNotPanic(t *testing.T) {
	c := Build(Inputs{}, nil)

	if c.CurrentTotal != 0 {
		t.Errorf("CurrentTotal = %v for zero inputs, want 0", c.CurrentTotal)
	}
	for _, s := range c.Scenarios {
		if math.IsNaN(s.AnnualSaving) || math.IsInf(s.AnnualSaving, 0) {
			t.Errorf("scenario %q produced %v", s.Name, s.AnnualSaving)
		}
	}
}

// TestFormatMoney covers the boundaries where hand-rolled separator logic
// usually breaks: the exact multiples of 1000, and negatives.
func TestFormatMoney(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{999999, "999,999"},
		{1000000, "1,000,000"},
		{5440000, "5,440,000"},
		{-104000, "-104,000"},
		{-1000, "-1,000"},
		{-7, "-7"},
	}

	for _, tc := range tests {
		if got := FormatMoney(tc.in); got != tc.want {
			t.Errorf("FormatMoney(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
