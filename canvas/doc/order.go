package doc

import "errors"

// OrderAlphabet is the order alphabet, in ASCII order: 0-9, A-Z, a-z. A key is
// a base-62 fraction 0.d1d2…dn with dn ≠ '0', so byte order equals numeric
// order and no two keys are equal in value.
const OrderAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// orderBase is len(OrderAlphabet), and also the digit an empty upper bound
// reads as: an empty b is 1.0, one past the last digit.
const orderBase = 62

// ErrOrder is every rejection of an order key or a pair of them.
var ErrOrder = errors.New("doc: invalid order keys")

// orderDigit is the value of one alphabet byte, or -1.
func orderDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 36
	}
	return -1
}

// ValidOrder reports whether k is a well-formed key: non-empty, alphabet only,
// and not ending in '0' (a trailing zero would be a second spelling of a value
// already spelled without it, so byte order would stop matching numeric order).
func ValidOrder(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		if orderDigit(k[i]) < 0 {
			return false
		}
	}
	return k[len(k)-1] != '0'
}

// Between returns a key strictly between a and b. a == "" is the bottom (0),
// b == "" the top (1). It errs when either key is invalid or a >= b.
//
// Digit by digit from the left, with the missing digits of a read as 0, of an
// empty b as 62, and of a non-empty b as 0:
//   - equal digits are copied and the walk continues;
//   - if b's digit exceeds a's by more than 1, emit floor((da+db)/2) and stop;
//   - otherwise emit a's digit, drop the upper bound (b = ""), and continue
//     with the rest of a.
func Between(a, b string) (string, error) {
	if a != "" && !ValidOrder(a) {
		return "", ErrOrder
	}
	if b != "" && !ValidOrder(b) {
		return "", ErrOrder
	}
	if a != "" && b != "" && a >= b {
		return "", ErrOrder
	}
	out := make([]byte, 0, len(a)+1)
	for i := 0; ; i++ {
		da := 0
		if i < len(a) {
			da = orderDigit(a[i])
		}
		db := orderBase
		if b != "" {
			db = 0
			if i < len(b) {
				db = orderDigit(b[i])
			}
		}
		switch {
		case da == db:
			out = append(out, OrderAlphabet[da])
		case db > da+1:
			return string(append(out, OrderAlphabet[(da+db)/2])), nil
		case db == da+1:
			// No room here. Keep a's digit and let the rest of a, which is
			// now the only bound, decide the tail.
			out = append(out, OrderAlphabet[da])
			b = ""
		default:
			// Unreachable while a < b holds: a validated pair can never show
			// b below a at the first digit that differs. A guard, so that a
			// later change cannot silently emit a key outside the bounds.
			return "", ErrOrder
		}
	}
}

// Spread returns n keys strictly between a and b, ascending: the middle key is
// Between(a, b), and each half is spread recursively.
func Spread(a, b string, n int) ([]string, error) {
	if n < 0 {
		return nil, ErrOrder
	}
	if _, err := Between(a, b); err != nil { // the bounds are checked even for n == 0
		return nil, err
	}
	if n == 0 {
		return []string{}, nil
	}
	return spread(a, b, n)
}

func spread(a, b string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	mid, err := Between(a, b)
	if err != nil {
		return nil, err
	}
	left, err := spread(a, mid, (n-1)/2)
	if err != nil {
		return nil, err
	}
	right, err := spread(mid, b, n-1-(n-1)/2)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	out = append(out, left...)
	out = append(out, mid)
	return append(out, right...), nil
}
