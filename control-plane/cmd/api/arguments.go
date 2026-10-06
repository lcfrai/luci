package main

import (
	"fmt"
	"io"

	"github.com/lcfrai/luci/control-plane/internal/product"
)

const commandUsage = `Usage: luci [--help | -h | --version]

With no arguments, start the Luci service configured by LUCI_* environment
variables. For the portable app, use ./start-luci.sh (Linux/macOS) or
Start-Luci.cmd (Windows) from the extracted package.

  --help, -h  Show this help without starting the service.
  --version   Show the Luci product version without starting the service.
`

// runCommand decides whether to start before any process, configuration, state
// or backend setup. Keeping every startup side effect inside startService also
// lets argument-only invocations run when the service configuration is invalid.
func runCommand(args []string, out, errOut io.Writer, startService func() int) int {
	if len(args) == 0 {
		return startService()
	}
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			fmt.Fprint(out, commandUsage)
			return 0
		case "--version":
			fmt.Fprintf(out, "luci %s\n", product.Version)
			return 0
		default:
			// An unsupported value may be a mistakenly supplied credential.
			fmt.Fprintln(errOut, "luci: unsupported argument; service configuration uses environment variables.")
			fmt.Fprintln(errOut)
		}
	} else {
		fmt.Fprintln(errOut, "luci: use one informational option at a time; service configuration uses environment variables.")
		fmt.Fprintln(errOut)
	}
	fmt.Fprint(errOut, commandUsage)
	return 2
}
