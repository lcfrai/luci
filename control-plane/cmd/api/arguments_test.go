package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestRunCommandArgumentsNeverStartService(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "help", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "version", args: []string{"--version"}},
		{name: "unknown option", args: []string{"--port=9000"}, code: 2},
		{name: "secret-shaped option", args: []string{"--api-key=fixture-secret-do-not-print"}, code: 2},
		{name: "unknown command", args: []string{"serve"}, code: 2},
		{name: "option value", args: []string{"--help=true"}, code: 2},
		{name: "bare separator", args: []string{"--"}, code: 2},
		{name: "empty argument", args: []string{""}, code: 2},
		{name: "conflicting options", args: []string{"--help", "--version"}, code: 2},
		{name: "repeated option", args: []string{"--help", "--help"}, code: 2},
		{name: "help with extra argument", args: []string{"--help", "project"}, code: 2},
		{name: "unknown before help", args: []string{"--port", "--help"}, code: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runCommand(tt.args, &out, &errOut, func() int {
				t.Fatal("argument-only invocation reached process/service setup")
				return 99
			})
			if code != tt.code {
				t.Fatalf("exit = %d, want %d", code, tt.code)
			}
			if tt.code == 2 {
				if out.Len() != 0 || !strings.Contains(errOut.String(), "luci:") || !strings.Contains(errOut.String(), "Usage: luci") {
					t.Fatalf("rejection must explain usage on stderr only: stdout=%q stderr=%q", out.String(), errOut.String())
				}
				if strings.Contains(out.String()+errOut.String(), "fixture-secret-do-not-print") {
					t.Fatal("rejected argument exposed a mistakenly supplied credential")
				}
				return
			}
			if errOut.Len() != 0 {
				t.Fatalf("informational option wrote stderr: %q", errOut.String())
			}
			if tt.name == "version" {
				if want := "luci " + product.Version + "\n"; out.String() != want {
					t.Fatalf("version = %q, want %q", out.String(), want)
				}
				return
			}
			for _, text := range []string{"Usage: luci", "With no arguments", "LUCI_*", "./start-luci.sh", "Start-Luci.cmd", "--version"} {
				if !strings.Contains(out.String(), text) {
					t.Errorf("help omits %q: %q", text, out.String())
				}
			}
		})
	}
}

func TestRunCommandNoArgumentsPreservesServiceResult(t *testing.T) {
	for _, code := range []int{0, 1, 2, 23} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var out, errOut bytes.Buffer
			calls := 0
			got := runCommand(nil, &out, &errOut, func() int {
				calls++
				return code
			})
			if got != code || calls != 1 {
				t.Fatalf("service result=%d calls=%d, want result=%d calls=1", got, calls, code)
			}
			if out.Len() != 0 || errOut.Len() != 0 {
				t.Fatalf("no-argument startup acquired CLI output: stdout=%q stderr=%q", out.String(), errOut.String())
			}
		})
	}
}
