package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func componentLibraryFixture() ProjectDesignPlan {
	plan := designSceneFixture()
	node := plan.Scene.Frames[0].Nodes[0]
	plan.ComponentLibrary = &DesignComponentLibrary{Version: 1, Items: []DesignComponentTemplate{{ID: "component-title", Name: "Page title", Description: "Top-level screen heading", Kind: node.Kind, Width: node.Width, Height: node.Height, Text: node.Text, Style: node.Style, Bindings: []DesignComponentBinding{{Property: "color", Token: "ink"}}}}}
	plan.Tokens = []DesignToken{{Name: "ink", Value: "#123456", Purpose: "Text"}}
	plan.Scene.Frames[0].Nodes[0].ComponentID = "component-title"
	return plan
}

func TestComponentLibrarySavedContractAndHandoff(t *testing.T) {
	plan := componentLibraryFixture()
	if err := validateDesignPlan(plan); err != nil {
		t.Fatal(err)
	}
	style, err := resolveDesignComponentStyle(plan.ComponentLibrary.Items[0], plan.Tokens)
	if err != nil || style.Color != "#123456" {
		t.Fatalf("resolved style: %+v %v", style, err)
	}
	if !strings.Contains(string(renderDesignPlan(plan)), "Reusable components") || !strings.Contains(string(renderDesignPlan(plan)), "component-title") {
		t.Fatal("component library missing from agent handoff")
	}
	raw, _ := json.Marshal(plan)
	var recovered ProjectDesignPlan
	if err := json.Unmarshal(raw, &recovered); err != nil || validateDesignPlan(recovered) != nil {
		t.Fatal("saved library did not survive the document roundtrip", err)
	}
}

func TestComponentLibraryRefusesMissingBindingsUnsafeStylesAndDanglingInstances(t *testing.T) {
	cases := map[string]func(*ProjectDesignPlan){
		"missing token":  func(p *ProjectDesignPlan) { p.Tokens = nil },
		"external style": func(p *ProjectDesignPlan) { p.Tokens[0].Value = "url(https://invalid.test)" },
		"duplicate binding": func(p *ProjectDesignPlan) {
			p.ComponentLibrary.Items[0].Bindings = append(p.ComponentLibrary.Items[0].Bindings, p.ComponentLibrary.Items[0].Bindings[0])
		},
		"dangling instance":   func(p *ProjectDesignPlan) { p.ComponentLibrary.Items = []DesignComponentTemplate{} },
		"wrong kind":          func(p *ProjectDesignPlan) { p.ComponentLibrary.Items[0].Kind = "button" },
		"asset template":      func(p *ProjectDesignPlan) { p.ComponentLibrary.Items[0].Kind = "image" },
		"unsupported version": func(p *ProjectDesignPlan) { p.ComponentLibrary.Version = 2 },
		"unbounded size token": func(p *ProjectDesignPlan) {
			p.ComponentLibrary.Items[0].Bindings = []DesignComponentBinding{{Property: "fontSize", Token: "ink"}}
			p.Tokens[0].Value = "10000"
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			plan := componentLibraryFixture()
			change(&plan)
			if validateDesignPlan(plan) == nil {
				t.Fatal("invalid library accepted")
			}
		})
	}
}

func TestComponentLibraryOmissionDoesNotEraseSavedComponentsAndLegacyBytesStayOptional(t *testing.T) {
	old := componentLibraryFixture()
	next := old
	next.ComponentLibrary = nil
	if designSceneOmission(old, next) == nil {
		t.Fatal("omitted library accepted")
	}
	next = componentLibraryFixture()
	next.ComponentLibrary.Items = []DesignComponentTemplate{}
	next.Scene.Frames[0].Nodes[0].ComponentID = ""
	if err := validateDesignPlan(next); err != nil {
		t.Fatal("explicit removal and detached copies rejected", err)
	}
	legacy := designSceneFixture()
	raw, _ := json.Marshal(legacy)
	if strings.Contains(string(raw), "componentLibrary") || strings.Contains(string(raw), "componentId") {
		t.Fatal("legacy document encoding changed")
	}
}
