// Package businesscase turns an organisation's own numbers into the one-page
// argument Masterclass 3's lab asks for.
//
// # Why this is not an ROI calculator
//
// Chapter 26 gives cited public figures for the link between latency and
// revenue, and then says plainly that "turning this into monetary estimates is
// harder to give guidance on." That restraint is deliberate and worth keeping:
// a tool that emits one confident number invites a CFO to attack the number
// instead of engaging with the argument, and the number will not survive
// scrutiny because most of its inputs are assumptions.
//
// So this package does three things instead:
//
//   - It computes the cost of the current state from numbers a leader can
//     actually look up: incident count, time to resolve, who gets pulled in,
//     and how much engineering time goes to unplanned work.
//   - It shows the arithmetic for every line, so each figure can be checked or
//     disputed on its own.
//   - It reports a range across three explicitly-labelled improvement
//     assumptions rather than a point estimate.
//
// The output is an argument with your numbers in it, not a forecast.
package businesscase

import (
	"fmt"
	"strings"
)

// FormatMoney renders a whole-dollar amount with thousands separators. Written
// out rather than taken from a dependency because this repo is meant to be
// read by attendees, and it is shared between the formulas below and the CLI so
// the two cannot drift.
func FormatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}

	digits := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(digit)
	}
	return b.String()
}

// WorkingHoursPerYear converts an annual loaded salary into an hourly rate.
// 2,080 is 40 hours across 52 weeks; it deliberately ignores leave, which makes
// the resulting hourly rate slightly conservative — a defensible direction to
// err when the audience is finance.
const WorkingHoursPerYear = 2080

// Inputs are the numbers a leader supplies. Every one of them is observable
// from an incident tracker, a payroll system, or a deploy pipeline; none of
// them requires a survey or a consultant.
type Inputs struct {
	// Engineers in the organisation whose work this covers.
	Engineers int
	// LoadedCostPerEngineer is annual fully-loaded cost: salary, benefits,
	// taxes, equipment. Finance will have this number and it is usually well
	// above base salary.
	LoadedCostPerEngineer float64

	// IncidentsPerMonth counts incidents that pull someone away from planned
	// work — not only customer-visible outages.
	IncidentsPerMonth float64
	// MeanTimeToResolveHours is wall-clock from "someone noticed" to
	// "resolved", which is the interval engineers actually spend.
	MeanTimeToResolveHours float64
	// RespondersPerIncident is how many people are engaged on average. This is
	// the number most often underestimated: incident channels pull in
	// observers who stop doing their own work.
	RespondersPerIncident float64
	// EscalationRate is the share of incidents that cannot be resolved by
	// whoever picked them up and must be escalated. This is the
	// Two-or-Three People Test expressed as a cost.
	EscalationRate float64
	// EscalationExtraResponders is how many additional people an escalation
	// pulls in, on top of RespondersPerIncident.
	EscalationExtraResponders float64

	// UnplannedWorkShare is the fraction of total engineering time spent on
	// unplanned work and rework rather than on roadmap. Chapter 27's
	// "firefighting trap" in numeric form.
	UnplannedWorkShare float64

	// RevenuePerHourAtRisk is revenue exposed per hour of degradation. Leave
	// at zero if you cannot defend a figure; the case then rests on
	// engineering cost alone, which is often the stronger ground anyway.
	RevenuePerHourAtRisk float64

	// ObservabilityAnnualSpend is current tooling spend, for comparison. Zero
	// is fine.
	ObservabilityAnnualSpend float64
}

// HourlyRate is the fully-loaded cost of one engineer-hour.
func (in Inputs) HourlyRate() float64 {
	if in.LoadedCostPerEngineer == 0 {
		return 0
	}
	return in.LoadedCostPerEngineer / WorkingHoursPerYear
}

// IncidentsPerYear is the annualised incident count.
func (in Inputs) IncidentsPerYear() float64 {
	return in.IncidentsPerMonth * 12
}

// LineItem is one component of the current-state cost, carrying the formula
// that produced it so the figure can be audited rather than trusted.
type LineItem struct {
	Label   string
	Formula string
	Annual  float64
}

