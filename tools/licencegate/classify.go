package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// licenceFiles are the names Classify reads, matched case-insensitively over
// the directory's entries (re-review RR-I9-3), never opened by a fixed name.
var licenceFiles = []string{
	"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "COPYING", "LICENSE-MIT", "LICENSE-APACHE",
}

// A print is one licence's fingerprint over normalised text (lower case,
// every run of non-alphanumerics one space). all must appear, none must not.
type print struct {
	id   string
	all  []string
	none []string
}

const bsdBase = "redistribution and use in source and binary forms with or without modification are permitted provided that the following conditions are met"

// copyleft headings name a licence and its version exactly. They are
// headings, never bare mentions: MPL-2.0 names the GNU licences as its
// "Secondary Licenses", and the GPL names the Affero and Lesser licences.
var copyleft = []print{
	{id: "AGPL-3.0-only", all: []string{"gnu affero general public license version 3 19 november 2007"}},
	{id: "LGPL-3.0-only", all: []string{"gnu lesser general public license version 3 29 june 2007"}},
	{id: "LGPL-2.1-only", all: []string{"gnu lesser general public license version 2 1 february 1999"}},
	{id: "LGPL-2.0-only", all: []string{"gnu library general public license version 2 june 1991"}},
	{id: "GPL-3.0-only", all: []string{"gnu general public license version 3 29 june 2007"}},
	{id: "GPL-2.0-only", all: []string{"gnu general public license version 2 june 1991"}},
	{id: "MPL-2.0", all: []string{"mozilla public license version 2 0"}},
	{id: "MPL-1.1", all: []string{"mozilla public license version 1 1"}},
	{id: "EUPL-1.2", all: []string{"european union public licence v 1 2"}},
	{id: "SSPL-1.0", all: []string{"server side public license version 1"}},
	{id: "BUSL-1.1", all: []string{"business source license 1 1"}},
}

// copyleftMentions fail a file that names a copyleft licence with no heading
// above: a notice such as "under the terms of the GNU General Public License".
// They apply only when no copyleft heading matched, for the reason above.
var copyleftMentions = []print{
	{id: "LicenseRef-GNU-AGPL", all: []string{"gnu affero general public license"}},
	{id: "LicenseRef-GNU-LGPL", all: []string{"gnu lesser general public license"}},
	{id: "LicenseRef-GNU-LGPL", all: []string{"gnu library general public license"}},
	{id: "LicenseRef-GNU-GPL", all: []string{"gnu general public license"}},
	{id: "LicenseRef-MPL", all: []string{"mozilla public license"}},
	{id: "LicenseRef-EUPL", all: []string{"european union public licence"}},
	{id: "LicenseRef-SSPL", all: []string{"server side public license"}},
	{id: "LicenseRef-BUSL", all: []string{"business source license"}},
}

// permissive are OD-11's four families, then the named licences outside them
// that a module might ship; the named ones parse and are never allowed.
var permissive = []print{
	{id: "Apache-2.0", all: []string{"apache license version 2 0 january 2004"}},
	{id: "Apache-2.0", all: []string{"licensed under the apache license version 2 0"}, none: []string{"apache license version 2 0 january 2004"}},
	{id: "MIT", all: []string{
		"permission is hereby granted free of charge to any person obtaining a copy",
		"the above copyright notice and this permission notice shall be included in all copies or substantial portions",
	}},
	{id: "BSD-4-Clause", all: []string{bsdBase, "all advertising materials mentioning features or use of this software"}},
	{id: "BSD-3-Clause", all: []string{bsdBase, "neither the name of"}, none: []string{"all advertising materials mentioning features"}},
	{id: "BSD-3-Clause", all: []string{bsdBase, "may not be used to endorse or promote products derived"},
		none: []string{"neither the name of", "all advertising materials mentioning features"}},
	{id: "BSD-2-Clause", all: []string{bsdBase},
		none: []string{"neither the name of", "may not be used to endorse or promote products derived", "all advertising materials mentioning features"}},
	{id: "ISC", all: []string{"permission to use copy modify and or distribute this software for any purpose with or without fee is hereby granted provided that the above copyright notice and this permission notice appear in all copies"}},
	{id: "ISC", all: []string{"permission to use copy modify and distribute this software for any purpose with or without fee is hereby granted provided that the above copyright notice and this permission notice appear in all copies"}},
	{id: "0BSD", all: []string{"permission to use copy modify and or distribute this software for any purpose with or without fee is hereby granted"},
		none: []string{"hereby granted provided that"}},
	{id: "Zlib", all: []string{"permission is granted to anyone to use this software for any purpose including commercial applications"}},
	{id: "Unlicense", all: []string{"this is free and unencumbered software released into the public domain"}},
	{id: "BSL-1.0", all: []string{"boost software license version 1 0"}},
	{id: "Unicode-3.0", all: []string{"unicode license v3"}},
}

