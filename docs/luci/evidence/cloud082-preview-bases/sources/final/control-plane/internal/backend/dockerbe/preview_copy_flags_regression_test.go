package dockerbe

import (
	"context"
	"strings"
	"testing"
)

func TestPreviewCopyQuotedFlagsRefusedBeforeInspection(t *testing.T) {
	for _, tc := range []struct{ name, instruction string }{
		{"double-quoted-space", `COPY --chown="0:0 " --from=alpine:3.22 /etc/os-release /out`},
		{"single-quoted-space", `COPY --chown='0:0 ' --from=alpine:3.22 /etc/os-release /out`},
		{"quoted-tab", "COPY --chown=\"0:0\t\" --from=alpine:3.22 /etc/os-release /out"},
		{"escaped-space", `COPY --chown=0:0\ --from=alpine:3.22 /etc/os-release /out`},
		{"embedded-quote", `COPY --chown=0:"0 " --from=alpine:3.22 /etc/os-release /out`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM node:fixture\n"+tc.instruction+"\n")
			_, err := b.PreparePreview(context.Background(), spec)
			if err == nil {
				t.Error("ambiguous COPY flags hid an external image from preparation")
			}
			assertPreviewBaseNoMutation(t, f)
			for _, event := range f.events {
				if strings.HasPrefix(event, "inspect:") {
					t.Errorf("ambiguous COPY flags reached base inspection: %v", f.events)
					break
				}
			}
		})
	}
}

func TestPreviewCopyQuotedStageFlagsRemainSupported(t *testing.T) {
	for _, instruction := range []string{
		`COPY --chown="0:0" --from="compile" /artifact /artifact`,
		`COPY --chown='root:root' --from='0' /artifact /artifact`,
		`COPY --chown="" --from=compile /artifact /artifact`,
		`COPY --from=compile ["/artifact with spaces", "/artifact"]`,
	} {
		t.Run(instruction, func(t *testing.T) {
			b, f, spec := newPreviewBaseProbe(t, "FROM node:fixture AS compile\nFROM scratch\n"+instruction+"\n")
			prepared, err := b.PreparePreview(context.Background(), spec)
			if err != nil {
				t.Fatalf("ordinary prior-stage COPY flags were refused: %v", err)
			}
			if len(prepared.BasePlan) != 1 || prepared.BasePlan[0].ImageID != f.image.ID {
				t.Fatalf("prior-stage COPY changed the declared base plan: %+v", prepared.BasePlan)
			}
			if got := tarDockerfile(t, prepared.PreparedContext); !strings.Contains(got, instruction) {
				t.Fatalf("COPY instruction changed during preparation: %q", got)
			}
			assertPreviewBaseNoMutation(t, f)
		})
	}
}
