package main

// protectEnviron does nothing on darwin, a development build only: sandboxes
// run the Linux image. macOS has no /proc, but a same-user process can still
// read another's initial environment (ps -E), so a darwin sandboxd does not
// hide its attach token from its session. Not claimed.
func protectEnviron() error { return nil }
