package api

// A comparable handle to an immutable server-owned typed input-evidence record.
// Syntax is not closure, authentication, permission, or installed-output proof.
// The authoritative full-record resolver/constructor is not implemented here;
// no production request, public route, or native capability is enabled.
type assemblyInputClosureRef struct {
	Schema int    `json:"schema"`
	Digest string `json:"digest"`
}

func validAssemblyInputClosureRef(ref assemblyInputClosureRef) bool {
	if ref.Schema != 1 || len(ref.Digest) != 64 {
		return false
	}
	for _, c := range ref.Digest {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Old unbound rows remain readable and retire/fence-only. They never acquire
// new execution authority merely because a later record has the same inputs.
func retainedAssemblyInputClosureRef(ref assemblyInputClosureRef) bool {
	return ref == (assemblyInputClosureRef{}) || validAssemblyInputClosureRef(ref)
}
