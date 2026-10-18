package api

import (
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// These are metadata/fake-backend refusal tests, not model or Docker proof.
func TestNativeDelegationDispatchRefusesUnknownImageAndUnsupportedBackend(t *testing.T) {
	for _, name := range []string{"mutable", "changed", "unsupported-backend", "unknown-profile"} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			image := "sha256:" + strings.Repeat("7", 64)
			run := product.ProjectRun{Input: product.ProjectRunInput{Route: product.ProjectRunRoute{Harness: "codex", PermissionMode: "bypass", NativeDelegation: product.NativeDelegationCodexHelpers2, NativeDelegationImage: image}}}
			switch name {
			case "mutable": run.Input.Route.NativeDelegationImage = "luci/sandbox:dev"
			case "changed": image = "sha256:" + strings.Repeat("8", 64)
			case "unsupported-backend": env.be.kind = backend.KindKata
			case "unknown-profile": run.Input.Route.NativeDelegation = "other"
			}
			profile, err := env.api.nativeDelegationDispatchProfile(run, "op-not-allocated", image)
			if err == nil || profile != "" || len(env.be.specs) != 0 || env.api.execution.snapshot.Load() != nil { t.Fatal("unsupported profile acquired execution effects", profile, err) }
		})
	}
}

func TestNativeDelegationDispatchLeavesLegacyTaskPolicyUnchanged(t *testing.T) {
	env := sessionsNewEnv(t)
	profile, err := env.api.nativeDelegationDispatchProfile(product.ProjectRun{}, "", "luci/sandbox:dev")
	if err != nil || profile != "" || len(env.be.specs) != 0 { t.Fatal("legacy task acquired a helper grant", profile, err) }
}
