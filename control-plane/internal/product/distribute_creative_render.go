package product

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"math"
	"strings"
	"unicode"
)

func creativeEscape(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

// Conservative advance estimates leave room for natural system-font spacing.
// No textLength/letter stretching is applied; fonts remain generic local faces.
func creativeWidth(value string, size int, family string, weight int) float64 {
	width := 0.0
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		unit := 0.75
		if family == "mono" {
			unit = 0.75
		} else if strings.ContainsRune("ilI.,:;!'`|", r) {
			unit = 0.40
		} else if r == ' ' {
			unit = 0.42
		} else if strings.ContainsRune("MW@%#&", r) {
			unit = 1.15
		} else if r > 0x2ff {
			unit = 1.25
		} else if unicode.IsUpper(r) {
			unit = 0.90
		}
		if family == "serif" {
			unit *= 1.10
		}
		width += unit * float64(size)
	}
	if weight >= 600 {
		width *= 1.08
	}
	return width
}

// Keep each whitespace-delimited token intact, including combining/emoji
// sequences. Native SVG shapes it. A token too large for its slot is refused,
// never split through a grapheme or silently omitted.
func creativeLines(value string, width, size int, family string, weight int) ([]string, error) {
	if width < 1 {
		return nil, errCreative
	}
	lines := []string{}
	for _, paragraph := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if creativeWidth(word, size, family, weight) > float64(width) {
				return nil, errCreative
			}
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if creativeWidth(candidate, size, family, weight) <= float64(width) {
				line = candidate
				continue
			}
			lines = append(lines, line)
			line = word
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func creativeTextSVG(out *bytes.Buffer, value string, x, y, width, height, size int, color, family, align string, weight int) error {
	if value == "" {
		return nil
	}
	if width < 1 || height < 1 || !validArtifactXMLText(value) {
		return errCreative
	}
	lines, err := creativeLines(value, width, size, family, weight)
	if err != nil {
		return err
	}
	step := int(math.Ceil(float64(size) * 1.35))
	if len(lines) == 0 || len(lines)*step > height {
		return errCreative
	}
	font := map[string]string{"system": "sans-serif", "serif": "serif", "mono": "monospace"}[family]
	if font == "" {
		return errCreative
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		anchor := "start"
		tx := x
		if align == "center" {
			anchor = "middle"
			tx += width / 2
		} else if align == "right" {
			anchor = "end"
			tx += width
		}
		fmt.Fprintf(out, `<text x="%d" y="%d" font-family="%s" font-size="%d" font-weight="%d" fill="%s" text-anchor="%s" xml:space="preserve">%s</text>`, tx, y+size+i*step, font, size, weight, color, anchor, creativeEscape(line))
	}
	return nil
}

func creativeImageSVG(out *bytes.Buffer, asset DesignAsset, images map[string][]byte, x, y, width, height int, layoutOnly bool) error {
	if layoutOnly {
		return nil
	}
	raw, ok := images[asset.ID]
	if !ok || len(raw) != asset.Content.Bytes {
		return errCreative
	}
	if out.Len()+base64.StdEncoding.EncodedLen(len(raw))+512 > creativeSVGBytes {
		return errCreative
	}
	fmt.Fprintf(out, `<image x="%d" y="%d" width="%d" height="%d" preserveAspectRatio="xMidYMid meet" href="data:%s;base64,%s"/>`, x, y, width, height, asset.Content.MimeType, base64.StdEncoding.EncodeToString(raw))
	return nil
}

func renderLaunchCreative(row DistributeArtifact, images map[string][]byte, layoutOnly bool) ([]byte, error) {
	if !validLaunchCreative(row) {
		return nil, errCreative
	}
	v := row.Creative
	b := v.Brief
	width, height := 1200, 675
	if b.Format == "square" {
		width, height = 1080, 1080
	}
	visual := v.Frame != nil || b.AssetID != ""
	textWidth := width - 112
	if visual && b.Format == "wide" {
		textWidth = 450
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img"><title>%s</title><desc>%s</desc><rect width="%d" height="%d" fill="%s"/>`, width, height, width, height, creativeEscape(b.Product), creativeEscape(b.Alt), width, height, b.Background)
	text := func(s string, x, y, w, h, size int, color string, weight int) error {
		return creativeTextSVG(&out, s, x, y, w, h, size, color, "system", "left", weight)
	}
	if err := text(b.Product, 56, 42, textWidth, 46, 23, b.Foreground, 600); err != nil {
		return nil, err
	}
	headlineHeight := 180
	bodyY, bodyHeight, ctaY := 300, 170, 545
	if b.Format == "square" && visual {
		headlineHeight = 125
		bodyY, bodyHeight, ctaY = 245, 110, 385
	}
	if b.Format == "square" && !visual {
		headlineHeight = 320
		bodyY, bodyHeight, ctaY = 490, 300, 910
	}
	if err := text(b.Headline, 56, 108, textWidth, headlineHeight, 44, b.Foreground, 700); err != nil {
		return nil, err
	}
	if err := text(b.Body, 56, bodyY, textWidth, bodyHeight, 21, b.Foreground, 400); err != nil {
		return nil, err
	}
	if b.CTA != "" {
		fmt.Fprintf(&out, `<rect x="56" y="%d" width="%d" height="62" rx="10" fill="%s"/>`, ctaY, textWidth, b.Accent)
		if err := text(b.CTA, 74, ctaY+13, textWidth-36, 38, 22, b.Background, 600); err != nil {
			return nil, err
		}
	}
	if visual {
		x, y, w, h := 570, 100, 574, 475
		if b.Format == "square" {
			x, y, w, h = 56, 500, 968, 500
		}
		if v.Frame != nil {
			frame := v.Frame
			scale := math.Min(float64(w)/float64(frame.Width), float64(h)/float64(frame.Height))
			// The complete frame is fitted, never cropped to resemble a screenshot.
			tx := float64(x) + (float64(w)-float64(frame.Width)*scale)/2
			ty := float64(y) + (float64(h)-float64(frame.Height)*scale)/2
			fmt.Fprintf(&out, `<g transform="translate(%.4f %.4f) scale(%.8f)"><rect width="%d" height="%d" fill="%s"/>`, tx, ty, scale, frame.Width, frame.Height, frame.Background)
			for _, node := range frame.Nodes {
				if node.Hidden {
					continue
				}
				s := node.Style
				fmt.Fprintf(&out, `<g opacity="%.2f"><rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s" stroke="%s" stroke-width="%d"/>`, float64(s.Opacity)/100, node.X, node.Y, node.Width, node.Height, s.Radius, s.Fill, s.Stroke, s.StrokeWidth)
				if node.Kind == "image" {
					found := false
					for _, a := range v.Assets {
						if a.ID == node.AssetID {
							found = true
							if err := creativeImageSVG(&out, a, images, node.X, node.Y, node.Width, node.Height, layoutOnly); err != nil {
								return nil, err
							}
						}
					}
					if !found {
						return nil, errCreative
					}
				} else if err := creativeTextSVG(&out, node.Text, node.X+s.Padding, node.Y+s.Padding, node.Width-2*s.Padding, node.Height-2*s.Padding, s.FontSize, s.Color, s.FontFamily, s.TextAlign, s.FontWeight); err != nil {
					return nil, err
				}
				out.WriteString(`</g>`)
			}
			out.WriteString(`</g>`)
		} else if err := creativeImageSVG(&out, v.Assets[0], images, x, y, w, h, layoutOnly); err != nil {
			return nil, err
		}
	}
	out.WriteString(`</svg>`)
	if out.Len() > creativeSVGBytes {
		return nil, errCreative
	}
	return out.Bytes(), nil
}
