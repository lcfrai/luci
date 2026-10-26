package dockerbe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestDockerInitializationDiagnosticPreservesExistingErrorChain(t *testing.T) {
	for _, stage := range []string{"docker_client_configuration_unavailable", "docker_initial_ping_unavailable"} {
		t.Run(stage, func(t *testing.T) {
			cause := fmt.Errorf("private transport detail: %w", backend.ErrUnavailable)
			err := &initializationError{stage: stage, cause: cause}
			wrapped := fmt.Errorf("outer: %w", err)
			var diagnostic interface{ DockerInitializationStage() string }
			if err.Error() != cause.Error() || !errors.Is(wrapped, backend.ErrUnavailable) || !errors.Is(wrapped, cause) || !errors.As(wrapped, &diagnostic) || diagnostic.DockerInitializationStage() != stage {
				t.Fatal("constructor diagnostic changed the existing error chain or stage")
			}
		})
	}
}