// unclassified marks text the gate recognises as a licence it has no print
// for: a Creative Commons section, the Python, Artistic, Eclipse or CDDL
// licences. The file then carries LicenseRef-unclassified beside what it
// matched, so a docs licence in an Apache LICENSE is never read as Apache.
var unclassified = []string{
	"creative commons", "python software foundation license", "artistic license",
	"eclipse public license", "common development and distribution license",
}

func normalise(b []byte) string {
	var sb strings.Builder
	space := true
	for _, r := range strings.ToLower(string(b)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			space = false
		} else if !space {
			sb.WriteByte(' ')
			space = true
		}
	}
	return " " + sb.String() + " "
}

func (p print) matches(text string) bool {
	for _, s := range p.all {
		if !strings.Contains(text, " "+s+" ") {
			return false
		}
	}
	for _, s := range p.none {
		if strings.Contains(text, " "+s+" ") {
			return false
		}
	}
	return true
}

// classifyText returns the licence ids one file holds, in table order:
// copyleft headings, else copyleft mentions, then permissive prints, then
// LicenseRef-unclassified for a recognised licence with no print, or for a
// file that matched nothing.
func classifyText(b []byte) []string {
	text := normalise(b)
	var ids []string
	add := func(id string) {
		for _, x := range ids {
			if x == id {
				return
			}
		}
		ids = append(ids, id)
	}
	for _, p := range copyleft {
		if p.matches(text) {
			add(p.id)
		}
	}
	if len(ids) == 0 {
		for _, p := range copyleftMentions {
			if p.matches(text) {
				add(p.id)
			}
		}
	}
	for _, p := range permissive {
		if p.matches(text) {
			add(p.id)
		}
	}
	for _, s := range unclassified {
		if strings.Contains(text, " "+s+" ") {
			add("LicenseRef-unclassified")
		}
	}
	if len(ids) == 0 {
		add("LicenseRef-unclassified")
	}
	return ids
}

// Classify reads the licence files of one Go module directory and returns an
// SPDX expression, and the real names of the files it read. LICENSE-MIT with
// LICENSE-APACHE is "MIT OR Apache-2.0" when each holds the licence its name
// says; any other set of files, or one file with two licences, is their AND.
// A directory with none of the files is ("", "none", nil); "" never passes.
func Classify(dir string) (expr string, source string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}
	var names []string
	for _, e := range entries {
		for _, want := range licenceFiles {
			if !e.IsDir() && strings.EqualFold(e.Name(), want) {
				names = append(names, e.Name())
			}
		}
	}
	if len(names) == 0 {
		return "", "none", nil
	}
	sort.Strings(names)
	perFile := map[string][]string{}
	var mitFile, apacheFile string
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return "", "", err
		}
		perFile[n] = classifyText(b)
		switch {
		case strings.EqualFold(n, "LICENSE-MIT"):
			mitFile = n
		case strings.EqualFold(n, "LICENSE-APACHE"):
			apacheFile = n
		}
	}
	var terms []string
	seen := map[string]bool{}
	term := func(t string) {
		if !seen[t] {
			seen[t] = true
			terms = append(terms, t)
		}
	}
	dual := mitFile != "" && apacheFile != "" &&
		strings.Join(perFile[mitFile], " ") == "MIT" && strings.Join(perFile[apacheFile], " ") == "Apache-2.0"
	for _, n := range names {
		if dual && (n == mitFile || n == apacheFile) {
			term("MIT OR Apache-2.0")
			continue
		}
		for _, id := range perFile[n] {
			term(id)
		}
	}
	if len(terms) == 1 {
		return terms[0], strings.Join(names, ", "), nil
	}
	for i, t := range terms {
		if strings.Contains(t, " OR ") {
			terms[i] = "(" + t + ")"
		}
	}
	return strings.Join(terms, " AND "), strings.Join(names, ", "), nil
}
