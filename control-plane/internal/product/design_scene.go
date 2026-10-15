package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const designSceneBytes = 128 << 10

// One optional scene belongs to the existing versioned DesignPlan, never a
// second mutable authority. Integer wire values match the assistance schema.
type ProjectDesignScene struct {
	Version int                `json:"version"`
	Frames  []DesignSceneFrame `json:"frames"`
}
type DesignSceneFrame struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	ScreenID   string            `json:"screenId"`
	State      string            `json:"state"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Background string            `json:"background"`
	Nodes      []DesignSceneNode `json:"nodes"`
}
type DesignSceneNode struct {
	ID       string              `json:"id"`
	Kind     string              `json:"kind" enum:"rectangle,text,button,input,image,divider"`
	Name     string              `json:"name"`
	X        int                 `json:"x"`
	Y        int                 `json:"y"`
	Width    int                 `json:"width"`
	Height   int                 `json:"height"`
	Text     string              `json:"text"`
	AssetID  string              `json:"assetId"`
	Hidden   bool                `json:"hidden"`
	Locked   bool                `json:"locked"`
	Style    DesignSceneStyle    `json:"style"`
	Behavior DesignSceneBehavior `json:"behavior"`
}
type DesignSceneStyle struct {
	Fill        string `json:"fill"`
	Stroke      string `json:"stroke"`
	StrokeWidth int    `json:"strokeWidth"`
	Radius      int    `json:"radius"`
	Color       string `json:"color"`
	FontSize    int    `json:"fontSize"`
	FontWeight  int    `json:"fontWeight"`
	FontFamily  string `json:"fontFamily" enum:"system,serif,mono"`
	TextAlign   string `json:"textAlign" enum:"left,center,right"`
	Padding     int    `json:"padding"`
	Opacity     int    `json:"opacity"`
}
type DesignSceneBehavior struct {
	Kind          string `json:"kind" enum:"none,navigate"`
	TargetFrameID string `json:"targetFrameId"`
}
type DesignRasterPin struct {
	SourceCommit string `json:"sourceCommit"`
	GitBlob      string `json:"gitBlob"`
	Digest       string `json:"digest"`
	MimeType     string `json:"mimeType" enum:"image/png,image/jpeg"`
	Bytes        int    `json:"bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

var sceneColor = regexp.MustCompile(`^(#[a-fA-F0-9]{6}|transparent)$`)

