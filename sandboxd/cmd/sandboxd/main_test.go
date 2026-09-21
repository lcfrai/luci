package main

import (
	"reflect"
	"testing"
)

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
