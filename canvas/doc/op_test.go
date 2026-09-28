package doc

import (
	"encoding/json"
	"testing"
)

func TestOp_jsonNullRemoves(t *testing.T) {
	raw := `{"kind":"set_props","nodeId":"n-0000000000a1","styles":{"color":null,"padding":"8px"}}`
	var op Op
	if err := json.Unmarshal([]byte(raw), &op); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if op.Kind != OpSetProps || op.NodeID != "n-0000000000a1" {
		t.Fatalf("op = %+v, want a set_props on n-0000000000a1", op)
	}
	v, present := op.Styles["color"]
	if !present {
		t.Fatal("colour is absent from Styles: a null must survive as a present key, or a removal is swallowed")
	}
	if v != nil {
		t.Fatalf("Styles[color] = %q, want nil: JSON null is the removal", *v)
	}
	if op.Styles["padding"] == nil || *op.Styles["padding"] != "8px" {
		t.Fatalf("Styles[padding] = %v, want 8px", op.Styles["padding"])
	}
}

func TestOp_jsonRoundTripsOmitEmpty(t *testing.T) {
	op := Op{Kind: OpReviewMark, BriefKey: "brief/D3"}
	b, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"kind":"review_mark","briefKey":"brief/D3"}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s: an op carries only the fields its kind names", b, want)
	}
}

func TestInvertible(t *testing.T) {
	content := []OpKind{
		OpCreatePage, OpUpdatePage, OpDeletePage,
		OpCreateNode, OpSetProps, OpMove, OpDeleteNode,
		OpSetToken, OpDeleteToken,
	}
	for _, k := range content {
		if !Invertible(k) {
			t.Errorf("Invertible(%q) = false, want true: it is design content", k)
		}
	}
	notContent := []OpKind{
		OpAddAsset, OpCreateThread, OpAddMessage, OpSetThreadStatus,
		OpRequestReview, OpCloseReview, OpReviewMark,
	}
	for _, k := range notContent {
		if Invertible(k) {
			t.Errorf("Invertible(%q) = true, want false: it is never undone", k)
		}
	}
	if Invertible("teleport") {
		t.Error("Invertible reports an unknown kind as content")
	}
}

func TestOpError_Error(t *testing.T) {
	e := &OpError{Index: 3, Code: CodeCycle, NodeID: "n-0000000000a1", Message: "a node inside its own subtree"}
	want := "doc: op 3 (n-0000000000a1): cycle: a node inside its own subtree"
	if e.Error() != want {
		t.Fatalf("Error() = %q, want %q", e.Error(), want)
	}
	e2 := &OpError{Index: 0, Code: CodeEmpty, Message: "a batch with no ops"}
	if got, want := e2.Error(), "doc: op 0: empty: a batch with no ops"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