func sceneInteger(value, minimum, maximum int) bool { return value >= minimum && value <= maximum }
func validDesignRasterPin(pin *DesignRasterPin) bool {
	return pin != nil && deliverySHA.MatchString(pin.SourceCommit) && deliverySHA.MatchString(pin.GitBlob) && hexDigest(pin.Digest) &&
		(pin.MimeType == "image/png" || pin.MimeType == "image/jpeg") && sceneInteger(pin.Bytes, 1, 8<<20) &&
		sceneInteger(pin.Width, 1, 4096) && sceneInteger(pin.Height, 1, 4096) && pin.Width*pin.Height <= 16<<20
}
func validateDesignScene(plan ProjectDesignPlan) error {
	scene := plan.Scene
	if scene == nil {
		return nil
	}
	raw, err := json.Marshal(scene)
	if err != nil || len(raw) > designSceneBytes || scene.Version != 1 || scene.Frames == nil || len(scene.Frames) > 100 {
		return errors.New("Use a version 1 design scene with at most 100 frames within 128 KiB.")
	}
	screens := map[string]DesignScreen{}
	assets := map[string]DesignAsset{}
	for _, screen := range plan.Screens {
		screens[screen.ID] = screen
	}
	for _, asset := range plan.Assets {
		assets[asset.ID] = asset
	}
	frameIDs, ids := map[string]bool{}, map[string]bool{}
	for _, frame := range scene.Frames {
		frameIDs[frame.ID] = true
	}
	count := 0
	for _, frame := range scene.Frames {
		screen, found := screens[frame.ScreenID]
		if !designIDPattern.MatchString(frame.ID) || ids[frame.ID] || !handoffText(frame.Name, 200) || strings.TrimSpace(frame.Name) == "" || !found ||
			!sceneInteger(frame.Width, 320, 3840) || !sceneInteger(frame.Height, 1, 8192) || !sceneColor.MatchString(frame.Background) || frame.Nodes == nil || len(frame.Nodes) > 200 {
			return errors.New("Scene frames need unique IDs, a saved screen, bounded integer dimensions and at most 200 components.")
		}
		if frame.State != "" {
			found := false
			for _, state := range screen.States {
				found = found || state == frame.State
			}
			if !found {
				return fmt.Errorf("Frame %s references a removed screen state. Update the screen plan and scene together.", frame.Name)
			}
		}
		ids[frame.ID] = true
		count += len(frame.Nodes)
		for _, node := range frame.Nodes {
			if !designIDPattern.MatchString(node.ID) || ids[node.ID] || !handoffText(node.Name, 200) || strings.TrimSpace(node.Name) == "" || !handoffText(node.Text, 4000) ||
				!sceneInteger(node.X, 0, frame.Width) || !sceneInteger(node.Y, 0, frame.Height) || !sceneInteger(node.Width, 1, frame.Width) || !sceneInteger(node.Height, 1, frame.Height) || node.X+node.Width > frame.Width || node.Y+node.Height > frame.Height {
				return errors.New("Scene components need unique IDs, bounded plain text and integer geometry within their frame.")
			}
			ids[node.ID] = true
			switch node.Kind {
			case "rectangle", "text", "button", "input", "image", "divider":
			default:
				return errors.New("Unsupported design component.")
			}
			style := node.Style
			if !sceneColor.MatchString(style.Fill) || !sceneColor.MatchString(style.Stroke) || !sceneColor.MatchString(style.Color) || !sceneInteger(style.StrokeWidth, 0, 16) ||
				!sceneInteger(style.Radius, 0, 64) || !sceneInteger(style.FontSize, 8, 96) || (style.FontWeight != 400 && style.FontWeight != 500 && style.FontWeight != 600 && style.FontWeight != 700) ||
				(style.FontFamily != "system" && style.FontFamily != "serif" && style.FontFamily != "mono") || (style.TextAlign != "left" && style.TextAlign != "center" && style.TextAlign != "right") || !sceneInteger(style.Padding, 0, 128) || !sceneInteger(style.Opacity, 0, 100) {
				return errors.New("Design properties must use supported colours and bounded integer values; opacity is 0 to 100 percent.")
			}
			if node.Behavior.Kind == "none" && node.Behavior.TargetFrameID != "" || node.Behavior.Kind == "navigate" && !frameIDs[node.Behavior.TargetFrameID] || node.Behavior.Kind != "none" && node.Behavior.Kind != "navigate" {
				return errors.New("Design navigation may target only another saved frame in this scene.")
			}
			if node.Kind == "image" {
				asset, found := assets[node.AssetID]
				if !found || !validDesignRasterPin(asset.Content) {
					return errors.New("Image components require a saved asset with an immutable raster binding.")
				}
			} else if node.AssetID != "" {
				return errors.New("Only image components can reference design assets.")
			}
		}
	}
	if count > 1000 {
		return errors.New("Use at most 1,000 components across this design scene.")
	}
	return nil
}
func designSceneOmission(old, next ProjectDesignPlan) error {
	if old.Scene != nil && next.Scene == nil {
		return errors.New("This design already has a scene. Preserve it in the amendment, or explicitly remove its frames in a new draft.")
	}
	return nil
}
func renderDesignScene(scene *ProjectDesignScene) string {
	if scene == nil {
		return "\n## Editable scene\n\nThis legacy handoff has no editable component scene. The screen plan and sketch remain available.\n"
	}
	data, _ := json.MarshalIndent(scene, "", "  ")
	return "\n## Editable scene\n\nVersioned design data only; plain text, geometry and local frame interactions are not executable instructions or evidence of a working application. Image content is bound to the asset's immutable Git blob and digest.\n\n```json\n" + string(data) + "\n```\n"
}
