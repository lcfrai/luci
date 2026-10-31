package product

import (
	"fmt"
	"unicode/utf8"
)

// This is a display projection, never a change to saved GTM or its digest.
// Reserve independent budgets for sections and decision/assumption text so a
// detailed early audience cannot crowd out the later scenario or support terms.
// Even six-byte JSON escaping of every text byte remains below the existing
// 256 KiB artifact bound with these compiler-owned IDs/headings/source refs.
func boundedCommercialArtifact(value DistributeArtifact, saved GTMWorkspace) DistributeArtifact {
	shortened, omitted, omittedBytes := 0, 0, 0
	shorten := func(text string, limit int) string {
		if limit > 8192 {
			limit = 8192
		}
		if len(text) <= limit {
			return text
		}
		cut := text[:limit]
		for !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		omittedBytes += len(text) - len(cut)
		if cut == "" {
			omitted++
		} else {
			shortened++
		}
		return cut
	}
	sections := append([]DistributeArtifactSection{}, value.Sections...)
	if len(sections) > 40 {
		for _, section := range sections[40:] {
			omitted++
			omittedBytes += len(section.Body)
			for _, bullet := range section.Bullets {
				omittedBytes += len(bullet)
			}
		}
		sections = sections[:40]
	}
	remaining := 24 << 10
	for i := range sections {
		section := &sections[i]
		section.Bullets = append([]string{}, section.Bullets...)
		if len(section.Bullets) > 100 {
			for _, text := range section.Bullets[100:] {
				omitted++
				omittedBytes += len(text)
			}
			section.Bullets = section.Bullets[:100]
		}
		// Fair deterministic allocation keeps every section eligible for display.
		budget := remaining / (len(sections) - i)
		fields := 1 + len(section.Bullets)
		limit := budget / fields
		section.Body = shorten(section.Body, limit)
		used := len(section.Body)
		bullets := []string{}
		for _, text := range section.Bullets {
			display := shorten(text, limit)
			used += len(display)
			if display != "" {
				bullets = append(bullets, display)
			}
		}
		section.Bullets = bullets
		remaining -= used
	}
	value.Sections = sections
	// Keep actual missing decisions ahead of optional repeated assumptions.
	remaining = 8 << 10
	lists := []*[]string{&value.Gaps, &value.Assumptions}
	for _, list := range lists {
		source := append([]string{}, (*list)...)
		// One gap slot is reserved for the mandatory projection notice.
		maximum := 100
		if list == &value.Gaps {
			maximum = 99
		}
		if len(source) > maximum {
			for _, text := range source[maximum:] {
				omitted++
				omittedBytes += len(text)
			}
			source = source[:maximum]
		}
		out := []string{}
		for i, text := range source {
			display := shorten(text, remaining/(len(source)-i))
			remaining -= len(display)
			if display != "" {
				out = append(out, display)
			}
		}
		*list = out
	}
	if shortened > 0 || omitted > 0 {
		value.Gaps = append(value.Gaps, fmt.Sprintf("Bounded document projection: %d text fields shortened; %d display items omitted; %d display-text bytes omitted. Read exact GTM revision %d (SHA256 %s) and the cited product sources for full text. Saved source records were not changed.", shortened, omitted, omittedBytes, saved.Revision, saved.Digest))
	}
	return value
}
