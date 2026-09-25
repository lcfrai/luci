package main

import (
	"flag"
	"reflect"
	"testing"
)

// The image's entrypoint passes no -mcp-listen, so the default is what every
// sandbox runs: the address the harness is given (task 19), on loopback.
func TestMCPListenFlag(t *testing.T) {
	fs := flag.NewFlagSet("sandboxd", flag.ContinueOnError)
	got := mcpListenFlag(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *got != "127.0.0.1:8731" {
		t.Errorf("-mcp-listen defaults to %q, want 127.0.0.1:8731", *got)
	}
	fs = flag.NewFlagSet("sandboxd", flag.ContinueOnError)
	got = mcpListenFlag(fs)
	if err := fs.Parse([]string{"-mcp-listen", "off"}); err != nil || *got != "off" {
		t.Errorf("-mcp-listen off parsed as %q, %v", *got, err)
	}
}

func TestParseArgsPrefersJSON(t *testing.T) {
	got, err := parseArgs("ignored,csv", `["--model","opus","add tests, then refactor"]`)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := []string{"--model", "opus", "add tests, then refactor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseArgsFallsBackToCSV(t *testing.T) {
	got, err := parseArgs("--help,--version", "")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := []string{"--help", "--version"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseArgsEmptyMeansNoArgs(t *testing.T) {
	got, err := parseArgs("", "")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want no arguments", got)
	}
}

func TestParseArgsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseArgs("", "not json"); err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestParseArgsRejectsNonArrayJSON(t *testing.T) {
	if _, err := parseArgs("", `{"a":1}`); err == nil {
		t.Fatal("expected an error for a JSON object, got nil")
	}
}

// json.Unmarshal accepts null into a []string and leaves it nil, so without
// an explicit check a mis-serialised argv would start an agent with no
// arguments and no complaint.
func TestParseArgsRejectsJSONNull(t *testing.T) {
	if _, err := parseArgs("", "null"); err == nil {
		t.Fatal("expected an error for a JSON null, got nil")
	}
}

// The empty array is how a caller says "no arguments" and must still parse.
func TestParseArgsAcceptsEmptyJSONArray(t *testing.T) {
	got, err := parseArgs("ignored,csv", "[]")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want no arguments", got)
	}
}
