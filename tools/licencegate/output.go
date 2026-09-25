package main

import (
	"fmt"
	"io"
	"strings"
)

// line is one finding as the gate prints it:
//
//	<VERDICT> <eco> <name> <version> <declared> — <why> (<via>, from <from>)
//
// with "(none)" for no declared licence and "-" for no version.
func line(f Finding) string {
	d := f.Dep
	declared, version := d.Declared, d.Version
	if declared == "" {
		declared = "(none)"
	}
	if version == "" {
		version = "-"
	}
	ctx := "from " + d.From
	if d.Via != "" {
		ctx = d.Via + ", " + ctx
	}
	return fmt.Sprintf("%s %s %s %s %s — %s (%s)", strings.ToUpper(string(f.Verdict)), d.Ecosystem, d.Name, version, declared, f.Why, ctx)
}

// report prints every enforced finding (the table), then each REPORT line
// (a dep that is not enforced and would have failed), the SKIP notes, the
// Cargo line (always: "not scanned" when cargo did not run), and a summary.
// A non-enforced dep that passes is counted, not printed.
func report(w io.Writer, findings []Finding, notes []string, cargoNotScanned string, code int) {
	var pass, excepted, fail, reported, crates, cargoReports int
	for _, f := range findings {
		if f.Dep.Ecosystem == Cargo {
			crates++
		}
		if !f.Dep.Enforced {
			continue
		}
		switch f.Verdict {
		case Pass:
			pass++
		case Excepted:
			excepted++
		case Fail:
			fail++
		}
		fmt.Fprintln(w, line(f))
	}
	for _, f := range findings {
		if f.Verdict == Report {
			reported++
			if f.Dep.Ecosystem == Cargo {
				cargoReports++
			}
			fmt.Fprintln(w, line(f))
		}
	}
	for _, n := range notes {
		fmt.Fprintln(w, n)
	}
	if cargoNotScanned != "" {
		fmt.Fprintln(w, "REPORT cargo: not scanned: "+cargoNotScanned)
	} else {
		fmt.Fprintf(w, "cargo: %d crates scanned, reported only (CargoEnforced %v, OQ-P5-2); %d would fail\n", crates, CargoEnforced, cargoReports)
	}
	fmt.Fprintf(w, "licencegate: %d enforced (%d pass, %d excepted, %d fail), %d reported; exit %d\n",
		pass+excepted+fail, pass, excepted, fail, reported, code)
}
