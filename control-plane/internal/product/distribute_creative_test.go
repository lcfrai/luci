package product

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func creativeRenderFixture() DistributeArtifact {
	return DistributeArtifact{Kind: "launch-creative", Creative: &LaunchCreative{Version: 1, Renderer: "luci-launch-svg-v1", Brief: LaunchCreativeBrief{Product: "A product", Headline: "A useful product", Body: "A clear message for customers.", CTA: "Learn more", Alt: "Product launch artwork", Format: "wide", Background: "#ffffff", Foreground: "#18181b", Accent: "#244f44"}, Assets: []DesignAsset{}}}
}

func TestLaunchCreativePreservesLegacyArtifactJSONAndDigest(t *testing.T) {
	const legacy = `{"id":"","projectId":"","revision":0,"kind":"risk","source":{"briefRevision":0,"briefDigest":"","specificationRevision":0,"specificationDigest":"","designRevision":0,"designDigest":"","planRevision":0,"planDigest":"","checks":"","release":""},"sourceDigest":"","title":"","sections":null,"gaps":null,"assumptions":null,"state":"","digest":"","actor":{"id":"","displayName":""},"at":""}`
	var row DistributeArtifact
	if err := json.Unmarshal([]byte(legacy), &row); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(row)
	if err != nil || string(raw) != legacy || row.Creative != nil || artifactDigest(row) != "485c6c058daf92904583549f2c32be671219190b1d1a9c6693c9c5472773609e" {
		t.Fatal("legacy artifact encoding or digest changed", string(raw), err)
	}
}

func TestLaunchCreativeEscapesContentAndRefusesExternalStyles(t *testing.T) {
	row := creativeRenderFixture()
	row.Creative.Brief.Product = "A & B"
	row.Creative.Brief.Headline = "<script> review </script>"
	raw, err := renderLaunchCreative(row, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("A &amp; B")) || bytes.Contains(raw, []byte("<script>")) || bytes.Contains(raw, []byte("textLength")) {
		t.Fatal("active content or stretched text", string(raw))
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "script" {
			t.Fatal("active element")
		}
	}
	row.Creative.Brief.Accent = "url(https://example.com/track)"
	if _, err = renderLaunchCreative(row, nil, false); err == nil {
		t.Fatal("external style accepted")
	}
}

func TestLaunchCreativeWrapsWordsWithoutSplittingClustersAndRefusesOverflow(t *testing.T) {
	input := "Review this product\nCafe\u0301 👩‍💻 ready"
	lines, err := creativeLines(input, 180, 16, "system", 400)
	if err != nil || len(lines) < 2 || strings.Join(strings.Fields(strings.Join(lines, " ")), " ") != strings.Join(strings.Fields(input), " ") {
		t.Fatal("lost or changed word", lines, err)
	}
	found := false
	for _, line := range lines {
		if strings.Contains(line, "👩‍💻") {
			found = true
		}
	}
	if !found {
		t.Fatal("grapheme split")
	}
	if _, err = creativeLines(strings.Repeat("word", 100), 180, 16, "system", 400); err == nil {
		t.Fatal("oversized token accepted")
	}
	row := creativeRenderFixture()
	row.Creative.Brief.Body = strings.Repeat("many words ", 60)
	if _, err = renderLaunchCreative(row, nil, false); err == nil {
		t.Fatal("body overflow silently clipped")
	}
}

func TestLaunchCreativeRetainsExactFrameAndRejectsChangedCapture(t *testing.T) {
	row := creativeRenderFixture()
	frame := DesignSceneFrame{ID: "frame-1", Name: "Product", ScreenID: "screen-1", Width: 640, Height: 360, Background: "#ffffff", Nodes: []DesignSceneNode{{ID: "node-1", Kind: "text", Name: "Title", Width: 500, Height: 100, Text: "Saved product design", Style: DesignSceneStyle{Fill: "transparent", Stroke: "transparent", Color: "#18181b", FontSize: 24, FontWeight: 400, FontFamily: "system", TextAlign: "left", Opacity: 100}, Behavior: DesignSceneBehavior{Kind: "none"}}}}
	row.Creative.Brief.FrameID = frame.ID
	row.Creative.Frame = &frame
	row.Creative.FrameDigest = reviewDigest(frame)
	if !validLaunchCreative(row) {
		t.Fatal("valid captured frame refused")
	}
	if _, err := renderLaunchCreative(row, nil, false); err != nil {
		t.Fatal(err)
	}
	row.Creative.Frame.Nodes[0].Text = "Changed after capture"
	if validLaunchCreative(row) {
		t.Fatal("changed original frame accepted with prior digest")
	}
}
