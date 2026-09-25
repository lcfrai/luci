// Package provenance owns the key grammar (contract P5): nothing else in the
// product may split a key. It also owns Ref, the one type Develop, Merge,
// Design and Deploy render (contract §1.4).
//
// The package is pure: no I/O, no clock, no Hub. discoverhub (task 07) hands
// it resolved revisions; the refs route (task 12) serves what it builds.
package provenance

import (
	"errors"
	"regexp"
	"strconv"
)

type Kind string

const (
	KindBrief  Kind = "brief"
	KindDesign Kind = "design"
	KindNone   Kind = "none"
)

// The grammar. brief and design are the Hub's, verbatim (Hub contract §3.2,
// hub-contract.md:120-121 at hub da8c7da); none is local to the control plane
// and never reaches the Hub (P4-C).
//
//	brief   ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$
//	design  ^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$
//	none    ^none:(declined)$
//
// Go's $ without the m flag matches only at the end of the text, so a key
// with a trailing line feed is not a key.
type Key struct {
	Kind       Kind
	ID         string // bf-… or the canvas id; "" for none
	Revision   int    // >= 1; 0 for none
	NoneReason string // "declined"; "" otherwise
}

// Declined is the contextRef of a session the person chose to start without
// its brief (4-D1). The empty string keeps its phase 2 meaning: no brief was
// materialised because none existed.
const Declined = "none:declined"

var ErrInvalidKey = errors.New("provenance: not a key")

// The three forms, compiled once. They are the grammar's only copy.
var (
	briefKeyRe  = regexp.MustCompile(`^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$`)
	designKeyRe = regexp.MustCompile(`^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$`)
	noneKeyRe   = regexp.MustCompile(`^none:(declined)$`)
	briefIDRe   = regexp.MustCompile(`^bf-[0-9a-f]{16}$`)
)

// Parse reads a key. Anything outside the grammar, "" included, is
// ErrInvalidKey beside the zero Key: "" is not a key, it is NoneRef's.
func Parse(s string) (Key, error) {
	if m := briefKeyRe.FindStringSubmatch(s); m != nil {
		return Key{Kind: KindBrief, ID: m[1], Revision: revision(m[2])}, nil
	}
	if m := designKeyRe.FindStringSubmatch(s); m != nil {
		return Key{Kind: KindDesign, ID: m[1], Revision: revision(m[2])}, nil
	}
	if m := noneKeyRe.FindStringSubmatch(s); m != nil {
		return Key{Kind: KindNone, NoneReason: m[1]}, nil
	}
	return Key{}, ErrInvalidKey
}

// revision converts a revision the grammar has already matched: one to nine
// decimal digits, no leading zero, so it always fits an int.
func revision(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil {
		panic("provenance: the grammar matched a revision Atoi refuses: " + digits)
	}
	return n
}

// String writes k in the grammar. A Key the grammar would not parse back to
// itself - a hand-built one with a bad id, a revision below 1, an unknown
// kind or reason - writes "", so String never mints a string that looks like
// a key and is not one.
func (k Key) String() string {
	var s string
	switch k.Kind {
	case KindBrief, KindDesign:
		s = string(k.Kind) + ":" + k.ID + "@" + strconv.Itoa(k.Revision)
	case KindNone:
		s = string(KindNone) + ":" + k.NoneReason
	default:
		return ""
	}
	if back, err := Parse(s); err != nil || back != k {
		return ""
	}
	return s
}

// BriefKey writes a brief key. It panics on a bad id or a revision outside
// 1..999999999: a programming error, not input (the Hub mints both).
func BriefKey(briefID string, revision int) string {
	if !briefIDRe.MatchString(briefID) || revision < 1 || revision > 999999999 {
		panic("provenance: BriefKey(" + strconv.Quote(briefID) + ", " + strconv.Itoa(revision) + ") is not a brief key")
	}
	return "brief:" + briefID + "@" + strconv.Itoa(revision)
}
