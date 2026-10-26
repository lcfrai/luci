//go:build !windows

package managedmodel

import (
	"context"
	"errors"
)

func lock(context.Context, string, bool) (func(), error) {
	return nil, errors.New("managed models require an owned Windows native service")
}