// Scenario is one improvement assumption and what it would be worth. The
// reductions are inputs, not findings — Name exists so the output can say so.
type Scenario struct {
	Name string
	// MTTRReduction and UnplannedWorkReduction are fractions between 0 and 1.
	MTTRReduction          float64
	UnplannedWorkReduction float64
	// AnnualSaving is what those reductions are worth against the current
	// state.
	AnnualSaving float64
}

// Case is the assembled argument.
type Case struct {
	Inputs       Inputs
	CurrentState []LineItem
	CurrentTotal float64
	Scenarios    []Scenario
}

// DefaultScenarios are the three improvement assumptions the output reports.
//
// These are ranges to reason about, not measurements. They are deliberately
// wide and the conservative case is deliberately unimpressive, because a
// business case that only works at its optimistic end is not a business case.
func DefaultScenarios() []Scenario {
	return []Scenario{
		{Name: "Conservative", MTTRReduction: 0.15, UnplannedWorkReduction: 0.05},
		{Name: "Moderate", MTTRReduction: 0.35, UnplannedWorkReduction: 0.15},
		{Name: "Optimistic", MTTRReduction: 0.60, UnplannedWorkReduction: 0.25},
	}
}

// Build assembles the case. Scenarios may be nil to use DefaultScenarios.
func Build(in Inputs, scenarios []Scenario) Case {
	if scenarios == nil {
		scenarios = DefaultScenarios()
	}

	hourly := in.HourlyRate()
	incidents := in.IncidentsPerYear()

	responderHours := incidents * in.MeanTimeToResolveHours * in.RespondersPerIncident
	escalationHours := incidents * in.EscalationRate * in.EscalationExtraResponders * in.MeanTimeToResolveHours

	items := []LineItem{
		{
			Label:   "Incident response labour",
			Formula: fmt.Sprintf("%.0f incidents/yr x %.1fh x %.1f responders x $%s/h", incidents, in.MeanTimeToResolveHours, in.RespondersPerIncident, FormatMoney(hourly)),
			Annual:  responderHours * hourly,
		},
		{
			Label:   "Escalation drag",
			Formula: fmt.Sprintf("%.0f incidents/yr x %.0f%% escalated x %.1f extra people x %.1fh x $%s/h", incidents, in.EscalationRate*100, in.EscalationExtraResponders, in.MeanTimeToResolveHours, FormatMoney(hourly)),
			Annual:  escalationHours * hourly,
		},
		{
			Label:   "Unplanned work and rework",
			Formula: fmt.Sprintf("%d engineers x $%s loaded x %.0f%% of time", in.Engineers, FormatMoney(in.LoadedCostPerEngineer), in.UnplannedWorkShare*100),
			Annual:  float64(in.Engineers) * in.LoadedCostPerEngineer * in.UnplannedWorkShare,
		},
	}

	if in.RevenuePerHourAtRisk > 0 {
		items = append(items, LineItem{
			Label:   "Revenue exposure during degradation",
			Formula: fmt.Sprintf("%.0f incidents/yr x %.1fh x $%s/h at risk", incidents, in.MeanTimeToResolveHours, FormatMoney(in.RevenuePerHourAtRisk)),
			Annual:  incidents * in.MeanTimeToResolveHours * in.RevenuePerHourAtRisk,
		})
	}

	var total float64
	for _, it := range items {
		total += it.Annual
	}

	// Only the time-to-resolve-sensitive lines shrink with MTTR; unplanned
	// work is its own lever. Keeping them separate stops a single optimistic
	// assumption from moving the entire total.
	var mttrSensitive float64
	for _, it := range items {
		switch it.Label {
		case "Incident response labour", "Escalation drag", "Revenue exposure during degradation":
			mttrSensitive += it.Annual
		}
	}
	unplanned := float64(in.Engineers) * in.LoadedCostPerEngineer * in.UnplannedWorkShare

	out := make([]Scenario, 0, len(scenarios))
	for _, s := range scenarios {
		s.AnnualSaving = mttrSensitive*s.MTTRReduction + unplanned*s.UnplannedWorkReduction
		out = append(out, s)
	}

	return Case{Inputs: in, CurrentState: items, CurrentTotal: total, Scenarios: out}
}

// NetOf returns a scenario's saving less current observability spend. Negative
// means the tooling costs more than this scenario recovers, which is a real
// possible answer and one the tool should be willing to print.
func (c Case) NetOf(s Scenario) float64 {
	return s.AnnualSaving - c.Inputs.ObservabilityAnnualSpend
}
