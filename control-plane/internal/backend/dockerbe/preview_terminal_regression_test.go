package dockerbe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func previewAuxFrame(id string) string {
	data, _ := json.Marshal(map[string]any{"aux": map[string]string{"ID": id}})
	return string(data) + "\n"
}

func previewStreamFrame(value string) string {
	data, _ := json.Marshal(map[string]string{"stream": value})
	return string(data) + "\n"
}

// Classic Docker emits an aux image ID after every stage. Its final success
// line is emitted by the outer build backend only after the entire build ends.
func TestPreviewClassicBuildRequiresFinalCompletionAfterLastStage(t *testing.T) {
	for _, mode := range []string{"first-stage-only-eof", "mismatched-final-marker", "marker-before-aux", "aux-after-marker", "later-mismatched-marker", "marker-with-extra-text", "short-aux", "proper-final-success", "proper-single-stage-success"} {
		t.Run(mode, func(t *testing.T) {
			b, f, spec := newPreviewProbe(t)
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			finalID := previewBuiltImage(f, prepared)
			stageID := "sha256:" + strings.Repeat("e", 64)
			stage := f.images[finalID]
			stage.ID = stageID
			f.images[stageID] = stage
			finalMarker := previewStreamFrame("Successfully built " + strings.TrimPrefix(finalID, "sha256:")[:12] + "\n")
			stageMarker := previewStreamFrame("Successfully built " + strings.TrimPrefix(stageID, "sha256:")[:12] + "\n")
			wantSuccess := false
			switch mode {
			case "first-stage-only-eof":
				f.stream = previewAuxFrame(stageID)
			case "mismatched-final-marker":
				f.stream = previewAuxFrame(finalID) + stageMarker
			case "marker-before-aux":
				f.stream = finalMarker + previewAuxFrame(finalID)
			case "aux-after-marker":
				f.stream = previewAuxFrame(stageID) + stageMarker + previewAuxFrame(finalID)
			case "later-mismatched-marker":
				f.stream = previewAuxFrame(finalID) + finalMarker + stageMarker
			case "marker-with-extra-text":
				f.stream = previewAuxFrame(finalID) + previewStreamFrame("Successfully built "+strings.TrimPrefix(finalID, "sha256:")[:12]+" unexpected\n")
			case "short-aux":
				f.stream = previewAuxFrame(strings.TrimPrefix(finalID, "sha256:")[:12]) + finalMarker
			case "proper-final-success":
				f.stream = previewAuxFrame(stageID) + previewStreamFrame("Step 4/4 : final stage\n") + previewAuxFrame(finalID) + finalMarker
				wantSuccess = true
			case "proper-single-stage-success":
				f.stream = previewAuxFrame(finalID) + finalMarker
				wantSuccess = true
			}
			result, err := b.BuildPreview(context.Background(), prepared, allowPreviewMutation)
			if wantSuccess {
				if err != nil || !result.Settled || result.ImageID != finalID {
					t.Fatalf("final completion refused: result=%+v error=%v", result, err)
				}
			} else if err == nil || result.Settled || result.ImageID != "" {
				t.Fatalf("nonfinal stream released builder capacity: result=%+v error=%v", result, err)
			}
			if len(f.buildOptions) != 1 || len(f.starts) != 0 || len(f.removes) != 0 {
				t.Fatal("completion observation replayed or mutated a serving resource")
			}
		})
	}
}
