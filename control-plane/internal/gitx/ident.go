package gitx

import "strings"

// Ident is one person's commit identity.
type Ident struct{ Name, Email string }

// ParseIdent reads `git var GIT_AUTHOR_IDENT`'s "Name <email> 1790156728 +1000". The first
// '<' and the first '>' after it delimit the email; git strips angle brackets from both
// parts. An empty name or email is not an identity (ok == false). This is the one parser
// of that line: task 04 points cmd/api's gitidentity.go at it and deletes its copy.
func ParseIdent(line string) (id Ident, ok bool) {
	lt := strings.IndexByte(line, '<')
	if lt < 0 {
		return Ident{}, false
	}
	gt := strings.IndexByte(line[lt+1:], '>')
	if gt < 0 {
		return Ident{}, false
	}
	id = Ident{Name: strings.TrimSpace(line[:lt]), Email: strings.TrimSpace(line[lt+1 : lt+1+gt])}
	if id.Name == "" || id.Email == "" {
		return Ident{}, false
	}
	return id, true
}
