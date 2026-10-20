package api

import (
 "strings"
 "testing"
)

func TestDebianProfileSupportEvidenceRolesKeepClosedBackendGrammar(t *testing.T){
 original:="support:manifests/windows-input-proof-tool-closure-v1.json"
 role:=debianProfileEvidenceRole(original)
 if role==original||len(role)!=72||strings.Trim(role,"abcdefghijklmnopqrstuvwxyz0123456789.")!=""{t.Fatal("support role leaked path into evidence role",role)}
 if role==debianProfileEvidenceRole("support:other/windows-input-proof-tool-closure-v1.json"){t.Fatal("different semantic support roles aliased")}
 if debianProfileEvidenceRole("projection-source")!="projection-source"||debianProfileEvidenceRole(original)!=role{t.Fatal("legacy role or deterministic mapping changed")}
}
