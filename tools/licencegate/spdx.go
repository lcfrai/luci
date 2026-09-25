package main

import (
	"fmt"
	"strings"
)

// Expr is a parsed SPDX license expression: an id, "X WITH E", or a
// parenthesised AND / OR of expressions. AND binds tighter than OR.
type Expr interface{ allowed() bool }

// never is every form that names no licence the gate can accept: UNKNOWN,
// UNLICENSED, "SEE LICENSE IN …", "Custom: …", a guess marked "*", "".
type never struct{ why string }

func (never) allowed() bool { return false }

// id is one licence identifier, matched case-insensitively (SPDX annex D).
type id struct{ name string }

func (i id) allowed() bool { return inSet(Allowed, i.name) }

// with is "X WITH E": allowed only when X is and E is an allowed exception.
type with struct {
	lic id
	exc string
}

func (w with) allowed() bool { return w.lic.allowed() && inSet(AllowedExceptions, w.exc) }

type and struct{ l, r Expr }

func (a and) allowed() bool { return a.l.allowed() && a.r.allowed() }

type or struct{ l, r Expr }

func (o or) allowed() bool { return o.l.allowed() || o.r.allowed() }

func inSet(set map[string]bool, name string) bool {
	for k, v := range set {
		if v && strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// ParseSPDX parses the forms npm and the classifier produce. "UNKNOWN",
// "UNLICENSED", "SEE LICENSE IN …", "Custom: …", "LicenseRef-…", a trailing
// "*" (a guess) and "" all parse to an Expr that is never allowed.
func ParseSPDX(s string) (Expr, error) {
	t := strings.TrimSpace(s)
	up := strings.ToUpper(t)
	switch {
	case t == "":
		return never{"no licence declared"}, nil
	case strings.HasPrefix(up, "SEE LICENSE IN"):
		return never{"a licence file, not an SPDX expression"}, nil
	case strings.HasPrefix(up, "CUSTOM:"):
		return never{"a custom licence"}, nil
	}
	p := &parser{toks: tokenize(t)}
	e, err := p.expr()
	if err != nil {
		return nil, fmt.Errorf("SPDX %q: %w", s, err)
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("SPDX %q: unexpected %q", s, p.toks[p.pos])
	}
	return e, nil
}

func tokenize(s string) []string {
	s = strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)
	return strings.Fields(s)
}

type parser struct {
	toks []string
	pos  int
}

func (p *parser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func isOp(tok, op string) bool { return strings.EqualFold(tok, op) }

func (p *parser) expr() (Expr, error) {
	l, err := p.andExpr()
	if err != nil {
		return nil, err
	}
	for isOp(p.peek(), "OR") {
		p.pos++
		r, err := p.andExpr()
		if err != nil {
			return nil, err
		}
		l = or{l, r}
	}
	return l, nil
}

func (p *parser) andExpr() (Expr, error) {
	l, err := p.withExpr()
	if err != nil {
		return nil, err
	}
	for isOp(p.peek(), "AND") {
		p.pos++
		r, err := p.withExpr()
		if err != nil {
			return nil, err
		}
		l = and{l, r}
	}
	return l, nil
}

func (p *parser) withExpr() (Expr, error) {
	a, err := p.atom()
	if err != nil {
		return nil, err
	}
	if !isOp(p.peek(), "WITH") {
		return a, nil
	}
	p.pos++
	lic, ok := a.(id)
	if !ok {
		// "(A OR B) WITH E" and "X* WITH E" are not forms the gate accepts.
		if _, isNever := a.(never); !isNever {
			return nil, fmt.Errorf("WITH must follow a licence id")
		}
	}
	exc := p.peek()
	if exc == "" || exc == "(" || exc == ")" || isKeyword(exc) {
		return nil, fmt.Errorf("WITH needs an exception id")
	}
	p.pos++
	if !ok {
		return never{"a guessed licence with an exception"}, nil
	}
	return with{lic, exc}, nil
}

func isKeyword(tok string) bool {
	return isOp(tok, "AND") || isOp(tok, "OR") || isOp(tok, "WITH")
}

func (p *parser) atom() (Expr, error) {
	tok := p.peek()
	switch {
	case tok == "":
		return nil, fmt.Errorf("unexpected end")
	case tok == ")" || isKeyword(tok):
		return nil, fmt.Errorf("unexpected %q", tok)
	case tok == "(":
		p.pos++
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing )")
		}
		p.pos++
		return e, nil
	}
	p.pos++
	if strings.HasSuffix(tok, "*") {
		// npm's license-checker marks a guess with "*": never accepted.
		return never{"a guessed licence"}, nil
	}
	return id{tok}, nil
}
