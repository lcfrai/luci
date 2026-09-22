package harness

import (
	"errors"
	"testing"
)

func TestValidateModelEnforcesAClosedList(t *testing.T) {
	d := Descriptor{
		ID:            Codex,
		Models:        []Model{{ID: "o4", Name: "o4"}},
		freeTextModel: false,
	}
	if err := validateModel(d, "o4"); err != nil {
		t.Fatalf("validateModel(listed model) = %v, want nil", err)
	}
	err := validateModel(d, "gpt-99")
	if !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("validateModel(unlisted model) = %v, want an error wrapping ErrUnknownModel", err)
	}
}

func TestValidateModelAcceptsAnyStringWhenTheHarnessTakesFreeText(t *testing.T) {
	d, ok := Lookup(ClaudeCode)
	if !ok {
		t.Fatal("Lookup(ClaudeCode) reported it is not in the catalogue")
	}
	if err := validateModel(d, "claude-fable-5-20260401"); err != nil {
		t.Fatalf("validateModel(full model name) = %v, want nil", err)
	}
}

func TestValidateModelAcceptsAnEmptyModel(t *testing.T) {
	d, _ := Lookup(Shell)
	if err := validateModel(d, ""); err != nil {
		t.Fatalf("validateModel(\"\") = %v, want nil: an unset field is not an option", err)
	}
}

func TestValidateModelRejectsAModelOnAHarnessWithNoModels(t *testing.T) {
	d, _ := Lookup(Shell)
	err := validateModel(d, "opus")
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Fatalf("validateModel(shell, \"opus\") = %v, want an error wrapping ErrUnsupportedOption", err)
	}
}
