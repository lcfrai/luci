package deploywatch

import "time"

// budget is one cycle's remaining time. It exists so that "a read begins only
// with a whole attempt left" is one sentence in one place: a read that BEGINS
// can then only fail as Panacea's own failure, never as the cockpit's budget,
// and the two are never reported as each other.
type budget struct {
	deadline time.Time
	now      func() time.Time
}

func (b budget) left() time.Duration { return b.deadline.Sub(b.now()) }

// canRead reports whether a read needing span may begin.
func (b budget) canRead(span time.Duration) bool { return b.left() >= span }
