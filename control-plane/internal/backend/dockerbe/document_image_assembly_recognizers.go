package dockerbe

import (
	"reflect"
	"strconv"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Only the exact dedicated label shape projects this managed resource kind.
// Instance ownership and actual limits are separately observed; no authority is
// conferred by this projection, including on another instance's valid labels.
func assemblyResourceIdentity(labels map[string]string) (backend.DocumentImageAssemblyIdentity, bool) {
	protocol, err := strconv.Atoi(labels["luci.assembly-protocol"])
	identity := backend.DocumentImageAssemblyIdentity{Protocol: protocol, Profile: labels["luci.assembly-profile"], EngineID: labels["luci.assembly-engine"], InstanceID: labels[backend.LabelInstance], AllocationID: labels[assemblyAllocationLabel], ScopeDigest: labels["luci.assembly-scope"], ActorDigest: labels["luci.assembly-actor"], ProjectID: labels["luci.assembly-project"], SourceCommit: labels["luci.assembly-source-commit"], SourceDigest: labels["luci.assembly-source"], InputDigest: labels["luci.assembly-input"], HelperDigest: labels["luci.assembly-helper"], RecipeDigest: labels["luci.assembly-recipe"], CommandDigest: labels["luci.assembly-command"]}
	return identity, err == nil && backend.ValidateDocumentImageAssemblyIdentity(identity, true) && reflect.DeepEqual(labels, assemblyLabels(identity))
}
