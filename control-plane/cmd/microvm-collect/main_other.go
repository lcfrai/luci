//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Luci microVM guest capture requires a verified Linux template")
	os.Exit(1)
}
