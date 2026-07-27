// business-case prints the one-page argument Masterclass 3's lab asks you to
// build, from your own numbers.
//
//	go run ./cmd/business-case \
//	  -engineers 120 -loaded-cost 260000 \
//	  -incidents-per-month 14 -mttr-hours 3.5 -responders 3 \
//	  -escalation-rate 0.45 -escalation-extra 2 \
//	  -unplanned-share 0.22 -observability-spend 350000
//
// Run it with no flags to see the defaults it would use and a warning that they
// are placeholders, not a template you should present.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/honeycombio/o11y-eng-masterclass/session-3-business-case/internal/businesscase"
)

func main() {
	var in businesscase.Inputs
	var showSources bool

	flag.IntVar(&in.Engineers, "engineers", 0, "engineers in scope")
	flag.Float64Var(&in.LoadedCostPerEngineer, "loaded-cost", 0, "annual fully-loaded cost per engineer (ask finance; it is well above base salary)")
	flag.Float64Var(&in.IncidentsPerMonth, "incidents-per-month", 0, "incidents per month that pull people off planned work")
	flag.Float64Var(&in.MeanTimeToResolveHours, "mttr-hours", 0, "mean wall-clock hours from noticed to resolved")
	flag.Float64Var(&in.RespondersPerIncident, "responders", 0, "people engaged per incident on average")
	flag.Float64Var(&in.EscalationRate, "escalation-rate", 0, "share of incidents that must be escalated (0-1)")
	flag.Float64Var(&in.EscalationExtraResponders, "escalation-extra", 0, "additional people an escalation pulls in")
	flag.Float64Var(&in.UnplannedWorkShare, "unplanned-share", 0, "share of engineering time on unplanned work and rework (0-1)")
	flag.Float64Var(&in.RevenuePerHourAtRisk, "revenue-per-hour", 0, "revenue exposed per hour of degradation; leave at 0 unless you can defend it")
	flag.Float64Var(&in.ObservabilityAnnualSpend, "observability-spend", 0, "current annual observability spend")
	flag.BoolVar(&showSources, "sources", false, "print the public figures Chapter 26 cites, with attribution")

	flag.Parse()

	if showSources {
		printSources()
		return
	}

	usingPlaceholders := in.Engineers == 0
	if usingPlaceholders {
		in = placeholders()
	}

	c := businesscase.Build(in, nil)
	print(c, usingPlaceholders)
}

// placeholders exist so the tool does something useful when run with no flags,
// but the output says loudly that they are not your numbers. Chapter 27's point
// is that the diagnosis has to rest on evidence; a borrowed set of inputs is
// the opposite of that.
func placeholders() businesscase.Inputs {
	return businesscase.Inputs{
		Engineers:                 100,
		LoadedCostPerEngineer:     260000,
		IncidentsPerMonth:         10,
		MeanTimeToResolveHours:    4,
		RespondersPerIncident:     3,
		EscalationRate:            0.5,
		EscalationExtraResponders: 2,
		UnplannedWorkShare:        0.20,
		ObservabilityAnnualSpend:  400000,
	}
}

func print(c businesscase.Case, usingPlaceholders bool) {
	out := os.Stdout

	if usingPlaceholders {
		fmt.Fprint(out, `
┌───────────────────────────────────────────────────────────────────────────┐
│  THESE ARE PLACEHOLDER NUMBERS. Do not present this.                      │
│  Rerun with your own -engineers, -incidents-per-month, -mttr-hours, and    │
│  -unplanned-share. The whole point of the exercise is that the figures     │
│  are yours and therefore hard to dismiss.                                 │
└───────────────────────────────────────────────────────────────────────────┘
`)
	}

	fmt.Fprintf(out, "\nCOST OF THE CURRENT STATE\n%s\n", strings.Repeat("=", 75))
	for _, it := range c.CurrentState {
		fmt.Fprintf(out, "\n  %-38s $%s / yr\n", it.Label, businesscase.FormatMoney(it.Annual))
		fmt.Fprintf(out, "  %s\n", it.Formula)
	}
	fmt.Fprintf(out, "\n  %-38s $%s / yr\n", "TOTAL", businesscase.FormatMoney(c.CurrentTotal))

	fmt.Fprintf(out, "\n\nWHAT IMPROVEMENT WOULD BE WORTH\n%s\n", strings.Repeat("=", 75))
	fmt.Fprint(out, `
  The percentages below are assumptions you are choosing, not findings. Say so
  out loud when you present this. A case that only works at its optimistic end
  is not a case.

`)
	fmt.Fprintf(out, "  %-14s %-22s %14s %14s\n", "Scenario", "Assumes", "Annual saving", "Net of spend")
	fmt.Fprintf(out, "  %s\n", strings.Repeat("-", 68))
	for _, s := range c.Scenarios {
		assumes := fmt.Sprintf("-%.0f%% MTTR, -%.0f%% rework", s.MTTRReduction*100, s.UnplannedWorkReduction*100)
		fmt.Fprintf(out, "  %-14s %-22s %14s %14s\n",
			s.Name, assumes, "$"+businesscase.FormatMoney(s.AnnualSaving), "$"+businesscase.FormatMoney(c.NetOf(s)))
	}

	if c.Inputs.ObservabilityAnnualSpend > 0 {
		fmt.Fprintf(out, "\n  Current observability spend: $%s / yr\n", businesscase.FormatMoney(c.Inputs.ObservabilityAnnualSpend))
	}

	if c.Inputs.RevenuePerHourAtRisk == 0 {
		fmt.Fprint(out, `
  No revenue-at-risk figure was supplied, so this case rests on engineering
  cost alone. That is usually the stronger ground: it is auditable, and nobody
  has to agree with your model of customer behaviour to accept it.
`)
	}

	fmt.Fprintf(out, `
WHAT THIS DOES NOT DO
%s

  It does not tell you whether observability caused an improvement. That is
  what the five tests in Chapter 28 are for, and why you rerun them after the
  work rather than declaring victory from a spreadsheet.

  Run 'business-case -sources' for the public figures Chapter 26 cites on the
  link between latency and revenue.

`, strings.Repeat("=", 75))
}

func printSources() {
	fmt.Print(`
PUBLIC FIGURES CITED IN CHAPTER 26
===========================================================================

These are the numbers the book actually cites for the link between latency
and business outcomes. Use them as evidence that the relationship is real and
measurable at scale — not as a multiplier to apply to your own revenue.

  Amazon    every 100ms of added page load time cost ~1% in revenue
            Steven van Vessum, Conductor, June 14 2023

  Walmart   ~2% increase in conversions for every second shaved off load time
            Cedric Moore, Trueform, February 10 2025

  Staples   ~10% increase in conversions from a 1s home page improvement
            Martyna Kozlowska, Reffine

Chapter 26 is explicit that "turning this into monetary estimates is harder to
give guidance on." Quoting someone else's conversion lift as though it were
your forecast is how a business case loses a room. Cite these to establish
that latency has measurable financial consequences, then argue your own case
from your own numbers.

`)
}
